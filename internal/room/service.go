package room

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// Options configures the room service.
type Options struct {
	TTL               time.Duration
	MaxFileSize       int64
	MinPasswordLength int
	TicketTTL         time.Duration
}

// Service implements room creation, access control and lifecycle.
type Service struct {
	store *Store
	key   []byte
	opts  Options
	now   func() time.Time
}

// NewService builds a service and defaults the ticket lifetime to two minutes.
func NewService(store *Store, signingKey []byte, opts Options) *Service {
	if opts.TicketTTL <= 0 {
		opts.TicketTTL = 2 * time.Minute
	}
	return &Service{store: store, key: signingKey, opts: opts, now: time.Now}
}

// SetClock overrides the clock (tests only).
func (s *Service) SetClock(now func() time.Time) { s.now = now }

// Create validates input and persists a new room.
func (s *Service) Create(ctx context.Context, in CreateInput) (*CreatedRoom, error) {
	fileName, mimeType, passwordHash, err := s.validateCreate(in)
	if err != nil {
		return nil, err
	}

	token, err := randomToken(16)
	if err != nil {
		return nil, err
	}
	ownerSecret, err := randomToken(32)
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	r := &Room{
		Token:           token,
		OwnerSecretHash: hashSecret(ownerSecret),
		FileName:        fileName,
		FileSize:        in.FileSize,
		MimeType:        mimeType,
		PasswordHash:    passwordHash,
		Status:          StatusWaiting,
		CreatedAt:       now,
		ExpiresAt:       now.Add(s.opts.TTL),
	}
	if err := s.store.Create(ctx, r); err != nil {
		return nil, err
	}
	_ = s.store.AddEvent(ctx, token, EventCreated, nil, now)

	return &CreatedRoom{Room: r, OwnerSecret: ownerSecret}, nil
}

// validateCreate checks the request and returns the normalized name, MIME type and password hash.
func (s *Service) validateCreate(in CreateInput) (fileName, mimeType, passwordHash string, err error) {
	fileName = strings.TrimSpace(in.FileName)
	if fileName == "" {
		return "", "", "", &ValidationError{Field: "fileName", Reason: "required"}
	}
	if utf8.RuneCountInString(fileName) > 255 {
		return "", "", "", &ValidationError{Field: "fileName", Reason: "too_long"}
	}
	for _, r := range fileName {
		if r < 0x20 || r == 0x7f {
			return "", "", "", &ValidationError{Field: "fileName", Reason: "control_characters"}
		}
	}
	if in.FileSize <= 0 {
		return "", "", "", &ValidationError{Field: "fileSize", Reason: "invalid"}
	}
	if in.FileSize > s.opts.MaxFileSize {
		return "", "", "", &ValidationError{Field: "fileSize", Reason: "too_large"}
	}
	mimeType = strings.TrimSpace(in.MimeType)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	if len(mimeType) > 120 || strings.ContainsAny(mimeType, "\r\n") {
		return "", "", "", &ValidationError{Field: "mimeType", Reason: "invalid"}
	}

	if in.Password != "" {
		length := utf8.RuneCountInString(in.Password)
		if length < s.opts.MinPasswordLength {
			return "", "", "", &ValidationError{Field: "password", Reason: "too_short"}
		}
		if length > 128 {
			return "", "", "", &ValidationError{Field: "password", Reason: "too_long"}
		}
		passwordHash, err = hashPassword(in.Password)
		if err != nil {
			return "", "", "", err
		}
	}
	return fileName, mimeType, passwordHash, nil
}

// Public returns downloader-safe metadata, hiding dead or used rooms.
func (s *Service) Public(ctx context.Context, token string) (*PublicRoom, error) {
	r, err := s.loadLive(ctx, token)
	if err != nil {
		return nil, err
	}
	pub := r.Public()
	return &pub, nil
}

// Join verifies the password (when required) and mints a signaling ticket.
func (s *Service) Join(ctx context.Context, token, password string) (*Ticket, error) {
	r, err := s.loadLive(ctx, token)
	if err != nil {
		return nil, err
	}
	if r.PasswordHash != "" && !verifyPassword(password, r.PasswordHash) {
		attempts := r.FailedAttempts + 1
		if err := s.store.SetFailedAttempts(ctx, token, attempts); err != nil {
			return nil, err
		}
		_ = s.store.AddEvent(ctx, token, EventWrongPassword, nil, s.now().UTC())
		return nil, ErrWrongPassword
	}
	value, expires, err := mintTicket(s.key, token, s.opts.TicketTTL, s.now())
	if err != nil {
		return nil, err
	}
	return &Ticket{Token: value, ExpiresAt: expires.UTC()}, nil
}

// ValidateTicket checks a signaling ticket and returns its room token.
func (s *Service) ValidateTicket(ctx context.Context, value string) (string, error) {
	token, err := parseTicket(s.key, value, s.now())
	if err != nil {
		return "", err
	}
	if _, err := s.loadLive(ctx, token); err != nil {
		return "", ErrInvalidTicket
	}
	return token, nil
}

// Owner returns the full room snapshot after verifying the owner secret.
func (s *Service) Owner(ctx context.Context, token, ownerSecret string) (*OwnerRoom, error) {
	r, err := s.store.Get(ctx, token)
	if err != nil {
		return nil, err
	}
	if hashSecret(ownerSecret) != r.OwnerSecretHash {
		return nil, ErrUnauthorized
	}
	events, err := s.store.Events(ctx, token, 50)
	if err != nil {
		return nil, err
	}
	return &OwnerRoom{
		PublicRoom:     r.Public(),
		BytesReceived:  r.BytesReceived,
		FailedAttempts: r.FailedAttempts,
		CreatedAt:      r.CreatedAt,
		ConnectedAt:    r.ConnectedAt,
		CompletedAt:    r.CompletedAt,
		ClosedAt:       r.ClosedAt,
		Events:         events,
	}, nil
}

// IsOwner reports whether the secret belongs to the room.
func (s *Service) IsOwner(ctx context.Context, token, ownerSecret string) (bool, error) {
	r, err := s.store.Get(ctx, token)
	if err != nil {
		return false, err
	}
	return hashSecret(ownerSecret) == r.OwnerSecretHash, nil
}

// Close ends a room on behalf of its owner.
func (s *Service) Close(ctx context.Context, token, ownerSecret string) error {
	r, err := s.store.Get(ctx, token)
	if err != nil {
		return err
	}
	if hashSecret(ownerSecret) != r.OwnerSecretHash {
		return ErrUnauthorized
	}
	if r.Status == StatusClosed || r.Status == StatusExpired || r.Status == StatusCompleted {
		return nil
	}
	now := s.now().UTC()
	if err := s.store.UpdateStatus(ctx, token, StatusClosed, now); err != nil {
		return err
	}
	return s.store.AddEvent(ctx, token, EventClosed, nil, now)
}

// CloseByToken marks a room closed unless it already reached a terminal state.
func (s *Service) CloseByToken(ctx context.Context, token string) error {
	r, err := s.store.Get(ctx, token)
	if errors.Is(err, ErrNotFound) || (r != nil && (r.Status == StatusClosed || r.Status == StatusExpired || r.Status == StatusCompleted)) {
		return nil
	}
	if err != nil {
		return err
	}
	now := s.now().UTC()
	if err := s.store.UpdateStatus(ctx, token, StatusClosed, now); err != nil {
		return err
	}
	return s.store.AddEvent(ctx, token, EventClosed, nil, now)
}

// MarkPaired records that an owner and downloader are connected.
func (s *Service) MarkPaired(ctx context.Context, token string) error {
	now := s.now().UTC()
	if err := s.store.UpdateStatus(ctx, token, StatusPaired, now); err != nil {
		return err
	}
	return s.store.AddEvent(ctx, token, EventDownloaderJoined, nil, now)
}

// MarkTransferring records the start of a transfer exactly once.
func (s *Service) MarkTransferring(ctx context.Context, token string) error {
	r, err := s.store.Get(ctx, token)
	if err != nil {
		return err
	}
	if r.Status == StatusTransferring {
		return nil
	}
	now := s.now().UTC()
	if err := s.store.UpdateStatus(ctx, token, StatusTransferring, now); err != nil {
		return err
	}
	return s.store.AddEvent(ctx, token, EventTransferStarted, nil, now)
}

// SetProgress stores acknowledged bytes and clamps negatives to zero.
func (s *Service) SetProgress(ctx context.Context, token string, bytes int64) error {
	if bytes < 0 {
		bytes = 0
	}
	return s.store.SetBytes(ctx, token, bytes)
}

// MarkCompleted persists a successful transfer.
func (s *Service) MarkCompleted(ctx context.Context, token string, bytes int64) error {
	r, err := s.store.Get(ctx, token)
	if err != nil {
		return err
	}
	if r.Status == StatusCompleted {
		return nil
	}
	now := s.now().UTC()
	if err := s.store.Complete(ctx, token, bytes, now); err != nil {
		return err
	}
	return s.store.AddEvent(ctx, token, EventTransferCompleted, nil, now)
}

// MarkFailed marks a dropped transfer as failed unless the room already ended.
func (s *Service) MarkFailed(ctx context.Context, token string, reason string) error {
	r, err := s.store.Get(ctx, token)
	if err != nil {
		return err
	}
	if r.Status == StatusCompleted || r.Status == StatusClosed || r.Status == StatusExpired {
		return nil
	}
	now := s.now().UTC()
	if err := s.store.UpdateStatus(ctx, token, StatusFailed, now); err != nil {
		return err
	}
	return s.store.AddEvent(ctx, token, EventTransferFailed, map[string]any{"reason": reason}, now)
}

// Stats builds the compact statistics block for the owner socket.
func (s *Service) Stats(ctx context.Context, token string) (*Stats, []Event, error) {
	r, err := s.store.Get(ctx, token)
	if err != nil {
		return nil, nil, err
	}
	events, err := s.store.Events(ctx, token, 50)
	if err != nil {
		return nil, nil, err
	}
	return &Stats{
		Status:         r.Status,
		BytesReceived:  r.BytesReceived,
		FailedAttempts: r.FailedAttempts,
		ConnectedAt:    r.ConnectedAt,
		CompletedAt:    r.CompletedAt,
		ClosedAt:       r.ClosedAt,
	}, events, nil
}

// ExpireStale closes unfinished rooms past their expiry.
func (s *Service) ExpireStale(ctx context.Context) ([]string, error) {
	return s.store.ExpireStale(ctx, s.now().UTC())
}

// Purge deletes old terminal rooms.
func (s *Service) PurgeBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	return s.store.Purge(ctx, cutoff)
}

// TTL exposes the configured room lifetime.
func (s *Service) TTL() time.Duration { return s.opts.TTL }

// loadLive returns a room only while it can still be joined.
func (s *Service) loadLive(ctx context.Context, token string) (*Room, error) {
	r, err := s.store.Get(ctx, token)
	if err != nil {
		return nil, err
	}
	switch r.Status {
	case StatusCompleted, StatusClosed, StatusExpired:
		return nil, ErrNotFound
	}
	if s.now().After(r.ExpiresAt) {
		_ = s.store.UpdateStatus(ctx, token, StatusExpired, s.now().UTC())
		_ = s.store.AddEvent(ctx, token, EventExpired, nil, s.now().UTC())
		return nil, ErrNotFound
	}
	return r, nil
}

// randomToken returns a URL-safe random string of size bytes.
func randomToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// hashSecret stores owner secrets as SHA-256 hex.
func hashSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}
