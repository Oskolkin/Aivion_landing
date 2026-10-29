package pricingconstraints

import (
	"context"
	"fmt"
	"testing"
)

type scopeCall struct {
	scopeType  ScopeType
	targetKind *ScopeTargetKind
	targetID   *int64
	targetCode *string
}

type fakeRuleRepository struct {
	calls   []scopeCall
	byScope map[string][]Rule
}

func (f *fakeRuleRepository) ListByScope(
	ctx context.Context,
	sellerAccountID int64,
	scopeType ScopeType,
	targetKind *ScopeTargetKind,
	targetID *int64,
	targetCode *string,
) ([]Rule, error) {
	f.calls = append(f.calls, scopeCall{scopeType: scopeType, targetKind: targetKind, targetID: targetID, targetCode: targetCode})
	return f.byScope[key(scopeType, targetKind, targetID, targetCode)], nil
}

func key(scopeType ScopeType, targetKind *ScopeTargetKind, targetID *int64, targetCode *string) string {
	kind := "nil"
	if targetKind != nil {
		kind = string(*targetKind)
	}
	id := "nil"
	if targetID != nil {
		id = intToStr(*targetID)
	}
	code := "nil"
	if targetCode != nil {
		code = *targetCode
	}
	return string(scopeType) + "|" + kind + "|" + id + "|" + code
}

func intToStr(v int64) string {
	return fmt.Sprintf("%d", v)
}

func TestResolverPrecedenceSKUOverrideWins(t *testing.T) {
	skuID := int64(55)
	categoryID := int64(701)
	repo := &fakeRuleRepository{
		byScope: map[string][]Rule{
			key(ScopeTypeSKUOverride, scopeTargetKindPtr(ScopeTargetKindSKU), &skuID, nil): {
				{
					ID:                     10,
					ScopeType:              ScopeTypeSKUOverride,
					IsActive:               true,
					ReferenceMarginPercent: floatPtr(0.2),
					MinPrice:               floatPtr(100),
				},
			},
		},
	}
	resolver := NewResolver(repo)
	out, err := resolver.Resolve(context.Background(), ResolveInput{
		SellerAccountID:       1,
		OzonProductID:         999,
		SKU:                   &skuID,
		DescriptionCategoryID: &categoryID,
		ReferencePrice:        1000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.HasConstraints || out.RuleID == nil || *out.RuleID != 10 {
		t.Fatalf("expected sku override rule id=10, got %+v", out)
	}
	if out.ResolvedFromScopeType == nil || *out.ResolvedFromScopeType != ScopeTypeSKUOverride {
		t.Fatalf("expected sku_override scope, got %+v", out.ResolvedFromScopeType)
	}
}

func TestResolverPrecedenceCategoryThenGlobal(t *testing.T) {
	categoryID := int64(701)
	productID := int64(999)
	repo := &fakeRuleRepository{
		byScope: map[string][]Rule{
			key(ScopeTypeSKUOverride, scopeTargetKindPtr(ScopeTargetKindProductID), &productID, nil): {},
			key(ScopeTypeCategoryRule, scopeTargetKindPtr(ScopeTargetKindCategoryID), &categoryID, nil): {
				{
					ID:                     11,
					ScopeType:              ScopeTypeCategoryRule,
					IsActive:               true,
					ReferenceMarginPercent: floatPtr(0.25),
				},
			},
		},
	}
	resolver := NewResolver(repo)
	out, err := resolver.Resolve(context.Background(), ResolveInput{
		SellerAccountID:       1,
		OzonProductID:         productID,
		DescriptionCategoryID: &categoryID,
		ReferencePrice:        1000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.ResolvedFromScopeType == nil || *out.ResolvedFromScopeType != ScopeTypeCategoryRule {
		t.Fatalf("expected category_rule, got %+v", out.ResolvedFromScopeType)
	}
}

func TestResolverNotFoundIsExplicit(t *testing.T) {
	repo := &fakeRuleRepository{byScope: map[string][]Rule{}}
	resolver := NewResolver(repo)
	out, err := resolver.Resolve(context.Background(), ResolveInput{
		SellerAccountID: 1,
		OzonProductID:   777,
		ReferencePrice:  900,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.HasConstraints {
		t.Fatalf("expected explicit no-constraints result")
	}
}

func TestPreviewExpectedMargin(t *testing.T) {
	implied := 750.0
	result := ResolvedResult{
		HasConstraints:       true,
		EffectiveImpliedCost: &implied,
	}
	margin, err := result.PreviewExpectedMargin(1200)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if margin == nil || *margin != 0.375 {
		t.Fatalf("expected margin 0.375, got %v", margin)
	}
}

func floatPtr(v float64) *float64 {
	return &v
}

func scopeTargetKindPtr(v ScopeTargetKind) *ScopeTargetKind {
	return &v
}
