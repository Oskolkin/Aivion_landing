package dashboardhome

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysissource"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/stockpolicy"
)

const FrozenDashboardDataSchemaVersion = "dashboard_data_snapshot_v1"

// FrozenDashboardData captures deterministic seller-facing dashboard widgets at pipeline generation time.
type FrozenDashboardData struct {
	SchemaVersion    string                        `json:"schema_version"`
	MetricDate       string                        `json:"metric_date"`
	DashboardMetrics analytics.DashboardMetricsDTO `json:"dashboard_metrics"`
	SellerPulse      SellerPulse                   `json:"seller_pulse"`
	SKUFocus         SKUFocus                      `json:"sku_focus"`
	StockCoverage    StockCoverageWidget           `json:"stock_coverage"`
	AdEfficiency     AdEfficiencyWidget            `json:"ad_efficiency"`
	SalesFunnel      SalesFunnelWidget             `json:"sales_funnel"`
	FunnelAnalysis   FunnelAnalysisWidget          `json:"funnel_analysis"`
	RiskDistribution RiskDistributionWidget        `json:"risk_distribution"`

	// Date freshness metadata (Unified Analysis date modes). Optional for legacy snapshots.
	RequestedDate          string `json:"requested_date,omitempty"`
	EffectiveMetricDate    string `json:"effective_metric_date,omitempty"`
	LatestCompleteDataDate string `json:"latest_complete_data_date,omitempty"`
	// DataFreshnessLagDays/DataIsStale always describe SOURCE staleness
	// (latest_complete_data_date lag vs today) — see analysisdata.MetricDateResolution.
	DataFreshnessLagDays int  `json:"data_freshness_lag_days,omitempty"`
	DataIsStale          bool `json:"data_is_stale,omitempty"`
	// HistoricalAnalysis/AnalysisAgeDays/SourceFreshnessLagDays let read-time
	// rendering (ApplyFrozenDashboardData) distinguish a deliberate
	// fixed-date historical analysis from genuine source staleness, so a
	// large analysis age never surfaces as "data not updated" on its own.
	HistoricalAnalysis     bool `json:"historical_analysis,omitempty"`
	AnalysisAgeDays        int  `json:"analysis_age_days,omitempty"`
	SourceFreshnessLagDays int  `json:"source_freshness_lag_days,omitempty"`
}

func NewFrozenDashboardData(metricDate string, dto analytics.DashboardMetricsDTO, resp *Response) FrozenDashboardData {
	out := FrozenDashboardData{
		SchemaVersion:    FrozenDashboardDataSchemaVersion,
		MetricDate:       metricDate,
		DashboardMetrics: dto,
	}
	if resp == nil {
		return out
	}
	out.SellerPulse = resp.SellerPulse
	out.SKUFocus = resp.SKUFocus
	out.StockCoverage = resp.AIDiagnostics.StockCoverage
	out.AdEfficiency = resp.AIDiagnostics.AdEfficiency
	out.SalesFunnel = resp.AIDiagnostics.SalesFunnel
	out.FunnelAnalysis = resp.AIDiagnostics.FunnelAnalysis
	out.RiskDistribution = resp.AIDiagnostics.RiskDistribution
	return out
}

func (f FrozenDashboardData) CanonicalHash() (string, error) {
	canonical, err := canonicalJSON(f)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func canonicalJSON(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, err
	}
	normalized := normalizeJSONValue(decoded)
	return json.Marshal(normalized)
}

func normalizeJSONValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out := make(map[string]any, len(t))
		for _, k := range keys {
			out[k] = normalizeJSONValue(t[k])
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i := range t {
			out[i] = normalizeJSONValue(t[i])
		}
		return out
	default:
		return v
	}
}

func ApplyFrozenDashboardData(resp *Response, frozen FrozenDashboardData) error {
	return applyFrozenDashboardData(resp, frozen, true)
}

// ApplyFrozenDashboardDataUnified applies a frozen snapshot without semantic
// order/revenue delta enrichment. Stock-tab membership is still reconciled from
// frozen metrics so Stock Coverage counters and SKU Focus filters stay aligned.
func ApplyFrozenDashboardDataUnified(resp *Response, frozen FrozenDashboardData) error {
	return applyFrozenDashboardData(resp, frozen, false)
}

func applyFrozenDashboardData(resp *Response, frozen FrozenDashboardData, enrichSKUFocus bool) error {
	if resp == nil {
		return fmt.Errorf("response is required")
	}
	if strings.TrimSpace(frozen.SchemaVersion) == "" {
		return fmt.Errorf("frozen dashboard schema_version is required")
	}
	resp.MetricDate = frozen.MetricDate
	resp.RequestedDate = frozen.RequestedDate
	resp.EffectiveMetricDate = firstNonEmpty(frozen.EffectiveMetricDate, frozen.MetricDate)
	resp.LatestCompleteDataDate = frozen.LatestCompleteDataDate
	resp.DataFreshnessLagDays = frozen.DataFreshnessLagDays
	resp.DataIsStale = frozen.DataIsStale
	resp.HistoricalAnalysis = frozen.HistoricalAnalysis
	resp.AnalysisAgeDays = frozen.AnalysisAgeDays
	resp.SourceFreshnessLagDays = frozen.SourceFreshnessLagDays
	if note := DateFreshnessNote(frozen.HistoricalAnalysis, frozen.DataIsStale, frozen.DataFreshnessLagDays, resp.EffectiveMetricDate); note != "" {
		already := false
		for _, existing := range resp.DataQualityNotes {
			if existing == note {
				already = true
				break
			}
		}
		if !already {
			resp.DataQualityNotes = append(resp.DataQualityNotes, note)
		}
	}
	resp.SellerPulse = frozen.SellerPulse
	resp.SKUFocus = frozen.SKUFocus
	resp.AIDiagnostics.StockCoverage = frozen.StockCoverage
	resp.AIDiagnostics.AdEfficiency = frozen.AdEfficiency
	resp.AIDiagnostics.SalesFunnel = frozen.SalesFunnel
	resp.AIDiagnostics.FunnelAnalysis = frozen.FunnelAnalysis
	resp.AIDiagnostics.RiskDistribution = frozen.RiskDistribution
	normalizeFrozenWidgetStatuses(resp)
	// Keep Stock Coverage ↔ SKU Focus stock-tab membership aligned with the
	// canonical stockpolicy segments already present in frozen metrics.
	reconcileFrozenStockFocusMembership(&resp.SKUFocus, frozen.DashboardMetrics)
	// Legacy snapshots may omit order/revenue deltas on SKU Focus cards; fill
	// from the frozen metrics DTO only when the caller opts in. Unified
	// publications must keep published SKU Focus semantics untouched on GET.
	if enrichSKUFocus {
		enrichFrozenSKUFocusFromMetrics(&resp.SKUFocus, frozen.DashboardMetrics)
	}
	if frozen.DashboardMetrics.SellerAccountID > 0 {
		hasAccount := frozen.DashboardMetrics.Account.RevenueToday > 0 ||
			frozen.DashboardMetrics.Account.OrdersToday > 0 ||
			frozen.DashboardMetrics.Account.RevenueLast7Days > 0 ||
			len(frozen.DashboardMetrics.SKUs) > 0
		hasPulse := false
		for _, kpi := range frozen.SellerPulse.KPIs {
			key := strings.ToLower(strings.TrimSpace(kpi.Key))
			switch key {
			case "revenue", "orders", "average_order_value", "ad_spend_share_total_revenue", "ctr", "drr":
				if strings.TrimSpace(kpi.Value) != "" {
					hasPulse = true
				}
			}
		}
		if hasAccount || hasPulse || len(frozen.SellerPulse.Trend7D) > 0 {
			resp.Status = StatusReady
		}
	}
	return nil
}

// applyFrozenForAnalysisSource applies a frozen snapshot. Unified analysis
// sources skip SKU Focus semantic enrichment so published membership/deltas
// stay frozen on GET.
func applyFrozenForAnalysisSource(resp *Response, frozen FrozenDashboardData, analysisSource string) error {
	if analysisSource == analysissource.SourceUnified {
		return ApplyFrozenDashboardDataUnified(resp, frozen)
	}
	return ApplyFrozenDashboardData(resp, frozen)
}

// normalizeFrozenWidgetStatuses maps legacy factual status aliases (notably
// dashboardfacts historically emitted "ok") onto the dashboardhome WidgetStatus
// vocabulary expected by the frontend ("available").
func normalizeFrozenWidgetStatuses(resp *Response) {
	if resp == nil {
		return
	}
	resp.AIDiagnostics.StockCoverage.Status = normalizeFactualWidgetStatus(resp.AIDiagnostics.StockCoverage.Status)
	resp.AIDiagnostics.AdEfficiency.Status = normalizeFactualWidgetStatus(resp.AIDiagnostics.AdEfficiency.Status)
	resp.AIDiagnostics.SalesFunnel.Status = normalizeFactualWidgetStatus(resp.AIDiagnostics.SalesFunnel.Status)
	resp.AIDiagnostics.FunnelAnalysis.Status = normalizeFactualWidgetStatus(resp.AIDiagnostics.FunnelAnalysis.Status)
	resp.AIDiagnostics.FunnelAnalysis.AIAnalysis.Status = normalizeFactualWidgetStatus(resp.AIDiagnostics.FunnelAnalysis.AIAnalysis.Status)
	resp.AIDiagnostics.RiskDistribution.Status = normalizeFactualWidgetStatus(resp.AIDiagnostics.RiskDistribution.Status)
}

func normalizeFactualWidgetStatus(status WidgetStatus) WidgetStatus {
	switch strings.TrimSpace(string(status)) {
	case "ok", "ready":
		return WidgetAvailable
	default:
		return status
	}
}

// reconcileFrozenStockFocusMembership ensures SKU Focus → Остатки contains every
// SKU whose canonical coverage segment is in the stock tab
// (out_of_stock / critical_0_3 / shortage_4_7 / planning_8_14), matching Stock
// Coverage counters. Existing AI problem/key_metric copy is preserved.
func reconcileFrozenStockFocusMembership(focus *SKUFocus, metrics analytics.DashboardMetricsDTO) {
	if focus == nil || len(metrics.SKUs) == 0 {
		return
	}

	existingByID := make(map[int64]SKUFocusCard, len(focus.StockRisk))
	for _, card := range focus.StockRisk {
		if card.OzonProductID == 0 {
			continue
		}
		existingByID[card.OzonProductID] = card
	}

	type candidate struct {
		sku    analytics.DashboardSKUMetricsItem
		bucket string
		rank   int
	}
	var cands []candidate
	attentionTotal := 0
	for _, sku := range metrics.SKUs {
		bucket := analytics.StockCoverageBucketKey(sku)
		if !stockpolicy.IsStockFocusStockTabSegment(bucket) {
			continue
		}
		if stockpolicy.IsStockFocusAttentionSegment(bucket) {
			attentionTotal++
		}
		cands = append(cands, candidate{sku: sku, bucket: bucket, rank: stockFocusSegmentRank(bucket)})
	}
	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].rank != cands[j].rank {
			return cands[i].rank < cands[j].rank
		}
		ci, cj := coverageOrMax(cands[i].sku.DaysOfCover), coverageOrMax(cands[j].sku.DaysOfCover)
		if ci != cj {
			return ci < cj
		}
		ri, rj := cands[i].sku.RevenueLast7Days, cands[j].sku.RevenueLast7Days
		if ri != rj {
			return ri > rj
		}
		return cands[i].sku.OzonProductID < cands[j].sku.OzonProductID
	})

	out := make([]SKUFocusCard, 0, len(cands))
	for _, cand := range cands {
		if card, ok := existingByID[cand.sku.OzonProductID]; ok {
			card.CoverageBucket = cand.bucket
			if card.CoverageDays == nil {
				card.CoverageDays = cand.sku.DaysOfCover
			}
			if card.CurrentStock == nil {
				stock := int32(cand.sku.StockAvailable)
				card.CurrentStock = &stock
			}
			out = append(out, card)
			continue
		}
		out = append(out, factualStockFocusCardFromMetrics(cand.sku, cand.bucket))
	}
	focus.StockRisk = out
	focus.StockRiskTotalCount = attentionTotal
}

func stockFocusSegmentRank(bucket string) int {
	switch bucket {
	case stockpolicy.SegmentOutOfStock:
		return 0
	case stockpolicy.SegmentCritical0_3:
		return 1
	case stockpolicy.SegmentShortage4_7:
		return 2
	case stockpolicy.SegmentPlanning8_14:
		return 3
	default:
		return 9
	}
}

func coverageOrMax(v *float64) float64 {
	if v == nil {
		return 1e18
	}
	return *v
}

func factualStockFocusCardFromMetrics(sku analytics.DashboardSKUMetricsItem, bucket string) SKUFocusCard {
	stock := int32(sku.StockAvailable)
	name := ""
	if sku.ProductName != nil {
		name = strings.TrimSpace(*sku.ProductName)
	}
	card := SKUFocusCard{
		OzonProductID:  sku.OzonProductID,
		SKU:            sku.SKU,
		OfferID:        sku.OfferID,
		ProductName:    name,
		Problem:        "Риск по остатку",
		KeyMetric:      stockFocusKeyMetricFromMetrics(sku),
		CurrentStock:   &stock,
		CoverageDays:   sku.DaysOfCover,
		CoverageBucket: bucket,
	}
	if stockpolicy.IsStockFocusAttentionSegment(bucket) && sku.RevenueLast7Days > 0 {
		v := sku.RevenueLast7Days / 7
		card.RevenueAtRisk7d = &v
		card.AvgDailyRevenue7d = &v
	}
	return card
}

func stockFocusKeyMetricFromMetrics(sku analytics.DashboardSKUMetricsItem) string {
	if sku.DaysOfCover != nil {
		return fmt.Sprintf("Покрытие: %.1f дн · остаток %d", *sku.DaysOfCover, sku.StockAvailable)
	}
	if sku.StockAvailable <= 0 {
		return "Нет остатка"
	}
	return fmt.Sprintf("Остаток %d", sku.StockAvailable)
}

// enrichFrozenSKUFocusFromMetrics fills order/revenue deltas on SKU Focus cards
// from frozen DashboardMetrics SKU rows. Older unified publications joined only
// stock/coverage into sku_focus; without deltas the frontend shows
// "Нет подтверждённых метрик просадки".
func enrichFrozenSKUFocusFromMetrics(focus *SKUFocus, metrics analytics.DashboardMetricsDTO) {
	if focus == nil || len(metrics.SKUs) == 0 {
		return
	}
	byProduct := make(map[int64]analytics.DashboardSKUMetricsItem, len(metrics.SKUs))
	for _, sku := range metrics.SKUs {
		byProduct[sku.OzonProductID] = sku
	}
	enrich := func(cards []SKUFocusCard) {
		for i := range cards {
			sku, ok := byProduct[cards[i].OzonProductID]
			if !ok {
				continue
			}
			orders := sku.OrdersCount
			prev := sku.OrdersCount - sku.OrdersDeltaDayToDay
			ordersDelta := sku.OrdersDeltaDayToDay
			revenueDelta := sku.RevenueDeltaDayToDay
			if cards[i].CurrentOrders == nil {
				cards[i].CurrentOrders = &orders
			}
			if cards[i].PreviousOrders == nil {
				cards[i].PreviousOrders = &prev
			}
			if cards[i].PreviousDayOrders == nil {
				cards[i].PreviousDayOrders = &prev
			}
			if cards[i].OrdersDeltaDayToDay == nil {
				cards[i].OrdersDeltaDayToDay = &ordersDelta
			}
			if cards[i].RevenueDeltaDayToDay == nil {
				cards[i].RevenueDeltaDayToDay = &revenueDelta
			}
		}
	}
	enrich(focus.OrderDrop)
	enrich(focus.StockRisk)
	enrich(focus.NoSalesWithStock)
	enrich(focus.SlowTurnover)
}

// DateFreshnessNote picks the seller-facing data-quality note for date
// freshness. A historical (fixed-date) analysis must never surface "data not
// updated N days" based on analysis age alone — that lag is expected and
// intentional. It only gets a staleness note when the underlying sources
// themselves are stale (source freshness lag over threshold); otherwise it
// gets a neutral "historical analysis as of DD.MM.YYYY" note instead.
//
// Shared by ApplyFrozenDashboardData (read-time projection from a frozen
// snapshot) and adapter.ApplyDateFreshness (publish-time projection from a
// fresh analysisdata.MetricDateResolution) so both paths render identical
// copy from the same historical/staleness signals.
func DateFreshnessNote(historical, isStale bool, lagDays int, effectiveDate string) string {
	if historical {
		if isStale && lagDays > 0 {
			return fmt.Sprintf("Данные не обновлялись %d дн.", lagDays)
		}
		if effectiveDate != "" {
			return fmt.Sprintf("Исторический анализ по данным на %s", formatDateRuDDMMYYYY(effectiveDate))
		}
		return ""
	}
	if isStale && lagDays > 0 {
		return fmt.Sprintf("Данные не обновлялись %d дн.", lagDays)
	}
	return ""
}

// FormatDateRuDDMMYYYY formats an ISO (YYYY-MM-DD) date as DD.MM.YYYY for
// seller-facing Russian copy. Returns the input unchanged if it cannot be
// parsed.
func FormatDateRuDDMMYYYY(iso string) string {
	return formatDateRuDDMMYYYY(iso)
}

func formatDateRuDDMMYYYY(iso string) string {
	t, err := time.Parse("2006-01-02", iso)
	if err != nil {
		return iso
	}
	return t.Format("02.01.2006")
}

func ParseFrozenDashboardData(raw []byte) (FrozenDashboardData, error) {
	var out FrozenDashboardData
	if len(raw) == 0 {
		return out, fmt.Errorf("empty frozen dashboard payload")
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, fmt.Errorf("decode frozen dashboard payload: %w", err)
	}
	if strings.TrimSpace(out.SchemaVersion) == "" {
		return out, fmt.Errorf("frozen dashboard schema_version is required")
	}
	return out, nil
}
