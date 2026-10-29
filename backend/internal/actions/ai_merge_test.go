package actions

import (
	"testing"
)

func TestMergeAndDedup_SameCandidateTypeOneAction(t *testing.T) {
	action := validStockAIAction()
	a := ScoredDraftAction{
		TempActionID:   "a1",
		Action:         action,
		Domain:         ActionDomainStock,
		EntityGroupRef: "stock_replenishment_2026-06-24_key_low_cover",
	}
	b := ScoredDraftAction{
		TempActionID:   "a2",
		Action:         action,
		Domain:         ActionDomainStock,
		EntityGroupRef: "stock_replenishment_2026-06-24_key_low_cover",
	}
	out := MergeAndDedup([]ScoredDraftAction{a, b})
	if len(out) != 1 {
		t.Fatalf("deduped=%d want 1", len(out))
	}
}

func TestMergeAndDedup_DifferentTypesBothRemain(t *testing.T) {
	a := ScoredDraftAction{
		TempActionID: "a1",
		Action: DomainAIAction{
			Type:               ActionTypeStockReplenishment,
			SourceCandidateIDs: []string{"c1"},
			Title:              "Action A",
		},
		Domain: ActionDomainStock,
	}
	b := ScoredDraftAction{
		TempActionID: "a2",
		Action: DomainAIAction{
			Type:               ActionTypeNoSalesWithStock,
			SourceCandidateIDs: []string{"c2"},
			Title:              "Action B",
		},
		Domain: ActionDomainStock,
	}
	out := MergeAndDedup([]ScoredDraftAction{a, b})
	if len(out) != 2 {
		t.Fatalf("count=%d want 2", len(out))
	}
}

func TestBuildActionKey_Deterministic(t *testing.T) {
	k1 := BuildActionKey("2026-06-24", ActionTypeStockReplenishment, []string{"c2", "c1"}, "group1")
	k2 := BuildActionKey("2026-06-24", ActionTypeStockReplenishment, []string{"c1", "c2"}, "group1")
	if k1 != k2 {
		t.Fatalf("keys differ: %q vs %q", k1, k2)
	}
	if err := ValidateActionKey(k1); err != nil {
		t.Fatalf("invalid key: %v", err)
	}
}

func TestBuildActionKey_LongKeyHashed(t *testing.T) {
	longIDs := make([]string, 50)
	for i := range longIDs {
		longIDs[i] = "very_long_candidate_id_" + string(rune('a'+i%26))
	}
	key := BuildActionKey("2026-06-24", ActionTypeStockReplenishment, longIDs, "group")
	if len(key) > 200 {
		t.Fatalf("key too long: %d", len(key))
	}
}
