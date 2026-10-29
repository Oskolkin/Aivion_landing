package materiality

import (
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
)

// BudgetProfile is a Discovery selection budget (separate from ranking policy).
type BudgetProfile struct {
	Version             string `json:"version"`
	MaxCategories       int    `json:"max_categories"`
	MaxSKUs             int    `json:"max_skus"`
	MaxCampaigns        int    `json:"max_campaigns"`
	AnchorMaxCategories int    `json:"anchor_max_categories"`
	AnchorMaxSKUs       int    `json:"anchor_max_skus"`
	AnchorMaxCampaigns  int    `json:"anchor_max_campaigns"`
}

// DiscoveryStandardBudget returns DISCOVERY_STANDARD_V1 pilot defaults.
func DiscoveryStandardBudget() BudgetProfile {
	return BudgetProfile{
		Version:             DiscoveryStandardBudgetV1,
		MaxCategories:       15,
		MaxSKUs:             40,
		MaxCampaigns:        15,
		AnchorMaxCategories: 8,
		AnchorMaxSKUs:       20,
		AnchorMaxCampaigns:  8,
	}
}

// Hash returns the canonical budget hash. RankingHash must not include this.
func (b BudgetProfile) Hash() string {
	return canonicaljson.Hash(b)
}
