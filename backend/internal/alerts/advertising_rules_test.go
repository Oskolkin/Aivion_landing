package alerts

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestAdGoodCampaignDoesNotAlert(t *testing.T) {
	campaign := AdCampaignMetricSummary{
		CampaignExternalID: 1,
		CampaignName:       "Good",
		Spend:              10_000,
		Revenue:            200_000,
		Orders:             50,
		Impressions:        1000,
		Clicks:             100,
	}
	if _, ok := evaluateAdWeakCampaignEfficiency(baseAdvertisingInput(), campaign); ok {
		t.Fatal("must not alert for low DRR campaign")
	}
}

func TestAdHighDRRCreatesHighAlert(t *testing.T) {
	campaign := AdCampaignMetricSummary{
		CampaignExternalID: 2,
		CampaignName:       "Weak",
		Spend:              50_000,
		Revenue:            80_000,
		Orders:             10,
	}
	rr, ok := evaluateAdWeakCampaignEfficiency(baseAdvertisingInput(), campaign)
	if !ok {
		t.Fatal("expected ad_weak_campaign_efficiency")
	}
	if rr.AlertType != AlertTypeAdWeakCampaignEfficiency {
		t.Fatalf("unexpected type: %s", rr.AlertType)
	}
	if rr.Severity != SeverityHigh {
		t.Fatalf("expected high severity, got %s", rr.Severity)
	}
	drr, _ := rr.EvidencePayload["drr"].(float64)
	if drr < 0.624 || drr > 0.626 {
		t.Fatalf("unexpected drr: %v", drr)
	}
	if rr.EvidencePayload["reason"] != adReasonHighDRR {
		t.Fatalf("unexpected reason: %v", rr.EvidencePayload["reason"])
	}
}

func TestAdCriticalDRRCreatesCriticalAlert(t *testing.T) {
	campaign := AdCampaignMetricSummary{
		CampaignExternalID: 3,
		Spend:              80_000,
		Revenue:            100_000,
		Orders:             5,
	}
	rr, ok := evaluateAdWeakCampaignEfficiency(baseAdvertisingInput(), campaign)
	if !ok {
		t.Fatal("expected ad_weak_campaign_efficiency")
	}
	if rr.Severity != SeverityCritical {
		t.Fatalf("expected critical severity, got %s", rr.Severity)
	}
}

func TestAdZeroRevenueCreatesWeakEfficiencyNotSpendWithoutResult(t *testing.T) {
	campaign := AdCampaignMetricSummary{
		CampaignExternalID: 4,
		Spend:              5_000,
		Revenue:            0,
		Orders:             0,
	}
	rr, ok := evaluateAdWeakCampaignEfficiency(baseAdvertisingInput(), campaign)
	if !ok {
		t.Fatal("expected ad_weak_campaign_efficiency")
	}
	if rr.AlertType != AlertTypeAdWeakCampaignEfficiency {
		t.Fatalf("unexpected type: %s", rr.AlertType)
	}
	if rr.EvidencePayload["drr"] != nil {
		t.Fatal("drr must be null when revenue is zero")
	}
	if rr.EvidencePayload["drr_percent"] != nil {
		t.Fatal("drr_percent must be null when revenue is zero")
	}
}

func TestAdZeroOrdersWithRevenue(t *testing.T) {
	campaign := AdCampaignMetricSummary{
		CampaignExternalID: 5,
		Spend:              2_000,
		Revenue:            10_000,
		Orders:             0,
	}
	rr, ok := evaluateAdWeakCampaignEfficiency(baseAdvertisingInput(), campaign)
	if !ok {
		t.Fatal("expected zero-result style alert")
	}
	if rr.EvidencePayload["cpo"] != nil {
		t.Fatal("cpo must be null when orders_count is zero")
	}
}

func TestAdTinySpendZeroResultNoAlert(t *testing.T) {
	campaign := AdCampaignMetricSummary{
		CampaignExternalID: 6,
		Spend:              500,
		Revenue:            0,
		Orders:             0,
	}
	if _, ok := evaluateAdWeakCampaignEfficiency(baseAdvertisingInput(), campaign); ok {
		t.Fatal("must not alert below noise floor")
	}
}

func TestEvaluateAdvertisingRulesOnlyWeakEfficiency(t *testing.T) {
	input := baseAdvertisingInput()
	input.CampaignSummaries = []AdCampaignMetricSummary{
		{CampaignExternalID: 1, CampaignName: "A", Spend: 50_000, Revenue: 80_000, Orders: 5},
		{CampaignExternalID: 2, CampaignName: "B", Spend: 5_000, Revenue: 0, Orders: 0},
		{CampaignExternalID: 3, CampaignName: "C", Spend: 10_000, Revenue: 200_000, Orders: 20},
	}
	res := EvaluateAdvertisingRules(input)
	for _, rr := range res.RuleResults {
		if rr.AlertType != AlertTypeAdWeakCampaignEfficiency {
			t.Fatalf("unexpected alert type: %s", rr.AlertType)
		}
	}
	deprecated := map[AlertType]bool{
		AlertTypeAdSpendWithoutResult:  true,
		AlertTypeAdBudgetOnWeakSKU:     true,
		AlertTypeAdBudgetOnLowStockSKU: true,
	}
	for _, rr := range res.RuleResults {
		if deprecated[rr.AlertType] {
			t.Fatalf("deprecated alert type generated: %s", rr.AlertType)
		}
	}
}

func TestAdEvidenceNoNaNOrInf(t *testing.T) {
	campaign := AdCampaignMetricSummary{
		CampaignExternalID: 7,
		Spend:              2_000,
		Revenue:            0,
		Orders:             0,
		Impressions:        0,
		Clicks:             0,
	}
	rr, ok := evaluateAdWeakCampaignEfficiency(baseAdvertisingInput(), campaign)
	if !ok {
		t.Fatal("expected alert")
	}
	raw, err := json.Marshal(rr.EvidencePayload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(raw)
	if strings.Contains(s, "NaN") || strings.Contains(s, "Infinity") {
		t.Fatalf("invalid evidence: %s", s)
	}
}

func TestAdCampaignFingerprintStable(t *testing.T) {
	campaign := AdCampaignMetricSummary{
		CampaignExternalID: 99,
		CampaignName:       "FP",
		Spend:              50_000,
		Revenue:            80_000,
		Orders:             5,
	}
	rr, ok := evaluateAdWeakCampaignEfficiency(baseAdvertisingInput(), campaign)
	if !ok {
		t.Fatal("expected alert")
	}
	entityID := fmt.Sprintf("%d", campaign.CampaignExternalID)
	if rr.EntityID == nil || *rr.EntityID != entityID {
		t.Fatalf("entity id=%v", rr.EntityID)
	}
	inA := RuleResultToUpsertInput(1, rr)
	inB := RuleResultToUpsertInput(1, rr)
	if inA.Fingerprint != inB.Fingerprint {
		t.Fatal("fingerprint must be stable")
	}
	fpSKU := BuildFingerprint(1, AlertTypeAdWeakCampaignEfficiency, EntityTypeCampaign,
		BuildEntityIdentity(EntityTypeCampaign, &entityID, int64Ptr(123), strPtr("offer")))
	if inA.Fingerprint == fpSKU {
		t.Fatal("fingerprint must not include SKU")
	}
}

func TestEvaluateAdvertisingRulesNoSKULevelAlerts(t *testing.T) {
	days := 2.0
	sku := baseAdSKU()
	sku.CurrentStock = 0
	sku.DaysOfCover = &days
	input := baseAdvertisingInput()
	input.CampaignSummaries = []AdCampaignMetricSummary{
		{CampaignExternalID: 10, CampaignName: "X", Spend: 5000, Revenue: 100000, Orders: 5},
	}
	res := EvaluateAdvertisingRules(input)
	for _, rr := range res.RuleResults {
		if rr.EntityType == EntityTypeSKU {
			t.Fatalf("SKU-level ad alert must not be generated: %s", rr.AlertType)
		}
	}
}

func baseAdvertisingInput() AdvertisingRuleEvaluationInput {
	return AdvertisingRuleEvaluationInput{
		SellerAccountID: 1,
		DateFrom:        "2026-04-24",
		DateTo:          "2026-04-30",
		AsOfDate:        "2026-04-30",
	}
}

func baseAdSKU() SKUDailyMetric {
	sku := int64(321)
	offer := "offer-321"
	name := "Ad SKU"
	return SKUDailyMetric{
		SellerAccountID: 1,
		OzonProductID:   999,
		SKU:             &sku,
		OfferID:         &offer,
		ProductName:     &name,
	}
}

func int64Ptr(v int64) *int64 { return &v }
func strPtr(v string) *string { return &v }
