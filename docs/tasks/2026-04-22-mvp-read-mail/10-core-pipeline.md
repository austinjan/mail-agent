# Task 10 — Core pipeline + slog

**目標**：實作 `internal/core/pipeline.go`：吃 `Source` + `Store` + `*slog.Logger`，執行 fetch → dedup → persist → log 流程。pipeline 本身只依賴介面，用 mock 驗證行為。

**依賴**：Task 05（`Store`）、Task 07（`Source`）。Task 09 可平行進行。

## 產出檔案

- Create: `internal/core/pipeline.go`
- Create: `internal/core/pipeline_test.go`
- Create: `internal/core/mocks_test.go`

## 設計筆記

- `Run(folder string, since time.Time) (Stats, error)` — 回傳執行統計（fetched、saved、skipped、attachmentsSaved、errors）。
- 錯誤處理遵循 design D10：單筆 mail parse / save 失敗 → log + 計入 errors，繼續下一封。IMAP 連線層級錯誤 → 直接回傳 error（讓 CLI 決定 exit code，但實際 design 要求 exit 0；main 會在最外層吞掉）。
- slog 事件名稱要和 design §D10 對齊：`fetch_start`、`fetch_done`、`mail_saved`、`mail_parse_failed`（core 裡對應 `mail_save_failed`）、`mail_skipped_dedup`、`attachment_saved`、`attachment_save_failed`。
- pipeline 內每次儲存用**交易語義**簡化：先 `HasSeen` 檢查（fallback 再查 `HasSeenByMessageID`），兩者都否才 `SaveMail`。

## Steps

- [ ] **Step 1: 寫 mock `internal/core/mocks_test.go`**

```go
package core

import (
	"time"

	"github.com/austinjan/mail-agent/internal/mail"
)

type mockSource struct {
	mails       []mail.Mail
	uidValidity uint32
	err         error
}

func (m *mockSource) Fetch(folder string, since time.Time) ([]mail.Mail, uint32, error) {
	return m.mails, m.uidValidity, m.err
}

type savedMail struct {
	ID   int64
	Mail mail.Mail
}

type mockStore struct {
	seenByUID map[string]bool     // key: fmt.Sprintf("%d-%d-%s", uidVal, uid, folder)
	seenByMID map[string]bool
	nextID    int64
	saved     []savedMail
	atts      map[int64][]mail.Attachment
	saveErr   error
}

func newMockStore() *mockStore {
	return &mockStore{
		seenByUID: map[string]bool{},
		seenByMID: map[string]bool{},
		atts:      map[int64][]mail.Attachment{},
	}
}

func (s *mockStore) SaveMail(m mail.Mail) (int64, error) {
	if s.saveErr != nil {
		return 0, s.saveErr
	}
	s.nextID++
	s.saved = append(s.saved, savedMail{ID: s.nextID, Mail: m})
	s.seenByUID[key(m.UIDValidity, m.UID, m.Folder)] = true
	if m.MessageID != "" {
		s.seenByMID[m.MessageID] = true
	}
	return s.nextID, nil
}

func (s *mockStore) HasSeen(uidValidity uint32, uid uint32, folder string) (bool, error) {
	return s.seenByUID[key(uidValidity, uid, folder)], nil
}

func (s *mockStore) HasSeenByMessageID(mid string) (bool, error) {
	if mid == "" {
		return false, nil
	}
	return s.seenByMID[mid], nil
}

func (s *mockStore) SaveAttachment(mailID int64, a mail.Attachment) error {
	s.atts[mailID] = append(s.atts[mailID], a)
	return nil
}

func (s *mockStore) Close() error { return nil }

func key(uv, uid uint32, f string) string {
	return fmtKey(uv, uid, f)
}

func fmtKey(uv, uid uint32, f string) string {
	return f + ":" + strconvU32(uv) + ":" + strconvU32(uid)
}

func strconvU32(n uint32) string {
	// avoid importing strconv in test helpers — short form
	return fmt.Sprintf("%d", n)
}
```

（實作時若偷懶直接 `import "strconv"` 當然更好，上面 `strconvU32` 是佔位；修掉即可。）

- [ ] **Step 2: 寫失敗測試 `internal/core/pipeline_test.go`**

```go
package core

import (
	"bytes"
	"log/slog"
	"testing"
	"time"

	"github.com/austinjan/mail-agent/internal/mail"
)

func newTestLogger(t *testing.T) (*slog.Logger, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	h := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	return slog.New(h), &buf
}

func TestRunSavesNewMails(t *testing.T) {
	src := &mockSource{
		uidValidity: 1,
		mails: []mail.Mail{
			{UIDValidity: 1, UID: 100, Folder: "INBOX", MessageID: "<a@x>", Subject: "one"},
			{UIDValidity: 1, UID: 101, Folder: "INBOX", MessageID: "<b@x>", Subject: "two"},
		},
	}
	st := newMockStore()
	logger, _ := newTestLogger(t)
	p := New(src, st, logger)

	stats, err := p.Run("INBOX", time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if stats.Fetched != 2 || stats.Saved != 2 || stats.SkippedDedup != 0 {
		t.Errorf("stats: %+v", stats)
	}
	if len(st.saved) != 2 {
		t.Errorf("saved mails: got %d want 2", len(st.saved))
	}
}

func TestRunSkipsDedupByUID(t *testing.T) {
	st := newMockStore()
	st.seenByUID[key(1, 100, "INBOX")] = true

	src := &mockSource{
		uidValidity: 1,
		mails: []mail.Mail{
			{UIDValidity: 1, UID: 100, Folder: "INBOX", MessageID: "<a@x>"},
			{UIDValidity: 1, UID: 101, Folder: "INBOX", MessageID: "<b@x>"},
		},
	}
	logger, _ := newTestLogger(t)
	p := New(src, st, logger)

	stats, err := p.Run("INBOX", time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if stats.Saved != 1 || stats.SkippedDedup != 1 {
		t.Errorf("stats: %+v", stats)
	}
}

func TestRunSkipsDedupByMessageID(t *testing.T) {
	st := newMockStore()
	st.seenByMID["<dup@x>"] = true

	// UIDValidity different so UID-based dedup misses, fallback to Message-ID.
	src := &mockSource{
		uidValidity: 99,
		mails: []mail.Mail{
			{UIDValidity: 99, UID: 1, Folder: "INBOX", MessageID: "<dup@x>"},
		},
	}
	logger, _ := newTestLogger(t)
	p := New(src, st, logger)

	stats, _ := p.Run("INBOX", time.Now().Add(-24*time.Hour))
	if stats.Saved != 0 || stats.SkippedDedup != 1 {
		t.Errorf("stats: %+v", stats)
	}
}

func TestRunSavesAttachments(t *testing.T) {
	src := &mockSource{
		uidValidity: 1,
		mails: []mail.Mail{
			{
				UIDValidity: 1, UID: 200, Folder: "INBOX", MessageID: "<att@x>",
				Attachments: []mail.Attachment{
					{Filename: "a.txt", Content: []byte("hi")},
					{Filename: "b.txt", Content: []byte("there")},
				},
			},
		},
	}
	st := newMockStore()
	logger, _ := newTestLogger(t)
	p := New(src, st, logger)

	stats, _ := p.Run("INBOX", time.Now().Add(-24*time.Hour))
	if stats.AttachmentsSaved != 2 {
		t.Errorf("AttachmentsSaved: got %d want 2", stats.AttachmentsSaved)
	}
	if len(st.atts[1]) != 2 {
		t.Errorf("atts[1]: got %d want 2", len(st.atts[1]))
	}
}

func TestRunLogsFetchStartAndDone(t *testing.T) {
	src := &mockSource{uidValidity: 1}
	st := newMockStore()
	logger, buf := newTestLogger(t)
	p := New(src, st, logger)

	if _, err := p.Run("INBOX", time.Now().Add(-24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{`"event":"fetch_start"`, `"event":"fetch_done"`} {
		if !bytes.Contains([]byte(out), []byte(want)) {
			t.Errorf("log missing %q; got:\n%s", want, out)
		}
	}
}
```

- [ ] **Step 3: 跑測試確認失敗**

```bash
go test ./internal/core/...
```

預期：編譯錯誤（`New`, `Pipeline`, `Stats` 未定義）。

- [ ] **Step 4: 實作 `internal/core/pipeline.go`**

```go
// Package core runs the fetch → dedup → persist pipeline.
// It depends only on the source.Source and store.Store interfaces.
package core

import (
	"log/slog"
	"time"

	"github.com/austinjan/mail-agent/internal/mail"
	"github.com/austinjan/mail-agent/internal/source"
	"github.com/austinjan/mail-agent/internal/store"
)

type Stats struct {
	Fetched          int
	Saved            int
	SkippedDedup     int
	AttachmentsSaved int
	Errors           int
}

type Pipeline struct {
	src    source.Source
	store  store.Store
	logger *slog.Logger
}

func New(src source.Source, st store.Store, logger *slog.Logger) *Pipeline {
	return &Pipeline{src: src, store: st, logger: logger}
}

func (p *Pipeline) Run(folder string, since time.Time) (Stats, error) {
	var stats Stats
	p.logger.Info("", "event", "fetch_start", "folder", folder, "since", since.Format(time.RFC3339))

	mails, uidValidity, err := p.src.Fetch(folder, since)
	if err != nil {
		p.logger.Error("", "event", "fetch_failed", "error", err.Error())
		return stats, err
	}
	stats.Fetched = len(mails)

	for _, m := range mails {
		if seen, _ := p.store.HasSeen(m.UIDValidity, m.UID, m.Folder); seen {
			stats.SkippedDedup++
			p.logger.Info("", "event", "mail_skipped_dedup", "uid", m.UID, "reason", "uid")
			continue
		}
		if m.MessageID != "" {
			if seen, _ := p.store.HasSeenByMessageID(m.MessageID); seen {
				stats.SkippedDedup++
				p.logger.Info("", "event", "mail_skipped_dedup", "uid", m.UID, "reason", "message_id")
				continue
			}
		}
		id, err := p.store.SaveMail(m)
		if err != nil {
			stats.Errors++
			p.logger.Error("", "event", "mail_save_failed", "uid", m.UID, "error", err.Error())
			continue
		}
		stats.Saved++
		p.logger.Info("", "event", "mail_saved", "uid", m.UID, "subject", m.Subject, "mail_id", id)

		for _, a := range m.Attachments {
			if err := p.store.SaveAttachment(id, a); err != nil {
				stats.Errors++
				p.logger.Error("", "event", "attachment_save_failed", "mail_id", id, "filename", a.Filename, "error", err.Error())
				continue
			}
			stats.AttachmentsSaved++
			p.logger.Info("", "event", "attachment_saved", "mail_id", id, "filename", a.Filename)
		}
	}

	p.logger.Info("", "event", "fetch_done",
		"uid_validity", uidValidity,
		"fetched", stats.Fetched,
		"saved", stats.Saved,
		"skipped", stats.SkippedDedup,
		"attachments", stats.AttachmentsSaved,
		"errors", stats.Errors,
	)
	return stats, nil
}
```

- [ ] **Step 5: 跑測試確認通過**

```bash
go test ./internal/core/...
```

預期：PASS，五個測試全過。

- [ ] **Step 6: Commit**

```bash
git add internal/core
git commit -m "新增 core pipeline 與結構化 logging"
```

## 驗收

- 五個單元測試全過，涵蓋新 mail 儲存、UID dedup、Message-ID dedup、attachment 存取、log 事件存在。
- pipeline 只 import `internal/source`、`internal/store`、`internal/mail`，不直接碰 `imap` 或 `sqlite`。
- slog JSON 輸出包含 design D10 規定的事件名稱。
