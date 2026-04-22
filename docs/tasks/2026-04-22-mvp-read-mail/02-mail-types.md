# Task 02 — Mail 資料型別

**目標**：定義 `Mail` 與 `Attachment` 結構，作為 `source` 和 `store` 之間的傳輸型別，避免兩者直接耦合。

**依賴**：Task 01。

## 產出檔案

- Create: `internal/mail/mail.go`
- Create: `internal/mail/mail_test.go`

## 設計筆記

- 這層是**純資料**，不含 IO，不引入 `go-imap` 或 `database/sql`。
- 所有位址欄位用 `string`；IMAP 端解析好再塞進來。
- `Refs` / `ToAddrs` / `CCAddrs` / `Flags` 使用 `[]string`；存到 SQLite 時再 JSON marshal（由 store 層負責）。
- `Attachment.Content` 是 `[]byte`，落地到檔案系統由 store 層處理；傳輸時仍在記憶體裡。

## Steps

- [x] **Step 1: 寫失敗測試**

`internal/mail/mail_test.go`：

```go
package mail

import (
	"testing"
	"time"
)

func TestMailZeroValue(t *testing.T) {
	var m Mail
	if m.UID != 0 {
		t.Errorf("zero value UID: want 0, got %d", m.UID)
	}
	if len(m.Attachments) != 0 {
		t.Errorf("zero value attachments: want empty, got %d", len(m.Attachments))
	}
}

func TestMailFieldsAssignable(t *testing.T) {
	now := time.Now().UTC()
	m := Mail{
		UIDValidity: 1,
		UID:         42,
		Folder:      "INBOX",
		MessageID:   "<abc@example.com>",
		Subject:     "hi",
		From:        "alice@example.com",
		ToAddrs:     []string{"bob@example.com"},
		CCAddrs:     []string{},
		ReplyTo:     "",
		InReplyTo:   "",
		Refs:        []string{},
		Flags:       []string{"\\Seen"},
		ReceivedAt:  now,
		BodyText:    "hello",
		BodyHTML:    "<p>hello</p>",
		RawHeaders:  "Subject: hi\r\n",
		Attachments: []Attachment{{Filename: "a.pdf", ContentType: "application/pdf", Content: []byte{0x25, 0x50}}},
	}
	if m.Attachments[0].Filename != "a.pdf" {
		t.Error("attachment filename not preserved")
	}
}
```

- [x] **Step 2: 跑測試確認失敗**

```bash
go test ./internal/mail/...
```

預期：編譯錯誤（`Mail` / `Attachment` 未定義）。

- [x] **Step 3: 實作 `internal/mail/mail.go`**

```go
// Package mail defines the shared data types that flow between
// sources (e.g. IMAP) and stores (e.g. SQLite). It contains no IO.
package mail

import "time"

// Mail is one fetched message, fully materialised in memory.
type Mail struct {
	UIDValidity uint32
	UID         uint32
	Folder      string

	MessageID string
	Subject   string
	From      string
	ToAddrs   []string
	CCAddrs   []string
	ReplyTo   string
	InReplyTo string
	Refs      []string
	Flags     []string

	ReceivedAt time.Time

	BodyText   string
	BodyHTML   string
	RawHeaders string

	Attachments []Attachment
}

// Attachment is one MIME part treated as a file.
// Content holds the decoded bytes; the store layer is responsible
// for hashing and writing them to disk.
type Attachment struct {
	Filename    string
	ContentType string
	Content     []byte
}
```

- [x] **Step 4: 跑測試確認通過**

```bash
go test ./internal/mail/...
```

預期：`ok  github.com/austinjan/mail-agent/internal/mail`。

- [x] **Step 5: Commit**

```bash
git add internal/mail
git commit -m "新增 Mail 與 Attachment 資料型別"
```

## 驗收

- `go test ./internal/mail/...` 全過。
- `mail` 套件不 import `database/sql`、`go-imap`、任何外部 IO 套件。
