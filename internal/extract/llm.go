package extract

import (
	"context"
	"time"

	"github.com/austinjan/mail-agent/internal/llm"
	"github.com/austinjan/mail-agent/internal/store"
)

type Extractor interface {
	Extract(ctx context.Context, text string, job store.ExtractionJob, sourceLabel string) ([]store.ExtractedField, error)
}

type RuleExtractor struct{}

func (RuleExtractor) Extract(_ context.Context, text string, job store.ExtractionJob, sourceLabel string) ([]store.ExtractedField, error) {
	return ExtractFields(text, job, sourceLabel), nil
}

type LLMExtractor struct {
	client *llm.Client
}

func NewLLMExtractor(client *llm.Client) *LLMExtractor {
	return &LLMExtractor{client: client}
}

func (e *LLMExtractor) Extract(ctx context.Context, text string, job store.ExtractionJob, sourceLabel string) ([]store.ExtractedField, error) {
	results, err := e.client.ExtractFields(ctx, text, sourceLabel)
	if err != nil {
		return nil, err
	}
	fields := make([]store.ExtractedField, 0, len(results))
	now := time.Now().UTC()
	for _, result := range results {
		if result.FieldName == "" || result.FieldValue == "" || result.EvidenceText == "" {
			continue
		}
		fields = append(fields, store.ExtractedField{
			MailID:       job.MailID,
			AttachmentID: job.AttachmentID,
			FieldName:    result.FieldName,
			FieldValue:   result.FieldValue,
			Unit:         result.Unit,
			Confidence:   result.Confidence,
			EvidenceText: result.EvidenceText,
			SourceType:   job.SourceType,
			SourceLabel:  sourceLabel,
			CreatedAt:    now,
		})
	}
	return fields, nil
}
