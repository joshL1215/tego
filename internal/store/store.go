package store

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Store is a SQLite-backed content-addressable store for deduplicated text blocks.
type Store struct {
	db *sql.DB
}

// Open creates or opens the dedup database at ~/.local/share/tego/dedup.db
// and runs TTL cleanup on startup.
func Open() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}

	dir := filepath.Join(home, ".local", "share", "tego")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("cannot create data directory: %w", err)
	}

	return OpenAt(filepath.Join(dir, "dedup.db"))
}

// OpenAt creates or opens the dedup database at the given path
// and runs TTL cleanup on startup.
func OpenAt(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open database: %w", err)
	}

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS blocks (
		hash       TEXT PRIMARY KEY,
		content    TEXT NOT NULL,
		created_at INTEGER NOT NULL
	)`); err != nil {
		db.Close()
		return nil, fmt.Errorf("cannot create table: %w", err)
	}

	s := &Store{db: db}
	s.Cleanup(24 * time.Hour)
	return s, nil
}

// Store saves content and returns a 12-character hex hash ID.
// If the content already exists, it returns the existing ID without re-inserting.
func (s *Store) Store(content string) (string, error) {
	hash := contentHash(content)
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO blocks (hash, content, created_at) VALUES (?, ?, ?)`,
		hash, content, time.Now().Unix(),
	)
	if err != nil {
		return "", fmt.Errorf("cannot store block: %w", err)
	}
	return hash, nil
}

// Retrieve looks up a block by its hash ID and returns the content.
func (s *Store) Retrieve(id string) (string, error) {
	var content string
	err := s.db.QueryRow(`SELECT content FROM blocks WHERE hash = ?`, id).Scan(&content)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("block %s not found", id)
	}
	if err != nil {
		return "", fmt.Errorf("cannot retrieve block: %w", err)
	}
	return content, nil
}

// Cleanup deletes blocks older than maxAge.
func (s *Store) Cleanup(maxAge time.Duration) {
	cutoff := time.Now().Add(-maxAge).Unix()
	s.db.Exec(`DELETE FROM blocks WHERE created_at < ?`, cutoff)
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// contentHash returns a 12-character hex SHA256 hash of the content.
func contentHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h[:6])
}
