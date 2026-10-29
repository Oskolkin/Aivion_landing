package actions

import "testing"

func aiFirstPersistableBaseItem() ActionItem {
	return ActionItem{
		Type:               ActionTypeSalesDropReview,
		Category:           ActionCategorySales,
		Priority:           ActionPriorityImportant,
		EvidenceFactIDs:    []string{"f1"},
		Evidence:           []ActionEvidence{{Kind: "metric", Label: "orders", Value: "5"}},
		CompletionCriteria: "Продажи восстановлены.",
		CTAs: []ActionCTA{
			{Type: "ask_ai", Label: "Спросить AI"},
			{Type: "mark_checked", Label: "Отметить выполненным"},
			{Type: "mark_irrelevant", Label: "Неактуально"},
		},
		SourceCandidateIDs: []string{"manifest_context:candidate:01"},
	}
}

func TestValidateAIFirstPersistableActionItem_AccountScopeWithoutEntities(t *testing.T) {
	item := aiFirstPersistableBaseItem()
	item.ScopeType = string(AIFirstActionScopeAccount)
	item.AffectedObjectsTotal = 0
	item.AffectedObjectsDisplayed = 0
	if err := ValidateAIFirstPersistableActionItem(item); err != nil {
		t.Fatalf("account scope without entities should persist: %v", err)
	}
}

func TestValidateAIFirstPersistableActionItem_EntityScopeWithoutEntities(t *testing.T) {
	item := aiFirstPersistableBaseItem()
	item.ScopeType = string(AIFirstActionScopeEntity)
	if err := ValidateAIFirstPersistableActionItem(item); err == nil {
		t.Fatal("entity scope without entities should be rejected")
	}
}

func TestValidateAIFirstPersistableActionItem_MixedScopeWithoutEntities(t *testing.T) {
	item := aiFirstPersistableBaseItem()
	item.ScopeType = string(AIFirstActionScopeMixed)
	if err := ValidateAIFirstPersistableActionItem(item); err == nil {
		t.Fatal("mixed scope without entities should be rejected")
	}
}

func TestValidateAIFirstPersistableActionItem_SalesDropAccountLevel(t *testing.T) {
	item := aiFirstPersistableBaseItem()
	item.Type = ActionTypeSalesDropReview
	item.ScopeType = string(AIFirstActionScopeAccount)
	item.SourceCandidateIDs = []string{
		"manifest_context:candidate:01",
		"account_funnel_quality_constraints:candidate:01",
	}
	if err := ValidateAIFirstPersistableActionItem(item); err != nil {
		t.Fatalf("run-71-like account sales_drop_review should persist: %v", err)
	}
}

func TestValidateAIFirstPersistableActionItem_ScopeNotInferredFromType(t *testing.T) {
	entityItem := aiFirstPersistableBaseItem()
	entityItem.ScopeType = string(AIFirstActionScopeEntity)
	entityItem.AffectedEntities = ActionAffectedEntities{SKUs: []ActionSKURef{{SKU: "801"}}}
	entityItem.OperationalDetail = &OperationalDetail{
		Status:        "available",
		FormatVersion: OperationalDetailFormatVersion,
		Why:           OperationalWhy{Headline: "h", Text: "t"},
		ActionPlan:    OperationalActionPlan{Headline: "Что сделать", Steps: []OperationalStep{{StepID: "s1", Title: "Шаг", Instruction: "Сделать"}}},
		ObjectActions: []OperationalObjectAction{{EntityType: "sku", EntityID: "801", RecommendedAction: "Проверить"}},
	}
	if err := ValidateAIFirstPersistableActionItem(entityItem); err != nil {
		t.Fatalf("sales_drop_review+entity should persist when entities exist: %v", err)
	}
}
