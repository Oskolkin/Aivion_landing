package aipipeline

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicontext"
)

func TestParseGlobalSemanticContextNormalizesStringRiskLists(t *testing.T) {
	raw := `{
		"schema_version": "global_semantic_context_v1",
		"overall_story": "Кабинет требует внимания.",
		"confidence": "medium",
		"source_fact_ids": "account.current_day.revenue",
		"primary_risks": "[]",
		"secondary_risks": "",
		"priority_order": []
	}`
	ctx, err := ParseGlobalSemanticContext(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(ctx.PrimaryRisks) != 0 {
		t.Fatalf("primary_risks=%v", ctx.PrimaryRisks)
	}
	if len(ctx.SourceFactIDs) != 1 || ctx.SourceFactIDs[0] != "account.current_day.revenue" {
		t.Fatalf("source_fact_ids=%v", ctx.SourceFactIDs)
	}
	_ = aicontext.GlobalSemanticContextSchemaVersion
}

func TestParseGlobalSemanticContextSkipsStringOpportunityEntries(t *testing.T) {
	raw := `{
		"schema_version": "global_semantic_context_v1",
		"overall_story": "Кабинет требует внимания.",
		"confidence": "medium",
		"source_fact_ids": ["account.current_day.revenue"],
		"opportunities": ["noise", {"signal_id":"s1","category":"sales","title":"Рост"}]
	}`
	ctx, err := ParseGlobalSemanticContext(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(ctx.Opportunities) != 1 || ctx.Opportunities[0].SignalID != "s1" {
		t.Fatalf("opportunities=%+v", ctx.Opportunities)
	}
}
