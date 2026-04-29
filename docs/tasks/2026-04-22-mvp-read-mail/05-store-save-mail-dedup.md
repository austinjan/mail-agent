# Task 05 ??Store: SaveMail + dedup é©—è?

**?®æ?**ï¼šå¯¦ä½?`SqliteStore.SaveMail`ï¼Œæ? `mail.Mail` å¯«å…¥ `mails` è³‡æ?è¡¨ã€‚ç„¶å¾Œä»¥?´å?æ¸¬è©¦é©—è? `HasSeen` ??`HasSeenByMessageID` ?¨å¯«?¥å??½æ­£ç¢ºå???`true`??

**ä¾è³´**ï¼šTask 04??

## ?¢å‡ºæª”æ?

- Modify: `internal/store/sqlite.go`ï¼ˆæ”¹å¯?`SaveMail`ï¼?
- Modify: `internal/store/sqlite_test.go`ï¼ˆæ–°å¢æ¸¬è©¦ï?

## è¨­è?ç­†è?

- `ToAddrs` / `CCAddrs` / `Refs` / `Flags` å­˜æ? JSON å­—ä¸²?‚å³ä½¿ç‚ºç©ºä?å­?`"[]"` ?Œä???NULLï¼Œè??–ç«¯ä¸ç”¨?¤ç©º??
- `fetched_at` ??`SaveMail` ?§è¨­??`time.Now().UTC()`ï¼Œä?å¾?caller ?¶ã€?
- design D6 ?‡å‡ºï¼šå?ä¸€å°?mail ?å¯«?¥æ?ä¸æ??è??‚ç”¨ UNIQUE constraint è§¸ç™¼ `sqlite3_constraint` ?¯èª¤ï¼Œ`SaveMail` ?å ±?ç¢º??`ErrAlreadyExists`ï¼Œè? pipeline ?¯ä»¥?¨è³½æ³ä? fallbackï¼ˆç?è«–ä? pipeline å·²å? `HasSeen` ?ï?ä½?race ?‚ä??ºå??¨ç¶²ï¼‰ã€?

## Steps

- [x] **Step 1: å®?? `ErrAlreadyExists`**

??`internal/store/store.go` ?°å?ï¼?

```go
import "errors"

var ErrAlreadyExists = errors.New("mail already exists")
```

- [x] **Step 2: å¯«å¤±?—æ¸¬è©?*

??`internal/store/sqlite_test.go` è¿½å?ï¼?

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

è¨˜å? test file è£œä? importï¼š`"errors"`, `"time"`, `"github.com/austinjan/mail-agent/internal/mail"`??

- [x] **Step 3: è·‘æ¸¬è©¦ç¢ºèªå¤±??*

```bash
go test ./internal/store/...
```

?æ?ï¼š`SaveMail` ??`not implemented`ï¼Œå…©?‹æ–°æ¸¬è©¦ fail??

- [x] **Step 4: å¯¦ä? `SaveMail`**

?¿æ? `sqlite.go` è£¡ç? `SaveMail` stubï¼?

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

è£œä? importï¼š`"encoding/json"`, `"strings"`, `"time"`, `"github.com/austinjan/mail-agent/internal/mail"`??

- [x] **Step 5: è·‘æ¸¬è©¦ç¢ºèªé€šé?**

```bash
go test ./internal/store/...
```

?æ?ï¼šPASSï¼Œå??¬æ–°?„å…©?‹æ¸¬è©¦ã€?

- [x] **Step 6: Commit**

```bash
git add internal/store
git commit -m "SqliteStore ?¯æ´ SaveMail ??dedup é©—è?"
```

## é©—æ”¶

- `SaveMail` ?å‚³ `mailID > 0`ï¼›å¯«?¥å? `HasSeen` / `HasSeenByMessageID` ?†å? `true`??
- ?è?å¯«å?ä¸€ç­†å? `ErrAlreadyExists`ï¼ˆç”¨ `errors.Is` æª¢æŸ¥ï¼‰ã€?
- ç©?slice ??JSON æ¬„ä??²å???`[]` ?Œé? NULL??
