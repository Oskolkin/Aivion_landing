// Package healthpolicy builds the backend-owned risk candidate registry that
// grounds Unified Analysis cabinet health_score in Dashboard Facts
// (dashboardfacts.DashboardFactsV1), and scores it via the existing
// dashboardhome.ComputeHealthScoreBreakdown formula.
//
// health_score contract: health_score is ALWAYS computed from the FULL
// backend candidate registry (BuildCandidateRegistry + ComputeHealthScore),
// never from the subset of risk drivers the AI chooses to surface for
// explanation. The AI may only:
//   - pick which registry candidates to explain first (display order),
//   - write seller-facing Title/Reason copy for a candidate it references,
//   - it must NEVER invent a driver_id outside the registry (see
//     IsKnownDriverID/UnknownDriverIDs — callers must treat an unknown id as
//     an error/drop, never as a new scored candidate),
//   - it must NEVER change a candidate's Severity/ImpactScore/Penalty.
package healthpolicy

import (
	"math"
	"sort"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
)

// HealthPolicyVersion identifies the versioned candidate-building/scoring
// formula implemented by this package. Bump it whenever candidate
// thresholds, the severity/impact math, or the known driver_id set change in
// a way that could move a previously-published score, and persist it
// alongside health_score so historical publications stay auditable.
const HealthPolicyVersion = "health_policy_v2"

// Known backend risk driver IDs. This is the complete, closed set of
// driver_ids the health-score candidate registry can ever produce. AI
// risk_drivers referencing an ID outside this set are unknown/invented and
// must never be used to explain a registry candidate or influence
// health_score (see IsKnownDriverID / UnknownDriverIDs).
const (
	DriverSalesDecline               = "sales_decline"
	DriverRevenueDecline             = "revenue_decline"
	DriverKeySKUStockRisk            = "key_sku_stock_risk"
	DriverStockout                   = "stockout"
	DriverNoSalesWithStock           = "no_sales_with_stock"
	DriverSurplus                    = "surplus"
	DriverViewToCartDecline          = "view_to_cart_decline"
	DriverCartToOrderDecline         = "cart_to_order_decline"
	DriverHighDRRCampaign            = "high_drr_campaign"
	DriverCampaignSpendWithoutOrders = "campaign_spend_without_orders"
	DriverAdShareGrowth              = "ad_share_growth"
	// DriverDataLimitation is informational only (see ScoringCandidates):
	// missing/optional data coverage is a transparency signal, never a
	// scored business-health risk.
	DriverDataLimitation = "data_limitation"
)

// KnownDriverIDs is the ordered list of every driver_id BuildCandidateRegistry
// can produce.
var KnownDriverIDs = []string{
	DriverSalesDecline,
	DriverRevenueDecline,
	DriverKeySKUStockRisk,
	DriverStockout,
	DriverNoSalesWithStock,
	DriverSurplus,
	DriverViewToCartDecline,
	DriverCartToOrderDecline,
	DriverHighDRRCampaign,
	DriverCampaignSpendWithoutOrders,
	DriverAdShareGrowth,
	DriverDataLimitation,
}

var knownDriverIDSet = buildKnownDriverIDSet()

func buildKnownDriverIDSet() map[string]struct{} {
	set := make(map[string]struct{}, len(KnownDriverIDs))
	for _, id := range KnownDriverIDs {
		set[id] = struct{}{}
	}
	return set
}

// IsKnownDriverID reports whether id belongs to the closed backend risk
// candidate registry. AI-authored risk_drivers with an unknown id must be
// dropped from display and can never influence health_score.
func IsKnownDriverID(id string) bool {
	_, ok := knownDriverIDSet[strings.TrimSpace(id)]
	return ok
}

// UnknownDriverIDs filters ids down to the ones NOT present in the backend
// registry (i.e. AI-invented ids). Empty ids are ignored.
func UnknownDriverIDs(ids []string) []string {
	var out []string
	for _, id := range ids {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		if !IsKnownDriverID(trimmed) {
			out = append(out, trimmed)
		}
	}
	return out
}

// RiskCandidate is one backend-computed, facts-grounded risk factor. The
// full slice returned by BuildCandidateRegistry is the sole source of truth
// for cabinet health_score (via ComputeHealthScore) — independent of which
// candidates the AI later chooses to explain.
type RiskCandidate struct {
	// DriverID is one of KnownDriverIDs.
	DriverID string
	// Domain is the risk category (sales, stock, sku, surplus, funnel,
	// advertising, data_quality) — mirrors dashboardhome.RiskDriver.Category.
	Domain string
	// FactIDs point at the Dashboard Facts fields this candidate was
	// computed from (dotted paths into dashboardfacts.DashboardFactsV1).
	FactIDs []string
	// EntityIDs are the affected entity identifiers (ozon_product_id /
	// campaign_external_id), when the candidate is entity-scoped.
	EntityIDs []string
	// Severity/ImpactScore are backend-calculated from Dashboard Facts; the
	// AI may read but never set or override them.
	Severity    string
	ImpactScore int
	// Penalty is the versioned, rank-independent severity+impact penalty
	// magnitude for this candidate, using the same per-driver formula as
	// dashboardhome.ComputeHealthScoreBreakdown (see ComputePenalty). It is
	// exposed for registry ordering/explainability; the actual health_score
	// always comes from ComputeHealthScore, which additionally applies
	// rank-based weight decay across the sorted registry.
	Penalty float64
	// Period is the metric_date (or date range "YYYY-MM-DD..YYYY-MM-DD")
	// the candidate was computed over.
	Period string
	// ReasonTemplateData carries the raw facts values used to render the
	// backend default Title/Reason (DefaultTitle/DefaultReason), so any
	// caller can re-derive seller-facing copy deterministically.
	ReasonTemplateData map[string]any
}

// severityBaseByLevel/impactPartFor mirror dashboardhome's private
// computeHealthScoreBreakdown formula (health policy v1 constants). Kept in
// sync by TestComputePenalty_MatchesDashboardhomeFormula in policy_test.go —
// bump HealthPolicyVersion if either side's constants change.
var severityBaseByLevel = map[string]float64{
	"critical": 18,
	"high":     13,
	"medium":   8,
	"low":      4,
}

func severityBaseFor(severity string) float64 {
	return severityBaseByLevel[NormalizeSeverity(severity)]
}

func impactPartFor(impactScore int) float64 {
	impact := clampInt(impactScore, 0, 100)
	return 14 * math.Sqrt(float64(impact)/100)
}

// ComputePenalty returns the versioned, rank-independent penalty magnitude
// for a severity/impact pair (see RiskCandidate.Penalty doc).
func ComputePenalty(severity string, impactScore int) float64 {
	return severityBaseFor(severity) + impactPartFor(impactScore)
}

// NormalizeSeverity maps free-form severity strings onto the four backend
// bands; unknown values fall back to "low" (never silently escalate).
func NormalizeSeverity(severity string) string {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "critical":
		return "critical"
	case "high":
		return "high"
	case "medium":
		return "medium"
	default:
		return "low"
	}
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

// SortCandidates returns a stable copy of candidates ordered by descending
// Penalty (ties broken by Domain then DriverID) — the same rank the scoring
// formula's weight decay applies over. Deterministic for identical inputs,
// independent of the registry's build order.
func SortCandidates(candidates []RiskCandidate) []RiskCandidate {
	out := make([]RiskCandidate, len(candidates))
	copy(out, candidates)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Penalty != out[j].Penalty {
			return out[i].Penalty > out[j].Penalty
		}
		if out[i].Domain != out[j].Domain {
			return out[i].Domain < out[j].Domain
		}
		return out[i].DriverID < out[j].DriverID
	})
	return out
}

// ScoringCandidates filters out candidates that must never influence
// health_score. Currently only DriverDataLimitation: missing/optional data
// coverage (e.g. no advertising source) is a transparency signal, not a
// business-health risk, and must never tank the score.
func ScoringCandidates(candidates []RiskCandidate) []RiskCandidate {
	out := make([]RiskCandidate, 0, len(candidates))
	for _, c := range candidates {
		if c.DriverID == DriverDataLimitation {
			continue
		}
		out = append(out, c)
	}
	return out
}

// ToRiskDriver converts one backend candidate into the dashboardhome.RiskDriver
// shape ComputeHealthScoreBreakdown consumes, using backend default copy
// (DefaultTitle/DefaultReason). Callers projecting a seller-facing display
// list should overlay AI copy on top without touching
// ID/Category/Severity/ImpactScore.
func ToRiskDriver(c RiskCandidate) dashboardhome.RiskDriver {
	return dashboardhome.RiskDriver{
		ID:          c.DriverID,
		Category:    c.Domain,
		Severity:    NormalizeSeverity(c.Severity),
		ImpactScore: clampInt(c.ImpactScore, 0, 100),
		Title:       DefaultTitle(c.DriverID),
		Reason:      DefaultReason(c),
	}
}

// ToRiskDrivers sorts candidates by Penalty (see SortCandidates) and
// converts them to dashboardhome.RiskDriver, matching the order
// dashboardhome.ComputeHealthScoreBreakdown expects (rank-based weight decay
// applies to list position).
func ToRiskDrivers(candidates []RiskCandidate) []dashboardhome.RiskDriver {
	sorted := SortCandidates(candidates)
	out := make([]dashboardhome.RiskDriver, 0, len(sorted))
	for _, c := range sorted {
		out = append(out, ToRiskDriver(c))
	}
	return out
}

// ComputeHealthScore is THE versioned publication formula for Unified
// Analysis cabinet health_score: always computed from the full backend
// candidate registry (candidates), never from an AI-selected subset. Callers
// must pass BuildCandidateRegistry's complete output — not a filtered
// display list — so the score stays independent of which drivers the AI
// picks to explain, and identical for identical facts regardless of AI
// output.
func ComputeHealthScore(candidates []RiskCandidate) dashboardhome.HealthScoreBreakdown {
	drivers := ToRiskDrivers(ScoringCandidates(candidates))
	return dashboardhome.ComputeHealthScoreBreakdown(drivers)
}
