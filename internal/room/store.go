package room

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"
)

// Store persists rooms and their event timeline.
type Store struct {
	db *sql.DB
}

// NewStore wraps a database handle.
func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// roomColumns keeps the SELECT and INSERT column order in sync.
const roomColumns = `token, owner_secret_hash, file_name, file_size, mime_type, password_hash,
	status, failed_attempts, bytes_received, created_at, connected_at, completed_at, closed_at, expires_at`

// Create inserts a new room using the initial state set by the service.
func (s *Store) Create(ctx context.Context, r *Room) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO rooms (`+roomColumns+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.Token, r.OwnerSecretHash, r.FileName, r.FileSize, r.MimeType, r.PasswordHash,
		string(r.Status), r.FailedAttempts, r.BytesReceived,
		r.CreatedAt.UnixMilli(), unixPtr(r.ConnectedAt), unixPtr(r.CompletedAt), unixPtr(r.ClosedAt), r.ExpiresAt.UnixMilli(),
	)
	if err != nil {
		return fmt.Errorf("insert room: %w", err)
	}
	return nil
}

// Get loads one room by token and maps no rows to ErrNotFound.
func (s *Store) Get(ctx context.Context, token string) (*Room, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+roomColumns+` FROM rooms WHERE token = ?`, token)
	r, err := scanRoom(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load room: %w", err)
	}
	return r, nil
}

// UpdateStatus moves a room to a new status, stamping connection or closure time.
func (s *Store) UpdateStatus(ctx context.Context, token string, status Status, at time.Time) error {
	// Each status either stamps one timestamp column or only changes the status.
	const (
		statusSQL = `UPDATE rooms SET status = ? WHERE token = ?`
		pairedSQL = `UPDATE rooms SET status = ?, connected_at = COALESCE(connected_at, ?) WHERE token = ?`
		closedSQL = `UPDATE rooms SET status = ?, closed_at = COALESCE(closed_at, ?) WHERE token = ?`
	)

	var (
		query string
		args  []any
	)
	switch status {
	case StatusPaired:
		query, args = pairedSQL, []any{string(status), at.UnixMilli(), token}
	case StatusClosed, StatusExpired:
		query, args = closedSQL, []any{string(status), at.UnixMilli(), token}
	default:
		query, args = statusSQL, []any{string(status), token}
	}
	if _, err := s.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("update room status: %w", err)
	}
	return nil
}

// SetBytes records acknowledged bytes for the owner dashboard.
func (s *Store) SetBytes(ctx context.Context, token string, bytes int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE rooms SET bytes_received = ? WHERE token = ?`, bytes, token)
	if err != nil {
		return fmt.Errorf("update bytes: %w", err)
	}
	return nil
}

// IncrementFailedAttempts bumps the wrong-password counter shown to the owner.
// The increment happens in SQL so concurrent failures are not lost.
func (s *Store) IncrementFailedAttempts(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE rooms SET failed_attempts = failed_attempts + 1 WHERE token = ?`, token)
	if err != nil {
		return fmt.Errorf("update failed attempts: %w", err)
	}
	return nil
}

// Complete records a finished transfer and its final byte count.
func (s *Store) Complete(ctx context.Context, token string, bytes int64, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE rooms SET status = ?, bytes_received = ?, completed_at = COALESCE(completed_at, ?)
		WHERE token = ?`, string(StatusCompleted), bytes, at.UnixMilli(), token)
	if err != nil {
		return fmt.Errorf("complete room: %w", err)
	}
	return nil
}

// AddEvent appends an entry to the room timeline.
func (s *Store) AddEvent(ctx context.Context, token, eventType string, meta map[string]any, at time.Time) error {
	var raw []byte
	if len(meta) > 0 {
		var err error
		raw, err = json.Marshal(meta)
		if err != nil {
			return fmt.Errorf("encode event meta: %w", err)
		}
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO room_events (token, type, meta, created_at) VALUES (?, ?, ?, ?)`,
		token, eventType, raw, at.UnixMilli())
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}
	return nil
}

// Events returns up to limit events for a room, oldest first.
func (s *Store) Events(ctx context.Context, token string, limit int) ([]Event, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT type, meta, created_at FROM room_events
		WHERE token = ? ORDER BY id DESC LIMIT ?`, token, limit)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var (
			eventType string
			raw       []byte
			millis    int64
		)
		if err := rows.Scan(&eventType, &raw, &millis); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		event := Event{Type: eventType, CreatedAt: time.UnixMilli(millis).UTC()}
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &event.Meta); err != nil {
				event.Meta = nil
			}
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate events: %w", err)
	}
	slices.Reverse(events)
	return events, nil
}

// ExpireStale marks unfinished rooms past their expiry and returns their tokens.
func (s *Store) ExpireStale(ctx context.Context, now time.Time) ([]string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin expire transaction: %w", err)
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `
		SELECT token FROM rooms
		WHERE expires_at < ? AND status NOT IN ('completed', 'closed', 'expired')`,
		now.UnixMilli())
	if err != nil {
		return nil, fmt.Errorf("select stale rooms: %w", err)
	}
	var tokens []string
	for rows.Next() {
		var token string
		if err := rows.Scan(&token); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan stale room: %w", err)
		}
		tokens = append(tokens, token)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate stale rooms: %w", err)
	}

	for _, token := range tokens {
		if _, err := tx.ExecContext(ctx, `
			UPDATE rooms SET status = 'expired', closed_at = ? WHERE token = ?`,
			now.UnixMilli(), token); err != nil {
			return nil, fmt.Errorf("expire room: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO room_events (token, type, created_at) VALUES (?, 'expired', ?)`,
			token, now.UnixMilli()); err != nil {
			return nil, fmt.Errorf("insert expiry event: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit expiry: %w", err)
	}
	return tokens, nil
}

// Purge removes closed, completed and expired rooms older than cutoff.
func (s *Store) Purge(ctx context.Context, cutoff time.Time) (int64, error) {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM rooms
		WHERE expires_at < ? AND status IN ('completed', 'closed', 'expired')`,
		cutoff.UnixMilli())
	if err != nil {
		return 0, fmt.Errorf("purge rooms: %w", err)
	}
	n, _ := result.RowsAffected()
	return n, nil
}

// scanner lets scanRoom accept both a single row and a rows cursor.
type scanner interface {
	Scan(dest ...any) error
}

// scanRoom reads a row in roomColumns order into a Room.
func scanRoom(row scanner) (*Room, error) {
	var (
		r                   Room
		status              string
		created, expires    int64
		connected, completd sql.NullInt64
		closed              sql.NullInt64
	)
	err := row.Scan(
		&r.Token, &r.OwnerSecretHash, &r.FileName, &r.FileSize, &r.MimeType, &r.PasswordHash,
		&status, &r.FailedAttempts, &r.BytesReceived, &created, &connected, &completd, &closed, &expires,
	)
	if err != nil {
		return nil, err
	}
	r.Status = Status(status)
	r.CreatedAt = time.UnixMilli(created).UTC()
	r.ExpiresAt = time.UnixMilli(expires).UTC()
	r.ConnectedAt = timeFromNull(connected)
	r.CompletedAt = timeFromNull(completd)
	r.ClosedAt = timeFromNull(closed)
	return &r, nil
}

// unixPtr converts an optional time to the nullable integer stored in SQLite.
func unixPtr(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UnixMilli()
}

// timeFromNull converts a nullable SQLite millisecond value back to UTC time.
func timeFromNull(v sql.NullInt64) *time.Time {
	if !v.Valid {
		return nil
	}
	t := time.UnixMilli(v.Int64).UTC()
	return &t
}
