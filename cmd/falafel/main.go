// Command falafel runs the Falafel signaling server.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mahbodhastam/falafel/internal/api"
	"github.com/mahbodhastam/falafel/internal/config"
	"github.com/mahbodhastam/falafel/internal/db"
	"github.com/mahbodhastam/falafel/internal/room"
	"github.com/mahbodhastam/falafel/internal/signaling"
	"github.com/mahbodhastam/falafel/internal/web"
)

// version is set at release time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println("falafel", version)
		return
	}
	if err := run(); err != nil {
		slog.Error("falafel stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	conn, err := db.Open(ctx, cfg.DBPath)
	if err != nil {
		return err
	}
	defer conn.Close()

	signingKey, err := db.SigningKey(ctx, conn, cfg.Secret)
	if err != nil {
		return err
	}

	store := room.NewStore(conn)
	svc := room.NewService(store, signingKey, room.Options{
		TTL:               cfg.RoomTTL,
		MaxFileSize:       cfg.MaxFileSize,
		MinPasswordLength: cfg.MinPasswordLength,
	})
	hub := signaling.NewHub(svc, logger, cfg.OwnerGrace)

	clientFS, err := web.FS()
	if err != nil {
		return err
	}
	handler := api.NewServer(cfg, svc, hub, logger, api.NewSPA(clientFS, logger)).Handler()

	go janitor(ctx, svc, hub, logger, cfg.Retention)

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("falafel listening", "version", version, "addr", cfg.Addr, "baseURL", cfg.BaseURL)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func janitor(ctx context.Context, svc *room.Service, hub *signaling.Hub, logger *slog.Logger, retention time.Duration) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tokens, err := svc.ExpireStale(ctx)
			if err != nil {
				logger.Warn("expire rooms", "error", err)
				continue
			}
			for _, token := range tokens {
				hub.CloseRoom(token, "expired")
			}
			if _, err := svc.PurgeBefore(ctx, time.Now().Add(-retention)); err != nil {
				logger.Warn("purge rooms", "error", err)
			}
		}
	}
}
