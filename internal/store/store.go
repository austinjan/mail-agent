// Package store persists fetched mails and attachments.
// The Store interface decouples the core pipeline from any
// particular backend; MVP uses SqliteStore.
package store

import "github.com/austinjan/mail-agent/internal/mail"

type Store interface {
	SaveMail(m mail.Mail) (mailID int64, err error)
	HasSeen(uidValidity uint32, uid uint32, folder string) (bool, error)
	HasSeenByMessageID(messageID string) (bool, error)
	SaveAttachment(mailID int64, a mail.Attachment) error
	Close() error
}
