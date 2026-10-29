package analytics

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CriticalSKUService struct {
	queries      *dbgen.Queries
	stocksViewer *StocksViewService
}

func NewCriticalSKUService(db *pgxpool.Pool) *CriticalSKUService {
	return &CriticalSKUService{
		queries:      dbgen.New(db),
		stocksViewer: NewStocksViewService(db),
	}
}

type CriticalSKUCard struct {
	OzonProductID int64   `json:"ozon_product_id"`
	OfferID       *string `json:"offer_id"`
	SKU           *int64  `json:"sku"`
	ProductName   *string `json:"product_name"`

	Revenue         float64 `json:"revenue"`
	SalesOps        int32   `json:"sales_ops"`
	RevenueDeltaDay float64 `json:"revenue_delta_day"`
	OrdersDeltaDay  int32   `json:"orders_delta_day"`

	StockAvailable int32    `json:"stock_available"`
	DaysOfCover    *float64 `json:"days_of_cover"`

	Importance     float64  `json:"importance"`
	OutOfStockRisk float64  `json:"out_of_stock_risk"`
	ProblemScore   float64  `json:"problem_score"`
	Signals        []string `json:"signals"`
	Badges         []string `json:"badges"`

	SKUOrdersShare           *float64 `json:"sku_orders_share,omitempty"`
	SKURevenueShare          *float64 `json:"sku_revenue_share,omitempty"`
	SKUSalesVelocityOrders7D *float64 `json:"sku_sales_velocity_orders_7d,omitempty"`
	SKUOrdersDeltaPercent    *float64 `json:"sku_orders_delta_percent,omitempty"`
	DiscountDepth            *float64 `json:"discount_depth,omitempty"`
	OzonIDC                  *int32   `json:"ozon_idc,omitempty"`
	StockTurnoverGrade       *string  `json:"stock_turnover_grade,omitempty"`
	ExcessStockShare         *float64 `json:"excess_stock_share,omitempty"`
	StockStatusBucket        *string  `json:"stock_status_bucket,omitempty"`
	StockStatusBucketSource  *string  `json:"stock_status_bucket_source,omitempty"`
}

type CriticalSKUResult struct {
	SellerAccountID     int64             `json:"seller_account_id"`
	AsOfDate            string            `json:"as_of_date"`
	LatestDataTimestamp *string           `json:"latest_data_timestamp"`
	StockSemantics      string            `json:"stock_semantics"`
	ScoringSemantics    map[string]string `json:"scoring_semantics"`
	Items               []CriticalSKUCard `json:"items"`
}

func (s *CriticalSKUService) ListCriticalSKUsForSellerAccount(
	ctx context.Context,
	sellerAccountID int64,
	asOfDate time.Time,
) (CriticalSKUResult, error) {
	asOf := normalizeDate(asOfDate)
	yesterday := asOf.AddDate(0, 0, -1)

	if _, err := s.queries.GetSellerAccountByID(ctx, sellerAccountID); err != nil {
		return CriticalSKUResult{}, fmt.Errorf("get seller account: %w", err)
	}

	rows, err := s.queries.ListDailySKUMetricsBySellerAndDateRange(ctx, dbgen.ListDailySKUMetricsBySellerAndDateRangeParams{
		SellerAccountID: sellerAccountID,
		MetricDate:      dateValue(yesterday),
		MetricDate_2:    dateValue(asOf),
	})
	if err != nil {
		return CriticalSKUResult{}, fmt.Errorf("list sku metrics by date range: %w", err)
	}

	todayKey := asOf.Format("2006-01-02")
	yesterdayKey := yesterday.Format("2006-01-02")
	todayRows := make([]dbgen.DailySkuMetric, 0)
	yesterdayByProduct := make(map[int64]dbgen.DailySkuMetric)
	for _, row := range rows {
		key := row.MetricDate.Time.Format("2006-01-02")
		if key == todayKey {
			todayRows = append(todayRows, row)
		}
		if key == yesterdayKey {
			yesterdayByProduct[row.OzonProductID] = row
		}
	}

	var accountRevenue float64
	for _, row := range todayRows {
		accountRevenue += numericToFloatCritical(row.Revenue)
	}
	accountRevenue = round2Critical(accountRevenue)

	stockRows, err := s.stocksViewer.ListCurrentStocksBySellerAccount(ctx, sellerAccountID)
	if err != nil {
		return CriticalSKUResult{}, fmt.Errorf("list current stocks: %w", err)
	}
	stockByProduct := make(map[int64]CurrentStockProductRow, len(stockRows))
	var latestDataTimestamp *string
	for _, row := range stockRows {
		stockByProduct[row.OzonProductID] = row
		latestDataTimestamp = maxRFC3339Ptr(latestDataTimestamp, row.SnapshotAt)
	}

	var todayAccount dbgen.DailyAccountMetric
	accountRows, err := s.queries.ListDailyAccountMetricsBySellerAndDateRange(ctx, dbgen.ListDailyAccountMetricsBySellerAndDateRangeParams{
		SellerAccountID: sellerAccountID,
		MetricDate:      dateValue(asOf),
		MetricDate_2:    dateValue(asOf),
	})
	if err != nil {
		return CriticalSKUResult{}, fmt.Errorf("list account metrics for enrichment: %w", err)
	}
	if len(accountRows) > 0 {
		todayAccount = accountRows[0]
	}
	enrichCtx, err := loadSKUEnrichmentContext(ctx, s.queries, sellerAccountID, asOf, todayAccount)
	if err != nil {
		return CriticalSKUResult{}, err
	}

	cards := make([]CriticalSKUCard, 0, len(todayRows))
	for _, today := range todayRows {
		yday := yesterdayByProduct[today.OzonProductID]

		revenue := numericToFloatCritical(today.Revenue)
		revenueDelta := round2Critical(revenue - numericToFloatCritical(yday.Revenue))
		ordersDelta := today.OrdersCount - yday.OrdersCount
		daysCover := numericToFloatPtrCritical(today.DaysOfCover)

		stock := stockByProduct[today.OzonProductID]
		stockAvailable := today.StockAvailable
		if stock.OzonProductID != 0 {
			stockAvailable = stock.AvailableStock
		}

		importance, importanceSignals := calcImportance(revenue, accountRevenue, revenueDelta)
		outRisk, stockSignals := calcOutOfStockRisk(stockAvailable, daysCover)
		salesSignalsScore, salesSignals := calcSalesDeclineSignals(revenueDelta, revenue)
		ordersSignalsScore, ordersSignals := calcOrdersDeclineSignals(ordersDelta)

		stockPoints := outRisk * 5
		importancePoints := importance * 3
		problemScore := round2Critical(salesSignalsScore + ordersSignalsScore + stockPoints + importancePoints)

		signals := append([]string{}, salesSignals...)
		signals = append(signals, ordersSignals...)
		signals = append(signals, stockSignals...)
		signals = append(signals, importanceSignals...)

		badges := buildBadges(outRisk, importance, revenueDelta, ordersDelta)

		item := DashboardSKUMetricsItem{
			OzonProductID:  today.OzonProductID,
			Revenue:        revenue,
			OrdersCount:    today.OrdersCount,
			StockAvailable: stockAvailable,
			DaysOfCover:    daysCover,
		}
		enrichSKUMetricsItem(&item, enrichCtx, yday.OrdersCount)

		cards = append(cards, CriticalSKUCard{
			OzonProductID:            today.OzonProductID,
			OfferID:                  textPtr(today.OfferID),
			SKU:                      int8Ptr(today.Sku),
			ProductName:              textPtr(today.ProductName),
			Revenue:                  revenue,
			SalesOps:                 today.OrdersCount,
			RevenueDeltaDay:          revenueDelta,
			OrdersDeltaDay:           ordersDelta,
			StockAvailable:           stockAvailable,
			DaysOfCover:              daysCover,
			Importance:               importance,
			OutOfStockRisk:           outRisk,
			ProblemScore:             problemScore,
			Signals:                  uniqStrings(signals),
			Badges:                   uniqStrings(badges),
			SKUOrdersShare:           item.SKUOrdersShare,
			SKURevenueShare:          item.SKURevenueShare,
			SKUSalesVelocityOrders7D: item.SKUSalesVelocityOrders7D,
			SKUOrdersDeltaPercent:    item.SKUOrdersDeltaPercent,
			DiscountDepth:            item.DiscountDepth,
			OzonIDC:                  item.OzonIDC,
			StockTurnoverGrade:       item.StockTurnoverGrade,
			ExcessStockShare:         item.ExcessStockShare,
			StockStatusBucket:        item.StockStatusBucket,
			StockStatusBucketSource:  item.StockStatusBucketSource,
		})
	}

	sort.Slice(cards, func(i, j int) bool {
		if cards[i].ProblemScore == cards[j].ProblemScore {
			return cards[i].OzonProductID < cards[j].OzonProductID
		}
		return cards[i].ProblemScore > cards[j].ProblemScore
	})

	return CriticalSKUResult{
		SellerAccountID:     sellerAccountID,
		AsOfDate:            todayKey,
		LatestDataTimestamp: latestDataTimestamp,
		StockSemantics:      "Current-state snapshot from latest product+warehouse rows; not stock event history.",
		ScoringSemantics: map[string]string{
			"sales_change":          "Revenue delta day-to-day points: strong drop +3, moderate +2, mild +1, else 0.",
			"orders_change":         "Sales ops delta day-to-day points: strong drop +2, moderate +1, else 0.",
			"stock_signal":          "Stock points follow stage-4 thresholds: out-of-stock strongest, low-stock medium.",
			"importance_signal":     "Importance is revenue-share and positive contribution context score.",
			"problem_score_formula": "problem_score = sales_points + orders_points + stock_points + importance_points.",
		},
		Items: cards,
	}, nil
}

func maxRFC3339Ptr(current *string, next *string) *string {
	if next == nil || *next == "" {
		return current
	}
	nextParsed, err := time.Parse(time.RFC3339, *next)
	if err != nil {
		return current
	}
	if current == nil || *current == "" {
		s := nextParsed.UTC().Format(time.RFC3339)
		return &s
	}
	currentParsed, err := time.Parse(time.RFC3339, *current)
	if err != nil || nextParsed.After(currentParsed) {
		s := nextParsed.UTC().Format(time.RFC3339)
		return &s
	}
	return current
}

func calcSalesDeclineSignals(revenueDelta float64, currentRevenue float64) (float64, []string) {
	if revenueDelta >= 0 {
		return 0, nil
	}
	dropRatio := 0.0
	if currentRevenue > 0 {
		dropRatio = math.Abs(revenueDelta) / currentRevenue
	}
	switch {
	case dropRatio >= 0.5 || math.Abs(revenueDelta) >= 3000:
		return 3, []string{"sales_drop_strong"}
	case dropRatio >= 0.25 || math.Abs(revenueDelta) >= 1200:
		return 2, []string{"sales_drop_moderate"}
	default:
		return 1, []string{"sales_drop_mild"}
	}
}

func calcOrdersDeclineSignals(ordersDelta int32) (float64, []string) {
	if ordersDelta >= 0 {
		return 0, nil
	}
	if ordersDelta <= -3 {
		return 2, []string{"sales_ops_drop_strong"}
	}
	return 1, []string{"sales_ops_drop_moderate"}
}

func calcOutOfStockRisk(stockAvailable int32, daysCover *float64) (float64, []string) {
	// Stage-4 docs explicit thresholds: 0 critical, 1-3 low, >3 normal.
	if stockAvailable <= 0 {
		return 1.0, []string{"out_of_stock"}
	}
	if stockAvailable <= 3 {
		return 0.6, []string{"low_stock"}
	}

	if daysCover != nil {
		switch {
		case *daysCover <= 1:
			return 0.8, []string{"days_of_cover_critical"}
		case *daysCover <= 3:
			return 0.4, []string{"days_of_cover_low"}
		}
	}
	return 0.0, nil
}

func calcImportance(revenue float64, accountRevenue float64, contributionDelta float64) (float64, []string) {
	share := 0.0
	if accountRevenue > 0 {
		share = revenue / accountRevenue
	}

	score := 0.0
	signals := make([]string, 0, 2)
	switch {
	case share >= 0.20:
		score += 1.0
		signals = append(signals, "importance_high_share")
	case share >= 0.10:
		score += 0.7
		signals = append(signals, "importance_medium_share")
	case share >= 0.05:
		score += 0.4
	}

	if contributionDelta > 0 {
		score += 0.2
	}
	if score > 1 {
		score = 1
	}
	return round2Critical(score), signals
}

func buildBadges(outRisk float64, importance float64, revenueDelta float64, ordersDelta int32) []string {
	badges := make([]string, 0, 4)
	if outRisk >= 1 {
		badges = append(badges, "OUT_OF_STOCK")
	} else if outRisk >= 0.6 {
		badges = append(badges, "LOW_STOCK")
	}
	if importance >= 0.7 {
		badges = append(badges, "HIGH_IMPORTANCE")
	}
	if revenueDelta < 0 {
		badges = append(badges, "REVENUE_DOWN")
	}
	if ordersDelta < 0 {
		badges = append(badges, "SALES_OPS_DOWN")
	}
	return badges
}

func uniqStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func numericToFloatCritical(n pgtype.Numeric) float64 {
	if !n.Valid {
		return 0
	}
	v, err := n.Float64Value()
	if err != nil || !v.Valid {
		return 0
	}
	return round2Critical(v.Float64)
}

func numericToFloatPtrCritical(n pgtype.Numeric) *float64 {
	if !n.Valid {
		return nil
	}
	v, err := n.Float64Value()
	if err != nil || !v.Valid {
		return nil
	}
	value := round2Critical(v.Float64)
	return &value
}

func round2Critical(v float64) float64 {
	return math.Round(v*100) / 100
}
