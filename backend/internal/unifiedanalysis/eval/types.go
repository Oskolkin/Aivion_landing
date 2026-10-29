package eval

// ExpectedChecks describes semantic expectations for a Unified Analysis output.
// Zero-value fields are ignored (no constraint).
type ExpectedChecks struct {
	Outcome string `json:"outcome,omitempty"`

	RequiredCategories  []string `json:"required_categories,omitempty"`
	RequiredSKUIDs      []string `json:"required_sku_ids,omitempty"`
	RequiredCampaignIDs []string `json:"required_campaign_ids,omitempty"`

	RequireTopAction bool `json:"require_top_action,omitempty"`

	ForbiddenCategories  []string `json:"forbidden_categories,omitempty"`
	ForbiddenSKUIDs      []string `json:"forbidden_sku_ids,omitempty"`
	ForbiddenCampaignIDs []string `json:"forbidden_campaign_ids,omitempty"`

	MaxActions     *int `json:"max_actions,omitempty"`
	NoDuplicateSKU bool `json:"no_duplicate_sku,omitempty"`
}

// EvalReport is the result of semantic evaluation.
type EvalReport struct {
	Passed   bool          `json:"passed"`
	Failures []EvalFailure `json:"failures,omitempty"`
}

// EvalFailure is one failed semantic check.
type EvalFailure struct {
	Check   string `json:"check"`
	Message string `json:"message"`
}
