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
						"text": "{\"fields\":[{\"field_name\":\"流量\",\"field_value\":\"120\",\"unit\":\"m3/h\",\"confidence\":0.91,\"evidence_text\":\"Flow 120 m3/h\"}]}"
					}]
				}]
			}`)),
		}, nil
	}))

	fields, err := client.ExtractFields(t.Context(), "Flow 120 m3/h", "mail body")
	if err != nil {
		t.Fatalf("ExtractFields: %v", err)
	}
	if len(fields) != 1 {
		t.Fatalf("fields: got %d want 1", len(fields))
	}
	if fields[0].FieldName != "流量" || fields[0].FieldValue != "120" || fields[0].Unit != "m3/h" {
		t.Fatalf("unexpected field: %+v", fields[0])
	}
}
