// Package api wires Falafel's HTTP surface.
package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mahbodhastam/falafel/internal/config"
	"github.com/mahbodhastam/falafel/internal/room"
	"github.com/mahbodhastam/falafel/internal/signaling"
)

const jsonBodyLimit = 16 * 1024

// Server exposes the REST and WebSocket API.
type Server struct {
	cfg           config.Config
	svc           *room.Service
	hub           *signaling.Hub
	log           *slog.Logger
	spa           http.Handler
	started       time.Time
	joinLimiter   *failureLimiter
	createLimiter *failureLimiter
}

// NewServer creates the HTTP API around an existing room service and hub.
func NewServer(cfg config.Config, svc *room.Service, hub *signaling.Hub, log *slog.Logger, spa http.Handler) *Server {
	return &Server{
		cfg:           cfg,
		svc:           svc,
		hub:           hub,
		log:           log,
		spa:           spa,
		started:       time.Now(),
		joinLimiter:   newFailureLimiter(5, 10*time.Minute, 60*time.Second),
		createLimiter: newFailureLimiter(30, time.Hour, time.Hour),
	}
}

// Handler returns the fully wrapped HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /api/config", s.handleConfig)
	mux.HandleFunc("POST /api/rooms", s.handleCreateRoom)
	mux.HandleFunc("GET /api/rooms/{token}", s.handleGetRoom)
	mux.HandleFunc("POST /api/rooms/{token}/join", s.handleJoin)
	mux.HandleFunc("GET /api/rooms/{token}/owner", s.handleOwner)
	mux.HandleFunc("POST /api/rooms/{token}/close", s.handleClose)
	mux.HandleFunc("GET /api/ws/{token}", s.handleWebSocket)
	mux.Handle("GET /", s.spa)
	return s.middleware(mux)
}

// middleware wraps the mux with panic recovery, security headers and API logs.
func (s *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				s.log.Error("panic recovered", "value", v, "path", r.URL.Path)
				writeError(w, http.StatusInternalServerError, "internal_error")
			}
		}()

		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self'; connect-src 'self' ws: wss:; media-src 'self' blob:; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)

		if strings.HasPrefix(r.URL.Path, "/api/") {
			s.log.Info("request",
				"method", r.Method,
				"path", maskTokenPath(r.URL.Path),
				"status", sw.status,
				"duration", time.Since(start).Round(time.Millisecond),
			)
		}
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (s *Server) writeRoomError(w http.ResponseWriter, err error, action string) {
	switch {
	case errors.Is(err, room.ErrUnauthorized):
		writeError(w, http.StatusUnauthorized, "unauthorized")
	case errors.Is(err, room.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found")
	default:
		s.log.Error(action, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error")
	}
}

type iceServer struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username,omitempty"`
	Credential string   `json:"credential,omitempty"`
}

func (s *Server) externalBaseURL(r *http.Request) string {
	if s.cfg.BaseURL != "" {
		return s.cfg.BaseURL
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if s.cfg.TrustProxy {
		if proto := r.Header.Get("X-Forwarded-Proto"); proto == "https" || proto == "http" {
			scheme = proto
		}
	}
	host := r.Host
	if s.cfg.TrustProxy {
		if forwarded := r.Header.Get("X-Forwarded-Host"); forwarded != "" {
			host = strings.TrimSpace(strings.Split(forwarded, ",")[0])
		}
	}
	return scheme + "://" + host
}

func clientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			if first := strings.TrimSpace(strings.Split(forwarded, ",")[0]); first != "" {
				return first
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func bearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(h), "bearer ") {
		val := strings.TrimSpace(h[7:])
		return val, val != ""
	}
	return "", false
}

func maskTokenPath(path string) string {
	parts := strings.Split(path, "/")
	for i := 0; i < len(parts)-1; i++ {
		if (parts[i] == "rooms" || parts[i] == "ws") && parts[i+1] != "" {
			parts[i+1] = "{token}"
		}
	}
	return strings.Join(parts, "/")
}

func validToken(token string) bool {
	if len(token) < 16 || len(token) > 64 {
		return false
	}
	for _, r := range token {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return false
	}
	return true
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, jsonBodyLimit)
	return json.NewDecoder(r.Body).Decode(dst)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code}})
}

func secondsCeil(d time.Duration) string {
	sec := int((d + time.Second - 1) / time.Second)
	if sec < 1 {
		sec = 1
	}
	return strconv.Itoa(sec)
}
