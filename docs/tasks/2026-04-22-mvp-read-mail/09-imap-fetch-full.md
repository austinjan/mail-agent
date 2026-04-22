# Task 09 — IMAPSource: 完整抓取 + MIME parse

**目標**：擴充 `IMAPSource.Fetch`，對 Task 08 拿到的 UID 清單做 UID FETCH（BODY.PEEK[] + FLAGS），解析 MIME，填好 `mail.Mail`（含 headers、body text、body html、attachments）。

**依賴**：Task 08。

## 產出檔案

- Modify: `internal/source/imap.go`
- Create: `internal/source/mime.go`（將 RFC-822 bytes → `mail.Mail` 的純函式）
- Create: `internal/source/mime_test.go`（以固定的 .eml fixtures 測 parse）
- Create: `internal/source/testdata/simple.eml`
- Create: `internal/source/testdata/with-attachment.eml`

## 設計筆記

- **關鍵原則**：把「bytes → `mail.Mail`」抽成純函式 `parseRFC822([]byte) (mail.Mail, error)`，用 fixture 測。這樣不需要 IMAP 也能驗證 parsing 正確性。
- Multipart 解析用 stdlib `net/mail` + `mime/multipart`。不引入額外套件。
- 優先順序：
  - `text/plain` 放 `BodyText`
  - `text/html` 放 `BodyHTML`
  - 其他 part（有 `Content-Disposition: attachment` 或 `name=` 參數）→ `Attachments`
- `Refs` 來自 `References` header，以空白切分。
- `Flags` 由 IMAP `FLAGS` 欄位填入（包含 Gmail labels 的 `\Flagged`、`$Label*` 等）。
- Subject / address 欄位用 `net/mail.Header.Get` + `mime.WordDecoder` 解碼 RFC 2047 encoded-word。

## Steps

- [ ] **Step 1: 準備 fixture**

`internal/source/testdata/simple.eml`：

```
From: Alice <alice@example.com>
To: Bob <bob@example.com>
Subject: Hello
Message-ID: <simple-001@example.com>
Date: Wed, 22 Apr 2026 10:00:00 +0000
MIME-Version: 1.0
Content-Type: text/plain; charset=utf-8

hi there
```

`internal/source/testdata/with-attachment.eml`：

```
From: Alice <alice@example.com>
To: Bob <bob@example.com>
Subject: Report
Message-ID: <att-001@example.com>
Date: Wed, 22 Apr 2026 11:00:00 +0000
MIME-Version: 1.0
Content-Type: multipart/mixed; boundary="XYZ"

--XYZ
Content-Type: text/plain; charset=utf-8

See attached.
--XYZ
Content-Type: application/octet-stream; name="note.txt"
Content-Disposition: attachment; filename="note.txt"
Content-Transfer-Encoding: base64

aGVsbG8gYXR0YWNobWVudA==
--XYZ--
```

> base64 `aGVsbG8gYXR0YWNobWVudA==` 解碼為 `hello attachment`。

- [ ] **Step 2: 寫 parse 失敗測試 `internal/source/mime_test.go`**

```go
package source

import (
	"os"
	"strings"
	"testing"
)

func TestParseRFC822Simple(t *testing.T) {
	raw, err := os.ReadFile("testdata/simple.eml")
	if err != nil {
		t.Fatal(err)
	}
	m, err := parseRFC822(raw)
	if err != nil {
		t.Fatalf("parseRFC822: %v", err)
	}
	if m.Subject != "Hello" {
		t.Errorf("Subject: got %q", m.Subject)
	}
	if !strings.Contains(m.From, "alice@example.com") {
		t.Errorf("From: got %q", m.From)
	}
	if m.MessageID != "<simple-001@example.com>" {
		t.Errorf("MessageID: got %q", m.MessageID)
	}
	if strings.TrimSpace(m.BodyText) != "hi there" {
		t.Errorf("BodyText: got %q", m.BodyText)
	}
	if len(m.Attachments) != 0 {
		t.Errorf("expected 0 attachments, got %d", len(m.Attachments))
	}
}

func TestParseRFC822WithAttachment(t *testing.T) {
	raw, err := os.ReadFile("testdata/with-attachment.eml")
	if err != nil {
		t.Fatal(err)
	}
	m, err := parseRFC822(raw)
	if err != nil {
		t.Fatalf("parseRFC822: %v", err)
	}
	if !strings.Contains(m.BodyText, "See attached.") {
		t.Errorf("BodyText: got %q", m.BodyText)
	}
	if len(m.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(m.Attachments))
	}
	a := m.Attachments[0]
	if a.Filename != "note.txt" {
		t.Errorf("Filename: got %q", a.Filename)
	}
	if string(a.Content) != "hello attachment" {
		t.Errorf("Content: got %q", a.Content)
	}
}
```

- [ ] **Step 3: 跑測試確認失敗**

```bash
go test ./internal/source/...
```

預期：編譯錯誤（`parseRFC822` 未定義）。

- [ ] **Step 4: 實作 `internal/source/mime.go`**

```go
package source

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	netmail "net/mail"
	"strings"
	"time"

	"github.com/austinjan/mail-agent/internal/mail"
)

// parseRFC822 converts a raw RFC 5322 message into mail.Mail.
// It does not populate UID / UIDValidity / Folder / Flags — those
// come from IMAP and are filled in by the caller.
func parseRFC822(raw []byte) (mail.Mail, error) {
	msg, err := netmail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return mail.Mail{}, fmt.Errorf("ReadMessage: %w", err)
	}

	dec := new(mime.WordDecoder)
	decodeHdr := func(v string) string {
		out, err := dec.DecodeHeader(v)
		if err != nil {
			return v
		}
		return out
	}

	m := mail.Mail{
		MessageID:  strings.TrimSpace(msg.Header.Get("Message-ID")),
		Subject:    decodeHdr(msg.Header.Get("Subject")),
		From:       decodeHdr(msg.Header.Get("From")),
		ToAddrs:    splitAddrs(msg.Header.Get("To"), decodeHdr),
		CCAddrs:    splitAddrs(msg.Header.Get("Cc"), decodeHdr),
		ReplyTo:    decodeHdr(msg.Header.Get("Reply-To")),
		InReplyTo:  strings.TrimSpace(msg.Header.Get("In-Reply-To")),
		Refs:       strings.Fields(msg.Header.Get("References")),
		RawHeaders: extractHeadersBlob(raw),
	}

	if d, err := netmail.ParseDate(msg.Header.Get("Date")); err == nil {
		m.ReceivedAt = d.UTC()
	}

	ct := msg.Header.Get("Content-Type")
	if ct == "" {
		ct = "text/plain"
	}
	mediaType, params, err := mime.ParseMediaType(ct)
	if err != nil {
		return mail.Mail{}, fmt.Errorf("parse Content-Type: %w", err)
	}

	if strings.HasPrefix(mediaType, "multipart/") {
		if err := walkMultipart(msg.Body, params["boundary"], &m); err != nil {
			return mail.Mail{}, err
		}
	} else {
		body, err := io.ReadAll(msg.Body)
		if err != nil {
			return mail.Mail{}, fmt.Errorf("read body: %w", err)
		}
		assignBodyPart(&m, mediaType, body, "", "")
	}
	return m, nil
}

func walkMultipart(body io.Reader, boundary string, m *mail.Mail) error {
	mr := multipart.NewReader(body, boundary)
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("multipart: %w", err)
		}
		partBody, err := io.ReadAll(part)
		if err != nil {
			return fmt.Errorf("read part: %w", err)
		}
		partCT := part.Header.Get("Content-Type")
		if partCT == "" {
			partCT = "text/plain"
		}
		mediaType, params, err := mime.ParseMediaType(partCT)
		if err != nil {
			mediaType = "application/octet-stream"
			params = nil
		}
		if strings.HasPrefix(mediaType, "multipart/") {
			if err := walkMultipart(bytes.NewReader(partBody), params["boundary"], m); err != nil {
				return err
			}
			continue
		}
		disposition := part.Header.Get("Content-Disposition")
		filename := ""
		if _, dparams, err := mime.ParseMediaType(disposition); err == nil {
			filename = dparams["filename"]
		}
		if filename == "" {
			filename = params["name"]
		}
		assignBodyPart(m, mediaType, decodePart(part.Header.Get("Content-Transfer-Encoding"), partBody), filename, partCT)
	}
}

func assignBodyPart(m *mail.Mail, mediaType string, body []byte, filename, rawCT string) {
	switch {
	case filename != "":
		m.Attachments = append(m.Attachments, mail.Attachment{
			Filename:    filename,
			ContentType: rawCT,
			Content:     body,
		})
	case mediaType == "text/plain" && m.BodyText == "":
		m.BodyText = string(body)
	case mediaType == "text/html" && m.BodyHTML == "":
		m.BodyHTML = string(body)
	default:
		// unknown — treat as attachment without filename
		m.Attachments = append(m.Attachments, mail.Attachment{
			ContentType: mediaType,
			Content:     body,
		})
	}
}

func decodePart(encoding string, body []byte) []byte {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "base64":
		// multipart reader already decoded quoted-printable/base64 in recent Go?
		// net/mail + mime/multipart do NOT auto-decode. Decode manually.
		dec, err := base64Decode(body)
		if err != nil {
			return body
		}
		return dec
	case "quoted-printable":
		dec, err := qpDecode(body)
		if err != nil {
			return body
		}
		return dec
	default:
		return body
	}
}

func splitAddrs(v string, decodeHdr func(string) string) []string {
	v = strings.TrimSpace(v)
	if v == "" {
		return []string{}
	}
	addrs, err := netmail.ParseAddressList(v)
	if err != nil {
		return []string{decodeHdr(v)}
	}
	out := make([]string, 0, len(addrs))
	for _, a := range addrs {
		out = append(out, a.Address)
	}
	return out
}

func extractHeadersBlob(raw []byte) string {
	// Headers end at first CRLFCRLF or LFLF.
	if i := bytes.Index(raw, []byte("\r\n\r\n")); i >= 0 {
		return string(raw[:i])
	}
	if i := bytes.Index(raw, []byte("\n\n")); i >= 0 {
		return string(raw[:i])
	}
	return string(raw)
}

// keep time import happy when unused in some refactors
var _ = time.Time{}
```

同時新增 helper `base64Decode` 和 `qpDecode`（用 stdlib `encoding/base64` 與 `mime/quotedprintable`）。可以把它們放在 `mime.go` 尾部：

```go
import (
	"encoding/base64"
	"mime/quotedprintable"
)

func base64Decode(b []byte) ([]byte, error) {
	// Strip whitespace/newlines which base64 in MIME bodies often contains.
	clean := bytes.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, b)
	return base64.StdEncoding.DecodeString(string(clean))
}

func qpDecode(b []byte) ([]byte, error) {
	r := quotedprintable.NewReader(bytes.NewReader(b))
	return io.ReadAll(r)
}
```

- [ ] **Step 5: 跑測試確認通過**

```bash
go test ./internal/source/...
```

預期：`TestParseRFC822Simple` 與 `TestParseRFC822WithAttachment` 都 PASS。

- [ ] **Step 6: 擴充 `IMAPSource.Fetch` 做完整抓取**

在 Task 08 的 `Fetch` 中，把「回傳空 slice」那段換成對每個 UID 做 `UID FETCH`、解析後填回 UID / UIDValidity / Folder / Flags：

```go
	mails := make([]mail.Mail, 0, len(uids))
	numSet := imap.UIDSet{}
	numSet.AddNum(uids...)
	fetchOpts := &imap.FetchOptions{
		Flags: true,
		BodySection: []*imap.FetchItemBodySection{{Peek: true}},
		UID: true,
	}
	msgs, err := c.Fetch(numSet, fetchOpts).Collect()
	if err != nil {
		return nil, uidValidity, fmt.Errorf("imap fetch: %w", err)
	}
	for _, fm := range msgs {
		raw, err := readFirstBodySection(fm)
		if err != nil {
			// log-and-skip: return partial slice; caller decides
			continue
		}
		parsed, err := parseRFC822(raw)
		if err != nil {
			continue
		}
		parsed.UIDValidity = uidValidity
		parsed.UID = uint32(fm.UID)
		parsed.Folder = folder
		parsed.Flags = flagsToStrings(fm.Flags)
		mails = append(mails, parsed)
	}
	return mails, uidValidity, nil
```

輔助函式（放同個檔案內）：

```go
func readFirstBodySection(fm *imapclient.FetchMessageBuffer) ([]byte, error) {
	for _, bs := range fm.BodySection {
		return bs.Bytes, nil
	}
	return nil, fmt.Errorf("no body section in fetch response")
}

func flagsToStrings(flags []imap.Flag) []string {
	out := make([]string, len(flags))
	for i, f := range flags {
		out[i] = string(f)
	}
	return out
}
```

> 同樣提醒：v2 型別名稱以實際版本為準，可能需要微調（如 `FetchMessageBuffer` vs `FetchMessageData`、`BodySection` 的 key vs slice）。實作時 `go doc github.com/emersion/go-imap/v2/imapclient` 先查。

- [ ] **Step 7: 整合測試擴充**

把 `TestIMAPSourceFetchLive` 改成：拿到 mails 後，若 `len(mails) > 0`，斷言第一封 mail 有 Subject 與 ReceivedAt。

- [ ] **Step 8: 跑整合測試**

```bash
MAIL_AGENT_IT=1 MAIL_AGENT_IMAP_HOST=... MAIL_AGENT_IMAP_USER=... MAIL_AGENT_IMAP_PASS=... \
  go test -run TestIMAPSourceFetchLive ./internal/source/...
```

預期：拿得到真實的 mail，`Subject` 非空。

- [ ] **Step 9: Commit**

```bash
git add internal/source
git commit -m "IMAPSource 完整抓取與 MIME 解析"
```

## 驗收

- `parseRFC822` 對 fixture 能正確分出 BodyText / BodyHTML / Attachments。
- 含 attachment 的 fixture 能正確 base64 decode。
- 整合測試能對真實 Gmail 拉到第一封 mail 的 Subject。
