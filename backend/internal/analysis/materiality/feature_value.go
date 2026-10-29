package materiality

import (
	"fmt"
	"math/big"
)

// FeatureState distinguishes known values from missing or blocked facts.
type FeatureState string

const (
	FeatureKnown         FeatureState = "KNOWN"
	FeatureUnknown       FeatureState = "UNKNOWN"
	FeatureUnavailable   FeatureState = "UNAVAILABLE"
	FeatureNotApplicable FeatureState = "NOT_APPLICABLE"
	FeatureStale         FeatureState = "STALE"
)

// FeatureValue is a typed operational fact with explicit missing semantics.
type FeatureValue[T any] struct {
	State FeatureState `json:"state"`
	Value *T           `json:"value,omitempty"`
}

// MoneyFeature is an exact decimal operational metric (money, ratios, counts-as-decimal).
type MoneyFeature struct {
	State FeatureState `json:"state"`
	Value *Exact       `json:"value,omitempty"`
}

// CountFeature is an integer-like count metric with explicit missing semantics.
type CountFeature struct {
	State FeatureState `json:"state"`
	Value *int64       `json:"value,omitempty"`
}

func KnownExact(v Exact) MoneyFeature {
	return MoneyFeature{State: FeatureKnown, Value: &v}
}

func KnownCount(v int64) CountFeature {
	return CountFeature{State: FeatureKnown, Value: &v}
}

func (f MoneyFeature) IsKnown() bool {
	return f.State == FeatureKnown && f.Value != nil
}

func (f CountFeature) IsKnown() bool {
	return f.State == FeatureKnown && f.Value != nil
}

func featureKnownZero(f MoneyFeature) bool {
	if !f.IsKnown() {
		return false
	}
	r, err := parseRat(*f.Value)
	if err != nil {
		return false
	}
	return r.Sign() == 0
}

func featureKnownPositive(f MoneyFeature) bool {
	if !f.IsKnown() {
		return false
	}
	r, err := parseRat(*f.Value)
	if err != nil {
		return false
	}
	return r.Sign() > 0
}

func featureExact(f MoneyFeature) (Exact, bool) {
	if !f.IsKnown() {
		return Exact(""), false
	}
	return *f.Value, true
}

func featureRat(f MoneyFeature) (*big.Rat, error) {
	if !f.IsKnown() {
		return nil, fmt.Errorf("feature not KNOWN")
	}
	return parseRat(*f.Value)
}

func maxKnownRat(vals ...MoneyFeature) *big.Rat {
	var best *big.Rat
	for _, v := range vals {
		if !v.IsKnown() {
			continue
		}
		r, err := parseRat(*v.Value)
		if err != nil {
			continue
		}
		if best == nil || cmpRat(r, best) > 0 {
			best = new(big.Rat).Set(r)
		}
	}
	if best == nil {
		return new(big.Rat)
	}
	return best
}

func countKnownNonPositive(f CountFeature) bool {
	if !f.IsKnown() {
		return true
	}
	return *f.Value <= 0
}
