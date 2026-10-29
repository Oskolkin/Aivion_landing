package landscape

import (
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

func TestClassifySKUCurrentDemandZeroVsUnknown(t *testing.T) {
	zeroAmt := &MoneyAmount{Amount: "0", CurrencyCode: "RUB"}
	zeroUnits := int64(0)
	positive := int64(3)
	cases := []struct {
		name string
		sk   SKULandscape
		want DemandKnowledge
	}{
		{"nil sales", SKULandscape{}, DemandUnknown},
		{"zero units and amount", SKULandscape{Sales: &SKUSalesBlock{OrderedUnits: CountComparison{Current: &zeroUnits}, OrderedAmount: MoneyComparison{Current: zeroAmt}}}, DemandZero},
		{"nonzero amount string is not positive if zero", SKULandscape{Sales: &SKUSalesBlock{OrderedUnits: CountComparison{Current: &zeroUnits}, OrderedAmount: MoneyComparison{Current: &MoneyAmount{Amount: "0", CurrencyCode: "RUB"}}}}, DemandZero},
		{"unknown both nil", SKULandscape{Sales: &SKUSalesBlock{}}, DemandUnknown},
		{"positive units", SKULandscape{Sales: &SKUSalesBlock{OrderedUnits: CountComparison{Current: &positive}}}, DemandPositive},
	}
	for _, tc := range cases {
		if got := classifySKUCurrentDemand(tc.sk); got != tc.want {
			t.Fatalf("%s: got %s want %s", tc.name, got, tc.want)
		}
	}
}

func TestShareOfAccountKnownZero(t *testing.T) {
	share, lim := shareOfAccount(
		&MoneyAmount{Amount: "0", CurrencyCode: "RUB"},
		&MoneyAmount{Amount: "100000", CurrencyCode: "RUB"},
	)
	if lim != nil {
		t.Fatalf("known zero numerator must not be a limitation: %+v", lim)
	}
	if share == nil || *share != canonicaljson.Decimal("0") {
		t.Fatalf("share=%v want 0", share)
	}
}

func TestShareOfAccountZeroDenominator(t *testing.T) {
	share, lim := shareOfAccount(
		&MoneyAmount{Amount: "0", CurrencyCode: "RUB"},
		&MoneyAmount{Amount: "0", CurrencyCode: "RUB"},
	)
	if share != nil {
		t.Fatalf("share=%v want nil", share)
	}
	if lim == nil || lim.Code != "CAMPAIGN_EXPOSURE_NO_DENOMINATOR" {
		t.Fatalf("limitation=%+v", lim)
	}
}

func TestApplySupportingTruncationPreservesPeriodAndAsOf(t *testing.T) {
	asOf := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	period := &DateRange{From: canonicaljson.Date(time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)), To: canonicaljson.Date(time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC))}
	sc := SupportingContext[CampaignContextItem]{
		State: BlockReady, WindowRef: "ADVERTISING_TRAFFIC_CURRENT", Period: period, AsOf: &asOf,
		Items:      []CampaignContextItem{{CampaignID: 1}, {CampaignID: 2}, {CampaignID: 3}},
		TotalCount: 3, ReturnedCount: 3, SortBasis: "PAID_SPEND_DESC", SemanticScope: "TOP_N_BY_PAID_SPEND",
	}
	out := ApplySupportingTruncation(sc, 2, "PAID_SPEND_DESC", "TOP_N_BY_PAID_SPEND")
	if out.WindowRef != "ADVERTISING_TRAFFIC_CURRENT" {
		t.Fatalf("window_ref lost: %s", out.WindowRef)
	}
	if out.Period == nil || out.Period.From != period.From || out.Period.To != period.To {
		t.Fatalf("period lost: %+v", out.Period)
	}
	if out.AsOf == nil || !out.AsOf.Equal(asOf) {
		t.Fatalf("as_of lost: %v", out.AsOf)
	}
}

func TestCampaignStockUnknownNotReadyEmpty(t *testing.T) {
	block := buildCampaignStockBlock(3, []int64{1, 2, 3}, false, nil)
	if block.State != BlockPartial {
		t.Fatalf("state=%s want PARTIAL", block.State)
	}
	if block.AssignedSKUCount == nil || *block.AssignedSKUCount != 3 {
		t.Fatalf("assigned=%v", block.AssignedSKUCount)
	}
	if block.ContextSKUCount == nil || *block.ContextSKUCount != 3 {
		t.Fatalf("context=%v", block.ContextSKUCount)
	}
	if block.ContextSKUWithKnownStockCount == nil || *block.ContextSKUWithKnownStockCount != 0 {
		t.Fatalf("known=%v", block.ContextSKUWithKnownStockCount)
	}
	found := false
	for _, lim := range block.Limitations {
		if lim.Code == "CAMPAIGN_STOCK_NO_KNOWN_SKU_STOCK" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing CAMPAIGN_STOCK_NO_KNOWN_SKU_STOCK: %+v", block.Limitations)
	}
}

func TestMoneyAccumulatorPreservesKnownZero(t *testing.T) {
	var a MoneyAccumulator
	a.Add(&MoneyAmount{Amount: "0", CurrencyCode: "RUB"})
	got := a.Result()
	if got == nil || got.Amount != "0" {
		t.Fatalf("known zero became missing: %+v", got)
	}
}

func TestCampaignStockMixedKnownUnknown(t *testing.T) {
	zero := int64(0)
	ten := int64(10)
	stocks := map[int64]factualapi.SKUStock{
		1: {SellableUnits: factualapi.Known(zero)},
		2: {SellableUnits: factualapi.Known(ten)},
		3: {SellableUnits: factualapi.Missing[int64](factualapi.ValueUnknown, "STOCK_UNKNOWN")},
	}
	block := buildCampaignStockBlock(3, []int64{1, 2, 3}, false, stocks)
	if block.State != BlockPartial {
		t.Fatalf("state=%s want PARTIAL", block.State)
	}
	if block.AssignedSKUCount == nil || *block.AssignedSKUCount != 3 {
		t.Fatalf("assigned=%v", block.AssignedSKUCount)
	}
	if block.ContextSKUWithKnownStockCount == nil || *block.ContextSKUWithKnownStockCount != 2 {
		t.Fatalf("known=%v", block.ContextSKUWithKnownStockCount)
	}
	if block.ContextSKUWithZeroStockCount == nil || *block.ContextSKUWithZeroStockCount != 1 {
		t.Fatalf("zero=%v", block.ContextSKUWithZeroStockCount)
	}
}

func TestPrompt96CalculatedDRRPeriodMismatch(t *testing.T) {
	trafficCur := map[int64]factualapi.CampaignTrafficPerformance{
		1: {
			CampaignID: 1,
			Period:     factualapi.Period{From: time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC)},
			PaidSpend:  factualapi.Known(factualapi.Money{Amount: mustDec("100"), CurrencyCode: "RUB"}),
		},
	}
	attrCur := map[int64]factualapi.CampaignAttributionPerformance{
		1: {
			CampaignID:            1,
			Period:                factualapi.Period{From: time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC)},
			AttributedOrderAmount: factualapi.Known(factualapi.Money{Amount: mustDec("200"), CurrencyCode: "RUB"}),
		},
	}
	block := buildCampaignPerfBlockV4(1, trafficCur, nil, nil, nil, true, BlockExecutionDecision{Execute: true}, attrCur, nil, nil, nil, true, BlockExecutionDecision{Execute: true})
	if block == nil || block.Attribution == nil {
		t.Fatal("expected split traffic/attribution block")
	}
	if block.Attribution.CalculatedDRR != nil {
		t.Fatalf("CalculatedDRR must be nil on period mismatch, got %+v", block.Attribution.CalculatedDRR)
	}
	found := false
	for _, lim := range block.Attribution.Limitations {
		if lim.Code == "DRR_PERIOD_MISMATCH" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing DRR_PERIOD_MISMATCH: %+v", block.Attribution.Limitations)
	}
}

func TestRollupCategoryAdsV4AccumulatesPreviousSpend(t *testing.T) {
	catID := int64(11)
	skus := []SKULandscape{{
		Identity: SKUIdentity{CanonicalSKUID: 1, CategoryID: &catID},
		Advertising: &SKUAdsBlock{
			Traffic: &SKUAdsTrafficBlock{
				State: BlockReady,
				PaidSpend: MoneyComparison{
					Current:  &MoneyAmount{Amount: "20", CurrencyCode: "RUB"},
					Previous: &MoneyAmount{Amount: "80", CurrencyCode: "RUB"},
				},
			},
		},
	}}
	got := rollupCategoryAdsV4(catID, skus)
	if got == nil || got.Traffic == nil {
		t.Fatal("expected category traffic rollup")
	}
	if got.Traffic.PaidSpend.Current == nil || got.Traffic.PaidSpend.Current.Amount != "20" {
		t.Fatalf("current=%v", got.Traffic.PaidSpend.Current)
	}
	if got.Traffic.PaidSpend.Previous == nil || got.Traffic.PaidSpend.Previous.Amount != "80" {
		t.Fatalf("previous spend dropped (short-circuit): %v", got.Traffic.PaidSpend.Previous)
	}
}

func TestRollupCategoryAdsV4OmitsWhenNoAds(t *testing.T) {
	catID := int64(11)
	got := rollupCategoryAdsV4(catID, []SKULandscape{{Identity: SKUIdentity{CanonicalSKUID: 1, CategoryID: &catID}}})
	if got != nil {
		t.Fatalf("expected nil advertising when no SKU ads, got state=%s", got.State)
	}
}

func mustDec(s string) factualapi.Decimal {
	d, err := factualapi.ParseDecimal(s)
	if err != nil {
		panic(err)
	}
	return d
}
