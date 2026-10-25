// Package config loads Falafel's runtime configuration from the environment.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the complete runtime configuration.
type Config struct {
	Addr              string
	DBPath            string
	BaseURL           string
	RoomTTL           time.Duration
	MaxFileSize       int64
	MinPasswordLength int
	OwnerGrace        time.Duration
	Retention         time.Duration
	STUNURLs          []string
	TURNURLs          []string
	TURNUsername      string
	TURNCredential    string
	Secret            string
	TrustProxy        bool
}

// Load reads configuration from environment variables, applying defaults.
func Load() (cfg Config, err error) {
	// ponytail: stdlib env parsing with single-pass defaults
	cfg.Addr = env("FALAFEL_ADDR", ":8080")
	cfg.DBPath = env("FALAFEL_DB_PATH", "./data/falafel.db")
	cfg.BaseURL = strings.TrimRight(env("FALAFEL_BASE_URL", ""), "/")
	cfg.STUNURLs = splitList(env("FALAFEL_STUN_URLS", "stun:stun.cloudflare.com:3478,stun:stun.l.google.com:19302"))
	cfg.TURNURLs = splitList(env("FALAFEL_TURN_URLS", env("FALAFEL_TURN_URL", "")))
	cfg.TURNUsername = env("FALAFEL_TURN_USERNAME", "")
	cfg.TURNCredential = env("FALAFEL_TURN_CREDENTIAL", "")
	cfg.Secret = env("FALAFEL_SECRET", "")
	cfg.TrustProxy = envBool("FALAFEL_TRUST_PROXY", false)

	if cfg.RoomTTL, err = envDuration("FALAFEL_ROOM_TTL", 24*time.Hour); err != nil {
		return Config{}, err
	}
	if cfg.OwnerGrace, err = envDuration("FALAFEL_OWNER_GRACE", 10*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.Retention, err = envDuration("FALAFEL_RETENTION", 7*24*time.Hour); err != nil {
		return Config{}, err
	}
	if cfg.MaxFileSize, err = envInt64("FALAFEL_MAX_FILE_SIZE", 10<<30); err != nil {
		return Config{}, err
	}
	minLen, err := envInt64("FALAFEL_MIN_PASSWORD_LENGTH", 4)
	if err != nil {
		return Config{}, err
	}
	cfg.MinPasswordLength = int(minLen)

	switch {
	case cfg.Addr == "":
		return Config{}, errors.New("FALAFEL_ADDR must not be empty")
	case cfg.DBPath == "":
		return Config{}, errors.New("FALAFEL_DB_PATH must not be empty")
	case cfg.MaxFileSize <= 0:
		return Config{}, errors.New("FALAFEL_MAX_FILE_SIZE must be positive")
	case cfg.MinPasswordLength < 1 || cfg.MinPasswordLength > 128:
		return Config{}, errors.New("FALAFEL_MIN_PASSWORD_LENGTH must be between 1 and 128")
	case cfg.RoomTTL <= 0:
		return Config{}, errors.New("FALAFEL_ROOM_TTL must be positive")
	case cfg.BaseURL != "" && !strings.HasPrefix(cfg.BaseURL, "http"):
		return Config{}, errors.New("FALAFEL_BASE_URL must start with http:// or https://")
	case len(cfg.TURNURLs) > 0 && cfg.TURNUsername == "":
		return Config{}, errors.New("FALAFEL_TURN_USERNAME is required when FALAFEL_TURN_URLS is set")
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if b, err := strconv.ParseBool(env(key, "")); err == nil {
		return b
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) (time.Duration, error) {
	v := env(key, "")
	if v == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%s must be positive", key)
	}
	return d, nil
}

func envInt64(key string, fallback int64) (int64, error) {
	v := env(key, "")
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return n, nil
}

func splitList(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
