package alerts

import (
	"testing"
	"time"
)

func TestSalesRevenueDropRuleTriggers(t *testing.T) {
	in := baseSalesInput()
	in.CurrentAccount.Revenue = 60
	in.PreviousAccount.Revenue = 100

	rr, ok := evaluateSalesRevenueDrop(in)
	if !ok {
		t.Fatal("expected revenue drop alert")
	}
	if rr.AlertType != AlertTypeSalesRevenueDrop {
		t.Fatalf("unexpected alert type: %s", rr.AlertType)
	}
}

func TestSalesOrdersDropRuleTriggers(t *testing.T) {
	in := baseSalesInput()
	in.CurrentAccount.OrdersCount = 50
	in.PreviousAccount.OrdersCount = 100

	rr, ok := evaluateSalesOrdersDrop(in)
	if !ok {
		t.Fatal("expected orders drop alert")
	}
	if rr.AlertType != AlertTypeSalesOrdersDrop {
		t.Fatalf("unexpected alert type: %s", rr.AlertType)
	}
}

func TestSKUOrdersDropTriggersOnOrdersNotRevenue(t *testing.T) {
	in := baseSalesInput()
	current := SKUDailyMetric{OzonProductID: 1, Revenue: 2000, OrdersCount: 3}
	prev := SKUDailyMetric{OzonProductID: 1, Revenue: 500, OrdersCount: 10}
	rr, ok := evaluateSKUOrdersDrop(in, current, prev)
	if !ok {
		t.Fatal("expected sku orders drop when orders fell sharply")
	}
	if rr.AlertType != AlertTypeSKUOrdersDrop {
		t.Fatalf("unexpected type: %s", rr.AlertType)
	}
	ev := rr.EvidencePayload
	if ev["current_orders"] != int32(3) || ev["comparison_orders"] != int32(10) {
		t.Fatalf("unexpected orders evidence: %+v", ev)
	}
}

func TestSKUOrdersDropSkippedWhenOnlyRevenueFalls(t *testing.T) {
	in := baseSalesInput()
	current := SKUDailyMetric{OzonProductID: 1, Revenue: 100, OrdersCount: 10}
	prev := SKUDailyMetric{OzonProductID: 1, Revenue: 2000, OrdersCount: 10}
	if _, ok := evaluateSKUOrdersDrop(in, current, prev); ok {
		t.Fatal("must not alert on revenue-only drop when orders unchanged")
	}
}

func TestEvaluateSalesRulesDoesNotEmitSKURevenueDrop(t *testing.T) {
	in := baseSalesInput()
	in.CurrentSKUs = []SKUDailyMetric{{OzonProductID: 1, Revenue: 500, OrdersCount: 3}}
	in.PreviousSKUsByID = map[int64]SKUDailyMetric{
		1: {OzonProductID: 1, Revenue: 5000, OrdersCount: 10},
	}
	eval := EvaluateSalesRules(in)
	for _, rr := range eval.RuleResults {
		if rr.AlertType == AlertTypeSKURevenueDrop {
			t.Fatal("active pipeline must not emit sku_revenue_drop")
		}
	}
	hasOrdersDrop := false
	for _, rr := range eval.RuleResults {
		if rr.AlertType == AlertTypeSKUOrdersDrop {
			hasOrdersDrop = true
		}
	}
	if !hasOrdersDrop {
		t.Fatal("expected sku_orders_drop in rule results")
	}
}

func TestSKURevenueDropLegacyRuleStillCallable(t *testing.T) {
	in := baseSalesInput()
	current := SKUDailyMetric{OzonProductID: 1, Revenue: 500}
	prevBig := SKUDailyMetric{OzonProductID: 1, Revenue: 1200}
	if _, ok := evaluateSKURevenueDrop(in, current, prevBig); !ok {
		t.Fatal("legacy evaluateSKURevenueDrop should still be callable")
	}
}

func TestSKUNegativeContributionRuleTriggers(t *testing.T) {
	in := baseSalesInput()
	current := SKUDailyMetric{OzonProductID: 1, Revenue: 700}
	previous := SKUDailyMetric{OzonProductID: 1, Revenue: 1000}
	accountDelta := -1000.0

	rr, ok := evaluateSKUNegativeContribution(in, current, previous, accountDelta)
	if !ok {
		t.Fatal("expected negative contribution alert")
	}
	if rr.AlertType != AlertTypeSKUNegativeContribution {
		t.Fatalf("unexpected alert type: %s", rr.AlertType)
	}
}

func TestPreviousZeroDoesNotCreateAlertOrPanic(t *testing.T) {
	in := baseSalesInput()
	in.CurrentAccount.Revenue = 10
	in.PreviousAccount.Revenue = 0
	if _, ok := evaluateSalesRevenueDrop(in); ok {
		t.Fatal("must not create revenue alert when previous=0")
	}

	in.CurrentAccount.OrdersCount = 10
	in.PreviousAccount.OrdersCount = 0
	if _, ok := evaluateSalesOrdersDrop(in); ok {
		t.Fatal("must not create orders alert when previous=0")
	}
}

func baseSalesInput() SalesRuleEvaluationInput {
	asOf := time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)
	prev := asOf.AddDate(0, 0, -1)
	return SalesRuleEvaluationInput{
		SellerAccountID: 1,
		AsOfDate:        asOf,
		PreviousDate:    prev,
		CurrentAccount: &AccountDailyMetric{
			SellerAccountID: 1,
			MetricDate:      asOf,
			Revenue:         100,
			OrdersCount:     100,
		},
		PreviousAccount: &AccountDailyMetric{
			SellerAccountID: 1,
			MetricDate:      prev,
			Revenue:         100,
			OrdersCount:     100,
		},
	}
}
