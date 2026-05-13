package extract

import "testing"

func TestHTMLToText(t *testing.T) {
	got := HTMLToText(`<html><body><p>Flow <b>120 m3/h</b></p><script>ignore()</script></body></html>`)
	if got != "Flow 120 m3/h" {
		t.Fatalf("HTMLToText: got %q", got)
	}
}

func TestAttachmentTextRejectsImagesWithoutOCR(t *testing.T) {
	_, err := AttachmentText("scan.png", "image/png", []byte("fake"))
	if err == nil {
		t.Fatal("expected image attachment to be unsupported")
	}
}

func TestPDFTextExtractsLiteralStrings(t *testing.T) {
	got, err := AttachmentText("spec.pdf", "application/pdf", []byte(`1 0 obj <<>> stream (Flow 120 m3/h) Tj endstream`))
	if err != nil {
		t.Fatalf("AttachmentText pdf: %v", err)
	}
	if got != "Flow 120 m3/h" {
		t.Fatalf("pdf text: got %q", got)
	}
}
