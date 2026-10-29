package actions

import (
	"testing"
)

func TestValidatePersistableActionItem_RejectsMissingEvidence(t *testing.T) {
	item := ActionItem{
		Type:               ActionTypeStockReplenishment,
		SourceCandidateIDs: []string{"c1"},
		EvidenceFactIDs:    []string{"f1"},
		CTAs:               []ActionCTA{{Type: "mark_checked", Label: "ok"}},
		EntityGroupRef:     "group",
	}
	if err := ValidatePersistableActionItem(item); err == nil {
		t.Fatal("expected missing evidence error")
	}
}

func TestValidatePersistableActionItem_RejectsMissingCTAs(t *testing.T) {
	item := ActionItem{
		Type:               ActionTypeStockReplenishment,
		SourceCandidateIDs: []string{"c1"},
		EvidenceFactIDs:    []string{"f1"},
		Evidence:           []ActionEvidence{{Kind: "metric", Label: "x", Value: "1"}},
		EntityGroupRef:     "group",
	}
	if err := ValidatePersistableActionItem(item); err == nil {
		t.Fatal("expected missing ctas error")
	}
}

func TestValidatePersistableActionItem_AcceptsValidObjectBased(t *testing.T) {
	item := ActionItem{
		Type:               ActionTypeStockReplenishment,
		SourceCandidateIDs: []string{"c1"},
		EvidenceFactIDs:    []string{"f1"},
		Evidence:           []ActionEvidence{{Kind: "metric", Label: "x", Value: "1"}},
		CTAs:               []ActionCTA{{Type: "mark_checked", Label: "ok"}},
		EntityGroupRef:     "group",
	}
	if err := ValidatePersistableActionItem(item); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidatePersistableActionItem_AcceptsFunnelTransition(t *testing.T) {
	item := ActionItem{
		Type:               ActionTypeFunnelWeakTransition,
		SourceCandidateIDs: []string{"c1"},
		EvidenceFactIDs:    []string{"f1"},
		Evidence:           []ActionEvidence{{Kind: "metric", Label: "x", Value: "1"}},
		CTAs:               []ActionCTA{{Type: "ask_ai", Label: "ask"}},
		AffectedEntities: ActionAffectedEntities{
			Funnel: &ActionFunnelRef{TransitionKey: "pdp_to_cart", FromStage: "a", ToStage: "b"},
		},
	}
	if err := ValidatePersistableActionItem(item); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCtaKeyToActionCTA_OpenSKUUsesProductsRoute(t *testing.T) {
	cta, ok := ctaKeyToActionCTA(ActionCTAOpenSKUList)
	if !ok {
		t.Fatal("expected cta")
	}
	if cta.Href != "/app/products" {
		t.Fatalf("href=%q want /app/products", cta.Href)
	}
}

func TestValidatePersistableActionItem_LegacyStillRequiresSourceCandidates(t *testing.T) {
	item := ActionItem{
		Type:            ActionTypeStockReplenishment,
		EvidenceFactIDs: []string{"f1"},
		Evidence:        []ActionEvidence{{Kind: "metric", Label: "x", Value: "1"}},
		CTAs:            []ActionCTA{{Type: "mark_checked", Label: "ok"}},
		EntityGroupRef:  "group",
	}
	if err := ValidatePersistableActionItem(item); err == nil {
		t.Fatal("expected missing source_candidate_ids error for legacy persist guard")
	}
}

func TestValidateAIFirstPersistableActionItem_DoesNotRequireSourceCandidates(t *testing.T) {
	item := ActionItem{
		Type:               ActionTypeSlowTurnoverReview,
		Category:           ActionCategorySlowTurnover,
		Priority:           ActionPriorityImportant,
		EvidenceFactIDs:    []string{"f1"},
		Evidence:           []ActionEvidence{{Kind: "metric", Label: "orders", Value: "5"}},
		CompletionCriteria: "Решение принято по каждому SKU.",
		AffectedEntities: ActionAffectedEntities{
			SKUs: []ActionSKURef{{SKU: "1", Name: "SKU 1"}},
		},
		OperationalDetail: &OperationalDetail{
			Status:        "available",
			FormatVersion: OperationalDetailFormatVersion,
			Why:           OperationalWhy{Headline: "h", Text: "t"},
			ActionPlan:    OperationalActionPlan{Headline: "Что сделать", Steps: []OperationalStep{{StepID: "s1", Title: "Шаг 1", Instruction: "Сделать"}}},
			ObjectActions: []OperationalObjectAction{{EntityType: "sku", EntityID: "sku:1", RecommendedAction: "Проверить"}},
		},
		CTAs: []ActionCTA{
			{Type: "ask_ai", Label: "Спросить AI"},
			{Type: "mark_checked", Label: "Отметить выполненным"},
			{Type: "mark_irrelevant", Label: "Неактуально"},
		},
	}
	if err := ValidateAIFirstPersistableActionItem(item); err != nil {
		t.Fatalf("unexpected ai-first persist guard error: %v", err)
	}
}

func TestMarshalActionItemPayloadJSON_PreservesCompletionCriteria(t *testing.T) {
	item := ActionItem{
		CompletionCriteria: "done when stock is covered",
		OperationalDetail: &OperationalDetail{
			Status:        "available",
			FormatVersion: OperationalDetailFormatVersion,
		},
	}
	raw := MarshalActionItemPayloadJSON(item)
	op, aiFirst := ParseActionPayload(raw)
	if aiFirst == nil || aiFirst.CompletionCriteria != item.CompletionCriteria {
		t.Fatalf("expected completion_criteria in payload, got %+v", aiFirst)
	}
	if op == nil {
		t.Fatal("expected operational detail")
	}
}
