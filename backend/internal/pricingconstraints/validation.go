package pricingconstraints

import (
	"fmt"
)

const (
	// Margin is decimal fraction: 0.25 means 25%.
	minMarginPercent = -0.99
	maxMarginPercent = 0.99
)

type RuleValidationInput struct {
	ScopeType              ScopeType
	ScopeTargetKind        *ScopeTargetKind
	MinPrice               *float64
	MaxPrice               *float64
	ReferenceMarginPercent *float64
	ReferencePrice         *float64
	ImpliedCost            *float64
}

func ValidateRuleInput(input RuleValidationInput) error {
	issues := &ValidationError{}

	switch input.ScopeType {
	case ScopeTypeGlobalDefault, ScopeTypeCategoryRule, ScopeTypeSKUOverride:
	default:
		issues.Add("scope_type must be one of: global_default, category_rule, sku_override")
	}
	if input.ScopeTargetKind != nil {
		switch *input.ScopeTargetKind {
		case ScopeTargetKindCategoryID, ScopeTargetKindSKU, ScopeTargetKindProductID, ScopeTargetKindOfferID:
		default:
			issues.Add("scope_target_kind must be one of: category_id, sku, product_id, offer_id")
		}
	}

	if input.MinPrice != nil && *input.MinPrice < 0 {
		issues.Add("min_price must be >= 0")
	}
	if input.MaxPrice != nil && *input.MaxPrice < 0 {
		issues.Add("max_price must be >= 0")
	}
	if input.MinPrice != nil && input.MaxPrice != nil && *input.MinPrice > *input.MaxPrice {
		issues.Add("min_price must be <= max_price")
	}

	if input.ReferencePrice != nil && *input.ReferencePrice <= 0 {
		issues.Add("reference_price must be > 0")
	}
	if input.ReferenceMarginPercent != nil && (*input.ReferenceMarginPercent < minMarginPercent || *input.ReferenceMarginPercent > maxMarginPercent) {
		issues.Add(fmt.Sprintf("reference_margin_percent must be in [%.2f, %.2f]", minMarginPercent, maxMarginPercent))
	}
	if input.ImpliedCost != nil && *input.ImpliedCost < 0 {
		issues.Add("implied_cost must be >= 0")
	}

	if !issues.Empty() {
		return issues
	}
	return nil
}

func ValidateReferenceInputs(referencePrice float64, referenceMarginPercent float64) error {
	return ValidateRuleInput(RuleValidationInput{
		ScopeType:              ScopeTypeGlobalDefault,
		ReferencePrice:         &referencePrice,
		ReferenceMarginPercent: &referenceMarginPercent,
	})
}

func ValidateExpectedMarginInputs(newPrice float64, impliedCost float64) error {
	issues := &ValidationError{}
	if newPrice <= 0 {
		issues.Add("new_price must be > 0")
	}
	if impliedCost < 0 {
		issues.Add("implied_cost must be >= 0")
	}
	if !issues.Empty() {
		return issues
	}
	return nil
}
