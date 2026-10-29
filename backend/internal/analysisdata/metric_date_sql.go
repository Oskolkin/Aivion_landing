package analysisdata

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SQLCoverageSource probes Analysis Pack dataset presence via SQL.
type SQLCoverageSource struct {
	pool *pgxpool.Pool
}

// NewSQLCoverageSource creates a CoverageSource backed by PostgreSQL.
func NewSQLCoverageSource(pool *pgxpool.Pool) *SQLCoverageSource {
	if pool == nil {
		panic("analysisdata: pool must not be nil")
	}
	return &SQLCoverageSource{pool: pool}
}

// DatasetDates implements CoverageSource.
func (s *SQLCoverageSource) DatasetDates(ctx context.Context, sellerAccountID int64, dataset string, from, to time.Time) ([]string, error) {
	from = DateOnlyUTC(from)
	to = DateOnlyUTC(to)
	query, err := coverageQuery(dataset)
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, query, sellerAccountID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var d time.Time
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		out = append(out, FormatDate(d))
	}
	return out, rows.Err()
}

// HasCampaigns implements CoverageSource.
func (s *SQLCoverageSource) HasCampaigns(ctx context.Context, sellerAccountID int64) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `
SELECT EXISTS(
  SELECT 1 FROM ad_campaigns WHERE seller_account_id = $1 LIMIT 1
)`, sellerAccountID).Scan(&exists)
	return exists, err
}

func coverageQuery(dataset string) (string, error) {
	switch dataset {
	case DatasetOrdersSales:
		return `
SELECT DISTINCT metric_date::date
FROM daily_account_metrics
WHERE seller_account_id = $1
  AND metric_date >= $2::date
  AND metric_date <= $3::date
ORDER BY 1`, nil
	case DatasetDailySKUMetrics:
		return `
SELECT DISTINCT metric_date::date
FROM daily_sku_metrics
WHERE seller_account_id = $1
  AND metric_date >= $2::date
  AND metric_date <= $3::date
ORDER BY 1`, nil
	case DatasetProductAnalytics, DatasetFunnelAccount:
		return `
SELECT DISTINCT metric_date::date
FROM product_analytics_daily
WHERE seller_account_id = $1
  AND metric_date >= $2::date
  AND metric_date <= $3::date
  AND scope_type = 'account'
ORDER BY 1`, nil
	case DatasetStocks:
		return `
SELECT DISTINCT snapshot_date::date
FROM stock_analytics_snapshots
WHERE seller_account_id = $1
  AND snapshot_date >= $2::date
  AND snapshot_date <= $3::date
ORDER BY 1`, nil
	case DatasetAdvertising, DatasetCampaignMetrics:
		return `
SELECT DISTINCT metric_date::date
FROM ad_metrics_daily
WHERE seller_account_id = $1
  AND metric_date >= $2::date
  AND metric_date <= $3::date
ORDER BY 1`, nil
	case DatasetProductEnrichment:
		return `
SELECT DISTINCT snapshot_date::date
FROM ozon_product_info_snapshots
WHERE seller_account_id = $1
  AND snapshot_date >= $2::date
  AND snapshot_date <= $3::date
ORDER BY 1`, nil
	default:
		return "", fmt.Errorf("unknown coverage dataset %q", dataset)
	}
}
