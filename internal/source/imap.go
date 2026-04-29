package source

import (
	"crypto/tls"
	"fmt"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"

	"github.com/austinjan/mail-agent/internal/mail"
)

type IMAPConfig struct {
	Host     string
	Port     int
	User     string
	Password string
}

type IMAPSource struct {
	cfg IMAPConfig
}

func NewIMAPSource(cfg IMAPConfig) *IMAPSource {
	return &IMAPSource{cfg: cfg}
}

// compile-time assertion: IMAPSource must satisfy Source.
var _ Source = (*IMAPSource)(nil)

func (s *IMAPSource) Fetch(folder string, since time.Time) ([]mail.Mail, uint32, error) {
	c, err := s.dial()
	if err != nil {
		return nil, 0, fmt.Errorf("imap dial: %w", err)
	}
	defer c.Close()

	if err := c.Login(s.cfg.User, s.cfg.Password).Wait(); err != nil {
		return nil, 0, fmt.Errorf("imap login: %w", err)
	}

	selectData, err := c.Select(folder, &imap.SelectOptions{ReadOnly: true}).Wait()
	if err != nil {
		return nil, 0, fmt.Errorf("imap select %q: %w", folder, err)
	}
	uidValidity := selectData.UIDValidity

	criteria := &imap.SearchCriteria{
		Since: since,
	}
	searchData, err := c.UIDSearch(criteria, nil).Wait()
	if err != nil {
		return nil, 0, fmt.Errorf("imap search: %w", err)
	}
	uids := searchData.AllUIDs()
	if len(uids) == 0 {
		return nil, uidValidity, nil
	}

	fetchOpts := &imap.FetchOptions{
		Flags: true,
		UID:   true,
		BodySection: []*imap.FetchItemBodySection{
			{Peek: true},
		},
	}
	fetchCmd := c.Fetch(imap.UIDSetNum(uids...), fetchOpts)
	defer fetchCmd.Close()

	msgs, err := fetchCmd.Collect()
	if err != nil {
		return nil, uidValidity, fmt.Errorf("imap fetch: %w", err)
	}

	mails := make([]mail.Mail, 0, len(msgs))
	for _, fm := range msgs {
		if len(fm.BodySection) == 0 {
			continue
		}
		parsed, err := parseRFC822(fm.BodySection[0].Bytes)
		if err != nil {
			continue
		}
		parsed.UIDValidity = uidValidity
		parsed.UID = uint32(fm.UID)
		parsed.Folder = folder
		parsed.Flags = flagsToStrings(fm.Flags)
		mails = append(mails, parsed)
	}
	return mails, uidValidity, nil
}

func flagsToStrings(flags []imap.Flag) []string {
	out := make([]string, len(flags))
	for i, f := range flags {
		out[i] = string(f)
	}
	return out
}

func (s *IMAPSource) dial() (*imapclient.Client, error) {
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	return imapclient.DialTLS(addr, &imapclient.Options{
		TLSConfig: &tls.Config{ServerName: s.cfg.Host},
	})
}
