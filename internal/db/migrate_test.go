package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func userVersion(t *testing.T, conn *sql.DB) int {
	t.Helper()
	var v int
	if err := conn.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestOpenRecordsSchemaVersionAndReopens(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "v.db")

	conn, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if got := userVersion(t, conn); got != len(migrations) {
		t.Fatalf("user_version = %d, want %d", got, len(migrations))
	}
	conn.Close()

	conn, err = Open(ctx, path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	conn.Close()
}

func TestOpenAdoptsUnversionedDatabase(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "legacy.db")

	// Databases created before versioning have the tables but user_version 0.
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(schema); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`INSERT INTO meta (key, value) VALUES ('signing_key', x'0102')`); err != nil {
		t.Fatal(err)
	}
	raw.Close()

	conn, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("open legacy: %v", err)
	}
	defer conn.Close()
	if got := userVersion(t, conn); got != len(migrations) {
		t.Fatalf("user_version = %d, want %d", got, len(migrations))
	}
	var n int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM meta`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("legacy data lost: n=%d err=%v", n, err)
	}
}

func TestOpenRefusesNewerSchema(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "future.db")

	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec("PRAGMA user_version = 999"); err != nil {
		t.Fatal(err)
	}
	raw.Close()

	if _, err := Open(ctx, path); err == nil {
		t.Fatal("expected an error for a database from a newer build")
	}
}
