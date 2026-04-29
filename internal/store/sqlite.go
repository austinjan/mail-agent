package store

import (
	"database/sql"
	_ "embed"
	"fmt"
	"os"

	"github.com/austinjan/mail-agent/internal/mail"
	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

// SqliteStore is the MVP Store backed by a local SQLite file
// and an on-disk attachments directory.
type SqliteStore struct {
	db            *sql.DB
	attachmentDir string
}

var _ Store = (*SqliteStore)(nil)

func OpenSQLite(dbPath, attachmentDir string) (*SqliteStore, error) {
	if err := os.MkdirAll(attachmentDir, 0o755); err != nil {
		return nil, fmt.Errorf("create attachment dir: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w", dbPath, err)
	}

	if _, err := db.Exec(schemaSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}

	return &SqliteStore{
		db:            db,
		attachmentDir: attachmentDir,
	}, nil
}

func (s *SqliteStore) Close() error {
	return s.db.Close()
}

func (s *SqliteStore) HasSeen(uidValidity uint32, uid uint32, folder string) (bool, error) {
	var one int
	err := s.db.QueryRow(
		`SELECT 1 FROM mails WHERE uid_validity = ? AND uid = ? AND folder = ? LIMIT 1`,
		uidValidity,
		uid,
		folder,
	).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("HasSeen: %w", err)
	}
	return true, nil
}

func (s *SqliteStore) HasSeenByMessageID(messageID string) (bool, error) {
	if messageID == "" {
		return false, nil
	}

	var one int
	err := s.db.QueryRow(
		`SELECT 1 FROM mails WHERE message_id = ? LIMIT 1`,
		messageID,
	).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("HasSeenByMessageID: %w", err)
	}
	return true, nil
}

// SaveMail is implemented in a later task.
func (s *SqliteStore) SaveMail(m mail.Mail) (int64, error) {
	return 0, fmt.Errorf("SaveMail: not implemented")
}

// SaveAttachment is implemented in a later task.
func (s *SqliteStore) SaveAttachment(mailID int64, a mail.Attachment) error {
	return fmt.Errorf("SaveAttachment: not implemented")
}
