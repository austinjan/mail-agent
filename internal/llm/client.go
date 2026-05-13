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
	Items []pumpData `json:"items"`
}

type pumpData struct {
	Item       int    `json:"Item"`
	CMH        string `json:"CMH"`
	M          string `json:"m"`
	RPM        string `json:"RPM"`
	Viscosity  string `json:"黏度"`
	Gravity    string `json:"比重"`
	SSVPLength string `json:"SSVP管長"`
	Casting    string `json:"機殼鑄造方式"`
	Evidence   string `json:"evidence_text"`
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
					"你是一個專業的工程助理。請從提供的郵件或附件文字中，找出所有「離心幫浦選型需求」。",
					"如果資料裡有多組需求，請輸出多個 items；如果沒有選型需求，items 請回空陣列。",
					"只擷取這 8 個欄位：Item, CMH, m, RPM, 黏度, 比重, SSVP管長, 機殼鑄造方式。",
					"CMH 是流量，可以包含單位，例如 120CMH 或 120 m3/h。",
					"m 是揚程，絕對不要包含單位，只填純數字。",
					"RPM 是轉速，絕對不要包含單位，只填純數字；若無請填空字串。",
					"黏度、比重、SSVP管長都絕對不要包含單位，只填純數字；若無請填 0。",
					"機殼鑄造方式是砂模鑄造、脫蠟鑄造等鑄造方式，不是材質或機型；若無請填空字串。",
					"數字小數點後最多三位，不要用無意義的 0 結尾。",
					"不要猜測。每個 item 都必須附 evidence_text，引用支撐該筆資料的原文。",
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
				"name":        "pump_selection_extraction",
				"description": "Centrifugal pump selection requirements extracted from mail or attachment text.",
				"strict":      true,
				"schema": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"items"},
					"properties": map[string]any{
						"items": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type":                 "object",
								"additionalProperties": false,
								"required": []string{
									"Item",
									"CMH",
									"m",
									"RPM",
									"黏度",
									"比重",
									"SSVP管長",
									"機殼鑄造方式",
									"evidence_text",
								},
								"properties": map[string]any{
									"Item":          map[string]any{"type": "integer"},
									"CMH":           map[string]any{"type": "string"},
									"m":             map[string]any{"type": "string"},
									"RPM":           map[string]any{"type": "string"},
									"黏度":            map[string]any{"type": "string"},
									"比重":            map[string]any{"type": "string"},
									"SSVP管長":        map[string]any{"type": "string"},
									"機殼鑄造方式":        map[string]any{"type": "string"},
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
	return pumpDataToFields(payload.Items), nil
}

func pumpDataToFields(items []pumpData) []ExtractedField {
	fields := make([]ExtractedField, 0, len(items)*8)
	for idx, item := range items {
		itemNo := item.Item
		if itemNo <= 0 {
			itemNo = idx + 1
		}
		evidence := strings.TrimSpace(item.Evidence)
		add := func(name, value string) {
			value = strings.TrimSpace(value)
			if value == "" {
				return
			}
			fields = append(fields, ExtractedField{
				FieldName:    fmt.Sprintf("%d.%s", itemNo, name),
				FieldValue:   value,
				Confidence:   0.9,
				EvidenceText: evidence,
			})
		}
		add("Item", fmt.Sprintf("%d", itemNo))
		add("CMH", item.CMH)
		add("m", item.M)
		add("RPM", item.RPM)
		add("黏度", defaultZero(item.Viscosity))
		add("比重", defaultZero(item.Gravity))
		add("SSVP管長", defaultZero(item.SSVPLength))
		add("機殼鑄造方式", item.Casting)
	}
	return fields
}

func defaultZero(s string) string {
	if strings.TrimSpace(s) == "" {
		return "0"
	}
	return s
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
