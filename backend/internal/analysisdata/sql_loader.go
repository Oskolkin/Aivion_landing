package analysisdata

import (
	"context"
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SQLLoader loads AnalysisDataSnapshot rows via dbgen queries.
type SQLLoader struct {
	queries *dbgen.Queries
}

// NewSQLLoader creates a SQLLoader backed by a connection pool.
func NewSQLLoader(db *pgxpool.Pool) *SQLLoader {
	if db == nil {
		panic("analysisdata: db pool must not be nil")
	}
	return &SQLLoader{queries: dbgen.New(db)}
}

// NewSQLLoaderWithQueries creates a SQLLoader from an existing Queries handle.
func NewSQLLoaderWithQueries(queries *dbgen.Queries) *SQLLoader {
	if queries == nil {
		panic("analysisdata: queries must not be nil")
	}
	return &SQLLoader{queries: queries}
}

// Load fetches the legacy 14-day analysis window snapshot for the seller.
func (l *SQLLoader) Load(
	ctx context.Context,
	sellerAccountID int64,
	metricDate time.Time,
) (*AnalysisDataSnapshot, error) {
	return l.LoadWithOptions(ctx, LoadOptions{
		SellerAccountID: sellerAccountID,
		MetricDate:      metricDate,
		HistoryDays:     LegacyHistoryDays,
	})
}

// LoadWithOptions fetches a snapshot for an explicit inclusive history window.
func (l *SQLLoader) LoadWithOptions(
	ctx context.Context,
	options LoadOptions,
) (*AnalysisDataSnapshot, error) {
	if options.SellerAccountID <= 0 {
		return nil, fmt.Errorf("%w", ErrInvalidSellerAccountID)
	}
	if l == nil || l.queries == nil {
		return nil, fmt.Errorf("analysisdata: SQLLoader is not configured")
	}
	historyDays := options.HistoryDays
	if historyDays == 0 {
		historyDays = LegacyHistoryDays
	}
	from, to, err := HistoryDateRange(options.MetricDate, historyDays)
	if err != nil {
		return nil, err
	}

	asOf := DateOnlyUTC(options.MetricDate)
	pgDate := pgtype.Date{Time: asOf, Valid: true}
	// Windows remain the legacy 14d rolling windows used by SellerAIDataPack.
	windows := ComputeDataWindows(asOf)

	pgFrom := pgtype.Date{Time: from, Valid: true}
	pgTo := pgtype.Date{Time: to, Valid: true}
	sellerAccountID := options.SellerAccountID

	accountMetrics, err := l.queries.ListDailyAccountMetricsBySellerAndDateRange(ctx, dbgen.ListDailyAccountMetricsBySellerAndDateRangeParams{
		SellerAccountID: sellerAccountID,
		MetricDate:      pgFrom,
		MetricDate_2:    pgTo,
	})
	if err != nil {
		return nil, fmt.Errorf("daily_account_metrics: %w", err)
	}

	skuMetrics, err := l.queries.ListDailySKUMetricsBySellerAndDateRange(ctx, dbgen.ListDailySKUMetricsBySellerAndDateRangeParams{
		SellerAccountID: sellerAccountID,
		MetricDate:      pgFrom,
		MetricDate_2:    pgTo,
	})
	if err != nil {
		return nil, fmt.Errorf("daily_sku_metrics: %w", err)
	}

	products, err := l.queries.ListProductsBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		return nil, fmt.Errorf("products: %w", err)
	}

	productInfo, err := l.queries.ListOzonProductInfoSnapshotsBySellerAndDate(ctx, dbgen.ListOzonProductInfoSnapshotsBySellerAndDateParams{
		SellerAccountID: sellerAccountID,
		SnapshotDate:    pgDate,
	})
	if err != nil {
		return nil, fmt.Errorf("ozon_product_info_snapshots: %w", err)
	}

	relatedSKUs, err := l.queries.ListOzonRelatedSKUsBySeller(ctx, sellerAccountID)
	if err != nil {
		return nil, fmt.Errorf("ozon_related_skus: %w", err)
	}

	stockSnapshots, err := l.queries.ListStockAnalyticsSnapshotsBySellerAndDate(ctx, dbgen.ListStockAnalyticsSnapshotsBySellerAndDateParams{
		SellerAccountID: sellerAccountID,
		SnapshotDate:    pgDate,
	})
	if err != nil {
		return nil, fmt.Errorf("stock_analytics_snapshots: %w", err)
	}

	campaigns, err := l.queries.ListAdCampaignsBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		return nil, fmt.Errorf("ad_campaigns: %w", err)
	}

	adMetrics, err := l.queries.ListAdMetricsDailyBySellerAndDateRange(ctx, dbgen.ListAdMetricsDailyBySellerAndDateRangeParams{
		SellerAccountID: sellerAccountID,
		DateFrom:        pgFrom,
		DateTo:          pgTo,
	})
	if err != nil {
		return nil, fmt.Errorf("ad_metrics_daily: %w", err)
	}

	campaignSKUs, err := l.queries.ListAdCampaignSKUMappingsBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		return nil, fmt.Errorf("ad_campaign_skus: %w", err)
	}

	funnelAll, err := l.queries.ListProductAnalyticsDailyAccountScopeByDateRange(ctx, dbgen.ListProductAnalyticsDailyAccountScopeByDateRangeParams{
		SellerAccountID: sellerAccountID,
		MetricDate:      pgFrom,
		MetricDate_2:    pgTo,
	})
	if err != nil {
		return nil, fmt.Errorf("product_analytics_daily: %w", err)
	}
	funnelRows := SelectAccountFunnelRows(funnelAll)

	categories, err := l.queries.ListDescriptionCategoriesAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("description_categories: %w", err)
	}

	priceConstraints, err := l.queries.ListSKUEffectiveConstraintsBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		return nil, fmt.Errorf("sku_effective_constraints: %w", err)
	}

	apiCaps, err := l.queries.GetOzonAPICapabilitiesBySeller(ctx, sellerAccountID)
	if err != nil {
		return nil, fmt.Errorf("ozon_api_capabilities: %w", err)
	}

	var enrichmentRun *dbgen.OzonEnrichmentSyncRun
	run, err := l.queries.GetLatestOzonEnrichmentSyncRunBySellerAndDate(ctx, dbgen.GetLatestOzonEnrichmentSyncRunBySellerAndDateParams{
		SellerAccountID: sellerAccountID,
		MetricDate:      pgDate,
	})
	if err == nil {
		enrichmentRun = &run
	}

	return &AnalysisDataSnapshot{
		SellerAccountID: sellerAccountID,
		MetricDate:      pgDate,
		Windows:         windows,
		LoadedHistory: LoadedHistory{
			From: FormatDate(from),
			To:   FormatDate(to),
			Days: historyDays,
		},
		EnrichmentSource:     DetectEnrichmentSource(enrichmentRun),
		AccountMetrics:       accountMetrics,
		SKUMetrics:           skuMetrics,
		Products:             products,
		ProductInfoSnapshots: productInfo,
		RelatedSKUs:          relatedSKUs,
		StockSnapshots:       stockSnapshots,
		Campaigns:            campaigns,
		AdMetrics:            adMetrics,
		CampaignSKUs:         campaignSKUs,
		FunnelRows:           funnelRows,
		Categories:           categories,
		PriceConstraints:     priceConstraints,
		EnrichmentRun:        enrichmentRun,
		APICapabilities:      apiCaps,
	}, nil
}
