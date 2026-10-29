package dashboardhome

import "testing"

func TestDedupeActionFeedRecommendationWinsOverAISummary(t *testing.T) {
	recID := int64(42)
	candidates := []ActionItem{
		{
			Priority: "high", Action: "Проверить остатки SKU 1001",
			RelatedEntityType: "sku", RelatedEntityID: actionStrPtr("1001"),
			Source: "ai_summary",
		},
		{
			Priority: "high", Action: "Проверить остатки SKU 1001",
			RelatedEntityType: "sku", RelatedEntityID: actionStrPtr("1001"),
			Source: "recommendation", LinkedRecommendationID: &recID,
		},
	}
	out := dedupeActionFeed(candidates, 10)
	if len(out) != 1 {
		t.Fatalf("len=%d want 1", len(out))
	}
	if out[0].Source != "recommendation" {
		t.Fatalf("source=%s want recommendation", out[0].Source)
	}
}

func TestDedupeActionFeedAISummaryWinsOverAlert(t *testing.T) {
	alertID := int64(7)
	candidates := []ActionItem{
		{
			Priority: "high", Action: "Снизить DRR кампании 55",
			RelatedEntityType: "campaign", RelatedEntityID: actionStrPtr("55"),
			Source: "alert", LinkedAlertID: &alertID,
		},
		{
			Priority: "high", Action: "Снизить DRR кампании 55",
			RelatedEntityType: "campaign", RelatedEntityID: actionStrPtr("55"),
			Source: "ai_summary",
		},
	}
	out := dedupeActionFeed(candidates, 10)
	if len(out) != 1 || out[0].Source != "ai_summary" {
		t.Fatalf("got %+v", out)
	}
}

func TestDedupeActionFeedDifferentSKUNotMerged(t *testing.T) {
	candidates := []ActionItem{
		{Priority: "high", Action: "Проверить остатки", RelatedEntityType: "sku", RelatedEntityID: actionStrPtr("1"), Source: "recommendation"},
		{Priority: "high", Action: "Проверить остатки", RelatedEntityType: "sku", RelatedEntityID: actionStrPtr("2"), Source: "ai_summary"},
	}
	out := dedupeActionFeed(candidates, 10)
	if len(out) != 2 {
		t.Fatalf("len=%d want 2 distinct SKUs", len(out))
	}
}

func TestDedupeActionFeedDifferentActionTypesNotMerged(t *testing.T) {
	candidates := []ActionItem{
		{Priority: "high", Action: "Пополнить склад", RelatedEntityType: "sku", RelatedEntityID: actionStrPtr("1"), Source: "recommendation"},
		{Priority: "high", Action: "Снизить рекламный бюджет", RelatedEntityType: "sku", RelatedEntityID: actionStrPtr("1"), Source: "ai_summary"},
	}
	out := dedupeActionFeed(candidates, 10)
	if len(out) != 2 {
		t.Fatalf("len=%d want 2 different actions", len(out))
	}
}

func actionStrPtr(s string) *string { return &s }
