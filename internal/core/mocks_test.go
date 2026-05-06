package core

import (
	"fmt"
	"time"

	"github.com/austinjan/mail-agent/internal/mail"
)

type mockSource struct {
	mails       []mail.Mail
	uidValidity uint32
	err         error
}

func (m *mockSource) Fetch(folder string, since time.Time) ([]mail.Mail, uint32, error) {
	return m.mails, m.uidValidity, m.err
}

type savedMail struct {
	ID   int64
	Mail mail.Mail
}

type mockStore struct {
	seenByUID     map[string]bool
	seenByMID     map[string]bool
	nextID        int64
	saved         []savedMail
	atts          map[int64][]mail.Attachment
	saveErr       error
	attachmentErr error
}

func newMockStore() *mockStore {
	return &mockStore{
		seenByUID: map[string]bool{},
		seenByMID: map[string]bool{},
		atts:      map[int64][]mail.Attachment{},
	}
}

func (s *mockStore) SaveMail(m mail.Mail) (int64, error) {
	if s.saveErr != nil {
		return 0, s.saveErr
	}
	s.nextID++
	s.saved = append(s.saved, savedMail{ID: s.nextID, Mail: m})
	s.seenByUID[key(m.UIDValidity, m.UID, m.Folder)] = true
	if m.MessageID != "" {
		s.seenByMID[m.MessageID] = true
	}
	return s.nextID, nil
}

func (s *mockStore) HasSeen(uidValidity uint32, uid uint32, folder string) (bool, error) {
	return s.seenByUID[key(uidValidity, uid, folder)], nil
}

func (s *mockStore) HasSeenByMessageID(messageID string) (bool, error) {
	if messageID == "" {
		return false, nil
	}
	return s.seenByMID[messageID], nil
}

func (s *mockStore) SaveAttachment(mailID int64, a mail.Attachment) error {
	if s.attachmentErr != nil {
		return s.attachmentErr
	}
	s.atts[mailID] = append(s.atts[mailID], a)
	return nil
}

func (s *mockStore) Close() error { return nil }

func key(uidValidity uint32, uid uint32, folder string) string {
	return fmt.Sprintf("%s:%d:%d", folder, uidValidity, uid)
}
