package store

import (
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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

func (s *SqliteStore) SaveMail(m mail.Mail) (int64, error) {
	to, err := json.Marshal(defaultSlice(m.ToAddrs))
	if err != nil {
		return 0, fmt.Errorf("marshal to_addrs: %w", err)
	}
	cc, err := json.Marshal(defaultSlice(m.CCAddrs))
	if err != nil {
		return 0, fmt.Errorf("marshal cc_addrs: %w", err)
	}
	refs, err := json.Marshal(defaultSlice(m.Refs))
	if err != nil {
		return 0, fmt.Errorf("marshal refs: %w", err)
	}
	flags, err := json.Marshal(defaultSlice(m.Flags))
	if err != nil {
		return 0, fmt.Errorf("marshal flags: %w", err)
	}

	res, err := s.db.Exec(`
INSERT INTO mails (
    uid_validity, uid, folder, message_id,
    subject, from_addr, to_addrs, cc_addrs,
    reply_to, in_reply_to, refs, flags,
    received_at, body_text, body_html, raw_headers,
    fetched_at
) VALUES (?,?,?,?, ?,?,?,?, ?,?,?,?, ?,?,?,?, ?)`,
		m.UIDValidity, m.UID, m.Folder, nullableString(m.MessageID),
		m.Subject, m.From, string(to), string(cc),
		m.ReplyTo, m.InReplyTo, string(refs), string(flags),
		m.ReceivedAt.UTC(), m.BodyText, m.BodyHTML, m.RawHeaders,
		time.Now().UTC(),
	)
	if err != nil {
		if isUniqueConstraint(err) {
			return 0, ErrAlreadyExists
		}
		return 0, fmt.Errorf("SaveMail insert: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("SaveMail lastID: %w", err)
	}
	return id, nil
}

func (s *SqliteStore) SaveAttachment(mailID int64, a mail.Attachment) error {
	sum := sha256.Sum256(a.Content)
	sumHex := hex.EncodeToString(sum[:])
	prefix := sumHex[:2]
	relPath := filepath.ToSlash(filepath.Join(prefix, sumHex))

	prefixDir := filepath.Join(s.attachmentDir, prefix)
	if err := os.MkdirAll(prefixDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %q: %w", prefixDir, err)
	}

	finalPath := filepath.Join(prefixDir, sumHex)
	if _, err := os.Stat(finalPath); os.IsNotExist(err) {
		tmp, err := os.CreateTemp(prefixDir, "att-*.tmp")
		if err != nil {
			return fmt.Errorf("create tmp: %w", err)
		}
		if _, err := tmp.Write(a.Content); err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
			return fmt.Errorf("write tmp: %w", err)
		}
		if err := tmp.Close(); err != nil {
			_ = os.Remove(tmp.Name())
			return fmt.Errorf("close tmp: %w", err)
		}
		if err := os.Rename(tmp.Name(), finalPath); err != nil {
			_ = os.Remove(tmp.Name())
			return fmt.Errorf("rename tmp: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("stat %q: %w", finalPath, err)
	}

	_, err := s.db.Exec(
		`INSERT INTO attachments (mail_id, filename, content_type, size_bytes, sha256, file_path)
		 VALUES (?,?,?,?,?,?)`,
		mailID, a.Filename, a.ContentType, len(a.Content), sumHex, relPath,
	)
	if err != nil {
		return fmt.Errorf("insert attachment row: %w", err)
	}
	return nil
}

func defaultSlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func isUniqueConstraint(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
