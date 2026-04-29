# Task 05: Store SaveMail 與 Dedup

**目標**：實作 `SqliteStore.SaveMail`，把 `mail.Mail` 寫入 `mails` 資料表，並確認 `HasSeen` 與 `HasSeenByMessageID` 在寫入後可以正確回報已存在。

**依賴**：Task 04 已完成。

## 產出檔案

- Modify: `internal/store/sqlite.go`
- Modify: `internal/store/sqlite_test.go`

## 設計筆記

- `ToAddrs`、`CCAddrs`、`Refs`、`Flags` 以 JSON 字串儲存
- 空 slice 要存成 `"[]"`，不要存成 `NULL`
- `fetched_at` 在 `SaveMail` 中用 `time.Now().UTC()`
- 若 hit 到 UNIQUE constraint，`SaveMail` 要回傳 `ErrAlreadyExists`

## Steps

- [x] **Step 1: 宣告 `ErrAlreadyExists`**

在 `internal/store/store.go` 中新增：

```go
import "errors"

var ErrAlreadyExists = errors.New("mail already exists")
```

- [x] **Step 2: 先寫失敗測試**

在 `internal/store/sqlite_test.go` 中新增：

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
		UIDValidity: 1,
		UID:         100,
		Folder:      "INBOX",
		MessageID:   "<x@example.com>",
		ReceivedAt:  time.Now().UTC(),
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

- [x] **Step 3: 跑測試確認先失敗**

```bash
go test ./internal/store/...
```

預期：`SaveMail` 尚未實作，新測試會失敗。

- [x] **Step 4: 實作 `SaveMail`**

```go
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
```

- [x] **Step 5: 跑測試確認通過**

```bash
go test ./internal/store/...
```

預期：兩個新測試都通過。

- [x] **Step 6: Commit**

```bash
git add internal/store
git commit -m "Implement SaveMail and dedup behavior"
```

## 驗收

- `SaveMail` 會回傳正整數 `mailID`
- 寫入後 `HasSeen` 與 `HasSeenByMessageID` 會回傳 `true`
- 重複寫入同一封 mail 會回傳 `ErrAlreadyExists`
- slice 欄位會以 JSON `[]` 形式寫入，不會是 `NULL`
