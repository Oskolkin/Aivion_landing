package analytics

import (
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

// BuildAdEfficiencyDashboardFromSnapshot computes ad efficiency purely from snapshot rows.
func BuildAdEfficiencyDashboardFromSnapshot(snap *analysisdata.AnalysisDataSnapshot) (AdEfficiencyDashboardDTO, error) {
	if snap == nil {
		return AdEfficiencyDashboardDTO{}, fmt.Errorf("snapshot is nil")
	}
	if !snap.MetricDate.Valid {
		return AdEfficiencyDashboardDTO{}, fmt.Errorf("snapshot metric_date is invalid")
	}

	metricDate := normalizeDate(snap.MetricDate.Time)
	currentTo := metricDate.AddDate(0, 0, -1)
	currentFrom := currentTo.AddDate(0, 0, -(AdEfficiencyDashboardTrendWindowDays - 1))
	previousFrom := currentFrom.AddDate(0, 0, -AdEfficiencyDashboardTrendWindowDays)
	dailyRows := filterAdMetricsInRange(snap.AdMetrics, previousFrom, currentTo)
	campaignRows := aggregateAdCampaignSummariesFromSnapshot(snap.Campaigns, filterAdMetricsInRange(dailyRows, currentFrom, currentTo), currentFrom, currentTo)
	accountRevenueByDate := accountRevenueByDateFromRows(filterAccountMetricsInRange(snap.AccountMetrics, previousFrom, currentTo))

	var lastUpdated pgtype.Timestamptz
	for _, row := range dailyRows {
		lastUpdated = maxTimestamptz(lastUpdated, row.UpdatedAt)
	}
	return computeAdEfficiencyDashboard(campaignRows, dailyRows, accountRevenueByDate, metricDate, timestamptzToRFC3339(lastUpdated)), nil
}

func filterAdMetricsInRange(rows []dbgen.AdMetricsDaily, from, to time.Time) []dbgen.AdMetricsDaily {
	fromDate := normalizeDate(from)
	toDate := normalizeDate(to)
	out := make([]dbgen.AdMetricsDaily, 0, len(rows))
	for _, row := range rows {
		if !row.MetricDate.Valid {
			continue
		}
		d := normalizeDate(row.MetricDate.Time)
		if d.Before(fromDate) || d.After(toDate) {
			continue
		}
		out = append(out, row)
	}
	return out
}

func aggregateAdCampaignSummariesFromSnapshot(
	campaigns []dbgen.AdCampaign,
	dailyRows []dbgen.AdMetricsDaily,
	dateFrom, dateTo time.Time,
) []dbgen.ListAdCampaignSummariesBySellerAndDateRangeRow {
	type agg struct {
		impressions, clicks, orders int64
		spend, revenue              float64
		latestDate                  time.Time
		hasMetrics                  bool
	}
	byCampaign := make(map[int64]*agg, len(campaigns))
	for _, c := range campaigns {
		byCampaign[c.CampaignExternalID] = &agg{}
	}
	for _, row := range dailyRows {
		cur := byCampaign[row.CampaignExternalID]
		if cur == nil {
			cur = &agg{}
			byCampaign[row.CampaignExternalID] = cur
		}
		cur.impressions += row.Impressions
		cur.clicks += row.Clicks
		cur.orders += int64(row.OrdersCount)
		cur.spend += numericToFloat(row.Spend)
		cur.revenue += numericToFloat(row.Revenue)
		cur.hasMetrics = true
		if row.MetricDate.Valid {
			d := normalizeDate(row.MetricDate.Time)
			if cur.latestDate.IsZero() || d.After(cur.latestDate) {
				cur.latestDate = d
			}
		}
	}

	out := make([]dbgen.ListAdCampaignSummariesBySellerAndDateRangeRow, 0, len(byCampaign))
	for _, c := range campaigns {
		cur := byCampaign[c.CampaignExternalID]
		if cur == nil {
			cur = &agg{}
		}
		row := dbgen.ListAdCampaignSummariesBySellerAndDateRangeRow{
			SellerAccountID:    c.SellerAccountID,
			CampaignExternalID: c.CampaignExternalID,
			CampaignName:       c.CampaignName,
			CampaignType:       c.CampaignType,
			PlacementType:      c.PlacementType,
			Status:             c.Status,
			BudgetAmount:       c.BudgetAmount,
			BudgetDaily:        c.BudgetDaily,
			ImpressionsTotal:   cur.impressions,
			ClicksTotal:        cur.clicks,
			SpendTotal:         numericValue(cur.spend),
			OrdersTotal:        cur.orders,
			RevenueTotal:       numericValue(cur.revenue),
		}
		if cur.hasMetrics && !cur.latestDate.IsZero() {
			row.LatestMetricDate = pgtype.Date{Time: cur.latestDate, Valid: true}
		}
		out = append(out, row)
	}
	return out
}
