package actions

// aiFirstActionTypeCategory is the canonical mapping for the future shared chunk
// analysis pipeline. Existing candidate builders still set Category directly and
// do not use this map yet.
var aiFirstActionTypeCategory = map[ActionType]ActionCategory{
	ActionTypeStockReplenishment:   ActionCategoryStock,
	ActionTypeStockRedistribution:  ActionCategoryStock,
	ActionTypeSlowTurnoverReview:   ActionCategorySlowTurnover,
	ActionTypeNoSalesWithStock:     ActionCategorySlowTurnover,
	ActionTypeSKUOrdersDrop:        ActionCategorySKU,
	ActionTypeKeySKUProtection:     ActionCategorySKU,
	ActionTypeAdHighDRRReview:      ActionCategoryAdvertising,
	ActionTypeAdSpendNoOrders:      ActionCategoryAdvertising,
	ActionTypeAdBudgetReallocation: ActionCategoryAdvertising,
	ActionTypeFunnelWeakTransition: ActionCategoryFunnel,
	ActionTypeCardConversionReview: ActionCategoryFunnel,
	ActionTypeCategoryUnderperform: ActionCategoryCategory,
	ActionTypeSalesDropReview:      ActionCategorySales,
}

// ActionCategoryForType returns the category for an active AI-first action type
// in the future shared chunk analysis pipeline.
//
// Unknown and empty types return ok=false. Legacy candidate-first types
// (key_sku_risk, ad_high_drr) and internal/future types (data_quality_issue)
// are intentionally excluded.
func ActionCategoryForType(actionType ActionType) (ActionCategory, bool) {
	if actionType == "" {
		return "", false
	}
	category, ok := aiFirstActionTypeCategory[actionType]
	return category, ok
}
