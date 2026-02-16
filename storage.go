package whitelistplus

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

// IP approval statuses.
const (
	StatusUnknown  = ""
	StatusPending  = "pending"
	StatusApproved = "approved"
	StatusDenied   = "denied"
)

// Store wraps a SQLite database for IP whitelist management.
type Store struct {
	db *sql.DB
}

// NewStore opens (or creates) a SQLite database at dbPath and
// ensures the schema exists.
func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	// Enable WAL mode and set a busy timeout for concurrent access.
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, err
		}
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS whitelist (
			ip            TEXT PRIMARY KEY,
			status        TEXT NOT NULL DEFAULT 'pending',
			requested_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			approved_at   DATETIME,
			host          TEXT,
			path          TEXT,
			user_agent    TEXT
		)
	`)
	if err != nil {
		db.Close()
		return nil, err
	}

	// Add user_agent column if it doesn't exist (migration for existing databases)
	_, _ = db.Exec(`ALTER TABLE whitelist ADD COLUMN user_agent TEXT`)

	return &Store{db: db}, nil
}

// GetIPStatus returns the approval status of an IP address.
// Returns StatusUnknown if the IP is not in the database.
func (s *Store) GetIPStatus(ip string) (string, error) {
	var status string
	err := s.db.QueryRow("SELECT status FROM whitelist WHERE ip = ?", ip).Scan(&status)
	if err == sql.ErrNoRows {
		return StatusUnknown, nil
	}
	return status, err
}

// AddIP inserts a new IP with the given status. Does nothing if
// the IP already exists (INSERT OR IGNORE).
func (s *Store) AddIP(ip, status, host, path, userAgent string) error {
	_, err := s.db.Exec(
		"INSERT OR IGNORE INTO whitelist (ip, status, requested_at, host, path, user_agent) VALUES (?, ?, ?, ?, ?, ?)",
		ip, status, time.Now().UTC(), host, path, userAgent,
	)
	return err
}

// UpdateStatus changes the status of an existing IP.
// If the new status is "approved", approved_at is set to now.
func (s *Store) UpdateStatus(ip, status string) error {
	var approvedAt interface{}
	if status == StatusApproved {
		approvedAt = time.Now().UTC()
	}
	_, err := s.db.Exec(
		"UPDATE whitelist SET status = ?, approved_at = ? WHERE ip = ?",
		status, approvedAt, ip,
	)
	return err
}

// Close closes the database connection.
func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
