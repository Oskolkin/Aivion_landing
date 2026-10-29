package devseed

import (
	"math"
	"testing"
)

func TestAdvertisingReuseDecision(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                 string
		reuseExisting        bool
		hasExistingAdMetrics bool
		wantSeedSynthetic    bool
		wantErr              bool
	}{
		{name: "no_reuse_seeds_synthetic_regardless", reuseExisting: false, hasExistingAdMetrics: false, wantSeedSynthetic: true, wantErr: false},
		{name: "no_reuse_seeds_synthetic_even_with_existing", reuseExisting: false, hasExistingAdMetrics: true, wantSeedSynthetic: true, wantErr: false},
		{name: "reuse_with_existing_skips_synthetic", reuseExisting: true, hasExistingAdMetrics: true, wantSeedSynthetic: false, wantErr: false},
		{name: "reuse_without_existing_errors", reuseExisting: true, hasExistingAdMetrics: false, wantSeedSynthetic: false, wantErr: true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			seedSynthetic, err := advertisingReuseDecision(tc.reuseExisting, tc.hasExistingAdMetrics, 1)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if seedSynthetic != tc.wantSeedSynthetic {
				t.Fatalf("seedSynthetic=%v want %v", seedSynthetic, tc.wantSeedSynthetic)
			}
		})
	}
}

func TestAdMetricsProfileForIndexDistribution(t *testing.T) {
	t.Parallel()
	seen := map[adMetricsProfile]bool{}
	for i := 0; i < 8; i++ {
		seen[adMetricsProfileForIndex(i)] = true
	}
	for _, want := range []adMetricsProfile{
		adMetricsSpendWithoutOrders,
		adMetricsHighDRR,
		adMetricsHealthyROAS,
		adMetricsStoppedOrInactive,
	} {
		if !seen[want] {
			t.Fatalf("missing profile %s", want)
		}
	}
}

func TestDailyAdMetricsForProfileScenarios(t *testing.T) {
	t.Parallel()
	cases := []struct {
		profile adMetricsProfile
		check   func(impressions, clicks int64, spend, revenue float64, orders int32)
	}{
		{
			profile: adMetricsSpendWithoutOrders,
			check: func(impressions, clicks int64, spend, revenue float64, orders int32) {
				if impressions <= 0 || clicks <= 0 || spend <= 0 || orders != 0 || revenue != 0 {
					t.Fatalf("spend_without_orders: imp=%d clicks=%d spend=%v revenue=%v orders=%d", impressions, clicks, spend, revenue, orders)
				}
			},
		},
		{
			profile: adMetricsHighDRR,
			check: func(impressions, clicks int64, spend, revenue float64, orders int32) {
				if impressions <= 0 || clicks <= 0 || spend <= 0 || orders <= 0 || revenue <= 0 {
					t.Fatalf("high_drr: imp=%d clicks=%d spend=%v revenue=%v orders=%d", impressions, clicks, spend, revenue, orders)
				}
				if spend/revenue*100 <= 100 {
					t.Fatalf("high_drr: DRR=%.2f%%", spend/revenue*100)
				}
			},
		},
		{
			profile: adMetricsHealthyROAS,
			check: func(impressions, clicks int64, spend, revenue float64, orders int32) {
				if spend <= 0 || orders <= 0 || revenue <= spend*3 {
					t.Fatalf("healthy_roas: spend=%v revenue=%v orders=%d", spend, revenue, orders)
				}
			},
		},
		{
			profile: adMetricsStoppedOrInactive,
			check: func(impressions, clicks int64, spend, revenue float64, orders int32) {
				if spend != 0 || revenue != 0 || orders != 0 {
					t.Fatalf("stopped_or_inactive: spend=%v revenue=%v orders=%d", spend, revenue, orders)
				}
			},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(string(tc.profile), func(t *testing.T) {
			t.Parallel()
			impressions, clicks, spend, revenue, orders := dailyAdMetricsForProfile(tc.profile)
			spendN := numericMoneyAmount(spend)
			revenueN := numericMoneyAmount(revenue)
			if !spendN.Valid || !revenueN.Valid {
				t.Fatal("spend/revenue must be valid numerics")
			}
			tc.check(impressions, clicks, spend, revenue, orders)
		})
	}
}

func TestRichAdMetricsProfilesAcrossCampaigns(t *testing.T) {
	t.Parallel()
	days := enrichmentStatsLookbackDays + 1
	var (
		hasSpendNoOrders bool
		hasHighDRR       bool
		hasHealthyROAS   bool
		hasZeroActivity  bool
	)
	for idx := 0; idx < 10; idx++ {
		profile := adMetricsProfileForIndex(idx)
		var spend14, revenue14 float64
		var orders14 int64
		for dayOffset := 0; dayOffset < days; dayOffset++ {
			_, _, dailySpend, dailyRevenue, dailyOrders := dailyAdMetricsForProfileOnDay(profile, dayOffset)
			spend14 += dailySpend
			revenue14 += dailyRevenue
			orders14 += int64(dailyOrders)
		}
		switch {
		case spend14 > 0 && orders14 == 0 && revenue14 == 0:
			hasSpendNoOrders = true
		case spend14 > 0 && revenue14 > 0 && spend14/revenue14*100 > 100:
			hasHighDRR = true
		case spend14 > 0 && revenue14 > spend14*3:
			hasHealthyROAS = true
		case spend14 == 0 && orders14 == 0 && revenue14 == 0:
			hasZeroActivity = true
		}
	}
	if !hasSpendNoOrders || !hasHighDRR || !hasHealthyROAS || !hasZeroActivity {
		t.Fatalf("missing scenarios: spend_no_orders=%v high_drr=%v healthy=%v zero=%v",
			hasSpendNoOrders, hasHighDRR, hasHealthyROAS, hasZeroActivity)
	}
}

func TestDayAdMetricScalesProduceVariedAttributedDRR(t *testing.T) {
	t.Parallel()
	const campaigns = 10
	drrByDay := make([]float64, 0, 7)
	var spend7d, revenue7d float64
	for daysBefore := 6; daysBefore >= 0; daysBefore-- {
		spend, revenue, drr := accountAttributedDRRForDay(campaigns, daysBefore)
		if revenue <= 0 {
			t.Fatalf("day offset %d: zero attributed revenue", daysBefore)
		}
		spend7d += spend
		revenue7d += revenue
		drrByDay = append(drrByDay, drr)
	}
	unique := map[float64]struct{}{}
	for _, v := range drrByDay {
		unique[v] = struct{}{}
	}
	if len(unique) < 5 {
		t.Fatalf("want >=5 distinct daily DRR values, got %d: %v", len(unique), drrByDay)
	}
	minV, maxV := drrByDay[0], drrByDay[0]
	for _, v := range drrByDay[1:] {
		if v < minV {
			minV = v
		}
		if v > maxV {
			maxV = v
		}
	}
	if maxV-minV < 8 {
		t.Fatalf("DRR spread too small: min=%.2f max=%.2f spread=%.2f", minV, maxV, maxV-minV)
	}
	want7d := math.Round((spend7d/revenue7d*100)*100) / 100
	meanOfDaily := 0.0
	for _, v := range drrByDay {
		meanOfDaily += v
	}
	meanOfDaily /= float64(len(drrByDay))
	if math.Abs(want7d-meanOfDaily) < 0.01 {
		// Not a hard failure if coincident, but document ratio-of-sums semantics.
		t.Logf("7d ratio-of-sums=%.2f mean-of-daily=%.2f (may coincide)", want7d, meanOfDaily)
	}
	// Metric day (offset 0) must match unscaled base account totals 23400/25500.
	spend0, rev0, drr0 := accountAttributedDRRForDay(campaigns, 0)
	if math.Abs(spend0-23400) > 0.01 || math.Abs(rev0-25500) > 0.01 {
		t.Fatalf("metric-day totals spend=%.2f revenue=%.2f want 23400/25500", spend0, rev0)
	}
	if math.Abs(drr0-91.76) > 0.05 {
		t.Fatalf("metric-day DRR=%.2f want ~91.76", drr0)
	}
}

func TestSpendWithoutOrdersStaysZeroRevenueOnAllDays(t *testing.T) {
	t.Parallel()
	for day := 0; day < 7; day++ {
		_, _, spend, revenue, orders := dailyAdMetricsForProfileOnDay(adMetricsSpendWithoutOrders, day)
		if spend <= 0 || revenue != 0 || orders != 0 {
			t.Fatalf("day %d: spend=%v revenue=%v orders=%d", day, spend, revenue, orders)
		}
	}
}

func TestAccountTotalsEqualSumOfCampaignDailyRows(t *testing.T) {
	t.Parallel()
	const campaigns = 10
	for daysBefore := 0; daysBefore < 7; daysBefore++ {
		var spendSum, revenueSum float64
		for idx := 0; idx < campaigns; idx++ {
			profile := adMetricsProfileForIndex(idx)
			_, _, spend, revenue, _ := dailyAdMetricsForProfileOnDay(profile, daysBefore)
			spendSum += spend
			revenueSum += revenue
		}
		accountSpend, accountRevenue, _ := accountAttributedDRRForDay(campaigns, daysBefore)
		if math.Abs(spendSum-accountSpend) > 0.01 || math.Abs(revenueSum-accountRevenue) > 0.01 {
			t.Fatalf("day offset %d: campaign sum spend=%.2f rev=%.2f account spend=%.2f rev=%.2f",
				daysBefore, spendSum, revenueSum, accountSpend, accountRevenue)
		}
	}
}

func TestSevenDayTotalsAreSumOfDailyTotals(t *testing.T) {
	t.Parallel()
	const campaigns = 10
	var spend7d, revenue7d float64
	for daysBefore := 0; daysBefore < 7; daysBefore++ {
		s, r, _ := accountAttributedDRRForDay(campaigns, daysBefore)
		spend7d += s
		revenue7d += r
	}
	wantDRR := math.Round((spend7d/revenue7d*100)*100) / 100
	if math.Abs(spend7d-170352) > 1 || math.Abs(revenue7d-183090) > 1 {
		t.Fatalf("7d totals spend=%.0f rev=%.0f want 170352/183090", spend7d, revenue7d)
	}
	if math.Abs(wantDRR-93.04) > 0.05 {
		t.Fatalf("7d ratio-of-sums DRR=%.2f want ~93.04", wantDRR)
	}
}
