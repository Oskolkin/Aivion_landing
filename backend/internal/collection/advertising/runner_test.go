package advertising

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon/performance"
)

func TestCampaignSemanticsRemainConservative(t *testing.T) {
	tests := []struct {
		campaign      performance.Campaign
		family, scope string
	}{
		{performance.Campaign{CampaignType: "MEDIA"}, "MEDIA_CPM", "NON_SKU_OBJECTS"},
		{performance.Campaign{CampaignType: "SKU", PaymentType: "CPC"}, "PRODUCT_CPC", "UNKNOWN"},
		{performance.Campaign{CampaignType: "SEARCH", PaymentType: "CPO"}, "SEARCH_PROMO_CPO", "UNKNOWN"},
		{performance.Campaign{CampaignType: "new-provider-kind"}, "UNKNOWN", "UNKNOWN"},
	}
	for _, tt := range tests {
		family, scope := campaignSemantics(tt.campaign)
		if family != tt.family || scope != tt.scope {
			t.Fatalf("%+v => %s/%s", tt.campaign, family, scope)
		}
	}
}

func TestCampaignAndTargetHashesIncludeMutableState(t *testing.T) {
	a := CampaignObservation{Title: "A", ProviderState: "ACTIVE", CurrencyCode: "RUB"}
	b := a
	b.ProviderState = "PAUSED"
	if campaignHash(a) == campaignHash(b) {
		t.Fatal("campaign state change was compressed")
	}
	t1 := TargetObservation{AssignmentState: "ACTIVE", BidUnit: "micro_rubles"}
	t2 := t1
	t2.AssignmentState = "INACTIVE"
	if targetHash(t1) == targetHash(t2) {
		t.Fatal("target interval state change was compressed")
	}
}

func TestDailyNormalizeKeepsPaidBonusAndProviderDRRSeparate(t *testing.T) {
	obs, err := normalizeDaily(performance.CampaignDailyMetric{
		CampaignExternalID: 7,
		MetricDate:         time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC),
		Impressions:        10,
		ImpressionsPresent: true,
		Clicks:             2,
		ClicksPresent:      true,
		Spend:              "40",
		BonusSpend:         "5",
		OrdersCount:        1,
		OrdersCountPresent: true,
		Revenue:            "100",
		ProviderDRR:        "40",
	}, 9)
	if err != nil {
		t.Fatal(err)
	}
	if obs.PaidSpendAmount == nil || obs.PaidSpendAmount.String() != "40" {
		t.Fatalf("paid=%v", obs.PaidSpendAmount)
	}
	if obs.BonusSpendAmount == nil || obs.BonusSpendAmount.String() != "5" {
		t.Fatalf("bonus=%v", obs.BonusSpendAmount)
	}
	if obs.ProviderDRR == nil || obs.ProviderDRR.String() != "40" {
		t.Fatalf("provider drr=%v", obs.ProviderDRR)
	}
	if dailyHash(obs) == dailyHash(DailyObservation{ProviderCampaignID: 7, BusinessDate: obs.BusinessDate, PaidSpendAmount: obs.PaidSpendAmount, CurrencyCode: obs.CurrencyCode, TrafficState: obs.TrafficState, AttributionState: obs.AttributionState, AttributionKind: obs.AttributionKind}) {
		t.Fatal("bonus/provider drr must participate in revision hash")
	}
}

func TestABACampaignConfigCreatesDistinctHashes(t *testing.T) {
	a := CampaignObservation{Title: "A", ProviderState: "ACTIVE", CurrencyCode: "RUB", TargetScopeType: "EXPLICIT_SKU_SET"}
	b := a
	b.Title = "B"
	c := a
	if campaignHash(a) == campaignHash(b) || campaignHash(a) != campaignHash(c) {
		t.Fatal("A→B→A hashes broken")
	}
}

func TestParseCPOPreservesPromotedVersusOrdered(t *testing.T) {
	body, _ := json.Marshal(map[string]any{"rows": []map[string]any{{
		"campaignId": 11, "date": "2026-08-24", "sku": "101", "orderedSku": "202", "orderId": "501", "quantity": 2, "sales": "80",
	}}})
	rows, err := parseCPORows(body)
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows=%v err=%v", rows, err)
	}
	if rows[0].PromotedSKU != 101 || rows[0].OrderedSKU != 202 || rows[0].OrderID != 501 {
		t.Fatalf("promoted/ordered/order not preserved: %+v", rows[0])
	}
}

func TestParseCPOExactNumberPreservesMoneyAndIDs(t *testing.T) {
	body := []byte(`{"rows":[{"campaignId":123456789012345,"date":"2026-08-24","sku":101,"orderedSku":202,"orderId":501,"quantity":2,"sales":123456789012345.123456}]}`)
	rows, err := parseCPORows(body)
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows=%v err=%v", rows, err)
	}
	if rows[0].CampaignID != 123456789012345 {
		t.Fatalf("campaign id lost precision: %d", rows[0].CampaignID)
	}
	if rows[0].Amount != "123456789012345.123456" {
		t.Fatalf("amount not exact: %q", rows[0].Amount)
	}
}

func TestCampaignSemanticsUnknownScopeUntilTargetEnumeration(t *testing.T) {
	family, scope := campaignSemantics(performance.Campaign{CampaignType: "SKU", PaymentType: "CPC"})
	if family != "PRODUCT_CPC" || scope != "UNKNOWN" {
		t.Fatalf("expected PRODUCT_CPC/UNKNOWN, got %s/%s", family, scope)
	}
	family, scope = campaignSemantics(performance.Campaign{CampaignType: "SEARCH_PROMO", PaymentType: "CPO"})
	if family != "SEARCH_PROMO_CPO" || scope != "UNKNOWN" {
		t.Fatalf("expected SEARCH_PROMO_CPO/UNKNOWN, got %s/%s", family, scope)
	}
}
