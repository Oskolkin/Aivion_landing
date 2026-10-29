package aisummaries

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/alerts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

const maxAccountContextBytes = 320 * 1024
const maxCategoryContextBytes = 120 * 1024
const maxFunnelSignalItems = 5
const accountContextMaxCategoryRollups = 12
const accountContextMaxEvidencePerCategory = 3
const accountContextMaxUniqueCandidates = 100

func pgDateUTC(t time.Time) pgtype.Date {
	u := t.UTC()
	return pgtype.Date{Time: time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC), Valid: true}
}

func truncateJSONForStore(raw []byte, max int) []byte {
	if len(raw) <= max {
		return raw
	}
	out := make([]byte, max)
	copy(out, raw)
	return append(out, []byte(`…"truncated":true}`)...)
}

// AccountContextPackage is the structured payload sent to the model for account summaries.
type AccountContextPackage struct {
	MetricDate                  string           `json:"metric_date"`
	DataQualityNotes            []string         `json:"data_quality_notes"`
	IncludedAccountMetricGroups []string         `json:"included_account_metric_groups,omitempty"`
	SKUCoverage                 map[string]any   `json:"sku_coverage,omitempty"`
	AccountAggregates           map[string]any   `json:"account_aggregates,omitempty"`
	ContextCompaction           map[string]any   `json:"context_compaction,omitempty"`
	PreviousSummary             map[string]any   `json:"previous_summary_context,omitempty"`
	Dashboard                   map[string]any   `json:"dashboard,omitempty"`
	FunnelContext               map[string]any   `json:"funnel_context,omitempty"`
	SKUCandidates               []map[string]any `json:"sku_candidates,omitempty"`
	OpenAlertsSample            []map[string]any `json:"open_alerts_sample,omitempty"`
	OpenRecommendationsSample   []map[string]any `json:"open_recommendations_sample,omitempty"`
	AdCampaignsToday            []map[string]any `json:"ad_campaigns_today,omitempty"`
	AdMetricsToday              []map[string]any `json:"ad_metrics_today,omitempty"`
	CategoryRollupTop           []map[string]any `json:"category_rollup_top,omitempty"`
	OtherCategoriesSummary      map[string]any   `json:"other_categories_summary,omitempty"`
	AccountMetricsWindow        []map[string]any `json:"account_metrics_last_7d,omitempty"`
	ContextTruncated            bool             `json:"context_truncated,omitempty"`
}

// CategoryContextPackage is the structured payload for a single category summary.
type CategoryContextPackage struct {
	MetricDate            string           `json:"metric_date"`
	DataQualityNotes      []string         `json:"data_quality_notes"`
	Identity              map[string]any   `json:"category"`
	PreviousSummary       map[string]any   `json:"previous_summary_context,omitempty"`
	SKUSlice              map[string]any   `json:"sku_slices,omitempty"`
	AlertsSample          []map[string]any `json:"alerts_sample,omitempty"`
	RecommendationsSample []map[string]any `json:"recommendations_sample,omitempty"`
	AdLinkageNote         string           `json:"ad_category_linkage_note,omitempty"`
}

func buildPreviousSummaryContextFromRow(row dbgen.AiDailySummary) map[string]any {
	if len(row.SummaryJson) == 0 {
		return nil
	}
	var raw map[string]any
	if err := json.Unmarshal(row.SummaryJson, &raw); err != nil {
		return map[string]any{"parse_error": true}
	}
	out := map[string]any{}
	for _, k := range []string{
		"headline", "health_score", "severity", "data_richness", "summary_depth",
		"executive_summary", "key_findings", "likely_causes", "recommended_actions",
		"watchlist", "data_quality_notes", "funnel_ai_analysis",
	} {
		if v, ok := raw[k]; ok {
			out[k] = v
		}
	}
	if w, ok := raw["what_changed_since_previous_summary"]; ok {
		out["what_changed_since_previous_summary"] = w
	}
	return out
}

func BuildAccountContext(
	ctx context.Context,
	q *dbgen.Queries,
	dash *analytics.DashboardService,
	sellerID int64,
	metricDate time.Time,
	prev *dbgen.AiDailySummary,
) (*AccountContextPackage, error) {
	asOf := metricDate.UTC()
	pkg := &AccountContextPackage{
		MetricDate:       asOf.Format("2006-01-02"),
		DataQualityNotes: []string{},
		IncludedAccountMetricGroups: []string{
			"backend_deterministic_scan: all active SKUs are scanned by backend metrics pipeline",
			"account_aggregates: full-assortment counts, dynamics, risk totals",
			"category_rollup_top + other_categories_summary: category-level aggregates from full assortment",
			"sku_candidates: compact evidence subset with selection_reason and coverage metadata",
			"funnel_context: account-level funnel and linked risk signals",
		},
		ContextCompaction: map[string]any{
			"raw_full_sku_list_in_prompt": false,
			"selection_policy":            "top revenue/risk/order_drop/stock/priority candidates; backend scans full SKU universe",
		},
	}
	if prev != nil && prev.ID.Valid {
		pkg.PreviousSummary = buildPreviousSummaryContextFromRow(*prev)
	} else {
		pkg.DataQualityNotes = append(pkg.DataQualityNotes, "no_previous_summary")
	}

	dto, err := dash.BuildDashboardMetrics(ctx, sellerID, &asOf)
	if err != nil {
		return nil, fmt.Errorf("dashboard metrics: %w", err)
	}

	openAlerts, err := q.ListOpenAlertsBySellerAccountID(ctx, dbgen.ListOpenAlertsBySellerAccountIDParams{
		SellerAccountID: sellerID,
		Limit:           120,
		Offset:          0,
	})
	if err != nil {
		return nil, err
	}
	for _, a := range openAlerts {
		alertType := alerts.AlertType(a.AlertType)
		alertGroup := alerts.AlertGroup(a.AlertGroup)
		if alerts.IsPriceEconomicsAlertType(alertType) || alerts.IsPriceEconomicsAlertGroup(alertGroup) {
			continue
		}
		if alerts.IsLegacyAlertType(alertType) {
			continue
		}
		pkg.OpenAlertsSample = append(pkg.OpenAlertsSample, map[string]any{
			"id": a.ID, "type": a.AlertType, "severity": a.Severity, "title": a.Title,
			"entity_sku": nullableInt64(a.EntitySku), "message": truncateStr(a.Message, 400),
		})
		if len(pkg.OpenAlertsSample) >= 20 {
			break
		}
	}

	recs, err := q.ListOpenRecommendationsBySellerAccountID(ctx, dbgen.ListOpenRecommendationsBySellerAccountIDParams{
		SellerAccountID: sellerID,
		Limit:           120,
		Offset:          0,
	})
	if err != nil {
		return nil, err
	}
	for _, r := range recs {
		pkg.OpenRecommendationsSample = append(pkg.OpenRecommendationsSample, map[string]any{
			"id": r.ID, "type": r.RecommendationType, "priority": r.PriorityLevel,
			"title": r.Title, "entity_sku": nullableInt64(r.EntitySku),
		})
		if len(pkg.OpenRecommendationsSample) >= 20 {
			break
		}
	}

	from7 := asOf.AddDate(0, 0, -6)
	accRows, err := q.ListDailyAccountMetricsBySellerAndDateRange(ctx, dbgen.ListDailyAccountMetricsBySellerAndDateRangeParams{
		SellerAccountID: sellerID,
		MetricDate:      pgDateUTC(from7),
		MetricDate_2:    pgDateUTC(asOf),
	})
	if err != nil {
		return nil, err
	}
	for _, r := range accRows {
		pkg.AccountMetricsWindow = append(pkg.AccountMetricsWindow, map[string]any{
			"date":    r.MetricDate.Time.Format("2006-01-02"),
			"revenue": numericToFloat(r.Revenue), "orders": r.OrdersCount,
			"units_sold": intOrZero(r.UnitsSold), "returns": r.ReturnsCount, "cancels": r.CancelCount,
		})
	}
	syncDashboardAccountOrdersFromMetrics(&dto, accRows, asOf)
	pkg.Dashboard = dashboardDTOToCompactMap(dto)
	pkg.AccountAggregates = buildAccountAggregates(dto, accRows, openAlerts, recs)
	pkg.SKUCandidates = buildAccountSKUCandidates(dto.SKUs, openAlerts, recs, accountContextMaxUniqueCandidates)
	pkg.SKUCoverage = map[string]any{
		"total_sku_count":           len(dto.SKUs),
		"backend_scanned_sku_count": len(dto.SKUs),
		"sku_candidates_in_prompt":  len(pkg.SKUCandidates),
		"selection_policy":          "top risk/revenue/stock/order_drop candidates; full SKU universe scanned by backend",
	}

	adCamp, err := q.ListAdCampaignSummariesBySellerAndDateRange(ctx, dbgen.ListAdCampaignSummariesBySellerAndDateRangeParams{
		SellerAccountID: sellerID,
		DateFrom:        pgDateUTC(asOf),
		DateTo:          pgDateUTC(asOf),
	})
	if err != nil {
		return nil, err
	}
	for i, c := range adCamp {
		if i >= 25 {
			break
		}
		spend := numericToFloat(c.SpendTotal)
		rev := numericToFloat(c.RevenueTotal)
		drr := 0.0
		if rev > 0 {
			drr = spend / rev * 100
		}
		pkg.AdCampaignsToday = append(pkg.AdCampaignsToday, map[string]any{
			"campaign_id": c.CampaignExternalID, "name": c.CampaignName,
			"spend": spend, "revenue": rev, "orders": c.OrdersTotal, "impressions": c.ImpressionsTotal,
			"clicks": c.ClicksTotal, "approx_drr_pct": drr,
		})
	}

	adDaily, err := q.ListAdMetricsDailyBySellerAndDateRange(ctx, dbgen.ListAdMetricsDailyBySellerAndDateRangeParams{
		SellerAccountID: sellerID,
		DateFrom:        pgDateUTC(asOf),
		DateTo:          pgDateUTC(asOf),
	})
	if err != nil {
		return nil, err
	}
	for i, m := range adDaily {
		if i >= 40 {
			break
		}
		spend := numericToFloat(m.Spend)
		rev := numericToFloat(m.Revenue)
		ctrPct := 0.0
		if m.Impressions > 0 {
			ctrPct = float64(m.Clicks) / float64(m.Impressions) * 100
		}
		cpc := 0.0
		if m.Clicks > 0 {
			cpc = spend / float64(m.Clicks)
		}
		roas := 0.0
		if spend > 0 {
			roas = rev / spend
		}
		cpo := 0.0
		if m.OrdersCount > 0 {
			cpo = spend / float64(m.OrdersCount)
		}
		pkg.AdMetricsToday = append(pkg.AdMetricsToday, map[string]any{
			"campaign_id": m.CampaignExternalID,
			"impressions": m.Impressions, "clicks": m.Clicks,
			"spend": spend, "revenue": rev, "orders": m.OrdersCount,
			"approx_ctr_pct": ctrPct, "approx_cpc": cpc, "approx_roas": roas, "approx_cpo": cpo,
		})
	}
	if len(pkg.AdMetricsToday) == 0 {
		pkg.DataQualityNotes = append(pkg.DataQualityNotes, "no_ad_metrics_for_metric_date")
	}

	rollup, err := q.ListDescriptionCategoryRollupForAISummary(ctx, dbgen.ListDescriptionCategoryRollupForAISummaryParams{
		SellerAccountID: sellerID,
		MetricDate:      pgDateUTC(asOf),
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(rollup, func(i, j int) bool {
		return numericToFloat(rollup[i].RevenueSum) > numericToFloat(rollup[j].RevenueSum)
	})
	totalCatCount := len(rollup)
	for i, r := range rollup {
		if i >= accountContextMaxCategoryRollups {
			break
		}
		if !r.DescriptionCategoryID.Valid {
			continue
		}
		cid := r.DescriptionCategoryID.Int64
		name := ""
		if dc, err := q.GetDescriptionCategoryByID(ctx, cid); err == nil {
			name = dc.CategoryName
		}
		if name == "" {
			pkg.DataQualityNotes = append(pkg.DataQualityNotes, fmt.Sprintf("missing_category_name_for_id:%d", cid))
		}
		catSignals := buildCategoryEvidenceSignals(dto.SKUs, cid)
		pkg.CategoryRollupTop = append(pkg.CategoryRollupTop, map[string]any{
			"category_id": cid, "category_name": name,
			"revenue_7d": numericToFloat(r.RevenueSum), "orders_7d": r.OrdersSum,
			"revenue_prev_7d": nil, "orders_prev_7d": nil,
			"active_products": r.ActiveProductCount, "with_stock": r.ProductsWithStockCount,
			"sku_count":            r.ActiveProductCount,
			"risky_sku_count":      catSignals["risky_sku_count"],
			"stock_risk_count":     catSignals["stock_risk_count"],
			"order_drop_count":     catSignals["order_drop_count"],
			"overstock_count":      catSignals["overstock_count"],
			"ad_risk_count":        catSignals["ad_risk_count"],
			"top_evidence_signals": catSignals["top_evidence_signals"],
		})
	}
	if totalCatCount > len(pkg.CategoryRollupTop) {
		pkg.OtherCategoriesSummary = map[string]any{
			"categories_count": totalCatCount - len(pkg.CategoryRollupTop),
			"note":             "remaining categories included in full-assortment aggregates but not expanded individually",
		}
	}
	if pkg.AccountAggregates != nil {
		pkg.AccountAggregates["category_count"] = totalCatCount
	}
	pkg.FunnelContext = buildFunnelContext(ctx, q, sellerID, asOf, dto, accRows, openAlerts, recs, adCamp)

	raw, _ := json.Marshal(pkg)
	if len(raw) > maxAccountContextBytes {
		pkg.OpenAlertsSample = clampMapSlice(pkg.OpenAlertsSample, 8)
		pkg.OpenRecommendationsSample = clampMapSlice(pkg.OpenRecommendationsSample, 8)
		pkg.AdMetricsToday = clampMapSlice(pkg.AdMetricsToday, 15)
		pkg.AdCampaignsToday = clampMapSlice(pkg.AdCampaignsToday, 10)
		pkg.CategoryRollupTop = clampMapSlice(pkg.CategoryRollupTop, 8)
		pkg.SKUCandidates = clampMapSlice(pkg.SKUCandidates, 60)
		if pkg.SKUCoverage != nil {
			pkg.SKUCoverage["sku_candidates_in_prompt"] = len(pkg.SKUCandidates)
		}
		pkg.AccountMetricsWindow = clampMapSlice(pkg.AccountMetricsWindow, 7)
		pkg.ContextTruncated = true
		pkg.DataQualityNotes = append(pkg.DataQualityNotes, "context_truncated_for_token_budget")
	}
	return pkg, nil
}

func BuildCategoryContext(
	ctx context.Context,
	q *dbgen.Queries,
	sellerID int64,
	categoryID int64,
	categoryName string,
	metricDate time.Time,
	prev *dbgen.AiDailySummary,
	skus []dbgen.ListProductSKUMetricsForCategoryAndDateRow,
) *CategoryContextPackage {
	asOf := metricDate.UTC()
	pkg := &CategoryContextPackage{
		MetricDate: asOf.Format("2006-01-02"),
		Identity: map[string]any{
			"category_id": categoryID, "category_name": categoryName,
			"active_sku_count": len(skus),
		},
		DataQualityNotes: []string{},
		AdLinkageNote:    "Рекламные метрики на уровне категории в данных не агрегированы; используй только SKU/кабинетные рекламные срезы из account-контекста как косвенные сигналы.",
	}
	if categoryName == "" {
		pkg.DataQualityNotes = append(pkg.DataQualityNotes, "category_name_missing_used_fallback")
	}
	if prev != nil && prev.ID.Valid {
		pkg.PreviousSummary = buildPreviousSummaryContextFromRow(*prev)
	} else {
		pkg.DataQualityNotes = append(pkg.DataQualityNotes, "no_previous_category_summary")
	}

	var rev []dbgen.ListProductSKUMetricsForCategoryAndDateRow
	rev = append(rev, skus...)
	sort.Slice(rev, func(i, j int) bool {
		return numericToFloat(rev[i].Revenue) > numericToFloat(rev[j].Revenue)
	})
	topByRev := clampSlice(rev, 12)

	var withStockNoSales []dbgen.ListProductSKUMetricsForCategoryAndDateRow
	for _, s := range skus {
		if s.OrdersCount == 0 && s.StockAvailable > 0 {
			withStockNoSales = append(withStockNoSales, s)
			if len(withStockNoSales) >= 12 {
				break
			}
		}
	}

	pkg.SKUSlice = map[string]any{
		"top_by_revenue":        skuRowsToMaps(topByRev),
		"no_sales_but_in_stock": skuRowsToMaps(withStockNoSales),
	}

	// enrich identity counts
	var ordersN, withSales, withStock int
	var totalStock int32
	for _, s := range skus {
		totalStock += s.StockAvailable
		if s.StockAvailable > 0 {
			withStock++
		}
		if s.OrdersCount > 0 {
			withSales++
		}
		ordersN += int(s.OrdersCount)
	}
	pkg.Identity["sku_with_stock_count"] = withStock
	pkg.Identity["sku_with_sales_count"] = withSales
	pkg.Identity["sku_without_sales_count"] = len(skus) - withSales
	pkg.Identity["orders_count_total"] = ordersN
	pkg.Identity["total_stock_available"] = totalStock

	categoryAlertRows, _ := q.ListOpenAlertsBySellerAccountID(ctx, dbgen.ListOpenAlertsBySellerAccountIDParams{SellerAccountID: sellerID, Limit: 80, Offset: 0})
	skuSet := map[int64]struct{}{}
	for _, s := range skus {
		if s.Sku.Valid {
			skuSet[s.Sku.Int64] = struct{}{}
		}
	}
	for _, a := range categoryAlertRows {
		alertType := alerts.AlertType(a.AlertType)
		alertGroup := alerts.AlertGroup(a.AlertGroup)
		if alerts.IsPriceEconomicsAlertType(alertType) || alerts.IsPriceEconomicsAlertGroup(alertGroup) {
			continue
		}
		if alerts.IsLegacyAlertType(alertType) {
			continue
		}
		if !a.EntitySku.Valid {
			continue
		}
		if _, ok := skuSet[a.EntitySku.Int64]; !ok {
			continue
		}
		pkg.AlertsSample = append(pkg.AlertsSample, map[string]any{
			"id": a.ID, "severity": a.Severity, "title": truncateStr(a.Title, 200),
		})
		if len(pkg.AlertsSample) >= 15 {
			break
		}
	}

	recs, _ := q.ListOpenRecommendationsBySellerAccountID(ctx, dbgen.ListOpenRecommendationsBySellerAccountIDParams{SellerAccountID: sellerID, Limit: 80, Offset: 0})
	for _, r := range recs {
		if !r.EntitySku.Valid {
			continue
		}
		if _, ok := skuSet[r.EntitySku.Int64]; !ok {
			continue
		}
		pkg.RecommendationsSample = append(pkg.RecommendationsSample, map[string]any{
			"id": r.ID, "title": truncateStr(r.Title, 200), "priority": r.PriorityLevel,
		})
		if len(pkg.RecommendationsSample) >= 15 {
			break
		}
	}

	raw, _ := json.Marshal(pkg)
	if len(raw) > maxCategoryContextBytes {
		pkg.SKUSlice["top_by_revenue"] = clampMapSlice(skuRowsToMaps(topByRev), 6)
		pkg.SKUSlice["no_sales_but_in_stock"] = clampMapSlice(skuRowsToMaps(withStockNoSales), 6)
		pkg.DataQualityNotes = append(pkg.DataQualityNotes, "category_context_truncated")
	}
	return pkg
}

type funnelStageSpec struct {
	key   string
	label string
	get   func(row dbgen.ProductAnalyticsDaily) *int64
}

var accountFunnelStageSpecs = []funnelStageSpec{
	{
		key:   "impressions",
		label: "Показы",
		get: func(row dbgen.ProductAnalyticsDaily) *int64 {
			return pgInt8Ptr(row.Impressions)
		},
	},
	{
		key:   "product_views",
		label: "Просмотры карточки",
		get: func(row dbgen.ProductAnalyticsDaily) *int64 {
			return pgInt8Ptr(row.ProductViews)
		},
	},
	{
		key:   "add_to_cart",
		label: "Добавления в корзину",
		get: func(row dbgen.ProductAnalyticsDaily) *int64 {
			return pgInt8Ptr(row.AddToCart)
		},
	},
	{
		key:   "orders",
		label: "Заказанные единицы",
		get: func(row dbgen.ProductAnalyticsDaily) *int64 {
			return pgInt8Ptr(row.OrdersCount)
		},
	},
	{
		key:   "delivered_units",
		label: "Доставленные единицы",
		get: func(row dbgen.ProductAnalyticsDaily) *int64 {
			return pgInt8Ptr(row.CompletedOrders)
		},
	},
}

func buildFunnelContext(
	ctx context.Context,
	q *dbgen.Queries,
	sellerID int64,
	asOf time.Time,
	dto analytics.DashboardMetricsDTO,
	accRows []dbgen.DailyAccountMetric,
	openAlerts []dbgen.Alert,
	recs []dbgen.Recommendation,
	adCamp []dbgen.ListAdCampaignSummariesBySellerAndDateRangeRow,
) map[string]any {
	adLinks, _ := q.ListAdCampaignSKUMappingsBySellerAccountID(ctx, sellerID)
	currentRow, currentStatus := loadAccountFunnelRow(ctx, q, sellerID, asOf)
	prevDate := asOf.AddDate(0, 0, -1)
	prevRow, prevStatus := loadAccountFunnelRow(ctx, q, sellerID, prevDate)

	currentStages := buildAccountFunnelStageMaps(currentRow, prevRow)
	prevStages := buildAccountFunnelStageMaps(prevRow, nil)

	historyRows, err := q.ListProductAnalyticsDailyBySellerAndDateRange(ctx, dbgen.ListProductAnalyticsDailyBySellerAndDateRangeParams{
		SellerAccountID: sellerID,
		MetricDate:      pgDateUTC(asOf.AddDate(0, 0, -6)),
		MetricDate_2:    pgDateUTC(asOf),
	})
	historyStatus := "available"
	if err != nil {
		historyStatus = "not_available"
	}
	accountHistory := filterAccountFunnelRows(historyRows)

	return map[string]any{
		"current_funnel": map[string]any{
			"metric_date":   asOf.Format("2006-01-02"),
			"status":        currentStatus,
			"source_status": funnelRowSourceStatus(currentRow),
			"stages":        currentStages,
			"conversions":   buildFunnelTransitionsFromStages(currentStages),
		},
		"previous_day_funnel": map[string]any{
			"metric_date":   prevDate.Format("2006-01-02"),
			"status":        prevStatus,
			"source_status": funnelRowSourceStatus(prevRow),
			"stages":        prevStages,
			"conversions":   buildFunnelTransitionsFromStages(prevStages),
		},
		"last_7d_context":           buildFunnel7DContext(asOf, currentStages, accountHistory, historyStatus),
		"comparison_buckets":        buildFunnelComparisonBuckets(currentStages, prevStages, accountHistory, dto, accRows),
		"related_account_context":   buildFunnelRelatedAccountContext(dto, accRows),
		"related_ads_context":       buildFunnelRelatedAdsContext(adCamp, adLinks),
		"related_stock_sku_context": buildFunnelRelatedSKUContext(dto.SKUs),
		"related_risks_context":     buildFunnelRelatedRisksContext(ctx, q, sellerID, openAlerts, recs),
	}
}

func buildFunnelComparisonBuckets(
	currentStages []map[string]any,
	previousStages []map[string]any,
	historyRows []dbgen.ProductAnalyticsDaily,
	dto analytics.DashboardMetricsDTO,
	accRows []dbgen.DailyAccountMetric,
) map[string]any {
	stageMetrics := make([]map[string]any, 0, len(currentStages))
	for _, stage := range currentStages {
		stageMetrics = append(stageMetrics, map[string]any{
			"key":           stage["stage_key"],
			"label":         stage["stage_label"],
			"value":         stage["value"],
			"delta_percent": stage["change_vs_previous_day_pct"],
		})
	}

	conversionMetrics := buildFunnelTransitionsFromStages(currentStages)
	for i := range conversionMetrics {
		conversionMetrics[i]["delta_pp"] = nil
	}
	if len(previousStages) > 1 {
		prevTransitions := buildFunnelTransitionsFromStages(previousStages)
		prevByKey := make(map[string]float64, len(prevTransitions))
		for _, tr := range prevTransitions {
			key := fmt.Sprintf("%v_to_%v", tr["from_stage_key"], tr["to_stage_key"])
			if v, ok := tr["conversion_pct"].(float64); ok {
				prevByKey[key] = v
			}
		}
		for i := range conversionMetrics {
			key := fmt.Sprintf("%v_to_%v", conversionMetrics[i]["from_stage_key"], conversionMetrics[i]["to_stage_key"])
			if cur, ok := conversionMetrics[i]["conversion_pct"].(float64); ok {
				if prev, ok := prevByKey[key]; ok {
					conversionMetrics[i]["delta_pp"] = round2(cur - prev)
				}
			}
		}
	}

	relatedAccount := buildFunnelRelatedAccountContext(dto, accRows)
	businessMetrics := []map[string]any{
		{
			"key":           "revenue",
			"label":         "Выручка",
			"value":         valueFromNested(relatedAccount, "revenue", "today"),
			"delta_percent": valueFromNested(relatedAccount, "revenue", "current_vs_7d_pct_diff"),
		},
		{
			"key":           "average_order_value",
			"label":         "Средний чек",
			"value":         valueFromNested(relatedAccount, "aov", "today"),
			"delta_percent": nil,
		},
	}

	sampleDays := len(filterAccountFunnelRows(historyRows))
	contextSignals := []map[string]any{
		{"key": "sample_days", "label": "Дней в истории воронки", "value": sampleDays},
		{"key": "history_status", "label": "Статус истории", "value": "available"},
	}

	return map[string]any{
		"stage_metrics":      stageMetrics,
		"conversion_metrics": conversionMetrics,
		"business_metrics":   businessMetrics,
		"context_signals":    contextSignals,
	}
}

func valueFromNested(root map[string]any, top string, nested string) any {
	obj, ok := root[top].(map[string]any)
	if !ok {
		return nil
	}
	return obj[nested]
}

func loadAccountFunnelRow(ctx context.Context, q *dbgen.Queries, sellerID int64, metricDate time.Time) (*dbgen.ProductAnalyticsDaily, string) {
	row, err := q.GetProductAnalyticsDailyBySellerDateScope(ctx, dbgen.GetProductAnalyticsDailyBySellerDateScopeParams{
		SellerAccountID: sellerID,
		MetricDate:      pgDateUTC(metricDate),
		ScopeType:       "account",
		ScopeID:         "",
	})
	if err != nil {
		return nil, "not_available"
	}
	if row.SourceStatus.Valid && row.SourceStatus.String != "" {
		return &row, row.SourceStatus.String
	}
	return &row, "available"
}

func filterAccountFunnelRows(rows []dbgen.ProductAnalyticsDaily) []dbgen.ProductAnalyticsDaily {
	out := make([]dbgen.ProductAnalyticsDaily, 0, len(rows))
	for _, row := range rows {
		if row.ScopeType != "account" || row.ScopeID != "" {
			continue
		}
		if row.SourceStatus.Valid && row.SourceStatus.String != "" && row.SourceStatus.String != "available" {
			continue
		}
		out = append(out, row)
	}
	return out
}

func buildAccountFunnelStageMaps(current *dbgen.ProductAnalyticsDaily, previous *dbgen.ProductAnalyticsDaily) []map[string]any {
	if current == nil {
		return []map[string]any{}
	}

	stages := make([]map[string]any, 0, len(accountFunnelStageSpecs))
	var topValue *int64
	var prevStageValue *int64
	for _, spec := range accountFunnelStageSpecs {
		value := spec.get(*current)
		if value == nil {
			continue
		}
		prevDayValue := int64ForFunnelStage(previous, spec.key)

		stage := map[string]any{
			"stage_key":                  spec.key,
			"stage_label":                spec.label,
			"value":                      *value,
			"conversion_from_top":        nil,
			"conversion_from_previous":   nil,
			"previous_day_value":         nil,
			"change_vs_previous_day_pct": nil,
		}
		if prevDayValue != nil {
			stage["previous_day_value"] = *prevDayValue
			stage["change_vs_previous_day_pct"] = percentDiffInt64(*value, *prevDayValue)
		}
		if topValue == nil {
			topValue = value
		}
		if topValue != nil && *topValue > 0 {
			stage["conversion_from_top"] = round2(float64(*value) / float64(*topValue) * 100)
		}
		if prevStageValue != nil && *prevStageValue > 0 {
			stage["conversion_from_previous"] = round2(float64(*value) / float64(*prevStageValue) * 100)
		}

		stages = append(stages, stage)
		prevStageValue = value
	}
	return stages
}

func buildFunnelTransitionsFromStages(stages []map[string]any) []map[string]any {
	if len(stages) < 2 {
		return []map[string]any{}
	}
	out := make([]map[string]any, 0, len(stages)-1)
	for i := 1; i < len(stages); i++ {
		conv, _ := stages[i]["conversion_from_previous"].(float64)
		out = append(out, map[string]any{
			"from_stage_key":   stages[i-1]["stage_key"],
			"from_stage_label": stages[i-1]["stage_label"],
			"to_stage_key":     stages[i]["stage_key"],
			"to_stage_label":   stages[i]["stage_label"],
			"conversion_pct":   conv,
		})
	}
	return out
}

func buildFunnel7DContext(
	asOf time.Time,
	currentStages []map[string]any,
	rows []dbgen.ProductAnalyticsDaily,
	status string,
) map[string]any {
	stageAverages := make([]map[string]any, 0, len(accountFunnelStageSpecs))
	transitionAverages := make([]map[string]any, 0, len(accountFunnelStageSpecs)-1)
	if len(rows) == 0 {
		return map[string]any{
			"status":              status,
			"date_range":          map[string]any{"from": asOf.AddDate(0, 0, -6).Format("2006-01-02"), "to": asOf.Format("2006-01-02")},
			"sample_days":         0,
			"stage_averages":      stageAverages,
			"transition_averages": transitionAverages,
		}
	}

	stageAverageByKey := make(map[string]float64, len(accountFunnelStageSpecs))
	for _, spec := range accountFunnelStageSpecs {
		total := 0.0
		count := 0
		for _, row := range rows {
			if value := spec.get(row); value != nil {
				total += float64(*value)
				count++
			}
		}
		if count > 0 {
			stageAverageByKey[spec.key] = total / float64(count)
		}
	}

	currentByKey := make(map[string]map[string]any, len(currentStages))
	for _, stage := range currentStages {
		if key, ok := stage["stage_key"].(string); ok {
			currentByKey[key] = stage
		}
	}

	for _, spec := range accountFunnelStageSpecs {
		avg, ok := stageAverageByKey[spec.key]
		if !ok {
			continue
		}
		currentValue, currentValueOK := optionalStageValue(currentByKey[spec.key], "value")
		stageAverages = append(stageAverages, map[string]any{
			"stage_key":              spec.key,
			"stage_label":            spec.label,
			"average_value":          round2(avg),
			"current_value":          nullableRounded(currentValue, currentValueOK),
			"current_vs_7d_abs_diff": diffOptional(currentValue, currentValueOK, avg),
			"current_vs_7d_pct_diff": percentDiffOptional(currentValue, currentValueOK, avg),
		})
	}

	for i := 1; i < len(accountFunnelStageSpecs); i++ {
		from := accountFunnelStageSpecs[i-1]
		to := accountFunnelStageSpecs[i]
		total := 0.0
		count := 0
		for _, row := range rows {
			fromValue := from.get(row)
			toValue := to.get(row)
			if fromValue == nil || toValue == nil || *fromValue <= 0 {
				continue
			}
			total += float64(*toValue) / float64(*fromValue) * 100
			count++
		}
		if count == 0 {
			continue
		}
		avgConv := total / float64(count)
		currentConv, currentConvOK := optionalStageValue(currentByKey[to.key], "conversion_from_previous")
		transitionAverages = append(transitionAverages, map[string]any{
			"from_stage_key":         from.key,
			"from_stage_label":       from.label,
			"to_stage_key":           to.key,
			"to_stage_label":         to.label,
			"average_conversion_pct": round2(avgConv),
			"current_conversion_pct": nullableRounded(currentConv, currentConvOK),
			"conversion_diff_pp":     diffOptional(currentConv, currentConvOK, avgConv),
		})
	}

	return map[string]any{
		"status":              status,
		"date_range":          map[string]any{"from": asOf.AddDate(0, 0, -6).Format("2006-01-02"), "to": asOf.Format("2006-01-02")},
		"sample_days":         len(rows),
		"stage_averages":      stageAverages,
		"transition_averages": transitionAverages,
	}
}

func buildFunnelRelatedAccountContext(dto analytics.DashboardMetricsDTO, rows []dbgen.DailyAccountMetric) map[string]any {
	todayKey := dto.AsOfDate
	yesterdayKey := ""
	if todayKey != "" {
		if parsed, err := time.Parse("2006-01-02", todayKey); err == nil {
			yesterdayKey = parsed.AddDate(0, 0, -1).Format("2006-01-02")
		}
	}

	var todayRow *dbgen.DailyAccountMetric
	var yesterdayRow *dbgen.DailyAccountMetric
	revenueSum := 0.0
	ordersSum := 0.0
	cancelsSum := 0.0
	returnsSum := 0.0
	aovSum := 0.0
	aovCount := 0.0

	for i := range rows {
		rowDate := rows[i].MetricDate.Time.Format("2006-01-02")
		revenue := numericToFloat(rows[i].Revenue)
		revenueSum += revenue
		ordersSum += float64(rows[i].OrdersCount)
		cancelsSum += float64(rows[i].CancelCount)
		returnsSum += float64(rows[i].ReturnsCount)
		if rows[i].OrdersCount > 0 {
			aovSum += revenue / float64(rows[i].OrdersCount)
			aovCount++
		}
		switch rowDate {
		case todayKey:
			todayRow = &rows[i]
		case yesterdayKey:
			yesterdayRow = &rows[i]
		}
	}

	avgRevenue := 0.0
	avgOrders := 0.0
	avgCancels := 0.0
	avgReturns := 0.0
	avgAOV := 0.0
	if len(rows) > 0 {
		avgRevenue = revenueSum / float64(len(rows))
		avgOrders = ordersSum / float64(len(rows))
		avgCancels = cancelsSum / float64(len(rows))
		avgReturns = returnsSum / float64(len(rows))
	}
	if aovCount > 0 {
		avgAOV = aovSum / aovCount
	}

	revenueToday := dto.Account.RevenueToday
	if todayRow != nil {
		revenueToday = numericToFloat(todayRow.Revenue)
	}
	revenueYesterday := dto.Account.RevenueYesterday
	if yesterdayRow != nil {
		revenueYesterday = numericToFloat(yesterdayRow.Revenue)
	}
	ordersToday := float64(dto.Account.OrdersToday)
	if todayRow != nil {
		ordersToday = float64(todayRow.OrdersCount)
	}
	ordersYesterday := float64(dto.Account.OrdersYesterday)
	if yesterdayRow != nil {
		ordersYesterday = float64(yesterdayRow.OrdersCount)
	}
	cancelsToday := float64(dto.Account.CancelsToday)
	if todayRow != nil {
		cancelsToday = float64(todayRow.CancelCount)
	}
	cancelsYesterday := 0.0
	if yesterdayRow != nil {
		cancelsYesterday = float64(yesterdayRow.CancelCount)
	}
	returnsToday := float64(dto.Account.ReturnsToday)
	if todayRow != nil {
		returnsToday = float64(todayRow.ReturnsCount)
	}
	returnsYesterday := 0.0
	if yesterdayRow != nil {
		returnsYesterday = float64(yesterdayRow.ReturnsCount)
	}

	var aovToday any
	if dto.Account.AverageOrderValue != nil {
		aovToday = round2(*dto.Account.AverageOrderValue)
	}
	var aovYesterday any
	if revenueYesterday > 0 && ordersYesterday > 0 {
		aovYesterday = round2(revenueYesterday / ordersYesterday)
	}
	var drrPct any
	if dto.Account.AccountAdSpendToRevenue != nil {
		drrPct = round2(*dto.Account.AccountAdSpendToRevenue * 100)
	}
	var cancellationRatePct any
	if dto.Account.CancellationRate != nil {
		cancellationRatePct = round2(*dto.Account.CancellationRate * 100)
	}

	return map[string]any{
		"status": "available",
		"revenue": map[string]any{
			"today":                  round2(revenueToday),
			"yesterday":              round2(revenueYesterday),
			"average_last_7d":        round2(avgRevenue),
			"current_vs_7d_abs_diff": diffFloat(revenueToday, avgRevenue),
			"current_vs_7d_pct_diff": percentDiffFloat(revenueToday, avgRevenue),
		},
		"orders": map[string]any{
			"today":                  int64(math.Round(ordersToday)),
			"yesterday":              int64(math.Round(ordersYesterday)),
			"average_last_7d":        round2(avgOrders),
			"current_vs_7d_abs_diff": diffFloat(ordersToday, avgOrders),
			"current_vs_7d_pct_diff": percentDiffFloat(ordersToday, avgOrders),
		},
		"aov": map[string]any{
			"today":           aovToday,
			"yesterday":       aovYesterday,
			"average_last_7d": nullableRounded(avgAOV, aovCount > 0),
		},
		"drr_pct": map[string]any{
			"today": drrPct,
		},
		"cancellations": map[string]any{
			"today":           int64(math.Round(cancelsToday)),
			"yesterday":       int64(math.Round(cancelsYesterday)),
			"average_last_7d": round2(avgCancels),
		},
		"cancellation_rate_pct": map[string]any{
			"today": cancellationRatePct,
		},
		"returns": map[string]any{
			"today":           int64(math.Round(returnsToday)),
			"yesterday":       int64(math.Round(returnsYesterday)),
			"average_last_7d": round2(avgReturns),
		},
	}
}

func buildFunnelRelatedAdsContext(rows []dbgen.ListAdCampaignSummariesBySellerAndDateRangeRow, links []dbgen.ListAdCampaignSKUMappingsBySellerAccountIDRow) map[string]any {
	if len(rows) == 0 {
		return map[string]any{
			"status":                           "not_available",
			"weak_campaigns":                   []map[string]any{},
			"campaigns_with_spend_zero_orders": []map[string]any{},
			"campaign_sku_links":               []map[string]any{},
		}
	}

	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		spend := numericToFloat(row.SpendTotal)
		revenue := numericToFloat(row.RevenueTotal)
		roas := 0.0
		if spend > 0 {
			roas = revenue / spend
		}
		drr := 0.0
		if revenue > 0 {
			drr = spend / revenue * 100
		}
		items = append(items, map[string]any{
			"campaign_external_id": row.CampaignExternalID,
			"campaign_name":        row.CampaignName,
			"spend":                round2(spend),
			"revenue":              round2(revenue),
			"orders":               row.OrdersTotal,
			"drr_pct":              nullableRounded(drr, revenue > 0),
			"roas":                 nullableRounded(roas, spend > 0),
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		iSpend := valueFromMap(items[i], "spend")
		jSpend := valueFromMap(items[j], "spend")
		iOrders := int64ValueFromMap(items[i], "orders")
		jOrders := int64ValueFromMap(items[j], "orders")
		if (iOrders == 0) != (jOrders == 0) {
			return iOrders == 0
		}
		iDrr := valueFromMap(items[i], "drr_pct")
		jDrr := valueFromMap(items[j], "drr_pct")
		if iDrr != jDrr {
			return iDrr > jDrr
		}
		iRoas := valueFromMap(items[i], "roas")
		jRoas := valueFromMap(items[j], "roas")
		if iRoas != jRoas {
			return iRoas < jRoas
		}
		return iSpend > jSpend
	})

	type campaignPriority struct {
		riskScore float64
		spend     float64
	}
	priorityByCampaign := make(map[int64]campaignPriority, len(items))
	for idx, item := range items {
		id := int64ValueFromMap(item, "campaign_external_id")
		if id == 0 {
			continue
		}
		spend := valueFromMap(item, "spend")
		orders := int64ValueFromMap(item, "orders")
		drr := valueFromMap(item, "drr_pct")
		roas := valueFromMap(item, "roas")
		// Higher score means higher priority for AI context:
		// high-spend weak campaigns first, then spend-heavy.
		riskScore := 0.0
		if spend > 0 && orders == 0 {
			riskScore += 200
		}
		riskScore += drr
		if spend > 0 && roas > 0 {
			riskScore += (1 / roas) * 25
		}
		riskScore += float64(len(items)-idx) * 5
		priorityByCampaign[id] = campaignPriority{riskScore: riskScore, spend: spend}
	}

	zeroOrders := make([]map[string]any, 0, maxFunnelSignalItems)
	for _, item := range items {
		if valueFromMap(item, "spend") <= 0 || int64ValueFromMap(item, "orders") > 0 {
			continue
		}
		zeroOrders = append(zeroOrders, item)
		if len(zeroOrders) >= maxFunnelSignalItems {
			break
		}
	}

	sortedLinks := append([]dbgen.ListAdCampaignSKUMappingsBySellerAccountIDRow(nil), links...)
	sort.SliceStable(sortedLinks, func(i, j int) bool {
		li := sortedLinks[i]
		lj := sortedLinks[j]
		if li.IsActive != lj.IsActive {
			return li.IsActive
		}
		pi := priorityByCampaign[li.CampaignExternalID]
		pj := priorityByCampaign[lj.CampaignExternalID]
		if pi.riskScore != pj.riskScore {
			return pi.riskScore > pj.riskScore
		}
		if pi.spend != pj.spend {
			return pi.spend > pj.spend
		}
		if li.CampaignExternalID != lj.CampaignExternalID {
			return li.CampaignExternalID < lj.CampaignExternalID
		}
		return li.OzonProductID < lj.OzonProductID
	})

	linkItems := make([]map[string]any, 0, maxFunnelSignalItems)
	for _, link := range sortedLinks {
		linkItems = append(linkItems, map[string]any{
			"campaign_external_id": link.CampaignExternalID,
			"campaign_name":        link.CampaignName,
			"ozon_product_id":      link.OzonProductID,
			"sku":                  nullableInt64(link.Sku),
			"offer_id":             textFromPg(link.OfferID),
			"is_active":            link.IsActive,
			"source_method":        "performance.campaign_products_or_objects",
		})
		if len(linkItems) >= maxFunnelSignalItems {
			break
		}
	}

	return map[string]any{
		"status":                           "available",
		"weak_campaigns":                   clampMapSlice(items, maxFunnelSignalItems),
		"campaigns_with_spend_zero_orders": zeroOrders,
		"campaign_sku_links":               linkItems,
	}
}

func textFromPg(v pgtype.Text) any {
	if !v.Valid || strings.TrimSpace(v.String) == "" {
		return nil
	}
	return v.String
}

func buildFunnelRelatedSKUContext(items []analytics.DashboardSKUMetricsItem) map[string]any {
	if len(items) == 0 {
		return map[string]any{
			"status":                "not_available",
			"stock_risk_skus":       []map[string]any{},
			"out_of_stock_key_skus": []map[string]any{},
			"no_sales_with_stock":   []map[string]any{},
			"order_drop_skus":       []map[string]any{},
			"key_sku_problems":      []map[string]any{},
		}
	}

	stockRisk := make([]map[string]any, 0)
	outOfStock := make([]map[string]any, 0)
	noSales := make([]map[string]any, 0)
	orderDrop := make([]map[string]any, 0)
	keyProblems := make([]map[string]any, 0)

	for _, item := range items {
		entry := map[string]any{
			"offer_id":             item.OfferID,
			"product_name":         item.ProductName,
			"stock":                item.StockAvailable,
			"days_of_cover":        item.DaysOfCover,
			"recent_orders":        item.OrdersCount,
			"orders_delta":         item.OrdersDeltaDayToDay,
			"share_of_revenue_pct": nullableRounded(percentValue(item.ShareOfRevenue), item.ShareOfRevenue != nil),
		}
		if isStockRiskSKU(item) {
			stockRisk = append(stockRisk, entry)
		}
		if item.StockAvailable <= 0 && isKeySKU(item) {
			outOfStock = append(outOfStock, entry)
		}
		if item.StockAvailable > 0 && item.OrdersCount == 0 {
			noSales = append(noSales, entry)
		}
		if item.OrdersDeltaDayToDay < 0 {
			orderDrop = append(orderDrop, entry)
		}
		if isKeySKU(item) && (isStockRiskSKU(item) || item.OrdersDeltaDayToDay < 0 || (item.StockAvailable > 0 && item.OrdersCount == 0)) {
			keyProblems = append(keyProblems, entry)
		}
	}

	sortSKUContextItems(stockRisk)
	sortSKUContextItems(outOfStock)
	sortSKUContextItems(noSales)
	sortSKUContextItems(orderDrop)
	sortSKUContextItems(keyProblems)

	return map[string]any{
		"status":                "available",
		"stock_risk_skus":       clampMapSlice(stockRisk, maxFunnelSignalItems),
		"out_of_stock_key_skus": clampMapSlice(outOfStock, maxFunnelSignalItems),
		"no_sales_with_stock":   clampMapSlice(noSales, maxFunnelSignalItems),
		"order_drop_skus":       clampMapSlice(orderDrop, maxFunnelSignalItems),
		"key_sku_problems":      clampMapSlice(keyProblems, maxFunnelSignalItems),
	}
}

func buildFunnelRelatedRisksContext(
	ctx context.Context,
	q *dbgen.Queries,
	sellerID int64,
	openAlerts []dbgen.Alert,
	recs []dbgen.Recommendation,
) map[string]any {
	alertCountsByGroup, _ := q.CountOpenAlertsByGroup(ctx, sellerID)
	alertCountsBySeverity, _ := q.CountOpenAlertsBySeverity(ctx, sellerID)
	recommendationCountsByPriority, _ := q.CountOpenRecommendationsByPriority(ctx, sellerID)
	return composeFunnelRelatedRisksContext(alertCountsByGroup, alertCountsBySeverity, recommendationCountsByPriority, openAlerts, recs)
}

func composeFunnelRelatedRisksContext(
	alertCountsByGroup []dbgen.CountOpenAlertsByGroupRow,
	alertCountsBySeverity []dbgen.CountOpenAlertsBySeverityRow,
	recommendationCountsByPriority []dbgen.CountOpenRecommendationsByPriorityRow,
	openAlerts []dbgen.Alert,
	recs []dbgen.Recommendation,
) map[string]any {
	groupCounts := make([]map[string]any, 0, len(alertCountsByGroup))
	for _, row := range alertCountsByGroup {
		groupCounts = append(groupCounts, map[string]any{
			"group": row.AlertGroup,
			"count": row.AlertsCount,
		})
	}

	severityCounts := make([]map[string]any, 0, len(alertCountsBySeverity))
	for _, row := range alertCountsBySeverity {
		severityCounts = append(severityCounts, map[string]any{
			"severity": row.Severity,
			"count":    row.AlertsCount,
		})
	}

	priorityCounts := make([]map[string]any, 0, len(recommendationCountsByPriority))
	for _, row := range recommendationCountsByPriority {
		priorityCounts = append(priorityCounts, map[string]any{
			"priority": row.PriorityLevel,
			"count":    row.RecommendationsCount,
		})
	}

	sort.SliceStable(openAlerts, func(i, j int) bool {
		ri := alertSeverityRank(openAlerts[i].Severity)
		rj := alertSeverityRank(openAlerts[j].Severity)
		if ri != rj {
			return ri < rj
		}
		return openAlerts[i].ID < openAlerts[j].ID
	})
	alertSample := make([]map[string]any, 0, maxFunnelSignalItems)
	for _, alert := range openAlerts {
		alertSample = append(alertSample, map[string]any{
			"id":       alert.ID,
			"severity": alert.Severity,
			"group":    alert.AlertGroup,
			"title":    truncateStr(alert.Title, 160),
			"message":  truncateStr(alert.Message, 220),
		})
		if len(alertSample) >= maxFunnelSignalItems {
			break
		}
	}

	sort.SliceStable(recs, func(i, j int) bool {
		ri := recommendationPriorityRank(recs[i].PriorityLevel)
		rj := recommendationPriorityRank(recs[j].PriorityLevel)
		if ri != rj {
			return ri < rj
		}
		return recs[i].ID < recs[j].ID
	})
	recommendedActions := make([]map[string]any, 0, maxFunnelSignalItems)
	for _, rec := range recs {
		recommendedActions = append(recommendedActions, map[string]any{
			"id":       rec.ID,
			"priority": rec.PriorityLevel,
			"title":    truncateStr(rec.Title, 160),
			"action":   truncateStr(rec.RecommendedAction, 220),
		})
		if len(recommendedActions) >= maxFunnelSignalItems {
			break
		}
	}

	return map[string]any{
		"status":                           "available",
		"open_alert_counts_by_group":       groupCounts,
		"open_alert_counts_by_severity":    severityCounts,
		"top_open_alerts":                  alertSample,
		"open_recommendations_by_priority": priorityCounts,
		"top_recommended_actions":          recommendedActions,
	}
}

func funnelRowSourceStatus(row *dbgen.ProductAnalyticsDaily) string {
	if row == nil || !row.SourceStatus.Valid || row.SourceStatus.String == "" {
		return "available"
	}
	return row.SourceStatus.String
}

func int64ForFunnelStage(row *dbgen.ProductAnalyticsDaily, key string) *int64 {
	if row == nil {
		return nil
	}
	for _, spec := range accountFunnelStageSpecs {
		if spec.key == key {
			return spec.get(*row)
		}
	}
	return nil
}

func percentDiffInt64(current, baseline int64) any {
	if baseline == 0 {
		return nil
	}
	return round2((float64(current) - float64(baseline)) / float64(baseline) * 100)
}

func percentDiffFloat(current, baseline float64) any {
	if baseline == 0 {
		return nil
	}
	return round2((current - baseline) / baseline * 100)
}

func diffFloat(current, baseline float64) any {
	return round2(current - baseline)
}

func diffOptional(current float64, ok bool, baseline float64) any {
	if !ok {
		return nil
	}
	return diffFloat(current, baseline)
}

func percentDiffOptional(current float64, ok bool, baseline float64) any {
	if !ok {
		return nil
	}
	return percentDiffFloat(current, baseline)
}

func optionalStageValue(stage map[string]any, key string) (float64, bool) {
	if stage == nil {
		return 0, false
	}
	switch v := stage[key].(type) {
	case float64:
		return v, true
	case int64:
		return float64(v), true
	case int:
		return float64(v), true
	default:
		return 0, false
	}
}

func valueFromMap(item map[string]any, key string) float64 {
	switch v := item[key].(type) {
	case float64:
		return v
	case int64:
		return float64(v)
	case int:
		return float64(v)
	default:
		return 0
	}
}

func int64ValueFromMap(item map[string]any, key string) int64 {
	switch v := item[key].(type) {
	case int64:
		return v
	case int32:
		return int64(v)
	case int:
		return int64(v)
	default:
		return 0
	}
}

func nullableRounded(value float64, ok bool) any {
	if !ok {
		return nil
	}
	return round2(value)
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func percentValue(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v * 100
}

func isStockRiskSKU(item analytics.DashboardSKUMetricsItem) bool {
	if item.StockAvailable <= 0 {
		return true
	}
	if item.DaysOfCover != nil && *item.DaysOfCover <= 7 {
		return true
	}
	if item.StockStatusBucket != nil {
		switch *item.StockStatusBucket {
		case "critical_shortage", "out_of_stock", "shortage_7d":
			return true
		}
	}
	return false
}

func isKeySKU(item analytics.DashboardSKUMetricsItem) bool {
	return item.Revenue > 0 || (item.ShareOfRevenue != nil && *item.ShareOfRevenue >= 0.05)
}

func sortSKUContextItems(items []map[string]any) {
	sort.SliceStable(items, func(i, j int) bool {
		iShare := valueFromMap(items[i], "share_of_revenue_pct")
		jShare := valueFromMap(items[j], "share_of_revenue_pct")
		if iShare != jShare {
			return iShare > jShare
		}
		iDelta := int64ValueFromMap(items[i], "orders_delta")
		jDelta := int64ValueFromMap(items[j], "orders_delta")
		if iDelta != jDelta {
			return iDelta < jDelta
		}
		return valueFromMap(items[i], "stock") > valueFromMap(items[j], "stock")
	})
}

func alertSeverityRank(severity string) int {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "critical":
		return 0
	case "high":
		return 1
	case "medium":
		return 2
	default:
		return 3
	}
}

func recommendationPriorityRank(priority string) int {
	switch strings.ToLower(strings.TrimSpace(priority)) {
	case "critical":
		return 0
	case "high":
		return 1
	case "medium":
		return 2
	default:
		return 3
	}
}

func syncDashboardAccountOrdersFromMetrics(dto *analytics.DashboardMetricsDTO, rows []dbgen.DailyAccountMetric, metricDate time.Time) {
	if dto == nil || len(rows) == 0 {
		return
	}

	todayKey := metricDate.UTC().Format("2006-01-02")
	yesterdayKey := metricDate.UTC().AddDate(0, 0, -1).Format("2006-01-02")

	var todayRow *dbgen.DailyAccountMetric
	var yesterdayRow *dbgen.DailyAccountMetric
	for i := range rows {
		key := rows[i].MetricDate.Time.Format("2006-01-02")
		switch key {
		case todayKey:
			todayRow = &rows[i]
		case yesterdayKey:
			yesterdayRow = &rows[i]
		}
	}

	if todayRow != nil {
		dto.Account.OrdersToday = todayRow.OrdersCount
		dto.Account.AverageOrderValue = analytics.RatioOrNil(dto.Account.RevenueToday, float64(todayRow.OrdersCount))
		dto.Account.CancellationRate = analytics.CancellationRate(todayRow.CancelCount, todayRow.OrdersCount)
	}
	if yesterdayRow != nil {
		dto.Account.OrdersYesterday = yesterdayRow.OrdersCount
	}
}

func classifyAccountDataRichness(dto analytics.DashboardMetricsDTO) string {
	if dto.Account.RevenueToday == 0 && dto.Account.OrdersToday == 0 && len(dto.SKUs) == 0 {
		return "low"
	}
	if dto.Account.OrdersToday == 0 && dto.Account.RevenueToday == 0 {
		return "low"
	}
	if len(dto.SKUs) < 5 {
		return "medium"
	}
	return "high"
}

func classifyCategoryDataRichness(skuCount int, revenue float64, alertsN, recsN int, totalStock int32) string {
	// Tight "low" band: sparse assortment, no commercial motion, no open pressure.
	if skuCount == 1 && totalStock < 50 && revenue == 0 && alertsN == 0 && recsN == 0 {
		return "low"
	}
	if skuCount <= 2 && revenue == 0 && alertsN == 0 && recsN == 0 {
		return "low"
	}
	if skuCount <= 5 && revenue == 0 && alertsN == 0 && recsN == 0 {
		return "low"
	}
	if skuCount <= 8 {
		return "medium"
	}
	return "high"
}

func dashboardDTOToCompactMap(d analytics.DashboardMetricsDTO) map[string]any {
	return map[string]any{
		"as_of_date": d.AsOfDate,
		"account":    d.Account,
		"summary":    d.Summary,
	}
}

func buildAccountAggregates(dto analytics.DashboardMetricsDTO, accRows []dbgen.DailyAccountMetric, alertsRows []dbgen.Alert, recRows []dbgen.Recommendation) map[string]any {
	total := len(dto.SKUs)
	active := 0
	withOrders := 0
	withoutSales := 0
	stockRisk := 0
	orderDrop := 0
	overstock := 0
	adRisk := 0
	for _, item := range dto.SKUs {
		active++
		if item.OrdersCount > 0 {
			withOrders++
		} else {
			withoutSales++
		}
		if isStockRiskSKU(item) {
			stockRisk++
		}
		if item.OrdersDeltaDayToDay < 0 {
			orderDrop++
		}
		if (item.DaysWithoutSales != nil && *item.DaysWithoutSales >= 14) || (item.ExcessStockShare != nil && *item.ExcessStockShare > 0.25) {
			overstock++
		}
		if item.OzonADS != nil && *item.OzonADS > 0.35 {
			adRisk++
		}
	}
	return map[string]any{
		"total_sku_count":            total,
		"active_sku_count":           active,
		"sku_with_orders_count":      withOrders,
		"sku_without_sales_count":    withoutSales,
		"revenue_today":              dto.Account.RevenueToday,
		"revenue_yesterday":          dto.Account.RevenueYesterday,
		"revenue_last_7d":            dto.Account.RevenueLast7Days,
		"orders_today":               dto.Account.OrdersToday,
		"orders_yesterday":           dto.Account.OrdersYesterday,
		"aov_today":                  dto.Account.AverageOrderValue,
		"stock_risk_count":           stockRisk,
		"order_drop_count":           orderDrop,
		"overstock_count":            overstock,
		"ad_risk_count":              adRisk,
		"category_count":             estimateCategoryCountFromSKUs(dto.SKUs),
		"alert_counts_by_severity":   countAlertsBySeverity(alertsRows),
		"alert_counts_by_group":      countAlertsByGroup(alertsRows),
		"recommendation_by_priority": countRecommendationsByPriority(recRows),
		"account_metrics_days":       len(accRows),
	}
}

func estimateCategoryCountFromSKUs(items []analytics.DashboardSKUMetricsItem) int {
	seen := map[int64]struct{}{}
	for _, item := range items {
		if item.OzonProductID <= 0 {
			continue
		}
		seen[item.OzonProductID/1000] = struct{}{}
	}
	if len(seen) == 0 && len(items) > 0 {
		return 1
	}
	return len(seen)
}

func countAlertsBySeverity(rows []dbgen.Alert) map[string]int {
	out := map[string]int{}
	for _, r := range rows {
		k := strings.ToLower(strings.TrimSpace(r.Severity))
		if k == "" {
			k = "unknown"
		}
		out[k]++
	}
	return out
}

func countAlertsByGroup(rows []dbgen.Alert) map[string]int {
	out := map[string]int{}
	for _, r := range rows {
		k := strings.ToLower(strings.TrimSpace(r.AlertGroup))
		if k == "" {
			k = "unknown"
		}
		out[k]++
	}
	return out
}

func countRecommendationsByPriority(rows []dbgen.Recommendation) map[string]int {
	out := map[string]int{}
	for _, r := range rows {
		k := strings.ToLower(strings.TrimSpace(r.PriorityLevel))
		if k == "" {
			k = "unknown"
		}
		out[k]++
	}
	return out
}

func skuRowsToMaps(rows []dbgen.ListProductSKUMetricsForCategoryAndDateRow) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, s := range rows {
		out = append(out, map[string]any{
			"ozon_product_id": s.OzonProductID,
			"sku":             nullableInt64(s.Sku),
			"name":            truncateStr(s.ProductName, 120),
			"revenue":         numericToFloat(s.Revenue),
			"orders":          s.OrdersCount,
			"stock":           s.StockAvailable,
			"days_of_cover":   numericPtr(s.DaysOfCover),
		})
	}
	return out
}

func nullableInt64(v pgtype.Int8) any {
	if !v.Valid {
		return nil
	}
	return v.Int64
}

func numericPtr(n pgtype.Numeric) any {
	if !n.Valid {
		return nil
	}
	f, _ := n.Float64Value()
	if !f.Valid {
		return nil
	}
	return f.Float64
}

func numericToFloat(n pgtype.Numeric) float64 {
	if !n.Valid {
		return 0
	}
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return 0
	}
	return f.Float64
}

func pgInt8Ptr(v pgtype.Int8) *int64 {
	if !v.Valid {
		return nil
	}
	x := v.Int64
	return &x
}

func intOrZero(v pgtype.Int4) int32 {
	if !v.Valid {
		return 0
	}
	return v.Int32
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func clampSlice[T any](s []T, n int) []T {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func clampMapSlice(s []map[string]any, n int) []map[string]any {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

type accountCandidate struct {
	SKU             int64
	OfferID         string
	ProductName     string
	Revenue7D       float64
	Orders7D        int64
	StockAvailable  int32
	DaysOfCover     any
	RiskTags        []string
	SelectionReason map[string]struct{}
}

func buildAccountSKUCandidates(items []analytics.DashboardSKUMetricsItem, alertsRows []dbgen.Alert, recRows []dbgen.Recommendation, limit int) []map[string]any {
	if limit <= 0 {
		limit = accountContextMaxUniqueCandidates
	}
	bySKU := map[int64]*accountCandidate{}
	ensure := func(item analytics.DashboardSKUMetricsItem, reason string, riskTags ...string) {
		if item.SKU == nil {
			return
		}
		key := *item.SKU
		c := bySKU[key]
		if c == nil {
			c = &accountCandidate{
				SKU:             key,
				Revenue7D:       item.RevenueLast7Days,
				Orders7D:        item.OrdersLast7Days,
				StockAvailable:  item.StockAvailable,
				DaysOfCover:     item.DaysOfCover,
				SelectionReason: map[string]struct{}{},
			}
			if item.OfferID != nil {
				c.OfferID = *item.OfferID
			}
			if item.ProductName != nil {
				c.ProductName = truncateStr(*item.ProductName, 90)
			}
			bySKU[key] = c
		}
		c.SelectionReason[reason] = struct{}{}
		for _, tag := range riskTags {
			if tag != "" {
				c.RiskTags = append(c.RiskTags, tag)
			}
		}
	}

	rev := append([]analytics.DashboardSKUMetricsItem(nil), items...)
	sort.SliceStable(rev, func(i, j int) bool { return rev[i].RevenueLast7Days > rev[j].RevenueLast7Days })
	for i := 0; i < len(rev) && i < 20; i++ {
		ensure(rev[i], "top_revenue_7d")
	}
	drop := append([]analytics.DashboardSKUMetricsItem(nil), items...)
	sort.SliceStable(drop, func(i, j int) bool { return drop[i].OrdersDeltaDayToDay < drop[j].OrdersDeltaDayToDay })
	for i := 0; i < len(drop) && i < 20; i++ {
		ensure(drop[i], "top_order_drop", "order_drop")
	}
	stock := append([]analytics.DashboardSKUMetricsItem(nil), items...)
	sort.SliceStable(stock, func(i, j int) bool {
		di, dj := 9999.0, 9999.0
		if stock[i].DaysOfCover != nil {
			di = *stock[i].DaysOfCover
		}
		if stock[j].DaysOfCover != nil {
			dj = *stock[j].DaysOfCover
		}
		return di < dj
	})
	for i := 0; i < len(stock) && i < 20; i++ {
		if isStockRiskSKU(stock[i]) {
			ensure(stock[i], "top_stock_risk", "stock_risk")
		}
	}
	for _, item := range items {
		if (item.DaysWithoutSales != nil && *item.DaysWithoutSales >= 14) || (item.ExcessStockShare != nil && *item.ExcessStockShare > 0.25) {
			ensure(item, "top_overstock", "overstock")
		}
	}
	seenByPrioritySKU := map[int64]struct{}{}
	for _, r := range recRows {
		if !r.EntitySku.Valid {
			continue
		}
		seenByPrioritySKU[r.EntitySku.Int64] = struct{}{}
	}
	for _, item := range items {
		if item.SKU == nil {
			continue
		}
		if _, ok := seenByPrioritySKU[*item.SKU]; ok {
			ensure(item, "top_recommendation_priority", "recommendation_pressure")
		}
	}
	alertBySKU := map[int64]struct{}{}
	for _, a := range alertsRows {
		if !a.EntitySku.Valid {
			continue
		}
		if alertSeverityRank(a.Severity) <= 1 {
			alertBySKU[a.EntitySku.Int64] = struct{}{}
		}
	}
	for _, item := range items {
		if item.SKU == nil {
			continue
		}
		if _, ok := alertBySKU[*item.SKU]; ok {
			ensure(item, "top_alert_severity", "alert_pressure")
		}
	}

	candidates := make([]*accountCandidate, 0, len(bySKU))
	for _, c := range bySKU {
		candidates = append(candidates, c)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		ri, rj := len(candidates[i].SelectionReason), len(candidates[j].SelectionReason)
		if ri != rj {
			return ri > rj
		}
		if candidates[i].Revenue7D != candidates[j].Revenue7D {
			return candidates[i].Revenue7D > candidates[j].Revenue7D
		}
		return candidates[i].SKU < candidates[j].SKU
	})
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	out := make([]map[string]any, 0, len(candidates))
	for _, c := range candidates {
		reasons := make([]string, 0, len(c.SelectionReason))
		for reason := range c.SelectionReason {
			reasons = append(reasons, reason)
		}
		sort.Strings(reasons)
		out = append(out, map[string]any{
			"sku":               c.SKU,
			"offer_id":          c.OfferID,
			"product_name":      c.ProductName,
			"revenue_7d":        round2(c.Revenue7D),
			"orders_7d":         c.Orders7D,
			"stock_available":   c.StockAvailable,
			"days_of_cover":     c.DaysOfCover,
			"risk_tags":         uniqueCompactStrings(c.RiskTags),
			"selection_reasons": reasons,
		})
	}
	return out
}

func uniqueCompactStrings(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func buildCategoryEvidenceSignals(items []analytics.DashboardSKUMetricsItem, categoryID int64) map[string]any {
	_ = items
	_ = categoryID
	return map[string]any{
		"risky_sku_count":      nil,
		"stock_risk_count":     nil,
		"order_drop_count":     nil,
		"overstock_count":      nil,
		"ad_risk_count":        nil,
		"top_evidence_signals": []string{},
	}
}
