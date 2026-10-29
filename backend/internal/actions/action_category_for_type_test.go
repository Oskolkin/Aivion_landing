package actions

import "testing"

func TestActionCategoryForType_ActiveAIFirstTypes(t *testing.T) {
	tests := []struct {
		actionType ActionType
		category   ActionCategory
	}{
		{ActionTypeStockReplenishment, ActionCategoryStock},
		{ActionTypeStockRedistribution, ActionCategoryStock},
		{ActionTypeSlowTurnoverReview, ActionCategorySlowTurnover},
		{ActionTypeNoSalesWithStock, ActionCategorySlowTurnover},
		{ActionTypeSKUOrdersDrop, ActionCategorySKU},
		{ActionTypeKeySKUProtection, ActionCategorySKU},
		{ActionTypeAdHighDRRReview, ActionCategoryAdvertising},
		{ActionTypeAdSpendNoOrders, ActionCategoryAdvertising},
		{ActionTypeAdBudgetReallocation, ActionCategoryAdvertising},
		{ActionTypeFunnelWeakTransition, ActionCategoryFunnel},
		{ActionTypeCardConversionReview, ActionCategoryFunnel},
		{ActionTypeCategoryUnderperform, ActionCategoryCategory},
		{ActionTypeSalesDropReview, ActionCategorySales},
	}

	for _, tc := range tests {
		t.Run(string(tc.actionType), func(t *testing.T) {
			category, ok := ActionCategoryForType(tc.actionType)
			if !ok {
				t.Fatalf("expected ok=true for %q", tc.actionType)
			}
			if category != tc.category {
				t.Fatalf("category=%q want %q", category, tc.category)
			}
		})
	}
}

func TestActionCategoryForType_UnknownLegacyAndInternalTypes(t *testing.T) {
	legacyTypes := []ActionType{
		ActionTypeKeySKURisk, // legacy candidate-first
		ActionTypeAdHighDRR,  // legacy candidate-first
	}
	for _, actionType := range legacyTypes {
		if _, ok := ActionCategoryForType(actionType); ok {
			t.Fatalf("expected ok=false for legacy type %q", actionType)
		}
	}

	internalTypes := []ActionType{
		ActionTypeDataQualityIssue, // internal/future, not user-facing
	}
	for _, actionType := range internalTypes {
		if _, ok := ActionCategoryForType(actionType); ok {
			t.Fatalf("expected ok=false for internal/future type %q", actionType)
		}
	}

	unknownTypes := []ActionType{
		"",
		ActionType("unknown_action_type"),
	}
	for _, actionType := range unknownTypes {
		if _, ok := ActionCategoryForType(actionType); ok {
			t.Fatalf("expected ok=false for unknown type %q", actionType)
		}
	}
}

func TestActionCategoryForType_CoversAllActiveAIFirstTypes(t *testing.T) {
	for _, actionType := range ActionsAIFirstActiveActionTypes {
		if _, ok := ActionCategoryForType(actionType); !ok {
			t.Fatalf("missing mapping for active ai-first type %q", actionType)
		}
	}
}
