package observability

import (
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/metrics"
)

func TestPrompt16MetricsLowCardinality(t *testing.T) {
	if metrics.NormalizeReasonCode("unknown fact_ref sku:99102 validation exploded") != metrics.ReasonUnknownFactRef {
		t.Fatal(metrics.NormalizeReasonCode("unknown fact_ref"))
	}
	raw := "pq: duplicate key seller_id=271 run_id=517"
	code := metrics.NormalizeReasonCode(raw)
	if code == raw || strings.Contains(code, "271") {
		t.Fatal(code)
	}
	if !ForbiddenMetricLabel("seller_id") || ForbiddenMetricLabel("reason_code") {
		t.Fatal("label policy")
	}
}

func TestPrompt16TraceSanitization(t *testing.T) {
	if SanitizeTraceValue("Authorization: Bearer sk-abc") != "[redacted]" {
		t.Fatal("auth")
	}
	if !IsSensitiveAttrKey("gen_ai.prompt") {
		t.Fatal("prompt attr")
	}
}
