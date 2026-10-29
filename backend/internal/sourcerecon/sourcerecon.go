// Package sourcerecon holds the shared, reusable reconciliation math that proves
// a seller's analysis data snapshot is internally consistent (account ↔ SKU ↔
// category ↔ advertising ↔ funnel ↔ stocks) before it is trusted by the
// dashboard facts layer or handed to the unified analysis (pre-OpenAI) gate.
//
// The core entrypoint, ReconcileSnapshot, operates purely on an in-memory
// analysisdata.AnalysisDataSnapshot so it can be shared by:
//   - dev-seed-mvp / devtools validate-seller-analysis-data (loads a snapshot)
//   - the unified analysis pre-OpenAI gate (uses result.Snapshot)
//   - dashboardfacts validation (uses the same snapshot as the pack)
//
// Callers that also have raw DB access (e.g. the devseed wrapper) layer
// additional source-of-truth checks on top of the report this package returns.
package sourcerecon

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

// RevenueTolerance is the relative tolerance applied to daily revenue equality
// checks (1%). Revenue is stored as numeric(18,2) at multiple layers, so small
// rounding differences must not fail reconciliation.
const RevenueTolerance = 0.01

// ReconciliationIssue is a single reconciliation finding. Expected/Actual are
// left as `any` so callers can surface typed values (ints, floats, strings).
type ReconciliationIssue struct {
	Code     string `json:"code"`
	Path     string `json:"path"`
	Date     string `json:"date,omitempty"`
	EntityID string `json:"entity_id,omitempty"`
	Expected any    `json:"expected,omitempty"`
	Actual   any    `json:"actual,omitempty"`
	Message  string `json:"message,omitempty"`
}

// Report is the full outcome of a reconciliation pass. Valid is false when
// Errors is non-empty. Warnings never affect Valid. Counts carries coverage
// counters (rows/days per domain) and Hash is a deterministic digest of the
// findings for change detection.
type Report struct {
	Valid    bool                  `json:"valid"`
	Errors   []ReconciliationIssue `json:"errors"`
	Warnings []ReconciliationIssue `json:"warnings"`
	Counts   map[string]int64      `json:"counts"`
	Hash     string                `json:"hash"`
}

// ReconciliationReport is the name used by DB-layer callers (devseed/CLI). It is
// an alias of Report so every caller shares one canonical type.
type ReconciliationReport = Report

// Error summarizes the reconciliation errors as a single string, suitable for
// wrapping into a failure message. Empty when the report is valid.
func (r Report) Error() string {
	if len(r.Errors) == 0 {
		return ""
	}
	parts := make([]string, 0, len(r.Errors))
	for _, issue := range r.Errors {
		seg := issue.Code
		if issue.Date != "" {
			seg += "@" + issue.Date
		}
		if issue.EntityID != "" {
			seg += "#" + issue.EntityID
		}
		if issue.Message != "" {
			seg += ": " + issue.Message
		}
		parts = append(parts, seg)
	}
	return strings.Join(parts, "; ")
}

// builder accumulates issues/counts while a reconciliation pass runs.
type builder struct {
	errors   []ReconciliationIssue
	warnings []ReconciliationIssue
	counts   map[string]int64
}

func (b *builder) err(issue ReconciliationIssue)  { b.errors = append(b.errors, issue) }
func (b *builder) warn(issue ReconciliationIssue) { b.warnings = append(b.warnings, issue) }
func (b *builder) count(key string, n int64)      { b.counts[key] += n }

// Finalize sorts the findings deterministically, sets Valid, computes Hash, and
// returns a report. Exposed so DB-layer wrappers can append their own issues and
// then re-finalize a single, canonical report.
func Finalize(report Report) Report {
	if report.Counts == nil {
		report.Counts = map[string]int64{}
	}
	sortIssues(report.Errors)
	sortIssues(report.Warnings)
	report.Valid = len(report.Errors) == 0
	report.Hash = hashReport(report)
	return report
}

func sortIssues(issues []ReconciliationIssue) {
	sort.SliceStable(issues, func(i, j int) bool {
		a, b := issues[i], issues[j]
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		if a.Date != b.Date {
			return a.Date < b.Date
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.EntityID < b.EntityID
	})
}

func hashReport(report Report) string {
	payload := struct {
		Errors   []ReconciliationIssue `json:"errors"`
		Warnings []ReconciliationIssue `json:"warnings"`
		Counts   map[string]int64      `json:"counts"`
	}{report.Errors, report.Warnings, report.Counts}
	raw, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// ReconcileSnapshot runs the full set of snapshot-only reconciliation checks and
// returns a finalized report. It returns an error only when the snapshot is nil
// (nothing to reconcile); all other problems are reported as issues so callers
// get a uniform report shape and can persist it even on failure.
func ReconcileSnapshot(snap *analysisdata.AnalysisDataSnapshot) (Report, error) {
	if snap == nil {
		return Report{}, fmt.Errorf("sourcerecon: snapshot is nil")
	}
	b := &builder{counts: map[string]int64{}}
	if snap.SellerAccountID <= 0 {
		b.err(ReconciliationIssue{Code: "invalid_seller_account_id", Path: "snapshot.seller_account_id", Actual: snap.SellerAccountID})
	}
	if !snap.MetricDate.Valid {
		b.err(ReconciliationIssue{Code: "invalid_metric_date", Path: "snapshot.metric_date"})
		return Finalize(Report{Errors: b.errors, Warnings: b.warnings, Counts: b.counts}), nil
	}

	anchor := analysisdata.DateOnlyUTC(snap.MetricDate.Time)
	anchorKey := analysisdata.FormatDate(anchor)

	checkSellerIdentityAndPeriod(b, snap, anchor, anchorKey)
	checkNoDuplicates(b, snap)
	checkNegatives(b, snap)
	checkAccountVsSKU(b, snap)
	checkCategoryMapping(b, snap)
	checkAdvertising(b, snap, anchor)
	checkFunnel(b, snap)
	checkStocksOnAnchor(b, snap, anchorKey)

	return Finalize(Report{Errors: b.errors, Warnings: b.warnings, Counts: b.counts}), nil
}

// ---------------------------------------------------------------------------
// Seller identity + period (no foreign sellers, no dates after anchor)
// ---------------------------------------------------------------------------

func checkSellerIdentityAndPeriod(b *builder, snap *analysisdata.AnalysisDataSnapshot, anchor time.Time, anchorKey string) {
	seller := snap.SellerAccountID

	foreign := func(path string, id int64, dateKey string) {
		if id != seller {
			b.err(ReconciliationIssue{Code: "foreign_seller_row", Path: path, Date: dateKey, EntityID: fmt.Sprint(id), Expected: seller, Actual: id})
		}
	}
	afterAnchor := func(path string, d pgtype.Date, dateKey string) {
		if d.Valid && analysisdata.DateOnlyUTC(d.Time).After(anchor) {
			b.err(ReconciliationIssue{Code: "date_after_anchor", Path: path, Date: dateKey, Expected: anchorKey, Actual: dateKey})
		}
	}

	for _, r := range snap.AccountMetrics {
		b.count("account_rows", 1)
		foreign("daily_account_metrics.seller_account_id", r.SellerAccountID, dk(r.MetricDate))
		afterAnchor("daily_account_metrics.metric_date", r.MetricDate, dk(r.MetricDate))
	}
	for _, r := range snap.SKUMetrics {
		b.count("sku_rows", 1)
		foreign("daily_sku_metrics.seller_account_id", r.SellerAccountID, dk(r.MetricDate))
		afterAnchor("daily_sku_metrics.metric_date", r.MetricDate, dk(r.MetricDate))
	}
	for _, r := range snap.AdMetrics {
		b.count("ad_rows", 1)
		foreign("ad_metrics_daily.seller_account_id", r.SellerAccountID, dk(r.MetricDate))
		afterAnchor("ad_metrics_daily.metric_date", r.MetricDate, dk(r.MetricDate))
	}
	for _, r := range snap.FunnelRows {
		b.count("funnel_rows", 1)
		foreign("product_analytics_daily.seller_account_id", r.SellerAccountID, dk(r.MetricDate))
		afterAnchor("product_analytics_daily.metric_date", r.MetricDate, dk(r.MetricDate))
	}
	for _, r := range snap.StockSnapshots {
		b.count("stock_rows", 1)
		foreign("stock_analytics_snapshots.seller_account_id", r.SellerAccountID, dk(r.SnapshotDate))
		afterAnchor("stock_analytics_snapshots.snapshot_date", r.SnapshotDate, dk(r.SnapshotDate))
	}
	for _, r := range snap.Campaigns {
		b.count("campaigns", 1)
		foreign("ad_campaigns.seller_account_id", r.SellerAccountID, "")
	}
	for _, r := range snap.Products {
		b.count("products", 1)
		foreign("products.seller_account_id", r.SellerAccountID, "")
	}
	b.count("categories", int64(len(snap.Categories)))
}

// ---------------------------------------------------------------------------
// No duplicate rows within a domain grain
// ---------------------------------------------------------------------------

func checkNoDuplicates(b *builder, snap *analysisdata.AnalysisDataSnapshot) {
	accSeen := map[string]struct{}{}
	for _, r := range snap.AccountMetrics {
		key := dk(r.MetricDate)
		if _, ok := accSeen[key]; ok {
			b.err(ReconciliationIssue{Code: "duplicate_row", Path: "daily_account_metrics", Date: key})
		}
		accSeen[key] = struct{}{}
	}
	skuSeen := map[string]struct{}{}
	for _, r := range snap.SKUMetrics {
		key := dk(r.MetricDate) + "|" + fmt.Sprint(r.OzonProductID)
		if _, ok := skuSeen[key]; ok {
			b.err(ReconciliationIssue{Code: "duplicate_row", Path: "daily_sku_metrics", Date: dk(r.MetricDate), EntityID: fmt.Sprint(r.OzonProductID)})
		}
		skuSeen[key] = struct{}{}
	}
	adSeen := map[string]struct{}{}
	for _, r := range snap.AdMetrics {
		key := dk(r.MetricDate) + "|" + fmt.Sprint(r.CampaignExternalID)
		if _, ok := adSeen[key]; ok {
			b.err(ReconciliationIssue{Code: "duplicate_row", Path: "ad_metrics_daily", Date: dk(r.MetricDate), EntityID: fmt.Sprint(r.CampaignExternalID)})
		}
		adSeen[key] = struct{}{}
	}
	funnelSeen := map[string]struct{}{}
	for _, r := range snap.FunnelRows {
		key := dk(r.MetricDate)
		if _, ok := funnelSeen[key]; ok {
			b.err(ReconciliationIssue{Code: "duplicate_row", Path: "product_analytics_daily(account)", Date: key})
		}
		funnelSeen[key] = struct{}{}
	}
}

// ---------------------------------------------------------------------------
// No negative measures anywhere
// ---------------------------------------------------------------------------

func checkNegatives(b *builder, snap *analysisdata.AnalysisDataSnapshot) {
	neg := func(path, dateKey, entity string, value float64) {
		if value < 0 {
			b.err(ReconciliationIssue{Code: "negative_value", Path: path, Date: dateKey, EntityID: entity, Actual: value})
		}
	}
	for _, r := range snap.AccountMetrics {
		d := dk(r.MetricDate)
		neg("daily_account_metrics.revenue", d, "", numericToFloat(r.Revenue))
		neg("daily_account_metrics.orders_count", d, "", float64(r.OrdersCount))
		neg("daily_account_metrics.returns_count", d, "", float64(r.ReturnsCount))
		neg("daily_account_metrics.cancel_count", d, "", float64(r.CancelCount))
		neg("daily_account_metrics.units_sold", d, "", float64(i4(r.UnitsSold)))
	}
	for _, r := range snap.SKUMetrics {
		d := dk(r.MetricDate)
		e := fmt.Sprint(r.OzonProductID)
		neg("daily_sku_metrics.revenue", d, e, numericToFloat(r.Revenue))
		neg("daily_sku_metrics.orders_count", d, e, float64(r.OrdersCount))
		neg("daily_sku_metrics.stock_available", d, e, float64(r.StockAvailable))
		neg("daily_sku_metrics.units_sold", d, e, float64(i4(r.UnitsSold)))
	}
	for _, r := range snap.AdMetrics {
		d := dk(r.MetricDate)
		e := fmt.Sprint(r.CampaignExternalID)
		neg("ad_metrics_daily.impressions", d, e, float64(r.Impressions))
		neg("ad_metrics_daily.clicks", d, e, float64(r.Clicks))
		neg("ad_metrics_daily.spend", d, e, numericToFloat(r.Spend))
		neg("ad_metrics_daily.orders_count", d, e, float64(r.OrdersCount))
		neg("ad_metrics_daily.revenue", d, e, numericToFloat(r.Revenue))
	}
	for _, r := range snap.FunnelRows {
		d := dk(r.MetricDate)
		neg("product_analytics_daily.impressions", d, "", float64(i8(r.Impressions)))
		neg("product_analytics_daily.product_views", d, "", float64(i8(r.ProductViews)))
		neg("product_analytics_daily.add_to_cart", d, "", float64(i8(r.AddToCart)))
		neg("product_analytics_daily.orders_count", d, "", float64(i8(r.OrdersCount)))
		neg("product_analytics_daily.completed_orders", d, "", float64(i8(r.CompletedOrders)))
	}
	for _, r := range snap.StockSnapshots {
		neg("stock_analytics_snapshots.available_stock_count", dk(r.SnapshotDate), fmt.Sprint(r.Sku), float64(r.AvailableStockCount))
	}
}

// ---------------------------------------------------------------------------
// Account ↔ SKU sums per day + per-SKU sanity
// ---------------------------------------------------------------------------

type dailySKUAgg struct {
	orders  int64
	revenue float64
	units   int64
}

func checkAccountVsSKU(b *builder, snap *analysisdata.AnalysisDataSnapshot) {
	skuByDay := map[string]*dailySKUAgg{}
	for _, r := range snap.SKUMetrics {
		d := dk(r.MetricDate)
		agg := skuByDay[d]
		if agg == nil {
			agg = &dailySKUAgg{}
			skuByDay[d] = agg
		}
		agg.orders += int64(r.OrdersCount)
		agg.revenue += numericToFloat(r.Revenue)
		agg.units += int64(i4(r.UnitsSold))
	}

	for _, acc := range snap.AccountMetrics {
		d := dk(acc.MetricDate)
		accOrders := int64(acc.OrdersCount)
		accCancels := int64(acc.CancelCount)
		accRevenue := numericToFloat(acc.Revenue)
		accUnits := int64(i4(acc.UnitsSold))

		agg := skuByDay[d]
		if agg == nil {
			agg = &dailySKUAgg{}
		}

		// Non-cancelled orders map 1:1 to SKU order rows.
		expectedSKUOrders := accOrders - accCancels
		if agg.orders != expectedSKUOrders {
			b.err(ReconciliationIssue{
				Code: "account_sku_orders_mismatch", Path: "daily_sku_metrics.orders_count", Date: d,
				Expected: expectedSKUOrders, Actual: agg.orders,
				Message: fmt.Sprintf("SUM(sku orders)=%d != account orders-cancels=%d (orders=%d cancels=%d)", agg.orders, expectedSKUOrders, accOrders, accCancels),
			})
		}
		if relDiff(agg.revenue, accRevenue) > RevenueTolerance {
			b.err(ReconciliationIssue{
				Code: "account_sku_revenue_mismatch", Path: "daily_sku_metrics.revenue", Date: d,
				Expected: round2(accRevenue), Actual: round2(agg.revenue),
			})
		}
		// Units can be sparsely populated; surface as a warning only.
		if accUnits > 0 && agg.units != accUnits {
			b.warn(ReconciliationIssue{
				Code: "account_sku_units_mismatch", Path: "daily_sku_metrics.units_sold", Date: d,
				Expected: accUnits, Actual: agg.units,
			})
		}
	}

	// Per-SKU: no single SKU exceeds its account day, and no revenue/orders
	// zero-crossings.
	accByDay := map[string]dbgen.DailyAccountMetric{}
	for _, acc := range snap.AccountMetrics {
		accByDay[dk(acc.MetricDate)] = acc
	}
	for _, r := range snap.SKUMetrics {
		d := dk(r.MetricDate)
		e := fmt.Sprint(r.OzonProductID)
		skuOrders := int64(r.OrdersCount)
		skuRevenue := numericToFloat(r.Revenue)
		skuUnits := int64(i4(r.UnitsSold))
		if acc, ok := accByDay[d]; ok {
			if skuOrders > int64(acc.OrdersCount) {
				b.err(ReconciliationIssue{Code: "sku_orders_exceed_account", Path: "daily_sku_metrics.orders_count", Date: d, EntityID: e, Expected: int64(acc.OrdersCount), Actual: skuOrders})
			}
			if skuRevenue > numericToFloat(acc.Revenue)*(1+RevenueTolerance) {
				b.err(ReconciliationIssue{Code: "sku_revenue_exceed_account", Path: "daily_sku_metrics.revenue", Date: d, EntityID: e, Expected: round2(numericToFloat(acc.Revenue)), Actual: round2(skuRevenue)})
			}
			if accUnits := int64(i4(acc.UnitsSold)); accUnits > 0 && skuUnits > accUnits {
				b.warn(ReconciliationIssue{Code: "sku_units_exceed_account", Path: "daily_sku_metrics.units_sold", Date: d, EntityID: e, Expected: accUnits, Actual: skuUnits})
			}
		}
		if skuOrders > 0 && skuRevenue <= 0 {
			b.err(ReconciliationIssue{Code: "sku_orders_without_revenue", Path: "daily_sku_metrics", Date: d, EntityID: e, Actual: skuRevenue})
		}
		if skuRevenue > 0 && skuOrders <= 0 {
			b.err(ReconciliationIssue{Code: "sku_revenue_without_orders", Path: "daily_sku_metrics", Date: d, EntityID: e, Actual: skuOrders})
		}
	}
}

// ---------------------------------------------------------------------------
// Category mapping present + sums match
// ---------------------------------------------------------------------------

func checkCategoryMapping(b *builder, snap *analysisdata.AnalysisDataSnapshot) {
	categoryNames := map[int64]string{}
	for _, c := range snap.Categories {
		categoryNames[c.DescriptionCategoryID] = c.CategoryName
	}
	catByProduct := map[int64]int64{}
	for _, p := range snap.Products {
		if !p.DescriptionCategoryID.Valid {
			continue
		}
		id := p.DescriptionCategoryID.Int64
		catByProduct[p.OzonProductID] = id
		if _, ok := categoryNames[id]; !ok {
			b.err(ReconciliationIssue{Code: "category_mapping_missing", Path: "description_categories", EntityID: fmt.Sprint(id), Message: fmt.Sprintf("product %d references category %d not present in snapshot.Categories", p.OzonProductID, id)})
		}
	}

	// Every SKU that carries metrics should resolve to a category. Missing
	// mappings are a soft (warning) signal — uncategorized long-tail SKUs are
	// tolerated but surfaced.
	skuProducts := map[int64]struct{}{}
	for _, r := range snap.SKUMetrics {
		skuProducts[r.OzonProductID] = struct{}{}
	}
	for pid := range skuProducts {
		if _, ok := catByProduct[pid]; !ok {
			b.warn(ReconciliationIssue{Code: "sku_category_unmapped", Path: "products.description_category_id", EntityID: fmt.Sprint(pid)})
		}
	}

	// Sums match: the total revenue/orders of categorized SKUs must not exceed
	// the total across all SKUs (a rollup can never invent value).
	var totalRevenue, categorizedRevenue float64
	var totalOrders, categorizedOrders int64
	for _, r := range snap.SKUMetrics {
		rev := numericToFloat(r.Revenue)
		ord := int64(r.OrdersCount)
		totalRevenue += rev
		totalOrders += ord
		if _, ok := catByProduct[r.OzonProductID]; ok {
			categorizedRevenue += rev
			categorizedOrders += ord
		}
	}
	if categorizedRevenue > totalRevenue*(1+RevenueTolerance) {
		b.err(ReconciliationIssue{Code: "category_revenue_exceeds_total", Path: "categories", Expected: round2(totalRevenue), Actual: round2(categorizedRevenue)})
	}
	if categorizedOrders > totalOrders {
		b.err(ReconciliationIssue{Code: "category_orders_exceeds_total", Path: "categories", Expected: totalOrders, Actual: categorizedOrders})
	}
}

// ---------------------------------------------------------------------------
// Advertising invariants
// ---------------------------------------------------------------------------

func checkAdvertising(b *builder, snap *analysisdata.AnalysisDataSnapshot, anchor time.Time) {
	type dayAgg struct {
		spend   float64
		revenue float64
		orders  int64
	}
	byDay := map[string]*dayAgg{}
	var noOrderRows int64

	for _, r := range snap.AdMetrics {
		d := dk(r.MetricDate)
		e := fmt.Sprint(r.CampaignExternalID)
		spend := numericToFloat(r.Spend)
		revenue := numericToFloat(r.Revenue)
		orders := int64(r.OrdersCount)

		// CTR ≤ 100% (clicks cannot exceed impressions).
		if r.Clicks > r.Impressions {
			b.err(ReconciliationIssue{Code: "ctr_over_100", Path: "ad_metrics_daily", Date: d, EntityID: e, Expected: r.Impressions, Actual: r.Clicks})
		}
		// Attributed revenue requires attributed orders.
		if revenue > 0 && orders <= 0 {
			b.err(ReconciliationIssue{Code: "ad_attributed_revenue_without_orders", Path: "ad_metrics_daily", Date: d, EntityID: e, Actual: round2(revenue)})
		}
		// A no-orders campaign row is spend>0 AND orders=0.
		if spend > 0 && orders == 0 {
			noOrderRows++
		}

		agg := byDay[d]
		if agg == nil {
			agg = &dayAgg{}
			byDay[d] = agg
		}
		agg.spend += spend
		agg.revenue += revenue
		agg.orders += orders
	}
	b.count("ad_no_order_rows", noOrderRows)

	// Weekly ratio diversity: over the trailing 7 days ending at the anchor,
	// require ≥5 distinct daily DRR ratios (spend/attributed-revenue). This is a
	// seed-acceptance signal (data variety), so it is a warning, not a failure.
	distinct := map[float64]struct{}{}
	daysWithData := 0
	for i := 0; i < 7; i++ {
		dayKey := analysisdata.FormatDate(anchor.AddDate(0, 0, -i))
		agg, ok := byDay[dayKey]
		if !ok {
			continue
		}
		daysWithData++
		if agg.revenue > 0 {
			distinct[math.Round(agg.spend/agg.revenue*100)/100] = struct{}{}
		}
	}
	if daysWithData >= 7 && len(distinct) < 5 {
		b.warn(ReconciliationIssue{Code: "ad_weekly_ratio_low_diversity", Path: "ad_metrics_daily", Expected: 5, Actual: len(distinct), Message: "fewer than 5 distinct daily DRR ratios across the trailing 7 days"})
	}
}

// ---------------------------------------------------------------------------
// Funnel monotonicity + conversion sanity
// ---------------------------------------------------------------------------

func checkFunnel(b *builder, snap *analysisdata.AnalysisDataSnapshot) {
	for _, r := range snap.FunnelRows {
		d := dk(r.MetricDate)
		impressions := i8(r.Impressions)
		views := i8(r.ProductViews)
		cart := i8(r.AddToCart)
		orders := i8(r.OrdersCount)
		completed := i8(r.CompletedOrders)

		stages := []struct {
			name string
			cur  int64
			prev int64
		}{
			{"product_views<=impressions", views, impressions},
			{"add_to_cart<=product_views", cart, views},
			{"orders<=add_to_cart", orders, cart},
			{"completed<=orders", completed, orders},
		}
		for _, s := range stages {
			// A stage exceeding its predecessor breaks monotonicity and implies a
			// conversion > 100%.
			if s.prev > 0 && s.cur > s.prev {
				b.err(ReconciliationIssue{Code: "funnel_not_monotonic", Path: "product_analytics_daily", Date: d, EntityID: s.name, Expected: s.prev, Actual: s.cur})
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Stocks on the anchor date
// ---------------------------------------------------------------------------

func checkStocksOnAnchor(b *builder, snap *analysisdata.AnalysisDataSnapshot, anchorKey string) {
	var skuStock int64
	for _, r := range snap.SKUMetrics {
		if dk(r.MetricDate) == anchorKey {
			skuStock += int64(r.StockAvailable)
		}
	}
	var analyticsStock int64
	analyticsRows := 0
	for _, r := range snap.StockSnapshots {
		if dk(r.SnapshotDate) == anchorKey {
			analyticsStock += int64(r.AvailableStockCount)
			analyticsRows++
		}
	}
	b.count("anchor_sku_stock", skuStock)
	b.count("anchor_stock_analytics", analyticsStock)

	// The daily SKU stock and the stock-analytics aggregate are both derived
	// from the same raw stocks table on the anchor, so they should agree. This
	// is surfaced as a warning rather than a hard failure because stock
	// analytics snapshots may carry per-cluster rows in addition to per-warehouse
	// rows, whose naive sum can legitimately differ from the SKU rollup; the
	// devseed DB wrapper performs the exact raw-stocks three-way check.
	if analyticsRows > 0 && skuStock != analyticsStock {
		b.warn(ReconciliationIssue{Code: "stock_aggregate_mismatch", Path: "stock_analytics_snapshots.available_stock_count", Date: anchorKey, Expected: skuStock, Actual: analyticsStock})
	}
}

// ---------------------------------------------------------------------------
// small typed helpers
// ---------------------------------------------------------------------------

func numericToFloat(n pgtype.Numeric) float64 {
	if !n.Valid {
		return 0
	}
	v, err := n.Float64Value()
	if err != nil || !v.Valid {
		return 0
	}
	return v.Float64
}

func i4(v pgtype.Int4) int32 {
	if !v.Valid {
		return 0
	}
	return v.Int32
}

func i8(v pgtype.Int8) int64 {
	if !v.Valid {
		return 0
	}
	return v.Int64
}

func dk(d pgtype.Date) string {
	if !d.Valid {
		return ""
	}
	return analysisdata.FormatDate(d.Time)
}

func relDiff(a, b float64) float64 {
	denom := math.Max(1, math.Abs(b))
	return math.Abs(a-b) / denom
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
