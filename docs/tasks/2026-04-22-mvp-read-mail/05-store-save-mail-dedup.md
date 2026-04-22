# Task 05 — Store: SaveMail + dedup 驗證

**目標**：實作 `SqliteStore.SaveMail`，把 `mail.Mail` 寫入 `mails` 資料表。然後以整合測試驗證 `HasSeen` 和 `HasSeenByMessageID` 在寫入後能正確回報 `true`。

**依賴**：Task 04。

## 產出檔案

- Modify: `internal/store/sqlite.go`（改寫 `SaveMail`）
- Modify: `internal/store/sqlite_test.go`（新增測試）

## 設計筆記

- `ToAddrs` / `CCAddrs` / `Refs` / `Flags` 存成 JSON 字串。即使為空也存 `"[]"` 而不是 NULL，讀取端不用判空。
- `fetched_at` 在 `SaveMail` 內設為 `time.Now().UTC()`，不從 caller 收。
- design D6 指出：同一封 mail 再寫入時不應重複。用 UNIQUE constraint 觸發 `sqlite3_constraint` 錯誤，`SaveMail` 回報明確的 `ErrAlreadyExists`，讓 pipeline 可以在賽況下 fallback（理論上 pipeline 已先 `HasSeen` 過，但 race 時作為安全網）。

## Steps

- [ ] **Step 1: 宣告 `ErrAlreadyExists`**

在 `internal/store/store.go` 新增：

```go
import "errors"

var ErrAlreadyExists = errors.New("mail already exists")
```

- [ ] **Step 2: 寫失敗測試**

在 `internal/store/sqlite_test.go` 追加：

```go
func TestSaveMailAndHasSeen(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenSQLite(filepath.Join(dir, "test.db"), filepath.Join(dir, "attachments"))
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer s.Close()

	m := mail.Mail{
		UIDValidity: 1,
		UID:         100,
		Folder:      "INBOX",
		MessageID:   "<hello@example.com>",
		Subject:     "hi",
		From:        "alice@example.com",
		ToAddrs:     []string{"bob@example.com"},
		CCAddrs:     []string{},
		Refs:        []string{},
		Flags:       []string{"\\Seen"},
		ReceivedAt:  time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC),
		BodyText:    "hello",
	}

	id, err := s.SaveMail(m)
	if err != nil {
		t.Fatalf("SaveMail: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive mailID, got %d", id)
	}

	seen, err := s.HasSeen(1, 100, "INBOX")
	if err != nil {
		t.Fatalf("HasSeen: %v", err)
	}
	if !seen {
		t.Error("HasSeen should be true after SaveMail")
	}

	seen, err = s.HasSeenByMessageID("<hello@example.com>")
	if err != nil {
		t.Fatalf("HasSeenByMessageID: %v", err)
	}
	if !seen {
		t.Error("HasSeenByMessageID should be true after SaveMail")
	}
}

func TestSaveMailDuplicateReturnsErrAlreadyExists(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenSQLite(filepath.Join(dir, "test.db"), filepath.Join(dir, "attachments"))
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer s.Close()

	m := mail.Mail{
		UIDValidity: 1, UID: 100, Folder: "INBOX",
		MessageID: "<x@example.com>",
		ReceivedAt: time.Now().UTC(),
	}
	if _, err := s.SaveMail(m); err != nil {
		t.Fatalf("first SaveMail: %v", err)
	}
	_, err = s.SaveMail(m)
	if !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("second SaveMail: expected ErrAlreadyExists, got %v", err)
	}
}
```

記得 test file 補上 import：`"errors"`, `"time"`, `"github.com/austinjan/mail-agent/internal/mail"`。

- [ ] **Step 3: 跑測試確認失敗**

```bash
go test ./internal/store/...
```

預期：`SaveMail` 回 `not implemented`，兩個新測試 fail。

- [ ] **Step 4: 實作 `SaveMail`**

替換 `sqlite.go` 裡的 `SaveMail` stub：

```go
func (s *SqliteStore) SaveMail(m mail.Mail) (int64, error) {
	to, _ := json.Marshal(defaultSlice(m.ToAddrs))
	cc, _ := json.Marshal(defaultSlice(m.CCAddrs))
	refs, _ := json.Marshal(defaultSlice(m.Refs))
	flags, _ := json.Marshal(defaultSlice(m.Flags))

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
	// modernc.org/sqlite surfaces the SQLite extended error codes
	// via error string. Matching on text is adequate for MVP.
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
```

補上 import：`"encoding/json"`, `"strings"`, `"time"`, `"github.com/austinjan/mail-agent/internal/mail"`。

- [ ] **Step 5: 跑測試確認通過**

```bash
go test ./internal/store/...
```

預期：PASS，包括新的兩個測試。

- [ ] **Step 6: Commit**

```bash
git add internal/store
git commit -m "SqliteStore 支援 SaveMail 與 dedup 驗證"
```

## 驗收

- `SaveMail` 回傳 `mailID > 0`；寫入後 `HasSeen` / `HasSeenByMessageID` 皆回 `true`。
- 重複寫同一筆回 `ErrAlreadyExists`（用 `errors.Is` 檢查）。
- 空 slice 的 JSON 欄位儲存為 `[]` 而非 NULL。
