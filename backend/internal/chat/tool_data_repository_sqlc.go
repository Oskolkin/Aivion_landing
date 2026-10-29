package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type SQLCToolDataRepository struct {
	q *dbgen.Queries
}

func NewSQLCToolDataRepository(q *dbgen.Queries) *SQLCToolDataRepository {
	return &SQLCToolDataRepository{q: q}
}

var _ ToolDataRepository = (*SQLCToolDataRepository)(nil)

func (r *SQLCToolDataRepository) GetDashboardSummary(ctx context.Context, sellerAccountID int64, asOfDate *time.Time) (*DashboardSummaryToolData, error) {
	asOf, source, err := r.resolveAsOfDate(ctx, sellerAccountID, asOfDate)
	if err != nil {
		return nil, err
	}
	from := asOf.AddDate(0, 0, -13)
	accountRows, err := r.q.ListDailyAccountMetricsBySellerAndDateRange(ctx, dbgen.ListDailyAccountMetricsBySellerAndDateRangeParams{
		SellerAccountID: sellerAccountID,
		MetricDate:      toDate(from),
		MetricDate_2:    toDate(asOf),
	})
	if err != nil {
		return nil, fmt.Errorf("list daily account metrics: %w", err)
	}
	byDate := make(map[string]dbgen.DailyAccountMetric, len(accountRows))
	for _, row := range accountRows {
		byDate[row.MetricDate.Time.Format("2006-01-02")] = row
	}
	todayKey := asOf.Format("2006-01-02")
	yesterday := asOf.AddDate(0, 0, -1)
	yesterdayKey := yesterday.Format("2006-01-02")
	today := byDate[todayKey]
	prev := byDate[yesterdayKey]

	revenueToday := numericFloat(today.Revenue)
	revenueYesterday := numericFloat(prev.Revenue)
	ordersToday := today.OrdersCount
	ordersYesterday := prev.OrdersCount
	revenueDelta := revenueToday - revenueYesterday
	ordersDelta := int32(ordersToday - ordersYesterday)
	weekNow := sumAccountRevenue(accountRows, asOf.AddDate(0, 0, -6), asOf)
	weekPrev := sumAccountRevenue(accountRows, asOf.AddDate(0, 0, -13), asOf.AddDate(0, 0, -7))
	weekDelta := weekNow - weekPrev
	lastUpdated := latestAccountUpdate(accountRows)

	return &DashboardSummaryToolData{
		AsOfDate:             todayKey,
		AsOfDateSource:       source,
		DataFreshness:        resolveFreshness(asOf),
		LastSuccessfulUpdate: lastUpdated,
		KPI: map[string]any{
			"revenue": revenueToday,
			"orders":  ordersToday,
			"returns": today.ReturnsCount,
			"cancels": today.CancelCount,
		},
		Deltas: map[string]any{
			"revenue_day_to_day_delta":         round2(revenueDelta),
			"revenue_day_to_day_delta_percent": ratio(revenueDelta, revenueYesterday),
			"orders_day_to_day_delta":          ordersDelta,
			"revenue_week_to_week_delta":       round2(weekDelta),
		},
	}, nil
}

func (r *SQLCToolDataRepository) ListOpenRecommendations(ctx context.Context, sellerAccountID int64, filter RecommendationToolFilter) ([]RecommendationToolItem, error) {
	rows, err := r.q.ListOpenRecommendationsBySellerAccountID(ctx, dbgen.ListOpenRecommendationsBySellerAccountIDParams{
		SellerAccountID: sellerAccountID,
		Limit:           normalizeLimit32(filter.Limit, 200),
		Offset:          0,
	})
	if err != nil {
		return nil, fmt.Errorf("list open recommendations: %w", err)
	}
	out := make([]RecommendationToolItem, 0, len(rows))
	for _, row := range rows {
		item := mapRecommendation(row)
		if len(filter.PriorityLevels) > 0 && !slices.Contains(filter.PriorityLevels, item.PriorityLevel) {
			continue
		}
		if filter.Horizon != nil && item.Horizon != *filter.Horizon {
			continue
		}
		out = append(out, item)
		if int32(len(out)) >= filter.Limit {
			break
		}
	}
	return out, nil
}

func (r *SQLCToolDataRepository) GetRecommendationDetail(ctx context.Context, sellerAccountID int64, recommendationID int64) (*RecommendationDetailToolData, error) {
	row, err := r.q.GetRecommendationByID(ctx, dbgen.GetRecommendationByIDParams{
		ID:              recommendationID,
		SellerAccountID: sellerAccountID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get recommendation by id: %w", err)
	}
	alertRows, err := r.q.ListAlertsByRecommendationID(ctx, dbgen.ListAlertsByRecommendationIDParams{
		SellerAccountID:  sellerAccountID,
		RecommendationID: recommendationID,
	})
	if err != nil {
		return nil, fmt.Errorf("list alerts by recommendation id: %w", err)
	}
	alerts := make([]AlertToolItem, 0, len(alertRows))
	for _, alert := range alertRows {
		alerts = append(alerts, mapAlert(alert))
	}
	return &RecommendationDetailToolData{
		Recommendation: mapRecommendation(row),
		RelatedAlerts:  alerts,
	}, nil
}

func (r *SQLCToolDataRepository) ListOpenAlerts(ctx context.Context, sellerAccountID int64, filter AlertToolFilter) ([]AlertToolItem, error) {
	rows, err := r.q.ListOpenAlertsBySellerAccountID(ctx, dbgen.ListOpenAlertsBySellerAccountIDParams{
		SellerAccountID: sellerAccountID,
		Limit:           normalizeLimit32(filter.Limit, 200),
		Offset:          0,
	})
	if err != nil {
		return nil, fmt.Errorf("list open alerts: %w", err)
	}
	out := make([]AlertToolItem, 0, len(rows))
	for _, row := range rows {
		item := mapAlert(row)
		if len(filter.Severities) > 0 && !slices.Contains(filter.Severities, item.Severity) {
			continue
		}
		if len(filter.Groups) > 0 && !slices.Contains(filter.Groups, item.AlertGroup) {
			continue
		}
		out = append(out, item)
		if int32(len(out)) >= filter.Limit {
			break
		}
	}
	return out, nil
}

func (r *SQLCToolDataRepository) ListAlertsByGroup(ctx context.Context, sellerAccountID int64, group string, limit int32) ([]AlertToolItem, error) {
	rows, err := r.q.ListAlertsByGroup(ctx, dbgen.ListAlertsByGroupParams{
		SellerAccountID: sellerAccountID,
		AlertGroup:      group,
		Limit:           normalizeLimit32(limit, 200),
		Offset:          0,
	})
	if err != nil {
		return nil, fmt.Errorf("list alerts by group: %w", err)
	}
	out := make([]AlertToolItem, 0, len(rows))
	for _, row := range rows {
		if row.Status != "open" {
			continue
		}
		out = append(out, mapAlert(row))
	}
	return out, nil
}

func (r *SQLCToolDataRepository) ListCriticalSKUs(ctx context.Context, sellerAccountID int64, filter CriticalSKUToolFilter) ([]CriticalSKUToolItem, error) {
	asOf, err := r.resolveAsOfMetricDate(ctx, sellerAccountID, filter.AsOfDate)
	if err != nil {
		return nil, err
	}
	rows, err := r.q.ListDailySKUMetricsBySellerAndDateRange(ctx, dbgen.ListDailySKUMetricsBySellerAndDateRangeParams{
		SellerAccountID: sellerAccountID,
		MetricDate:      toDate(asOf),
		MetricDate_2:    toDate(asOf),
	})
	if err != nil {
		return nil, fmt.Errorf("list sku metrics for critical skus: %w", err)
	}
	out := make([]CriticalSKUToolItem, 0, len(rows))
	for _, row := range rows {
		revenue := numericFloat(row.Revenue)
		daysCover := numericFloatPtr(row.DaysOfCover)
		problem := criticalProblemScore(row.StockAvailable, daysCover, revenue)
		importance := criticalImportanceScore(revenue)
		out = append(out, CriticalSKUToolItem{
			SKU:             pgInt64Ptr(row.Sku),
			OfferID:         pgTextPtr(row.OfferID),
			ProductID:       row.OzonProductID,
			ProductName:     pgTextPtr(row.ProductName),
			ProblemScore:    problem,
			ImportanceScore: importance,
			Revenue:         revenue,
			Orders:          row.OrdersCount,
			CurrentStock:    row.StockAvailable,
			DaysOfCover:     daysCover,
			Signals:         buildCriticalSignals(row.StockAvailable, daysCover, revenue),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ProblemScore == out[j].ProblemScore {
			return out[i].ProductID < out[j].ProductID
		}
		return out[i].ProblemScore > out[j].ProblemScore
	})
	if len(out) > int(filter.Limit) {
		out = out[:filter.Limit]
	}
	return out, nil
}

func (r *SQLCToolDataRepository) ListStockRisks(ctx context.Context, sellerAccountID int64, filter StockRiskToolFilter) ([]StockRiskToolItem, error) {
	asOf, err := r.resolveAsOfMetricDate(ctx, sellerAccountID, filter.AsOfDate)
	if err != nil {
		return nil, err
	}
	rows, err := r.q.ListDailySKUMetricsBySellerAndDateRange(ctx, dbgen.ListDailySKUMetricsBySellerAndDateRangeParams{
		SellerAccountID: sellerAccountID,
		MetricDate:      toDate(asOf),
		MetricDate_2:    toDate(asOf),
	})
	if err != nil {
		return nil, fmt.Errorf("list sku metrics for stock risks: %w", err)
	}
	out := make([]StockRiskToolItem, 0, len(rows))
	for _, row := range rows {
		days := numericFloatPtr(row.DaysOfCover)
		risk, priority := classifyStockRisk(row.StockAvailable, days)
		out = append(out, StockRiskToolItem{
			SKU:                   pgInt64Ptr(row.Sku),
			OfferID:               pgTextPtr(row.OfferID),
			ProductID:             row.OzonProductID,
			ProductName:           pgTextPtr(row.ProductName),
			CurrentStock:          row.StockAvailable,
			DaysOfCover:           days,
			DepletionRisk:         risk,
			ReplenishmentPriority: priority,
			EstimatedStockoutDate: estimateStockoutDate(asOf, days),
			Reason:                stockRiskReason(row.StockAvailable, days),
			Revenue:               float64Ptr(numericFloat(row.Revenue)),
			Orders:                int32Ptr(row.OrdersCount),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		ri, rj := riskRank(out[i].DepletionRisk), riskRank(out[j].DepletionRisk)
		if ri == rj {
			return out[i].ProductID < out[j].ProductID
		}
		return ri > rj
	})
	if len(out) > int(filter.Limit) {
		out = out[:filter.Limit]
	}
	return out, nil
}

func (r *SQLCToolDataRepository) GetAdvertisingAnalytics(ctx context.Context, sellerAccountID int64, filter AdvertisingToolFilter) (*AdvertisingToolData, error) {
	dateFrom, dateTo := normalizeDateRange(filter.DateFrom, filter.DateTo, DefaultToolDateRangeDays)
	summaries, err := r.q.ListAdCampaignSummariesBySellerAndDateRange(ctx, dbgen.ListAdCampaignSummariesBySellerAndDateRangeParams{
		SellerAccountID: sellerAccountID,
		DateFrom:        toDate(*dateFrom),
		DateTo:          toDate(*dateTo),
	})
	if err != nil {
		return nil, fmt.Errorf("list ad campaign summaries: %w", err)
	}
	rows := make([]AdvertisingCampaignToolItem, 0, len(summaries))
	totalSpend := 0.0
	totalRevenue := 0.0
	var totalOrders int64
	weak := 0
	for _, row := range summaries {
		if filter.CampaignID != nil && row.CampaignExternalID != *filter.CampaignID {
			continue
		}
		spend := numericFloat(row.SpendTotal)
		revenue := numericFloat(row.RevenueTotal)
		orders := row.OrdersTotal
		roas := ratioPtr(revenue, spend)
		risk := adRiskSignal(spend, revenue, orders)
		if risk != "ok" {
			weak++
		}
		rows = append(rows, AdvertisingCampaignToolItem{
			CampaignID: row.CampaignExternalID,
			Name:       row.CampaignName,
			Type:       pgTextPtr(row.CampaignType),
			Spend:      spend,
			Revenue:    revenue,
			Orders:     orders,
			ROAS:       roas,
			RiskSignal: risk,
		})
		totalSpend += spend
		totalRevenue += revenue
		totalOrders += orders
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Spend > rows[j].Spend })
	if len(rows) > int(filter.Limit) {
		rows = rows[:filter.Limit]
	}
	skuRisks := make([]map[string]any, 0)
	links, err := r.q.ListAdCampaignSKUMappingsBySellerAccountID(ctx, sellerAccountID)
	if err == nil {
		maxRows := minInt(int(filter.Limit), len(links))
		for i := 0; i < maxRows; i++ {
			link := links[i]
			if filter.CampaignID != nil && link.CampaignExternalID != *filter.CampaignID {
				continue
			}
			skuRisks = append(skuRisks, map[string]any{
				"campaign_external_id": link.CampaignExternalID,
				"sku":                  pgInt64Ptr(link.Sku),
				"offer_id":             pgTextPtr(link.OfferID),
				"product_name":         pgTextPtr(link.ProductName),
			})
		}
	}
	return &AdvertisingToolData{
		Summary: map[string]any{
			"total_spend":                     round2(totalSpend),
			"total_revenue":                   round2(totalRevenue),
			"total_orders":                    totalOrders,
			"average_roas":                    ratioPtr(totalRevenue, totalSpend),
			"weak_campaigns_count":            weak,
			"spend_without_result_count":      countSpendWithoutResult(rows),
			"low_stock_advertised_skus_count": nil,
			"date_from":                       dateFrom.Format("2006-01-02"),
			"date_to":                         dateTo.Format("2006-01-02"),
		},
		Campaigns: rows,
		SKURisks:  skuRisks,
	}, nil
}

func (r *SQLCToolDataRepository) ListSKUMetrics(ctx context.Context, sellerAccountID int64, filter SKUMetricsToolFilter) ([]SKUMetricToolItem, error) {
	dateFrom, dateTo := normalizeDateRange(filter.DateFrom, filter.DateTo, DefaultToolDateRangeDays)
	rows, err := r.q.ListDailySKUMetricsBySellerAndDateRange(ctx, dbgen.ListDailySKUMetricsBySellerAndDateRangeParams{
		SellerAccountID: sellerAccountID,
		MetricDate:      toDate(*dateFrom),
		MetricDate_2:    toDate(*dateTo),
	})
	if err != nil {
		return nil, fmt.Errorf("list sku metrics by range: %w", err)
	}
	products, _ := r.q.ListProductsBySellerAccountID(ctx, sellerAccountID)
	productByID := make(map[int64]dbgen.Product, len(products))
	for _, product := range products {
		productByID[product.OzonProductID] = product
	}
	aggregated := map[int64]SKUMetricToolItem{}
	for _, row := range rows {
		if filter.SKU != nil && (!row.Sku.Valid || row.Sku.Int64 != *filter.SKU) {
			continue
		}
		if filter.OfferID != nil && (!row.OfferID.Valid || row.OfferID.String != *filter.OfferID) {
			continue
		}
		if filter.CategoryHint != nil && !matchCategoryHint(productByID[row.OzonProductID], *filter.CategoryHint) {
			continue
		}
		current := aggregated[row.OzonProductID]
		current.ProductID = row.OzonProductID
		current.SKU = pgInt64Ptr(row.Sku)
		current.OfferID = pgTextPtr(row.OfferID)
		current.ProductName = pgTextPtr(row.ProductName)
		current.Revenue = round2(current.Revenue + numericFloat(row.Revenue))
		current.Orders += row.OrdersCount
		current.CurrentStock = int32Ptr(row.StockAvailable)
		current.DaysOfCover = numericFloatPtr(row.DaysOfCover)
		aggregated[row.OzonProductID] = current
	}
	out := make([]SKUMetricToolItem, 0, len(aggregated))
	for _, item := range aggregated {
		out = append(out, item)
	}
	sortSKUMetrics(out, filter.SortBy)
	if len(out) > int(filter.Limit) {
		out = out[:filter.Limit]
	}
	return out, nil
}

func (r *SQLCToolDataRepository) GetSKUContext(ctx context.Context, sellerAccountID int64, filter SKUContextToolFilter) (*SKUContextToolData, error) {
	products, err := r.q.ListProductsBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		return nil, fmt.Errorf("list products for sku context: %w", err)
	}
	var target *dbgen.Product
	for _, product := range products {
		if filter.SKU != nil && product.Sku.Valid && product.Sku.Int64 == *filter.SKU {
			p := product
			target = &p
			break
		}
		if filter.OfferID != nil && product.OfferID.Valid && product.OfferID.String == *filter.OfferID {
			p := product
			target = &p
			break
		}
	}
	if target == nil {
		return nil, nil
	}
	end := time.Now().UTC().Truncate(24 * time.Hour)
	start := end.AddDate(0, 0, -30)
	skuMetrics, err := r.ListSKUMetrics(ctx, sellerAccountID, SKUMetricsToolFilter{
		Limit:    MaxSKUMetricsLimit,
		DateFrom: &start,
		DateTo:   &end,
		SKU:      pgInt64Ptr(target.Sku),
		OfferID:  pgTextPtr(target.OfferID),
		SortBy:   "revenue",
	})
	if err != nil {
		return nil, err
	}
	var metric SKUMetricToolItem
	if len(skuMetrics) > 0 {
		metric = skuMetrics[0]
	}
	stockRows, _ := r.q.ListCurrentStockProductSummariesBySellerAccountID(ctx, sellerAccountID)
	var stock map[string]any
	stock = map[string]any{}
	for _, row := range stockRows {
		if row.OzonProductID == target.OzonProductID {
			stock = map[string]any{
				"current_stock": row.AvailableStock,
				"total_stock":   row.TotalStock,
				"days_of_cover": metric.DaysOfCover,
			}
			break
		}
	}
	pricing := map[string]any{}
	if target.Sku.Valid {
		constraint, err := r.q.GetSKUEffectiveConstraintBySellerAndSKU(ctx, dbgen.GetSKUEffectiveConstraintBySellerAndSKUParams{
			SellerAccountID: sellerAccountID,
			Sku:             pgtype.Int8{Int64: target.Sku.Int64, Valid: true},
		})
		if err == nil {
			pricing = map[string]any{
				"effective_min_price": numericFloatPtr(constraint.EffectiveMinPrice),
				"effective_max_price": numericFloatPtr(constraint.EffectiveMaxPrice),
				"reference_price":     numericFloatPtr(constraint.ReferencePrice),
				"implied_cost":        numericFloatPtr(constraint.ImpliedCost),
				"constraint_source":   constraint.ResolvedFromScopeType,
			}
		}
	}
	alertRows, _ := r.q.ListAlertsByEntitySKU(ctx, dbgen.ListAlertsByEntitySKUParams{
		SellerAccountID: sellerAccountID,
		EntitySku:       target.Sku,
		Limit:           20,
		Offset:          0,
	})
	alerts := make([]AlertToolItem, 0, len(alertRows))
	for _, row := range alertRows {
		if row.Status != "open" {
			continue
		}
		alerts = append(alerts, mapAlert(row))
	}
	recs, _ := r.q.ListOpenRecommendationsBySellerAccountID(ctx, dbgen.ListOpenRecommendationsBySellerAccountIDParams{
		SellerAccountID: sellerAccountID,
		Limit:           50,
		Offset:          0,
	})
	recommendations := make([]RecommendationToolItem, 0)
	for _, rec := range recs {
		if target.Sku.Valid && rec.EntitySku.Valid && rec.EntitySku.Int64 == target.Sku.Int64 {
			recommendations = append(recommendations, mapRecommendation(rec))
			continue
		}
		if target.OfferID.Valid && rec.EntityOfferID.Valid && rec.EntityOfferID.String == target.OfferID.String {
			recommendations = append(recommendations, mapRecommendation(rec))
		}
	}
	links, _ := r.q.ListAdCampaignSKUMappingsBySellerAccountID(ctx, sellerAccountID)
	ads := make([]map[string]any, 0)
	for _, link := range links {
		if link.OzonProductID == target.OzonProductID {
			ads = append(ads, map[string]any{
				"campaign_external_id": link.CampaignExternalID,
				"campaign_name":        pgTextPtr(link.CampaignName),
			})
		}
	}
	return &SKUContextToolData{
		Product: map[string]any{
			"product_id":              target.OzonProductID,
			"ozon_product_id":         target.OzonProductID,
			"sku":                     pgInt64Ptr(target.Sku),
			"offer_id":                pgTextPtr(target.OfferID),
			"product_name":            target.Name,
			"status":                  pgTextPtr(target.Status),
			"description_category_id": pgInt64Ptr(target.DescriptionCategoryID),
		},
		Sales: map[string]any{
			"recent_revenue": metric.Revenue,
			"recent_orders":  metric.Orders,
		},
		Stock:           stock,
		Pricing:         pricing,
		Alerts:          alerts,
		Recommendations: recommendations,
		Advertising:     ads,
	}, nil
}

func (r *SQLCToolDataRepository) GetCampaignContext(ctx context.Context, sellerAccountID int64, campaignID int64) (*CampaignContextToolData, error) {
	end := time.Now().UTC().Truncate(24 * time.Hour)
	start := end.AddDate(0, 0, -30)
	summaries, err := r.q.ListAdCampaignSummariesBySellerAndDateRange(ctx, dbgen.ListAdCampaignSummariesBySellerAndDateRangeParams{
		SellerAccountID: sellerAccountID,
		DateFrom:        toDate(start),
		DateTo:          toDate(end),
	})
	if err != nil {
		return nil, fmt.Errorf("list campaign summaries for context: %w", err)
	}
	var campaign *dbgen.ListAdCampaignSummariesBySellerAndDateRangeRow
	for _, row := range summaries {
		if row.CampaignExternalID == campaignID {
			rw := row
			campaign = &rw
			break
		}
	}
	if campaign == nil {
		return nil, nil
	}
	links, _ := r.q.ListAdCampaignSKUMappingsBySellerAccountID(ctx, sellerAccountID)
	linked := make([]map[string]any, 0)
	for _, link := range links {
		if link.CampaignExternalID != campaignID {
			continue
		}
		linked = append(linked, map[string]any{
			"sku":          pgInt64Ptr(link.Sku),
			"offer_id":     pgTextPtr(link.OfferID),
			"product_name": pgTextPtr(link.ProductName),
		})
	}
	alertRows, _ := r.q.ListAlertsByEntityID(ctx, dbgen.ListAlertsByEntityIDParams{
		SellerAccountID: sellerAccountID,
		EntityType:      "campaign",
		EntityID:        pgtype.Text{String: fmt.Sprintf("%d", campaignID), Valid: true},
		Limit:           20,
		Offset:          0,
	})
	alerts := make([]AlertToolItem, 0, len(alertRows))
	for _, row := range alertRows {
		if row.Status != "open" {
			continue
		}
		alerts = append(alerts, mapAlert(row))
	}
	recs, _ := r.q.ListOpenRecommendationsBySellerAccountID(ctx, dbgen.ListOpenRecommendationsBySellerAccountIDParams{
		SellerAccountID: sellerAccountID,
		Limit:           50,
		Offset:          0,
	})
	recommendations := make([]RecommendationToolItem, 0)
	campaignIDStr := fmt.Sprintf("%d", campaignID)
	for _, rec := range recs {
		if rec.EntityType == "campaign" && rec.EntityID.Valid && rec.EntityID.String == campaignIDStr {
			recommendations = append(recommendations, mapRecommendation(rec))
		}
	}
	spend := numericFloat(campaign.SpendTotal)
	revenue := numericFloat(campaign.RevenueTotal)
	return &CampaignContextToolData{
		Campaign: map[string]any{
			"id":          campaign.CampaignExternalID,
			"external_id": campaign.CampaignExternalID,
			"name":        campaign.CampaignName,
			"type":        pgTextPtr(campaign.CampaignType),
			"status":      pgTextPtr(campaign.Status),
		},
		Metrics: map[string]any{
			"spend":       spend,
			"revenue":     revenue,
			"orders":      campaign.OrdersTotal,
			"impressions": campaign.ImpressionsTotal,
			"clicks":      campaign.ClicksTotal,
			"ctr":         ratioFloat(float64(campaign.ClicksTotal), float64(campaign.ImpressionsTotal)),
			"cpc":         ratioFloat(spend, float64(campaign.ClicksTotal)),
			"roas":        ratioPtr(revenue, spend),
		},
		LinkedSKUs:      linked,
		Alerts:          alerts,
		Recommendations: recommendations,
	}, nil
}

func (r *SQLCToolDataRepository) resolveAsOfDate(ctx context.Context, sellerAccountID int64, requested *time.Time) (time.Time, string, error) {
	if requested != nil {
		d := requested.UTC().Truncate(24 * time.Hour)
		return d, "request", nil
	}
	latest, err := r.q.GetLatestAvailableDashboardMetricDateBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("get latest dashboard date: %w", err)
	}
	if latest.Valid {
		return latest.Time.UTC().Truncate(24 * time.Hour), "latest_available", nil
	}
	now := time.Now().UTC().Truncate(24 * time.Hour)
	return now, "fallback_today", nil
}

func (r *SQLCToolDataRepository) resolveAsOfMetricDate(ctx context.Context, sellerAccountID int64, requested *time.Time) (time.Time, error) {
	asOf, _, err := r.resolveAsOfDate(ctx, sellerAccountID, requested)
	return asOf, err
}

func mapRecommendation(row dbgen.Recommendation) RecommendationToolItem {
	var supporting map[string]any
	_ = json.Unmarshal(row.SupportingMetricsPayload, &supporting)
	var constraints map[string]any
	_ = json.Unmarshal(row.ConstraintsPayload, &constraints)
	var lastSeen *string
	if row.LastSeenAt.Valid {
		s := row.LastSeenAt.Time.UTC().Format(time.RFC3339)
		lastSeen = &s
	}
	return RecommendationToolItem{
		ID:                 row.ID,
		RecommendationType: row.RecommendationType,
		Horizon:            row.Horizon,
		EntityType:         row.EntityType,
		EntityID:           pgTextPtr(row.EntityID),
		EntitySKU:          pgInt64Ptr(row.EntitySku),
		EntityOfferID:      pgTextPtr(row.EntityOfferID),
		Title:              row.Title,
		WhatHappened:       row.WhatHappened,
		WhyItMatters:       row.WhyItMatters,
		RecommendedAction:  row.RecommendedAction,
		ExpectedEffect:     pgTextPtr(row.ExpectedEffect),
		PriorityScore:      numericFloat(row.PriorityScore),
		PriorityLevel:      row.PriorityLevel,
		Urgency:            row.Urgency,
		ConfidenceLevel:    row.ConfidenceLevel,
		Status:             row.Status,
		SupportingMetrics:  supporting,
		Constraints:        constraints,
		LastSeenAt:         lastSeen,
	}
}

func mapAlert(row dbgen.Alert) AlertToolItem {
	var evidence map[string]any
	_ = json.Unmarshal(row.EvidencePayload, &evidence)
	return AlertToolItem{
		ID:            row.ID,
		AlertType:     row.AlertType,
		AlertGroup:    row.AlertGroup,
		EntityType:    row.EntityType,
		EntityID:      pgTextPtr(row.EntityID),
		EntitySKU:     pgInt64Ptr(row.EntitySku),
		EntityOfferID: pgTextPtr(row.EntityOfferID),
		Title:         row.Title,
		Message:       row.Message,
		Severity:      row.Severity,
		Urgency:       row.Urgency,
		Status:        row.Status,
		Evidence:      compactJSONMap(evidence, []string{"current_price", "effective_min_price", "effective_max_price", "expected_margin", "days_of_cover", "available_stock", "spend", "revenue", "orders_count"}, 4096),
		FirstSeenAt:   timePtr(row.FirstSeenAt),
		LastSeenAt:    timePtr(row.LastSeenAt),
	}
}

func toDate(t time.Time) pgtype.Date {
	return pgtype.Date{Time: t.UTC(), Valid: true}
}

func pgTextPtr(v pgtype.Text) *string {
	if !v.Valid {
		return nil
	}
	s := strings.TrimSpace(v.String)
	if s == "" {
		return nil
	}
	return &s
}

func pgInt64Ptr(v pgtype.Int8) *int64 {
	if !v.Valid {
		return nil
	}
	i := v.Int64
	return &i
}

func int32Ptr(v int32) *int32 {
	i := v
	return &i
}

func float64Ptr(v float64) *float64 {
	x := v
	return &x
}

func timePtr(v pgtype.Timestamptz) *string {
	if !v.Valid {
		return nil
	}
	s := v.Time.UTC().Format(time.RFC3339)
	return &s
}

func numericFloat(v pgtype.Numeric) float64 {
	if !v.Valid {
		return 0
	}
	f, err := v.Float64Value()
	if err != nil || !f.Valid {
		return 0
	}
	return round2(f.Float64)
}

func numericFloatPtr(v pgtype.Numeric) *float64 {
	if !v.Valid {
		return nil
	}
	f, err := v.Float64Value()
	if err != nil || !f.Valid {
		return nil
	}
	out := round2(f.Float64)
	return &out
}

func normalizeLimit32(v int32, max int32) int32 {
	if v <= 0 {
		return 20
	}
	if v > max {
		return max
	}
	return v
}

func round2(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}

func ratio(delta float64, base float64) *float64 {
	if base == 0 {
		return nil
	}
	v := round2(delta / base)
	return &v
}

func ratioPtr(a float64, b float64) *float64 {
	if b == 0 {
		return nil
	}
	v := round2(a / b)
	return &v
}

func ratioFloat(a float64, b float64) float64 {
	if b == 0 {
		return 0
	}
	return round2(a / b)
}

func latestAccountUpdate(rows []dbgen.DailyAccountMetric) *string {
	var best time.Time
	found := false
	for _, row := range rows {
		if row.UpdatedAt.Valid && (!found || row.UpdatedAt.Time.After(best)) {
			best = row.UpdatedAt.Time
			found = true
		}
	}
	if !found {
		return nil
	}
	s := best.UTC().Format(time.RFC3339)
	return &s
}

func sumAccountRevenue(rows []dbgen.DailyAccountMetric, from time.Time, to time.Time) float64 {
	total := 0.0
	for _, row := range rows {
		d := row.MetricDate.Time.UTC()
		if d.Before(from) || d.After(to) {
			continue
		}
		total += numericFloat(row.Revenue)
	}
	return round2(total)
}

func resolveFreshness(asOf time.Time) string {
	days := int(time.Since(asOf).Hours() / 24)
	switch {
	case days <= 0:
		return "fresh"
	case days <= 1:
		return "stale_1d"
	default:
		return "stale"
	}
}

func criticalProblemScore(stock int32, days *float64, revenue float64) float64 {
	score := 0.0
	if stock <= 0 {
		score += 6
	} else if stock <= 3 {
		score += 3
	}
	if days != nil && *days <= 3 {
		score += 2
	}
	if revenue <= 0 {
		score += 1
	}
	return round2(score)
}

func criticalImportanceScore(revenue float64) float64 {
	switch {
	case revenue >= 50000:
		return 1.0
	case revenue >= 20000:
		return 0.7
	case revenue > 0:
		return 0.4
	default:
		return 0.1
	}
}

func buildCriticalSignals(stock int32, days *float64, revenue float64) []string {
	signals := make([]string, 0)
	if stock <= 0 {
		signals = append(signals, "out_of_stock")
	} else if stock <= 3 {
		signals = append(signals, "low_stock")
	}
	if days != nil && *days <= 3 {
		signals = append(signals, "days_of_cover_le_3")
	}
	if revenue <= 0 {
		signals = append(signals, "no_revenue")
	}
	return signals
}

func classifyStockRisk(stock int32, days *float64) (string, string) {
	switch {
	case stock <= 0:
		return "critical_out_of_stock", "high"
	case days != nil && *days <= 3:
		return "high_depletion_risk", "high"
	case stock <= 3 || (days != nil && *days <= 7):
		return "medium_depletion_risk", "medium"
	default:
		return "low_depletion_risk", "low"
	}
}

func stockRiskReason(stock int32, days *float64) string {
	if stock <= 0 {
		return "current stock is zero"
	}
	if days != nil && *days <= 3 {
		return "days_of_cover is below 3"
	}
	if stock <= 3 {
		return "available stock is below threshold"
	}
	return "stock level is stable"
}

func estimateStockoutDate(asOf time.Time, days *float64) *string {
	if days == nil || *days <= 0 {
		return nil
	}
	d := asOf.Add(time.Duration(*days*24) * time.Hour).UTC().Format("2006-01-02")
	return &d
}

func normalizeDateRange(from *time.Time, to *time.Time, defaultDays int) (*time.Time, *time.Time) {
	if from != nil && to != nil {
		f := from.UTC().Truncate(24 * time.Hour)
		t := to.UTC().Truncate(24 * time.Hour)
		return &f, &t
	}
	if from != nil {
		f := from.UTC().Truncate(24 * time.Hour)
		t := f.AddDate(0, 0, defaultDays)
		return &f, &t
	}
	if to != nil {
		t := to.UTC().Truncate(24 * time.Hour)
		f := t.AddDate(0, 0, -defaultDays)
		return &f, &t
	}
	end := time.Now().UTC().Truncate(24 * time.Hour)
	start := end.AddDate(0, 0, -defaultDays)
	return &start, &end
}

func adRiskSignal(spend float64, revenue float64, orders int64) string {
	if spend > 0 && (revenue == 0 || orders == 0) {
		return "spend_without_result"
	}
	if spend > 0 && revenue/spend < 1 {
		return "low_roas"
	}
	return "ok"
}

func countSpendWithoutResult(campaigns []AdvertisingCampaignToolItem) int {
	count := 0
	for _, c := range campaigns {
		if c.Spend > 0 && (c.Revenue == 0 || c.Orders == 0) {
			count++
		}
	}
	return count
}

func matchCategoryHint(product dbgen.Product, hint string) bool {
	hint = strings.TrimSpace(strings.ToLower(hint))
	if hint == "" {
		return true
	}
	if product.Name != "" && strings.Contains(strings.ToLower(product.Name), hint) {
		return true
	}
	return false
}

func sortSKUMetrics(items []SKUMetricToolItem, sortBy string) {
	key := strings.ToLower(strings.TrimSpace(sortBy))
	if key == "" {
		key = "revenue"
	}
	sort.Slice(items, func(i, j int) bool {
		switch key {
		case "orders":
			return items[i].Orders > items[j].Orders
		default:
			return items[i].Revenue > items[j].Revenue
		}
	})
}

func riskRank(v string) int {
	switch v {
	case "critical_out_of_stock":
		return 4
	case "high_depletion_risk":
		return 3
	case "medium_depletion_risk":
		return 2
	default:
		return 1
	}
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
