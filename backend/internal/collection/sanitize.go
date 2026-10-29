package collection

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
)

var secretKeys = map[string]struct{}{
	"authorization": {}, "proxy-authorization": {}, "cookie": {}, "set-cookie": {}, "api-key": {}, "apikey": {}, "client-secret": {}, "client_secret": {},
	"oauth": {}, "token": {}, "access_token": {}, "refresh_token": {}, "secret": {}, "password": {},
}

// SanitizeMetadata recursively drops credential-bearing keys before any
// metadata is persisted or used as fingerprint input.
func SanitizeMetadata(input map[string]any) map[string]any {
	out := make(map[string]any, len(input))
	for key, value := range input {
		if isSecretKey(key) {
			continue
		}
		switch child := value.(type) {
		case map[string]any:
			out[key] = SanitizeMetadata(child)
		case []any:
			items := make([]any, len(child))
			for i, item := range child {
				if nested, ok := item.(map[string]any); ok {
					items[i] = SanitizeMetadata(nested)
				} else {
					items[i] = item
				}
			}
			out[key] = items
		default:
			out[key] = value
		}
	}
	return out
}

func isSecretKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	if _, forbidden := secretKeys[normalized]; forbidden {
		return true
	}
	for _, marker := range []string{"authorization", "cookie", "api-key", "apikey", "token", "secret", "password"} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

// RequestFingerprint is a SHA-256 over canonical JSON of a sanitized request identity.
func RequestFingerprint(provider Provider, endpointKey string, identity map[string]any) (string, error) {
	canonical, err := canonicalJSON(map[string]any{"provider": provider, "endpoint_key": endpointKey, "identity": SanitizeMetadata(identity)})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}

func canonicalJSON(value any) ([]byte, error) {
	return json.Marshal(normalize(value))
}
func normalize(value any) any {
	switch v := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := make(map[string]any, len(v))
		for _, k := range keys {
			out[k] = normalize(v[k])
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i := range v {
			out[i] = normalize(v[i])
		}
		return out
	default:
		return value
	}
}
