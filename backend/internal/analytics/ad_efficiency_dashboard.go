package analytics

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	AdEfficiencyHighDRRThresholdPct      = 100.0
	AdEfficiencyDashboardCampaignLimit   = 5
	AdEfficiencyDashboardTrendWindowDays = 7

	adCampaignReasonHighDRR             = "high_drr"
	adCampaignReasonNoOrders            = "no_orders"
	adCampaignReasonSpendWithoutRevenue = "spend_without_revenue"
)

type AdEfficiencyDashboardDTO struct {
	Period           AdEfficiencyPeriodDTO
	Summary          AdEfficiencySummaryDTO
	Trend7D          []AdEfficiencyTrendPointDTO
	HighDRRCampaigns []AdCampaignDisplayRowDTO
	NoOrderCampaigns []AdCampaignDisplayRowDTO
	LegacyPoints     []AdEfficiencyLegacyPointDTO
	LastUpdatedAt    *string
	HasData          bool
	WidgetStatus     string
}

type AdEfficiencyPeriodDTO struct {
	MetricDate            string
	DateFrom              string // current completed 7d window; retained for compatibility
	DateTo                string // current completed 7d window; retained for compatibility
	CurrentCompletedDate  string
	PreviousCompletedDate string
	Current7dDateFrom     string
	Current7dDateTo       string
	Previous7dDateFrom    string
	Previous7dDateTo      string
	SampleDays            int
	PreviousSampleDays    int
}

type AdEfficiencySummaryDTO struct {
	CurrentDRRPct              *float64
	PreviousDRRPct             *float64
	AverageDRR7dPct            *float64
	DeltaVsPreviousPp          *float64
	DeltaVsAverage7dPp         *float64
	PreviousDRR7dPct           *float64
	DeltaVsPrevious7dPp        *float64
	TotalSpendToday            float64
	TotalRevenueToday          float64 // attributed ad revenue today (denominator for ads share)
	TotalStoreRevenueToday     float64 // cabinet store revenue today (for tooltips / dual-metric context)
	TotalOrdersToday           int64
	TotalSpend7d               float64
	TotalAttributedRevenue7d   float64
	TotalStoreRevenue7d        float64
	CampaignsHighDRRCount      int
	CampaignsNoOrdersCount     int
	HighDRRSpend               float64
	SpendWithoutOrders         float64
	SpendWithoutOrdersDateFrom string
	SpendWithoutOrdersDateTo   string
	HighDRRThresholdPct        float64
}

type AdEfficiencyTrendPointDTO struct {
	Date        string
	Label       string
	Spend       float64
	Revenue     float64
	Orders      int64
	DRRPct      *float64
	IsToday     bool
	IsYesterday bool
	DataStatus  string
}

type AdCampaignDisplayRowDTO struct {
	CampaignExternalID int64
	CampaignName       string
	DRRPct             *float64
	Spend              float64
	Orders             int64
	Revenue            float64
	ROAS               *float64
	CPO                *float64
	CTR                *float64
	CPC                *float64
	ClickToOrderRate   *float64
	ReasonCode         string
	EfficiencySignal   string
	ClicksTotal        int64
}

type AdEfficiencyLegacyPointDTO struct {
	CampaignExternalID int64
	CampaignName       string
	Spend              float64
	DRR                *float64
	Revenue            float64
	Orders             int64
	EfficiencySignal   string
}

type dailyAdAggregate struct {
	spend, revenue float64
	orders         int64
	clicks         int64
	impressions    int64
	hasData        bool
}

func (s *AdvertisingService) BuildAdEfficiencyDashboard(
	ctx context.Context,
	sellerAccountID int64,
	query AdvertisingQuery,
	metricDate time.Time,
) (AdEfficiencyDashboardDTO, error) {
	currentTo := metricDate.UTC().AddDate(0, 0, -1)
	currentFrom := currentTo.AddDate(0, 0, -(AdEfficiencyDashboardTrendWindowDays - 1))
	previousFrom := currentFrom.AddDate(0, 0, -AdEfficiencyDashboardTrendWindowDays)

	campaignRows, err := s.queries.ListAdCampaignSummariesBySellerAndDateRange(ctx, dbgen.ListAdCampaignSummariesBySellerAndDateRangeParams{
		SellerAccountID: sellerAccountID,
		DateFrom:        dateValue(currentFrom),
		DateTo:          dateValue(currentTo),
	})
	if err != nil {
		return AdEfficiencyDashboardDTO{}, fmt.Errorf("list ad campaign summaries: %w", err)
	}

	dailyRows, err := s.queries.ListAdMetricsDailyBySellerAndDateRange(ctx, dbgen.ListAdMetricsDailyBySellerAndDateRangeParams{
		SellerAccountID: sellerAccountID,
		DateFrom:        dateValue(previousFrom),
		DateTo:          dateValue(currentTo),
	})
	if err != nil {
		return AdEfficiencyDashboardDTO{}, fmt.Errorf("list ad daily metrics: %w", err)
	}

	accountRows, err := s.queries.ListDailyAccountMetricsBySellerAndDateRange(ctx, dbgen.ListDailyAccountMetricsBySellerAndDateRangeParams{
		SellerAccountID: sellerAccountID,
		MetricDate:      dateValue(previousFrom),
		MetricDate_2:    dateValue(currentTo),
	})
	if err != nil {
		return AdEfficiencyDashboardDTO{}, fmt.Errorf("list account metrics for account DRR: %w", err)
	}

	var lastUpdated pgtype.Timestamptz
	for _, row := range dailyRows {
		lastUpdated = maxTimestamptz(lastUpdated, row.UpdatedAt)
	}

	return computeAdEfficiencyDashboard(
		campaignRows,
		dailyRows,
		accountRevenueByDateFromRows(accountRows),
		metricDate,
		timestamptzToRFC3339(lastUpdated),
	), nil
}

func accountRevenueByDateFromRows(rows []dbgen.DailyAccountMetric) map[string]float64 {
	out := make(map[string]float64, len(rows))
	for _, row := range rows {
		if !row.MetricDate.Valid {
			continue
		}
		out[row.MetricDate.Time.UTC().Format("2006-01-02")] = numericToFloat(row.Revenue)
	}
	return out
}

func computeAdEfficiencyDashboard(
	campaignRows []dbgen.ListAdCampaignSummariesBySellerAndDateRangeRow,
	dailyRows []dbgen.AdMetricsDaily,
	accountRevenueByDate map[string]float64,
	metricDate time.Time,
	lastUpdatedAt *string,
) AdEfficiencyDashboardDTO {
	metricDate = metricDate.UTC()
	currentCompletedDate := metricDate.AddDate(0, 0, -1)
	previousCompletedDate := metricDate.AddDate(0, 0, -2)
	dateFrom := currentCompletedDate.AddDate(0, 0, -(AdEfficiencyDashboardTrendWindowDays - 1))
	previousDateFrom := dateFrom.AddDate(0, 0, -AdEfficiencyDashboardTrendWindowDays)
	dateFromKey := dateFrom.Format("2006-01-02")
	dateToKey := metricDate.Format("2006-01-02")
	currentCompletedKey := currentCompletedDate.Format("2006-01-02")
	previousCompletedKey := previousCompletedDate.Format("2006-01-02")

	dailyByDate := aggregateDailyAdMetrics(dailyRows)
	metricsByCampaign := groupDailyMetricsByCampaign(dailyRows)
	spendByDate := make(map[string]float64, len(dailyByDate))
	attributedRevenueByDate := make(map[string]float64, len(dailyByDate))
	for key, agg := range dailyByDate {
		if agg.hasData {
			spendByDate[key] = agg.spend
			attributedRevenueByDate[key] = agg.revenue
		}
	}
	if accountRevenueByDate == nil {
		accountRevenueByDate = map[string]float64{}
	}

	sampleDays := 0
	for d := dateFrom; !d.After(currentCompletedDate); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		if agg, ok := dailyByDate[key]; ok && agg.hasData {
			sampleDays++
		}
	}
	previousSampleDays := 0
	for d := previousDateFrom; d.Before(dateFrom); d = d.AddDate(0, 0, 1) {
		if agg, ok := dailyByDate[d.Format("2006-01-02")]; ok && agg.hasData {
			previousSampleDays++
		}
	}

	currentAgg := dailyByDate[currentCompletedKey]
	previousAgg := dailyByDate[previousCompletedKey]
	currentCabinetRev := accountRevenueByDate[currentCompletedKey]
	currentAttributedRev := currentAgg.revenue
	previousAttributedRev := previousAgg.revenue

	// Advertising-block share: spend / attributed ad revenue (NOT store revenue).
	// Seller Pulse uses AccountDRRPct(spend, store revenue) separately.
	currentShare := AdSpendShareOfAttributedRevenuePct(currentAgg.spend, currentAttributedRev)
	previousShare := AdSpendShareOfAttributedRevenuePct(previousAgg.spend, previousAttributedRev)
	periodShare := AccountDRRPeriodPct(spendByDate, attributedRevenueByDate, dateFrom, currentCompletedDate)
	previousPeriodShare := AccountDRRPeriodPct(spendByDate, attributedRevenueByDate, previousDateFrom, previousDateFrom.AddDate(0, 0, 6))
	spend7d, attrRev7d := SumWindowTotals(spendByDate, attributedRevenueByDate, dateFrom, currentCompletedDate)
	_, storeRev7d := SumWindowTotals(spendByDate, accountRevenueByDate, dateFrom, currentCompletedDate)

	summary := AdEfficiencySummaryDTO{
		CurrentDRRPct:              currentShare,
		PreviousDRRPct:             previousShare,
		AverageDRR7dPct:            periodShare,
		DeltaVsPreviousPp:          PercentagePointsDelta(currentShare, previousShare),
		DeltaVsAverage7dPp:         PercentagePointsDelta(currentShare, periodShare),
		PreviousDRR7dPct:           previousPeriodShare,
		DeltaVsPrevious7dPp:        PercentagePointsDelta(periodShare, previousPeriodShare),
		TotalSpendToday:            round2(currentAgg.spend),
		TotalRevenueToday:          round2(currentAttributedRev),
		TotalStoreRevenueToday:     round2(currentCabinetRev),
		TotalOrdersToday:           currentAgg.orders,
		TotalSpend7d:               round2(spend7d),
		TotalAttributedRevenue7d:   round2(attrRev7d),
		TotalStoreRevenue7d:        round2(storeRev7d),
		HighDRRThresholdPct:        AdEfficiencyHighDRRThresholdPct,
		SpendWithoutOrdersDateFrom: dateFromKey,
		SpendWithoutOrdersDateTo:   dateToKey,
	}

	trend7d := buildAdEfficiencyTrend7D(dateFrom, currentCompletedDate, dailyByDate)

	displayRows := make([]AdCampaignDisplayRowDTO, 0, len(campaignRows))
	legacyPoints := make([]AdEfficiencyLegacyPointDTO, 0, len(campaignRows))
	for _, row := range campaignRows {
		spend := numericToFloat(row.SpendTotal)
		revenue := numericToFloat(row.RevenueTotal)
		ctr := CampaignCTR(row.ClicksTotal, row.ImpressionsTotal)
		cpcVal := 0.0
		if cpc := CampaignCPC(spend, row.ClicksTotal); cpc != nil {
			cpcVal = *cpc
		}
		ctrVal := 0.0
		if ctr != nil {
			ctrVal = *ctr
		}
		signal := campaignEfficiencySignal(row, metricsByCampaign[row.CampaignExternalID], ctrVal, cpcVal)
		reasonCode := adCampaignReasonHighDRR
		if revenue <= 0 && spend > 0 && row.OrdersTotal == 0 {
			reasonCode = adCampaignReasonNoOrders
		} else if revenue <= 0 && spend > 0 && row.OrdersTotal > 0 {
			reasonCode = adCampaignReasonSpendWithoutRevenue
		} else if spend > 0 && row.OrdersTotal == 0 {
			reasonCode = adCampaignReasonNoOrders
		}

		display := AdCampaignDisplayRowDTO{
			CampaignExternalID: row.CampaignExternalID,
			CampaignName:       row.CampaignName,
			DRRPct:             CampaignDRRPct(spend, revenue),
			Spend:              round2(spend),
			Orders:             row.OrdersTotal,
			Revenue:            round2(revenue),
			ROAS:               CampaignROAS(revenue, spend),
			CPO:                CampaignCPO(spend, row.OrdersTotal),
			CTR:                ctr,
			CPC:                CampaignCPC(spend, row.ClicksTotal),
			ClickToOrderRate:   CampaignClickToOrderRate(row.OrdersTotal, row.ClicksTotal),
			ReasonCode:         reasonCode,
			EfficiencySignal:   signal,
			ClicksTotal:        row.ClicksTotal,
		}
		displayRows = append(displayRows, display)
		legacyPoints = append(legacyPoints, AdEfficiencyLegacyPointDTO{
			CampaignExternalID: row.CampaignExternalID,
			CampaignName:       row.CampaignName,
			Spend:              round2(spend),
			DRR:                CampaignDRR(spend, revenue),
			Revenue:            round2(revenue),
			Orders:             row.OrdersTotal,
			EfficiencySignal:   signal,
		})
	}

	highDRRAll, noOrderAll := classifyAdCampaignRows(displayRows)
	summary.CampaignsHighDRRCount = len(highDRRAll)
	summary.CampaignsNoOrdersCount = len(noOrderAll)
	for _, row := range highDRRAll {
		summary.HighDRRSpend += row.Spend
	}
	summary.HighDRRSpend = round2(summary.HighDRRSpend)
	for _, row := range noOrderAll {
		summary.SpendWithoutOrders += row.Spend
	}
	summary.SpendWithoutOrders = round2(summary.SpendWithoutOrders)

	hasData := sampleDays > 0 || summary.TotalSpendToday > 0 || summary.TotalRevenueToday > 0 || (len(displayRows) > 0 && hasCampaignSpend(displayRows))
	widgetStatus := "not_enough_data"
	if hasData {
		widgetStatus = "available"
		if sampleDays > 0 && sampleDays < AdEfficiencyDashboardTrendWindowDays {
			widgetStatus = "partial"
		}
		if currentAgg.spend == 0 && currentAgg.revenue == 0 && currentAgg.orders == 0 && sampleDays > 0 {
			widgetStatus = "partial"
		}
	}

	sortAdCampaignDisplayRows(highDRRAll, sortHighDRRCampaignsLess)
	sortAdCampaignDisplayRows(noOrderAll, sortNoOrderCampaignsLess)

	return AdEfficiencyDashboardDTO{
		Period: AdEfficiencyPeriodDTO{
			MetricDate:            dateToKey,
			DateFrom:              dateFromKey,
			DateTo:                currentCompletedKey,
			CurrentCompletedDate:  currentCompletedKey,
			PreviousCompletedDate: previousCompletedKey,
			Current7dDateFrom:     dateFromKey,
			Current7dDateTo:       currentCompletedKey,
			Previous7dDateFrom:    previousDateFrom.Format("2006-01-02"),
			Previous7dDateTo:      dateFrom.AddDate(0, 0, -1).Format("2006-01-02"),
			SampleDays:            sampleDays,
			PreviousSampleDays:    previousSampleDays,
		},
		Summary:          summary,
		Trend7D:          trend7d,
		HighDRRCampaigns: limitAdCampaignRows(highDRRAll, AdEfficiencyDashboardCampaignLimit),
		NoOrderCampaigns: limitAdCampaignRows(noOrderAll, AdEfficiencyDashboardCampaignLimit),
		LegacyPoints:     legacyPoints,
		LastUpdatedAt:    lastUpdatedAt,
		HasData:          hasData,
		WidgetStatus:     widgetStatus,
	}
}

func aggregateDailyAdMetrics(rows []dbgen.AdMetricsDaily) map[string]dailyAdAggregate {
	out := make(map[string]dailyAdAggregate)
	for _, row := range rows {
		if !row.MetricDate.Valid {
			continue
		}
		key := row.MetricDate.Time.UTC().Format("2006-01-02")
		agg := out[key]
		agg.spend += numericToFloat(row.Spend)
		agg.revenue += numericToFloat(row.Revenue)
		agg.orders += int64(row.OrdersCount)
		agg.clicks += row.Clicks
		agg.impressions += row.Impressions
		agg.hasData = true
		out[key] = agg
	}
	return out
}

func groupDailyMetricsByCampaign(rows []dbgen.AdMetricsDaily) map[int64][]dbgen.AdMetricsDaily {
	out := make(map[int64][]dbgen.AdMetricsDaily)
	for _, row := range rows {
		out[row.CampaignExternalID] = append(out[row.CampaignExternalID], row)
	}
	return out
}

func buildAdEfficiencyTrend7D(
	from, to time.Time,
	dailyByDate map[string]dailyAdAggregate,
) []AdEfficiencyTrendPointDTO {
	out := make([]AdEfficiencyTrendPointDTO, 0, AdEfficiencyDashboardTrendWindowDays)
	toKey := to.Format("2006-01-02")
	spendByDate := make(map[string]float64, len(dailyByDate))
	attributedByDate := make(map[string]float64, len(dailyByDate))
	for key, agg := range dailyByDate {
		if agg.hasData {
			spendByDate[key] = agg.spend
			attributedByDate[key] = agg.revenue
		}
	}
	series := BuildAccountDRRDailySeries(from, to, spendByDate, attributedByDate)
	seriesByDate := make(map[string]AccountDRRDayPoint, len(series))
	for _, pt := range series {
		seriesByDate[pt.Date] = pt
	}
	for d := from.UTC(); !d.After(to.UTC()); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		pt := AdEfficiencyTrendPointDTO{
			Date:       key,
			IsToday:    key == toKey, // legacy marker; the point is the completed D-1 day.
			DataStatus: "missing",
		}
		agg, hasAd := dailyByDate[key]
		day, hasSeries := seriesByDate[key]
		if hasAd && agg.hasData {
			pt.Orders = agg.orders
		}
		if hasSeries && day.HasData {
			pt.DataStatus = "available"
			pt.Spend = round2(day.Spend)
			// Revenue is attributed ad revenue (denominator for advertising-block share).
			pt.Revenue = round2(day.Revenue)
			pt.DRRPct = day.DRRPct
		} else if hasAd && agg.hasData {
			pt.DataStatus = "available"
			pt.Spend = round2(agg.spend)
			pt.Revenue = round2(agg.revenue)
			if agg.revenue > 0 {
				pt.DRRPct = AdSpendShareOfAttributedRevenuePct(agg.spend, agg.revenue)
			}
		}
		out = append(out, pt)
	}
	return out
}

func classifyAdCampaignRows(rows []AdCampaignDisplayRowDTO) (highDRR []AdCampaignDisplayRowDTO, noOrders []AdCampaignDisplayRowDTO) {
	for _, row := range rows {
		if row.Spend > 0 && row.Revenue > 0 && row.DRRPct != nil && *row.DRRPct > AdEfficiencyHighDRRThresholdPct {
			high := row
			high.ReasonCode = adCampaignReasonHighDRR
			highDRR = append(highDRR, high)
		}
		if row.Spend > 0 && row.Orders == 0 {
			no := row
			no.ReasonCode = adCampaignReasonNoOrders
			noOrders = append(noOrders, no)
		}
	}
	return highDRR, noOrders
}

func hasCampaignSpend(rows []AdCampaignDisplayRowDTO) bool {
	for _, row := range rows {
		if row.Spend > 0 {
			return true
		}
	}
	return false
}

func sortAdCampaignDisplayRows(rows []AdCampaignDisplayRowDTO, less func(a, b AdCampaignDisplayRowDTO) bool) {
	sort.Slice(rows, func(i, j int) bool {
		return less(rows[i], rows[j])
	})
}

func sortHighDRRCampaignsLess(left, right AdCampaignDisplayRowDTO) bool {
	if left.DRRPct == nil && right.DRRPct == nil {
		return cmpFloat(left.Spend, right.Spend, left.CampaignExternalID, right.CampaignExternalID, true)
	}
	if left.DRRPct == nil {
		return false
	}
	if right.DRRPct == nil {
		return true
	}
	if *left.DRRPct != *right.DRRPct {
		return *left.DRRPct > *right.DRRPct
	}
	if left.Spend != right.Spend {
		return left.Spend > right.Spend
	}
	return left.CampaignExternalID < right.CampaignExternalID
}

func sortNoOrderCampaignsLess(left, right AdCampaignDisplayRowDTO) bool {
	if left.Spend != right.Spend {
		return left.Spend > right.Spend
	}
	if left.ClicksTotal != right.ClicksTotal {
		return left.ClicksTotal > right.ClicksTotal
	}
	return left.CampaignExternalID < right.CampaignExternalID
}

func limitAdCampaignRows(rows []AdCampaignDisplayRowDTO, limit int) []AdCampaignDisplayRowDTO {
	if limit <= 0 || len(rows) <= limit {
		return rows
	}
	return append([]AdCampaignDisplayRowDTO(nil), rows[:limit]...)
}
