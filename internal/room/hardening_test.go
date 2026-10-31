package room

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestConcurrentWrongPasswordsAreAllCounted(t *testing.T) {
	svc, ctx := newTestService(t)
	created := createRoom(t, svc, "tahini")

	const attempts = 12
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := svc.Join(ctx, created.Room.Token, "wrong"); !errors.Is(err, ErrWrongPassword) {
				t.Errorf("join error = %v", err)
			}
		}()
	}
	wg.Wait()

	owner, err := svc.Owner(ctx, created.Room.Token, created.OwnerSecret)
	if err != nil {
		t.Fatalf("owner: %v", err)
	}
	if owner.FailedAttempts != attempts {
		t.Fatalf("failed attempts = %d, want %d", owner.FailedAttempts, attempts)
	}
}

func TestHashSlotsBoundConcurrency(t *testing.T) {
	svc, _ := newTestService(t)
	var releases []func()
	for i := 0; i < cap(svc.hashSlots); i++ {
		release, err := svc.acquireHashSlot(context.Background())
		if err != nil {
			t.Fatalf("acquire %d: %v", i, err)
		}
		releases = append(releases, release)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := svc.acquireHashSlot(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("acquire with full slots = %v, want deadline exceeded", err)
	}

	releases[0]()
	release, err := svc.acquireHashSlot(context.Background())
	if err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
	release()
}

func TestDownloaderLeft(t *testing.T) {
	svc, ctx := newTestService(t)

	t.Run("browsing without transferring does not fail the room", func(t *testing.T) {
		created := createRoom(t, svc, "")
		if err := svc.MarkPaired(ctx, created.Room.Token); err != nil {
			t.Fatal(err)
		}
		if err := svc.DownloaderLeft(ctx, created.Room.Token); err != nil {
			t.Fatal(err)
		}
		owner, _ := svc.Owner(ctx, created.Room.Token, created.OwnerSecret)
		if owner.Status != StatusWaiting {
			t.Fatalf("status = %q, want waiting", owner.Status)
		}
		for _, e := range owner.Events {
			if e.Type == EventTransferFailed {
				t.Fatal("unexpected transfer_failed event")
			}
		}
	})

	t.Run("leaving mid-transfer fails the room", func(t *testing.T) {
		created := createRoom(t, svc, "")
		if err := svc.MarkTransferring(ctx, created.Room.Token); err != nil {
			t.Fatal(err)
		}
		if err := svc.DownloaderLeft(ctx, created.Room.Token); err != nil {
			t.Fatal(err)
		}
		owner, _ := svc.Owner(ctx, created.Room.Token, created.OwnerSecret)
		if owner.Status != StatusFailed {
			t.Fatalf("status = %q, want failed", owner.Status)
		}
	})
}

func TestRejectsBidiOverrideInFileName(t *testing.T) {
	svc, ctx := newTestService(t)
	for _, name := range []string{"invoice‮fdp.exe", "a⁦b.txt"} {
		_, err := svc.Create(ctx, CreateInput{FileName: name, FileSize: 1})
		var invalid *ValidationError
		if !errors.As(err, &invalid) || invalid.Field != "fileName" {
			t.Fatalf("%q: error = %v", name, err)
		}
	}
	// Persian file names use ZWNJ and RLM legitimately.
	if _, err := svc.Create(ctx, CreateInput{FileName: "می‌خواهم‏.txt", FileSize: 1}); err != nil {
		t.Fatalf("persian name rejected: %v", err)
	}
}

func TestTicketExpires(t *testing.T) {
	svc, ctx := newTestService(t)
	created := createRoom(t, svc, "")
	ticket, err := svc.Join(ctx, created.Room.Token, "")
	if err != nil {
		t.Fatal(err)
	}
	svc.SetClock(func() time.Time { return time.Now().Add(3 * time.Minute) })
	if _, err := svc.ValidateTicket(ctx, ticket.Token); !errors.Is(err, ErrInvalidTicket) {
		t.Fatalf("expired ticket error = %v", err)
	}
}

func TestPurgeRemovesOnlyOldTerminalRooms(t *testing.T) {
	svc, ctx := newTestService(t)
	old := createRoom(t, svc, "")
	if err := svc.Close(ctx, old.Room.Token, old.OwnerSecret); err != nil {
		t.Fatal(err)
	}
	live := createRoom(t, svc, "")

	n, err := svc.PurgeBefore(ctx, time.Now().Add(48*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("purged %d rooms, want 1", n)
	}
	if _, err := svc.Owner(ctx, old.Room.Token, old.OwnerSecret); !errors.Is(err, ErrNotFound) {
		t.Fatalf("closed room should be gone, got %v", err)
	}
	if _, err := svc.Owner(ctx, live.Room.Token, live.OwnerSecret); err != nil {
		t.Fatalf("waiting room must survive purge: %v", err)
	}
}
