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

func TestParseRFC822SinglePartBase64(t *testing.T) {
	raw := []byte(strings.Join([]string{
		"From: Alice <alice@example.com>",
		"To: Bob <bob@example.com>",
		"Subject: Encoded",
		"Message-ID: <b64-001@example.com>",
		"Date: Wed, 22 Apr 2026 10:00:00 +0000",
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=utf-8",
		"Content-Transfer-Encoding: base64",
		"",
		"aGVsbG8gd29ybGQ=",
	}, "\r\n"))

	m, err := parseRFC822(raw)
	if err != nil {
		t.Fatalf("parseRFC822: %v", err)
	}
	if strings.TrimSpace(m.BodyText) != "hello world" {
		t.Errorf("BodyText: got %q", m.BodyText)
	}
}

func TestParseRFC822SinglePartQuotedPrintable(t *testing.T) {
	raw := []byte(strings.Join([]string{
		"From: Alice <alice@example.com>",
		"To: Bob <bob@example.com>",
		"Subject: Encoded QP",
		"Message-ID: <qp-001@example.com>",
		"Date: Wed, 22 Apr 2026 10:00:00 +0000",
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=utf-8",
		"Content-Transfer-Encoding: quoted-printable",
		"",
		"hello=20world",
	}, "\r\n"))

	m, err := parseRFC822(raw)
	if err != nil {
		t.Fatalf("parseRFC822: %v", err)
	}
	if strings.TrimSpace(m.BodyText) != "hello world" {
		t.Errorf("BodyText: got %q", m.BodyText)
	}
}
