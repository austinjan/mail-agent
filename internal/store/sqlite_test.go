package store

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/austinjan/mail-agent/internal/mail"
)

func TestOpenCreatesSchema(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	attDir := filepath.Join(dir, "attachments")

	s, err := OpenSQLite(dbPath, attDir)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer s.Close()

	s2, err := OpenSQLite(dbPath, attDir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()
}

func TestHasSeenEmpty(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenSQLite(filepath.Join(dir, "test.db"), filepath.Join(dir, "attachments"))
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer s.Close()

	seen, err := s.HasSeen(1, 42, "INBOX")
	if err != nil {
		t.Fatalf("HasSeen: %v", err)
	}
	if seen {
		t.Error("empty DB should report HasSeen = false")
	}

	seen, err = s.HasSeenByMessageID("<nope@example.com>")
	if err != nil {
		t.Fatalf("HasSeenByMessageID: %v", err)
	}
	if seen {
		t.Error("empty DB should report HasSeenByMessageID = false")
	}
}

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
		UIDValidity: 1,
		UID:         100,
		Folder:      "INBOX",
		MessageID:   "<x@example.com>",
		ReceivedAt:  time.Now().UTC(),
	}

	if _, err := s.SaveMail(m); err != nil {
		t.Fatalf("first SaveMail: %v", err)
	}

	_, err = s.SaveMail(m)
	if !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("second SaveMail: expected ErrAlreadyExists, got %v", err)
	}
}

func TestSaveAttachmentWritesFile(t *testing.T) {
	dir := t.TempDir()
	attDir := filepath.Join(dir, "attachments")
	s, err := OpenSQLite(filepath.Join(dir, "test.db"), attDir)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer s.Close()

	mailID, err := s.SaveMail(mail.Mail{
		UIDValidity: 1,
		UID:         1,
		Folder:      "INBOX",
		ReceivedAt:  time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("SaveMail: %v", err)
	}

	content := []byte("hello world")
	a := mail.Attachment{
		Filename:    "greeting.txt",
		ContentType: "text/plain",
		Content:     content,
	}
	if err := s.SaveAttachment(mailID, a); err != nil {
		t.Fatalf("SaveAttachment: %v", err)
	}

	wantSha := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	wantStoredName := wantSha + "_greeting.txt"
	wantPath := filepath.Join(attDir, "b9", wantStoredName)
	got, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read stored file %q: %v", wantPath, err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("stored content mismatch: got %q want %q", got, content)
	}

	var sha, relPath string
	var size int64
	err = s.db.QueryRow(
		`SELECT sha256, size_bytes, file_path FROM attachments WHERE mail_id = ?`,
		mailID,
	).Scan(&sha, &size, &relPath)
	if err != nil {
		t.Fatalf("query attachments: %v", err)
	}
	if sha != wantSha {
		t.Errorf("sha256: got %q want %q", sha, wantSha)
	}
	if size != int64(len(content)) {
		t.Errorf("size_bytes: got %d want %d", size, len(content))
	}
	if relPath != "b9/"+wantStoredName {
		t.Errorf("file_path: got %q want %q", relPath, "b9/"+wantStoredName)
	}
}

func TestOpenCreatesExtractionSchema(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenSQLite(filepath.Join(dir, "test.db"), filepath.Join(dir, "attachments"))
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer s.Close()

	for _, table := range []string{"extraction_jobs", "extracted_fields"} {
		var name string
		err := s.db.QueryRow(
			`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`,
			table,
		).Scan(&name)
		if err != nil {
			t.Fatalf("query table %s: %v", table, err)
		}
		if name != table {
			t.Fatalf("table name: got %q want %q", name, table)
		}
	}
}

func TestExtractionSchemaConstraints(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenSQLite(filepath.Join(dir, "test.db"), filepath.Join(dir, "attachments"))
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer s.Close()

	mailID, err := s.SaveMail(mail.Mail{
		UIDValidity: 1,
		UID:         1,
		Folder:      "INBOX",
		ReceivedAt:  time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("SaveMail: %v", err)
	}

	now := time.Now().UTC()
	res, err := s.db.Exec(
		`INSERT INTO extraction_jobs (mail_id, source_type, status, created_at, updated_at)
		 VALUES (?, 'body', 'pending', ?, ?)`,
		mailID,
		now,
		now,
	)
	if err != nil {
		t.Fatalf("insert extraction job: %v", err)
	}
	jobID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId: %v", err)
	}

	_, err = s.db.Exec(
		`INSERT INTO extracted_fields (
			job_id, mail_id, field_name, field_value, unit, confidence,
			evidence_text, source_type, source_label, created_at
		) VALUES (?, ?, '流量', '120', 'm3/h', 0.9, 'Flow rate 120 m3/h', 'body', 'mail body', ?)`,
		jobID,
		mailID,
		now,
	)
	if err != nil {
		t.Fatalf("insert extracted field: %v", err)
	}

	_, err = s.db.Exec(
		`INSERT INTO extracted_fields (
			job_id, mail_id, field_name, field_value, confidence,
			evidence_text, source_type, created_at
		) VALUES (?, ?, '揚程', '45', 1.5, '45m TDH', 'body', ?)`,
		jobID,
		mailID,
		now,
	)
	if err == nil {
		t.Fatal("expected confidence > 1 to fail")
	}
}

func TestExtractionJobLifecycle(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenSQLite(filepath.Join(dir, "test.db"), filepath.Join(dir, "attachments"))
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer s.Close()

	receivedAt := time.Now().UTC()
	mailID, err := s.SaveMail(mail.Mail{
		UIDValidity: 1,
		UID:         1,
		Folder:      "INBOX",
		ReceivedAt:  receivedAt,
		BodyText:    "Flow 120 m3/h",
	})
	if err != nil {
		t.Fatalf("SaveMail: %v", err)
	}
	if err := s.SaveAttachment(mailID, mail.Attachment{
		Filename:    "spec.txt",
		ContentType: "text/plain",
		Content:     []byte("Head 45m TDH"),
	}); err != nil {
		t.Fatalf("SaveAttachment: %v", err)
	}

	stats, err := s.EnqueueExtractionJobs(receivedAt.Add(-time.Hour))
	if err != nil {
		t.Fatalf("EnqueueExtractionJobs: %v", err)
	}
	if stats.BodyJobs != 1 || stats.AttachmentJobs != 1 {
		t.Fatalf("enqueue stats: %+v", stats)
	}

	stats, err = s.EnqueueExtractionJobs(receivedAt.Add(-time.Hour))
	if err != nil {
		t.Fatalf("EnqueueExtractionJobs second run: %v", err)
	}
	if stats.BodyJobs != 0 || stats.AttachmentJobs != 0 {
		t.Fatalf("second enqueue should be idempotent, got %+v", stats)
	}

	jobs, err := s.PendingExtractionJobs(10)
	if err != nil {
		t.Fatalf("PendingExtractionJobs: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("pending jobs: got %d want 2", len(jobs))
	}

	bodyJob := jobs[0]
	if bodyJob.SourceType != "body" {
		bodyJob = jobs[1]
	}
	if err := s.MarkExtractionJobRunning(bodyJob.ID); err != nil {
		t.Fatalf("MarkExtractionJobRunning: %v", err)
	}
	if err := s.CompleteExtractionJob(bodyJob.ID, []ExtractedField{
		{
			MailID:       mailID,
			FieldName:    "流量",
			FieldValue:   "120",
			Unit:         "m3/h",
			Confidence:   0.8,
			EvidenceText: "Flow 120 m3/h",
			SourceType:   "body",
			SourceLabel:  "mail body",
		},
	}); err != nil {
		t.Fatalf("CompleteExtractionJob: %v", err)
	}

	fields, err := s.ExtractedFieldsByMailID(mailID)
	if err != nil {
		t.Fatalf("ExtractedFieldsByMailID: %v", err)
	}
	if len(fields) != 1 {
		t.Fatalf("fields: got %d want 1", len(fields))
	}
	if fields[0].FieldName != "流量" || fields[0].FieldValue != "120" || fields[0].EvidenceText == "" {
		t.Fatalf("unexpected field: %+v", fields[0])
	}
}

func TestSaveAttachmentSanitizesFilename(t *testing.T) {
	dir := t.TempDir()
	attDir := filepath.Join(dir, "attachments")
	s, err := OpenSQLite(filepath.Join(dir, "test.db"), attDir)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer s.Close()

	mailID, err := s.SaveMail(mail.Mail{
		UIDValidity: 1,
		UID:         1,
		Folder:      "INBOX",
		ReceivedAt:  time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("SaveMail: %v", err)
	}

	content := []byte("report")
	a := mail.Attachment{
		Filename:    `..\..\quarter:1?.pdf`,
		ContentType: "application/pdf",
		Content:     content,
	}
	if err := s.SaveAttachment(mailID, a); err != nil {
		t.Fatalf("SaveAttachment: %v", err)
	}

	var relPath string
	if err := s.db.QueryRow(`SELECT file_path FROM attachments WHERE mail_id = ?`, mailID).Scan(&relPath); err != nil {
		t.Fatalf("query attachment path: %v", err)
	}

	if strings.Contains(relPath, "..") || strings.ContainsAny(relPath, `<>:"|?*`) {
		t.Fatalf("file_path was not sanitized: %q", relPath)
	}
	if filepath.Ext(relPath) != ".pdf" {
		t.Fatalf("file_path should preserve extension, got %q", relPath)
	}
	if _, err := os.Stat(filepath.Join(attDir, filepath.FromSlash(relPath))); err != nil {
		t.Fatalf("stored sanitized file should exist: %v", err)
	}
}

func TestSaveAttachmentDeduplicatesFile(t *testing.T) {
	dir := t.TempDir()
	attDir := filepath.Join(dir, "attachments")
	s, err := OpenSQLite(filepath.Join(dir, "test.db"), attDir)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer s.Close()

	m1, err := s.SaveMail(mail.Mail{UIDValidity: 1, UID: 1, Folder: "INBOX", ReceivedAt: time.Now().UTC()})
	if err != nil {
		t.Fatalf("SaveMail m1: %v", err)
	}
	m2, err := s.SaveMail(mail.Mail{UIDValidity: 1, UID: 2, Folder: "INBOX", ReceivedAt: time.Now().UTC()})
	if err != nil {
		t.Fatalf("SaveMail m2: %v", err)
	}

	a := mail.Attachment{Filename: "x.bin", Content: []byte("shared content")}
	if err := s.SaveAttachment(m1, a); err != nil {
		t.Fatalf("SaveAttachment m1: %v", err)
	}
	if err := s.SaveAttachment(m2, a); err != nil {
		t.Fatalf("SaveAttachment m2: %v", err)
	}

	fileCount := 0
	err = filepath.Walk(attDir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info != nil && !info.IsDir() {
			fileCount++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk attachment dir: %v", err)
	}
	if fileCount != 1 {
		t.Errorf("expected 1 physical file, found %d", fileCount)
	}

	var rows int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM attachments`).Scan(&rows); err != nil {
		t.Fatalf("count attachment rows: %v", err)
	}
	if rows != 2 {
		t.Errorf("expected 2 attachment rows, got %d", rows)
	}
}
