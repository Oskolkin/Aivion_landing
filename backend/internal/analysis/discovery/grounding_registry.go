package discovery

type recipe struct {
	Path       string
	EntityKind EntityKind
}

func closedBlockOrder() []string {
	return []string{
		"sales.ordered_amount_comparison",
		"sales.ordered_units_comparison",
		"sales.analysis_day_amount_comparison",
		"sales.analysis_day_units_comparison",
		"price.observed_price",
		"price.last_observed_price_change",
		"stock.current_state",
		"stock.coverage",
		"advertising.traffic.paid_spend_comparison",
		"advertising.attribution.attributed_order_amount_comparison",
		"funnel.search.impressions_comparison",
		"search.provider_average_position",
		"finance.net_accrual_comparison",
		"supply.open_inbound",
	}
}
