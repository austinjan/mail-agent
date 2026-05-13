package store

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"time"
)

type ExtractionEnqueueStats struct {
	BodyJobs       int64
	AttachmentJobs int64
}

type ExtractionJob struct {
	ID                    int64
	MailID                int64
	AttachmentID          *int64
	SourceType            string
	Status                string
	Attempts              int
	BodyText              string
	BodyHTML              string
	AttachmentFilename    string
	AttachmentContentType string
	AttachmentPath        string
}

type ExtractedField struct {
	JobID        int64
	MailID       int64
	AttachmentID *int64
	FieldName    string
	FieldValue   string
	Unit         string
	Confidence   float64
	EvidenceText string
	SourceType   string
	SourceLabel  string
	CreatedAt    time.Time
}

func (s *SqliteStore) EnqueueExtractionJobs(since time.Time) (ExtractionEnqueueStats, error) {
	now := time.Now().UTC()
	stats := ExtractionEnqueueStats{}

	bodyRes, err := s.db.Exec(`
INSERT OR IGNORE INTO extraction_jobs (mail_id, source_type, status, created_at, updated_at)
SELECT id, 'body', 'pending', ?, ?
FROM mails
WHERE received_at >= ?
`, now, now, since.UTC())
	if err != nil {
		return stats, fmt.Errorf("enqueue body jobs: %w", err)
	}
	stats.BodyJobs, _ = bodyRes.RowsAffected()

	attachmentRes, err := s.db.Exec(`
INSERT OR IGNORE INTO extraction_jobs (mail_id, attachment_id, source_type, status, created_at, updated_at)
SELECT m.id, a.id, 'attachment', 'pending', ?, ?
FROM attachments a
JOIN mails m ON m.id = a.mail_id
WHERE m.received_at >= ?
`, now, now, since.UTC())
	if err != nil {
		return stats, fmt.Errorf("enqueue attachment jobs: %w", err)
	}
	stats.AttachmentJobs, _ = attachmentRes.RowsAffected()

	return stats, nil
}

func (s *SqliteStore) PendingExtractionJobs(limit int) ([]ExtractionJob, error) {
	if limit <= 0 {
		limit = 20
	}

	rows, err := s.db.Query(`
SELECT
	j.id, j.mail_id, j.attachment_id, j.source_type, j.status, j.attempts,
	COALESCE(m.body_text, ''), COALESCE(m.body_html, ''),
	COALESCE(a.filename, ''), COALESCE(a.content_type, ''), COALESCE(a.file_path, '')
FROM extraction_jobs j
JOIN mails m ON m.id = j.mail_id
LEFT JOIN attachments a ON a.id = j.attachment_id
WHERE j.status IN ('pending', 'failed')
  AND j.attempts < 5
ORDER BY
	CASE j.status WHEN 'pending' THEN 0 ELSE 1 END,
	j.updated_at ASC,
	j.id ASC
LIMIT ?
`, limit)
	if err != nil {
		return nil, fmt.Errorf("query pending extraction jobs: %w", err)
	}
	defer rows.Close()

	var jobs []ExtractionJob
	for rows.Next() {
		var job ExtractionJob
		var attachmentID sql.NullInt64
		if err := rows.Scan(
			&job.ID,
			&job.MailID,
			&attachmentID,
			&job.SourceType,
			&job.Status,
			&job.Attempts,
			&job.BodyText,
			&job.BodyHTML,
			&job.AttachmentFilename,
			&job.AttachmentContentType,
			&job.AttachmentPath,
		); err != nil {
			return nil, fmt.Errorf("scan extraction job: %w", err)
		}
		if attachmentID.Valid {
			id := attachmentID.Int64
			job.AttachmentID = &id
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate extraction jobs: %w", err)
	}
	return jobs, nil
}

func (s *SqliteStore) MarkExtractionJobRunning(jobID int64) error {
	now := time.Now().UTC()
	_, err := s.db.Exec(`
UPDATE extraction_jobs
SET status = 'running', attempts = attempts + 1, error = NULL, updated_at = ?
WHERE id = ?
`, now, jobID)
	if err != nil {
		return fmt.Errorf("mark extraction job running: %w", err)
	}
	return nil
}

func (s *SqliteStore) CompleteExtractionJob(jobID int64, fields []ExtractedField) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin complete extraction job: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM extracted_fields WHERE job_id = ?`, jobID); err != nil {
		return fmt.Errorf("delete old extracted fields: %w", err)
	}

	now := time.Now().UTC()
	for _, f := range fields {
		createdAt := f.CreatedAt
		if createdAt.IsZero() {
			createdAt = now
		}
		var attachmentID any
		if f.AttachmentID != nil {
			attachmentID = *f.AttachmentID
		}
		if _, err := tx.Exec(`
INSERT INTO extracted_fields (
	job_id, mail_id, attachment_id, field_name, field_value, unit,
	confidence, evidence_text, source_type, source_label, created_at
) VALUES (?,?,?,?,?,?,?,?,?,?,?)
`, jobID, f.MailID, attachmentID, f.FieldName, f.FieldValue, nullableString(f.Unit),
			f.Confidence, f.EvidenceText, f.SourceType, f.SourceLabel, createdAt); err != nil {
			return fmt.Errorf("insert extracted field %q: %w", f.FieldName, err)
		}
	}

	if _, err := tx.Exec(`
UPDATE extraction_jobs
SET status = 'done', error = NULL, updated_at = ?, finished_at = ?
WHERE id = ?
`, now, now, jobID); err != nil {
		return fmt.Errorf("mark extraction job done: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit complete extraction job: %w", err)
	}
	return nil
}

func (s *SqliteStore) MarkExtractionJobFailed(jobID int64, message string) error {
	return s.setExtractionJobStatus(jobID, "failed", message)
}

func (s *SqliteStore) MarkExtractionJobUnsupported(jobID int64, message string) error {
	return s.setExtractionJobStatus(jobID, "unsupported", message)
}

func (s *SqliteStore) setExtractionJobStatus(jobID int64, status, message string) error {
	now := time.Now().UTC()
	_, err := s.db.Exec(`
UPDATE extraction_jobs
SET status = ?, error = ?, updated_at = ?, finished_at = ?
WHERE id = ?
`, status, message, now, now, jobID)
	if err != nil {
		return fmt.Errorf("mark extraction job %s: %w", status, err)
	}
	return nil
}

func (s *SqliteStore) ExtractedFieldsByMailID(mailID int64) ([]ExtractedField, error) {
	rows, err := s.db.Query(`
SELECT
	job_id, mail_id, attachment_id, field_name, field_value, COALESCE(unit, ''),
	COALESCE(confidence, 0), evidence_text, source_type, COALESCE(source_label, ''), created_at
FROM extracted_fields
WHERE mail_id = ?
ORDER BY id ASC
`, mailID)
	if err != nil {
		return nil, fmt.Errorf("query extracted fields: %w", err)
	}
	defer rows.Close()

	var fields []ExtractedField
	for rows.Next() {
		var f ExtractedField
		var attachmentID sql.NullInt64
		if err := rows.Scan(
			&f.JobID,
			&f.MailID,
			&attachmentID,
			&f.FieldName,
			&f.FieldValue,
			&f.Unit,
			&f.Confidence,
			&f.EvidenceText,
			&f.SourceType,
			&f.SourceLabel,
			&f.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan extracted field: %w", err)
		}
		if attachmentID.Valid {
			id := attachmentID.Int64
			f.AttachmentID = &id
		}
		fields = append(fields, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate extracted fields: %w", err)
	}
	return fields, nil
}

func (s *SqliteStore) ExtractedFields(mailID *int64) ([]ExtractedField, error) {
	query := `
SELECT
	job_id, mail_id, attachment_id, field_name, field_value, COALESCE(unit, ''),
	COALESCE(confidence, 0), evidence_text, source_type, COALESCE(source_label, ''), created_at
FROM extracted_fields`
	var args []any
	if mailID != nil {
		query += ` WHERE mail_id = ?`
		args = append(args, *mailID)
	}
	query += ` ORDER BY mail_id ASC, id ASC`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query extracted fields: %w", err)
	}
	defer rows.Close()
	return scanExtractedFields(rows)
}

func (s *SqliteStore) AttachmentFullPath(relPath string) string {
	return filepath.Join(s.attachmentDir, filepath.FromSlash(relPath))
}

func scanExtractedFields(rows *sql.Rows) ([]ExtractedField, error) {
	var fields []ExtractedField
	for rows.Next() {
		var f ExtractedField
		var attachmentID sql.NullInt64
		if err := rows.Scan(
			&f.JobID,
			&f.MailID,
			&attachmentID,
			&f.FieldName,
			&f.FieldValue,
			&f.Unit,
			&f.Confidence,
			&f.EvidenceText,
			&f.SourceType,
			&f.SourceLabel,
			&f.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan extracted field: %w", err)
		}
		if attachmentID.Valid {
			id := attachmentID.Int64
			f.AttachmentID = &id
		}
		fields = append(fields, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate extracted fields: %w", err)
	}
	return fields, nil
}
