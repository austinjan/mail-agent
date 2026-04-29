package source

import (
	"os"
	"testing"
	"time"
)

// Integration test — needs a real IMAP account.
// Enable with `MAIL_AGENT_IT=1 go test ./internal/source/...`.
// Reads credentials from env to avoid committing secrets.
func TestIMAPSourceFetchLive(t *testing.T) {
	if os.Getenv("MAIL_AGENT_IT") != "1" {
		t.Skip("integration test; set MAIL_AGENT_IT=1 to run")
	}
	cfg := IMAPConfig{
		Host:     os.Getenv("MAIL_AGENT_IMAP_HOST"),
		Port:     993,
		User:     os.Getenv("MAIL_AGENT_IMAP_USER"),
		Password: os.Getenv("MAIL_AGENT_IMAP_PASS"),
	}
	if cfg.Host == "" || cfg.User == "" || cfg.Password == "" {
		t.Fatal("MAIL_AGENT_IMAP_HOST / USER / PASS must be set")
	}
	src := NewIMAPSource(cfg)
	mails, uidValidity, err := src.Fetch("INBOX", time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if uidValidity == 0 {
		t.Error("expected non-zero UIDVALIDITY")
	}
	if len(mails) > 0 {
		m := mails[0]
		if m.Subject == "" {
			t.Error("expected non-empty Subject on first mail")
		}
		if m.ReceivedAt.IsZero() {
			t.Error("expected non-zero ReceivedAt on first mail")
		}
	}
}
