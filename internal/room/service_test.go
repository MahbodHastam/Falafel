package room

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mahbodhastam/falafel/internal/db"
)

func newTestService(t *testing.T) (*Service, context.Context) {
	t.Helper()
	ctx := context.Background()
	conn, err := db.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	key, err := db.SigningKey(ctx, conn, "")
	if err != nil {
		t.Fatalf("signing key: %v", err)
	}
	return NewService(NewStore(conn), key, Options{
		TTL:               time.Hour,
		MaxFileSize:       1 << 30,
		MinPasswordLength: 4,
	}), ctx
}

func createRoom(t *testing.T, svc *Service, password string) *CreatedRoom {
	t.Helper()
	created, err := svc.Create(context.Background(), CreateInput{
		FileName: "chickpeas.txt",
		FileSize: 42,
		MimeType: "text/plain",
		Password: password,
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	return created
}

func TestCreateAndPublicView(t *testing.T) {
	svc, ctx := newTestService(t)
	created := createRoom(t, svc, "secret")

	if len(created.Room.Token) < 16 {
		t.Fatalf("token too short: %q", created.Room.Token)
	}
	if created.Room.Status != StatusWaiting {
		t.Fatalf("status = %q, want waiting", created.Room.Status)
	}

	pub, err := svc.Public(ctx, created.Room.Token)
	if err != nil {
		t.Fatalf("public: %v", err)
	}
	if !pub.PasswordRequired {
		t.Fatal("expected passwordRequired")
	}
	if pub.FileName != "chickpeas.txt" || pub.FileSize != 42 {
		t.Fatalf("unexpected metadata: %+v", pub)
	}
}

func TestPasswordVerificationAndAttempts(t *testing.T) {
	svc, ctx := newTestService(t)
	created := createRoom(t, svc, "tahini")

	if _, err := svc.Join(ctx, created.Room.Token, "wrong"); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("wrong password error = %v", err)
	}
	if _, err := svc.Join(ctx, created.Room.Token, ""); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("empty password error = %v", err)
	}

	owner, err := svc.Owner(ctx, created.Room.Token, created.OwnerSecret)
	if err != nil {
		t.Fatalf("owner: %v", err)
	}
	if owner.FailedAttempts != 2 {
		t.Fatalf("failed attempts = %d, want 2", owner.FailedAttempts)
	}

	ticket, err := svc.Join(ctx, created.Room.Token, "tahini")
	if err != nil {
		t.Fatalf("join: %v", err)
	}
	token, err := svc.ValidateTicket(ctx, ticket.Token)
	if err != nil {
		t.Fatalf("validate ticket: %v", err)
	}
	if token != created.Room.Token {
		t.Fatalf("ticket token = %q", token)
	}
}

func TestTicketRejectsTampering(t *testing.T) {
	svc, ctx := newTestService(t)
	created := createRoom(t, svc, "")
	ticket, err := svc.Join(ctx, created.Room.Token, "")
	if err != nil {
		t.Fatalf("join: %v", err)
	}

	parts := strings.Split(ticket.Token, ".")
	if len(parts) != 2 {
		t.Fatalf("ticket format: %q", ticket.Token)
	}
	tampered := parts[0] + "x." + parts[1]
	if _, err := svc.ValidateTicket(ctx, tampered); !errors.Is(err, ErrInvalidTicket) {
		t.Fatalf("tampered ticket error = %v", err)
	}
	if _, err := svc.ValidateTicket(ctx, "nonsense"); !errors.Is(err, ErrInvalidTicket) {
		t.Fatalf("malformed ticket error = %v", err)
	}
}

func TestOwnerSecret(t *testing.T) {
	svc, ctx := newTestService(t)
	created := createRoom(t, svc, "")

	if _, err := svc.Owner(ctx, created.Room.Token, "not-the-secret"); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("owner with bad secret error = %v", err)
	}
	if err := svc.Close(ctx, created.Room.Token, "not-the-secret"); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("close with bad secret error = %v", err)
	}
	if err := svc.Close(ctx, created.Room.Token, created.OwnerSecret); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, err := svc.Public(ctx, created.Room.Token); !errors.Is(err, ErrNotFound) {
		t.Fatalf("public after close error = %v", err)
	}
	if err := svc.Close(ctx, created.Room.Token, created.OwnerSecret); err != nil {
		t.Fatalf("idempotent close: %v", err)
	}
}

func TestLifecycleCompletedRoomIsGone(t *testing.T) {
	svc, ctx := newTestService(t)
	created := createRoom(t, svc, "")

	if err := svc.MarkPaired(ctx, created.Room.Token); err != nil {
		t.Fatalf("paired: %v", err)
	}
	if err := svc.MarkTransferring(ctx, created.Room.Token); err != nil {
		t.Fatalf("transferring: %v", err)
	}
	if err := svc.MarkTransferring(ctx, created.Room.Token); err != nil {
		t.Fatalf("transferring again: %v", err)
	}
	if err := svc.SetProgress(ctx, created.Room.Token, 21); err != nil {
		t.Fatalf("progress: %v", err)
	}
	if err := svc.MarkCompleted(ctx, created.Room.Token, 42); err != nil {
		t.Fatalf("complete: %v", err)
	}

	if _, err := svc.Public(ctx, created.Room.Token); !errors.Is(err, ErrNotFound) {
		t.Fatalf("completed room must be hidden, got %v", err)
	}

	owner, err := svc.Owner(ctx, created.Room.Token, created.OwnerSecret)
	if err != nil {
		t.Fatalf("owner after complete: %v", err)
	}
	if owner.Status != StatusCompleted || owner.BytesReceived != 42 {
		t.Fatalf("owner stats = %+v", owner)
	}
	if owner.CompletedAt == nil {
		t.Fatal("completedAt not set")
	}

	events := map[string]int{}
	for _, event := range owner.Events {
		events[event.Type]++
	}
	if events[EventTransferCompleted] != 1 {
		t.Fatalf("completion events = %d, want 1", events[EventTransferCompleted])
	}
	if events[EventTransferStarted] != 1 {
		t.Fatalf("start events = %d, want 1", events[EventTransferStarted])
	}
}

func TestExpiry(t *testing.T) {
	svc, ctx := newTestService(t)
	created := createRoom(t, svc, "")
	svc.SetClock(func() time.Time { return time.Now().Add(2 * time.Hour) })

	stale, err := svc.ExpireStale(ctx)
	if err != nil {
		t.Fatalf("expire stale: %v", err)
	}
	if len(stale) != 1 || stale[0] != created.Room.Token {
		t.Fatalf("expired tokens = %v", stale)
	}
	if _, err := svc.Public(ctx, created.Room.Token); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expired room error = %v", err)
	}
}

func TestValidation(t *testing.T) {
	svc, ctx := newTestService(t)
	cases := []struct {
		name string
		in   CreateInput
		file string
	}{
		{"empty name", CreateInput{FileSize: 1}, "fileName"},
		{"zero size", CreateInput{FileName: "a.txt"}, "fileSize"},
		{"too large", CreateInput{FileName: "a.txt", FileSize: 2 << 30}, "fileSize"},
		{"short password", CreateInput{FileName: "a.txt", FileSize: 1, Password: "abc"}, "password"},
		{"control chars", CreateInput{FileName: "a\x00b", FileSize: 1}, "fileName"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Create(ctx, tc.in)
			var invalid *ValidationError
			if !errors.As(err, &invalid) {
				t.Fatalf("error = %v, want ValidationError", err)
			}
			if invalid.Field != tc.file {
				t.Fatalf("field = %q, want %q", invalid.Field, tc.file)
			}
		})
	}
}
