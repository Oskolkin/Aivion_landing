package devseed

import (
	"context"
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SeedMVP runs the full deterministic MVP source-data seed for one seller account.
func SeedMVP(ctx context.Context, pool *pgxpool.Pool, opts MVPSeedOptions) (*MVPSeedResult, error) {
	if err := AssertDevSeedAllowed(); err != nil {
		return nil, err
	}
	if err := ValidateMVPSeedOptions(opts); err != nil {
		return nil, err
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	q := dbgen.New(tx)

	sellerID, demoEmail, demoPwdUpdated, err := ResolveSellerForMVP(ctx, q, opts)
	if err != nil {
		return nil, err
	}

	adminEmail, adminPwdUpdated, err := EnsureAdminUserIfRequested(ctx, q, opts)
	if err != nil {
		return nil, err
	}
	var adminUserID int64
	if adminEmail != "" {
		au, err := q.GetUserByEmail(ctx, auth.NormalizeEmail(adminEmail))
		if err == nil {
			adminUserID = au.ID
		}
	}

	result := &MVPSeedResult{
		DemoUserEmail:        demoEmail,
		AdminEmail:           adminEmail,
		DemoPasswordUpdated:  demoPwdUpdated,
		AdminPasswordUpdated: adminPwdUpdated,
		SellerAccountID:      sellerID,
	}

	if opts.Reset {
		if err := cleanupMVPSellerData(ctx, tx, sellerID); err != nil {
			return nil, err
		}
	}

	if err := SeedOzonConnectionMock(ctx, q, sellerID, opts.AnchorDate.UTC(), opts.EncryptionKey); err != nil {
		return nil, err
	}

	if _, err := SeedMVPCommerce(ctx, q, sellerID, opts); err != nil {
		return nil, err
	}

	if _, err := SeedMVPAds(ctx, q, sellerID, opts); err != nil {
		return nil, err
	}

	if _, err := SeedMVPPricing(ctx, q, sellerID); err != nil {
		return nil, err
	}

	commerceStats, err := countCommerceLike(ctx, q, sellerID)
	if err != nil {
		return nil, err
	}
	adsStats, err := countAdsLike(ctx, tx, sellerID)
	if err != nil {
		return nil, err
	}

	if err := SeedSellerBillingSupportStub(ctx, q, sellerID, commerceStats.Products); err != nil {
		return nil, err
	}

	if err := SeedMVPSyncArtifacts(ctx, q, sellerID, opts.AnchorDate.UTC(), commerceStats, adsStats); err != nil {
		return nil, err
	}

	if err := SeedMVPAdminMetadataInTx(ctx, q, sellerID, adminUserID, adminEmail, opts.AnchorDate.UTC()); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	if err := PatchLegacyPicsumProductImages(ctx, pool, sellerID); err != nil {
		return nil, fmt.Errorf("patch legacy product images: %w", err)
	}

	// Post-commit derived-metrics steps. If any fails, roll the seller data back with a
	// DB-only compensating cleanup so we never leave a half-built dataset behind, then
	// return the ORIGINAL error. Compensation only runs for full-reset seeds so we never
	// destroy pre-existing (e.g. seller-1 live) data on an append-style seed.
	if err := RebuildMVPAnalytics(ctx, pool, sellerID, opts.AnchorDate, opts.Days); err != nil {
		compensateFailedMVPSeed(ctx, pool, sellerID, opts)
		return nil, err
	}

	// Scale ads after store revenue exists so demo spend share stays realistic.
	if err := scaleDemoAdSpendToStoreShare(ctx, pool, sellerID, MVPDateUTC(opts.AnchorDate)); err != nil {
		compensateFailedMVPSeed(ctx, pool, sellerID, opts)
		return nil, fmt.Errorf("scale demo ad spend: %w", err)
	}

	if _, err := SeedMVPProductAnalytics(ctx, pool, sellerID, opts); err != nil {
		compensateFailedMVPSeed(ctx, pool, sellerID, opts)
		return nil, err
	}

	if err := SeedMVPAdminPostMetricsLog(ctx, pool, sellerID, adminUserID, adminEmail); err != nil {
		return nil, err
	}

	if err := loadMVPSummaryCounts(ctx, pool, sellerID, result); err != nil {
		return nil, err
	}
	if err := LoadMVPDateSummary(ctx, pool, sellerID, opts, result); err != nil {
		return nil, err
	}

	return result, nil
}

// compensateFailedMVPSeed rolls back seeded seller data (DB only) after a post-commit
// derived-metrics failure. It runs cleanupMVPSellerData on a fresh transaction and is a
// no-op unless the seed was a full reset, so append-style seeds against live data are safe.
func compensateFailedMVPSeed(ctx context.Context, pool *pgxpool.Pool, sellerID int64, opts MVPSeedOptions) {
	if !opts.Reset {
		return
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return
	}
	defer tx.Rollback(ctx)
	if err := cleanupMVPSellerData(ctx, tx, sellerID); err != nil {
		return
	}
	_ = tx.Commit(ctx)
}

func countCommerceLike(ctx context.Context, q *dbgen.Queries, sellerID int64) (*CommerceSeedStats, error) {
	p, err := q.CountProductsBySellerAccountID(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	o, err := q.CountOrdersBySellerAccountID(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	s, err := q.CountSalesBySellerAccountID(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	st, err := q.CountStocksBySellerAccountID(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	return &CommerceSeedStats{
		Products: int(p),
		Orders:   int(o),
		Sales:    int(s),
		Stocks:   int(st),
	}, nil
}

func countAdsLike(ctx context.Context, tx pgx.Tx, sellerID int64) (*AdsSeedStats, error) {
	row := tx.QueryRow(ctx, `
SELECT
    (SELECT COUNT(*)::int FROM ad_campaigns WHERE seller_account_id = $1),
    (SELECT COUNT(*)::int FROM ad_metrics_daily WHERE seller_account_id = $1),
    (SELECT COUNT(*)::int FROM ad_campaign_skus WHERE seller_account_id = $1)
`, sellerID)
	var c, m, l int32
	if err := row.Scan(&c, &m, &l); err != nil {
		return nil, err
	}
	return &AdsSeedStats{
		Campaigns:  int(c),
		MetricRows: int(m),
		SKULinks:   int(l),
	}, nil
}

func loadMVPSummaryCounts(ctx context.Context, pool *pgxpool.Pool, sellerID int64, into *MVPSeedResult) error {
	row := pool.QueryRow(ctx, `
SELECT
    (SELECT COUNT(*)::bigint FROM products WHERE seller_account_id = $1),
    (SELECT COUNT(*)::bigint FROM orders WHERE seller_account_id = $1),
    (SELECT COUNT(*)::bigint FROM sales WHERE seller_account_id = $1),
    (SELECT COUNT(*)::bigint FROM stocks WHERE seller_account_id = $1),
    (SELECT COUNT(*)::bigint FROM ad_campaigns WHERE seller_account_id = $1),
    (SELECT COUNT(*)::bigint FROM ad_metrics_daily WHERE seller_account_id = $1),
    (SELECT COUNT(*)::bigint FROM pricing_constraint_rules WHERE seller_account_id = $1),
    (SELECT COUNT(*)::bigint FROM sku_effective_constraints WHERE seller_account_id = $1),
    (SELECT COUNT(*)::bigint FROM sync_jobs WHERE seller_account_id = $1),
    (SELECT COUNT(*)::bigint FROM import_jobs WHERE seller_account_id = $1),
    (SELECT COUNT(*)::bigint FROM daily_account_metrics WHERE seller_account_id = $1),
    (SELECT COUNT(*)::bigint FROM daily_sku_metrics WHERE seller_account_id = $1)
`, sellerID)
	return row.Scan(
		&into.ProductsCount,
		&into.OrdersCount,
		&into.SalesCount,
		&into.StocksCount,
		&into.AdCampaignsCount,
		&into.AdMetricRows,
		&into.PricingRulesCount,
		&into.EffectiveConstraints,
		&into.SyncJobsCount,
		&into.ImportJobsCount,
		&into.AccountMetricRows,
		&into.SKUMetricRows,
	)
}
