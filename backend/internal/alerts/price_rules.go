// Legacy price/economics alert rules. Not invoked by active RunForAccount while
// PriceEconomicsGenerationDisabled is true (no reliable unit economics in MVP).
package alerts

import (
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/pricingconstraints"
)

const (
	marginRiskThreshold = 0.10
)

type PriceRuleEvaluationInput struct {
	SellerAccountID        int64
	AsOfDate               string
	Products               []ProductPricingContext
	SKUImportanceByProduct SKUImportanceByProduct
}

type PriceRuleEvaluationResult struct {
	RuleResults []RuleResult
	Skipped     int
}

func EvaluatePriceEconomicsRules(input PriceRuleEvaluationInput) PriceRuleEvaluationResult {
	results := make([]RuleResult, 0)
	skipped := 0

	for _, p := range input.Products {
		if rr, ok := evaluatePriceBelowMinConstraint(input, p); ok {
			results = append(results, rr)
		} else {
			skipped++
		}
		if rr, ok := evaluatePriceAboveMaxConstraint(input, p); ok {
			results = append(results, rr)
		} else {
			skipped++
		}
		if rr, ok := evaluateMarginRiskAtCurrentPrice(input, p); ok {
			results = append(results, rr)
		} else {
			skipped++
		}
		if rr, ok := evaluateMissingPricingConstraintsForKeySKU(input, p); ok {
			results = append(results, rr)
		} else {
			skipped++
		}
	}

	return PriceRuleEvaluationResult{RuleResults: results, Skipped: skipped}
}

func evaluatePriceBelowMinConstraint(input PriceRuleEvaluationInput, p ProductPricingContext) (RuleResult, bool) {
	if p.ReferencePrice == nil || *p.ReferencePrice <= 0 || p.EffectiveMinPrice == nil {
		return RuleResult{}, false
	}
	if *p.ReferencePrice >= *p.EffectiveMinPrice {
		return RuleResult{}, false
	}
	deltaAbs := *p.ReferencePrice - *p.EffectiveMinPrice
	deltaPct := deltaAbs / *p.EffectiveMinPrice * 100.0
	entityID := fmt.Sprintf("%d", p.OzonProductID)
	sev := SeverityHigh
	urg := UrgencyHigh
	if deltaPct <= -10 {
		sev = SeverityCritical
		urg = UrgencyImmediate
	}
	return RuleResult{
		AlertType:     AlertTypePriceBelowMinConstraint,
		AlertGroup:    AlertGroupPriceEconomics,
		EntityType:    EntityTypeProduct,
		EntityID:      &entityID,
		EntitySKU:     p.SKU,
		EntityOfferID: p.OfferID,
		Title:         "Цена ниже минимального ограничения",
		Message:       "Текущая цена товара ниже заданного минимального ограничения.",
		Severity:      sev,
		Urgency:       urg,
		EvidencePayload: BuildPriceEvidence(*p.ReferencePrice, p.EffectiveMinPrice, p.EffectiveMaxPrice, p.ImpliedCost, nil, EvidencePayload{
			"metric":              "price_below_min_constraint",
			"product_id":          p.OzonProductID,
			"ozon_product_id":     p.OzonProductID,
			"sku":                 p.SKU,
			"offer_id":            p.OfferID,
			"product_name":        p.ProductName,
			"current_price":       *p.ReferencePrice,
			"effective_min_price": *p.EffectiveMinPrice,
			"delta_absolute":      deltaAbs,
			"delta_percent":       deltaPct,
			"constraint_source":   p.ConstraintSource,
			"rule_id":             p.ConstraintRuleID,
			"as_of_date":          input.AsOfDate,
		}),
	}, true
}

func evaluatePriceAboveMaxConstraint(input PriceRuleEvaluationInput, p ProductPricingContext) (RuleResult, bool) {
	if p.ReferencePrice == nil || *p.ReferencePrice <= 0 || p.EffectiveMaxPrice == nil {
		return RuleResult{}, false
	}
	if *p.ReferencePrice <= *p.EffectiveMaxPrice {
		return RuleResult{}, false
	}
	deltaAbs := *p.ReferencePrice - *p.EffectiveMaxPrice
	deltaPct := deltaAbs / *p.EffectiveMaxPrice * 100.0
	entityID := fmt.Sprintf("%d", p.OzonProductID)
	sev := SeverityMedium
	urg := UrgencyMedium
	if deltaPct > 10 {
		sev = SeverityHigh
		urg = UrgencyHigh
	}
	return RuleResult{
		AlertType:     AlertTypePriceAboveMaxConstraint,
		AlertGroup:    AlertGroupPriceEconomics,
		EntityType:    EntityTypeProduct,
		EntityID:      &entityID,
		EntitySKU:     p.SKU,
		EntityOfferID: p.OfferID,
		Title:         "Цена выше максимального ограничения",
		Message:       "Текущая цена товара выше заданного максимального ограничения.",
		Severity:      sev,
		Urgency:       urg,
		EvidencePayload: BuildPriceEvidence(*p.ReferencePrice, p.EffectiveMinPrice, p.EffectiveMaxPrice, p.ImpliedCost, nil, EvidencePayload{
			"metric":              "price_above_max_constraint",
			"product_id":          p.OzonProductID,
			"ozon_product_id":     p.OzonProductID,
			"sku":                 p.SKU,
			"offer_id":            p.OfferID,
			"product_name":        p.ProductName,
			"current_price":       *p.ReferencePrice,
			"effective_max_price": *p.EffectiveMaxPrice,
			"delta_absolute":      deltaAbs,
			"delta_percent":       deltaPct,
			"constraint_source":   p.ConstraintSource,
			"rule_id":             p.ConstraintRuleID,
			"as_of_date":          input.AsOfDate,
		}),
	}, true
}

func evaluateMarginRiskAtCurrentPrice(input PriceRuleEvaluationInput, p ProductPricingContext) (RuleResult, bool) {
	if p.ReferencePrice == nil || *p.ReferencePrice <= 0 || p.ImpliedCost == nil {
		return RuleResult{}, false
	}
	expectedMargin, err := pricingconstraints.ComputeExpectedMargin(*p.ReferencePrice, *p.ImpliedCost)
	if err != nil || expectedMargin >= marginRiskThreshold {
		return RuleResult{}, false
	}

	entityID := fmt.Sprintf("%d", p.OzonProductID)
	sev, urg := severityUrgencyForExpectedMargin(expectedMargin)
	return RuleResult{
		AlertType:     AlertTypeMarginRiskAtCurrentPrice,
		AlertGroup:    AlertGroupPriceEconomics,
		EntityType:    EntityTypeProduct,
		EntityID:      &entityID,
		EntitySKU:     p.SKU,
		EntityOfferID: p.OfferID,
		Title:         "Риск низкой маржинальности",
		Message:       "Ожидаемая маржа при текущей цене ниже безопасного порога.",
		Severity:      sev,
		Urgency:       urg,
		EvidencePayload: BuildPriceEvidence(*p.ReferencePrice, p.EffectiveMinPrice, p.EffectiveMaxPrice, p.ImpliedCost, &expectedMargin, EvidencePayload{
			"metric":            "expected_margin",
			"product_id":        p.OzonProductID,
			"ozon_product_id":   p.OzonProductID,
			"sku":               p.SKU,
			"offer_id":          p.OfferID,
			"product_name":      p.ProductName,
			"current_price":     *p.ReferencePrice,
			"implied_cost":      *p.ImpliedCost,
			"expected_margin":   expectedMargin,
			"threshold_margin":  marginRiskThreshold,
			"constraint_source": p.ConstraintSource,
			"rule_id":           p.ConstraintRuleID,
			"as_of_date":        input.AsOfDate,
		}),
	}, true
}

func evaluateMissingPricingConstraintsForKeySKU(input PriceRuleEvaluationInput, p ProductPricingContext) (RuleResult, bool) {
	if p.HasEffectiveConstraint {
		return RuleResult{}, false
	}
	importance, ok := input.SKUImportanceByProduct[p.OzonProductID]
	if !ok || !importance.IsKeySKU {
		return RuleResult{}, false
	}
	entityID := fmt.Sprintf("%d", p.OzonProductID)
	sev := SeverityMedium
	urg := UrgencyMedium
	if isHighPriorityKeySKU(importance) {
		sev = SeverityHigh
		urg = UrgencyHigh
	}
	evidence := MergeKeySKUImportanceEvidence(EvidencePayload{
		"metric":          "missing_pricing_constraints",
		"product_id":      p.OzonProductID,
		"ozon_product_id": p.OzonProductID,
		"sku":             p.SKU,
		"offer_id":        p.OfferID,
		"product_name":    p.ProductName,
		"period":          "relative_seller_context_7d",
		"as_of_date":      input.AsOfDate,
	}, importance)
	return RuleResult{
		AlertType:       AlertTypeMissingPricingConstraintsForKeySKU,
		AlertGroup:      AlertGroupPriceEconomics,
		EntityType:      EntityTypeProduct,
		EntityID:        &entityID,
		EntitySKU:       p.SKU,
		EntityOfferID:   p.OfferID,
		Title:           "Важный SKU без ценовых ограничений",
		Message:         "SKU значим для кабинета за 7 дней, но для него не заданы pricing constraints.",
		Severity:        sev,
		Urgency:         urg,
		EvidencePayload: BuildPriceEvidence(valueOrZero(p.ReferencePrice), nil, nil, nil, nil, evidence),
	}, true
}

func severityUrgencyForExpectedMargin(margin float64) (Severity, Urgency) {
	if margin < 0 {
		return SeverityCritical, UrgencyImmediate
	}
	if margin < 0.05 {
		return SeverityHigh, UrgencyHigh
	}
	return SeverityMedium, UrgencyMedium
}

func valueOrZero(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}
