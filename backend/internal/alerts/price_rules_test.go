package alerts

import "testing"

func TestPriceBelowMinConstraintTriggers(t *testing.T) {
	ref := 90.0
	minp := 100.0
	p := basePriceContext()
	p.ReferencePrice = &ref
	p.EffectiveMinPrice = &minp
	p.HasEffectiveConstraint = true

	if _, ok := evaluatePriceBelowMinConstraint(basePriceInput([]ProductPricingContext{p}, nil), p); !ok {
		t.Fatal("expected price_below_min_constraint")
	}
}

func TestPriceAboveMaxConstraintTriggers(t *testing.T) {
	ref := 120.0
	maxp := 100.0
	p := basePriceContext()
	p.ReferencePrice = &ref
	p.EffectiveMaxPrice = &maxp
	p.HasEffectiveConstraint = true

	if _, ok := evaluatePriceAboveMaxConstraint(basePriceInput([]ProductPricingContext{p}, nil), p); !ok {
		t.Fatal("expected price_above_max_constraint")
	}
}

func TestMarginRiskAtCurrentPriceTriggers(t *testing.T) {
	ref := 100.0
	cost := 95.0
	p := basePriceContext()
	p.ReferencePrice = &ref
	p.ImpliedCost = &cost
	p.HasEffectiveConstraint = true

	if _, ok := evaluateMarginRiskAtCurrentPrice(basePriceInput([]ProductPricingContext{p}, nil), p); !ok {
		t.Fatal("expected margin_risk_at_current_price")
	}
}

func TestMissingPricingConstraintsForKeySKUTiggers(t *testing.T) {
	p := basePriceContext()
	p.OzonProductID = 1
	p.HasEffectiveConstraint = false

	importance := BuildSKUImportanceFromDailyMetrics([]SKUDailyMetric{
		dailyMetric(1, 100_000, 10),
		dailyMetric(2, 100, 5),
	})
	if _, ok := evaluateMissingPricingConstraintsForKeySKU(basePriceInput([]ProductPricingContext{p}, importance), p); !ok {
		t.Fatal("expected missing_pricing_constraints_for_key_sku")
	}
}

func TestMissingPricingConstraintsNotTriggeredForNonKeySKU(t *testing.T) {
	p := basePriceContext()
	p.OzonProductID = 500
	p.HasEffectiveConstraint = false

	metrics := make([]SKUDailyMetric, 0, 1000)
	for i := 0; i < 1000; i++ {
		pid := int64(i + 1)
		revenue := 1_000_000.0
		orders := int32(100)
		if pid == p.OzonProductID {
			revenue = 10_000
			orders = 5
		}
		metrics = append(metrics, dailyMetric(pid, revenue, orders))
	}
	importance := BuildSKUImportanceFromDailyMetrics(metrics)
	if _, ok := evaluateMissingPricingConstraintsForKeySKU(basePriceInput([]ProductPricingContext{p}, importance), p); ok {
		t.Fatal("must not trigger for non-key sku in large seller catalog")
	}
}

func TestMissingPricingConstraintsRequiresNoEffectiveConstraint(t *testing.T) {
	p := basePriceContext()
	p.OzonProductID = 1
	p.HasEffectiveConstraint = true
	importance := BuildSKUImportanceFromDailyMetrics([]SKUDailyMetric{
		dailyMetric(1, 100_000, 10),
	})
	if _, ok := evaluateMissingPricingConstraintsForKeySKU(basePriceInput([]ProductPricingContext{p}, importance), p); ok {
		t.Fatal("must not trigger when effective constraint exists")
	}
}

func TestNoPriceAlertsWithoutEffectiveConstraint(t *testing.T) {
	ref := 100.0
	p := basePriceContext()
	p.ReferencePrice = &ref
	p.HasEffectiveConstraint = false

	if _, ok := evaluatePriceBelowMinConstraint(basePriceInput([]ProductPricingContext{p}, nil), p); ok {
		t.Fatal("must not trigger below-min without min constraint")
	}
	if _, ok := evaluatePriceAboveMaxConstraint(basePriceInput([]ProductPricingContext{p}, nil), p); ok {
		t.Fatal("must not trigger above-max without max constraint")
	}
	if _, ok := evaluateMarginRiskAtCurrentPrice(basePriceInput([]ProductPricingContext{p}, nil), p); ok {
		t.Fatal("must not trigger margin risk without implied cost")
	}
}

func TestMissingReferencePriceDoesNotPanic(t *testing.T) {
	p := basePriceContext()
	p.ReferencePrice = nil
	p.HasEffectiveConstraint = true

	if _, ok := evaluatePriceBelowMinConstraint(basePriceInput([]ProductPricingContext{p}, nil), p); ok {
		t.Fatal("must not trigger without reference price")
	}
	if _, ok := evaluateMarginRiskAtCurrentPrice(basePriceInput([]ProductPricingContext{p}, nil), p); ok {
		t.Fatal("must not trigger margin risk without reference price")
	}
}

func basePriceInput(products []ProductPricingContext, importance SKUImportanceByProduct) PriceRuleEvaluationInput {
	return PriceRuleEvaluationInput{
		SellerAccountID:        1,
		AsOfDate:               "2026-04-30",
		Products:               products,
		SKUImportanceByProduct: importance,
	}
}

func basePriceContext() ProductPricingContext {
	sku := int64(444)
	offer := "offer-444"
	return ProductPricingContext{
		SellerAccountID: 1,
		OzonProductID:   444,
		SKU:             &sku,
		OfferID:         &offer,
		ProductName:     "Price SKU",
	}
}
