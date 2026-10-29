package analytics

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

func mustNumeric(t *testing.T, v float64) pgtype.Numeric {
	t.Helper()
	var n pgtype.Numeric
	if err := n.Scan(fmt.Sprintf("%.2f", v)); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestCampaignDRRPct_AndDelta(t *testing.T) {
	spend := 400.0
	revenue := 1000.0
	got := CampaignDRRPct(spend, revenue)
	if got == nil || math.Abs(*got-40) > 0.01 {
		t.Fatalf("expected 40%% DRR, got %v", got)
	}
	prev := 31.6
	delta := PercentagePointsDelta(got, &prev)
	if delta == nil || math.Abs(*delta-8.4) > 0.01 {
		t.Fatalf("expected +8.4 pp, got %v", delta)
	}
}

func TestCampaignDRRPct_ZeroRevenueNil(t *testing.T) {
	if got := CampaignDRRPct(500, 0); got != nil {
		t.Fatalf("expected nil DRR at zero revenue, got %v", *got)
	}
}

func TestComputeAdEfficiencyDashboard_CurrentAttributedShare(t *testing.T) {
	metricDate := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	campaigns := []dbgen.ListAdCampaignSummariesBySellerAndDateRangeRow{
		{
			CampaignExternalID: 1,
			CampaignName:       "Alpha",
			SpendTotal:         mustNumeric(t, 200),
			RevenueTotal:       mustNumeric(t, 1000),
			OrdersTotal:        4,
		},
	}
	daily := []dbgen.AdMetricsDaily{
		adDailyRow(metricDate, 1, 200, 1000, 4),
		adDailyRow(metricDate.AddDate(0, 0, -1), 1, 100, 500, 2),
	}
	// Store revenue differs — advertising block must use attributed revenue.
	accountRev := map[string]float64{
		"2026-03-10": 5000,
		"2026-03-09": 4000,
	}
	dto := computeAdEfficiencyDashboard(campaigns, daily, accountRev, metricDate.AddDate(0, 0, 1), nil)
	if dto.Summary.CurrentDRRPct == nil || math.Abs(*dto.Summary.CurrentDRRPct-20) > 0.01 {
		t.Fatalf("current attributed share: got %v want 20", dto.Summary.CurrentDRRPct)
	}
	if math.Abs(dto.Summary.TotalRevenueToday-1000) > 0.01 {
		t.Fatalf("total revenue today must be attributed ad revenue, got %v", dto.Summary.TotalRevenueToday)
	}
	if math.Abs(dto.Summary.TotalStoreRevenueToday-5000) > 0.01 {
		t.Fatalf("store revenue today got %v want 5000", dto.Summary.TotalStoreRevenueToday)
	}
}

func TestComputeAdEfficiencyDashboard_AverageViaAttributedTotals(t *testing.T) {
	metricDate := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	daily := []dbgen.AdMetricsDaily{
		adDailyRow(metricDate, 1, 300, 1000, 3),
		adDailyRow(metricDate.AddDate(0, 0, -1), 1, 100, 500, 1),
	}
	accountRev := map[string]float64{"2026-03-10": 9000, "2026-03-09": 8000}
	dto := computeAdEfficiencyDashboard(nil, daily, accountRev, metricDate.AddDate(0, 0, 1), nil)
	want := 400.0 / 1500.0 * 100
	if dto.Summary.AverageDRR7dPct == nil || math.Abs(*dto.Summary.AverageDRR7dPct-want) > 0.1 {
		t.Fatalf("7d attributed share got %v want %v", dto.Summary.AverageDRR7dPct, want)
	}
}

func TestComputeAdEfficiencyDashboard_ExcludesMetricDate(t *testing.T) {
	metricDate := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)
	daily := []dbgen.AdMetricsDaily{
		adDailyRow(metricDate.AddDate(0, 0, -2), 1, 512, 1000, 1),
		adDailyRow(metricDate.AddDate(0, 0, -1), 1, 75, 200, 1),
		adDailyRow(metricDate, 1, 999, 1, 1),
	}
	dto := computeAdEfficiencyDashboard(nil, daily, nil, metricDate, nil)
	if dto.Period.CurrentCompletedDate != "2026-07-07" || dto.Period.PreviousCompletedDate != "2026-07-06" || dto.Period.Current7dDateFrom != "2026-07-01" || dto.Period.Previous7dDateFrom != "2026-06-24" {
		t.Fatalf("unexpected completed windows: %+v", dto.Period)
	}
	if dto.Summary.CurrentDRRPct == nil || math.Abs(*dto.Summary.CurrentDRRPct-37.5) > .01 || dto.Summary.DeltaVsPreviousPp == nil || math.Abs(*dto.Summary.DeltaVsPreviousPp+13.7) > .01 {
		t.Fatalf("unexpected completed daily DRR: %+v", dto.Summary)
	}
	if last := dto.Trend7D[len(dto.Trend7D)-1].Date; last != "2026-07-07" {
		t.Fatalf("last trend point %s is not D-1", last)
	}
}

func TestComputeAdEfficiencyDashboard_UsesAttributedNotStore(t *testing.T) {
	metricDate := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)
	daily := []dbgen.AdMetricsDaily{
		adDailyRow(metricDate, 1, 23400, 25500, 15),
	}
	accountRev := map[string]float64{"2026-07-08": 120791.25}
	dto := computeAdEfficiencyDashboard(nil, daily, accountRev, metricDate.AddDate(0, 0, 1), nil)
	if dto.Summary.CurrentDRRPct == nil {
		t.Fatal("nil current share")
	}
	if math.Abs(*dto.Summary.CurrentDRRPct-91.76) > 0.05 {
		t.Fatalf("got %v want ~91.76 (attributed), not ~19.4 (store)", *dto.Summary.CurrentDRRPct)
	}
	storeShare := AccountDRRPct(23400, 120791.25)
	if storeShare == nil || math.Abs(*dto.Summary.CurrentDRRPct-*storeShare) < 50 {
		t.Fatalf("ads share must differ from store share: ads=%v store=%v", *dto.Summary.CurrentDRRPct, storeShare)
	}
}

func TestComputeAdEfficiencyDashboard_AllCampaignsInShare(t *testing.T) {
	metricDate := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	daily := []dbgen.AdMetricsDaily{
		adDailyRow(metricDate, 1, 100, 400, 2),
		adDailyRow(metricDate, 2, 100, 600, 1), // no orders on campaign 2 still counted
	}
	accountRev := map[string]float64{"2026-03-10": 99999}
	dto := computeAdEfficiencyDashboard(nil, daily, accountRev, metricDate.AddDate(0, 0, 1), nil)
	if dto.Summary.CurrentDRRPct == nil || math.Abs(*dto.Summary.CurrentDRRPct-20) > 0.01 {
		t.Fatalf("must include all campaigns: got %v want 20", dto.Summary.CurrentDRRPct)
	}
}

func TestComputeAdEfficiencyDashboard_NotArithmeticMeanOfCampaignDRR(t *testing.T) {
	metricDate := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	// Campaign A: 10/100=10%, B: 90/100=90%; mean=50%; totals 100/200=50% wait
	// Better: A 10/200=5%, B 90/1800=5%; totals 100/2000=5% vs mean of 5,5 = 5
	// Use unequal: A 50/100=50%, B 50/900=5.56%; mean≈27.8%; totals 100/1000=10%
	daily := []dbgen.AdMetricsDaily{
		adDailyRow(metricDate, 1, 50, 100, 1),
		adDailyRow(metricDate, 2, 50, 900, 1),
	}
	dto := computeAdEfficiencyDashboard(nil, daily, map[string]float64{"2026-03-10": 5000}, metricDate.AddDate(0, 0, 1), nil)
	if dto.Summary.CurrentDRRPct == nil || math.Abs(*dto.Summary.CurrentDRRPct-10) > 0.01 {
		t.Fatalf("must use totals not mean of campaign DRR: got %v want 10", dto.Summary.CurrentDRRPct)
	}
}

func TestComputeAdEfficiencyDashboard_ZeroAttributedRevenueNil(t *testing.T) {
	metricDate := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	daily := []dbgen.AdMetricsDaily{adDailyRow(metricDate, 1, 100, 0, 0)}
	dto := computeAdEfficiencyDashboard(nil, daily, map[string]float64{"2026-03-10": 1000}, metricDate, nil)
	if dto.Summary.CurrentDRRPct != nil {
		t.Fatalf("zero attributed revenue must yield nil share, got %v", *dto.Summary.CurrentDRRPct)
	}
}

func TestComputeAdEfficiencyDashboard_HighDRRClassification(t *testing.T) {
	metricDate := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	campaigns := []dbgen.ListAdCampaignSummariesBySellerAndDateRangeRow{
		{CampaignExternalID: 1, CampaignName: "Hot", SpendTotal: mustNumeric(t, 200), RevenueTotal: mustNumeric(t, 100), OrdersTotal: 1},
		{CampaignExternalID: 2, CampaignName: "Ok", SpendTotal: mustNumeric(t, 50), RevenueTotal: mustNumeric(t, 200), OrdersTotal: 2},
	}
	daily := []dbgen.AdMetricsDaily{
		adDailyRow(metricDate, 1, 200, 100, 1),
		adDailyRow(metricDate, 2, 50, 200, 2),
	}
	dto := computeAdEfficiencyDashboard(campaigns, daily, map[string]float64{"2026-03-10": 5000}, metricDate, nil)
	if dto.Summary.CampaignsHighDRRCount != 1 {
		t.Fatalf("high DRR count=%d want 1", dto.Summary.CampaignsHighDRRCount)
	}
}

func TestComputeAdEfficiencyDashboard_NoOrderCampaigns(t *testing.T) {
	metricDate := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	campaigns := []dbgen.ListAdCampaignSummariesBySellerAndDateRangeRow{
		{CampaignExternalID: 1, CampaignName: "Waste", SpendTotal: mustNumeric(t, 80), RevenueTotal: mustNumeric(t, 0), OrdersTotal: 0},
	}
	daily := []dbgen.AdMetricsDaily{adDailyRow(metricDate, 1, 80, 0, 0)}
	dto := computeAdEfficiencyDashboard(campaigns, daily, map[string]float64{"2026-03-10": 5000}, metricDate, nil)
	if dto.Summary.CampaignsNoOrdersCount != 1 {
		t.Fatalf("no-order count=%d", dto.Summary.CampaignsNoOrdersCount)
	}
	if math.Abs(dto.Summary.SpendWithoutOrders-80) > 0.01 {
		t.Fatalf("spend without orders=%v", dto.Summary.SpendWithoutOrders)
	}
}

func TestComputeAdEfficiencyDashboard_CountsAllNotOnlyLimited(t *testing.T) {
	metricDate := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	var campaigns []dbgen.ListAdCampaignSummariesBySellerAndDateRangeRow
	var daily []dbgen.AdMetricsDaily
	for i := 1; i <= 8; i++ {
		campaigns = append(campaigns, dbgen.ListAdCampaignSummariesBySellerAndDateRangeRow{
			CampaignExternalID: int64(i),
			CampaignName:       fmt.Sprintf("C%d", i),
			SpendTotal:         mustNumeric(t, 200),
			RevenueTotal:       mustNumeric(t, 100),
			OrdersTotal:        1,
		})
		daily = append(daily, adDailyRow(metricDate, int64(i), 200, 100, 1))
	}
	dto := computeAdEfficiencyDashboard(campaigns, daily, map[string]float64{"2026-03-10": 5000}, metricDate, nil)
	if dto.Summary.CampaignsHighDRRCount != 8 {
		t.Fatalf("count=%d want 8", dto.Summary.CampaignsHighDRRCount)
	}
	if len(dto.HighDRRCampaigns) != AdEfficiencyDashboardCampaignLimit {
		t.Fatalf("display limited to %d, got %d", AdEfficiencyDashboardCampaignLimit, len(dto.HighDRRCampaigns))
	}
}

func TestComputeAdEfficiencyDashboard_HighDRRSpendCountsAllNotOnlyLimited(t *testing.T) {
	metricDate := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	var campaigns []dbgen.ListAdCampaignSummariesBySellerAndDateRangeRow
	var daily []dbgen.AdMetricsDaily
	for i := 1; i <= 8; i++ {
		campaigns = append(campaigns, dbgen.ListAdCampaignSummariesBySellerAndDateRangeRow{
			CampaignExternalID: int64(i),
			CampaignName:       fmt.Sprintf("C%d", i),
			SpendTotal:         mustNumeric(t, 100),
			RevenueTotal:       mustNumeric(t, 50),
			OrdersTotal:        1,
		})
		daily = append(daily, adDailyRow(metricDate, int64(i), 100, 50, 1))
	}
	dto := computeAdEfficiencyDashboard(campaigns, daily, map[string]float64{"2026-03-10": 5000}, metricDate, nil)
	if math.Abs(dto.Summary.HighDRRSpend-800) > 0.01 {
		t.Fatalf("high DRR spend=%v want 800", dto.Summary.HighDRRSpend)
	}
}

func TestComputeAdEfficiencyDashboard_MissingDayNotFakeZero(t *testing.T) {
	metricDate := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	daily := []dbgen.AdMetricsDaily{
		adDailyRow(metricDate, 1, 100, 200, 1),
	}
	dto := computeAdEfficiencyDashboard(nil, daily, map[string]float64{"2026-03-10": 1000}, metricDate, nil)
	missing := 0
	for _, pt := range dto.Trend7D {
		if pt.DataStatus == "missing" {
			missing++
			if pt.DRRPct != nil {
				t.Fatalf("missing day must not invent DRR, got %v", *pt.DRRPct)
			}
		}
	}
	if missing < 5 {
		t.Fatalf("expected several missing days, got %d", missing)
	}
}

func mustNumericValue(v float64) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(fmt.Sprintf("%.2f", v))
	return n
}

func adDailyRow(day time.Time, campaignID int64, spend, revenue float64, orders int32) dbgen.AdMetricsDaily {
	return dbgen.AdMetricsDaily{
		SellerAccountID:    1,
		CampaignExternalID: campaignID,
		MetricDate:         pgtype.Date{Time: day.UTC(), Valid: true},
		Spend:              mustNumericValue(spend),
		Revenue:            mustNumericValue(revenue),
		OrdersCount:        orders,
	}
}
