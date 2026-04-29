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
