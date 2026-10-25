// Package signaling relays WebRTC session descriptions between the two peers
// of a room and keeps the owner's statistics up to date. It never carries file content.
package signaling

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/mahbodhastam/falafel/internal/room"
)

// Roles for a signaling connection.
const (
	RoleOwner      = "owner"
	RoleDownloader = "downloader"

	authTimeout    = 5 * time.Second
	writeTimeout   = 10 * time.Second
	pingInterval   = 25 * time.Second
	maxSignalBytes = 256 * 1024

	// Application close codes mirror HTTP 4xx meanings for socket errors.
	closeBadRequest   = 4400
	closeUnauthorized = 4401
	closeNotFound     = 4404
	closeBusy         = 4409
)

var errRoomBusy = errors.New("room busy")

// ClientMessage is the inbound JSON envelope.
type ClientMessage struct {
	Type        string          `json:"type"`
	Role        string          `json:"role,omitempty"`
	OwnerSecret string          `json:"ownerSecret,omitempty"`
	Ticket      string          `json:"ticket,omitempty"`
	Data        json.RawMessage `json:"data,omitempty"`
	Phase       string          `json:"phase,omitempty"`
	Bytes       int64           `json:"bytes,omitempty"`
}

// ServerMessage is the outbound JSON envelope.
type ServerMessage struct {
	Type       string          `json:"type"`
	Role       string          `json:"role,omitempty"`
	Room       any             `json:"room,omitempty"`
	Stats      any             `json:"stats,omitempty"`
	Events     any             `json:"events,omitempty"`
	PeerOnline bool            `json:"peerOnline"`
	Data       json.RawMessage `json:"data,omitempty"`
	From       string          `json:"from,omitempty"`
	Reason     string          `json:"reason,omitempty"`
	Code       string          `json:"code,omitempty"`
}

// conn is one authenticated socket; writeMu serializes JSON writes.
type conn struct {
	ws      *websocket.Conn
	token   string
	role    string
	writeMu sync.Mutex
}

func (c *conn) write(msg ServerMessage) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
	defer cancel()
	return wsjson.Write(ctx, c.ws, msg)
}

func (c *conn) pingLoop(ctx context.Context) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pingCtx, cancel := context.WithTimeout(ctx, writeTimeout)
			err := c.ws.Ping(pingCtx)
			cancel()
			if err != nil {
				c.ws.CloseNow()
				return
			}
		}
	}
}

// peers tracks the two sockets of one live room.
type peers struct {
	owner      *conn
	downloader *conn
	completed  bool
	ownerLeft  *disconnect
	closeTimer *time.Timer
}

type disconnect struct{}

// Hub owns all live signaling rooms.
type Hub struct {
	svc    *room.Service
	logger *slog.Logger
	grace  time.Duration

	mu    sync.Mutex
	rooms map[string]*peers
}

// NewHub creates a hub; grace is how long an owner may reconnect before the room closes.
func NewHub(svc *room.Service, logger *slog.Logger, grace time.Duration) *Hub {
	return &Hub{
		svc:    svc,
		logger: logger,
		grace:  grace,
		rooms:  make(map[string]*peers),
	}
}

// ServeWS upgrades an HTTP request and runs one signaling participant.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request, token string) {
	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		CompressionMode: websocket.CompressionDisabled,
	})
	if err != nil {
		h.logger.Debug("websocket accept failed", "error", err)
		return
	}
	ws.SetReadLimit(maxSignalBytes)
	defer ws.CloseNow()

	auth, failCode, failReason := h.authenticate(ws, token)
	if failCode != 0 {
		_ = ws.Close(failCode, failReason)
		return
	}

	c := &conn{ws: ws, token: token, role: auth.Role}
	ready, peerJoined, err := h.attach(token, auth.Role, c, auth.OwnerSecret)
	if err != nil {
		code, closeCode := closeInfoFor(err)
		_ = c.write(ServerMessage{Type: "error", Code: code})
		_ = ws.Close(closeCode, code)
		return
	}
	defer h.detach(token, auth.Role, c)

	if err := c.write(*ready); err != nil {
		return
	}
	if peerJoined {
		h.sendToOther(token, auth.Role, ServerMessage{Type: "peer-joined", PeerOnline: true})
	}

	pingCtx, stopPings := context.WithCancel(context.Background())
	defer stopPings()
	go c.pingLoop(pingCtx)

	for {
		var msg ClientMessage
		if err := wsjson.Read(context.Background(), ws, &msg); err != nil {
			return
		}
		h.handle(c, msg)
	}
}

func (h *Hub) authenticate(ws *websocket.Conn, token string) (ClientMessage, websocket.StatusCode, string) {
	authCtx, cancelAuth := context.WithTimeout(context.Background(), authTimeout)
	defer cancelAuth()

	var auth ClientMessage
	if err := wsjson.Read(authCtx, ws, &auth); err != nil {
		return auth, closeBadRequest, "auth timeout"
	}
	if auth.Type != "auth" {
		return auth, closeBadRequest, "auth required"
	}
	switch auth.Role {
	case RoleOwner:
		ok, err := h.svc.IsOwner(context.Background(), token, auth.OwnerSecret)
		if err != nil || !ok {
			return auth, closeUnauthorized, "invalid owner secret"
		}
	case RoleDownloader:
		ticketToken, err := h.svc.ValidateTicket(context.Background(), auth.Ticket)
		if err != nil || ticketToken != token {
			return auth, closeUnauthorized, "invalid ticket"
		}
	default:
		return auth, closeBadRequest, "invalid role"
	}
	return auth, 0, ""
}

func closeInfoFor(err error) (string, websocket.StatusCode) {
	switch {
	case errors.Is(err, errRoomBusy):
		return "room_busy", closeBusy
	case errors.Is(err, room.ErrNotFound):
		return "room_not_found", closeNotFound
	case errors.Is(err, room.ErrUnauthorized):
		return "unauthorized", closeUnauthorized
	default:
		return "connection_failed", closeBadRequest
	}
}

func (h *Hub) handle(c *conn, msg ClientMessage) {
	switch msg.Type {
	case "signal":
		if len(msg.Data) == 0 || len(msg.Data) > maxSignalBytes {
			_ = c.write(ServerMessage{Type: "error", Code: "invalid_signal"})
			return
		}
		h.sendToOther(c.token, c.role, ServerMessage{
			Type: "signal",
			From: c.role,
			Data: msg.Data,
		})
	case "status":
		h.handleStatus(c, msg)
	case "cancel":
		if c.role != RoleOwner {
			return
		}
		if err := h.svc.CloseByToken(context.Background(), c.token); err != nil {
			h.logger.Warn("owner cancel failed", "token", c.token, "error", err)
		}
		h.CloseRoom(c.token, "owner_closed")
	default:
		_ = c.write(ServerMessage{Type: "error", Code: "unknown_message"})
	}
}

func (h *Hub) handleStatus(c *conn, msg ClientMessage) {
	if msg.Bytes < 0 {
		msg.Bytes = 0
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if msg.Phase == "completed" {
		if err := h.svc.MarkCompleted(ctx, c.token, msg.Bytes); err != nil && !errors.Is(err, room.ErrNotFound) {
			h.logger.Warn("mark completed", "token", c.token, "error", err)
		}
		h.markCompleted(c.token)
		h.NotifyStats(c.token)
		return
	}

	if c.role != RoleOwner {
		return
	}

	switch msg.Phase {
	case "transferring":
		if err := h.svc.MarkTransferring(ctx, c.token); err != nil && !errors.Is(err, room.ErrNotFound) {
			h.logger.Warn("mark transferring", "token", c.token, "error", err)
		}
		if err := h.svc.SetProgress(ctx, c.token, msg.Bytes); err != nil && !errors.Is(err, room.ErrNotFound) {
			h.logger.Warn("set progress", "token", c.token, "error", err)
		}
	case "failed":
		if err := h.svc.MarkFailed(ctx, c.token, "transfer_error"); err != nil && !errors.Is(err, room.ErrNotFound) {
			h.logger.Warn("mark failed", "token", c.token, "error", err)
		}
		h.NotifyStats(c.token)
	}
}

// attach registers a connection and builds its ready payload.
func (h *Hub) attach(token, role string, c *conn, ownerSecret string) (*ServerMessage, bool, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	p := h.rooms[token]
	if p == nil {
		p = &peers{}
		h.rooms[token] = p
	}

	if role == RoleOwner {
		if p.owner != nil {
			return nil, false, errRoomBusy
		}
		if p.closeTimer != nil {
			p.closeTimer.Stop()
			p.closeTimer = nil
		}
		p.ownerLeft = nil
		p.owner = c
		owner, err := h.svc.Owner(context.Background(), token, ownerSecret)
		if err != nil {
			p.owner = nil
			return nil, false, err
		}
		return &ServerMessage{
			Type:       "ready",
			Role:       role,
			Room:       owner,
			PeerOnline: p.downloader != nil,
		}, false, nil
	}

	if p.downloader != nil {
		return nil, false, errRoomBusy
	}
	pub, err := h.svc.Public(context.Background(), token)
	if err != nil {
		return nil, false, err
	}
	p.downloader = c
	p.completed = false
	if p.owner != nil {
		if err := h.svc.MarkPaired(context.Background(), token); err != nil {
			h.logger.Warn("mark paired failed", "token", token, "error", err)
		}
	}
	return &ServerMessage{
		Type:       "ready",
		Role:       role,
		Room:       pub,
		PeerOnline: p.owner != nil,
	}, p.owner != nil, nil
}

// detach removes a connection and starts the owner grace timer when needed.
func (h *Hub) detach(token, role string, c *conn) {
	h.mu.Lock()
	p := h.rooms[token]
	if p == nil {
		h.mu.Unlock()
		return
	}

	if role == RoleOwner {
		if p.owner != c {
			h.mu.Unlock()
			return
		}
		p.owner = nil
		p.ownerLeft = &disconnect{}
		mark := p.ownerLeft
		if p.closeTimer != nil {
			p.closeTimer.Stop()
		}
		p.closeTimer = time.AfterFunc(h.grace, func() { h.closeIfOwnerGone(token, mark) })
		h.mu.Unlock()
		h.sendToOther(token, RoleOwner, ServerMessage{Type: "peer-left", Reason: "owner_reconnecting"})
		return
	}

	if p.downloader != c {
		h.mu.Unlock()
		return
	}
	p.downloader = nil
	completed := p.completed
	h.mu.Unlock()

	if !completed {
		if err := h.svc.MarkFailed(context.Background(), token, "downloader_left"); err != nil &&
			!errors.Is(err, room.ErrNotFound) {
			h.logger.Warn("mark failed", "token", token, "error", err)
		}
	}
	h.sendToOther(token, RoleDownloader, ServerMessage{Type: "peer-left", Reason: "downloader_left"})
}

func (h *Hub) closeIfOwnerGone(token string, mark *disconnect) {
	h.mu.Lock()
	p := h.rooms[token]
	if p == nil || p.owner != nil || p.ownerLeft != mark {
		h.mu.Unlock()
		return
	}
	owner, downloader := p.owner, p.downloader
	delete(h.rooms, token)
	p.closeTimer = nil
	h.mu.Unlock()

	if err := h.svc.CloseByToken(context.Background(), token); err != nil {
		h.logger.Warn("close room after owner left", "token", token, "error", err)
	}
	if downloader != nil {
		_ = downloader.write(ServerMessage{Type: "closed", Reason: "owner_left"})
		_ = downloader.ws.Close(closeNotFound, "owner_left")
	}
	if owner != nil {
		_ = owner.ws.Close(closeNotFound, "owner_left")
	}
}

// CloseRoom ends a room and disconnects both participants.
func (h *Hub) CloseRoom(token, reason string) {
	h.mu.Lock()
	p := h.rooms[token]
	if p == nil {
		h.mu.Unlock()
		return
	}
	delete(h.rooms, token)
	if p.closeTimer != nil {
		p.closeTimer.Stop()
	}
	owner, downloader := p.owner, p.downloader
	h.mu.Unlock()

	for _, c := range []*conn{owner, downloader} {
		if c == nil {
			continue
		}
		_ = c.write(ServerMessage{Type: "closed", Reason: reason})
		_ = c.ws.Close(closeNotFound, reason)
	}
}

// NotifyStats pushes the latest statistics and timeline to the owner.
func (h *Hub) NotifyStats(token string) {
	var owner *conn
	h.mu.Lock()
	if p := h.rooms[token]; p != nil {
		owner = p.owner
	}
	h.mu.Unlock()
	if owner == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	stats, events, err := h.svc.Stats(ctx, token)
	if err != nil {
		return
	}
	_ = owner.write(ServerMessage{Type: "stats", Stats: stats, Events: events})
}

func (h *Hub) markCompleted(token string) {
	h.mu.Lock()
	if p := h.rooms[token]; p != nil {
		p.completed = true
	}
	h.mu.Unlock()
}

func (h *Hub) sendToOther(token, from string, msg ServerMessage) {
	h.mu.Lock()
	p := h.rooms[token]
	var target *conn
	if p != nil {
		if from == RoleOwner {
			target = p.downloader
		} else {
			target = p.owner
		}
	}
	h.mu.Unlock()
	if target != nil {
		_ = target.write(msg)
	}
}
