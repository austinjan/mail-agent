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
