package signaling

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/mahbodhastam/falafel/internal/db"
	"github.com/mahbodhastam/falafel/internal/room"
)

type hubEnv struct {
	server *httptest.Server
	svc    *room.Service
	hub    *Hub
	grace  time.Duration
}

func newHubEnv(t *testing.T) *hubEnv {
	t.Helper()
	ctx := context.Background()
	conn, err := db.Open(ctx, filepath.Join(t.TempDir(), "hub.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	key, err := db.SigningKey(ctx, conn, "")
	if err != nil {
		t.Fatalf("signing key: %v", err)
	}
	svc := room.NewService(room.NewStore(conn), key, room.Options{
		TTL:               time.Hour,
		MaxFileSize:       1 << 20,
		MinPasswordLength: 4,
	})
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	grace := 150 * time.Millisecond
	hub := NewHub(svc, logger, grace)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/ws/{token}", func(w http.ResponseWriter, r *http.Request) {
		hub.ServeWS(w, r, r.PathValue("token"))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return &hubEnv{server: server, svc: svc, hub: hub, grace: grace}
}

func (e *hubEnv) createRoom(t *testing.T) (*room.CreatedRoom, string) {
	t.Helper()
	ctx := context.Background()
	created, err := e.svc.Create(ctx, room.CreateInput{
		FileName: "falafel.pdf",
		FileSize: 1024,
		MimeType: "application/pdf",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	ticket, err := e.svc.Join(ctx, created.Room.Token, "")
	if err != nil {
		t.Fatalf("join: %v", err)
	}
	return created, ticket.Token
}

func (e *hubEnv) wsURL(token string) string {
	return "ws" + strings.TrimPrefix(e.server.URL, "http") + "/api/ws/" + token
}

func dialAndAuth(t *testing.T, url string, auth ClientMessage) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if err := wsjson.Write(ctx, conn, auth); err != nil {
		t.Fatalf("send auth: %v", err)
	}
	return conn
}

func readMessage(t *testing.T, conn *websocket.Conn) ServerMessage {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var msg ServerMessage
	if err := wsjson.Read(ctx, conn, &msg); err != nil {
		t.Fatalf("read message: %v", err)
	}
	return msg
}

func TestSignalingRelayAndLifecycle(t *testing.T) {
	env := newHubEnv(t)
	created, ticket := env.createRoom(t)
	url := env.wsURL(created.Room.Token)

	owner := dialAndAuth(t, url, ClientMessage{Type: "auth", Role: RoleOwner, OwnerSecret: created.OwnerSecret})
	defer owner.CloseNow()
	ownerReady := readMessage(t, owner)
	if ownerReady.Type != "ready" || ownerReady.Role != RoleOwner {
		t.Fatalf("owner ready = %+v", ownerReady)
	}
	if ownerReady.PeerOnline {
		t.Fatal("owner should have no peer yet")
	}

	downloader := dialAndAuth(t, url, ClientMessage{Type: "auth", Role: RoleDownloader, Ticket: ticket})
	defer downloader.CloseNow()
	downReady := readMessage(t, downloader)
	if downReady.Type != "ready" || downReady.Role != RoleDownloader || !downReady.PeerOnline {
		t.Fatalf("downloader ready = %+v", downReady)
	}

	joined := readMessage(t, owner)
	if joined.Type != "peer-joined" || !joined.PeerOnline {
		t.Fatalf("peer-joined = %+v", joined)
	}

	// Owner -> downloader signaling relay.
	payload := json.RawMessage(`{"sdp":"v=0","type":"offer"}`)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := wsjson.Write(ctx, owner, ClientMessage{Type: "signal", Data: payload}); err != nil {
		t.Fatalf("owner signal: %v", err)
	}
	relayed := readMessage(t, downloader)
	if relayed.Type != "signal" || relayed.From != RoleOwner || string(relayed.Data) != string(payload) {
		t.Fatalf("relayed signal = %+v data=%s", relayed, relayed.Data)
	}

	// Downloader -> owner signaling relay.
	answer := json.RawMessage(`{"sdp":"v=0","type":"answer"}`)
	if err := wsjson.Write(ctx, downloader, ClientMessage{Type: "signal", Data: answer}); err != nil {
		t.Fatalf("downloader signal: %v", err)
	}
	relayed = readMessage(t, owner)
	if relayed.From != RoleDownloader || string(relayed.Data) != string(answer) {
		t.Fatalf("relayed answer = %+v", relayed)
	}

	// A second downloader is rejected while the room is busy.
	second := dialAndAuth(t, url, ClientMessage{Type: "auth", Role: RoleDownloader, Ticket: ticket})
	defer second.CloseNow()
	busy := readMessage(t, second)
	if busy.Type != "error" || busy.Code != "room_busy" {
		t.Fatalf("second downloader = %+v", busy)
	}

	// Owner departure eventually closes the room and the downloader socket.
	if err := owner.Close(websocket.StatusNormalClosure, "bye"); err != nil {
		t.Fatalf("close owner: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	gotClosed := false
	for time.Now().Before(deadline) && !gotClosed {
		readCtx, cancelRead := context.WithTimeout(context.Background(), time.Second)
		var msg ServerMessage
		err := wsjson.Read(readCtx, downloader, &msg)
		cancelRead()
		if err != nil {
			break
		}
		if msg.Type == "closed" && msg.Reason == "owner_left" {
			gotClosed = true
		}
	}
	if !gotClosed {
		t.Fatal("downloader never received owner_left close")
	}

	var pubErr error
	for i := 0; i < 20; i++ {
		_, pubErr = env.svc.Public(context.Background(), created.Room.Token)
		if errors.Is(pubErr, room.ErrNotFound) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !errors.Is(pubErr, room.ErrNotFound) {
		t.Fatalf("room should be closed, got %v", pubErr)
	}
}

func TestInvalidAuthRejected(t *testing.T) {
	env := newHubEnv(t)
	created, _ := env.createRoom(t)
	url := env.wsURL(created.Room.Token)

	conn := dialAndAuth(t, url, ClientMessage{Type: "auth", Role: RoleOwner, OwnerSecret: "wrong"})
	defer conn.CloseNow()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var msg ServerMessage
	err := wsjson.Read(ctx, conn, &msg)
	if err == nil {
		t.Fatalf("expected close, got message %+v", msg)
	}
	var closeErr websocket.CloseError
	if !errors.As(err, &closeErr) || closeErr.Code != closeUnauthorized {
		t.Fatalf("close error = %v", err)
	}
}

func TestOwnerStatusStored(t *testing.T) {
	env := newHubEnv(t)
	created, ticket := env.createRoom(t)
	url := env.wsURL(created.Room.Token)

	owner := dialAndAuth(t, url, ClientMessage{Type: "auth", Role: RoleOwner, OwnerSecret: created.OwnerSecret})
	defer owner.CloseNow()
	readMessage(t, owner)

	downloader := dialAndAuth(t, url, ClientMessage{Type: "auth", Role: RoleDownloader, Ticket: ticket})
	defer downloader.CloseNow()
	readMessage(t, downloader)
	readMessage(t, owner) // peer-joined

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := wsjson.Write(ctx, owner, ClientMessage{Type: "status", Phase: "transferring", Bytes: 512}); err != nil {
		t.Fatalf("status write: %v", err)
	}
	if err := wsjson.Write(ctx, owner, ClientMessage{Type: "status", Phase: "completed", Bytes: 1024}); err != nil {
		t.Fatalf("complete write: %v", err)
	}

	var snapshot *room.OwnerRoom
	for i := 0; i < 40; i++ {
		ownerRoom, err := env.svc.Owner(context.Background(), created.Room.Token, created.OwnerSecret)
		if err != nil {
			t.Fatalf("owner snapshot: %v", err)
		}
		if ownerRoom.Status == room.StatusCompleted {
			snapshot = ownerRoom
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if snapshot == nil {
		t.Fatal("room never completed")
	}
	if snapshot.BytesReceived != 1024 {
		t.Fatalf("bytes = %d, want 1024", snapshot.BytesReceived)
	}
}

func TestOwnerReconnectWithinGraceKeepsRoom(t *testing.T) {
	env := newHubEnv(t)
	created, _ := env.createRoom(t)
	url := env.wsURL(created.Room.Token)
	auth := ClientMessage{Type: "auth", Role: RoleOwner, OwnerSecret: created.OwnerSecret}

	first := dialAndAuth(t, url, auth)
	readMessage(t, first)
	first.CloseNow()

	time.Sleep(env.grace / 3)
	second := dialAndAuth(t, url, auth)
	defer second.CloseNow()
	if ready := readMessage(t, second); ready.Type != "ready" {
		t.Fatalf("reconnect = %+v", ready)
	}

	time.Sleep(env.grace * 2)
	if _, err := env.svc.Public(context.Background(), created.Room.Token); err != nil {
		t.Fatalf("room closed despite owner reconnecting: %v", err)
	}
}

func TestSecondOwnerRejected(t *testing.T) {
	env := newHubEnv(t)
	created, _ := env.createRoom(t)
	url := env.wsURL(created.Room.Token)
	auth := ClientMessage{Type: "auth", Role: RoleOwner, OwnerSecret: created.OwnerSecret}

	first := dialAndAuth(t, url, auth)
	defer first.CloseNow()
	readMessage(t, first)

	second := dialAndAuth(t, url, auth)
	defer second.CloseNow()
	if msg := readMessage(t, second); msg.Type != "error" || msg.Code != "room_busy" {
		t.Fatalf("second owner = %+v", msg)
	}
}

func TestDownloaderLeavingWithoutTransferKeepsRoomJoinable(t *testing.T) {
	env := newHubEnv(t)
	created, ticket := env.createRoom(t)
	url := env.wsURL(created.Room.Token)

	owner := dialAndAuth(t, url, ClientMessage{Type: "auth", Role: RoleOwner, OwnerSecret: created.OwnerSecret})
	defer owner.CloseNow()
	readMessage(t, owner)
	downloader := dialAndAuth(t, url, ClientMessage{Type: "auth", Role: RoleDownloader, Ticket: ticket})
	readMessage(t, downloader)
	readMessage(t, owner) // peer-joined
	downloader.CloseNow()

	if msg := readMessage(t, owner); msg.Type != "peer-left" {
		t.Fatalf("owner message = %+v", msg)
	}
	var status room.Status
	for i := 0; i < 40; i++ {
		snapshot, err := env.svc.Owner(context.Background(), created.Room.Token, created.OwnerSecret)
		if err != nil {
			t.Fatal(err)
		}
		if status = snapshot.Status; status == room.StatusWaiting {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("status = %q, want waiting", status)
}

func TestHubForgetsEmptyRooms(t *testing.T) {
	env := newHubEnv(t)
	created, ticket := env.createRoom(t)
	url := env.wsURL(created.Room.Token)

	downloader := dialAndAuth(t, url, ClientMessage{Type: "auth", Role: RoleDownloader, Ticket: ticket})
	readMessage(t, downloader)
	downloader.CloseNow()

	for i := 0; i < 40; i++ {
		env.hub.mu.Lock()
		n := len(env.hub.rooms)
		env.hub.mu.Unlock()
		if n == 0 {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("hub still tracks a room with nobody connected")
}
