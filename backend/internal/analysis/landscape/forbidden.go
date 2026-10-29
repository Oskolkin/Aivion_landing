package landscape

import "strings"

// ForbiddenSemanticSubstrings must never appear in Landscape JSON field names,
// tags, or free-form semantic labels invented by the builder. Provider-native
// status strings (e.g. Account Health OK/WARNING/CRITICAL) are out of scope —
// this gate targets Aivion-invented interpretation vocabulary.
var ForbiddenSemanticSubstrings = []string{
	"problem",
	"opportunity",
	"severity",
	"priority",
	"healthy",
	"unhealthy",
	"overpriced",
	"underpriced",
	"inefficient",
	"needs_attention",
	"needs attention",
	"recommended_action",
	"recommended action",
	"root_cause",
	"root cause",
	"revenue_at_risk",
	"revenue at risk",
	"materiality_score",
	"materiality",
	"stock_problem",
	"stockout_risk",
	"worsened",
	"improved",
	"significant_change",
	"important",
	"high_concentration_risk",
	"profit",
	"margin",
	"total_future_stock",
	"projected_stock",
	// Standalone interpretation labels (provider_* field mirrors are allowlisted in the scanner).
	"critical",
	"bad",
	"good",
}

func containsForbidden(s string) (string, bool) {
	lower := strings.ToLower(s)
	for _, f := range ForbiddenSemanticSubstrings {
		if strings.Contains(lower, strings.ToLower(f)) {
			return f, true
		}
	}
	return "", false
}
