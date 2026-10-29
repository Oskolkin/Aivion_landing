package analytics

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AdvertisingService struct {
	queries *dbgen.Queries
}

func NewAdvertisingService(db *pgxpool.Pool) *AdvertisingService {
	return &AdvertisingService{queries: dbgen.New(db)}
}

type AdvertisingQuery struct {
	DateFrom  time.Time
	DateTo    time.Time
	Limit     int
	Offset    int
	SortBy    string
	SortOrder string
}

type AdvertisingAnalyticsDTO struct {
	Summary  AdvertisingSummaryDTO      `json:"summary"`
	Campaign []AdvertisingCampaignRow   `json:"campaigns"`
	SKURisks []AdvertisingSKURiskRowDTO `json:"sku_risks"`
}

type AdvertisingSummaryDTO struct {
	TotalSpend             float64 `json:"total_spend"`
	ActiveCampaignsCount   int     `json:"active_campaigns_count"`
	WeakCampaignsCount     int     `json:"weak_campaigns_count"`
	LowStockAdvertisedSKUs int     `json:"low_stock_advertised_skus_count"`
	DateFrom               string  `json:"date_from"`
	DateTo                 string  `json:"date_to"`
	LastUpdatedAt          *string `json:"last_updated_at"`
}

type AdvertisingCampaignRow struct {
	CampaignExternalID       int64    `json:"campaign_external_id"`
	CampaignName             string   `json:"campaign_name"`
	CampaignType             string   `json:"campaign_type"`
	State                    string   `json:"state"`
	Placement                string   `json:"placement"`
	SpendTotal               float64  `json:"spend_total"`
	ImpressionsTotal         int64    `json:"impressions_total"`
	ClicksTotal              int64    `json:"clicks_total"`
	OrdersTotal              int64    `json:"orders_total"`
	RevenueTotal             float64  `json:"revenue_total"`
	CTR                      *float64 `json:"ctr,omitempty"`
	CPC                      *float64 `json:"cpc,omitempty"`
	CampaignDRR              *float64 `json:"campaign_drr,omitempty"`
	CampaignROAS             *float64 `json:"campaign_roas,omitempty"`
	CampaignCPO              *float64 `json:"campaign_cpo,omitempty"`
	CampaignClickToOrderRate *float64 `json:"campaign_click_to_order_rate,omitempty"`
	CampaignSpendShare       *float64 `json:"campaign_spend_share,omitempty"`
	EfficiencySignal         string   `json:"efficiency_signal"`
	LatestMetricDate         *string  `json:"latest_metric_date"`
}

type AdvertisingSKURiskRowDTO struct {
	ProductID          int64   `json:"product_id"`
	SKU                *int64  `json:"sku"`
	OfferID            *string `json:"offer_id"`
	ProductName        *string `json:"product_name"`
	CampaignExternalID int64   `json:"campaign_external_id"`
	CampaignName       string  `json:"campaign_name"`
	SpendTotal         float64 `json:"spend_total"`
	SalesTrendSignal   string  `json:"sales_trend_signal"`
	StockSignal        string  `json:"stock_signal"`
	AdWasteSignal      string  `json:"ad_waste_signal"`
	CombinedReason     string  `json:"combined_reason"`
}

func (s *AdvertisingService) BuildAdvertisingAnalytics(ctx context.Context, sellerAccountID int64, query AdvertisingQuery) (AdvertisingAnalyticsDTO, error) {
	campaignRows, err := s.queries.ListAdCampaignSummariesBySellerAndDateRange(ctx, dbgen.ListAdCampaignSummariesBySellerAndDateRangeParams{
		SellerAccountID: sellerAccountID,
		DateFrom:        dateValue(query.DateFrom),
		DateTo:          dateValue(query.DateTo),
	})
	if err != nil {
		return AdvertisingAnalyticsDTO{}, fmt.Errorf("list ad campaign summaries: %w", err)
	}

	dailyRows, err := s.queries.ListAdMetricsDailyBySellerAndDateRange(ctx, dbgen.ListAdMetricsDailyBySellerAndDateRangeParams{
		SellerAccountID: sellerAccountID,
		DateFrom:        dateValue(query.DateFrom),
		DateTo:          dateValue(query.DateTo),
	})
	if err != nil {
		return AdvertisingAnalyticsDTO{}, fmt.Errorf("list ad daily metrics: %w", err)
	}

	links, err := s.queries.ListAdCampaignSKUMappingsBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		return AdvertisingAnalyticsDTO{}, fmt.Errorf("list ad campaign sku links: %w", err)
	}

	skuRows, err := s.queries.ListDailySKUMetricsBySellerAndDateRange(ctx, dbgen.ListDailySKUMetricsBySellerAndDateRangeParams{
		SellerAccountID: sellerAccountID,
		MetricDate:      dateValue(query.DateFrom.AddDate(0, 0, -1)),
		MetricDate_2:    dateValue(query.DateTo),
	})
	if err != nil {
		return AdvertisingAnalyticsDTO{}, fmt.Errorf("list daily sku metrics: %w", err)
	}

	metricsByCampaign := make(map[int64][]dbgen.AdMetricsDaily)
	var lastUpdated pgtype.Timestamptz
	for _, row := range dailyRows {
		metricsByCampaign[row.CampaignExternalID] = append(metricsByCampaign[row.CampaignExternalID], row)
		lastUpdated = maxTimestamptz(lastUpdated, row.UpdatedAt)
	}
	for _, row := range links {
		lastUpdated = maxTimestamptz(lastUpdated, row.UpdatedAt)
	}
	for _, row := range skuRows {
		lastUpdated = maxTimestamptz(lastUpdated, row.UpdatedAt)
	}

	campaigns := make([]AdvertisingCampaignRow, 0, len(campaignRows))
	campaignByID := make(map[int64]AdvertisingCampaignRow, len(campaignRows))
	totalSpend := 0.0
	activeCount := 0
	weakCount := 0
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
		if isActiveCampaign(row.Status) {
			activeCount++
		}
		if signal != "ok" {
			weakCount++
		}
		campaign := AdvertisingCampaignRow{
			CampaignExternalID:       row.CampaignExternalID,
			CampaignName:             row.CampaignName,
			CampaignType:             safeText(row.CampaignType),
			State:                    safeText(row.Status),
			Placement:                safeText(row.PlacementType),
			SpendTotal:               spend,
			ImpressionsTotal:         row.ImpressionsTotal,
			ClicksTotal:              row.ClicksTotal,
			OrdersTotal:              row.OrdersTotal,
			RevenueTotal:             revenue,
			CTR:                      ctr,
			CPC:                      CampaignCPC(spend, row.ClicksTotal),
			CampaignDRR:              CampaignDRR(spend, revenue),
			CampaignROAS:             CampaignROAS(revenue, spend),
			CampaignCPO:              CampaignCPO(spend, row.OrdersTotal),
			CampaignClickToOrderRate: CampaignClickToOrderRate(row.OrdersTotal, row.ClicksTotal),
			EfficiencySignal:         signal,
			LatestMetricDate:         datePtr(row.LatestMetricDate),
		}
		campaignByID[campaign.CampaignExternalID] = campaign
		campaigns = append(campaigns, campaign)
		totalSpend += spend
	}

	latestSKUByProduct, previousSKUByProduct := splitLatestAndPreviousByProduct(skuRows, query.DateTo)
	linksByCampaign := make(map[int64][]dbgen.ListAdCampaignSKUMappingsBySellerAccountIDRow)
	for _, link := range links {
		linksByCampaign[link.CampaignExternalID] = append(linksByCampaign[link.CampaignExternalID], link)
	}

	skuRisks := make([]AdvertisingSKURiskRowDTO, 0)
	lowStockProduct := map[int64]struct{}{}
	for campaignID, mapped := range linksByCampaign {
		campaign, exists := campaignByID[campaignID]
		if !exists || len(mapped) == 0 {
			continue
		}
		spendPerSKU := campaign.SpendTotal / float64(len(mapped))
		for _, link := range mapped {
			latest := latestSKUByProduct[link.OzonProductID]
			prev := previousSKUByProduct[link.OzonProductID]
			salesSignal := resolveSalesTrendSignal(latest, prev)
			stockSignal := resolveStockSignal(latest)
			adWasteSignal := resolveAdWasteSignal(spendPerSKU, salesSignal, campaign.EfficiencySignal)
			reasons := make([]string, 0, 3)
			if adWasteSignal != "ok" {
				reasons = append(reasons, "growth_without_result")
			}
			if stockSignal != "ok" {
				reasons = append(reasons, "low_stock_advertised")
				lowStockProduct[link.OzonProductID] = struct{}{}
			}
			if salesSignal != "stable" && spendPerSKU > 0 {
				reasons = append(reasons, "ad_spend_on_weak_sales_trend")
			}
			combined := strings.Join(reasons, "; ")
			if combined == "" {
				combined = "no_material_risk_detected"
			}
			skuRisks = append(skuRisks, AdvertisingSKURiskRowDTO{
				ProductID:          link.OzonProductID,
				SKU:                int8Ptr(link.Sku),
				OfferID:            textPtr(link.OfferID),
				ProductName:        textPtr(link.ProductName),
				CampaignExternalID: campaign.CampaignExternalID,
				CampaignName:       campaign.CampaignName,
				SpendTotal:         round2(spendPerSKU),
				SalesTrendSignal:   salesSignal,
				StockSignal:        stockSignal,
				AdWasteSignal:      adWasteSignal,
				CombinedReason:     combined,
			})
		}
	}

	for i := range campaigns {
		campaigns[i].CampaignSpendShare = CampaignSpendShare(campaigns[i].SpendTotal, totalSpend)
	}

	sortCampaignRows(campaigns, query.SortBy, query.SortOrder)
	sortSKURiskRows(skuRisks, query.SortBy, query.SortOrder)
	campaigns = applyPaginationCampaign(campaigns, query.Offset, query.Limit)
	skuRisks = applyPaginationRisk(skuRisks, query.Offset, query.Limit)

	return AdvertisingAnalyticsDTO{
		Summary: AdvertisingSummaryDTO{
			TotalSpend:             round2(totalSpend),
			ActiveCampaignsCount:   activeCount,
			WeakCampaignsCount:     weakCount,
			LowStockAdvertisedSKUs: len(lowStockProduct),
			DateFrom:               query.DateFrom.Format("2006-01-02"),
			DateTo:                 query.DateTo.Format("2006-01-02"),
			LastUpdatedAt:          timestamptzToRFC3339(lastUpdated),
		},
		Campaign: campaigns,
		SKURisks: skuRisks,
	}, nil
}

func splitLatestAndPreviousByProduct(rows []dbgen.DailySkuMetric, upperBound time.Time) (map[int64]dbgen.DailySkuMetric, map[int64]dbgen.DailySkuMetric) {
	latest := make(map[int64]dbgen.DailySkuMetric)
	previous := make(map[int64]dbgen.DailySkuMetric)
	for _, row := range rows {
		if row.MetricDate.Time.After(upperBound.AddDate(0, 0, 1)) {
			continue
		}
		curLatest, exists := latest[row.OzonProductID]
		if !exists || row.MetricDate.Time.After(curLatest.MetricDate.Time) {
			previous[row.OzonProductID] = curLatest
			latest[row.OzonProductID] = row
			continue
		}
		curPrev, prevExists := previous[row.OzonProductID]
		if !prevExists || row.MetricDate.Time.After(curPrev.MetricDate.Time) {
			previous[row.OzonProductID] = row
		}
	}
	return latest, previous
}

func campaignEfficiencySignal(summary dbgen.ListAdCampaignSummariesBySellerAndDateRangeRow, daily []dbgen.AdMetricsDaily, ctr float64, cpc float64) string {
	spend := numericToFloat(summary.SpendTotal)
	revenue := numericToFloat(summary.RevenueTotal)
	if spend <= 0 {
		return "ok"
	}
	if hasSpendGrowthWithoutResult(daily) {
		return "growth_without_result"
	}
	if summary.OrdersTotal == 0 || revenue < spend || ctr < 0.005 || cpc > 60 {
		return "weak_efficiency"
	}
	return "ok"
}

func hasSpendGrowthWithoutResult(rows []dbgen.AdMetricsDaily) bool {
	if len(rows) < 4 {
		return false
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].MetricDate.Time.Before(rows[j].MetricDate.Time)
	})
	split := len(rows) / 2
	var prevSpend, curSpend, prevOrders, curOrders float64
	for i, row := range rows {
		targetSpend := &curSpend
		targetOrders := &curOrders
		if i < split {
			targetSpend = &prevSpend
			targetOrders = &prevOrders
		}
		*targetSpend += numericToFloat(row.Spend)
		*targetOrders += float64(row.OrdersCount)
	}
	if prevSpend <= 0 {
		return false
	}
	spendGrowth := (curSpend - prevSpend) / prevSpend
	if spendGrowth < 0.2 {
		return false
	}
	if prevOrders <= 0 && curOrders <= 0 {
		return true
	}
	if prevOrders <= 0 {
		return false
	}
	orderGrowth := (curOrders - prevOrders) / prevOrders
	return orderGrowth < 0.05
}

func resolveSalesTrendSignal(latest dbgen.DailySkuMetric, previous dbgen.DailySkuMetric) string {
	latestRevenue := numericToFloat(latest.Revenue)
	prevRevenue := numericToFloat(previous.Revenue)
	if latest.OzonProductID == 0 || previous.OzonProductID == 0 {
		if latestRevenue <= 0 {
			return "weak"
		}
		return "stable"
	}
	if latestRevenue <= 0 && prevRevenue > 0 {
		return "declining"
	}
	if prevRevenue > 0 && latestRevenue < prevRevenue*0.8 {
		return "declining"
	}
	if latest.OrdersCount < previous.OrdersCount {
		return "weak"
	}
	return "stable"
}

func resolveStockSignal(latest dbgen.DailySkuMetric) string {
	daysCover := numericToFloatPtr(latest.DaysOfCover)
	if latest.StockAvailable <= 0 {
		return "critical"
	}
	if latest.StockAvailable <= 3 {
		return "low"
	}
	if daysCover != nil && *daysCover <= 3 {
		return "low"
	}
	return "ok"
}

func resolveAdWasteSignal(spendPerSKU float64, salesSignal string, efficiencySignal string) string {
	if spendPerSKU <= 0 {
		return "ok"
	}
	if salesSignal == "declining" || efficiencySignal == "growth_without_result" {
		return "high"
	}
	if salesSignal == "weak" || efficiencySignal == "weak_efficiency" {
		return "medium"
	}
	return "ok"
}

func isActiveCampaign(status pgtype.Text) bool {
	v := strings.ToLower(strings.TrimSpace(status.String))
	if v == "" {
		return false
	}
	return strings.Contains(v, "active") || v == "running" || v == "enabled"
}

func ptrFloat(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

func safeText(v pgtype.Text) string {
	if !v.Valid {
		return ""
	}
	return strings.TrimSpace(v.String)
}

func datePtr(v pgtype.Date) *string {
	if !v.Valid {
		return nil
	}
	s := v.Time.Format("2006-01-02")
	return &s
}

func sortCampaignRows(rows []AdvertisingCampaignRow, sortBy string, sortOrder string) {
	key := strings.ToLower(strings.TrimSpace(sortBy))
	desc := strings.ToLower(strings.TrimSpace(sortOrder)) != "asc"
	if key == "" {
		key = "spend_total"
	}
	sort.Slice(rows, func(i, j int) bool {
		left, right := rows[i], rows[j]
		switch key {
		case "revenue_total":
			return cmpFloat(left.RevenueTotal, right.RevenueTotal, left.CampaignExternalID, right.CampaignExternalID, desc)
		case "ctr":
			return cmpFloat(ptrFloat(left.CTR), ptrFloat(right.CTR), left.CampaignExternalID, right.CampaignExternalID, desc)
		case "cpc":
			return cmpFloat(ptrFloat(left.CPC), ptrFloat(right.CPC), left.CampaignExternalID, right.CampaignExternalID, desc)
		case "clicks_total":
			return cmpInt64(left.ClicksTotal, right.ClicksTotal, left.CampaignExternalID, right.CampaignExternalID, desc)
		default:
			return cmpFloat(left.SpendTotal, right.SpendTotal, left.CampaignExternalID, right.CampaignExternalID, desc)
		}
	})
}

func sortSKURiskRows(rows []AdvertisingSKURiskRowDTO, sortBy string, sortOrder string) {
	key := strings.ToLower(strings.TrimSpace(sortBy))
	desc := strings.ToLower(strings.TrimSpace(sortOrder)) != "asc"
	if key == "" {
		key = "spend_total"
	}
	sort.Slice(rows, func(i, j int) bool {
		left, right := rows[i], rows[j]
		switch key {
		case "stock_signal":
			return cmpString(left.StockSignal, right.StockSignal, left.ProductID, right.ProductID, desc)
		case "sales_trend_signal":
			return cmpString(left.SalesTrendSignal, right.SalesTrendSignal, left.ProductID, right.ProductID, desc)
		default:
			return cmpFloat(left.SpendTotal, right.SpendTotal, left.ProductID, right.ProductID, desc)
		}
	})
}

func applyPaginationCampaign(rows []AdvertisingCampaignRow, offset int, limit int) []AdvertisingCampaignRow {
	total := len(rows)
	if offset >= total {
		return []AdvertisingCampaignRow{}
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return rows[offset:end]
}

func applyPaginationRisk(rows []AdvertisingSKURiskRowDTO, offset int, limit int) []AdvertisingSKURiskRowDTO {
	total := len(rows)
	if offset >= total {
		return []AdvertisingSKURiskRowDTO{}
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return rows[offset:end]
}

func cmpFloat(left float64, right float64, leftTie int64, rightTie int64, desc bool) bool {
	if left == right {
		return leftTie < rightTie
	}
	if desc {
		return left > right
	}
	return left < right
}

func cmpInt64(left int64, right int64, leftTie int64, rightTie int64, desc bool) bool {
	if left == right {
		return leftTie < rightTie
	}
	if desc {
		return left > right
	}
	return left < right
}

func cmpString(left string, right string, leftTie int64, rightTie int64, desc bool) bool {
	if left == right {
		return leftTie < rightTie
	}
	if desc {
		return left > right
	}
	return left < right
}
