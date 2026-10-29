package analysisdata

import (
	"context"
	"testing"
	"time"
)

type fakeCoverageSrc struct {
	dates        map[string][]string
	hasCampaigns bool
}

func (f *fakeCoverageSrc) DatasetDates(_ context.Context, _ int64, dataset string, _, _ time.Time) ([]string, error) {
	return append([]string(nil), f.dates[dataset]...), nil
}

func (f *fakeCoverageSrc) HasCampaigns(_ context.Context, _ int64) (bool, error) {
	return f.hasCampaigns, nil
}

func completeFixture(dates ...string) map[string][]string {
	m := map[string][]string{}
	for _, ds := range []string{
		DatasetOrdersSales,
		DatasetDailySKUMetrics,
		DatasetProductAnalytics,
		DatasetFunnelAccount,
		DatasetStocks,
		DatasetProductEnrichment,
		DatasetAdvertising,
		DatasetCampaignMetrics,
	} {
		m[ds] = append([]string(nil), dates...)
	}
	return m
}

func TestResolveAnalysisMetricDate_CurrentComplete(t *testing.T) {
	src := &fakeCoverageSrc{dates: completeFixture("2026-07-13"), hasCampaigns: true}
	now := time.Date(2026, 7, 13, 15, 0, 0, 0, time.UTC)
	res, err := ResolveAnalysisMetricDate(context.Background(), src, ResolveAnalysisMetricDateInput{
		SellerAccountID: 1,
		DateMode:        DateModeCurrent,
		Timezone:        "UTC",
		Now:             now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Allowed || res.EffectiveMetricDate != "2026-07-13" || res.ResolutionReason != ReasonCurrentDateComplete {
		t.Fatalf("unexpected resolution: %+v", res)
	}
	if res.FreshnessLagDays != 0 || res.IsStale {
		t.Fatalf("expected fresh: %+v", res)
	}
	if !res.PublishAsCurrentDefault {
		t.Fatal("current mode should default publish_as_current=true")
	}
}

func TestResolveAnalysisMetricDate_CurrentLagOneDay(t *testing.T) {
	src := &fakeCoverageSrc{dates: completeFixture("2026-07-12"), hasCampaigns: true}
	now := time.Date(2026, 7, 13, 15, 0, 0, 0, time.UTC)
	res, err := ResolveAnalysisMetricDate(context.Background(), src, ResolveAnalysisMetricDateInput{
		SellerAccountID: 1,
		DateMode:        DateModeCurrent,
		Timezone:        "UTC",
		Now:             now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.EffectiveMetricDate != "2026-07-12" || res.FreshnessLagDays != 1 {
		t.Fatalf("unexpected: %+v", res)
	}
	if res.ResolutionReason != ReasonCurrentDateAdjustedToLatestComplete {
		t.Fatalf("reason=%s", res.ResolutionReason)
	}
	if res.IsStale {
		t.Fatal("lag=1 should not be stale with default threshold 3")
	}
}

func TestResolveAnalysisMetricDate_CurrentStaleTestData(t *testing.T) {
	src := &fakeCoverageSrc{dates: completeFixture("2026-07-08"), hasCampaigns: true}
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	res, err := ResolveAnalysisMetricDate(context.Background(), src, ResolveAnalysisMetricDateInput{
		SellerAccountID: 1,
		DateMode:        DateModeCurrent,
		Timezone:        "UTC",
		Now:             now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.EffectiveMetricDate != "2026-07-08" || res.FreshnessLagDays != 9 || !res.IsStale {
		t.Fatalf("unexpected stale resolution: %+v", res)
	}
	if res.RequestedDate != "2026-07-17" {
		t.Fatalf("requested=%s", res.RequestedDate)
	}
	// Current-mode staleness is today − latest_complete only (not publication age).
	if res.SourceFreshnessLagDays != 9 || res.AnalysisAgeDays != 9 {
		t.Fatalf("current mode must keep source lag == analysis age == latest-complete lag: %+v", res)
	}
	if res.LatestCompleteDataDate != "2026-07-08" || res.HistoricalAnalysis {
		t.Fatalf("expected current+stale from latest_complete: %+v", res)
	}
}

// TestResolveAnalysisMetricDate_CurrentStaleFromLatestCompleteNotPublicationAge
// locks the product rule: stale depends on today − latest_complete_data_date,
// never on how old a previously published analysis is.
func TestResolveAnalysisMetricDate_CurrentStaleFromLatestCompleteNotPublicationAge(t *testing.T) {
	src := &fakeCoverageSrc{dates: completeFixture("2026-07-10"), hasCampaigns: true}
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	res, err := ResolveAnalysisMetricDate(context.Background(), src, ResolveAnalysisMetricDateInput{
		SellerAccountID: 1,
		DateMode:        DateModeCurrent,
		Timezone:        "UTC",
		Now:             now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.EffectiveMetricDate != "2026-07-10" || res.LatestCompleteDataDate != "2026-07-10" {
		t.Fatalf("unexpected dates: %+v", res)
	}
	if res.SourceFreshnessLagDays != 7 || res.FreshnessLagDays != 7 || !res.IsStale {
		t.Fatalf("stale must come from latest_complete lag only: %+v", res)
	}
	if res.AnalysisAgeDays != res.SourceFreshnessLagDays {
		t.Fatalf("current mode analysis age must equal source lag: %+v", res)
	}
}

func TestResolveAnalysisMetricDate_FixedAccepted(t *testing.T) {
	src := &fakeCoverageSrc{dates: completeFixture("2026-07-08"), hasCampaigns: true}
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	res, err := ResolveAnalysisMetricDate(context.Background(), src, ResolveAnalysisMetricDateInput{
		SellerAccountID:     1,
		DateMode:            DateModeFixed,
		RequestedMetricDate: "2026-07-08",
		Timezone:            "UTC",
		Now:                 now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Allowed || res.ResolutionReason != ReasonFixedDateAccepted {
		t.Fatalf("unexpected: %+v", res)
	}
	if res.PublishAsCurrentDefault {
		t.Fatal("fixed mode should default publish_as_current=false")
	}
	if !res.HistoricalAnalysis || res.AnalysisAgeDays != 9 {
		t.Fatalf("fixed must be historical with analysis age from effective date: %+v", res)
	}
	// When latest_complete == effective fixed date, source lag equals analysis age.
	if res.LatestCompleteDataDate != "2026-07-08" || res.SourceFreshnessLagDays != 9 || !res.IsStale {
		t.Fatalf("source lag from latest_complete: %+v", res)
	}
}

// TestResolveAnalysisMetricDate_FixedOldDateWithFreshLatestComplete verifies
// that an intentionally old fixed analysis date does not imply stale sources
// when latest_complete_data_date is current.
func TestResolveAnalysisMetricDate_FixedOldDateWithFreshLatestComplete(t *testing.T) {
	src := &fakeCoverageSrc{dates: completeFixture("2026-07-08", "2026-07-17"), hasCampaigns: true}
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	res, err := ResolveAnalysisMetricDate(context.Background(), src, ResolveAnalysisMetricDateInput{
		SellerAccountID:     1,
		DateMode:            DateModeFixed,
		RequestedMetricDate: "2026-07-08",
		Timezone:            "UTC",
		Now:                 now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Allowed || !res.HistoricalAnalysis {
		t.Fatalf("expected allowed historical: %+v", res)
	}
	if res.EffectiveMetricDate != "2026-07-08" || res.LatestCompleteDataDate != "2026-07-17" {
		t.Fatalf("dates: %+v", res)
	}
	if res.AnalysisAgeDays != 9 {
		t.Fatalf("analysis_age_days=%d want 9", res.AnalysisAgeDays)
	}
	if res.SourceFreshnessLagDays != 0 || res.FreshnessLagDays != 0 || res.IsStale {
		t.Fatalf("fresh latest_complete must not mark sources stale: %+v", res)
	}
}

func TestResolveAnalysisMetricDate_FixedIncomplete(t *testing.T) {
	src := &fakeCoverageSrc{dates: completeFixture("2026-07-08"), hasCampaigns: true}
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	res, err := ResolveAnalysisMetricDate(context.Background(), src, ResolveAnalysisMetricDateInput{
		SellerAccountID:     1,
		DateMode:            DateModeFixed,
		RequestedMetricDate: "2026-07-13",
		Timezone:            "UTC",
		Now:                 now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Allowed || res.ErrorCode != ErrCodeRequestedFixedDateIncomplete {
		t.Fatalf("expected reject: %+v", res)
	}
	if res.ResolutionReason != ReasonFixedDateIncomplete {
		t.Fatalf("reason=%s", res.ResolutionReason)
	}
	if len(res.MissingRequiredDatasets) == 0 {
		t.Fatal("expected missing datasets")
	}
}

func TestResolveAnalysisMetricDate_DoesNotHardcodeJuly08(t *testing.T) {
	src := &fakeCoverageSrc{dates: completeFixture("2026-07-16"), hasCampaigns: false}
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	res, err := ResolveAnalysisMetricDate(context.Background(), src, ResolveAnalysisMetricDateInput{
		SellerAccountID: 1,
		DateMode:        DateModeCurrent,
		Timezone:        "UTC",
		Now:             now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.EffectiveMetricDate != "2026-07-16" {
		t.Fatalf("product current mode must pick latest complete, got %s", res.EffectiveMetricDate)
	}
}
