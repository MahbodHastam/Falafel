// Package room contains Falafel's room domain: lifecycle, metadata, passwords
// and owner statistics. File bytes never pass through this package.
package room

import (
	"errors"
	"fmt"
	"time"
)

// Status is the lifecycle state of a room.
type Status string

const (
	StatusWaiting      Status = "waiting"
	StatusPaired       Status = "paired"
	StatusTransferring Status = "transferring"
	StatusCompleted    Status = "completed"
	StatusFailed       Status = "failed"
	StatusClosed       Status = "closed"
	StatusExpired      Status = "expired"
)

// Common errors returned by the service.
var (
	ErrNotFound      = errors.New("room not found")
	ErrWrongPassword = errors.New("wrong password")
	ErrUnauthorized  = errors.New("invalid owner secret")
	ErrRoomClosed    = errors.New("room closed")
	ErrRoomExpired   = errors.New("room expired")
	ErrRoomCompleted = errors.New("room already completed")
	ErrInvalidTicket = errors.New("invalid download ticket")
	ErrInvalidInput  = errors.New("invalid input")
)

// ValidationError describes a rejected input field.
type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("invalid %s: %s", e.Field, e.Reason)
}

func (e *ValidationError) Unwrap() error { return ErrInvalidInput }

// Room is the persisted room record.
type Room struct {
	Token           string
	OwnerSecretHash string
	FileName        string
	FileSize        int64
	MimeType        string
	PasswordHash    string
	Status          Status
	FailedAttempts  int
	BytesReceived   int64
	CreatedAt       time.Time
	ConnectedAt     *time.Time
	CompletedAt     *time.Time
	ClosedAt        *time.Time
	ExpiresAt       time.Time
}

// Public returns the downloader-safe metadata for the room.
func (r *Room) Public() PublicRoom {
	return PublicRoom{
		Token:            r.Token,
		FileName:         r.FileName,
		FileSize:         r.FileSize,
		MimeType:         r.MimeType,
		PasswordRequired: r.PasswordHash != "",
		Status:           r.Status,
		ExpiresAt:        r.ExpiresAt,
	}
}

// PublicRoom is the metadata exposed to a downloader.
type PublicRoom struct {
	Token            string    `json:"token"`
	FileName         string    `json:"fileName"`
	FileSize         int64     `json:"fileSize"`
	MimeType         string    `json:"mimeType"`
	PasswordRequired bool      `json:"passwordRequired"`
	Status           Status    `json:"status"`
	ExpiresAt        time.Time `json:"expiresAt"`
}

// OwnerRoom is the metadata plus statistics exposed to the uploader.
type OwnerRoom struct {
	PublicRoom
	BytesReceived  int64      `json:"bytesReceived"`
	FailedAttempts int        `json:"failedAttempts"`
	CreatedAt      time.Time  `json:"createdAt"`
	ConnectedAt    *time.Time `json:"connectedAt"`
	CompletedAt    *time.Time `json:"completedAt"`
	ClosedAt       *time.Time `json:"closedAt"`
	Events         []Event    `json:"events"`
}

// Event is an append-only entry in the owner's activity timeline.
type Event struct {
	Type      string         `json:"type"`
	Meta      map[string]any `json:"meta,omitempty"`
	CreatedAt time.Time      `json:"createdAt"`
}

// Event type constants.
const (
	EventCreated           = "created"
	EventDownloaderJoined  = "downloader_joined"
	EventWrongPassword     = "wrong_password"
	EventTransferStarted   = "transfer_started"
	EventTransferCompleted = "transfer_completed"
	EventTransferFailed    = "transfer_failed"
	EventClosed            = "closed"
	EventExpired           = "expired"
)

// CreateInput is the validated input for a new room.
type CreateInput struct {
	FileName string
	FileSize int64
	MimeType string
	Password string
}

// CreatedRoom is returned once, at creation time.
type CreatedRoom struct {
	Room        *Room
	URL         string
	OwnerSecret string
}

// Ticket grants a downloader access to the signaling socket.
type Ticket struct {
	Token     string
	ExpiresAt time.Time
}

// Stats is the compact statistics block pushed to the owner socket.
type Stats struct {
	Status         Status     `json:"status"`
	BytesReceived  int64      `json:"bytesReceived"`
	FailedAttempts int        `json:"failedAttempts"`
	ConnectedAt    *time.Time `json:"connectedAt"`
	CompletedAt    *time.Time `json:"completedAt"`
	ClosedAt       *time.Time `json:"closedAt"`
}
