package alerts

import "testing"

func assertStockAlert(t *testing.T, rr RuleResult, ok bool, wantOK bool, wantType AlertType, wantSev Severity, wantUrg Urgency, wantReason string) {
	t.Helper()
	if ok != wantOK {
		t.Fatalf("ok=%v want %v", ok, wantOK)
	}
	if !wantOK {
		return
	}
	if rr.AlertType != wantType {
		t.Fatalf("type=%s want %s", rr.AlertType, wantType)
	}
	if rr.Severity != wantSev {
		t.Fatalf("severity=%s want %s", rr.Severity, wantSev)
	}
	if rr.Urgency != wantUrg {
		t.Fatalf("urgency=%s want %s", rr.Urgency, wantUrg)
	}
	if rr.EvidencePayload["reason"] != wantReason {
		t.Fatalf("reason=%v want %s", rr.EvidencePayload["reason"], wantReason)
	}
}

func TestStockLowCoverageAt13Days(t *testing.T) {
	days := 13.0
	sku := baseStockSKU()
	sku.CurrentStock = 20
	sku.DaysOfCover = &days
	rr, ok := evaluateStockLowCoverage(baseStockInput([]SKUDailyMetric{sku}, nil), sku)
	assertStockAlert(t, rr, ok, true, AlertTypeStockLowCoverage, SeverityMedium, UrgencyMedium, stockReasonSupplyPlanning)
}

func TestStockLowCoverageAt10Days(t *testing.T) {
	days := 10.0
	sku := baseStockSKU()
	sku.CurrentStock = 20
	sku.DaysOfCover = &days
	rr, ok := evaluateStockLowCoverage(baseStockInput([]SKUDailyMetric{sku}, nil), sku)
	assertStockAlert(t, rr, ok, true, AlertTypeStockLowCoverage, SeverityMedium, UrgencyMedium, stockReasonSupplyPlanning)
}

func TestStockOOSRiskAt6Days(t *testing.T) {
	days := 6.0
	sku := baseStockSKU()
	sku.CurrentStock = 10
	sku.DaysOfCover = &days
	rr, ok := evaluateStockOOSRisk(baseStockInput([]SKUDailyMetric{sku}, nil), sku)
	assertStockAlert(t, rr, ok, true, AlertTypeStockOOSRisk, SeverityHigh, UrgencyHigh, stockReasonLastCall)
}

func TestStockOOSRiskAt2Days(t *testing.T) {
	days := 2.0
	sku := baseStockSKU()
	sku.CurrentStock = 5
	sku.DaysOfCover = &days
	rr, ok := evaluateStockOOSRisk(baseStockInput([]SKUDailyMetric{sku}, nil), sku)
	assertStockAlert(t, rr, ok, true, AlertTypeStockOOSRisk, SeverityCritical, UrgencyImmediate, stockReasonImminentOOS)
}

func TestStockNoAlertAt15Days(t *testing.T) {
	days := 15.0
	sku := baseStockSKU()
	sku.CurrentStock = 50
	sku.DaysOfCover = &days
	in := baseStockInput([]SKUDailyMetric{sku}, nil)
	if _, ok := evaluateStockLowCoverage(in, sku); ok {
		t.Fatal("stock_low_coverage must not trigger at 15 days")
	}
	if _, ok := evaluateStockOOSRisk(in, sku); ok {
		t.Fatal("stock_oos_risk must not trigger at 15 days")
	}
}

func TestStockNoAlertZeroStockNilDays(t *testing.T) {
	sku := baseStockSKU()
	sku.CurrentStock = 0
	sku.DaysOfCover = nil
	in := baseStockInput([]SKUDailyMetric{sku}, nil)
	if _, ok := evaluateStockOOSRisk(in, sku); ok {
		t.Fatal("must not alert on zero stock alone")
	}
	if _, ok := evaluateStockLowCoverage(in, sku); ok {
		t.Fatal("must not alert on zero stock alone")
	}
}

func TestStockNoAlertNegativeStockZeroDays(t *testing.T) {
	days := 0.0
	sku := baseStockSKU()
	sku.CurrentStock = -1
	sku.DaysOfCover = &days
	in := baseStockInput([]SKUDailyMetric{sku}, nil)
	if _, ok := evaluateStockOOSRisk(in, sku); ok {
		t.Fatal("must not alert on negative stock with zero days")
	}
}

func TestStockLowCoverageAt10DaysNotOldSevenDayRule(t *testing.T) {
	days := 10.0
	sku := baseStockSKU()
	sku.CurrentStock = 12
	sku.DaysOfCover = &days
	if _, ok := evaluateStockLowCoverage(baseStockInput([]SKUDailyMetric{sku}, nil), sku); !ok {
		t.Fatal("days_of_cover=10 must create stock_low_coverage under new FBO thresholds")
	}
}

func TestStockCriticalKeySKUAt12Days(t *testing.T) {
	days := 12.0
	sku := baseStockSKU()
	sku.OzonProductID = 1
	sku.CurrentStock = 30
	sku.DaysOfCover = &days
	importance := BuildSKUImportanceFromDailyMetrics([]SKUDailyMetric{dailyMetric(1, 100_000, 10)})
	rr, ok := evaluateStockCriticalSKULowStock(baseStockInput([]SKUDailyMetric{sku}, importance), sku)
	assertStockAlert(t, rr, ok, true, AlertTypeStockCriticalSKULowStock, SeverityHigh, UrgencyHigh, stockReasonKeySupplyPlanning)
}

func TestStockCriticalKeySKUAt6Days(t *testing.T) {
	days := 6.0
	sku := baseStockSKU()
	sku.OzonProductID = 1
	sku.CurrentStock = 8
	sku.DaysOfCover = &days
	importance := BuildSKUImportanceFromDailyMetrics([]SKUDailyMetric{dailyMetric(1, 100_000, 10)})
	rr, ok := evaluateStockCriticalSKULowStock(baseStockInput([]SKUDailyMetric{sku}, importance), sku)
	assertStockAlert(t, rr, ok, true, AlertTypeStockCriticalSKULowStock, SeverityCritical, UrgencyHigh, stockReasonKeyLastCall)
}

func TestStockCriticalKeySKUAt2Days(t *testing.T) {
	days := 2.0
	sku := baseStockSKU()
	sku.OzonProductID = 1
	sku.CurrentStock = 3
	sku.DaysOfCover = &days
	importance := BuildSKUImportanceFromDailyMetrics([]SKUDailyMetric{dailyMetric(1, 100_000, 10)})
	rr, ok := evaluateStockCriticalSKULowStock(baseStockInput([]SKUDailyMetric{sku}, importance), sku)
	assertStockAlert(t, rr, ok, true, AlertTypeStockCriticalSKULowStock, SeverityCritical, UrgencyImmediate, stockReasonKeyImminentOOS)
}

func TestStockCriticalKeySKUNoAlertZeroStock(t *testing.T) {
	sku := baseStockSKU()
	sku.OzonProductID = 1
	sku.CurrentStock = 0
	sku.DaysOfCover = nil
	importance := BuildSKUImportanceFromDailyMetrics([]SKUDailyMetric{dailyMetric(1, 100_000, 10)})
	if _, ok := evaluateStockCriticalSKULowStock(baseStockInput([]SKUDailyMetric{sku}, importance), sku); ok {
		t.Fatal("key SKU must not alert on zero stock alone")
	}
}

func TestStockCriticalSKULowStockOnlyForKeySKU(t *testing.T) {
	days := 6.0
	sku := baseStockSKU()
	sku.OzonProductID = 500
	sku.CurrentStock = 5
	sku.DaysOfCover = &days

	largeSellerMetrics := make([]SKUDailyMetric, 0, 1000)
	for i := 0; i < 1000; i++ {
		pid := int64(i + 1)
		revenue := 1_000_000.0
		orders := int32(100)
		if pid == sku.OzonProductID {
			revenue = 10_000
			orders = 5
		}
		largeSellerMetrics = append(largeSellerMetrics, dailyMetric(pid, revenue, orders))
	}
	importance := BuildSKUImportanceFromDailyMetrics(largeSellerMetrics)
	if _, ok := evaluateStockCriticalSKULowStock(baseStockInput([]SKUDailyMetric{sku}, importance), sku); ok {
		t.Fatal("must not alert for non-key sku in large seller catalog")
	}

	sku.OzonProductID = 1
	smallSellerMetrics := make([]SKUDailyMetric, 0, 20)
	for i := 0; i < 20; i++ {
		pid := int64(i + 1)
		revenue := float64(20 - i)
		orders := int32(1)
		if pid == sku.OzonProductID {
			revenue = 10_000
			orders = 5
		}
		smallSellerMetrics = append(smallSellerMetrics, dailyMetric(pid, revenue, orders))
	}
	smallImportance := BuildSKUImportanceFromDailyMetrics(smallSellerMetrics)
	if _, ok := evaluateStockCriticalSKULowStock(baseStockInput([]SKUDailyMetric{sku}, smallImportance), sku); !ok {
		t.Fatal("expected alert for key sku in small seller catalog")
	}
}

func TestStockLowCoverageNoPanicWhenDaysMissing(t *testing.T) {
	sku := baseStockSKU()
	sku.CurrentStock = 10
	sku.DaysOfCover = nil
	if _, ok := evaluateStockLowCoverage(baseStockInput([]SKUDailyMetric{sku}, nil), sku); ok {
		t.Fatal("must not alert with missing days_of_cover")
	}
}

func TestEvaluateStockRulesNoData(t *testing.T) {
	res := EvaluateStockRules(baseStockInput(nil, nil))
	if len(res.RuleResults) != 0 {
		t.Fatalf("expected no alerts, got %d", len(res.RuleResults))
	}
}

func TestStockAlertFingerprintStable(t *testing.T) {
	days := 2.0
	sku := baseStockSKU()
	sku.CurrentStock = 3
	sku.DaysOfCover = &days
	rr, ok := evaluateStockOOSRisk(baseStockInput([]SKUDailyMetric{sku}, nil), sku)
	if !ok {
		t.Fatal("expected oos alert")
	}
	inA := RuleResultToUpsertInput(77, rr)
	inB := RuleResultToUpsertInput(77, rr)
	if inA.Fingerprint != inB.Fingerprint {
		t.Fatal("fingerprint must be stable")
	}
}

func baseStockInput(metrics []SKUDailyMetric, importance SKUImportanceByProduct) StockRuleEvaluationInput {
	return StockRuleEvaluationInput{
		SellerAccountID:        1,
		AsOfDate:               "2026-04-30",
		SKUMetrics:             metrics,
		SKUImportanceByProduct: importance,
		FBOStorageRiskBySKU:    FBOStorageRiskBySKU{},
	}
}

func baseStockSKU() SKUDailyMetric {
	sku := int64(123456)
	offer := "offer-123456"
	name := "Test SKU"
	return SKUDailyMetric{
		SellerAccountID: 1,
		OzonProductID:   111,
		SKU:             &sku,
		OfferID:         &offer,
		ProductName:     &name,
	}
}
