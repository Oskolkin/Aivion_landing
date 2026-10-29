package aisummaries

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestBuildAccountSKUCandidates_DedupesAcrossBuckets(t *testing.T) {
	sku := int64(1001)
	offer := "OFF-1"
	name := "SKU 1"
	items := []analytics.DashboardSKUMetricsItem{
		{
			SKU:                 &sku,
			OfferID:             &offer,
			ProductName:         &name,
			RevenueLast7Days:    10000,
			OrdersLast7Days:     20,
			OrdersDeltaDayToDay: -10,
			StockAvailable:      0,
		},
	}
	alerts := []dbgen.Alert{{EntitySku: pgtype.Int8{Int64: sku, Valid: true}, Severity: "high"}}
	recs := []dbgen.Recommendation{{EntitySku: pgtype.Int8{Int64: sku, Valid: true}, PriorityLevel: "high"}}
	candidates := buildAccountSKUCandidates(items, alerts, recs, 100)
	if len(candidates) != 1 {
		t.Fatalf("expected deduped candidate, got %d", len(candidates))
	}
}

func TestTrimAccountContextForTokenBudget_ReducesEstimatedTokens(t *testing.T) {
	ctx := map[string]any{
		"metric_date":                 "2026-06-17",
		"sku_candidates":              make([]map[string]any, 0, 200),
		"category_rollup_top":         make([]map[string]any, 0, 40),
		"open_alerts_sample":          make([]map[string]any, 0, 60),
		"open_recommendations_sample": make([]map[string]any, 0, 60),
		"dashboard":                   map[string]any{"large": "x"},
		"account_metrics_last_7d":     make([]map[string]any, 0, 20),
	}
	for i := 0; i < 200; i++ {
		ctx["sku_candidates"] = append(ctx["sku_candidates"].([]map[string]any), map[string]any{
			"sku": i, "text": "very long compact candidate payload for token estimation regression test",
		})
	}
	for i := 0; i < 40; i++ {
		ctx["category_rollup_top"] = append(ctx["category_rollup_top"].([]map[string]any), map[string]any{"category_id": i, "name": "Category"})
	}
	res := trimAccountContextForTokenBudget("sys", "user", ctx, 2000)
	if !res.Trimmed {
		t.Fatal("expected context to be trimmed")
	}
	if res.FinalTokens >= res.OriginalTokens {
		t.Fatalf("expected reduced tokens, before=%d after=%d", res.OriginalTokens, res.FinalTokens)
	}
}

func TestAttachTrimDebugToContext_CopiesCoverageFlags(t *testing.T) {
	ctx := map[string]any{
		"sku_coverage": map[string]any{},
	}
	attachTrimDebugToContext(ctx, accountContextTrimResult{
		Trimmed:             true,
		TrimTier:            3,
		OriginalTokens:      22000,
		FinalTokens:         18000,
		SKUCandidatesBefore: 120,
		SKUCandidatesAfter:  80,
	})
	dbg, ok := ctx["token_budget_debug"].(map[string]any)
	if !ok || dbg["trim_tier"] != 3 {
		t.Fatalf("token_budget_debug=%v", ctx["token_budget_debug"])
	}
	cov := ctx["sku_coverage"].(map[string]any)
	if cov["context_trimmed"] != true {
		t.Fatalf("sku_coverage=%v", cov)
	}
}
