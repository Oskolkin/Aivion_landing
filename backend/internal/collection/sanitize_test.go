package collection

import (
	"strings"
	"testing"
)

func TestFingerprintIsDeterministicAndSanitized(t *testing.T) {
	identity := map[string]any{"page": 2, "Authorization": "Bearer test-secret", "X-API-Key": "also-nope", "nested": map[string]any{"access_token": "nope", "filter": "x"}}
	sanitized := SanitizeMetadata(identity)
	encoded := string(mustJSON(sanitized))
	if strings.Contains(encoded, "test-secret") || strings.Contains(encoded, "nope") || strings.Contains(encoded, "also-nope") {
		t.Fatalf("secret survived sanitization: %s", encoded)
	}
	a, err := RequestFingerprint(ProviderOzon, "product/list", identity)
	if err != nil {
		t.Fatal(err)
	}
	b, err := RequestFingerprint(ProviderOzon, "product/list", map[string]any{"nested": map[string]any{"filter": "x", "access_token": "different"}, "page": 2})
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("fingerprint changed with secret: %s != %s", a, b)
	}
}
func mustJSON(v any) []byte {
	b, err := canonicalJSON(v)
	if err != nil {
		panic(err)
	}
	return b
}
