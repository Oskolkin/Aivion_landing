package productcatalog

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	DefaultLimit = 50
	MaxLimit     = 200
)

type ListParams struct {
	Search            string
	Limit             int
	Offset            int
	SortBy            string
	HasSalesLastDay   *bool
	StockStatusFilter string
	KeySKUsOnly       bool
}

type SparklinePoint struct {
	Date        string   `json:"date"`
	OrdersCount *int32   `json:"orders_count,omitempty"`
	Revenue     *float64 `json:"revenue,omitempty"`
}

type LastDayMetrics struct {
	Date        string  `json:"date"`
	OrdersCount int32   `json:"orders_count"`
	UnitsSold   int32   `json:"units_sold"`
	Revenue     float64 `json:"revenue"`
}

type StockInfo struct {
	StockAvailable    int32    `json:"stock_available"`
	DaysOfCover       *float64 `json:"days_of_cover,omitempty"`
	StockStatusBucket *string  `json:"stock_status_bucket,omitempty"`
}

type CatalogItem struct {
	SKU          *int64           `json:"sku"`
	OfferID      *string          `json:"offer_id,omitempty"`
	ProductName  string           `json:"product_name"`
	ImageURL     *string          `json:"image_url,omitempty"`
	CurrentPrice *float64         `json:"current_price,omitempty"`
	OldPrice     *float64         `json:"old_price,omitempty"`
	MinPrice     *float64         `json:"min_price,omitempty"`
	LastDay      *LastDayMetrics  `json:"last_day,omitempty"`
	Sparkline7D  []SparklinePoint `json:"sparkline_7d"`
	Stock        *StockInfo       `json:"stock,omitempty"`
	OzonURL      *string          `json:"ozon_url,omitempty"`
}

type ListResult struct {
	Items            []CatalogItem `json:"items"`
	Total            int64         `json:"total"`
	Limit            int           `json:"limit"`
	Offset           int           `json:"offset"`
	LatestMetricDate *string       `json:"latest_metric_date,omitempty"`
}

type Service struct {
	queries   *dbgen.Queries
	dashboard *analytics.DashboardService
}

func NewService(db *pgxpool.Pool, dashboard *analytics.DashboardService) *Service {
	return &Service{queries: dbgen.New(db), dashboard: dashboard}
}

func (s *Service) ListForSeller(ctx context.Context, sellerAccountID int64, params ListParams) (ListResult, error) {
	if params.KeySKUsOnly {
		return s.listKeySKUProducts(ctx, sellerAccountID, params)
	}
	limit := params.Limit
	if limit <= 0 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}
	offset := params.Offset
	if offset < 0 {
		offset = 0
	}

	sortBy := normalizeSortBy(params.SortBy)
	search := strings.TrimSpace(params.Search)
	var searchArg pgtype.Text
	if search != "" {
		searchArg = pgtype.Text{String: search, Valid: true}
	}

	var hasSales pgtype.Bool
	if params.HasSalesLastDay != nil {
		hasSales = pgtype.Bool{Bool: *params.HasSalesLastDay, Valid: true}
	}

	total, err := s.queries.CountProductsCatalogBySeller(ctx, dbgen.CountProductsCatalogBySellerParams{
		SellerAccountID: sellerAccountID,
		Search:          searchArg,
		HasSalesLastDay: hasSales,
	})
	if err != nil {
		return ListResult{}, fmt.Errorf("count products catalog: %w", err)
	}

	rows, err := s.queries.ListProductsCatalogPage(ctx, dbgen.ListProductsCatalogPageParams{
		SellerAccountID: sellerAccountID,
		Search:          searchArg,
		HasSalesLastDay: hasSales,
		SortBy:          sortBy,
		PageLimit:       int32(limit),
		PageOffset:      int32(offset),
	})
	if err != nil {
		return ListResult{}, fmt.Errorf("list products catalog: %w", err)
	}

	if len(rows) == 0 {
		return ListResult{
			Items:  []CatalogItem{},
			Total:  total,
			Limit:  limit,
			Offset: offset,
		}, nil
	}

	var latestDate *time.Time
	if d, ok := parseMetricDate(rows[0].LastMetricDate); ok {
		latestDate = &d
	}

	productIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		productIDs = append(productIDs, row.OzonProductID)
	}

	sparklineByProduct := map[int64][]SparklinePoint{}
	if latestDate != nil {
		sparklineByProduct, err = s.loadSparklines(ctx, sellerAccountID, productIDs, *latestDate)
		if err != nil {
			return ListResult{}, err
		}
	}

	items := make([]CatalogItem, 0, len(rows))
	for _, row := range rows {
		item := mapCatalogRow(row, sparklineByProduct[row.OzonProductID])
		if params.StockStatusFilter != "" && !stockMatchesFilter(item, params.StockStatusFilter) {
			continue
		}
		items = append(items, item)
	}

	var latestMetricDate *string
	if latestDate != nil {
		formatted := latestDate.Format("2006-01-02")
		latestMetricDate = &formatted
	}

	return ListResult{
		Items:            items,
		Total:            total,
		Limit:            limit,
		Offset:           offset,
		LatestMetricDate: latestMetricDate,
	}, nil
}

func (s *Service) loadSparklines(
	ctx context.Context,
	sellerAccountID int64,
	productIDs []int64,
	latestDate time.Time,
) (map[int64][]SparklinePoint, error) {
	from := latestDate.AddDate(0, 0, -6)
	metricRows, err := s.queries.ListDailySKUMetricsBySellerProductsAndDateRange(ctx, dbgen.ListDailySKUMetricsBySellerProductsAndDateRangeParams{
		SellerAccountID: sellerAccountID,
		MetricDate:      dateValue(from),
		MetricDate_2:    dateValue(latestDate),
		Column4:         productIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("list sku metrics for sparkline: %w", err)
	}

	byProductDate := make(map[int64]map[string]dbgen.DailySkuMetric)
	for _, row := range metricRows {
		key := row.MetricDate.Time.Format("2006-01-02")
		if byProductDate[row.OzonProductID] == nil {
			byProductDate[row.OzonProductID] = make(map[string]dbgen.DailySkuMetric)
		}
		byProductDate[row.OzonProductID][key] = row
	}

	out := make(map[int64][]SparklinePoint, len(productIDs))
	for _, productID := range productIDs {
		productMetrics := byProductDate[productID]
		if productMetrics == nil {
			productMetrics = map[string]dbgen.DailySkuMetric{}
		}
		out[productID] = buildSparklineSeries(from, latestDate, productMetrics)
	}
	return out, nil
}

// buildSparklineSeries returns one point per calendar day in [from, latestDate].
// OrdersCount and Revenue are set only when daily_sku_metrics has a row for that day.
// Missing rows are omitted (not zero-filled) because rollup creates rows on sales activity,
// not for every SKU on every day.
func buildSparklineSeries(from, latestDate time.Time, metricsByDate map[string]dbgen.DailySkuMetric) []SparklinePoint {
	points := make([]SparklinePoint, 0, 7)
	for d := from; !d.After(latestDate); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		pt := SparklinePoint{Date: key}
		if metric, ok := metricsByDate[key]; ok {
			orders := metric.OrdersCount
			revenue := numericToFloat(metric.Revenue)
			pt.OrdersCount = &orders
			pt.Revenue = &revenue
		}
		points = append(points, pt)
	}
	return points
}

func mapCatalogRow(row dbgen.ListProductsCatalogPageRow, sparkline []SparklinePoint) CatalogItem {
	item := CatalogItem{
		ProductName: row.Name,
		Sparkline7D: sparkline,
	}
	if sparkline == nil {
		item.Sparkline7D = []SparklinePoint{}
	}

	if row.Sku.Valid {
		v := row.Sku.Int64
		item.SKU = &v
	}
	if row.OfferID.Valid && row.OfferID.String != "" {
		v := row.OfferID.String
		item.OfferID = &v
	}
	if row.PrimaryImageUrl.Valid && row.PrimaryImageUrl.String != "" {
		v := row.PrimaryImageUrl.String
		item.ImageURL = &v
	}
	item.CurrentPrice = numericToFloatPtr(row.ReferencePrice)
	item.OldPrice = numericToFloatPtr(row.OldPrice)
	item.MinPrice = numericToFloatPtr(row.OzonMinPrice)

	if metricDate, ok := parseMetricDate(row.LastMetricDate); ok {
		hasMetrics := row.LastDayOrdersCount.Valid || row.LastDayRevenue.Valid || row.LastDayUnitsSold.Valid
		if hasMetrics {
			last := &LastDayMetrics{
				Date: metricDate.Format("2006-01-02"),
			}
			if row.LastDayOrdersCount.Valid {
				last.OrdersCount = row.LastDayOrdersCount.Int32
			}
			if row.LastDayUnitsSold.Valid {
				last.UnitsSold = row.LastDayUnitsSold.Int32
			}
			if row.LastDayRevenue.Valid {
				last.Revenue = numericToFloat(row.LastDayRevenue)
			}
			item.LastDay = last
		}
	}

	if row.LastDayStockAvailable.Valid || row.LastDayDaysOfCover.Valid {
		stock := &StockInfo{}
		if row.LastDayStockAvailable.Valid {
			stock.StockAvailable = row.LastDayStockAvailable.Int32
		}
		if row.LastDayDaysOfCover.Valid {
			v := numericToFloat(row.LastDayDaysOfCover)
			stock.DaysOfCover = &v
		}
		bucket := analytics.ComputeStockStatusBucket(stock.StockAvailable, stock.DaysOfCover, nil, nil, nil, nil)
		stock.StockStatusBucket = &bucket.Bucket
		item.Stock = stock
	}

	if url := BuildOzonProductURL(item.SKU); url != "" {
		item.OzonURL = &url
	}

	return item
}

// BuildOzonProductURL returns marketplace link when SKU is present.
func BuildOzonProductURL(sku *int64) string {
	if sku == nil || *sku <= 0 {
		return ""
	}
	return fmt.Sprintf("https://www.ozon.ru/context/detail/id/%d", *sku)
}

func normalizeSortBy(sortBy string) string {
	switch strings.TrimSpace(sortBy) {
	case "orders_last_day", "revenue_last_day", "current_price_desc", "current_price_asc", "stock_available_asc", "product_name_asc":
		return strings.TrimSpace(sortBy)
	default:
		return "revenue_last_day"
	}
}

func stockMatchesFilter(item CatalogItem, filter string) bool {
	if filter == "" || filter == "all" {
		return true
	}
	bucket := ""
	if item.Stock != nil && item.Stock.StockStatusBucket != nil {
		bucket = *item.Stock.StockStatusBucket
	}
	switch filter {
	case "had_sales_last_day":
		return item.LastDay != nil && item.LastDay.OrdersCount > 0
	case "no_sales_last_day":
		return item.LastDay == nil || item.LastDay.OrdersCount == 0
	case "stock_risk":
		return bucket == "out_of_stock" || bucket == "critical_shortage" || bucket == "shortage_7d"
	default:
		return true
	}
}

func dateValue(v time.Time) pgtype.Date {
	return pgtype.Date{
		Time:  time.Date(v.Year(), v.Month(), v.Day(), 0, 0, 0, 0, time.UTC),
		Valid: true,
	}
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

func parseMetricDate(v any) (time.Time, bool) {
	switch t := v.(type) {
	case pgtype.Date:
		if t.Valid {
			return t.Time, true
		}
	case time.Time:
		return t, true
	case *time.Time:
		if t != nil {
			return *t, true
		}
	}
	return time.Time{}, false
}

func numericToFloatPtr(n pgtype.Numeric) *float64 {
	if !n.Valid {
		return nil
	}
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return nil
	}
	v := f.Float64
	return &v
}

func (s *Service) listKeySKUProducts(ctx context.Context, sellerAccountID int64, params ListParams) (ListResult, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}
	offset := params.Offset
	if offset < 0 {
		offset = 0
	}

	latest, err := s.queries.GetLatestAvailableDashboardMetricDateBySellerAccountID(ctx, sellerAccountID)
	if err != nil || !latest.Valid || latest.Time.Year() < 2000 {
		return ListResult{Items: []CatalogItem{}, Total: 0, Limit: limit, Offset: offset}, nil
	}
	metricDate := latest.Time.UTC()
	dto, err := s.dashboard.BuildDashboardMetrics(ctx, sellerAccountID, &metricDate)
	if err != nil {
		return ListResult{}, fmt.Errorf("build dashboard metrics for key skus: %w", err)
	}

	keyIDs := dashboardhome.KeySkuProductIDs(dto.SKUs)
	if len(keyIDs) == 0 {
		formatted := metricDate.Format("2006-01-02")
		return ListResult{Items: []CatalogItem{}, Total: 0, Limit: limit, Offset: offset, LatestMetricDate: &formatted}, nil
	}
	idSet := make(map[int64]struct{}, len(keyIDs))
	for _, id := range keyIDs {
		idSet[id] = struct{}{}
	}

	keySKUs := make([]analytics.DashboardSKUMetricsItem, 0, len(keyIDs))
	for _, sku := range dto.SKUs {
		if _, ok := idSet[sku.OzonProductID]; ok {
			keySKUs = append(keySKUs, sku)
		}
	}
	sort.Slice(keySKUs, func(i, j int) bool {
		a, b := keySKUs[i], keySKUs[j]
		if a.RevenueLast7Days != b.RevenueLast7Days {
			return a.RevenueLast7Days > b.RevenueLast7Days
		}
		if a.OrdersLast7Days != b.OrdersLast7Days {
			return a.OrdersLast7Days > b.OrdersLast7Days
		}
		nameA := ""
		if a.ProductName != nil {
			nameA = *a.ProductName
		}
		nameB := ""
		if b.ProductName != nil {
			nameB = *b.ProductName
		}
		if nameA != nameB {
			return nameA < nameB
		}
		return a.OzonProductID < b.OzonProductID
	})

	total := int64(len(keySKUs))
	if offset >= len(keySKUs) {
		formatted := metricDate.Format("2006-01-02")
		return ListResult{Items: []CatalogItem{}, Total: total, Limit: limit, Offset: offset, LatestMetricDate: &formatted}, nil
	}
	end := offset + limit
	if end > len(keySKUs) {
		end = len(keySKUs)
	}
	page := keySKUs[offset:end]

	productIDs := make([]int64, 0, len(page))
	for _, sku := range page {
		productIDs = append(productIDs, sku.OzonProductID)
	}
	sparklineByProduct, err := s.loadSparklines(ctx, sellerAccountID, productIDs, metricDate)
	if err != nil {
		return ListResult{}, err
	}

	catalogRows, err := s.queries.ListProductsCatalogPage(ctx, dbgen.ListProductsCatalogPageParams{
		SellerAccountID: sellerAccountID,
		Search:          pgtype.Text{},
		HasSalesLastDay: pgtype.Bool{},
		SortBy:          "revenue_last_day",
		PageLimit:       int32(len(productIDs) + 10),
		PageOffset:      0,
	})
	if err != nil {
		return ListResult{}, fmt.Errorf("list products catalog for key skus: %w", err)
	}
	byProductID := make(map[int64]dbgen.ListProductsCatalogPageRow, len(catalogRows))
	for _, row := range catalogRows {
		byProductID[row.OzonProductID] = row
	}

	items := make([]CatalogItem, 0, len(page))
	for _, sku := range page {
		row, ok := byProductID[sku.OzonProductID]
		if !ok {
			continue
		}
		items = append(items, mapCatalogRow(row, sparklineByProduct[sku.OzonProductID]))
	}

	formatted := metricDate.Format("2006-01-02")
	return ListResult{
		Items:            items,
		Total:            total,
		Limit:            limit,
		Offset:           offset,
		LatestMetricDate: &formatted,
	}, nil
}
