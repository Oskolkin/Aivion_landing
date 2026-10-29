package aijsonrepair

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var (
	markdownJSONFencePattern = regexp.MustCompile("(?is)```(?:json)?\\s*([\\s\\S]*?)```")
	trailingCommaPattern     = regexp.MustCompile(`,\s*([}\]])`)
)

// RepairRawJSON extracts and fixes common LLM JSON formatting issues.
func RepairRawJSON(raw string) (string, []string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil, fmt.Errorf("empty json")
	}
	repairs := make([]string, 0, 4)

	if match := markdownJSONFencePattern.FindStringSubmatch(raw); len(match) > 1 {
		raw = strings.TrimSpace(match[1])
		repairs = append(repairs, "json_extracted_from_markdown_fence")
	}

	if !strings.HasPrefix(raw, "{") {
		start := strings.Index(raw, "{")
		end := strings.LastIndex(raw, "}")
		if start >= 0 && end > start {
			raw = raw[start : end+1]
			repairs = append(repairs, "json_extracted_from_surrounding_text")
		}
	}

	for pass := 0; pass < 3; pass++ {
		if json.Valid([]byte(raw)) {
			return raw, repairs, nil
		}
		fixed := trailingCommaPattern.ReplaceAllString(raw, "$1")
		if fixed == raw {
			break
		}
		raw = fixed
		repairs = append(repairs, "json_removed_trailing_commas")
	}

	if !json.Valid([]byte(raw)) {
		return raw, repairs, fmt.Errorf("invalid_json_after_repair")
	}
	return raw, repairs, nil
}
