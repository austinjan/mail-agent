// Package source fetches mails from a mail provider.
// The Source interface decouples the pipeline from any specific protocol;
// MVP uses IMAPSource.
package source

import (
	"time"

	"github.com/austinjan/mail-agent/internal/mail"
)

type Source interface {
	// Fetch returns all mails received at or after `since`.
	// Also reports the folder's current UIDVALIDITY — callers
	// use this to detect mailbox rebuilds.
	Fetch(folder string, since time.Time) (mails []mail.Mail, uidValidity uint32, err error)
}
