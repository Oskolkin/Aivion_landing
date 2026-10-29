package aidatapack

func buildFinanceRollup() FinanceRollupTable {
	return FinanceRollupTable{
		DataStatus: "not_available",
		Reason:     "finance ingestion is not implemented in P0 enrichment",
	}
}

func buildConstraintsBlock(ctx *loadContext) ConstraintsBlock {
	if len(ctx.PriceConstraints) == 0 {
		return ConstraintsBlock{
			PriceConstraintsAvailable: false,
			SKUPriceConstraints:       []any{},
			Limitations: []string{
				"No margin/cost constraints available; AI must not suggest direct price decrease without checking margin.",
			},
		}
	}
	items := make([]any, 0, len(ctx.PriceConstraints))
	for _, c := range ctx.PriceConstraints {
		items = append(items, map[string]any{
			"ozon_product_id":      c.OzonProductID,
			"sku":                  int64ToString(c.Sku.Int64),
			"effective_min_price":  nullableFloat(numericToFloatPtr(c.EffectiveMinPrice)),
			"effective_max_price":  nullableFloat(numericToFloatPtr(c.EffectiveMaxPrice)),
			"reference_margin_pct": nullableFloat(numericToFloatPtr(c.ReferenceMarginPercent)),
			"implied_cost":         nullableFloat(numericToFloatPtr(c.ImpliedCost)),
			"rule_scope_type":      c.ResolvedFromScopeType,
		})
	}
	return ConstraintsBlock{
		PriceConstraintsAvailable: true,
		SKUPriceConstraints:       items,
	}
}
