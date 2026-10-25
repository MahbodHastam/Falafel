CREATE TABLE IF NOT EXISTS rooms (
    token             TEXT PRIMARY KEY,
    owner_secret_hash TEXT NOT NULL,
    file_name         TEXT NOT NULL,
    file_size         INTEGER NOT NULL,
    mime_type         TEXT NOT NULL,
    password_hash     TEXT,
    status            TEXT NOT NULL,
    failed_attempts   INTEGER NOT NULL DEFAULT 0,
    bytes_received    INTEGER NOT NULL DEFAULT 0,
    created_at        INTEGER NOT NULL,
    connected_at      INTEGER,
    completed_at      INTEGER,
    closed_at         INTEGER,
    expires_at        INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_rooms_expires_at ON rooms (expires_at);
CREATE INDEX IF NOT EXISTS idx_rooms_status ON rooms (status);

CREATE TABLE IF NOT EXISTS room_events (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    token      TEXT NOT NULL REFERENCES rooms (token) ON DELETE CASCADE,
    type       TEXT NOT NULL,
    meta       TEXT,
    created_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_room_events_token ON room_events (token, id);

CREATE TABLE IF NOT EXISTS meta (
    key   TEXT PRIMARY KEY,
    value BLOB NOT NULL
);
