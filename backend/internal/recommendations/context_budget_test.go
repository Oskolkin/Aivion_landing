package recommendations

import (
	"strings"
	"testing"
	"time"
)

func TestApplyRecommendationContextBudgetTruncatesByBytes(t *testing.T) {
	now := time.Now().UTC()
	ctx := &AIRecommendationContext{
		ContextVersion:  "v",
		SellerAccountID: 1,
		AsOfDate:        "2026-01-01",
		GeneratedAt:     now,
		Alerts: AlertsContext{
			TopOpen: []AlertSignal{{
				ID: 1, Title: strings.Repeat("A", 8000), Message: "m",
				Severity: "high", Urgency: "u", LastSeenAt: now,
			}},
		},
	}
	ApplyRecommendationContextBudget(ctx, ContextBuildLimits{
		MaxItemsPerList: 50,
		MaxContextBytes: 1500,
	})
	if !ctx.ContextTruncated {
		t.Fatalf("expected truncation for oversized context")
	}
	if ctx.ContextApproxUncompressedBytes == 0 {
		t.Fatalf("expected byte estimate populated")
	}
}
