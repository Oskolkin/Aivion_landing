package pricingconstraints

import (
	"fmt"
	"math"
)

// ComputeImpliedCost calculates C_est from reference price and margin:
// C_est = P_ref * (1 - M_ref),
// where margin is (price - cost) / price as decimal fraction.
func ComputeImpliedCost(referencePrice float64, referenceMarginPercent float64) (float64, error) {
	if err := ValidateReferenceInputs(referencePrice, referenceMarginPercent); err != nil {
		return 0, err
	}
	cost := referencePrice * (1 - referenceMarginPercent)
	if cost < 0 {
		return 0, fmt.Errorf("implied cost is negative")
	}
	return round6(cost), nil
}

// ComputeExpectedMargin calculates expected margin at a new price:
// ExpectedMargin(P_new) = (P_new - C_est) / P_new.
func ComputeExpectedMargin(newPrice float64, impliedCost float64) (float64, error) {
	if err := ValidateExpectedMarginInputs(newPrice, impliedCost); err != nil {
		return 0, err
	}
	margin := (newPrice - impliedCost) / newPrice
	return round6(margin), nil
}

// ComputeExpectedMarginFromReference is a convenience helper that calculates
// implied cost from reference inputs first, then expected margin at new price.
func ComputeExpectedMarginFromReference(newPrice float64, referencePrice float64, referenceMarginPercent float64) (float64, float64, error) {
	impliedCost, err := ComputeImpliedCost(referencePrice, referenceMarginPercent)
	if err != nil {
		return 0, 0, err
	}
	margin, err := ComputeExpectedMargin(newPrice, impliedCost)
	if err != nil {
		return 0, 0, err
	}
	return impliedCost, margin, nil
}

func round6(v float64) float64 {
	return math.Round(v*1_000_000) / 1_000_000
}
