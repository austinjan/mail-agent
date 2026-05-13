package llm

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestExtractFieldsParsesStructuredResponse(t *testing.T) {
	client := NewClientForTest("key", "model", "https://example.test/responses", roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.Header.Get("Authorization"); got != "Bearer key" {
			t.Fatalf("Authorization: got %q", got)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), "json_schema") {
			t.Fatalf("request should use structured outputs schema: %s", body)
		}
		return &http.Response{
			StatusCode: 200,
			Body: io.NopCloser(strings.NewReader(`{
				"output": [{
					"content": [{
						"type": "output_text",
						"text": "{\"items\":[{\"Item\":1,\"CMH\":\"120CMH\",\"m\":\"45\",\"RPM\":\"1750\",\"黏度\":\"0\",\"比重\":\"1\",\"SSVP管長\":\"0\",\"機殼鑄造方式\":\"砂模鑄造\",\"evidence_text\":\"Flow 120CMH Head 45m\"}]}"
					}]
				}]
			}`)),
		}, nil
	}))

	fields, err := client.ExtractFields(t.Context(), "Flow 120 m3/h", "mail body")
	if err != nil {
		t.Fatalf("ExtractFields: %v", err)
	}
	if len(fields) == 0 {
		t.Fatal("fields should not be empty")
	}
	got := map[string]string{}
	for _, field := range fields {
		got[field.FieldName] = field.FieldValue
	}
	if got["1.CMH"] != "120CMH" || got["1.m"] != "45" || got["1.機殼鑄造方式"] != "砂模鑄造" {
		t.Fatalf("unexpected fields: %+v", got)
	}
}

func TestGeminiExtractFieldsParsesStructuredResponse(t *testing.T) {
	client := NewGeminiClientForTest("gemini-key", "gemini-2.5-flash", "https://example.test/gemini", roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.Header.Get("x-goog-api-key"); got != "gemini-key" {
			t.Fatalf("x-goog-api-key: got %q", got)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}
		bodyText := string(body)
		if !strings.Contains(bodyText, "responseJsonSchema") || !strings.Contains(bodyText, "responseMimeType") {
			t.Fatalf("gemini request should use structured outputs schema: %s", body)
		}
		return &http.Response{
			StatusCode: 200,
			Body: io.NopCloser(strings.NewReader(`{
				"candidates": [{
					"content": {
						"parts": [{
							"text": "{\"items\":[{\"Item\":1,\"CMH\":\"80CMH\",\"m\":\"30\",\"RPM\":\"\",\"黏度\":\"0\",\"比重\":\"1\",\"SSVP管長\":\"0\",\"機殼鑄造方式\":\"\",\"evidence_text\":\"80CMH 30m\"}]}"
						}]
					}
				}]
			}`)),
		}, nil
	}))

	fields, err := client.ExtractFields(t.Context(), "80CMH 30m", "mail body")
	if err != nil {
		t.Fatalf("ExtractFields: %v", err)
	}
	got := map[string]string{}
	for _, field := range fields {
		got[field.FieldName] = field.FieldValue
	}
	if got["1.CMH"] != "80CMH" || got["1.m"] != "30" {
		t.Fatalf("unexpected fields: %+v", got)
	}
}
