# Task 04: Store 介面與 SQLite Schema

**目標**：定義 `Store` 介面，建立 SQLite schema，並完成 `SqliteStore` 的開啟、關閉與 schema 初始化。這個 task 先不處理 mail 寫入與 attachment 寫入邏輯。

**依賴**：Task 02 已完成，已有 `mail.Mail` 型別可用。

## 產出檔案

- Create: `internal/store/store.go`
- Create: `internal/store/schema.sql`
- Create: `internal/store/sqlite.go`
- Create: `internal/store/sqlite_test.go`

## Steps

- [x] **Step 1: 加入 SQLite 依賴**

```bash
go get modernc.org/sqlite
```

- [x] **Step 2: 定義介面 `internal/store/store.go`**

```go
// Package store persists fetched mails and attachments.
// The Store interface decouples the core pipeline from any
// particular backend; MVP uses SqliteStore.
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

- [x] **Step 3: 建立 schema `internal/store/schema.sql`**

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

- [x] **Step 4: 先寫失敗測試**

`internal/store/sqlite_test.go`

```go
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
```

- [x] **Step 5: 跑測試確認先失敗**

```bash
go test ./internal/store/...
```

預期：因為 `OpenSQLite` 尚未實作，測試會先失敗。

- [x] **Step 6: 實作 `internal/store/sqlite.go`**

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
		_ = db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &SqliteStore{db: db, attachmentDir: attachmentDir}, nil
}

func (s *SqliteStore) Close() error {
	return s.db.Close()
}
```

- [x] **Step 7: 確認 `SqliteStore` 實作 `Store` 介面**

在 `sqlite.go` 中加入 compile-time assertion：

```go
var _ Store = (*SqliteStore)(nil)
```

- [x] **Step 8: 跑測試確認通過**

```bash
go test ./internal/store/...
```

預期：`TestOpenCreatesSchema` 與 `TestHasSeenEmpty` 通過。

- [x] **Step 9: Commit**

```bash
git add go.mod go.sum internal/store
git commit -m "Implement store interface and SQLite schema"
```

## 驗收

- `go test ./internal/store/...` 通過
- 重複開啟同一個 DB 不會因 schema 初始化失敗
- `SqliteStore` 在 compile time 滿足 `Store` 介面
