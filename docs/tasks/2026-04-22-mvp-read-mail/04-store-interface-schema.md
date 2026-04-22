# Task 04 — Store 介面 + SQLite schema

**目標**：定義 `Store` 介面、建立 SQLite 檔案、套 schema。此 task 還不寫入實際的 mail 資料；只確保 schema 正確、`SqliteStore` 可以 open / close。

**依賴**：Task 02（需要 `mail.Mail`）。

## 產出檔案

- Create: `internal/store/store.go`（介面）
- Create: `internal/store/schema.sql`
- Create: `internal/store/sqlite.go`（`Open`、`Close`、套 schema）
- Create: `internal/store/sqlite_test.go`

## Steps

- [ ] **Step 1: 加入 SQLite 依賴**

```bash
go get modernc.org/sqlite
```

- [ ] **Step 2: 定義介面 `internal/store/store.go`**

```go
// Package store persists fetched mails and attachments.
// The Store interface decouples the core pipeline from any
// particular backend — MVP uses SqliteStore.
package store

import "github.com/austinjan/mail-agent/internal/mail"

type Store interface {
	SaveMail(m mail.Mail) (mailID int64, err error)
	HasSeen(uidValidity uint32, uid uint32, folder string) (bool, error)
	HasSeenByMessageID(messageID string) (bool, error)
	SaveAttachment(mailID int64, a mail.Attachment) error
	Close() error
}
```

- [ ] **Step 3: 寫 schema `internal/store/schema.sql`**

```sql
CREATE TABLE IF NOT EXISTS mails (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    uid_validity  INTEGER NOT NULL,
    uid           INTEGER NOT NULL,
    folder        TEXT NOT NULL,
    message_id    TEXT,
    subject       TEXT,
    from_addr     TEXT,
    to_addrs      TEXT,
    cc_addrs      TEXT,
    reply_to      TEXT,
    in_reply_to   TEXT,
    refs          TEXT,
    flags         TEXT,
    received_at   TIMESTAMP,
    body_text     TEXT,
    body_html     TEXT,
    raw_headers   TEXT,
    fetched_at    TIMESTAMP NOT NULL,
    UNIQUE (uid_validity, uid, folder)
);

CREATE INDEX IF NOT EXISTS idx_mails_message_id ON mails(message_id);

CREATE TABLE IF NOT EXISTS attachments (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    mail_id       INTEGER NOT NULL REFERENCES mails(id),
    filename      TEXT,
    content_type  TEXT,
    size_bytes    INTEGER,
    sha256        TEXT NOT NULL,
    file_path     TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_attachments_mail_id ON attachments(mail_id);
CREATE INDEX IF NOT EXISTS idx_attachments_sha256 ON attachments(sha256);
```

- [ ] **Step 4: 寫失敗測試**

`internal/store/sqlite_test.go`：

```go
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

	// Re-open should be idempotent (schema uses IF NOT EXISTS).
	s2, err := OpenSQLite(dbPath, attDir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	s2.Close()
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
```

- [ ] **Step 5: 跑測試確認失敗**

```bash
go test ./internal/store/...
```

預期：編譯錯誤（`OpenSQLite` 未定義）。

- [ ] **Step 6: 實作 `internal/store/sqlite.go`**

```go
package store

import (
	"database/sql"
	_ "embed"
	"fmt"
	"os"

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

func OpenSQLite(dbPath, attachmentDir string) (*SqliteStore, error) {
	if err := os.MkdirAll(attachmentDir, 0o755); err != nil {
		return nil, fmt.Errorf("create attachment dir: %w", err)
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w", dbPath, err)
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &SqliteStore{db: db, attachmentDir: attachmentDir}, nil
}

func (s *SqliteStore) Close() error {
	return s.db.Close()
}

func (s *SqliteStore) HasSeen(uidValidity uint32, uid uint32, folder string) (bool, error) {
	var one int
	err := s.db.QueryRow(
		`SELECT 1 FROM mails WHERE uid_validity = ? AND uid = ? AND folder = ? LIMIT 1`,
		uidValidity, uid, folder,
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

// SaveMail / SaveAttachment are implemented in later tasks.
func (s *SqliteStore) SaveMail(m mail.Mail) (int64, error) {
	return 0, fmt.Errorf("SaveMail: not implemented")
}

func (s *SqliteStore) SaveAttachment(mailID int64, a mail.Attachment) error {
	return fmt.Errorf("SaveAttachment: not implemented")
}
```

> **注意 import**：這個檔案 import `github.com/austinjan/mail-agent/internal/mail`。加在 import block 裡。

- [ ] **Step 7: 確認 `SqliteStore` 實現 `Store` 介面**

在 `sqlite.go` 末尾加 compile-time assertion：

```go
var _ Store = (*SqliteStore)(nil)
```

- [ ] **Step 8: 跑測試確認通過**

```bash
go test ./internal/store/...
```

預期：PASS。`TestOpenCreatesSchema` 與 `TestHasSeenEmpty` 都要過。

- [ ] **Step 9: Commit**

```bash
git add go.mod go.sum internal/store
git commit -m "建立 Store 介面與 SQLite schema"
```

## 驗收

- `go test ./internal/store/...` 全過。
- 重複 open 同一個 DB 檔不會錯誤（`IF NOT EXISTS`）。
- `SqliteStore` 在 compile time 滿足 `Store` 介面。
