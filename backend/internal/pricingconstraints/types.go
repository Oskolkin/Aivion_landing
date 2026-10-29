package pricingconstraints

import "time"

type ScopeType string

const (
	ScopeTypeGlobalDefault ScopeType = "global_default"
	ScopeTypeCategoryRule  ScopeType = "category_rule"
	ScopeTypeSKUOverride   ScopeType = "sku_override"
)

type ScopeTargetKind string

const (
	ScopeTargetKindCategoryID ScopeTargetKind = "category_id"
	ScopeTargetKindSKU        ScopeTargetKind = "sku"
	ScopeTargetKindProductID  ScopeTargetKind = "product_id"
	ScopeTargetKindOfferID    ScopeTargetKind = "offer_id"
)

type Rule struct {
	ID                     int64
	SellerAccountID        int64
	ScopeType              ScopeType
	ScopeTargetKind        *ScopeTargetKind
	ScopeTargetID          *int64
	ScopeTargetCode        *string
	MinPrice               *float64
	MaxPrice               *float64
	ReferenceMarginPercent *float64
	ReferencePrice         *float64
	ImpliedCost            *float64
	IsActive               bool
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

// RuleComputationReference captures explainability inputs and outputs
// used by future effective-constraint resolver steps.
type RuleComputationReference struct {
	ReferencePrice         float64
	ReferenceMarginPercent float64
	ImpliedCost            float64
}

// EffectiveConstraint carries resolved values and their origin.
type EffectiveConstraint struct {
	SellerAccountID         int64
	OzonProductID           int64
	SKU                     *int64
	OfferID                 *string
	ResolvedFromScopeType   ScopeType
	RuleID                  int64
	EffectiveMinPrice       *float64
	EffectiveMaxPrice       *float64
	ReferencePrice          *float64
	ReferenceMarginPercent  *float64
	ImpliedCost             *float64
	ComputedAt              time.Time
	ComputationExplainBasis *RuleComputationReference
}
