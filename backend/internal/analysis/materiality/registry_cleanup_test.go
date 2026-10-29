package materiality

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
)

func TestSupersededPolicyV1CanonicalHash(t *testing.T) {
	payload := map[string]string{
		"version": "MATERIALITY_POLICY_V1",
		"status":  "SUPERSEDED_BY_V1_1",
		"note":    "absolute soft-diversity pilot; do not mutate",
	}
	const want = "712a8c89f6687b118080fc0d9e182782483087a6ad09559fe99241262ab0f22a"
	got := canonicaljson.Hash(payload)
	if got != want {
		t.Fatalf("superseded V1 hash=%s (update migration 000088 if intentional)", got)
	}
}
