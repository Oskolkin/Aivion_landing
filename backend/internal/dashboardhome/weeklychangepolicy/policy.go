// Package weeklychangepolicy builds the backend-owned weekly-change candidate
// registry that grounds Dashboard "Что изменилось за неделю" in Dashboard Facts.
//
// Contract: AI may only select change_ids from this registry, write seller-facing
// copy, and set display priority. It must NEVER invent previous/current values,
// change_type, or entity sets. One change_id belongs to exactly one group.
package weeklychangepolicy

import (
	"math"
	"sort"
	"strings"
)

// PolicyVersion pins the candidate-building formula. Bump when thresholds or
// classification rules change in a way that alters published group membership.
const PolicyVersion = "weekly_change_policy_v1"

// ChangeType is mutually exclusive across the four Dashboard groups.
type ChangeType string

const (
	ChangeWorsened      ChangeType = "worsened"
	ChangeNewRisk       ChangeType = "new_risk"
	ChangeActiveProblem ChangeType = "active_problem"
	ChangeImproved      ChangeType = "improved"
)

// Group keys match the home-compatible what_changed contract.
const (
	GroupKeyWorsened       = "worsened"
	GroupKeyNewRisks       = "new_risks"
	GroupKeyUnchangedRisks = "unchanged_risks" // UI label: Активные проблемы
	GroupKeyImproved       = "improved"
)

// Metric codes used as stable change_id prefixes / fingerprint keys.
const (
	MetricRevenue7d          = "sales.revenue_7d"
	MetricOrders7d           = "sales.orders_7d"
	MetricStockoutSKUCount   = "stock.stockout_sku_count"
	MetricCriticalSKUCount   = "stock.critical_sku_count"
	MetricShortageSKUCount   = "stock.shortage_sku_count"
	MetricNoSalesWithStock   = "no_sales.with_stock_count"
	MetricSurplusSKUCount    = "surplus.sku_count"
	MetricHighDRRCampaigns   = "advertising.high_drr_campaign_count"
	MetricSpendWithoutOrders = "advertising.spend_without_orders"
	MetricAdShareOfRevenue   = "advertising.ad_share_of_revenue_pct"
	MetricFunnelViewToCart   = "funnel.view_to_cart_conversion"
	MetricFunnelCartToOrder  = "funnel.cart_to_order_conversion"
)

// ChangeCandidate is one backend-computed, facts-grounded weekly change.
type ChangeCandidate struct {
	ChangeID             string     `json:"change_id"`
	Domain               string     `json:"domain"`
	ChangeType           ChangeType `json:"change_type"`
	MetricCode           string     `json:"metric_code"`
	TitleCandidate       string     `json:"title_candidate"`
	DescriptionCandidate string     `json:"description_candidate"`
	CurrentValue         *float64   `json:"current_value,omitempty"`
	PreviousValue        *float64   `json:"previous_value,omitempty"`
	CurrentValueText     string     `json:"current_value_text,omitempty"`
	PreviousValueText    string     `json:"previous_value_text,omitempty"`
	AbsoluteDelta        *float64   `json:"absolute_delta,omitempty"`
	RelativeDeltaPct     *float64   `json:"relative_delta_pct,omitempty"`
	DeltaPp              *float64   `json:"delta_pp,omitempty"`
	DeltaText            string     `json:"delta_text,omitempty"`
	CurrentPeriod        string     `json:"current_period,omitempty"`
	PreviousPeriod       string     `json:"previous_period,omitempty"`
	EntityType           string     `json:"entity_type,omitempty"`
	EntityIDs            []string   `json:"entity_ids,omitempty"`
	EntityNames          []string   `json:"entity_names,omitempty"`
	EntityCount          int        `json:"entity_count,omitempty"`
	FinancialImpact      *float64   `json:"financial_impact,omitempty"`
	ImpactText           string     `json:"impact_text,omitempty"`
	Severity             string     `json:"severity,omitempty"`
	MaterialityScore     int        `json:"materiality_score"`
	EvidenceFactIDs      []string   `json:"evidence_fact_ids,omitempty"`
	RelatedDriverID      string     `json:"related_driver_id,omitempty"`
	RelatedActionID      string     `json:"related_action_id,omitempty"`
	ActionLinkLabel      string     `json:"action_link_label,omitempty"`
	DeepLink             string     `json:"deep_link,omitempty"`
	Confidence           string     `json:"confidence,omitempty"`
	Limitations          []string   `json:"limitations,omitempty"`
	HistoryRequired      bool       `json:"history_required,omitempty"`
}

// PreviousContext carries proof for new_risk vs active_problem classification.
type PreviousContext struct {
	// HistoryAvailable is true when previous publication fingerprints or a
	// previous facts snapshot can ground problem continuity. When false,
	// active_problems must use the history-insufficient empty state — never
	// "no long-lived problems". KPI week deltas alone do NOT set this flag.
	HistoryAvailable       bool               `json:"history_available"`
	HasPreviousPublication bool               `json:"has_previous_publication"`
	HasPreviousFacts       bool               `json:"has_previous_facts"`
	PeriodLabel            string             `json:"period_label,omitempty"`
	Problems               map[string]Problem `json:"problems,omitempty"`
	Metrics                map[string]float64 `json:"metrics,omitempty"`
}

// Problem is one fingerprint of a material problem observed previously.
type Problem struct {
	MetricCode string   `json:"metric_code"`
	DriverID   string   `json:"driver_id,omitempty"`
	EntityIDs  []string `json:"entity_ids,omitempty"`
	Value      float64  `json:"value,omitempty"`
	Present    bool     `json:"present"`
}

// RegistryResult is the full deterministic weekly-change candidate set.
type RegistryResult struct {
	PolicyVersion    string            `json:"policy_version"`
	HistoryAvailable bool              `json:"history_available"`
	CurrentPeriod    string            `json:"current_period,omitempty"`
	PreviousPeriod   string            `json:"previous_period,omitempty"`
	Candidates       []ChangeCandidate `json:"candidates"`
	ByChangeType     map[ChangeType][]ChangeCandidate
	EmptyTextByGroup map[string]string `json:"empty_text_by_group"`
	// Limitations records contract caveats (e.g. previous_period_facts_cloned_from_current).
	Limitations []string `json:"limitations,omitempty"`
}

// GroupKeyForChangeType maps change_type → home group key.
func GroupKeyForChangeType(t ChangeType) string {
	switch t {
	case ChangeWorsened:
		return GroupKeyWorsened
	case ChangeNewRisk:
		return GroupKeyNewRisks
	case ChangeActiveProblem:
		return GroupKeyUnchangedRisks
	case ChangeImproved:
		return GroupKeyImproved
	default:
		return ""
	}
}

// SortCandidates orders by materiality, severity, financial impact, entity count.
func SortCandidates(in []ChangeCandidate) []ChangeCandidate {
	out := append([]ChangeCandidate(nil), in...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.MaterialityScore != b.MaterialityScore {
			return a.MaterialityScore > b.MaterialityScore
		}
		if sevRank(a.Severity) != sevRank(b.Severity) {
			return sevRank(a.Severity) > sevRank(b.Severity)
		}
		ai, bi := absOrZero(a.FinancialImpact), absOrZero(b.FinancialImpact)
		if ai != bi {
			return ai > bi
		}
		if a.EntityCount != b.EntityCount {
			return a.EntityCount > b.EntityCount
		}
		return a.ChangeID < b.ChangeID
	})
	return out
}

func sevRank(s string) int {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "critical":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

func absOrZero(v *float64) float64 {
	if v == nil || !isFinite(*v) {
		return 0
	}
	return math.Abs(*v)
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

func f64(v float64) *float64 { return &v }

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// EmptyTextForGroup returns seller-facing empty copy. When history is missing,
// active_problems must NOT claim that long-lived problems are absent.
func EmptyTextForGroup(groupKey string, historyAvailable bool) string {
	switch groupKey {
	case GroupKeyWorsened:
		return "Заметных ухудшений за неделю не выявлено."
	case GroupKeyNewRisks:
		return "Новых рисков не выявлено. В текущей неделе не появились новые материальные проблемы."
	case GroupKeyUnchangedRisks:
		if !historyAvailable {
			return "Недостаточно истории публикаций. Это первая доступная AI-сводка, поэтому пока невозможно определить, какие проблемы сохраняются из предыдущих отчётов. Активные риски текущей недели отражены в остальных блоках."
		}
		return "Нет подтверждённых активных проблем, перешедших из предыдущего периода."
	case GroupKeyImproved:
		return "Заметных улучшений за неделю не выявлено."
	default:
		return "Заметных изменений не выявлено."
	}
}

// DefaultGroupLabel returns the seller-facing group label.
func DefaultGroupLabel(groupKey string) string {
	switch groupKey {
	case GroupKeyWorsened:
		return "Ухудшилось"
	case GroupKeyNewRisks:
		return "Новые риски"
	case GroupKeyUnchangedRisks:
		return "Активные проблемы"
	case GroupKeyImproved:
		return "Улучшилось"
	default:
		return groupKey
	}
}

// DefaultGroupTone returns the tone for a group key.
func DefaultGroupTone(groupKey string) string {
	switch groupKey {
	case GroupKeyWorsened:
		return "negative"
	case GroupKeyNewRisks:
		return "warning"
	case GroupKeyUnchangedRisks:
		return "neutral"
	case GroupKeyImproved:
		return "positive"
	default:
		return "neutral"
	}
}
