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

func TestTURNConfiguration(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		ok   bool
	}{
		{"urls alone are rejected", map[string]string{"FALAFEL_TURN_URLS": "turn:t.example:3478"}, false},
		{"username without credential is rejected", map[string]string{"FALAFEL_TURN_URLS": "turn:t.example:3478", "FALAFEL_TURN_USERNAME": "u"}, false},
		{"static credentials", map[string]string{"FALAFEL_TURN_URLS": "turn:t.example:3478", "FALAFEL_TURN_USERNAME": "u", "FALAFEL_TURN_CREDENTIAL": "p"}, true},
		{"shared secret", map[string]string{"FALAFEL_TURN_URLS": "turn:t.example:3478", "FALAFEL_TURN_SECRET": "s"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			_, err := Load()
			if (err == nil) != tc.ok {
				t.Fatalf("Load error = %v, want ok=%v", err, tc.ok)
			}
		})
	}
}

func TestBaseURLValidation(t *testing.T) {
	for raw, ok := range map[string]bool{
		"https://falafel.example.com": true,
		"http://localhost:8080/":      true,
		"httpfoo":                     false,
		"ftp://falafel.example.com":   false,
		"https://":                    false,
		"falafel.example.com":         false,
	} {
		t.Setenv("FALAFEL_BASE_URL", raw)
		if _, err := Load(); (err == nil) != ok {
			t.Fatalf("FALAFEL_BASE_URL=%q: error = %v, want ok=%v", raw, err, ok)
		}
	}
}
