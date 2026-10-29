package pricingconstraints

import "testing"

func TestComputeImpliedCost(t *testing.T) {
	cost, err := ComputeImpliedCost(1000, 0.25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cost != 750 {
		t.Fatalf("expected 750, got %v", cost)
	}
}

func TestComputeExpectedMargin(t *testing.T) {
	margin, err := ComputeExpectedMargin(1200, 750)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if margin != 0.375 {
		t.Fatalf("expected 0.375, got %v", margin)
	}
}

func TestValidateRuleInput(t *testing.T) {
	min := 200.0
	max := 100.0
	margin := 1.2
	refPrice := 0.0
	input := RuleValidationInput{
		ScopeType:              ScopeTypeCategoryRule,
		MinPrice:               &min,
		MaxPrice:               &max,
		ReferenceMarginPercent: &margin,
		ReferencePrice:         &refPrice,
	}
	if err := ValidateRuleInput(input); err == nil {
		t.Fatalf("expected validation error")
	}
}
