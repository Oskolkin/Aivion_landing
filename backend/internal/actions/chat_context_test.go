package actions

import "testing"

func TestBuildActionChatContext_IncludesAIFirstFields(t *testing.T) {
	action := ActionItem{
		ActionKey:          "k1",
		Title:              "Пополнить SKU",
		Reason:             "Низкий DOC",
		NextStep:           "Сделать поставку",
		EvidenceFactIDs:    []string{"sku.1.last_7d.orders"},
		CompletionCriteria: "SKU покрыт запасом на 14 дней",
		Limitations:        []string{"legacy limitation"},
		VisibleLimitations: []string{"campaign-level limitation"},
		OperationalDetail: &OperationalDetail{
			ObjectActions: []OperationalObjectAction{
				{
					EntityType:        "sku",
					EntityID:          "sku:1",
					Name:              "SKU-1",
					RecommendedAction: "Добавить в поставку",
				},
			},
		},
	}

	ctx := BuildActionChatContext(action)
	if len(ctx.EvidenceFactIDs) != 1 || ctx.EvidenceFactIDs[0] != "sku.1.last_7d.orders" {
		t.Fatalf("unexpected evidence_fact_ids: %#v", ctx.EvidenceFactIDs)
	}
	if len(ctx.ObjectActions) != 1 || ctx.ObjectActions[0] == "" {
		t.Fatalf("unexpected object_actions: %#v", ctx.ObjectActions)
	}
	if ctx.CompletionCriteria == "" {
		t.Fatal("expected completion criteria in chat context")
	}
	if len(ctx.Limitations) != 1 || ctx.Limitations[0] != "campaign-level limitation" {
		t.Fatalf("visible limitations must be preferred, got %#v", ctx.Limitations)
	}
}
