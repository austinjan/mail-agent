package source

import (
	"os"
	"testing"
	"time"

	"github.com/austinjan/mail-agent/internal/mail"
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

func TestIncludeMailSince(t *testing.T) {
	since := time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		m    mail.Mail
		want bool
	}{
		{
			name: "after cutoff",
			m:    mail.Mail{ReceivedAt: since.Add(5 * time.Minute)},
			want: true,
		},
		{
			name: "exactly at cutoff",
			m:    mail.Mail{ReceivedAt: since},
			want: true,
		},
		{
			name: "before cutoff on same day",
			m:    mail.Mail{ReceivedAt: since.Add(-5 * time.Minute)},
			want: false,
		},
		{
			name: "unknown received time kept",
			m:    mail.Mail{},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := includeMailSince(tt.m, since)
			if got != tt.want {
				t.Fatalf("includeMailSince() = %v, want %v", got, tt.want)
			}
		})
	}
}
