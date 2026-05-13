package extract

import (
	"testing"

	"github.com/austinjan/mail-agent/internal/store"
)

func TestExtractFieldsFindsPumpSpecs(t *testing.T) {
	text := `Pump capacity shall be 120 m3/h at 45m TDH.
Casing material is SS316.
Model: PUMP-200.
Application: cooling water.
Qty: 2 sets.`
	fields := ExtractFields(text, store.ExtractionJob{MailID: 1, SourceType: "body"}, "mail body")
	got := map[string]string{}
	for _, f := range fields {
		got[f.FieldName] = f.FieldValue + " " + f.Unit
		if f.EvidenceText == "" {
			t.Fatalf("missing evidence for %s", f.FieldName)
		}
	}
	if got["流量"] != "120 m3/h" {
		t.Fatalf("flow: got %q", got["流量"])
	}
	if got["揚程"] != "45 m" {
		t.Fatalf("head: got %q", got["揚程"])
	}
	if got["材質"] != "SS316 " {
		t.Fatalf("material: got %q", got["材質"])
	}
	if got["數量"] != "2 sets" {
		t.Fatalf("quantity: got %q", got["數量"])
	}
	if got["型號"] != "PUMP-200 " {
		t.Fatalf("model: got %q", got["型號"])
	}
	if got["用途"] != "cooling water " {
		t.Fatalf("application: got %q", got["用途"])
	}
}

func TestExtractFieldsAvoidsGenericModelAndApplicationWords(t *testing.T) {
	text := `The Gemini Flash model will be discontinued.
No changes to application logic are required for continued operation.`
	fields := ExtractFields(text, store.ExtractionJob{MailID: 1, SourceType: "body"}, "mail body")
	if len(fields) != 0 {
		t.Fatalf("generic notification should not produce fields: %+v", fields)
	}
}
