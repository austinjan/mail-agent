package core

import (
	"bytes"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/austinjan/mail-agent/internal/mail"
)

func newTestLogger(t *testing.T) (*slog.Logger, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	return slog.New(handler), &buf
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

	stats, err := New(src, st, logger).Run("INBOX", time.Now().Add(-24*time.Hour))
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

	stats, err := New(src, st, logger).Run("INBOX", time.Now().Add(-24*time.Hour))
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
	src := &mockSource{
		uidValidity: 99,
		mails: []mail.Mail{
			{UIDValidity: 99, UID: 1, Folder: "INBOX", MessageID: "<dup@x>"},
		},
	}
	logger, _ := newTestLogger(t)

	stats, err := New(src, st, logger).Run("INBOX", time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if stats.Saved != 0 || stats.SkippedDedup != 1 {
		t.Errorf("stats: %+v", stats)
	}
}

func TestRunSavesAttachments(t *testing.T) {
	src := &mockSource{
		uidValidity: 1,
		mails: []mail.Mail{
			{
				UIDValidity: 1,
				UID:         200,
				Folder:      "INBOX",
				MessageID:   "<att@x>",
				Attachments: []mail.Attachment{
					{Filename: "a.txt", Content: []byte("hi")},
					{Filename: "b.txt", Content: []byte("there")},
				},
			},
		},
	}
	st := newMockStore()
	logger, _ := newTestLogger(t)

	stats, err := New(src, st, logger).Run("INBOX", time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
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

	if _, err := New(src, st, logger).Run("INBOX", time.Now().Add(-24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{`"event":"fetch_start"`, `"event":"fetch_done"`} {
		if !bytes.Contains([]byte(out), []byte(want)) {
			t.Errorf("log missing %q; got:\n%s", want, out)
		}
	}
}

func TestRunContinuesAfterSaveError(t *testing.T) {
	src := &mockSource{
		uidValidity: 1,
		mails: []mail.Mail{
			{UIDValidity: 1, UID: 100, Folder: "INBOX", MessageID: "<a@x>"},
		},
	}
	st := newMockStore()
	st.saveErr = errors.New("boom")
	logger, _ := newTestLogger(t)

	stats, err := New(src, st, logger).Run("INBOX", time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if stats.Errors != 1 || stats.Saved != 0 {
		t.Errorf("stats: %+v", stats)
	}
}
