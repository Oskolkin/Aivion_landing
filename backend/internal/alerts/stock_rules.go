package alerts

import "fmt"

const (
	stockSupplyPlanningMaxDays = 14.0
	stockSupplyPlanningMinDays = 7.0
	stockOOSRiskMaxDays        = 7.0
	stockOOSCriticalDays       = 3.0

	stockReasonSupplyPlanning    = "supply_planning_window"
	stockReasonLastCall          = "last_call_replenishment_window"
	stockReasonImminentOOS       = "imminent_out_of_stock_risk"
	stockReasonKeySupplyPlanning = "key_sku_supply_planning_window"
	stockReasonKeyLastCall       = "key_sku_last_call_replenishment_window"
	stockReasonKeyImminentOOS    = "key_sku_imminent_out_of_stock_risk"
	stockSupplyPlanningFBONote   = "fbo_replenishment_cycle_7_14_days"
)

type StockRuleEvaluationInput struct {
	SellerAccountID        int64
	AsOfDate               string
	SKUMetrics             []SKUDailyMetric
	SKUImportanceByProduct SKUImportanceByProduct
	FBOStorageRiskBySKU    FBOStorageRiskBySKU
}

type StockRuleEvaluationResult struct {
	RuleResults []RuleResult
	Skipped     int
}

func EvaluateStockRules(input StockRuleEvaluationInput) StockRuleEvaluationResult {
	if len(input.SKUMetrics) == 0 && len(input.FBOStorageRiskBySKU) == 0 {
		return StockRuleEvaluationResult{RuleResults: nil, Skipped: 1}
	}

	results := make([]RuleResult, 0)
	skipped := 0

	for _, ctx := range input.FBOStorageRiskBySKU {
		if rr, ok := evaluateFBOSurplusStorageRisk(input, ctx); ok {
			results = append(results, rr)
		} else {
			skipped++
		}
	}

	for _, sku := range input.SKUMetrics {
		if rr, ok := evaluateStockOOSRisk(input, sku); ok {
			results = append(results, rr)
		} else {
			skipped++
		}
		if rr, ok := evaluateStockLowCoverage(input, sku); ok {
			results = append(results, rr)
		} else {
			skipped++
		}
		if rr, ok := evaluateStockCriticalSKULowStock(input, sku); ok {
			results = append(results, rr)
		} else {
			skipped++
		}
	}

	return StockRuleEvaluationResult{
		RuleResults: results,
		Skipped:     skipped,
	}
}

func stockActiveDaysOfCover(sku SKUDailyMetric) (float64, bool) {
	if sku.CurrentStock <= 0 || sku.DaysOfCover == nil {
		return 0, false
	}
	days := *sku.DaysOfCover
	if days <= 0 {
		return 0, false
	}
	return days, true
}

func baseStockEvidenceExtra(sku SKUDailyMetric, input StockRuleEvaluationInput, reason string) EvidencePayload {
	return EvidencePayload{
		"sku":                         sku.SKU,
		"offer_id":                    sku.OfferID,
		"product_id":                  sku.OzonProductID,
		"ozon_product_id":             sku.OzonProductID,
		"product_name":                sku.ProductName,
		"current_stock":               sku.CurrentStock,
		"days_of_cover":               sku.DaysOfCover,
		"threshold_low_coverage_days": stockSupplyPlanningMaxDays,
		"threshold_oos_risk_days":     stockOOSRiskMaxDays,
		"threshold_critical_days":     stockOOSCriticalDays,
		"supply_planning_reason":      stockSupplyPlanningFBONote,
		"reason":                      reason,
		"as_of_date":                  input.AsOfDate,
	}
}

func evaluateStockLowCoverage(input StockRuleEvaluationInput, sku SKUDailyMetric) (RuleResult, bool) {
	days, ok := stockActiveDaysOfCover(sku)
	if !ok || days <= stockSupplyPlanningMinDays || days > stockSupplyPlanningMaxDays {
		return RuleResult{}, false
	}

	entityID := fmt.Sprintf("%d", sku.OzonProductID)
	evidence := baseStockEvidenceExtra(sku, input, stockReasonSupplyPlanning)
	evidence["metric"] = "days_of_cover"
	return RuleResult{
		AlertType:       AlertTypeStockLowCoverage,
		AlertGroup:      AlertGroupStock,
		EntityType:      EntityTypeSKU,
		EntityID:        &entityID,
		EntitySKU:       sku.SKU,
		EntityOfferID:   sku.OfferID,
		Title:           "Запаса может не хватить на 14 дней",
		Message:         "Запаса товара может не хватить на ближайшие 14 дней. С учётом FBO-цикла поставки пора планировать пополнение.",
		Severity:        SeverityMedium,
		Urgency:         UrgencyMedium,
		EvidencePayload: BuildStockEvidence(days, int64(sku.CurrentStock), 0, evidence),
	}, true
}

func evaluateStockOOSRisk(input StockRuleEvaluationInput, sku SKUDailyMetric) (RuleResult, bool) {
	days, ok := stockActiveDaysOfCover(sku)
	if !ok || days > stockOOSRiskMaxDays {
		return RuleResult{}, false
	}

	reason := stockReasonLastCall
	sev := SeverityHigh
	urg := UrgencyHigh
	title := "Последнее окно для пополнения"
	message := "Запаса товара осталось меньше чем на неделю. Это последний шанс успеть пополнить товар до out-of-stock."
	if days <= stockOOSCriticalDays {
		reason = stockReasonImminentOOS
		sev = SeverityCritical
		urg = UrgencyImmediate
		title = "Риск out-of-stock в ближайшие дни"
		message = "Товар может закончиться в ближайшие дни."
	}

	entityID := fmt.Sprintf("%d", sku.OzonProductID)
	evidence := baseStockEvidenceExtra(sku, input, reason)
	evidence["metric"] = "stock_oos_risk"
	return RuleResult{
		AlertType:       AlertTypeStockOOSRisk,
		AlertGroup:      AlertGroupStock,
		EntityType:      EntityTypeSKU,
		EntityID:        &entityID,
		EntitySKU:       sku.SKU,
		EntityOfferID:   sku.OfferID,
		Title:           title,
		Message:         message,
		Severity:        sev,
		Urgency:         urg,
		EvidencePayload: BuildStockEvidence(days, int64(sku.CurrentStock), 0, evidence),
	}, true
}

func evaluateStockCriticalSKULowStock(input StockRuleEvaluationInput, sku SKUDailyMetric) (RuleResult, bool) {
	importance, ok := input.SKUImportanceByProduct[sku.OzonProductID]
	if !ok || !importance.IsKeySKU {
		return RuleResult{}, false
	}

	days, ok := stockActiveDaysOfCover(sku)
	if !ok || days > stockSupplyPlanningMaxDays {
		return RuleResult{}, false
	}

	reason := stockReasonKeySupplyPlanning
	sev := SeverityHigh
	urg := UrgencyHigh
	title := "Ключевой SKU: пора планировать пополнение"
	message := "Значимый SKU имеет запас на 14 дней или меньше. С учётом FBO-цикла поставки стоит запланировать пополнение."
	if days <= stockOOSCriticalDays {
		reason = stockReasonKeyImminentOOS
		sev = SeverityCritical
		urg = UrgencyImmediate
		title = "Ключевой SKU: критический риск out-of-stock"
		message = "Значимый SKU может закончиться в ближайшие дни."
	} else if days <= stockOOSRiskMaxDays {
		reason = stockReasonKeyLastCall
		sev = SeverityCritical
		urg = UrgencyHigh
		title = "Ключевой SKU: последнее окно для пополнения"
		message = "Значимый SKU имеет запас меньше недели — высокий риск потери продаж."
	}

	entityID := fmt.Sprintf("%d", sku.OzonProductID)
	evidence := MergeKeySKUImportanceEvidence(baseStockEvidenceExtra(sku, input, reason), importance)
	evidence["metric"] = "key_sku_stock"
	return RuleResult{
		AlertType:       AlertTypeStockCriticalSKULowStock,
		AlertGroup:      AlertGroupStock,
		EntityType:      EntityTypeSKU,
		EntityID:        &entityID,
		EntitySKU:       sku.SKU,
		EntityOfferID:   sku.OfferID,
		Title:           title,
		Message:         message,
		Severity:        sev,
		Urgency:         urg,
		EvidencePayload: BuildStockEvidence(days, int64(sku.CurrentStock), 0, evidence),
	}, true
}
