package alerts

import (
	"fmt"
	"time"
)

const (
	salesDropThresholdPercent           = -30.0
	skuOrdersDropThresholdPercent       = -40.0
	skuOrdersDropMinimumPreviousOrders  = 5
	skuRevenueDropThresholdPercent      = -40.0 // legacy rule only — not used in active pipeline
	skuRevenueDropMinimumPreviousAmount = 1000.0
	skuNegativeContributionThreshold    = 0.20
)

type SalesRuleEvaluationInput struct {
	SellerAccountID  int64
	AsOfDate         time.Time
	PreviousDate     time.Time
	CurrentAccount   *AccountDailyMetric
	PreviousAccount  *AccountDailyMetric
	CurrentSKUs      []SKUDailyMetric
	PreviousSKUsByID map[int64]SKUDailyMetric
}

type SalesRuleEvaluationResult struct {
	RuleResults []RuleResult
	Skipped     int
}

func EvaluateSalesRules(input SalesRuleEvaluationInput) SalesRuleEvaluationResult {
	var out []RuleResult
	skipped := 0

	if rr, ok := evaluateSalesRevenueDrop(input); ok {
		out = append(out, rr)
	} else {
		skipped++
	}
	if rr, ok := evaluateSalesOrdersDrop(input); ok {
		out = append(out, rr)
	} else {
		skipped++
	}

	accountDelta := 0.0
	if input.CurrentAccount != nil && input.PreviousAccount != nil {
		accountDelta = DeltaAbsolute(input.CurrentAccount.Revenue, input.PreviousAccount.Revenue)
	}

	for _, current := range input.CurrentSKUs {
		previous, exists := input.PreviousSKUsByID[current.OzonProductID]
		if !exists {
			skipped++
			continue
		}
		if rr, ok := evaluateSKUOrdersDrop(input, current, previous); ok {
			out = append(out, rr)
		} else {
			skipped++
		}

		if rr, ok := evaluateSKUNegativeContribution(input, current, previous, accountDelta); ok {
			out = append(out, rr)
		} else {
			skipped++
		}
	}

	return SalesRuleEvaluationResult{
		RuleResults: out,
		Skipped:     skipped,
	}
}

func evaluateSalesRevenueDrop(input SalesRuleEvaluationInput) (RuleResult, bool) {
	if input.CurrentAccount == nil || input.PreviousAccount == nil {
		return RuleResult{}, false
	}
	previous := input.PreviousAccount.Revenue
	current := input.CurrentAccount.Revenue
	deltaPercent, ok := DeltaPercent(current, previous)
	if !ok || previous <= 0 || !IsDropAtOrBelow(deltaPercent, salesDropThresholdPercent) {
		return RuleResult{}, false
	}
	sev := severityForAccountDrop(deltaPercent)
	return RuleResult{
		AlertType:  AlertTypeSalesRevenueDrop,
		AlertGroup: AlertGroupSales,
		EntityType: EntityTypeAccount,
		Title:      "Резкое падение выручки",
		Message:    fmt.Sprintf("Выручка снизилась на %.1f%% (порог %.0f%%).", deltaPercent, salesDropThresholdPercent),
		Severity:   sev,
		Urgency:    urgencyForAccountDrop(sev),
		EvidencePayload: BuildSalesEvidence(1, current, previous, int64(input.CurrentAccount.OrdersCount), int64(input.PreviousAccount.OrdersCount), EvidencePayload{
			"metric":            "revenue",
			"delta_absolute":    DeltaAbsolute(current, previous),
			"delta_percent":     deltaPercent,
			"threshold_percent": salesDropThresholdPercent,
			"current_date":      formatDate(input.AsOfDate),
			"previous_date":     formatDate(input.PreviousDate),
		}),
	}, true
}

func evaluateSalesOrdersDrop(input SalesRuleEvaluationInput) (RuleResult, bool) {
	if input.CurrentAccount == nil || input.PreviousAccount == nil {
		return RuleResult{}, false
	}
	previous := float64(input.PreviousAccount.OrdersCount)
	current := float64(input.CurrentAccount.OrdersCount)
	deltaPercent, ok := DeltaPercent(current, previous)
	if !ok || previous <= 0 || !IsDropAtOrBelow(deltaPercent, salesDropThresholdPercent) {
		return RuleResult{}, false
	}
	sev := severityForAccountDrop(deltaPercent)
	return RuleResult{
		AlertType:  AlertTypeSalesOrdersDrop,
		AlertGroup: AlertGroupSales,
		EntityType: EntityTypeAccount,
		Title:      "Резкое падение заказов",
		Message:    fmt.Sprintf("Количество заказов снизилось на %.1f%% (порог %.0f%%).", deltaPercent, salesDropThresholdPercent),
		Severity:   sev,
		Urgency:    urgencyForAccountDrop(sev),
		EvidencePayload: BuildSalesEvidence(1, input.CurrentAccount.Revenue, input.PreviousAccount.Revenue, int64(input.CurrentAccount.OrdersCount), int64(input.PreviousAccount.OrdersCount), EvidencePayload{
			"metric":            "orders",
			"current_orders":    input.CurrentAccount.OrdersCount,
			"previous_orders":   input.PreviousAccount.OrdersCount,
			"delta_absolute":    DeltaAbsolute(current, previous),
			"delta_percent":     deltaPercent,
			"threshold_percent": salesDropThresholdPercent,
			"current_date":      formatDate(input.AsOfDate),
			"previous_date":     formatDate(input.PreviousDate),
		}),
	}, true
}

func evaluateSKUOrdersDrop(input SalesRuleEvaluationInput, current SKUDailyMetric, previous SKUDailyMetric) (RuleResult, bool) {
	if previous.OrdersCount < skuOrdersDropMinimumPreviousOrders || previous.OrdersCount <= 0 {
		return RuleResult{}, false
	}
	deltaPercent, ok := DeltaPercent(float64(current.OrdersCount), float64(previous.OrdersCount))
	if !ok || !IsDropAtOrBelow(deltaPercent, skuOrdersDropThresholdPercent) {
		return RuleResult{}, false
	}
	sev := severityForSKUOrdersDrop(deltaPercent)
	entityID := fmt.Sprintf("%d", current.OzonProductID)
	ordersDelta := int64(current.OrdersCount) - int64(previous.OrdersCount)
	return RuleResult{
		AlertType:     AlertTypeSKUOrdersDrop,
		AlertGroup:    AlertGroupSales,
		EntityType:    EntityTypeSKU,
		EntityID:      &entityID,
		EntitySKU:     current.SKU,
		EntityOfferID: current.OfferID,
		Title:         "Просадка заказов по SKU",
		Message:       fmt.Sprintf("Количество заказов по SKU снизилось на %.1f%% (порог %.0f%%).", deltaPercent, skuOrdersDropThresholdPercent),
		Severity:      sev,
		Urgency:       urgencyForSKUSeverity(sev),
		EvidencePayload: BuildSalesEvidence(1, current.Revenue, previous.Revenue, int64(current.OrdersCount), int64(previous.OrdersCount), EvidencePayload{
			"metric":                   "sku_orders",
			"current_orders":           current.OrdersCount,
			"comparison_orders":        previous.OrdersCount,
			"orders_delta":             ordersDelta,
			"orders_delta_percent":     deltaPercent,
			"sku_orders_delta_percent": deltaPercent,
			"threshold_percent":        skuOrdersDropThresholdPercent,
			"minimum_previous_orders":  skuOrdersDropMinimumPreviousOrders,
			"metric_date":              formatDate(input.AsOfDate),
			"comparison_date":          formatDate(input.PreviousDate),
			"comparison_period":        "previous_day",
			"sku":                      current.SKU,
			"offer_id":                 current.OfferID,
			"product_id":               current.OzonProductID,
			"ozon_product_id":          current.OzonProductID,
			"reason":                   "orders_drop_vs_previous_day",
		}),
	}, true
}

// evaluateSKURevenueDrop is legacy-only; not invoked by EvaluateSalesRules while sku_orders_drop is active.
func evaluateSKURevenueDrop(input SalesRuleEvaluationInput, current SKUDailyMetric, previous SKUDailyMetric) (RuleResult, bool) {
	if previous.Revenue < skuRevenueDropMinimumPreviousAmount || previous.Revenue <= 0 {
		return RuleResult{}, false
	}
	deltaPercent, ok := DeltaPercent(current.Revenue, previous.Revenue)
	if !ok || !IsDropAtOrBelow(deltaPercent, skuRevenueDropThresholdPercent) {
		return RuleResult{}, false
	}
	sev := severityForSKURevenueDrop(deltaPercent)
	entityID := fmt.Sprintf("%d", current.OzonProductID)
	return RuleResult{
		AlertType:     AlertTypeSKURevenueDrop,
		AlertGroup:    AlertGroupSales,
		EntityType:    EntityTypeSKU,
		EntityID:      &entityID,
		EntitySKU:     current.SKU,
		EntityOfferID: current.OfferID,
		Title:         "Просадка выручки по SKU",
		Message:       fmt.Sprintf("Выручка по SKU снизилась на %.1f%% (порог %.0f%%).", deltaPercent, skuRevenueDropThresholdPercent),
		Severity:      sev,
		Urgency:       urgencyForSKUSeverity(sev),
		EvidencePayload: BuildSalesEvidence(1, current.Revenue, previous.Revenue, int64(current.OrdersCount), int64(previous.OrdersCount), EvidencePayload{
			"metric":                   "sku_revenue",
			"sku":                      current.SKU,
			"offer_id":                 current.OfferID,
			"product_id":               current.OzonProductID,
			"ozon_product_id":          current.OzonProductID,
			"delta_absolute":           DeltaAbsolute(current.Revenue, previous.Revenue),
			"delta_percent":            deltaPercent,
			"threshold_percent":        skuRevenueDropThresholdPercent,
			"minimum_previous_revenue": skuRevenueDropMinimumPreviousAmount,
			"current_date":             formatDate(input.AsOfDate),
			"previous_date":            formatDate(input.PreviousDate),
		}),
	}, true
}

func evaluateSKUNegativeContribution(input SalesRuleEvaluationInput, current SKUDailyMetric, previous SKUDailyMetric, accountRevenueDelta float64) (RuleResult, bool) {
	if accountRevenueDelta >= 0 {
		return RuleResult{}, false
	}
	skuDelta := DeltaAbsolute(current.Revenue, previous.Revenue)
	if skuDelta >= 0 {
		return RuleResult{}, false
	}
	share := abs(skuDelta) / abs(accountRevenueDelta)
	if share < skuNegativeContributionThreshold {
		return RuleResult{}, false
	}
	sev := severityForContributionShare(share)
	entityID := fmt.Sprintf("%d", current.OzonProductID)
	return RuleResult{
		AlertType:     AlertTypeSKUNegativeContribution,
		AlertGroup:    AlertGroupSales,
		EntityType:    EntityTypeSKU,
		EntityID:      &entityID,
		EntitySKU:     current.SKU,
		EntityOfferID: current.OfferID,
		Title:         "SKU внёс значимый вклад в падение выручки",
		Message:       "SKU объясняет значимую часть общей просадки выручки аккаунта.",
		Severity:      sev,
		Urgency:       urgencyForContributionSeverity(sev),
		EvidencePayload: BuildSalesEvidence(1, current.Revenue, previous.Revenue, int64(current.OrdersCount), int64(previous.OrdersCount), EvidencePayload{
			"metric":                "revenue_contribution",
			"sku":                   current.SKU,
			"offer_id":              current.OfferID,
			"product_id":            current.OzonProductID,
			"ozon_product_id":       current.OzonProductID,
			"account_revenue_delta": accountRevenueDelta,
			"sku_revenue_delta":     skuDelta,
			"contribution_share":    share,
			"threshold_share":       skuNegativeContributionThreshold,
			"current_date":          formatDate(input.AsOfDate),
			"previous_date":         formatDate(input.PreviousDate),
		}),
	}, true
}

func severityForAccountDrop(deltaPercent float64) Severity {
	if deltaPercent <= -60 {
		return SeverityCritical
	}
	if deltaPercent <= -40 {
		return SeverityHigh
	}
	return SeverityMedium
}

func severityForSKUOrdersDrop(deltaPercent float64) Severity {
	if deltaPercent <= -70 {
		return SeverityCritical
	}
	if deltaPercent <= -50 {
		return SeverityHigh
	}
	return SeverityMedium
}

func severityForSKURevenueDrop(deltaPercent float64) Severity {
	if deltaPercent <= -70 {
		return SeverityCritical
	}
	if deltaPercent <= -50 {
		return SeverityHigh
	}
	return SeverityMedium
}

func severityForContributionShare(share float64) Severity {
	if share >= 0.50 {
		return SeverityCritical
	}
	if share >= 0.30 {
		return SeverityHigh
	}
	return SeverityMedium
}

func urgencyForAccountDrop(severity Severity) Urgency {
	if severity == SeverityCritical || severity == SeverityHigh {
		return UrgencyHigh
	}
	return UrgencyMedium
}

func urgencyForSKUSeverity(severity Severity) Urgency {
	if severity == SeverityCritical || severity == SeverityHigh {
		return UrgencyHigh
	}
	return UrgencyMedium
}

func urgencyForContributionSeverity(severity Severity) Urgency {
	if severity == SeverityCritical || severity == SeverityHigh {
		return UrgencyHigh
	}
	return UrgencyMedium
}

func formatDate(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
