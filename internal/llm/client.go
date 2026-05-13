package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultResponsesURL = "https://api.openai.com/v1/responses"

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type Client struct {
	apiKey string
	model  string
	url    string
	doer   HTTPDoer
}

type ExtractedField struct {
	FieldName    string  `json:"field_name"`
	FieldValue   string  `json:"field_value"`
	Unit         string  `json:"unit"`
	Confidence   float64 `json:"confidence"`
	EvidenceText string  `json:"evidence_text"`
}

type extractionPayload struct {
	Fields []ExtractedField `json:"fields"`
}

func NewClient(apiKey, model string) *Client {
	return &Client{
		apiKey: strings.TrimSpace(apiKey),
		model:  strings.TrimSpace(model),
		url:    defaultResponsesURL,
		doer: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func NewClientForTest(apiKey, model, url string, doer HTTPDoer) *Client {
	c := NewClient(apiKey, model)
	c.url = url
	c.doer = doer
	return c
}

func (c *Client) ExtractFields(ctx context.Context, text, sourceLabel string) ([]ExtractedField, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("missing OpenAI API key")
	}
	if c.model == "" {
		return nil, fmt.Errorf("missing OpenAI model")
	}

	reqBody := map[string]any{
		"model": c.model,
		"input": []map[string]string{
			{
				"role": "system",
				"content": strings.Join([]string{
					"You extract pump and procurement related fields from email or attachment text.",
					"Return only fields that are explicitly supported by the provided text.",
					"Do not guess. If a value is absent, omit it.",
					"Each field must include concise evidence copied from the source text.",
					"Common target fields include 流量, 揚程, 材質, 型號, 數量, 品牌, 用途, 備註, but other useful requested business fields are allowed.",
				}, " "),
			},
			{
				"role":    "user",
				"content": fmt.Sprintf("Source: %s\n\nText:\n%s", sourceLabel, truncateForLLM(text, 18000)),
			},
		},
		"text": map[string]any{
			"format": map[string]any{
				"type":        "json_schema",
				"name":        "mail_field_extraction",
				"description": "Structured extraction results from mail or attachment text.",
				"strict":      true,
				"schema": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"fields"},
					"properties": map[string]any{
						"fields": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type":                 "object",
								"additionalProperties": false,
								"required": []string{
									"field_name",
									"field_value",
									"unit",
									"confidence",
									"evidence_text",
								},
								"properties": map[string]any{
									"field_name":  map[string]any{"type": "string"},
									"field_value": map[string]any{"type": "string"},
									"unit":        map[string]any{"type": "string"},
									"confidence": map[string]any{
										"type":    "number",
										"minimum": 0,
										"maximum": 1,
									},
									"evidence_text": map[string]any{"type": "string"},
								},
							},
						},
					},
				},
			},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal responses request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create responses request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call responses API: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read responses body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("responses API status %d: %s", resp.StatusCode, string(respBody))
	}

	out, err := responseOutputText(respBody)
	if err != nil {
		return nil, err
	}
	var payload extractionPayload
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		return nil, fmt.Errorf("parse extraction JSON: %w: %s", err, out)
	}
	return payload.Fields, nil
}

func responseOutputText(data []byte) (string, error) {
	var envelope struct {
		OutputText string `json:"output_text"`
		Output     []struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return "", fmt.Errorf("parse responses envelope: %w", err)
	}
	if envelope.OutputText != "" {
		return envelope.OutputText, nil
	}
	for _, output := range envelope.Output {
		for _, content := range output.Content {
			if content.Text != "" {
				return content.Text, nil
			}
		}
	}
	return "", fmt.Errorf("responses output did not contain text")
}

func truncateForLLM(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes])
}
