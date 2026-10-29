package devseed

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/productanalyticssync"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type mvpFunnelDayKind int

const (
	mvpFunnelDayNormal mvpFunnelDayKind = iota
	mvpFunnelDayWeakViewToCart
	mvpFunnelDayWeakCartToOrder
	mvpFunnelDayNoTraffic
	mvpFunnelDayHighTrafficLowOrders
)

// SeedMVPProductAnalytics upserts account- and product-level funnel rows for the seed date range.
func SeedMVPProductAnalytics(ctx context.Context, pool *pgxpool.Pool, sellerAccountID int64, opts MVPSeedOptions) (*ProductAnalyticsSeedStats, error) {
	anchor := MVPDateUTC(opts.AnchorDate)
	dateRange, err := SeedDateRange(anchor, opts.Days)
	if err != nil {
		return nil, err
	}

	q := dbgen.New(pool)
	ordersByDay, revenueByDay, err := loadMVPAccountOrdersByDay(ctx, pool, sellerAccountID, dateRange[0], anchor)
	if err != nil {
		return nil, err
	}

	rng := rand.New(rand.NewSource(opts.Seed + sellerAccountID*53 + 9001))
	stats := &ProductAnalyticsSeedStats{}

	for dayIdx, day := range dateRange {
		kind := mvpFunnelDayKindForIndex(dayIdx, len(dateRange), day.Equal(anchor))
		orders := ordersByDay[day.Format("2006-01-02")]
		rev := revenueByDay[day.Format("2006-01-02")]

		if err := upsertMVPAccountFunnelDay(ctx, q, sellerAccountID, day, kind, orders, rev, rng); err != nil {
			return nil, err
		}
		stats.AccountRows++
		stats.DistinctDates++
		if day.Equal(anchor) {
			stats.HasAnchorRow = true
			stats.AnchorSourceStatus = productanalyticssync.SourceStatusAvailable
		}
	}

	products, err := q.ListProductsBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		return nil, fmt.Errorf("list products for funnel sku seed: %w", err)
	}
	if len(products) > 0 {
		if err := seedMVPProductScopeFunnelSamples(ctx, q, sellerAccountID, anchor, products, rng); err != nil {
			return nil, err
		}
		stats.ProductScopeRows = 4
	}

	stats.FunnelAvailableForAnchor, err = MVPAnchorFunnelAvailable(ctx, pool, sellerAccountID, anchor)
	if err != nil {
		return nil, err
	}
	return stats, nil
}

func loadMVPAccountOrdersByDay(
	ctx context.Context,
	pool *pgxpool.Pool,
	sellerAccountID int64,
	from, to time.Time,
) (ordersByDay map[string]int64, revenueByDay map[string]float64, err error) {
	ordersByDay = make(map[string]int64)
	revenueByDay = make(map[string]float64)
	rows, err := pool.Query(ctx, `
SELECT metric_date, orders_count, revenue::float8
FROM daily_account_metrics
WHERE seller_account_id = $1
  AND metric_date >= $2::date
  AND metric_date <= $3::date
ORDER BY metric_date`, sellerAccountID, MVPDateUTC(from), MVPDateUTC(to))
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var d pgtype.Date
		var orders int32
		var rev float64
		if scanErr := rows.Scan(&d, &orders, &rev); scanErr != nil {
			return nil, nil, scanErr
		}
		if !d.Valid {
			continue
		}
		key := MVPDateUTC(d.Time).Format("2006-01-02")
		ordersByDay[key] = int64(orders)
		revenueByDay[key] = rev
	}
	return ordersByDay, revenueByDay, rows.Err()
}

func mvpFunnelDayKindForIndex(dayIdx, total int, isAnchor bool) mvpFunnelDayKind {
	if isAnchor {
		return mvpFunnelDayNormal
	}
	switch dayIdx % 11 {
	case 3:
		return mvpFunnelDayWeakViewToCart
	case 5:
		return mvpFunnelDayWeakCartToOrder
	case 7:
		return mvpFunnelDayNoTraffic
	case 9:
		return mvpFunnelDayHighTrafficLowOrders
	default:
		return mvpFunnelDayNormal
	}
}

func upsertMVPAccountFunnelDay(
	ctx context.Context,
	q *dbgen.Queries,
	sellerAccountID int64,
	day time.Time,
	kind mvpFunnelDayKind,
	orders int64,
	revenue float64,
	rng *rand.Rand,
) error {
	day = MVPDateUTC(day)
	if kind == mvpFunnelDayNoTraffic {
		_, err := q.UpsertProductAnalyticsDaily(ctx, dbgen.UpsertProductAnalyticsDailyParams{
			SellerAccountID: sellerAccountID,
			MetricDate:      pgDate(day),
			ScopeType:       "account",
			ScopeID:         "",
			Impressions:     pgtype.Int8{Valid: false},
			ProductViews:    pgtype.Int8{Valid: false},
			AddToCart:       pgtype.Int8{Valid: false},
			OrdersCount:     pgtype.Int8{Valid: false},
			CompletedOrders: pgtype.Int8{Valid: false},
			Revenue:         pgtype.Numeric{Valid: false},
			RawPayloadID:    pgtype.Int8{Valid: false},
			SourceStatus:    textNV(productanalyticssync.SourceStatusNotEnoughData),
			SourceMessage:   textNV("Нет трафика за день (demo seed)."),
		})
		return err
	}

	impr, views, cart, ord, completed, rev := mvpSynthesizeFunnelMetrics(kind, orders, revenue, rng)
	_, err := q.UpsertProductAnalyticsDaily(ctx, dbgen.UpsertProductAnalyticsDailyParams{
		SellerAccountID: sellerAccountID,
		MetricDate:      pgDate(day),
		ScopeType:       "account",
		ScopeID:         "",
		Impressions:     int8NV(impr),
		ProductViews:    int8NV(views),
		AddToCart:       int8NV(cart),
		OrdersCount:     int8NV(ord),
		CompletedOrders: int8NV(completed),
		Revenue:         moneyNV(rev),
		RawPayloadID:    pgtype.Int8{Valid: false},
		SourceStatus:    textNV(productanalyticssync.SourceStatusAvailable),
		SourceMessage:   pgtype.Text{Valid: false},
	})
	return err
}

func mvpSynthesizeFunnelMetrics(kind mvpFunnelDayKind, orders int64, revenue float64, rng *rand.Rand) (impr, views, cart, ord, completed int64, rev float64) {
	// Funnel orders MUST equal account metrics orders for the seller-1 acceptance dataset.
	// Never invent independent order counts (section 11).
	ord = orders
	if ord < 0 {
		ord = 0
	}

	if ord == 0 {
		// Zero-order days keep zero conversions; light top-of-funnel traffic is allowed.
		impr = 80 + int64(rng.Intn(40))
		views = 8 + int64(rng.Intn(12))
		if views > impr {
			views = impr
		}
		cart = 0
		completed = 0
		rev = 0
		if kind == mvpFunnelDayHighTrafficLowOrders {
			impr = 400 + int64(rng.Intn(200))
			views = 60 + int64(rng.Intn(40))
		}
		return impr, views, cart, ord, completed, 0
	}

	completed = int64(math.Round(float64(ord) * (0.86 + 0.06*rng.Float64())))
	if completed > ord {
		completed = ord
	}
	if completed < 1 {
		completed = 1
	}

	// Build stages upward from orders so monotonicity holds without changing ord.
	orderFromCart := 0.35 + 0.12*rng.Float64()
	cartRate := 0.22 + 0.1*rng.Float64()
	viewRate := 0.13 + 0.04*rng.Float64()
	baseMult := 42.0 + 8*rng.Float64()

	switch kind {
	case mvpFunnelDayWeakViewToCart:
		cartRate = 0.04 + 0.02*rng.Float64()
	case mvpFunnelDayWeakCartToOrder:
		orderFromCart = 0.06 + 0.03*rng.Float64()
	case mvpFunnelDayHighTrafficLowOrders:
		// Documented demo semantics: more top-funnel traffic for the same account orders.
		baseMult = 95 + 15*rng.Float64()
	case mvpFunnelDayNormal:
		// defaults
	}

	if orderFromCart < 0.05 {
		orderFromCart = 0.05
	}
	cart = int64(math.Ceil(float64(ord) / orderFromCart))
	if cart < ord {
		cart = ord
	}
	if cartRate < 0.02 {
		cartRate = 0.02
	}
	views = int64(math.Ceil(float64(cart) / cartRate))
	if views < cart {
		views = cart
	}
	if viewRate < 0.05 {
		viewRate = 0.05
	}
	impr = int64(math.Ceil(float64(views) / viewRate))
	if impr < views {
		impr = views
	}
	// Ensure impressions stay well above views for demo realism.
	minImpr := int64(math.Round(float64(ord) * baseMult))
	if impr < minImpr {
		impr = minImpr
	}

	impr, views, cart, ord, completed = mvpEnforceFunnelMonotonicKeepOrders(impr, views, cart, ord, completed)

	rev = revenue
	if rev <= 0 && ord > 0 {
		rev = float64(ord) * (900 + rng.Float64()*400)
	}
	if ord == 0 {
		rev = 0
	}
	return impr, views, cart, ord, completed, mvpRoundMoney(rev)
}

// mvpEnforceFunnelMonotonicKeepOrders enforces impressions >= views >= cart >= orders >= delivered
// without inventing or reducing the account-derived orders count.
func mvpEnforceFunnelMonotonicKeepOrders(impr, views, cart, ord, completed int64) (int64, int64, int64, int64, int64) {
	if ord < 0 {
		ord = 0
	}
	if completed > ord {
		completed = ord
	}
	if completed < 0 {
		completed = 0
	}
	if cart < ord {
		cart = ord
	}
	if views < cart {
		views = cart
	}
	if impr < views {
		impr = views
	}
	return impr, views, cart, ord, completed
}

func seedMVPProductScopeFunnelSamples(
	ctx context.Context,
	q *dbgen.Queries,
	sellerAccountID int64,
	anchor time.Time,
	products []dbgen.Product,
	rng *rand.Rand,
) error {
	if len(products) == 0 {
		return nil
	}
	pick := func(i int) dbgen.Product {
		return products[i%len(products)]
	}

	samples := []struct {
		productIdx int
		kind       mvpFunnelDayKind
	}{
		{0, mvpFunnelDayNormal},
		{len(products) / 3, mvpFunnelDayWeakViewToCart},
		{len(products) / 2, mvpFunnelDayWeakCartToOrder},
		{len(products) - 1, mvpFunnelDayNoTraffic},
	}
	for _, s := range samples {
		p := pick(s.productIdx)
		scopeID := fmt.Sprintf("%d", p.OzonProductID)
		if s.kind == mvpFunnelDayNoTraffic {
			_, err := q.UpsertProductAnalyticsDaily(ctx, dbgen.UpsertProductAnalyticsDailyParams{
				SellerAccountID: sellerAccountID,
				MetricDate:      pgDate(anchor),
				ScopeType:       "product",
				ScopeID:         scopeID,
				SourceStatus:    textNV(productanalyticssync.SourceStatusNotEnoughData),
				SourceMessage:   textNV("SKU без трафика за день (demo seed)."),
			})
			if err != nil {
				return err
			}
			continue
		}
		impr, views, cart, ord, completed, rev := mvpSynthesizeFunnelMetrics(s.kind, 8+int64(rng.Intn(20)), 0, rng)
		_, err := q.UpsertProductAnalyticsDaily(ctx, dbgen.UpsertProductAnalyticsDailyParams{
			SellerAccountID: sellerAccountID,
			MetricDate:      pgDate(anchor),
			ScopeType:       "product",
			ScopeID:         scopeID,
			Impressions:     int8NV(impr),
			ProductViews:    int8NV(views),
			AddToCart:       int8NV(cart),
			OrdersCount:     int8NV(ord),
			CompletedOrders: int8NV(completed),
			Revenue:         moneyNV(rev),
			SourceStatus:    textNV(productanalyticssync.SourceStatusAvailable),
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// MVPAnchorFunnelAvailable reports whether dashboard home can show sales_funnel as available on anchorDate.
func MVPAnchorFunnelAvailable(ctx context.Context, pool *pgxpool.Pool, sellerAccountID int64, anchorDate time.Time) (bool, error) {
	q := dbgen.New(pool)
	row, err := q.GetProductAnalyticsDailyBySellerDateScope(ctx, dbgen.GetProductAnalyticsDailyBySellerDateScopeParams{
		SellerAccountID: sellerAccountID,
		MetricDate:      pgDate(MVPDateUTC(anchorDate)),
		ScopeType:       "account",
		ScopeID:         "",
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	if row.SourceStatus.Valid {
		st := row.SourceStatus.String
		if st != "" && st != productanalyticssync.SourceStatusAvailable {
			return false, nil
		}
	}
	if !row.Impressions.Valid || row.Impressions.Int64 <= 0 {
		return false, nil
	}
	if !row.ProductViews.Valid || row.ProductViews.Int64 <= 0 {
		return false, nil
	}
	if !row.AddToCart.Valid || row.AddToCart.Int64 <= 0 {
		return false, nil
	}
	if !row.OrdersCount.Valid || row.OrdersCount.Int64 <= 0 {
		return false, nil
	}
	return true, nil
}

func pgDate(t time.Time) pgtype.Date {
	t = MVPDateUTC(t)
	return pgtype.Date{Time: t, Valid: true}
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
