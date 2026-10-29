package chat

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	openAIKeyPattern  = regexp.MustCompile(`(?i)\bsk-[a-z0-9_-]+\b`)
	bearerPattern     = regexp.MustCompile(`(?i)\bbearer\s+[a-z0-9._~+/=-]+\b`)
	forbiddenTraceKey = map[string]struct{}{
		"api_key":          {},
		"apikey":           {},
		"openai_api_key":   {},
		"authorization":    {},
		"token":            {},
		"access_token":     {},
		"refresh_token":    {},
		"password":         {},
		"secret":           {},
		"cookie":           {},
		"session_token":    {},
		"bearer":           {},
		"raw_payload":      {},
		"raw_ozon_payload": {},
	}
)

func sanitizeTracePayload(payload map[string]any) map[string]any {
	if payload == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(payload))
	for key, value := range payload {
		if isForbiddenTraceKey(key) {
			out[key] = "[REDACTED]"
			continue
		}
		out[key] = sanitizeTraceValue(value)
	}
	return out
}

func sanitizeTraceValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		return sanitizeTracePayload(v)
	case []any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, sanitizeTraceValue(item))
		}
		return out
	case string:
		return sanitizeTraceString(v)
	case fmt.Stringer:
		return sanitizeTraceString(v.String())
	default:
		return value
	}
}

func sanitizeTraceString(s string) string {
	redacted := openAIKeyPattern.ReplaceAllString(s, "[REDACTED_OPENAI_KEY]")
	redacted = bearerPattern.ReplaceAllString(redacted, "Bearer [REDACTED]")
	lower := strings.ToLower(redacted)
	if strings.Contains(lower, "authorization: bearer") {
		return "authorization: Bearer [REDACTED]"
	}
	if strings.Contains(lower, "openai_api_key") {
		return "[REDACTED]"
	}
	return redacted
}

func isForbiddenTraceKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	_, ok := forbiddenTraceKey[normalized]
	return ok
}
