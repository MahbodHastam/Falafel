package db

import (
	"context"
	"path/filepath"
	"testing"
)

func TestOpenAndSigningKey(t *testing.T) {
	ctx := context.Background()
	conn, err := Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	key1, err := SigningKey(ctx, conn, "")
	if err != nil {
		t.Fatalf("SigningKey 1: %v", err)
	}
	key2, err := SigningKey(ctx, conn, "")
	if err != nil {
		t.Fatalf("SigningKey 2: %v", err)
	}
	if string(key1) != string(key2) {
		t.Fatal("signing keys do not match across calls")
	}
}
