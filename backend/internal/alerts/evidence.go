package alerts

func BuildSalesEvidence(metricWindowDays int, currentRevenue float64, previousRevenue float64, currentOrders int64, previousOrders int64, extra EvidencePayload) EvidencePayload {
	payload := EvidencePayload{
		"domain":                "sales",
		"metric_window_days":    metricWindowDays,
		"current_revenue":       currentRevenue,
		"previous_revenue":      previousRevenue,
		"current_orders_count":  currentOrders,
		"previous_orders_count": previousOrders,
	}
	mergeEvidence(payload, extra)
	return payload
}

func BuildStockEvidence(daysOfCover float64, availableStock int64, avgDailySales float64, extra EvidencePayload) EvidencePayload {
	payload := EvidencePayload{
		"domain":              "stock",
		"days_of_cover":       daysOfCover,
		"available_stock":     availableStock,
		"average_daily_sales": avgDailySales,
	}
	mergeEvidence(payload, extra)
	return payload
}

func BuildAdvertisingEvidence(spend float64, clicks int64, orders int64, revenue float64, acos float64, extra EvidencePayload) EvidencePayload {
	payload := EvidencePayload{
		"domain":       "advertising",
		"spend":        spend,
		"clicks":       clicks,
		"orders_count": orders,
		"revenue":      revenue,
		"acos_percent": acos,
	}
	mergeEvidence(payload, extra)
	return payload
}

func BuildPriceEvidence(currentPrice float64, minConstraint *float64, maxConstraint *float64, impliedCost *float64, expectedMarginPercent *float64, extra EvidencePayload) EvidencePayload {
	payload := EvidencePayload{
		"domain":        "price_economics",
		"current_price": currentPrice,
	}
	if minConstraint != nil {
		payload["min_constraint"] = *minConstraint
	}
	if maxConstraint != nil {
		payload["max_constraint"] = *maxConstraint
	}
	if impliedCost != nil {
		payload["implied_cost"] = *impliedCost
	}
	if expectedMarginPercent != nil {
		payload["expected_margin_percent"] = *expectedMarginPercent
	}
	mergeEvidence(payload, extra)
	return payload
}

func normalizeEvidence(payload EvidencePayload) EvidencePayload {
	if payload == nil {
		return EvidencePayload{}
	}
	return payload
}

func mergeEvidence(base EvidencePayload, extra EvidencePayload) {
	for k, v := range extra {
		base[k] = v
	}
}
