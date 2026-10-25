// Package db opens and migrates Falafel's SQLite database.
package db

import (
	"context"
	"crypto/rand"
	"database/sql"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

// Open creates the database (and its parent directory) and applies the schema.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	if path != ":memory:" && !strings.HasPrefix(path, "file::memory:") {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
	}

	dsn := path
	if path == ":memory:" {
		dsn = "file::memory:?cache=shared"
	}
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	dsn += sep + "_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)"

	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	// ponytail: single connection to serialize SQLite writes safely
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)

	if err := conn.PingContext(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	if _, err := conn.ExecContext(ctx, schema); err != nil {
		conn.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return conn, nil
}

// SigningKey returns the persisted HMAC key, generating it on first use.
// When override is a valid base64 string it is used instead and never stored.
func SigningKey(ctx context.Context, conn *sql.DB, override string) ([]byte, error) {
	if override != "" {
		key, err := base64.RawURLEncoding.DecodeString(override)
		if err != nil || len(key) < 16 {
			return nil, errors.New("FALAFEL_SECRET must be a base64url string of at least 16 bytes")
		}
		return key, nil
	}

	var stored []byte
	err := conn.QueryRowContext(ctx, "SELECT value FROM meta WHERE key = 'signing_key'").Scan(&stored)
	switch {
	case err == nil:
		return stored, nil
	case !errors.Is(err, sql.ErrNoRows):
		return nil, fmt.Errorf("read signing key: %w", err)
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate signing key: %w", err)
	}
	if _, err := conn.ExecContext(ctx, "INSERT INTO meta (key, value) VALUES ('signing_key', ?)", key); err != nil {
		return nil, fmt.Errorf("store signing key: %w", err)
	}
	return key, nil
}
