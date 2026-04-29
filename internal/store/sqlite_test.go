package store

import (
	"path/filepath"
	"testing"
)

func TestOpenCreatesSchema(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	attDir := filepath.Join(dir, "attachments")

	s, err := OpenSQLite(dbPath, attDir)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer s.Close()

	s2, err := OpenSQLite(dbPath, attDir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()
}

func TestHasSeenEmpty(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenSQLite(filepath.Join(dir, "test.db"), filepath.Join(dir, "attachments"))
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer s.Close()

	seen, err := s.HasSeen(1, 42, "INBOX")
	if err != nil {
		t.Fatalf("HasSeen: %v", err)
	}
	if seen {
		t.Error("empty DB should report HasSeen = false")
	}

	seen, err = s.HasSeenByMessageID("<nope@example.com>")
	if err != nil {
		t.Fatalf("HasSeenByMessageID: %v", err)
	}
	if seen {
		t.Error("empty DB should report HasSeenByMessageID = false")
	}
}
