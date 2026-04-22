// Package mail defines the shared data types that flow between
// sources (e.g. IMAP) and stores (e.g. SQLite). It contains no IO.
package mail

import "time"

// Mail is one fetched message, fully materialised in memory.
type Mail struct {
	UIDValidity uint32
	UID         uint32
	Folder      string

	MessageID string
	Subject   string
	From      string
	ToAddrs   []string
	CCAddrs   []string
	ReplyTo   string
	InReplyTo string
	Refs      []string
	Flags     []string

	ReceivedAt time.Time

	BodyText   string
	BodyHTML   string
	RawHeaders string

	Attachments []Attachment
}

// Attachment is one MIME part treated as a file.
// Content holds the decoded bytes; the store layer is responsible
// for hashing and writing them to disk.
type Attachment struct {
	Filename    string
	ContentType string
	Content     []byte
}
