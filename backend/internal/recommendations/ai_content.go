package recommendations

import (
	"encoding/json"
	"errors"
	"strings"
)

var errParseRecommendationsJSON = errors.New("expected JSON object with recommendations[] or recommendations array")

// normalizeAIJSONContent strips optional markdown fences and surrounding prose from model output.
func normalizeAIJSONContent(content string) string {
	s := strings.TrimSpace(content)
	if s == "" {
		return s
	}
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```JSON")
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSpace(s)
		if idx := strings.LastIndex(s, "```"); idx >= 0 {
			s = strings.TrimSpace(s[:idx])
		}
	}
	startObj := strings.Index(s, "{")
	startArr := strings.Index(s, "[")
	start := -1
	if startObj >= 0 && (startArr < 0 || startObj < startArr) {
		start = startObj
	} else if startArr >= 0 {
		start = startArr
	}
	if start > 0 {
		s = s[start:]
	}
	endObj := strings.LastIndex(s, "}")
	endArr := strings.LastIndex(s, "]")
	end := -1
	if endObj >= 0 {
		end = endObj
	}
	if endArr > end {
		end = endArr
	}
	if end >= 0 && end < len(s)-1 {
		s = s[:end+1]
	}
	return strings.TrimSpace(s)
}

func parseCandidates(content string) ([]AIRecommendationCandidate, []map[string]any, error) {
	normalized := normalizeAIJSONContent(content)
	if normalized == "" {
		return nil, nil, errParseRecommendationsJSON
	}

	var envelope recommendationsEnvelope
	if err := json.Unmarshal([]byte(normalized), &envelope); err == nil && envelope.Recommendations != nil {
		raws := make([]map[string]any, 0, len(envelope.Recommendations))
		for _, item := range envelope.Recommendations {
			b, _ := json.Marshal(item)
			var raw map[string]any
			_ = json.Unmarshal(b, &raw)
			raws = append(raws, raw)
		}
		return envelope.Recommendations, raws, nil
	}

	var arr []AIRecommendationCandidate
	if err := json.Unmarshal([]byte(normalized), &arr); err == nil {
		raws := make([]map[string]any, 0, len(arr))
		for _, item := range arr {
			b, _ := json.Marshal(item)
			var raw map[string]any
			_ = json.Unmarshal(b, &raw)
			raws = append(raws, raw)
		}
		return arr, raws, nil
	}
	return nil, nil, errParseRecommendationsJSON
}
