package pricingconstraints

import (
	"context"
	"fmt"
	"time"
)

type ResolveInput struct {
	SellerAccountID       int64
	OzonProductID         int64
	SKU                   *int64
	OfferID               *string
	DescriptionCategoryID *int64
	ReferencePrice        float64
}

type ResolvedResult struct {
	HasConstraints         bool
	EffectiveMinPrice      *float64
	EffectiveMaxPrice      *float64
	EffectiveMargin        *float64
	EffectiveImpliedCost   *float64
	ResolvedFromScopeType  *ScopeType
	RuleID                 *int64
	ReferencePrice         *float64
	ReferenceMarginPercent *float64
	ImpliedCost            *float64
	ComputedAt             time.Time
}

func (r ResolvedResult) PreviewExpectedMargin(newPrice float64) (*float64, error) {
	if !r.HasConstraints {
		return nil, fmt.Errorf("constraints are not resolved")
	}
	if r.EffectiveImpliedCost == nil {
		return nil, fmt.Errorf("effective implied cost is missing")
	}
	margin, err := ComputeExpectedMargin(newPrice, *r.EffectiveImpliedCost)
	if err != nil {
		return nil, err
	}
	return &margin, nil
}

func (r ResolvedResult) ToEffectiveConstraintRecord(input ResolveInput) (*EffectiveConstraintRecord, error) {
	if !r.HasConstraints || r.RuleID == nil || r.ResolvedFromScopeType == nil {
		return nil, fmt.Errorf("cannot materialize effective constraint without winning rule")
	}
	return &EffectiveConstraintRecord{
		SellerAccountID:        input.SellerAccountID,
		OzonProductID:          input.OzonProductID,
		SKU:                    input.SKU,
		OfferID:                input.OfferID,
		ResolvedFromScopeType:  *r.ResolvedFromScopeType,
		RuleID:                 *r.RuleID,
		EffectiveMinPrice:      r.EffectiveMinPrice,
		EffectiveMaxPrice:      r.EffectiveMaxPrice,
		ReferencePrice:         r.ReferencePrice,
		ReferenceMarginPercent: r.ReferenceMarginPercent,
		ImpliedCost:            r.EffectiveImpliedCost,
		ComputedAt:             time.Now().UTC(),
	}, nil
}

type Resolver struct {
	repo RuleRepository
}

func NewResolver(repo RuleRepository) *Resolver {
	return &Resolver{repo: repo}
}

func (r *Resolver) Resolve(ctx context.Context, input ResolveInput) (ResolvedResult, error) {
	if input.SellerAccountID <= 0 {
		return ResolvedResult{}, fmt.Errorf("seller_account_id must be > 0")
	}
	if input.OzonProductID <= 0 {
		return ResolvedResult{}, fmt.Errorf("ozon_product_id must be > 0")
	}
	if input.ReferencePrice <= 0 {
		return ResolvedResult{}, fmt.Errorf("reference_price must be > 0")
	}

	rule, found, err := r.resolveByPrecedence(ctx, input)
	if err != nil {
		return ResolvedResult{}, err
	}
	if !found {
		return ResolvedResult{
			HasConstraints: false,
			ComputedAt:     time.Now().UTC(),
		}, nil
	}

	result, err := r.buildResolved(rule, input.ReferencePrice)
	if err != nil {
		return ResolvedResult{}, err
	}
	result.HasConstraints = true
	result.ComputedAt = time.Now().UTC()
	return result, nil
}

func (r *Resolver) resolveByPrecedence(ctx context.Context, input ResolveInput) (Rule, bool, error) {
	if input.SKU != nil {
		kind := ScopeTargetKindSKU
		rule, found, err := r.lookupWinning(ctx, input.SellerAccountID, ScopeTypeSKUOverride, &kind, input.SKU, nil)
		if err != nil {
			return Rule{}, false, err
		}
		if found {
			return rule, true, nil
		}
	}

	productID := input.OzonProductID
	productKind := ScopeTargetKindProductID
	rule, found, err := r.lookupWinning(ctx, input.SellerAccountID, ScopeTypeSKUOverride, &productKind, &productID, nil)
	if err != nil {
		return Rule{}, false, err
	}
	if found {
		return rule, true, nil
	}

	if input.OfferID != nil {
		offerKind := ScopeTargetKindOfferID
		rule, found, err := r.lookupWinning(ctx, input.SellerAccountID, ScopeTypeSKUOverride, &offerKind, nil, input.OfferID)
		if err != nil {
			return Rule{}, false, err
		}
		if found {
			return rule, true, nil
		}
	}

	if input.DescriptionCategoryID != nil {
		categoryKind := ScopeTargetKindCategoryID
		rule, found, err := r.lookupWinning(
			ctx,
			input.SellerAccountID,
			ScopeTypeCategoryRule,
			&categoryKind,
			input.DescriptionCategoryID,
			nil,
		)
		if err != nil {
			return Rule{}, false, err
		}
		if found {
			return rule, true, nil
		}
	}

	return r.lookupWinning(ctx, input.SellerAccountID, ScopeTypeGlobalDefault, nil, nil, nil)
}

func (r *Resolver) lookupWinning(
	ctx context.Context,
	sellerAccountID int64,
	scopeType ScopeType,
	targetKind *ScopeTargetKind,
	targetID *int64,
	targetCode *string,
) (Rule, bool, error) {
	rules, err := r.repo.ListByScope(ctx, sellerAccountID, scopeType, targetKind, targetID, targetCode)
	if err != nil {
		return Rule{}, false, err
	}
	for _, rule := range rules {
		if rule.IsActive {
			return rule, true, nil
		}
	}

	// Backward compatibility for rows created before target kind became explicit.
	if targetKind != nil {
		legacyRules, legacyErr := r.repo.ListByScope(ctx, sellerAccountID, scopeType, nil, targetID, targetCode)
		if legacyErr != nil {
			return Rule{}, false, legacyErr
		}
		for _, rule := range legacyRules {
			if rule.IsActive {
				return rule, true, nil
			}
		}
	}
	return Rule{}, false, nil
}

func (r *Resolver) buildResolved(rule Rule, fallbackReferencePrice float64) (ResolvedResult, error) {
	refPrice := fallbackReferencePrice
	if rule.ReferencePrice != nil && *rule.ReferencePrice > 0 {
		refPrice = *rule.ReferencePrice
	}

	var effectiveMargin *float64
	if rule.ReferenceMarginPercent != nil {
		m := *rule.ReferenceMarginPercent
		effectiveMargin = &m
	}

	impliedCost, err := resolveImpliedCost(rule, refPrice)
	if err != nil {
		return ResolvedResult{}, err
	}

	// Validate the winning rule in domain terms before returning resolved state.
	if err := ValidateRuleInput(RuleValidationInput{
		ScopeType:              rule.ScopeType,
		ScopeTargetKind:        rule.ScopeTargetKind,
		MinPrice:               rule.MinPrice,
		MaxPrice:               rule.MaxPrice,
		ReferenceMarginPercent: effectiveMargin,
		ReferencePrice:         &refPrice,
		ImpliedCost:            impliedCost,
	}); err != nil {
		return ResolvedResult{}, err
	}

	scope := rule.ScopeType
	ruleID := rule.ID
	return ResolvedResult{
		EffectiveMinPrice:      rule.MinPrice,
		EffectiveMaxPrice:      rule.MaxPrice,
		EffectiveMargin:        effectiveMargin,
		EffectiveImpliedCost:   impliedCost,
		ResolvedFromScopeType:  &scope,
		RuleID:                 &ruleID,
		ReferencePrice:         &refPrice,
		ReferenceMarginPercent: effectiveMargin,
		ImpliedCost:            impliedCost,
	}, nil
}

func resolveImpliedCost(rule Rule, referencePrice float64) (*float64, error) {
	if rule.ImpliedCost != nil {
		value := *rule.ImpliedCost
		return &value, nil
	}
	if rule.ReferenceMarginPercent == nil {
		return nil, nil
	}
	cost, err := ComputeImpliedCost(referencePrice, *rule.ReferenceMarginPercent)
	if err != nil {
		return nil, err
	}
	return &cost, nil
}
