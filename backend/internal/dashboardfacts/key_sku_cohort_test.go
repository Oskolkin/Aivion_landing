package dashboardfacts

import "testing"

func TestBuildKeySKUCohortFacts_Ordering(t *testing.T) {
	skuLow := int64(10)
	skuHigh := int64(20)
	idx := SKUFactsIndex{ByOzonProductID: map[string]SKUFactRecord{
		"1": {OzonProductID: 1, SKU: &skuHigh, Revenue7d: 90, Orders7d: 1},
		"2": {OzonProductID: 2, SKU: &skuLow, Revenue7d: 90, Orders7d: 9},
		"3": {OzonProductID: 3, Revenue7d: 70, Orders7d: 2},
	}}
	// Store total 250; first two = 72% so third is required to cross 70%.
	cohort := buildKeySKUCohortFacts(idx)
	if cohort.Status != "available" {
		t.Fatalf("status=%q", cohort.Status)
	}
	if len(cohort.Items) != 2 {
		t.Fatalf("expected 2 items (72%% crosses ≥70%%), got %d: %+v", len(cohort.Items), cohort.Items)
	}
	// Equal revenue 90: SKU ASC (10 before 20).
	if cohort.Items[0].OzonProductID != 2 || cohort.Items[1].OzonProductID != 1 {
		t.Fatalf("expected order [2,1], got %+v", cohort.Items)
	}
}

func TestBuildKeySKUCohortFacts_ThresholdStopsAt70(t *testing.T) {
	idx := SKUFactsIndex{ByOzonProductID: map[string]SKUFactRecord{
		"1": {OzonProductID: 1, Revenue7d: 800, Orders7d: 8},
		"2": {OzonProductID: 2, Revenue7d: 150, Orders7d: 2},
		"3": {OzonProductID: 3, Revenue7d: 50, Orders7d: 1},
	}}
	cohort := buildKeySKUCohortFacts(idx)
	if len(cohort.Items) != 1 {
		t.Fatalf("expected single SKU covering >=70%%, got %d: %+v", len(cohort.Items), cohort.Items)
	}
	if cohort.Items[0].OzonProductID != 1 {
		t.Fatalf("expected top SKU, got %d", cohort.Items[0].OzonProductID)
	}
	if cohort.Items[0].CumulativeRevenueSharePct < KeySKUCohortThresholdPct {
		t.Fatalf("cumulative=%.1f want >= %.0f", cohort.Items[0].CumulativeRevenueSharePct, KeySKUCohortThresholdPct)
	}
	if cohort.ThresholdPct != KeySKUCohortThresholdPct || cohort.PolicyVersion != KeySKUCohortPolicyVersion {
		t.Fatalf("policy mismatch: %+v", cohort)
	}
}

func TestBuildKeySKUCohortFacts_ZeroStoreNotEnoughData(t *testing.T) {
	idx := SKUFactsIndex{ByOzonProductID: map[string]SKUFactRecord{
		"1": {OzonProductID: 1, Revenue7d: 0, Orders7d: 5},
		"2": {OzonProductID: 2, Revenue7d: -1, Orders7d: 1},
	}}
	cohort := buildKeySKUCohortFacts(idx)
	if cohort.Status != "not_enough_data" {
		t.Fatalf("status=%q want not_enough_data", cohort.Status)
	}
	if len(cohort.Items) != 0 {
		t.Fatalf("expected empty items, got %+v", cohort.Items)
	}
	if cohort.StoreTotalRevenue7d != 0 {
		t.Fatalf("store total=%v want 0", cohort.StoreTotalRevenue7d)
	}
}

func TestBuildKeySKUCohortFacts_SharesSumAndMonotonicCumulative(t *testing.T) {
	idx := SKUFactsIndex{ByOzonProductID: map[string]SKUFactRecord{
		"1": {OzonProductID: 1, Revenue7d: 400, Orders7d: 4},
		"2": {OzonProductID: 2, Revenue7d: 300, Orders7d: 3},
		"3": {OzonProductID: 3, Revenue7d: 200, Orders7d: 2},
		"4": {OzonProductID: 4, Revenue7d: 100, Orders7d: 1},
	}}
	cohort := buildKeySKUCohortFacts(idx)
	if cohort.Status != "available" {
		t.Fatalf("status=%q", cohort.Status)
	}
	if cohort.StoreTotalRevenue7d != 1000 {
		t.Fatalf("store total=%v want 1000", cohort.StoreTotalRevenue7d)
	}
	var shareSum float64
	var prevCum float64
	for i, item := range cohort.Items {
		shareSum += item.RevenueSharePct
		if item.CumulativeRevenueSharePct+1e-9 < prevCum {
			t.Fatalf("cumulative not monotonic at %d: prev=%.1f cur=%.1f", i, prevCum, item.CumulativeRevenueSharePct)
		}
		prevCum = item.CumulativeRevenueSharePct
		if item.Rank != i+1 {
			t.Fatalf("rank=%d want %d", item.Rank, i+1)
		}
	}
	if shareSum < KeySKUCohortThresholdPct {
		t.Fatalf("selected shares sum=%.1f want >= %.0f", shareSum, KeySKUCohortThresholdPct)
	}
	last := cohort.Items[len(cohort.Items)-1]
	if last.CumulativeRevenueSharePct < KeySKUCohortThresholdPct {
		t.Fatalf("final cumulative=%.1f want >= %.0f", last.CumulativeRevenueSharePct, KeySKUCohortThresholdPct)
	}
	if cohort.CumulativeRevenueSharePct != last.CumulativeRevenueSharePct {
		t.Fatalf("cohort cumulative=%.1f item=%.1f", cohort.CumulativeRevenueSharePct, last.CumulativeRevenueSharePct)
	}
}

func TestBuildKeySKUCohortFacts_ExcludesZeroRevenue(t *testing.T) {
	idx := SKUFactsIndex{ByOzonProductID: map[string]SKUFactRecord{
		"1": {OzonProductID: 1, Revenue7d: 0, Orders7d: 10},
		"2": {OzonProductID: 2, Revenue7d: 1000, Orders7d: 3},
	}}
	cohort := buildKeySKUCohortFacts(idx)
	if len(cohort.Items) != 1 || cohort.Items[0].OzonProductID != 2 {
		t.Fatalf("unexpected selection: %+v", cohort.Items)
	}
}

func TestSumSparklineRevenueOrders7d(t *testing.T) {
	rev := 100.0
	ord := 2.0
	spark := []SparklinePoint{
		{HasData: false},
		{HasData: true, Revenue: &rev, Value: &ord},
		{HasData: true, Revenue: f64ptr(50), Value: f64ptr(1)},
	}
	gotRev, gotOrd := sumSparklineRevenueOrders7d(spark)
	if gotRev != 150 || gotOrd != 3 {
		t.Fatalf("got rev=%v ord=%d want 150/3", gotRev, gotOrd)
	}
}

func f64ptr(v float64) *float64 { return &v }
