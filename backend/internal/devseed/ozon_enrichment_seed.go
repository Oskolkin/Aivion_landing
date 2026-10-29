package devseed

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OzonEnrichmentSeedOptions struct {
	SellerAccountID int64
	MetricDate      *time.Time
	Scenario        OzonEnrichmentSeedScenario
	EncryptionKey   string
	// ReuseExistingAdvertising, when true, requires the seller to already have
	// real ad_metrics_daily rows and reuses the existing ad_campaigns instead of
	// generating synthetic advertising fallback. Existing ad_metrics_daily rows
	// are never overwritten. When false, synthetic advertising fallback may be
	// generated for sellers without ads.
	ReuseExistingAdvertising bool
}

type OzonEnrichmentSeedResult struct {
	SellerAccountID         int64                      `json:"seller_account_id"`
	MetricDate              string                     `json:"metric_date"`
	Scenario                OzonEnrichmentSeedScenario `json:"scenario"`
	Source                  string                     `json:"source"`
	Scope                   string                     `json:"scope"`
	ProductInfoCount        int32                      `json:"product_info_count"`
	RelatedSKUCount         int32                      `json:"related_sku_count"`
	StockAnalyticsCount     int32                      `json:"stock_analytics_count"`
	CampaignCount           int32                      `json:"campaign_count"`
	CampaignDailyStatsCount int32                      `json:"campaign_daily_stats_count"`
	CampaignObjectsCount    int32                      `json:"campaign_objects_count"`
	CampaignProductsCount   int32                      `json:"campaign_products_count"`
	SKUsUsed                int                        `json:"skus_used"`
}

// SeedOzonEnrichment writes deterministic synthetic P0 enrichment data from local DB entities.
func SeedOzonEnrichment(ctx context.Context, pool *pgxpool.Pool, opts OzonEnrichmentSeedOptions) (*OzonEnrichmentSeedResult, error) {
	if err := AssertDevSeedAllowed(); err != nil {
		return nil, err
	}
	if opts.SellerAccountID <= 0 {
		return nil, fmt.Errorf("seller-id must be > 0")
	}
	switch opts.Scenario {
	case OzonEnrichmentScenarioRich, OzonEnrichmentScenarioMinimal, OzonEnrichmentScenarioEdge:
	default:
		return nil, fmt.Errorf("unknown scenario %q (supported: rich)", opts.Scenario)
	}
	if opts.Scenario != OzonEnrichmentScenarioRich {
		return nil, fmt.Errorf("scenario %q is not implemented yet; use rich", opts.Scenario)
	}

	metricDate, err := resolveOzonEnrichmentMetricDate(ctx, pool, opts.SellerAccountID, opts.MetricDate)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(opts.EncryptionKey) != "" {
		if err := SeedOzonConnectionMock(ctx, dbgen.New(pool), opts.SellerAccountID, metricDate, opts.EncryptionKey); err != nil {
			return nil, fmt.Errorf("ensure ozon connection: %w", err)
		}
	}
	capturedAt := time.Now().UTC()

	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)

	skus, err := loadEnrichmentLocalSKUs(ctx, q, opts.SellerAccountID, metricDate)
	if err != nil {
		return nil, err
	}

	result := &OzonEnrichmentSeedResult{
		SellerAccountID: opts.SellerAccountID,
		MetricDate:      metricDate.Format("2006-01-02"),
		Scenario:        opts.Scenario,
		Source:          DevOzonEnrichmentSeedSource,
		Scope:           OzonEnrichmentSeedScope,
		SKUsUsed:        len(skus),
	}

	if err := seedOzonAPICapabilities(ctx, q, opts.SellerAccountID, capturedAt, opts.Scenario); err != nil {
		return nil, err
	}

	productInfoCount, err := seedOzonProductInfoSnapshots(ctx, q, opts.SellerAccountID, metricDate, capturedAt, skus, opts.Scenario)
	if err != nil {
		return nil, err
	}
	result.ProductInfoCount = productInfoCount

	relatedCount, err := seedOzonRelatedSKUs(ctx, q, opts.SellerAccountID, capturedAt, skus, opts.Scenario)
	if err != nil {
		return nil, err
	}
	result.RelatedSKUCount = relatedCount

	stockCount, err := seedStockAnalyticsSnapshots(ctx, q, opts.SellerAccountID, metricDate, capturedAt, skus, opts.Scenario)
	if err != nil {
		return nil, err
	}
	result.StockAnalyticsCount = stockCount

	existingAdMetricsCount, err := countExistingAdMetricsDaily(ctx, q, opts.SellerAccountID)
	if err != nil {
		return nil, err
	}
	seedSyntheticAdMetrics, err := advertisingReuseDecision(opts.ReuseExistingAdvertising, existingAdMetricsCount > 0, opts.SellerAccountID)
	if err != nil {
		return nil, err
	}

	campaigns, err := ensureEnrichmentCampaigns(ctx, q, opts.SellerAccountID, capturedAt, opts.Scenario, opts.ReuseExistingAdvertising)
	if err != nil {
		return nil, err
	}
	result.CampaignCount = int32(len(campaigns))

	if err := enrichAdCampaigns(ctx, q, opts.SellerAccountID, capturedAt, campaigns, metricDate, opts.Scenario); err != nil {
		return nil, err
	}

	if seedSyntheticAdMetrics {
		statsCount, err := seedAdMetricsDaily(ctx, q, opts.SellerAccountID, metricDate, campaigns, opts.Scenario)
		if err != nil {
			return nil, err
		}
		result.CampaignDailyStatsCount = statsCount
	} else {
		// Reusing existing advertising: do not overwrite ad_metrics_daily rows.
		result.CampaignDailyStatsCount = int32(existingAdMetricsCount)
	}

	objectsCount, err := seedCampaignObjectsAndProducts(ctx, q, opts.SellerAccountID, capturedAt, campaigns, skus, opts.Scenario)
	if err != nil {
		return nil, err
	}
	result.CampaignObjectsCount = objectsCount

	skuCount, err := enrichAllAdCampaignSKUs(ctx, q, opts.SellerAccountID, capturedAt, campaigns, skus, opts.Scenario)
	if err != nil {
		return nil, err
	}
	result.CampaignProductsCount = skuCount

	if err := writeEnrichmentSyncRun(ctx, q, opts.SellerAccountID, metricDate, capturedAt, result); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit ozon enrichment seed: %w", err)
	}
	return result, nil
}

func resolveOzonEnrichmentMetricDate(ctx context.Context, pool *pgxpool.Pool, sellerID int64, override *time.Time) (time.Time, error) {
	if override != nil {
		t := override.UTC()
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), nil
	}
	q := dbgen.New(pool)
	bounds, err := q.GetDailySKUMetricSourceDateBoundsBySellerAccountID(ctx, sellerID)
	if err == nil && bounds.MaxDate.Valid {
		t := bounds.MaxDate.Time.UTC()
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), nil
	}
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC), nil
}

func loadEnrichmentLocalSKUs(ctx context.Context, q *dbgen.Queries, sellerID int64, metricDate time.Time) ([]enrichmentLocalSKU, error) {
	products, err := q.ListProductsBySellerAccountID(ctx, sellerID)
	if err != nil {
		return nil, fmt.Errorf("list products: %w", err)
	}

	metrics, err := q.ListDailySKUMetricsBySellerAndDateRange(ctx, dbgen.ListDailySKUMetricsBySellerAndDateRangeParams{
		SellerAccountID: sellerID,
		MetricDate:      pgDateUTC(metricDate),
		MetricDate_2:    pgDateUTC(metricDate),
	})
	if err != nil {
		return nil, fmt.Errorf("list daily sku metrics: %w", err)
	}

	byProduct := make(map[int64]enrichmentLocalSKU)
	for _, p := range products {
		sku := int64(0)
		if p.Sku.Valid && p.Sku.Int64 > 0 {
			sku = p.Sku.Int64
		} else {
			sku = fallbackSKU(p.OzonProductID, sellerID)
		}
		offerID := ""
		if p.OfferID.Valid {
			offerID = p.OfferID.String
		}
		if offerID == "" {
			offerID = fallbackOfferID(sku)
		}
		price := parseNumericFloat(p.ReferencePrice)
		if price <= 0 {
			price = deterministicPrice(sku, sellerID)
		}
		oldPrice := parseNumericFloat(p.OldPrice)
		if oldPrice <= 0 {
			oldPrice = math.Round(price*1.2*100) / 100
		}
		minPrice := parseNumericFloat(p.OzonMinPrice)
		if minPrice <= 0 {
			minPrice = math.Round(price*0.85*100) / 100
		}
		descCat := int64(0)
		if p.DescriptionCategoryID.Valid {
			descCat = p.DescriptionCategoryID.Int64
		}
		byProduct[p.OzonProductID] = enrichmentLocalSKU{
			OzonProductID:         p.OzonProductID,
			SKU:                   sku,
			OfferID:               offerID,
			Name:                  p.Name,
			DescriptionCategoryID: descCat,
			Price:                 price,
			OldPrice:              oldPrice,
			MinPrice:              minPrice,
		}
	}

	for _, m := range metrics {
		ref, ok := byProduct[m.OzonProductID]
		if !ok {
			sku := int64(0)
			if m.Sku.Valid && m.Sku.Int64 > 0 {
				sku = m.Sku.Int64
			} else {
				sku = fallbackSKU(m.OzonProductID, sellerID)
			}
			offerID := ""
			if m.OfferID.Valid {
				offerID = m.OfferID.String
			}
			if offerID == "" {
				offerID = fallbackOfferID(sku)
			}
			name := ""
			if m.ProductName.Valid {
				name = m.ProductName.String
			}
			ref = enrichmentLocalSKU{
				OzonProductID: m.OzonProductID,
				SKU:           sku,
				OfferID:       offerID,
				Name:          name,
				Price:         deterministicPrice(sku, sellerID),
			}
			ref.OldPrice = math.Round(ref.Price * 1.2 * 100 / 100)
			ref.MinPrice = math.Round(ref.Price * 0.85 * 100 / 100)
		}
		ref.StockAvailable = m.StockAvailable
		ref.DaysOfCover = parseNumericFloat(m.DaysOfCover)
		ref.OrdersCount = m.OrdersCount
		ref.Revenue = parseNumericFloat(m.Revenue)
		byProduct[m.OzonProductID] = ref
	}

	out := make([]enrichmentLocalSKU, 0, len(byProduct))
	for _, sku := range byProduct {
		if sku.SKU <= 0 {
			continue
		}
		out = append(out, sku)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no local products/SKUs found for seller %d", sellerID)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SKU == out[j].SKU {
			return out[i].OzonProductID < out[j].OzonProductID
		}
		return out[i].SKU < out[j].SKU
	})
	return out, nil
}

func seedOzonAPICapabilities(ctx context.Context, q *dbgen.Queries, sellerID int64, capturedAt time.Time, scenario OzonEnrichmentSeedScenario) error {
	sellerMethods := []string{
		"/v1/roles", "/v3/product/list", "/v3/product/info/list",
		"/v1/product/related-sku/get", "/v1/analytics/stocks",
	}
	perfMethods := []string{
		"/api/client/campaign", "/api/client/statistics/daily/json",
		"/api/client/campaign/{id}/objects", "/api/client/campaign/{id}/v2/products",
	}
	sellerRoles, _ := json.Marshal([]map[string]any{
		{"name": "Dev Admin", "methods": sellerMethods},
	})
	sellerMethodsJSON, _ := json.Marshal(sellerMethods)
	sellerRaw := devSeedEnvelope(scenario, map[string]any{
		"api_source": "seller_api",
		"status":     "available",
		"roles":      json.RawMessage(sellerRoles),
		"methods":    sellerMethods,
	})
	if _, err := q.UpsertOzonAPICapability(ctx, dbgen.UpsertOzonAPICapabilityParams{
		SellerAccountID: sellerID,
		Source:          "seller_api",
		CapturedAt:      pgTimestamptzUTC(capturedAt),
		ExpiresAt:       pgTimestamptzUTC(capturedAt.Add(365 * 24 * time.Hour)),
		RolesJson:       sellerRoles,
		MethodsJson:     sellerMethodsJSON,
		RawResponseJson: sellerRaw,
		Status:          "available",
	}); err != nil {
		return fmt.Errorf("upsert seller api capability: %w", err)
	}

	perfRoles, _ := json.Marshal([]map[string]any{{"name": "Dev Performance", "methods": perfMethods}})
	perfMethodsJSON, _ := json.Marshal(perfMethods)
	perfRaw := devSeedEnvelope(scenario, map[string]any{"api_source": "performance_api", "status": "available"})
	if _, err := q.UpsertOzonAPICapability(ctx, dbgen.UpsertOzonAPICapabilityParams{
		SellerAccountID: sellerID,
		Source:          "performance_api",
		CapturedAt:      pgTimestamptzUTC(capturedAt),
		ExpiresAt:       pgTimestamptzUTC(capturedAt.Add(365 * 24 * time.Hour)),
		RolesJson:       perfRoles,
		MethodsJson:     perfMethodsJSON,
		RawResponseJson: perfRaw,
		Status:          "available",
	}); err != nil {
		return fmt.Errorf("upsert performance api capability: %w", err)
	}
	return nil
}

func seedOzonProductInfoSnapshots(
	ctx context.Context,
	q *dbgen.Queries,
	sellerID int64,
	metricDate time.Time,
	capturedAt time.Time,
	skus []enrichmentLocalSKU,
	scenario OzonEnrichmentSeedScenario,
) (int32, error) {
	var count int32
	pgSnapshot := pgDateUTC(metricDate)
	for idx, item := range skus {
		variant := productInfoVariantForIndex(idx)
		color := priceIndexColorForVariant(variant, idx)
		extMin, extIdx, ozMin, ozIdx, selfMin, selfIdx := buildPriceIndexes(item.Price, color)

		status := "active"
		statusName := "Продаётся"
		visStock := true
		if variant == productVariantNoStock {
			statusName = "Нет остатка"
			visStock = false
		}
		moderate := "approved"
		validation := "success"
		if variant == productVariantValidation {
			validation = "warning"
		}

		rawItem := devSeedEnvelope(scenario, map[string]any{
			"product_id": item.OzonProductID,
			"sku":        item.SKU,
			"variant":    string(variant),
		})

		params := dbgen.UpsertOzonProductInfoSnapshotByProductIDParams{
			SellerAccountID:                 sellerID,
			CapturedAt:                      pgTimestamptzUTC(capturedAt),
			SnapshotDate:                    pgSnapshot,
			ProductID:                       pgInt8(item.OzonProductID),
			Sku:                             pgInt8(item.SKU),
			OfferID:                         pgText(item.OfferID),
			Name:                            pgText(item.Name),
			DescriptionCategoryID:           pgInt8(item.DescriptionCategoryID),
			TypeID:                          pgInt8(int64(970000 + idx%100)),
			CurrencyCode:                    pgText("RUB"),
			Price:                           numericFromFloat(item.Price),
			OldPrice:                        numericFromFloat(item.OldPrice),
			MinPrice:                        numericFromFloat(item.MinPrice),
			Vat:                             pgText("0.20"),
			IsArchived:                      pgBool(false),
			IsAutoarchived:                  pgBool(false),
			IsDiscounted:                    pgBool(idx%6 == 0),
			HasDiscountedFboItem:            pgBool(false),
			IsKgt:                           pgBool(false),
			IsSuper:                         pgBool(idx%9 == 0),
			Status:                          pgText(status),
			StatusName:                      pgText(statusName),
			StatusDescription:               pgText(""),
			ModerateStatus:                  pgText(moderate),
			ValidationStatus:                pgText(validation),
			VisibilityHasPrice:              pgBool(true),
			VisibilityHasStock:              pgBool(visStock),
			PriceIndexColor:                 pgText(color),
			ExternalMinimalPrice:            numericFromFloat(extMin),
			ExternalPriceIndexValue:         numericFromFloat(extIdx),
			OzonMinimalPrice:                numericFromFloat(ozMin),
			OzonPriceIndexValue:             numericFromFloat(ozIdx),
			SelfMarketplacesMinimalPrice:    numericFromFloat(selfMin),
			SelfMarketplacesPriceIndexValue: numericFromFloat(selfIdx),
			PrimaryImage:                    pgText(fmt.Sprintf("https://cdn.dev.example/ozon/%d.jpg", item.SKU)),
			ImagesJson:                      []byte(fmt.Sprintf(`["https://cdn.dev.example/ozon/%d.jpg"]`, item.SKU)),
			BarcodesJson:                    []byte("[]"),
			CommissionsJson:                 buildCommissionsJSON(item.Price),
			PromotionsJson:                  buildPromotionsJSON(idx%5 == 0),
			ErrorsJson:                      buildErrorsJSON(variant),
			AvailabilitiesJson:              buildAvailabilitiesJSON(item.SKU, variant),
			StocksJson:                      []byte(`{"has_stock":true,"stocks":[]}`),
			RawResponseJson:                 rawItem,
		}
		if item.OzonProductID > 0 {
			if _, err := q.UpsertOzonProductInfoSnapshotByProductID(ctx, params); err != nil {
				return count, fmt.Errorf("upsert product info product_id=%d: %w", item.OzonProductID, err)
			}
		} else {
			skuParams := productInfoParamsToSKU(params)
			if _, err := q.UpsertOzonProductInfoSnapshotBySKU(ctx, skuParams); err != nil {
				return count, fmt.Errorf("upsert product info sku=%d: %w", item.SKU, err)
			}
		}
		count++
	}
	return count, nil
}

func productInfoParamsToSKU(p dbgen.UpsertOzonProductInfoSnapshotByProductIDParams) dbgen.UpsertOzonProductInfoSnapshotBySKUParams {
	return dbgen.UpsertOzonProductInfoSnapshotBySKUParams{
		SellerAccountID:                 p.SellerAccountID,
		CapturedAt:                      p.CapturedAt,
		SnapshotDate:                    p.SnapshotDate,
		ProductID:                       p.ProductID,
		Sku:                             p.Sku,
		OfferID:                         p.OfferID,
		Name:                            p.Name,
		DescriptionCategoryID:           p.DescriptionCategoryID,
		TypeID:                          p.TypeID,
		CurrencyCode:                    p.CurrencyCode,
		Price:                           p.Price,
		OldPrice:                        p.OldPrice,
		MinPrice:                        p.MinPrice,
		Vat:                             p.Vat,
		IsArchived:                      p.IsArchived,
		IsAutoarchived:                  p.IsAutoarchived,
		IsDiscounted:                    p.IsDiscounted,
		HasDiscountedFboItem:            p.HasDiscountedFboItem,
		DiscountedFboStocks:             p.DiscountedFboStocks,
		IsKgt:                           p.IsKgt,
		IsSuper:                         p.IsSuper,
		Status:                          p.Status,
		StatusName:                      p.StatusName,
		StatusDescription:               p.StatusDescription,
		StatusFailed:                    p.StatusFailed,
		ModerateStatus:                  p.ModerateStatus,
		ValidationStatus:                p.ValidationStatus,
		StatusUpdatedAt:                 p.StatusUpdatedAt,
		VisibilityHasPrice:              p.VisibilityHasPrice,
		VisibilityHasStock:              p.VisibilityHasStock,
		PriceIndexColor:                 p.PriceIndexColor,
		ExternalMinimalPrice:            p.ExternalMinimalPrice,
		ExternalPriceIndexValue:         p.ExternalPriceIndexValue,
		OzonMinimalPrice:                p.OzonMinimalPrice,
		OzonPriceIndexValue:             p.OzonPriceIndexValue,
		SelfMarketplacesMinimalPrice:    p.SelfMarketplacesMinimalPrice,
		SelfMarketplacesPriceIndexValue: p.SelfMarketplacesPriceIndexValue,
		PrimaryImage:                    p.PrimaryImage,
		ImagesJson:                      p.ImagesJson,
		BarcodesJson:                    p.BarcodesJson,
		CommissionsJson:                 p.CommissionsJson,
		PromotionsJson:                  p.PromotionsJson,
		ErrorsJson:                      p.ErrorsJson,
		AvailabilitiesJson:              p.AvailabilitiesJson,
		StocksJson:                      p.StocksJson,
		RawResponseJson:                 p.RawResponseJson,
	}
}

func seedOzonRelatedSKUs(
	ctx context.Context,
	q *dbgen.Queries,
	sellerID int64,
	capturedAt time.Time,
	skus []enrichmentLocalSKU,
	scenario OzonEnrichmentSeedScenario,
) (int32, error) {
	_ = q.DeleteOzonRelatedSKUErrorsBySellerAndCode(ctx, dbgen.DeleteOzonRelatedSKUErrorsBySellerAndCodeParams{
		SellerAccountID: sellerID,
		ErrorCode:       pgText(devRelatedSKUErrorCode),
	})

	limit := len(skus)
	if limit > 10 {
		limit = 10
	}
	if limit < 1 {
		limit = len(skus)
	}
	var count int32
	for idx := 0; idx < limit; idx++ {
		item := skus[idx]
		raw := devSeedEnvelope(scenario, map[string]any{"requested_sku": item.SKU})
		links := []struct {
			related int64
			schema  string
			avail   string
			deleted *time.Time
		}{
			{item.SKU, "FBO", "AVAILABLE", nil},
			{item.SKU + 1_000_000_000, "FBS", "AVAILABLE", nil},
		}
		if idx%3 == 0 {
			deleted := capturedAt.AddDate(0, 0, -30)
			links = append(links, struct {
				related int64
				schema  string
				avail   string
				deleted *time.Time
			}{item.SKU + 2_000_000_000, "FBO", "DELETED", &deleted})
		}
		for _, link := range links {
			var deletedAt pgtype.Timestamptz
			if link.deleted != nil {
				deletedAt = pgTimestamptzUTC(*link.deleted)
			}
			if _, err := q.UpsertOzonRelatedSKU(ctx, dbgen.UpsertOzonRelatedSKUParams{
				SellerAccountID: sellerID,
				RequestedSku:    item.SKU,
				RelatedSku:      link.related,
				ProductID:       pgInt8(item.OzonProductID),
				DeliverySchema:  pgText(link.schema),
				Availability:    pgText(link.avail),
				DeletedAt:       deletedAt,
				CapturedAt:      pgTimestamptzUTC(capturedAt),
				RawResponseJson: raw,
			}); err != nil {
				return count, err
			}
			count++
		}
	}

	_, _ = q.InsertOzonRelatedSKUError(ctx, dbgen.InsertOzonRelatedSKUErrorParams{
		SellerAccountID: sellerID,
		RequestedSku:    999_999_999,
		ErrorSku:        pgInt8(999_999_999),
		ErrorCode:       pgText(devRelatedSKUErrorCode),
		ErrorMessage:    pgText("Synthetic related SKU error for diagnostics"),
		CapturedAt:      pgTimestamptzUTC(capturedAt),
	})
	return count, nil
}

func seedStockAnalyticsSnapshots(
	ctx context.Context,
	q *dbgen.Queries,
	sellerID int64,
	metricDate time.Time,
	capturedAt time.Time,
	skus []enrichmentLocalSKU,
	scenario OzonEnrichmentSeedScenario,
) (int32, error) {
	if scenario == OzonEnrichmentScenarioRich {
		if err := prepareRichStockScenarios(ctx, q, sellerID, capturedAt, skus); err != nil {
			return 0, err
		}
	}

	availableStockByProduct, err := loadAvailableStockByProduct(ctx, q, sellerID)
	if err != nil {
		return 0, err
	}

	var count int32
	pgSnapshot := pgDateUTC(metricDate)
	for idx, item := range skus {
		availableStock := availableStockByProduct[item.OzonProductID]
		// Prefer stocks aggregate; fall back to metric-day stock on the SKU row.
		// Rich seed never invents available_stock — it may only adjust stocks via
		// prepareRichStockScenarios and reshape cover/orders.
		if availableStock == 0 && item.StockAvailable > 0 && scenario != OzonEnrichmentScenarioRich {
			availableStock = item.StockAvailable
		}
		if scenario == OzonEnrichmentScenarioRich {
			want := stockScenarioForIndex(idx)
			item = reshapeEnrichmentItemForScenario(want, item)
		}
		scen := deriveStockScenarioFromFacts(availableStock, item.DaysOfCover, item.OrdersCount, item.OrdersCount)
		vals := stockAnalyticsValues(scen, idx, item, availableStock)
		warehouseID := vals["warehouse_id"].(int64)
		clusterID := vals["cluster_id"].(int64)
		raw := devSeedEnvelope(scenario, map[string]any{
			"sku":                    item.SKU,
			"turnover_grade":         string(scen),
			"stock_scenario":         string(scen),
			"available_stock_source": "stocks_aggregate",
			"derived_from":           "days_of_cover_orders_stock",
		})
		ads := 0.0
		if v, ok := vals["ads"]; ok {
			ads = v.(float64)
		}
		idcVal := 0.0
		if v, ok := vals["idc"]; ok {
			idcVal = v.(float64)
		}
		if _, err := q.UpsertStockAnalyticsSnapshot(ctx, dbgen.UpsertStockAnalyticsSnapshotParams{
			SellerAccountID:              sellerID,
			SnapshotDate:                 pgSnapshot,
			Sku:                          item.SKU,
			OfferID:                      pgText(item.OfferID),
			ProductName:                  pgText(item.Name),
			WarehouseID:                  warehouseID,
			WarehouseName:                pgText(vals["warehouse_name"].(string)),
			ClusterID:                    clusterID,
			ClusterName:                  pgText(vals["cluster_name"].(string)),
			MacrolocalClusterID:          pgInt8(int64(1 + idx%4)),
			AvailableStockCount:          availableStock,
			ExcessStockCount:             int32ValueFromMap(vals, "excess_stock_count"),
			Ads:                          numericFromFloat(ads),
			AdsCluster:                   numericFromFloat(ads * 0.95),
			Idc:                          pgInt4(int32(idcVal)),
			IdcCluster:                   numericFromFloat(idcVal * 0.9),
			TurnoverGrade:                pgText(string(scen)),
			TurnoverGradeCluster:         pgText(string(scen)),
			DaysWithoutSales:             int32FromMap(vals, "days_without_sales"),
			DaysWithoutSalesCluster:      int32FromMap(vals, "days_without_sales"),
			RequestedStockCount:          int32FromMap(vals, "requested_stock_count"),
			TransitStockCount:            int32FromMap(vals, "transit_stock_count"),
			ValidStockCount:              int32FromMap(vals, "valid_stock_count"),
			WaitingDocsStockCount:        int32FromMap(vals, "waiting_docs_stock_count"),
			ReturnFromCustomerStockCount: int32FromMap(vals, "return_from_customer_stock_count"),
			StockDefectStockCount:        int32FromMap(vals, "stock_defect_stock_count"),
			ExpiringStockCount:           int32FromMap(vals, "expiring_stock_count"),
			ItemTagsJson:                 []byte("[]"),
			CapturedAt:                   pgTimestamptzUTC(capturedAt),
			RawResponseJson:              raw,
		}); err != nil {
			return count, fmt.Errorf("upsert stock analytics sku=%d: %w", item.SKU, err)
		}
		count++
	}
	return count, nil
}

// prepareRichStockScenarios makes deriveStockScenarioFromFacts able to emit every
// turnover grade for rich demos/tests:
//   - WAITING_FOR_SUPPLY SKUs get zero stocks aggregate
//   - all other grades get a positive stocks aggregate when none exists yet
//
// AvailableStockCount written to snapshots always mirrors the stocks table.
func prepareRichStockScenarios(
	ctx context.Context,
	q *dbgen.Queries,
	sellerID int64,
	capturedAt time.Time,
	skus []enrichmentLocalSKU,
) error {
	existing, err := loadAvailableStockByProduct(ctx, q, sellerID)
	if err != nil {
		return err
	}
	for idx, item := range skus {
		want := stockScenarioForIndex(idx)
		if want == stockScenarioWaitingForSupply {
			for _, wh := range []string{"WH-A", "WH-B", "rich-waiting"} {
				if _, err := q.UpsertStock(ctx, dbgen.UpsertStockParams{
					SellerAccountID:     sellerID,
					ProductExternalID:   fmt.Sprintf("%d", item.OzonProductID),
					WarehouseExternalID: wh,
					QuantityTotal:       pgInt4(0),
					QuantityReserved:    pgInt4(0),
					QuantityAvailable:   pgInt4(0),
					SnapshotAt:          pgTimestamptzUTC(capturedAt),
					RawAttributes:       []byte(`{"source":"dev_seed_rich_waiting"}`),
				}); err != nil {
					return fmt.Errorf("clear stocks for waiting sku=%d wh=%s: %w", item.SKU, wh, err)
				}
			}
			continue
		}
		if existing[item.OzonProductID] > 0 {
			continue
		}
		qty := item.StockAvailable
		if qty <= 0 {
			qty = int32(10 + idx%20)
		}
		if _, err := q.UpsertStock(ctx, dbgen.UpsertStockParams{
			SellerAccountID:     sellerID,
			ProductExternalID:   fmt.Sprintf("%d", item.OzonProductID),
			WarehouseExternalID: "WH-A",
			QuantityTotal:       pgInt4(qty),
			QuantityReserved:    pgInt4(0),
			QuantityAvailable:   pgInt4(qty),
			SnapshotAt:          pgTimestamptzUTC(capturedAt),
			RawAttributes:       []byte(`{"source":"dev_seed_rich_stock_foundation"}`),
		}); err != nil {
			return fmt.Errorf("seed stocks for sku=%d: %w", item.SKU, err)
		}
	}
	return nil
}

// loadAvailableStockByProduct returns, keyed by ozon_product_id, the canonical
// available stock aggregate: the SUM of the latest quantity_available per
// warehouse for each product. This matches the Analysis Pack aggregation
// (ListCurrentStockBySellerAccountAndProduct). Products without stocks rows are
// absent from the map and therefore resolve to zero available stock.
func loadAvailableStockByProduct(ctx context.Context, q *dbgen.Queries, sellerID int64) (map[int64]int32, error) {
	rows, err := q.ListCurrentStockBySellerAccountAndProduct(ctx, sellerID)
	if err != nil {
		return nil, fmt.Errorf("load available stock aggregate: %w", err)
	}
	out := make(map[int64]int32, len(rows))
	for _, r := range rows {
		out[r.OzonProductID] = r.StockAvailable
	}
	return out, nil
}

func int32ValueFromMap(m map[string]any, key string) int32 {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case int32:
		return n
	case int:
		return int32(n)
	default:
		return 0
	}
}

func int32FromMap(m map[string]any, key string) pgtype.Int4 {
	v, ok := m[key]
	if !ok {
		return pgtype.Int4{Valid: false}
	}
	switch n := v.(type) {
	case int32:
		return pgInt4(n)
	case int:
		return pgInt4(int32(n))
	default:
		return pgtype.Int4{Valid: false}
	}
}

func ensureEnrichmentCampaigns(
	ctx context.Context,
	q *dbgen.Queries,
	sellerID int64,
	capturedAt time.Time,
	scenario OzonEnrichmentSeedScenario,
	reuseExisting bool,
) ([]dbgen.AdCampaign, error) {
	campaigns, err := q.ListAdCampaignsBySellerAccountID(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	if len(campaigns) > 0 {
		return campaigns, nil
	}
	if reuseExisting {
		return nil, fmt.Errorf("reuse-existing-advertising is set but seller %d has no existing ad_campaigns to reuse", sellerID)
	}
	defs := []struct {
		id   int64
		name string
	}{
		{990_001, "Dev SKU Manual CPC"},
		{990_002, "Dev SKU Auto MAX_CLICKS"},
		{990_003, "Dev SKU Stopped Budget"},
		{990_004, "Dev SKU Inactive"},
	}
	for idx, def := range defs {
		raw := devSeedEnvelope(scenario, map[string]any{"campaign_external_id": def.id})
		kind := campaignKindForIndex(idx)
		state, payment, mode, strategy, daily := campaignStateFromKind(kind)
		dailyMicro := microRubles(daily)
		if _, err := q.UpsertAdCampaign(ctx, dbgen.UpsertAdCampaignParams{
			SellerAccountID:          sellerID,
			CampaignExternalID:       def.id,
			CampaignName:             def.name,
			CampaignType:             pgText("SKU"),
			PlacementType:            pgText("PLACEMENT_SEARCH_AND_CATEGORY"),
			Status:                   pgText(state),
			BudgetAmount:             numericFromFloat(daily * 30),
			BudgetDaily:              numericFromFloat(daily),
			PaymentType:              pgText(payment),
			DailyBudgetRawMicro:      pgInt8(dailyMicro),
			BudgetWeekly:             numericFromFloat(daily * 7),
			WeeklyBudgetRawMicro:     pgInt8(microRubles(daily * 7)),
			ProductCampaignMode:      pgText(mode),
			ProductAutopilotStrategy: pgText(strategy),
			CapturedAt:               pgTimestamptzUTC(capturedAt),
			RawAttributes:            raw,
		}); err != nil {
			return nil, err
		}
	}
	return q.ListAdCampaignsBySellerAccountID(ctx, sellerID)
}

func campaignStateFromKind(kind campaignEnrichmentKind) (state, payment, mode, strategy string, dailyBudget float64) {
	switch kind {
	case campaignKindAutoRunning:
		return "CAMPAIGN_STATE_RUNNING", "CPC", "PRODUCT_CAMPAIGN_MODE_AUTO", "MAX_CLICKS", 4500
	case campaignKindStoppedBudget:
		return "CAMPAIGN_STATE_STOPPED", "CPC", "PRODUCT_CAMPAIGN_MODE_MANUAL", "", 5000
	case campaignKindInactive:
		return "CAMPAIGN_STATE_INACTIVE", "CPC", "PRODUCT_CAMPAIGN_MODE_MANUAL", "", 3000
	default:
		return "CAMPAIGN_STATE_RUNNING", "CPC", "PRODUCT_CAMPAIGN_MODE_MANUAL", "", 6000
	}
}

func enrichAdCampaigns(
	ctx context.Context,
	q *dbgen.Queries,
	sellerID int64,
	capturedAt time.Time,
	campaigns []dbgen.AdCampaign,
	metricDate time.Time,
	scenario OzonEnrichmentSeedScenario,
) error {
	for idx, c := range campaigns {
		kind := campaignKindForIndex(idx)
		state, payment, mode, strategy, daily := campaignStateFromKind(kind)
		autostop := ""
		if kind == campaignKindStoppedBudget {
			autostop = "AUTOSTOP_STATUS_STOPPED"
		}
		dailyMicro := microRubles(daily)
		weeklyMicro := microRubles(daily * 7)
		budgetMicro := microRubles(daily * 30)
		placementJSON, _ := json.Marshal(map[string]any{"placement": "PLACEMENT_SEARCH_AND_CATEGORY", "source": DevOzonEnrichmentSeedSource})
		raw := devSeedEnvelope(scenario, map[string]any{
			"campaign_external_id": c.CampaignExternalID,
			"enrichment_kind":      string(kind),
		})
		name := c.CampaignName
		if name == "" {
			name = fmt.Sprintf("campaign_%d", c.CampaignExternalID)
		}
		campaignType := "SKU"
		if c.CampaignType.Valid && c.CampaignType.String != "" {
			campaignType = c.CampaignType.String
		}
		if _, err := q.UpsertAdCampaign(ctx, dbgen.UpsertAdCampaignParams{
			SellerAccountID:          sellerID,
			CampaignExternalID:       c.CampaignExternalID,
			CampaignName:             name,
			CampaignType:             pgText(campaignType),
			PlacementType:            pgText("PLACEMENT_SEARCH_AND_CATEGORY"),
			Status:                   pgText(state),
			BudgetAmount:             numericFromFloat(daily * 30),
			BudgetDaily:              numericFromFloat(daily),
			PaymentType:              pgText(payment),
			PlacementJson:            placementJSON,
			BudgetRawMicro:           pgInt8(budgetMicro),
			DailyBudgetRawMicro:      pgInt8(dailyMicro),
			WeeklyBudgetRawMicro:     pgInt8(weeklyMicro),
			BudgetWeekly:             numericFromFloat(daily * 7),
			FromDate:                 pgDateUTC(metricDate.AddDate(0, 0, -30)),
			ToDate:                   pgDateUTC(metricDate.AddDate(0, 0, 30)),
			AutostopStatus:           pgText(autostop),
			IsAutocreated:            pgBool(kind == campaignKindAutoRunning),
			ProductAutopilotStrategy: pgText(strategy),
			ProductCampaignMode:      pgText(mode),
			PerformanceCreatedAt:     pgTimestamptzUTC(capturedAt.AddDate(0, 0, -60)),
			PerformanceUpdatedAt:     pgTimestamptzUTC(capturedAt),
			CapturedAt:               pgTimestamptzUTC(capturedAt),
			RawAttributes:            raw,
		}); err != nil {
			return fmt.Errorf("enrich campaign %d: %w", c.CampaignExternalID, err)
		}
	}
	return nil
}

// countExistingAdMetricsDaily returns how many ad_metrics_daily rows already
// exist for the seller across all dates.
func countExistingAdMetricsDaily(ctx context.Context, q *dbgen.Queries, sellerID int64) (int, error) {
	rows, err := q.ListAdMetricsDailyBySellerAndDateRange(ctx, dbgen.ListAdMetricsDailyBySellerAndDateRangeParams{
		SellerAccountID: sellerID,
		DateFrom:        pgDateUTC(time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)),
		DateTo:          pgDateUTC(time.Date(2999, 12, 31, 0, 0, 0, 0, time.UTC)),
	})
	if err != nil {
		return 0, fmt.Errorf("count existing ad_metrics_daily: %w", err)
	}
	return len(rows), nil
}

// advertisingReuseDecision resolves how advertising should be seeded.
//
// It returns whether synthetic ad_metrics_daily rows should be generated. When
// reuseExisting is true the caller reuses existing ads and never overwrites
// ad_metrics_daily; if no real ad metrics exist it is a hard error (the final
// seller-1 flow must not fall back to synthetic advertising).
func advertisingReuseDecision(reuseExisting, hasExistingAdMetrics bool, sellerID int64) (seedSyntheticAdMetrics bool, err error) {
	if !reuseExisting {
		return true, nil
	}
	if !hasExistingAdMetrics {
		return false, fmt.Errorf("reuse-existing-advertising is set but seller %d has no existing ad_metrics_daily rows; refusing to seed synthetic advertising fallback", sellerID)
	}
	return false, nil
}

func seedAdMetricsDaily(
	ctx context.Context,
	q *dbgen.Queries,
	sellerID int64,
	metricDate time.Time,
	campaigns []dbgen.AdCampaign,
	scenario OzonEnrichmentSeedScenario,
) (int32, error) {
	var count int32
	from := metricDate.AddDate(0, 0, -enrichmentStatsLookbackDays)
	metricDay := time.Date(metricDate.Year(), metricDate.Month(), metricDate.Day(), 0, 0, 0, 0, time.UTC)
	for day := from; !day.After(metricDate); day = day.AddDate(0, 0, 1) {
		dayUTC := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
		daysBefore := int(metricDay.Sub(dayUTC).Hours() / 24)
		if daysBefore < 0 {
			daysBefore = 0
		}
		for idx, c := range campaigns {
			profile := adMetricsProfileForIndex(idx)
			impressions, clicks, spend, revenue, orders := dailyAdMetricsForProfileOnDay(profile, daysBefore)
			raw := devSeedEnvelope(scenario, map[string]any{
				"campaign_external_id": c.CampaignExternalID,
				"metric_date":          day.Format("2006-01-02"),
				"ad_metrics_profile":   string(profile),
				"days_before_metric":   daysBefore,
			})
			name := c.CampaignName
			if _, err := q.UpsertAdMetricDaily(ctx, dbgen.UpsertAdMetricDailyParams{
				SellerAccountID:    sellerID,
				CampaignExternalID: c.CampaignExternalID,
				MetricDate:         pgDateUTC(day),
				CampaignName:       pgText(name),
				Impressions:        impressions,
				Clicks:             clicks,
				Spend:              numericMoneyAmount(spend),
				OrdersCount:        orders,
				Revenue:            numericMoneyAmount(revenue),
				RawAttributes:      raw,
			}); err != nil {
				return count, err
			}
			count++
		}
	}
	return count, nil
}

func seedCampaignObjectsAndProducts(
	ctx context.Context,
	q *dbgen.Queries,
	sellerID int64,
	capturedAt time.Time,
	campaigns []dbgen.AdCampaign,
	skus []enrichmentLocalSKU,
	scenario OzonEnrichmentSeedScenario,
) (int32, error) {
	var objectsCount int32
	kindByCampaign := campaignKindByExternalID(campaigns)
	for cIdx, campaign := range campaigns {
		kind := kindByCampaign[campaign.CampaignExternalID]
		if kind == "" {
			kind = campaignKindForIndex(cIdx)
		}
		linkCount := 3
		if len(skus) < linkCount {
			linkCount = len(skus)
		}
		if linkCount > 8 {
			linkCount = 8
		}
		for i := 0; i < linkCount; i++ {
			item := skus[(cIdx+i)%len(skus)]
			objRaw := devSeedEnvelope(scenario, map[string]any{
				"campaign_id": campaign.CampaignExternalID,
				"object_id":   formatSKU(item.SKU),
				"object_type": "sku",
			})
			if _, err := q.UpsertAdCampaignObject(ctx, dbgen.UpsertAdCampaignObjectParams{
				SellerAccountID:    sellerID,
				CampaignExternalID: campaign.CampaignExternalID,
				ObjectExternalID:   formatSKU(item.SKU),
				ObjectType:         pgText("sku"),
				CapturedAt:         pgTimestamptzUTC(capturedAt),
				RawResponseJson:    objRaw,
			}); err != nil {
				return objectsCount, err
			}
			objectsCount++

			linkID := item.OzonProductID
			if linkID <= 0 {
				linkID = item.SKU
			}
			if _, err := q.UpsertAdCampaignSKU(ctx, buildEnrichedCampaignSKUParams(
				sellerID,
				campaign.CampaignExternalID,
				linkID,
				pgText(item.OfferID),
				pgInt8(item.SKU),
				true,
				pgText("active"),
				item.Name,
				item.SKU,
				kind,
				capturedAt,
				scenario,
				nil,
				map[string]any{
					"campaign_external_id": campaign.CampaignExternalID,
					"sku":                  item.SKU,
				},
			)); err != nil {
				return objectsCount, err
			}
		}
	}
	return objectsCount, nil
}

func enrichAllAdCampaignSKUs(
	ctx context.Context,
	q *dbgen.Queries,
	sellerID int64,
	capturedAt time.Time,
	campaigns []dbgen.AdCampaign,
	skus []enrichmentLocalSKU,
	scenario OzonEnrichmentSeedScenario,
) (int32, error) {
	links, err := q.ListAdCampaignSKUMappingsBySellerAccountID(ctx, sellerID)
	if err != nil {
		return 0, fmt.Errorf("list ad campaign skus: %w", err)
	}

	kindByCampaign := campaignKindByExternalID(campaigns)
	campaignByExternalID := make(map[int64]dbgen.AdCampaign, len(campaigns))
	for _, c := range campaigns {
		campaignByExternalID[c.CampaignExternalID] = c
	}
	titles := buildProductTitleLookups(skus)

	var count int32
	for _, link := range links {
		if !link.Sku.Valid || link.Sku.Int64 <= 0 {
			continue
		}
		kind := kindByCampaign[link.CampaignExternalID]
		if kind == "" {
			kind = campaignKindManualRunning
			if c, ok := campaignByExternalID[link.CampaignExternalID]; ok && isAutoCampaign(c) {
				kind = campaignKindAutoRunning
			}
		}

		title := resolveCampaignSKUProductTitle(link, titles)
		extra := map[string]any{
			"campaign_external_id": link.CampaignExternalID,
			"sku":                  link.Sku.Int64,
		}
		if kind == campaignKindAutoRunning {
			extra["bid_applicability"] = "not_applicable_auto_strategy"
		}

		if _, err := q.UpsertAdCampaignSKU(ctx, buildEnrichedCampaignSKUParams(
			sellerID,
			link.CampaignExternalID,
			link.OzonProductID,
			link.OfferID,
			link.Sku,
			link.IsActive,
			link.Status,
			title,
			link.Sku.Int64,
			kind,
			capturedAt,
			scenario,
			link.RawAttributes,
			extra,
		)); err != nil {
			return count, fmt.Errorf("enrich campaign sku %d/%d: %w", link.CampaignExternalID, link.Sku.Int64, err)
		}
		count++
	}
	return count, nil
}

func buildEnrichedCampaignSKUParams(
	sellerID int64,
	campaignExternalID int64,
	ozonProductID int64,
	offerID pgtype.Text,
	sku pgtype.Int8,
	isActive bool,
	status pgtype.Text,
	productTitle string,
	skuValue int64,
	kind campaignEnrichmentKind,
	capturedAt time.Time,
	scenario OzonEnrichmentSeedScenario,
	existingRaw []byte,
	extra map[string]any,
) dbgen.UpsertAdCampaignSKUParams {
	bid, hasBid := bidAmountForSKUInt64(skuValue, kind)
	raw := mergeDevSeedRawAttributes(existingRaw, scenario, extra)
	params := dbgen.UpsertAdCampaignSKUParams{
		SellerAccountID:    sellerID,
		CampaignExternalID: campaignExternalID,
		OzonProductID:      ozonProductID,
		OfferID:            offerID,
		Sku:                sku,
		IsActive:           isActive,
		Status:             status,
		ProductTitle:       pgText(productTitle),
		CapturedAt:         pgTimestamptzUTC(capturedAt),
		RawAttributes:      raw,
	}
	if hasBid {
		params.BidRawMicro = pgInt8(microRubles(bid))
		params.BidAmount = numericMoneyAmount(bid)
		params.BidUnitAssumption = pgText("micro_rubles")
	}
	return params
}

func writeEnrichmentSyncRun(
	ctx context.Context,
	q *dbgen.Queries,
	sellerID int64,
	metricDate time.Time,
	capturedAt time.Time,
	result *OzonEnrichmentSeedResult,
) error {
	_ = q.DeleteOzonEnrichmentSyncRunsBySellerDateScope(ctx, dbgen.DeleteOzonEnrichmentSyncRunsBySellerDateScopeParams{
		SellerAccountID: sellerID,
		MetricDate:      pgDateUTC(metricDate),
		Scope:           OzonEnrichmentSeedScope,
	})
	run, err := q.CreateOzonEnrichmentSyncRun(ctx, dbgen.CreateOzonEnrichmentSyncRunParams{
		SellerAccountID: sellerID,
		MetricDate:      pgDateUTC(metricDate),
		Scope:           OzonEnrichmentSeedScope,
		StartedAt:       pgTimestamptzUTC(capturedAt),
	})
	if err != nil {
		return err
	}
	summary, _ := json.Marshal(map[string]any{
		"source":      DevOzonEnrichmentSeedSource,
		"scenario":    string(result.Scenario),
		"seller_id":   sellerID,
		"metric_date": metricDate.Format("2006-01-02"),
		"generated_tables": []string{
			"ozon_api_capabilities",
			"ozon_product_info_snapshots",
			"ozon_related_skus",
			"stock_analytics_snapshots",
			"ad_campaigns",
			"ad_metrics_daily",
			"ad_campaign_objects",
			"ad_campaign_skus",
		},
	})
	_, err = q.FinishOzonEnrichmentSyncRun(ctx, dbgen.FinishOzonEnrichmentSyncRunParams{
		ID:                      run.ID,
		Status:                  "completed",
		FinishedAt:              pgTimestamptzUTC(capturedAt),
		SellerApiAvailable:      true,
		PerformanceApiAvailable: true,
		ProductInfoCount:        result.ProductInfoCount,
		RelatedSkuCount:         result.RelatedSKUCount,
		StockAnalyticsCount:     result.StockAnalyticsCount,
		CampaignCount:           result.CampaignCount,
		CampaignDailyStatsCount: result.CampaignDailyStatsCount,
		CampaignObjectsCount:    result.CampaignObjectsCount,
		CampaignProductsCount:   result.CampaignProductsCount,
		ErrorsJson:              []byte("[]"),
		WarningsJson:            devSeedWarningJSON(),
		RawSummaryJson:          summary,
	})
	return err
}
