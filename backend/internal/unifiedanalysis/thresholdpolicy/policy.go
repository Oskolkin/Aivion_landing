// Package thresholdpolicy is the versioned registry of comparative business
// thresholds that Unified Analysis may cite in free text (e.g. «ДРР > 80%»).
// Thresholds are not Pack facts: the AI must copy policy_id / metric_code /
// scope / operator / value verbatim into threshold_claims. Invented values
// are rejected by outputvalidator.
package thresholdpolicy

// Policy IDs (stable strings stored on publications / AI output).
const (
	PolicyHighDRRCampaignThreshold = "high_drr_campaign_threshold"
	PolicyLowStockCoverageDays     = "low_stock_coverage_days_threshold"
)

// Metric codes allowed on threshold claims.
const (
	MetricCampaignDRRPct      = "campaign_drr_pct"
	MetricStockCoverageDays7d = "stock_coverage_days_7d"
)

// Scopes / operators mirror the JSON schema enums.
const (
	ScopeCampaign = "campaign"
	ScopeAccount  = "account"
	ScopeSKU      = "sku"

	OpGT  = "gt"
	OpGTE = "gte"
	OpLT  = "lt"
	OpLTE = "lte"
	OpEQ  = "eq"
)

// Threshold is one immutable registry entry.
type Threshold struct {
	PolicyID   string
	MetricCode string
	Scope      string
	Operator   string
	Value      float64
}

// Registry is the authoritative list for new Unified runs (schema v10+).
// Bump a new policy_id (do not mutate values in place) when business rules change.
var Registry = []Threshold{
	{
		PolicyID:   PolicyHighDRRCampaignThreshold,
		MetricCode: MetricCampaignDRRPct,
		Scope:      ScopeCampaign,
		Operator:   OpGT,
		Value:      80,
	},
	{
		PolicyID:   PolicyLowStockCoverageDays,
		MetricCode: MetricStockCoverageDays7d,
		Scope:      ScopeSKU,
		Operator:   OpLT,
		Value:      7,
	},
}

// Lookup returns the registry entry for policy_id, or false when unknown.
func Lookup(policyID string) (Threshold, bool) {
	for _, t := range Registry {
		if t.PolicyID == policyID {
			return t, true
		}
	}
	return Threshold{}, false
}

// MatchExact reports whether claim fields equal the registry entry exactly
// (value compared with a tiny absolute epsilon for float JSON noise).
func MatchExact(policyID, metricCode, scope, operator string, value float64) bool {
	t, ok := Lookup(policyID)
	if !ok {
		return false
	}
	if t.MetricCode != metricCode || t.Scope != scope || t.Operator != operator {
		return false
	}
	const eps = 1e-9
	diff := t.Value - value
	if diff < 0 {
		diff = -diff
	}
	return diff <= eps
}
