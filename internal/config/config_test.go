package config

import (
	"testing"
)

func TestConfigLoadDefaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Addr != ":8080" {
		t.Fatalf("Addr = %q, want :8080", cfg.Addr)
	}
	if cfg.MinPasswordLength != 4 {
		t.Fatalf("MinPasswordLength = %d, want 4", cfg.MinPasswordLength)
	}
}

func TestConfigValidation(t *testing.T) {
	t.Setenv("FALAFEL_MAX_FILE_SIZE", "-1")
	if _, err := Load(); err == nil {
		t.Fatal("expected error on negative max file size")
	}
}
