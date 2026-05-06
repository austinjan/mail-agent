// Package core runs the fetch, dedup, persist, and log pipeline.
package core

import (
	"errors"
	"log/slog"
	"time"

	"github.com/austinjan/mail-agent/internal/mail"
	"github.com/austinjan/mail-agent/internal/source"
	"github.com/austinjan/mail-agent/internal/store"
)

type Stats struct {
	Fetched          int
	Saved            int
	SkippedDedup     int
	AttachmentsSaved int
	Errors           int
}

type Pipeline struct {
	src    source.Source
	store  store.Store
	logger *slog.Logger
}

func New(src source.Source, st store.Store, logger *slog.Logger) *Pipeline {
	if logger == nil {
		logger = slog.Default()
	}
	return &Pipeline{src: src, store: st, logger: logger}
}

func (p *Pipeline) Run(folder string, since time.Time) (Stats, error) {
	var stats Stats
	p.logger.Info("", "event", "fetch_start", "folder", folder, "since", since.UTC().Format(time.RFC3339))

	mails, uidValidity, err := p.src.Fetch(folder, since)
	if err != nil {
		p.logger.Error("", "event", "fetch_failed", "folder", folder, "error", err.Error())
		return stats, err
	}
	stats.Fetched = len(mails)

	for _, m := range mails {
		if p.seen(m, &stats) {
			continue
		}

		mailID, err := p.store.SaveMail(m)
		if err != nil {
			if errors.Is(err, store.ErrAlreadyExists) {
				stats.SkippedDedup++
				p.logger.Info("", "event", "mail_skipped_dedup", "uid", m.UID, "message_id", m.MessageID, "reason", "already_exists")
				continue
			}
			stats.Errors++
			p.logger.Error("", "event", "mail_save_failed", "uid", m.UID, "message_id", m.MessageID, "error", err.Error())
			continue
		}

		stats.Saved++
		p.logger.Info("", "event", "mail_saved", "uid", m.UID, "message_id", m.MessageID, "subject", m.Subject, "mail_id", mailID)
		p.saveAttachments(mailID, m.Attachments, &stats)
	}

	p.logger.Info("", "event", "fetch_done",
		"uid_validity", uidValidity,
		"fetched", stats.Fetched,
		"saved", stats.Saved,
		"skipped", stats.SkippedDedup,
		"attachments", stats.AttachmentsSaved,
		"errors", stats.Errors,
	)
	return stats, nil
}

func (p *Pipeline) seen(m mail.Mail, stats *Stats) bool {
	seen, err := p.store.HasSeen(m.UIDValidity, m.UID, m.Folder)
	if err != nil {
		stats.Errors++
		p.logger.Error("", "event", "dedup_check_failed", "uid", m.UID, "message_id", m.MessageID, "error", err.Error())
		return true
	}
	if seen {
		stats.SkippedDedup++
		p.logger.Info("", "event", "mail_skipped_dedup", "uid", m.UID, "message_id", m.MessageID, "reason", "uid")
		return true
	}

	if m.MessageID == "" {
		return false
	}
	seen, err = p.store.HasSeenByMessageID(m.MessageID)
	if err != nil {
		stats.Errors++
		p.logger.Error("", "event", "dedup_check_failed", "uid", m.UID, "message_id", m.MessageID, "error", err.Error())
		return true
	}
	if seen {
		stats.SkippedDedup++
		p.logger.Info("", "event", "mail_skipped_dedup", "uid", m.UID, "message_id", m.MessageID, "reason", "message_id")
		return true
	}
	return false
}

func (p *Pipeline) saveAttachments(mailID int64, attachments []mail.Attachment, stats *Stats) {
	for _, a := range attachments {
		if err := p.store.SaveAttachment(mailID, a); err != nil {
			stats.Errors++
			p.logger.Error("", "event", "attachment_save_failed", "mail_id", mailID, "filename", a.Filename, "error", err.Error())
			continue
		}
		stats.AttachmentsSaved++
		p.logger.Info("", "event", "attachment_saved", "mail_id", mailID, "filename", a.Filename)
	}
}
