package extract

import (
	"regexp"
	"strings"
	"time"

	"github.com/austinjan/mail-agent/internal/store"
)

type fieldSpec struct {
	Name     string
	Aliases  []string
	Patterns []*regexp.Regexp
}

var fieldSpecs = []fieldSpec{
	{
		Name:    "流量",
		Aliases: []string{"流量", "flow", "capacity", "q"},
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*(m3/h|m³/h|m\^3/h|lpm|gpm|cmd|cms)`),
		},
	},
	{
		Name:    "揚程",
		Aliases: []string{"揚程", "head", "tdh"},
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*(m|meter|metre|ft)\b`),
		},
	},
	{
		Name:    "材質",
		Aliases: []string{"材質", "material", "casing", "impeller"},
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)\b(SS\s*316|SUS\s*316|SS\s*304|SUS\s*304|stainless steel|cast iron|ductile iron|bronze|PVC|PVDF|PTFE)\b`),
			regexp.MustCompile(`(不鏽鋼|鑄鐵|球墨鑄鐵|青銅|塑膠|工程塑膠)`),
		},
	},
	{
		Name:    "型號",
		Aliases: []string{"型號", "model", "type"},
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(?:model(?:\s*(?:no\.?|number|#))?|type|型號)\s*[:：#]\s*([A-Z0-9][A-Z0-9._/-]{2,})`),
		},
	},
	{
		Name:    "數量",
		Aliases: []string{"數量", "quantity", "qty"},
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(?:qty|quantity|數量)\s*[:：]?\s*(\d+(?:\.\d+)?)\s*(sets|set|pcs|台|組|個)?`),
		},
	},
	{
		Name:    "品牌",
		Aliases: []string{"品牌", "brand", "maker", "manufacturer"},
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(?:brand|maker|manufacturer|品牌)\s*[:：]\s*([A-Z][A-Za-z0-9._ -]{1,40})`),
		},
	},
	{
		Name:    "用途",
		Aliases: []string{"用途", "application", "use"},
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(?:application|用途|use)\s*[:：]\s*([^。;\n\r]{2,80})`),
		},
	},
}

func ExtractFields(text string, job store.ExtractionJob, sourceLabel string) []store.ExtractedField {
	chunks := splitChunks(text)
	seen := map[string]bool{}
	var fields []store.ExtractedField
	for _, spec := range fieldSpecs {
		for _, chunk := range chunks {
			if !chunkMatches(spec, chunk) {
				continue
			}
			value, unit := extractValue(spec, chunk)
			if value == "" {
				continue
			}
			key := spec.Name + "\x00" + value + "\x00" + unit
			if seen[key] {
				continue
			}
			seen[key] = true
			fields = append(fields, store.ExtractedField{
				MailID:       job.MailID,
				AttachmentID: job.AttachmentID,
				FieldName:    spec.Name,
				FieldValue:   value,
				Unit:         unit,
				Confidence:   0.72,
				EvidenceText: truncateEvidence(chunk),
				SourceType:   job.SourceType,
				SourceLabel:  sourceLabel,
				CreatedAt:    time.Now().UTC(),
			})
		}
	}
	return fields
}

func chunkMatches(spec fieldSpec, chunk string) bool {
	lower := strings.ToLower(chunk)
	for _, alias := range spec.Aliases {
		if strings.Contains(lower, strings.ToLower(alias)) {
			return true
		}
	}
	return false
}

func extractValue(spec fieldSpec, chunk string) (string, string) {
	for _, pattern := range spec.Patterns {
		match := pattern.FindStringSubmatch(chunk)
		if len(match) == 0 {
			continue
		}
		if len(match) >= 3 {
			return cleanExtractedValue(match[1]), cleanExtractedValue(match[2])
		}
		return cleanExtractedValue(match[1]), ""
	}
	return "", ""
}

func cleanExtractedValue(s string) string {
	return strings.Trim(strings.TrimSpace(s), ".,;，。；")
}

func splitChunks(text string) []string {
	splitter := regexp.MustCompile(`[\r\n。；;]+`)
	raw := splitter.Split(text, -1)
	var chunks []string
	for _, part := range raw {
		part = normalizeWhitespace(part)
		if part == "" {
			continue
		}
		if len([]rune(part)) > 280 {
			chunks = append(chunks, windowChunks(part, 240)...)
			continue
		}
		chunks = append(chunks, part)
	}
	return chunks
}

func windowChunks(text string, size int) []string {
	runes := []rune(text)
	var out []string
	for start := 0; start < len(runes); start += size {
		end := start + size
		if end > len(runes) {
			end = len(runes)
		}
		out = append(out, string(runes[start:end]))
	}
	return out
}

func truncateEvidence(s string) string {
	runes := []rune(s)
	if len(runes) <= 300 {
		return s
	}
	return string(runes[:300])
}
