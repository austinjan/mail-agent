package source

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	netmail "net/mail"
	"strings"

	"github.com/austinjan/mail-agent/internal/mail"
)

// parseRFC822 converts a raw RFC 5322 message into mail.Mail.
// It does not populate UID / UIDValidity / Folder / Flags — those
// come from IMAP and are filled in by the caller.
func parseRFC822(raw []byte) (mail.Mail, error) {
	msg, err := netmail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return mail.Mail{}, fmt.Errorf("ReadMessage: %w", err)
	}

	dec := new(mime.WordDecoder)
	decodeHdr := func(v string) string {
		out, err := dec.DecodeHeader(v)
		if err != nil {
			return v
		}
		return out
	}

	m := mail.Mail{
		MessageID:  strings.TrimSpace(msg.Header.Get("Message-ID")),
		Subject:    decodeHdr(msg.Header.Get("Subject")),
		From:       decodeHdr(msg.Header.Get("From")),
		ToAddrs:    splitAddrs(msg.Header.Get("To"), decodeHdr),
		CCAddrs:    splitAddrs(msg.Header.Get("Cc"), decodeHdr),
		ReplyTo:    decodeHdr(msg.Header.Get("Reply-To")),
		InReplyTo:  strings.TrimSpace(msg.Header.Get("In-Reply-To")),
		Refs:       strings.Fields(msg.Header.Get("References")),
		RawHeaders: extractHeadersBlob(raw),
	}

	if d, err := netmail.ParseDate(msg.Header.Get("Date")); err == nil {
		m.ReceivedAt = d.UTC()
	}

	ct := msg.Header.Get("Content-Type")
	if ct == "" {
		ct = "text/plain"
	}
	mediaType, params, err := mime.ParseMediaType(ct)
	if err != nil {
		return mail.Mail{}, fmt.Errorf("parse Content-Type: %w", err)
	}

	if strings.HasPrefix(mediaType, "multipart/") {
		if err := walkMultipart(msg.Body, params["boundary"], &m); err != nil {
			return mail.Mail{}, err
		}
	} else {
		body, err := io.ReadAll(msg.Body)
		if err != nil {
			return mail.Mail{}, fmt.Errorf("read body: %w", err)
		}
		assignBodyPart(&m, mediaType, body, "", "")
	}
	return m, nil
}

func walkMultipart(body io.Reader, boundary string, m *mail.Mail) error {
	mr := multipart.NewReader(body, boundary)
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("multipart: %w", err)
		}
		partBody, err := io.ReadAll(part)
		if err != nil {
			return fmt.Errorf("read part: %w", err)
		}
		partCT := part.Header.Get("Content-Type")
		if partCT == "" {
			partCT = "text/plain"
		}
		mediaType, params, err := mime.ParseMediaType(partCT)
		if err != nil {
			mediaType = "application/octet-stream"
			params = nil
		}
		if strings.HasPrefix(mediaType, "multipart/") {
			if err := walkMultipart(bytes.NewReader(partBody), params["boundary"], m); err != nil {
				return err
			}
			continue
		}
		disposition := part.Header.Get("Content-Disposition")
		filename := ""
		if _, dparams, err := mime.ParseMediaType(disposition); err == nil {
			filename = dparams["filename"]
		}
		if filename == "" {
			filename = params["name"]
		}
		decoded := decodePart(part.Header.Get("Content-Transfer-Encoding"), partBody)
		assignBodyPart(m, mediaType, decoded, filename, partCT)
	}
}

func assignBodyPart(m *mail.Mail, mediaType string, body []byte, filename, rawCT string) {
	switch {
	case filename != "":
		m.Attachments = append(m.Attachments, mail.Attachment{
			Filename:    filename,
			ContentType: rawCT,
			Content:     body,
		})
	case mediaType == "text/plain" && m.BodyText == "":
		m.BodyText = string(body)
	case mediaType == "text/html" && m.BodyHTML == "":
		m.BodyHTML = string(body)
	default:
		m.Attachments = append(m.Attachments, mail.Attachment{
			ContentType: mediaType,
			Content:     body,
		})
	}
}

func decodePart(encoding string, body []byte) []byte {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "base64":
		dec, err := base64Decode(body)
		if err != nil {
			return body
		}
		return dec
	case "quoted-printable":
		dec, err := qpDecode(body)
		if err != nil {
			return body
		}
		return dec
	default:
		return body
	}
}

func base64Decode(b []byte) ([]byte, error) {
	clean := bytes.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, b)
	return base64.StdEncoding.DecodeString(string(clean))
}

func qpDecode(b []byte) ([]byte, error) {
	r := quotedprintable.NewReader(bytes.NewReader(b))
	return io.ReadAll(r)
}

func splitAddrs(v string, decodeHdr func(string) string) []string {
	v = strings.TrimSpace(v)
	if v == "" {
		return []string{}
	}
	addrs, err := netmail.ParseAddressList(v)
	if err != nil {
		return []string{decodeHdr(v)}
	}
	out := make([]string, 0, len(addrs))
	for _, a := range addrs {
		out = append(out, a.Address)
	}
	return out
}

func extractHeadersBlob(raw []byte) string {
	if i := bytes.Index(raw, []byte("\r\n\r\n")); i >= 0 {
		return string(raw[:i])
	}
	if i := bytes.Index(raw, []byte("\n\n")); i >= 0 {
		return string(raw[:i])
	}
	return string(raw)
}
