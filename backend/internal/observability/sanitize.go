package observability

import (
	"strings"
)

func SanitizeTraceValue(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	lower := strings.ToLower(s)
	if strings.Contains(lower, "authorization") || strings.Contains(lower, "bearer ") || strings.Contains(lower, "sk-") {
		return "[redacted]"
	}
	if len(s) > 256 {
		return s[:256]
	}
	return s
}

func IsSensitiveAttrKey(key string) bool {
	k := strings.ToLower(key)
	return strings.Contains(k, "prompt") || strings.Contains(k, "output_body") ||
		strings.Contains(k, "authorization") || strings.Contains(k, "api_key") ||
		strings.Contains(k, "tool_json")
}
