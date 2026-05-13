package extract

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/austinjan/mail-agent/internal/store"
)

type Pipeline struct {
	store  *store.SqliteStore
	logger *slog.Logger
}

type RunStats struct {
	Processed   int
	Done        int
	Unsupported int
	Failed      int
	Fields      int
}

func New(st *store.SqliteStore, logger *slog.Logger) *Pipeline {
	if logger == nil {
		logger = slog.Default()
	}
	return &Pipeline{store: st, logger: logger}
}

func (p *Pipeline) Run(limit int) (RunStats, error) {
	var stats RunStats
	jobs, err := p.store.PendingExtractionJobs(limit)
	if err != nil {
		return stats, err
	}

	for _, job := range jobs {
		stats.Processed++
		if err := p.store.MarkExtractionJobRunning(job.ID); err != nil {
			stats.Failed++
			p.logger.Error("", "event", "extract_job_mark_running_failed", "job_id", job.ID, "error", err.Error())
			continue
		}

		text, label, err := p.textForJob(job)
		if err != nil {
			if errors.Is(err, ErrUnsupported) {
				stats.Unsupported++
				_ = p.store.MarkExtractionJobUnsupported(job.ID, err.Error())
				p.logger.Info("", "event", "extract_job_unsupported", "job_id", job.ID, "mail_id", job.MailID, "error", err.Error())
				continue
			}
			stats.Failed++
			_ = p.store.MarkExtractionJobFailed(job.ID, err.Error())
			p.logger.Error("", "event", "extract_job_failed", "job_id", job.ID, "mail_id", job.MailID, "error", err.Error())
			continue
		}

		fields := ExtractFields(text, job, label)
		if err := p.store.CompleteExtractionJob(job.ID, fields); err != nil {
			stats.Failed++
			p.logger.Error("", "event", "extract_job_complete_failed", "job_id", job.ID, "mail_id", job.MailID, "error", err.Error())
			continue
		}
		stats.Done++
		stats.Fields += len(fields)
		p.logger.Info("", "event", "extract_job_done", "job_id", job.ID, "mail_id", job.MailID, "source_type", job.SourceType, "fields", len(fields))
	}

	p.logger.Info("", "event", "extract_done", "processed", stats.Processed, "done", stats.Done, "unsupported", stats.Unsupported, "failed", stats.Failed, "fields", stats.Fields)
	return stats, nil
}

func (p *Pipeline) textForJob(job store.ExtractionJob) (string, string, error) {
	switch job.SourceType {
	case "body":
		text := job.BodyText
		if job.BodyHTML != "" {
			text += "\n" + HTMLToText(job.BodyHTML)
		}
		if normalizeWhitespace(text) == "" {
			return "", "mail body", fmt.Errorf("%w: empty mail body", ErrUnsupported)
		}
		return text, "mail body", nil
	case "attachment":
		path := p.store.AttachmentFullPath(job.AttachmentPath)
		data, err := os.ReadFile(path)
		if err != nil {
			return "", job.AttachmentFilename, fmt.Errorf("read attachment %q: %w", path, err)
		}
		text, err := AttachmentText(job.AttachmentFilename, job.AttachmentContentType, data)
		if err != nil {
			return "", job.AttachmentFilename, err
		}
		return text, job.AttachmentFilename, nil
	default:
		return "", "", fmt.Errorf("%w: source type %q", ErrUnsupported, job.SourceType)
	}
}
