package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/austinjan/mail-agent/internal/store"
)

func TestVersionStringNotEmpty(t *testing.T) {
	if versionString() == "" {
		t.Fatal("versionString should never return empty")
	}
}

func TestWriteExtractedFieldsCSV(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.csv")
	err := writeExtractedFieldsCSV(path, []store.ExtractedField{
		{
			MailID:       1,
			JobID:        2,
			FieldName:    "流量",
			FieldValue:   "120",
			Unit:         "m3/h",
			Confidence:   0.8,
			EvidenceText: "Flow 120 m3/h",
			SourceType:   "body",
			SourceLabel:  "mail body",
			CreatedAt:    time.Date(2026, 5, 13, 1, 2, 3, 0, time.UTC),
		},
	})
	if err != nil {
		t.Fatalf("writeExtractedFieldsCSV: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}) {
		t.Fatal("csv should start with UTF-8 BOM for Excel")
	}
	for _, want := range [][]byte{[]byte("field_name"), []byte("流量"), []byte("Flow 120 m3/h")} {
		if !bytes.Contains(data, want) {
			t.Fatalf("csv missing %q:\n%s", want, data)
		}
	}
}
