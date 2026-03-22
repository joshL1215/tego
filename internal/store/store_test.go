package store

import (
	"path/filepath"
	"testing"
	"time"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := OpenAt(dbPath)
	if err != nil {
		t.Fatalf("OpenAt failed: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestStoreAndRetrieve(t *testing.T) {
	s := openTestStore(t)

	content := "line one\nline two\nline three"
	id, err := s.Store(content)
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}
	if len(id) != 12 {
		t.Errorf("expected 12-char hash, got %d chars: %q", len(id), id)
	}

	got, err := s.Retrieve(id)
	if err != nil {
		t.Fatalf("Retrieve failed: %v", err)
	}
	if got != content {
		t.Errorf("Retrieve returned %q, want %q", got, content)
	}
}

func TestStoreIdempotent(t *testing.T) {
	s := openTestStore(t)

	content := "same content"
	id1, err := s.Store(content)
	if err != nil {
		t.Fatalf("first Store failed: %v", err)
	}
	id2, err := s.Store(content)
	if err != nil {
		t.Fatalf("second Store failed: %v", err)
	}
	if id1 != id2 {
		t.Errorf("expected same hash for same content: %q != %q", id1, id2)
	}
}

func TestStoreDifferentContent(t *testing.T) {
	s := openTestStore(t)

	id1, _ := s.Store("content A")
	id2, _ := s.Store("content B")
	if id1 == id2 {
		t.Errorf("different content should produce different hashes")
	}
}

func TestRetrieveNotFound(t *testing.T) {
	s := openTestStore(t)

	_, err := s.Retrieve("nonexistent00")
	if err == nil {
		t.Fatal("expected error for nonexistent block")
	}
}

func TestCleanup(t *testing.T) {
	s := openTestStore(t)

	id, _ := s.Store("old content")

	// Insert a row with an old timestamp directly
	s.db.Exec(`UPDATE blocks SET created_at = ? WHERE hash = ?`,
		time.Now().Add(-48*time.Hour).Unix(), id)

	s.Cleanup(24 * time.Hour)

	_, err := s.Retrieve(id)
	if err == nil {
		t.Error("expected old block to be cleaned up")
	}
}

func TestCleanupKeepsRecent(t *testing.T) {
	s := openTestStore(t)

	id, _ := s.Store("recent content")
	s.Cleanup(24 * time.Hour)

	got, err := s.Retrieve(id)
	if err != nil {
		t.Fatalf("recent block should not be cleaned up: %v", err)
	}
	if got != "recent content" {
		t.Errorf("got %q, want %q", got, "recent content")
	}
}

func TestContentHash(t *testing.T) {
	h1 := contentHash("hello")
	h2 := contentHash("hello")
	h3 := contentHash("world")

	if h1 != h2 {
		t.Error("same input should produce same hash")
	}
	if h1 == h3 {
		t.Error("different input should produce different hash")
	}
	if len(h1) != 12 {
		t.Errorf("expected 12-char hash, got %d", len(h1))
	}
}
