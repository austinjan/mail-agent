package extract

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

var ErrUnsupported = errors.New("unsupported attachment format")

func AttachmentText(filename, contentType string, data []byte) (string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	contentType = strings.ToLower(contentType)

	switch {
	case strings.HasPrefix(contentType, "text/") || ext == ".txt" || ext == ".csv" || ext == ".md":
		return string(data), nil
	case contentType == "text/html" || ext == ".html" || ext == ".htm":
		return HTMLToText(string(data)), nil
	case strings.Contains(contentType, "wordprocessingml") || ext == ".docx":
		return docxText(data)
	case strings.Contains(contentType, "spreadsheetml") || ext == ".xlsx":
		return xlsxText(data)
	case contentType == "application/pdf" || ext == ".pdf":
		return pdfText(data)
	case isImageContent(contentType, ext):
		return "", fmt.Errorf("%w: image attachments require OCR, which is disabled", ErrUnsupported)
	default:
		return "", fmt.Errorf("%w: %s %s", ErrUnsupported, contentType, ext)
	}
}

func HTMLToText(input string) string {
	scriptRe := regexp.MustCompile(`(?is)<script.*?</script>`)
	styleRe := regexp.MustCompile(`(?is)<style.*?</style>`)
	cleaned := scriptRe.ReplaceAllString(input, " ")
	cleaned = styleRe.ReplaceAllString(cleaned, " ")
	tagRe := regexp.MustCompile(`(?s)<[^>]+>`)
	cleaned = tagRe.ReplaceAllString(cleaned, " ")
	return normalizeWhitespace(html.UnescapeString(cleaned))
}

func docxText(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("open docx: %w", err)
	}
	for _, f := range zr.File {
		if f.Name != "word/document.xml" {
			continue
		}
		raw, err := readZipFile(f)
		if err != nil {
			return "", err
		}
		return xmlText(raw), nil
	}
	return "", fmt.Errorf("%w: docx missing word/document.xml", ErrUnsupported)
}

func xlsxText(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("open xlsx: %w", err)
	}

	var parts []string
	for _, f := range zr.File {
		if f.Name == "xl/sharedStrings.xml" || strings.HasPrefix(f.Name, "xl/worksheets/") {
			raw, err := readZipFile(f)
			if err != nil {
				return "", err
			}
			if text := xmlText(raw); text != "" {
				parts = append(parts, text)
			}
		}
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("%w: xlsx has no readable worksheet text", ErrUnsupported)
	}
	return strings.Join(parts, "\n"), nil
}

func pdfText(data []byte) (string, error) {
	// This intentionally only handles embedded text strings. Scanned/image-only
	// PDFs remain unsupported because OCR is disabled for this project.
	re := regexp.MustCompile(`\((?:\\.|[^\\)])*\)`)
	matches := re.FindAll(data, -1)
	var parts []string
	for _, m := range matches {
		text := decodePDFLiteral(string(m[1 : len(m)-1]))
		if readableText(text) {
			parts = append(parts, text)
		}
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("%w: pdf has no extractable text layer", ErrUnsupported)
	}
	return normalizeWhitespace(strings.Join(parts, "\n")), nil
}

func readZipFile(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("open zip member %q: %w", f.Name, err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("read zip member %q: %w", f.Name, err)
	}
	return data, nil
}

func xmlText(data []byte) string {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var parts []string
	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return normalizeWhitespace(stripXMLTags(string(data)))
		}
		if chars, ok := tok.(xml.CharData); ok {
			if text := normalizeWhitespace(string(chars)); text != "" {
				parts = append(parts, text)
			}
		}
	}
	return normalizeWhitespace(strings.Join(parts, " "))
}

func stripXMLTags(s string) string {
	re := regexp.MustCompile(`(?s)<[^>]+>`)
	return html.UnescapeString(re.ReplaceAllString(s, " "))
}

func decodePDFLiteral(s string) string {
	replacer := strings.NewReplacer(`\(`, "(", `\)`, ")", `\\`, `\`, `\n`, "\n", `\r`, "\r", `\t`, "\t")
	return replacer.Replace(s)
}

func readableText(s string) bool {
	letters := 0
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			letters++
		}
	}
	return letters >= 2
}

func isImageContent(contentType, ext string) bool {
	if strings.HasPrefix(contentType, "image/") {
		return true
	}
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tif", ".tiff", ".webp":
		return true
	default:
		return false
	}
}

func normalizeWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
