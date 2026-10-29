package dashboardhome

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

const stockSKUFocusTarget = "sku-focus-stock"

var stockNarrativeSegmentOrder = []string{
	"critical_0_3", "shortage_4_7", "planning_8_14", "healthy_14plus", "surplus",
}

var stockNarrativeForbiddenActionVerbs = []string{
	"проверьте", "пополните", "закажите", "остановите", "сократите", "сделайте", "запланируйте",
}

var stockNarrativeForbiddenExampleLabels = []string{
	"товары в приоритете", "пополнить первым", "что сделать", "что сделать по sku",
}

type stockCoverageRevenueAtRisk struct {
	Status            string
	TotalDaily        float64
	CriticalDaily     float64
	ShortageDaily     float64
	AffectedSKUs      int
	SharePct          *float64
	FormattedTotal    string
	FormattedCritical string
	FormattedShortage string
}

type stockCoverageRiskSKU struct {
	ProductName        string
	SKU                string
	Stock              int32
	DaysOfCover        *float64
	RiskLevel          string
	AvgDailyRevenue7d  string
	AvgDailyRevenueNum float64
	SalesSignal        bool
	WeightedDailyRisk  float64
}

type stockCoverageNarrativeValidationContext struct {
	SegmentCounts       map[string]int
	RevenueTotal        string
	RevenueCritical     string
	RevenueShortage     string
	RevenueBackend      stockCoverageRevenueAtRisk
	AdsEvidence         bool
	NoSalesCount        int
	NoSalesStockUnits   int
	AllowedProductNames map[string]struct{}
	AllowedNumbers      map[string]struct{}
}

func stockCoverageRiskWeight(bucket string) float64 {
	switch bucket {
	case "out_of_stock", "critical_0_3":
		return 1.0
	case "shortage_4_7":
		return 0.0 // excluded from revenue-at-risk; only critical/oos
	default:
		return 0
	}
}

// computeStockCoverageRevenueAtRisk is average daily revenue of SKUs in
// out_of_stock or critical_shortage: sum(revenue_7d) / 7.
func computeStockCoverageRevenueAtRisk(skus []analytics.DashboardSKUMetricsItem) stockCoverageRevenueAtRisk {
	out := stockCoverageRevenueAtRisk{Status: "unavailable"}
	var totalRev7d, critRev7d float64
	affected := 0
	for _, sku := range skus {
		bucket := StockCoverageBucketKey(sku)
		if stockCoverageRiskWeight(bucket) <= 0 {
			continue
		}
		if sku.RevenueLast7Days <= 0 && !skuHasDemandSignal(sku) {
			// Still count oos with recent demand via OrdersLast7Days.
			if sku.OrdersLast7Days <= 0 {
				continue
			}
		}
		rev := sku.RevenueLast7Days
		if rev <= 0 {
			continue
		}
		totalRev7d += rev
		affected++
		if bucket == "critical_0_3" || bucket == "out_of_stock" {
			critRev7d += rev
		}
	}
	if affected == 0 || totalRev7d <= 0 {
		return out
	}
	total := totalRev7d / 7
	crit := critRev7d / 7
	out.Status = "available"
	out.TotalDaily = total
	out.CriticalDaily = crit
	out.ShortageDaily = 0
	out.AffectedSKUs = affected
	out.FormattedTotal = formatRubPerDay(total)
	out.FormattedCritical = formatRubPerDay(crit)
	out.FormattedShortage = formatRubPerDay(0)
	return out
}

// attachRevenueAtRiskShare sets share of average daily account revenue (7d/7).
func attachRevenueAtRiskShare(rev *stockCoverageRevenueAtRisk, accountRevenue7d float64) {
	if rev == nil || rev.Status != "available" || rev.TotalDaily <= 0 || accountRevenue7d <= 0 {
		return
	}
	dailyAccount := accountRevenue7d / 7
	if dailyAccount <= 0 {
		return
	}
	pct := rev.TotalDaily / dailyAccount * 100
	if pct > 100 {
		pct = 100
	}
	pct = math.Round(pct*10) / 10
	rev.SharePct = &pct
	rev.FormattedTotal = fmt.Sprintf("%s · %.0f%% дневной выручки", formatRubPerDay(rev.TotalDaily), pct)
}

func denomForAvgDaily7d(sku analytics.DashboardSKUMetricsItem) int {
	d := sku.ActiveSalesDays7D
	if d < 1 {
		if sku.RevenueLast7Days > 0 || sku.OrdersLast7Days > 0 {
			return 7
		}
		return 7
	}
	if d > 7 {
		return 7
	}
	return d
}

func avgDailyRevenue7d(sku analytics.DashboardSKUMetricsItem) float64 {
	d := denomForAvgDaily7d(sku)
	if d <= 0 {
		return 0
	}
	return sku.RevenueLast7Days / float64(d)
}

func avgDailyOrders7d(sku analytics.DashboardSKUMetricsItem) float64 {
	d := denomForAvgDaily7d(sku)
	if d <= 0 {
		return 0
	}
	return float64(sku.OrdersLast7Days) / float64(d)
}

func skuHasDemandSignal(sku analytics.DashboardSKUMetricsItem) bool {
	return avgDailyRevenue7d(sku) > 0 || avgDailyOrders7d(sku) > 0
}

func formatRubPerDay(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return "0 ₽/день"
	}
	return fmt.Sprintf("%s/день", formatMoney(math.Round(v)))
}

func formatDaysOfCoverLabel(v *float64) string {
	if v == nil {
		return "Покрытие: нет"
	}
	if math.IsNaN(*v) || math.IsInf(*v, 0) {
		return "Покрытие: нет"
	}
	if *v <= 0 {
		return "Покрытие: нет"
	}
	s := strings.ReplaceAll(fmt.Sprintf("%.1f", *v), ".", ",")
	return "Покрытие: " + s + " дня"
}

func buildStockRiskSKURows(skus []analytics.DashboardSKUMetricsItem) []stockCoverageRiskSKU {
	rows := make([]stockCoverageRiskSKU, 0, len(skus))
	for _, sku := range skus {
		bucket := StockCoverageBucketKey(sku)
		w := stockCoverageRiskWeight(bucket)
		if w <= 0 {
			continue
		}
		rev7 := sku.RevenueLast7Days
		if rev7 <= 0 {
			continue
		}
		avg := rev7 / 7
		name := derefStr(sku.ProductName, "")
		if name == "" {
			name = "Товар"
		}
		skuCode := ""
		if sku.SKU != nil {
			skuCode = fmt.Sprintf("%d", *sku.SKU)
		}
		rows = append(rows, stockCoverageRiskSKU{
			ProductName:        name,
			SKU:                skuCode,
			Stock:              sku.StockAvailable,
			DaysOfCover:        sku.DaysOfCover,
			RiskLevel:          bucket,
			AvgDailyRevenue7d:  formatMoney(math.Round(avg)),
			AvgDailyRevenueNum: avg,
			SalesSignal:        true,
			WeightedDailyRisk:  avg * w,
		})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].WeightedDailyRisk != rows[j].WeightedDailyRisk {
			return rows[i].WeightedDailyRisk > rows[j].WeightedDailyRisk
		}
		return rows[i].ProductName < rows[j].ProductName
	})
	return rows
}

func countNoSalesWithStock(skus []analytics.DashboardSKUMetricsItem) (count int, units int) {
	for _, sku := range skus {
		if sku.StockAvailable <= 0 {
			continue
		}
		if sku.RevenueLast7Days > 0 || sku.OrdersLast7Days > 0 {
			continue
		}
		count++
		units += int(sku.StockAvailable)
	}
	return count, units
}

func buildFactualStockCoverageSummary(segments []StockCoverageNarrativeSegment, noSalesCount int) string {
	return buildStockCoverageSellerSummary(segments, noSalesCount)
}

func buildStockCoverageFactualNarrative(
	dto analytics.DashboardMetricsDTO,
	resp *Response,
	recs []dbgen.Recommendation,
) *StockCoverageNarrative {
	if resp.AIDiagnostics.StockCoverage.Status != WidgetAvailable || resp.AIDiagnostics.StockCoverage.TotalSKUs <= 0 {
		return nil
	}
	rev := computeStockCoverageRevenueAtRisk(dto.SKUs)
	attachRevenueAtRiskShare(&rev, dto.Account.RevenueLast7Days)
	nsCount, nsUnits := countNoSalesWithStock(dto.SKUs)
	segments := make([]StockCoverageNarrativeSegment, 0, len(resp.AIDiagnostics.StockCoverage.Segments))
	for _, s := range resp.AIDiagnostics.StockCoverage.Segments {
		segments = append(segments, StockCoverageNarrativeSegment{
			Key:      s.Key,
			Label:    s.Label,
			Count:    s.SKUCount,
			Severity: s.Severity,
		})
	}
	revBox := StockCoverageNarrativeRevenue{
		Status: "unavailable",
		Label:  "Выручка под риском",
	}
	if rev.Status == "available" {
		revBox = StockCoverageNarrativeRevenue{
			Status:                 "available",
			Label:                  "Выручка под риском",
			Value:                  rev.FormattedTotal,
			MethodNote:             sellerFacingRevenueMethodNote,
			ShareOfDailyRevenuePct: rev.SharePct,
		}
	} else {
		revBox.MethodNote = "Недостаточно данных для оценки выручки под риском."
	}
	ns := StockCoverageNoSalesWithStock{Status: "unavailable"}
	if nsCount > 0 {
		ns = StockCoverageNoSalesWithStock{
			Status:     "available",
			Count:      nsCount,
			StockUnits: nsUnits,
		}
	}
	ads := StockCoverageAdvertisingLink{Status: "unavailable"}
	if advertisingLowStockEvidence(recs) {
		ads = StockCoverageAdvertisingLink{Status: "available"}
	}
	n := &StockCoverageNarrative{
		Status:           "available",
		Headline:         buildStockCoverageSellerHeadline(segments, nsCount),
		Summary:          buildFactualStockCoverageSummary(segments, nsCount),
		NextStep:         defaultStockCoverageNextStep(segments, nsCount),
		Tone:             "warning",
		Source:           "dashboard_narrative_ai_only",
		RevenueAtRisk:    revBox,
		Segments:         segments,
		NoSalesWithStock: ns,
		AdvertisingLink:  ads,
		SKUFocusLink: StockCoverageSKUFocusLink{
			Label:  defaultSKUFocusLinkLabel,
			Target: stockSKUFocusTarget,
		},
	}
	if nsCount > 0 {
		n.NoSalesFocusLink = &StockCoverageSKUFocusLink{
			Label:  defaultNoSalesFocusLinkLabel,
			Target: stockNoSalesSKUFocusTarget,
		}
	}
	return n
}

// repairStockCoverageNarrativeFromFactual fills structural fields required for validation
// while preserving AI-authored summary/headline when present.
func repairStockCoverageNarrativeFromFactual(n, factual *StockCoverageNarrative) {
	if n == nil || factual == nil {
		return
	}
	if strings.TrimSpace(n.Status) == "" {
		n.Status = factual.Status
	}
	if len(n.Segments) == 0 {
		n.Segments = append([]StockCoverageNarrativeSegment(nil), factual.Segments...)
	}
	if strings.TrimSpace(n.RevenueAtRisk.Status) == "" {
		n.RevenueAtRisk = factual.RevenueAtRisk
	}
	if strings.TrimSpace(n.NoSalesWithStock.Status) == "" {
		n.NoSalesWithStock = factual.NoSalesWithStock
	}
	if strings.TrimSpace(n.AdvertisingLink.Status) == "" {
		n.AdvertisingLink = factual.AdvertisingLink
	}
	if strings.TrimSpace(n.SKUFocusLink.Label) == "" {
		n.SKUFocusLink = factual.SKUFocusLink
	}
	if n.NoSalesFocusLink == nil {
		n.NoSalesFocusLink = factual.NoSalesFocusLink
	}
	if strings.TrimSpace(n.Summary) == "" && isStockCoverageSummarySubstantive(factual.Summary) {
		n.Summary = factual.Summary
	}
	if strings.TrimSpace(n.Headline) == "" && strings.TrimSpace(factual.Headline) != "" {
		n.Headline = factual.Headline
	}
	if strings.TrimSpace(n.NextStep) == "" && strings.TrimSpace(factual.NextStep) != "" {
		n.NextStep = factual.NextStep
	}
	if len(n.RiskExamples) > 0 {
		n.RiskExamples = nil
	}
}

// buildStockCoverageNarrativeFallback is kept as an alias for tests and legacy callers.
func buildStockCoverageNarrativeFallback(
	dto analytics.DashboardMetricsDTO,
	resp *Response,
	recs []dbgen.Recommendation,
) *StockCoverageNarrative {
	return buildStockCoverageFactualNarrative(dto, resp, recs)
}

func stockNarrativeContainsNoSalesSignal(text string) bool {
	l := strings.ToLower(text)
	return strings.Contains(l, "без продаж") || (strings.Contains(l, "не дал продаж") || strings.Contains(l, "не дали продаж"))
}

func stockNarrativeMentionsProductNames(text string, allowed map[string]struct{}) bool {
	l := strings.ToLower(text)
	for name := range allowed {
		n := strings.TrimSpace(strings.ToLower(name))
		if n == "" {
			continue
		}
		if strings.Contains(l, n) {
			return true
		}
	}
	return false
}

func advertisingLowStockEvidence(recs []dbgen.Recommendation) bool {
	for _, r := range recs {
		if !strings.EqualFold(strings.TrimSpace(r.Status), "open") {
			continue
		}
		if strings.TrimSpace(r.RecommendationType) == "avoid_ads_for_low_stock_sku" {
			return true
		}
	}
	return false
}

func buildStockCoverageContextMap(
	dto analytics.DashboardMetricsDTO,
	resp *Response,
	recs []dbgen.Recommendation,
) map[string]any {
	segments := make([]map[string]any, 0, len(resp.AIDiagnostics.StockCoverage.Segments))
	for _, seg := range resp.AIDiagnostics.StockCoverage.Segments {
		segments = append(segments, map[string]any{
			"key":       seg.Key,
			"label":     seg.Label,
			"sku_count": seg.SKUCount,
			"severity":  seg.Severity,
		})
	}
	rev := computeStockCoverageRevenueAtRisk(dto.SKUs)
	attachRevenueAtRiskShare(&rev, dto.Account.RevenueLast7Days)
	revMap := map[string]any{
		"status": rev.Status,
	}
	if rev.Status == "available" {
		revMap["daily_value"] = rev.FormattedTotal
		revMap["critical_value"] = rev.FormattedCritical
		revMap["shortage_value"] = rev.FormattedShortage
		revMap["method"] = "weighted_avg_daily_revenue_7d"
	}
	risks := buildStockRiskSKURows(dto.SKUs)
	top := risks
	if len(top) > 8 {
		top = top[:8]
	}
	topAny := make([]map[string]any, 0, len(top))
	for _, r := range top {
		row := map[string]any{
			"product_name":         r.ProductName,
			"stock":                r.Stock,
			"risk_level":           r.RiskLevel,
			"avg_daily_revenue_7d": r.AvgDailyRevenue7d,
			"sales_signal":         r.SalesSignal,
			"days_of_cover":        nil,
			"sku":                  r.SKU,
		}
		if r.DaysOfCover != nil {
			row["days_of_cover"] = strings.TrimPrefix(formatDaysOfCoverLabel(r.DaysOfCover), "Покрытие: ")
		}
		topAny = append(topAny, row)
	}
	nsCount, nsUnits := countNoSalesWithStock(dto.SKUs)
	nsExamples := make([]map[string]any, 0, 2)
	for _, sku := range dto.SKUs {
		if sku.StockAvailable <= 0 || sku.RevenueLast7Days > 0 || sku.OrdersLast7Days > 0 {
			continue
		}
		name := derefStr(sku.ProductName, "")
		if name == "" {
			continue
		}
		nsExamples = append(nsExamples, map[string]any{
			"product_name": name,
			"stock":        sku.StockAvailable,
		})
		if len(nsExamples) >= 2 {
			break
		}
	}
	nsMap := map[string]any{
		"count":    nsCount,
		"examples": nsExamples,
	}
	if nsUnits > 0 {
		nsMap["stock_units"] = nsUnits
	}
	ads := map[string]any{"status": "unavailable"}
	if advertisingLowStockEvidence(recs) {
		ads = map[string]any{
			"status":       "available",
			"comment_fact": "есть рекомендация avoid_ads_for_low_stock_sku",
		}
		c := 0
		for _, r := range recs {
			if strings.EqualFold(strings.TrimSpace(r.Status), "open") && strings.TrimSpace(r.RecommendationType) == "avoid_ads_for_low_stock_sku" {
				c++
			}
		}
		if c > 0 {
			ads["linked_low_stock_ads_count"] = c
		}
	}
	allowedNames := make([]string, 0)
	seen := map[string]struct{}{}
	addName := func(n string) {
		n = strings.TrimSpace(n)
		if n == "" {
			return
		}
		if _, ok := seen[n]; ok {
			return
		}
		seen[n] = struct{}{}
		allowedNames = append(allowedNames, n)
	}
	for _, r := range risks {
		addName(r.ProductName)
	}
	for _, ex := range nsExamples {
		if n, ok := ex["product_name"].(string); ok {
			addName(n)
		}
	}
	allowedNums := buildStockCoverageAllowedNumbers(resp, rev, nsCount, nsUnits, risks)
	return map[string]any{
		"segments":              segments,
		"total_skus":            resp.AIDiagnostics.StockCoverage.TotalSKUs,
		"revenue_at_risk":       revMap,
		"top_stock_risks":       topAny,
		"no_sales_with_stock":   nsMap,
		"advertising_link":      ads,
		"allowed_product_names": allowedNames,
		"allowed_numbers":       allowedNums,
	}
}

func buildStockCoverageAllowedNumbers(
	resp *Response,
	rev stockCoverageRevenueAtRisk,
	nsCount, nsUnits int,
	risks []stockCoverageRiskSKU,
) []string {
	seen := map[string]struct{}{}
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
	}
	add(fmt.Sprintf("%d", resp.AIDiagnostics.StockCoverage.TotalSKUs))
	for _, seg := range resp.AIDiagnostics.StockCoverage.Segments {
		add(fmt.Sprintf("%d", seg.SKUCount))
	}
	if rev.Status == "available" {
		add(rev.FormattedTotal)
		add(rev.FormattedCritical)
		add(rev.FormattedShortage)
		add(fmt.Sprintf("%.0f", math.Round(rev.TotalDaily)))
		add(fmt.Sprintf("%.0f", math.Round(rev.CriticalDaily)))
		add(fmt.Sprintf("%.0f", math.Round(rev.ShortageDaily)))
	}
	add(fmt.Sprintf("%d", nsCount))
	add(fmt.Sprintf("%d", nsUnits))
	for _, r := range risks {
		add(fmt.Sprintf("%d", r.Stock))
		add(r.AvgDailyRevenue7d)
		if r.DaysOfCover != nil {
			add(strings.TrimPrefix(formatDaysOfCoverLabel(r.DaysOfCover), "Покрытие: "))
			add(fmt.Sprintf("%.1f", *r.DaysOfCover))
			add(strings.ReplaceAll(fmt.Sprintf("%.1f", *r.DaysOfCover), ".", ","))
		}
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func buildStockCoverageNarrativeValidationContext(
	dto analytics.DashboardMetricsDTO,
	resp *Response,
	recs []dbgen.Recommendation,
) stockCoverageNarrativeValidationContext {
	seg := map[string]int{}
	for _, s := range resp.AIDiagnostics.StockCoverage.Segments {
		seg[s.Key] = s.SKUCount
	}
	rev := computeStockCoverageRevenueAtRisk(dto.SKUs)
	attachRevenueAtRiskShare(&rev, dto.Account.RevenueLast7Days)
	nsC, nsU := countNoSalesWithStock(dto.SKUs)
	risks := buildStockRiskSKURows(dto.SKUs)
	names := map[string]struct{}{}
	for _, sku := range dto.SKUs {
		if n := strings.TrimSpace(derefStr(sku.ProductName, "")); n != "" {
			names[n] = struct{}{}
		}
	}
	nums := map[string]struct{}{}
	for _, s := range buildStockCoverageAllowedNumbers(resp, rev, nsC, nsU, risks) {
		nums[s] = struct{}{}
	}
	return stockCoverageNarrativeValidationContext{
		SegmentCounts:       seg,
		RevenueTotal:        rev.FormattedTotal,
		RevenueCritical:     rev.FormattedCritical,
		RevenueShortage:     rev.FormattedShortage,
		RevenueBackend:      rev,
		AdsEvidence:         advertisingLowStockEvidence(recs),
		NoSalesCount:        nsC,
		NoSalesStockUnits:   nsU,
		AllowedProductNames: names,
		AllowedNumbers:      nums,
	}
}

func applyStockCoverageNarrativeToResponse(
	dto analytics.DashboardMetricsDTO,
	resp *Response,
	recs []dbgen.Recommendation,
	narrativeApplied bool,
	payload *dashboardNarrativePayload,
) {
	if resp.AIDiagnostics.StockCoverage.Status != WidgetAvailable || resp.AIDiagnostics.StockCoverage.TotalSKUs <= 0 {
		return
	}
	ctx := buildStockCoverageNarrativeValidationContext(dto, resp, recs)
	if narrativeApplied && payload != nil && payload.StockCoverageNarrative != nil {
		n := cloneStockCoverageNarrative(payload.StockCoverageNarrative)
		repairStockCoverageNarrativeFromFactual(n, buildStockCoverageFactualNarrative(dto, resp, recs))
		forceStockNarrativeRevenueFromBackend(n, ctx.RevenueBackend)
		repairStockCoverageNarrativeActions(n)
		enforceStockCoverageNoSalesFactual(n, ctx, resp)
		if errs := validateStockCoverageNarrativePayload(n, ctx); len(errs) == 0 {
			polishStockCoverageNarrativeForSeller(n)
			resp.AIDiagnostics.StockCoverage.Narrative = n
			return
		}
		recordNarrativeBlockFallback("stock_coverage_narrative")
	}
	fb := buildStockCoverageFactualNarrative(dto, resp, recs)
	polishStockCoverageNarrativeForSeller(fb)
	if narrativeApplied && (payload == nil || payload.StockCoverageNarrative == nil) {
		recordNarrativeBlockFallback("stock_coverage_narrative")
	}
	resp.AIDiagnostics.StockCoverage.Narrative = fb
}

func cloneStockCoverageNarrative(src *StockCoverageNarrative) *StockCoverageNarrative {
	if src == nil {
		return nil
	}
	dst := *src
	dst.Segments = append([]StockCoverageNarrativeSegment(nil), src.Segments...)
	dst.RiskExamples = append([]StockCoverageRiskExample(nil), src.RiskExamples...)
	dst.Actions = append([]StockCoverageAction(nil), src.Actions...)
	dst.Checklist = append([]StockCoverageAction(nil), src.Checklist...)
	if src.NoSalesFocusLink != nil {
		link := *src.NoSalesFocusLink
		dst.NoSalesFocusLink = &link
	}
	return &dst
}

func forceStockNarrativeRevenueFromBackend(n *StockCoverageNarrative, rev stockCoverageRevenueAtRisk) {
	if n == nil {
		return
	}
	if rev.Status != "available" {
		n.RevenueAtRisk = StockCoverageNarrativeRevenue{
			Status:     "unavailable",
			Label:      "Выручка под риском",
			MethodNote: "Недостаточно данных для оценки выручки под риском.",
		}
		return
	}
	n.RevenueAtRisk.Status = "available"
	n.RevenueAtRisk.Label = "Потенциальная выручка под риском"
	n.RevenueAtRisk.Value = rev.FormattedTotal
	// Seller-facing only; never expose internal method ids in API text.
	n.RevenueAtRisk.MethodNote = sellerFacingRevenueMethodNote
	n.RevenueAtRisk.ShareOfDailyRevenuePct = rev.SharePct
}

func isStockCoverageSummarySubstantive(summary string) bool {
	s := strings.TrimSpace(summary)
	if s == "" {
		return false
	}
	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, "покрытие показывает, на сколько дней") {
		return false
	}
	generic := []string{
		"недостаточно данных",
		"краткая аналитическая сводка",
		"аналитика по остаткам",
	}
	for _, g := range generic {
		if lower == g || strings.HasPrefix(lower, g+".") {
			return false
		}
	}
	return len([]rune(s)) >= 24
}

func stockCoverageNarrativeInterpretationText(n *StockCoverageNarrative) string {
	if n == nil {
		return ""
	}
	return strings.TrimSpace(n.Summary + " " + n.RiskComment + " " + n.NoSalesInsight)
}

func isStockCoverageActionGeneric(title, text string) bool {
	combined := strings.ToLower(strings.TrimSpace(title + " " + text))
	if combined == "" {
		return true
	}
	generic := []string{"проверьте всё", "что сделать", "нет рекомендаций", "нет данных"}
	for _, g := range generic {
		if combined == g {
			return true
		}
	}
	return false
}

func repairStockCoverageActionTarget(target *StockCoverageActionTarget) *StockCoverageActionTarget {
	if target == nil {
		return nil
	}
	t := strings.TrimSpace(target.Type)
	tab := strings.TrimSpace(target.Tab)
	if t == "" {
		return nil
	}
	switch t {
	case "sku_focus_tab":
		if tab == "" {
			tab = "stock"
		}
		return &StockCoverageActionTarget{Type: t, Tab: tab}
	default:
		return nil
	}
}

func repairStockCoverageNarrativeActions(n *StockCoverageNarrative) {
	if n == nil {
		return
	}
	raw := n.Actions
	if len(raw) == 0 {
		raw = n.Checklist
	}
	repaired := make([]StockCoverageAction, 0, len(raw))
	for _, action := range raw {
		title := cleanupSellerFacingText(action.Title)
		text := cleanupSellerFacingText(action.Text)
		if title == "" && text == "" {
			continue
		}
		if isStockCoverageActionGeneric(title, text) {
			continue
		}
		if title == "" {
			title = text
			text = ""
		}
		repaired = append(repaired, StockCoverageAction{
			Title:    title,
			Text:     text,
			Priority: strings.TrimSpace(action.Priority),
			Target:   repairStockCoverageActionTarget(action.Target),
		})
		if len(repaired) >= 4 {
			break
		}
	}
	n.Actions = repaired
	n.Checklist = nil
}

func enforceStockCoverageNoSalesFactual(n *StockCoverageNarrative, ctx stockCoverageNarrativeValidationContext, resp *Response) {
	if n == nil {
		return
	}
	count := ctx.NoSalesCount
	shown := 0
	if resp != nil {
		if resp.SKUFocus.NoSalesWithStockTotalCount > count {
			count = resp.SKUFocus.NoSalesWithStockTotalCount
		}
		shown = len(resp.SKUFocus.NoSalesWithStock)
	}
	if count <= 0 {
		n.NoSalesWithStock = StockCoverageNoSalesWithStock{Status: "unavailable"}
		n.NoSalesFocusLink = nil
		return
	}
	n.NoSalesWithStock.Status = "available"
	n.NoSalesWithStock.Count = count
	n.NoSalesWithStock.StockUnits = ctx.NoSalesStockUnits
	if shown > 0 {
		n.NoSalesWithStock.ShownCount = shown
	}
	if strings.TrimSpace(n.NoSalesWithStock.Summary) == "" {
		displayed := shown
		if displayed <= 0 {
			displayed = minInt(count, 6)
		}
		n.NoSalesWithStock.Summary = fmt.Sprintf(
			"Есть %d SKU с остатком и без заказов за период; первые %d показаны в SKU-фокусе.",
			count, displayed,
		)
	}
	n.NoSalesWithStock.Target = stockNoSalesSKUFocusTarget
	if n.NoSalesFocusLink == nil {
		n.NoSalesFocusLink = &StockCoverageSKUFocusLink{
			Label:  defaultNoSalesFocusLinkLabel,
			Target: stockNoSalesSKUFocusTarget,
		}
	}
}

func validateStockCoverageNarrativePayload(n *StockCoverageNarrative, ctx stockCoverageNarrativeValidationContext) []string {
	var errs []string
	if n == nil {
		return []string{"nil_stock_coverage_narrative"}
	}
	if strings.TrimSpace(n.Status) == "" {
		errs = append(errs, "stock_narrative_missing_status")
	}
	if !isStockCoverageSummarySubstantive(n.Summary) {
		errs = append(errs, "stock_narrative_missing_summary")
	}
	if len(n.Segments) != len(stockNarrativeSegmentOrder) {
		errs = append(errs, "stock_narrative_segments_len")
	}
	segByKey := map[string]StockCoverageNarrativeSegment{}
	for _, s := range n.Segments {
		segByKey[s.Key] = s
	}
	for _, key := range stockNarrativeSegmentOrder {
		got, ok := segByKey[key]
		if !ok {
			errs = append(errs, "stock_narrative_missing_segment_"+key)
			continue
		}
		want, wok := ctx.SegmentCounts[key]
		if !wok || got.Count != want {
			errs = append(errs, "stock_narrative_segment_count_"+key)
		}
	}
	if n.RevenueAtRisk.Status == "available" {
		if strings.TrimSpace(n.RevenueAtRisk.Value) != strings.TrimSpace(ctx.RevenueTotal) {
			errs = append(errs, "stock_narrative_revenue_value_mismatch")
		}
	}
	for _, ex := range n.RiskExamples {
		if _, ok := ctx.AllowedProductNames[strings.TrimSpace(ex.ProductName)]; !ok {
			errs = append(errs, "stock_narrative_unknown_product")
			break
		}
	}
	if len(n.RiskExamples) > 3 {
		errs = append(errs, "stock_narrative_too_many_examples")
	}
	for _, ex := range n.RiskExamples {
		l := strings.ToLower(ex.RiskLabel)
		for _, bad := range stockNarrativeForbiddenExampleLabels {
			if strings.Contains(l, bad) {
				errs = append(errs, "stock_narrative_forbidden_example_label")
			}
		}
	}
	interpretation := stockCoverageNarrativeInterpretationText(n)
	textWithoutAdsBlock := strings.ToLower(interpretation)
	if stockNarrativeMentionsProductNames(interpretation, ctx.AllowedProductNames) {
		errs = append(errs, "stock_narrative_summary_mentions_product_names")
	}
	if strings.Contains(strings.ToLower(interpretation), "зоне 0–3") || strings.Contains(strings.ToLower(interpretation), "зоне 4–7") {
		errs = append(errs, "stock_narrative_summary_uses_internal_zone_term")
	}
	if ctx.NoSalesCount > 0 && !stockNarrativeContainsNoSalesSignal(interpretation) &&
		!stockNarrativeContainsNoSalesSignal(n.NoSalesInsight) {
		errs = append(errs, "stock_narrative_missing_no_sales_signal")
	}
	for _, v := range stockNarrativeForbiddenActionVerbs {
		if strings.Contains(textWithoutAdsBlock, v) {
			errs = append(errs, "stock_narrative_forbidden_action_verb")
			break
		}
	}
	if !ctx.AdsEvidence {
		if mentionsAdvertising(textWithoutAdsBlock) {
			errs = append(errs, "stock_narrative_ads_without_evidence")
		}
		if n.AdvertisingLink.Status == "available" && strings.TrimSpace(n.AdvertisingLink.Comment) != "" {
			errs = append(errs, "stock_narrative_ads_block_without_evidence")
		}
	}
	if n.SKUFocusLink.Target != stockSKUFocusTarget {
		errs = append(errs, "stock_narrative_bad_sku_focus_target")
	}
	if ctx.NoSalesCount == 0 {
		if n.NoSalesWithStock.Status == "available" && n.NoSalesWithStock.Count != 0 {
			errs = append(errs, "stock_narrative_no_sales_invented")
		}
	} else {
		if n.NoSalesWithStock.Status != "available" {
			errs = append(errs, "stock_narrative_no_sales_status")
		} else if n.NoSalesWithStock.Count != ctx.NoSalesCount {
			errs = append(errs, "stock_narrative_no_sales_count_mismatch")
		} else if ctx.NoSalesStockUnits > 0 && n.NoSalesWithStock.StockUnits != ctx.NoSalesStockUnits {
			errs = append(errs, "stock_narrative_no_sales_units_mismatch")
		}
	}
	return errs
}

func normalizeStockCoverageNarrativePayload(n *StockCoverageNarrative) {
	if n == nil {
		return
	}
	n.Status = strings.TrimSpace(n.Status)
	n.Summary = cleanupSellerFacingText(n.Summary)
	n.NoSalesInsight = cleanupSellerFacingText(n.NoSalesInsight)
	n.RiskComment = cleanupSellerFacingText(n.RiskComment)
	repairStockCoverageNarrativeActions(n)
	n.RevenueAtRisk.Status = strings.TrimSpace(n.RevenueAtRisk.Status)
	n.RevenueAtRisk.Label = cleanupSellerFacingText(n.RevenueAtRisk.Label)
	n.RevenueAtRisk.Value = strings.TrimSpace(n.RevenueAtRisk.Value)
	n.RevenueAtRisk.MethodNote = cleanupSellerFacingText(n.RevenueAtRisk.MethodNote)
	for i := range n.Segments {
		n.Segments[i].Key = strings.TrimSpace(n.Segments[i].Key)
		n.Segments[i].Label = cleanupSellerFacingText(n.Segments[i].Label)
		n.Segments[i].Severity = strings.TrimSpace(n.Segments[i].Severity)
		n.Segments[i].Comment = cleanupSellerFacingText(n.Segments[i].Comment)
	}
	for i := range n.RiskExamples {
		n.RiskExamples[i].ProductName = strings.TrimSpace(n.RiskExamples[i].ProductName)
		n.RiskExamples[i].StockLabel = cleanupSellerFacingText(n.RiskExamples[i].StockLabel)
		n.RiskExamples[i].CoverageLabel = cleanupSellerFacingText(n.RiskExamples[i].CoverageLabel)
		n.RiskExamples[i].RiskLabel = cleanupSellerFacingText(n.RiskExamples[i].RiskLabel)
		n.RiskExamples[i].Reason = cleanupSellerFacingText(n.RiskExamples[i].Reason)
	}
	n.NoSalesWithStock.Status = strings.TrimSpace(n.NoSalesWithStock.Status)
	n.NoSalesWithStock.Comment = cleanupSellerFacingText(n.NoSalesWithStock.Comment)
	n.AdvertisingLink.Status = strings.TrimSpace(n.AdvertisingLink.Status)
	n.AdvertisingLink.Comment = cleanupSellerFacingText(n.AdvertisingLink.Comment)
	n.SKUFocusLink.Label = cleanupSellerFacingText(n.SKUFocusLink.Label)
	n.SKUFocusLink.Target = strings.TrimSpace(n.SKUFocusLink.Target)
	if n.NoSalesFocusLink != nil {
		n.NoSalesFocusLink.Label = cleanupSellerFacingText(n.NoSalesFocusLink.Label)
		n.NoSalesFocusLink.Target = strings.TrimSpace(n.NoSalesFocusLink.Target)
	}
}

func mentionsAdvertising(s string) bool {
	return strings.Contains(s, "реклам") || strings.Contains(s, "трафик") || strings.Contains(s, "продвижен")
}

// ValidateStockCoverageNarrativePayloadExported is exported for tests.
func ValidateStockCoverageNarrativePayloadExported(n *StockCoverageNarrative, ctx stockCoverageNarrativeValidationContext) []string {
	return validateStockCoverageNarrativePayload(n, ctx)
}

// BuildStockCoverageNarrativeFallbackExported is exported for tests.
func BuildStockCoverageNarrativeFallbackExported(dto analytics.DashboardMetricsDTO, resp *Response, recs []dbgen.Recommendation) *StockCoverageNarrative {
	return buildStockCoverageNarrativeFallback(dto, resp, recs)
}

// ComputeStockCoverageRevenueAtRiskExported is exported for tests.
func ComputeStockCoverageRevenueAtRiskExported(skus []analytics.DashboardSKUMetricsItem) stockCoverageRevenueAtRisk {
	return computeStockCoverageRevenueAtRisk(skus)
}
