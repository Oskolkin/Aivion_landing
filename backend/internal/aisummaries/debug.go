package aisummaries

// SummaryDebug describes how stored AI daily summaries were applied on dashboard home.
type SummaryDebug struct {
	RunID              string             `json:"run_id,omitempty"`
	RunStatus          string             `json:"run_status,omitempty"`
	AccountSummaryID   string             `json:"account_summary_id,omitempty"`
	AccountStatus      string             `json:"account_status,omitempty"`
	AccountValidation  string             `json:"account_validation_status,omitempty"`
	FunnelFallback     string             `json:"funnel_fallback_reason,omitempty"`
	FunnelRepairable   []string           `json:"funnel_repairable_errors,omitempty"`
	CategoryRows       []CategoryDebugRow `json:"category_rows,omitempty"`
	RecommendedActions RecommendedDebug   `json:"recommended_actions,omitempty"`
}

type CategoryDebugRow struct {
	ScopeID          string   `json:"scope_id"`
	CategoryName     string   `json:"category_name,omitempty"`
	Status           string   `json:"status"`
	ValidationStatus string   `json:"validation_status,omitempty"`
	RepairableErrors []string `json:"repairable_errors,omitempty"`
	FallbackReason   string   `json:"fallback_reason,omitempty"`
	Accepted         bool     `json:"accepted"`
}

type RecommendedDebug struct {
	AIReceived   int    `json:"ai_actions_received"`
	AIAccepted   int    `json:"ai_actions_accepted"`
	Dropped      int    `json:"dropped_actions"`
	FallbackNote string `json:"fallback_note,omitempty"`
}
