package landscape

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
)

// BuildMoneyComparison builds a money period comparison. FROM_ZERO forces
// pct_delta = nil. Currency mismatch → NOT_COMPARABLE. No float64 / Inf / NaN.
func BuildMoneyComparison(current, previous *MoneyAmount) MoneyComparison {
	out := MoneyComparison{Current: current, Previous: previous}
	if current == nil && previous == nil {
		out.ChangeState = ChangeNoCurrent
		return out
	}
	if current == nil {
		out.ChangeState = ChangeNoCurrent
		return out
	}
	if previous == nil {
		out.ChangeState = ChangeNoBase
		return out
	}
	if strings.TrimSpace(current.CurrencyCode) == "" || strings.TrimSpace(previous.CurrencyCode) == "" {
		out.ChangeState = ChangeNotComparable
		return out
	}
	if current.CurrencyCode != previous.CurrencyCode {
		out.ChangeState = ChangeNotComparable
		return out
	}
	cur, err1 := parseDec(current.Amount)
	prev, err2 := parseDec(previous.Amount)
	if err1 != nil || err2 != nil {
		out.ChangeState = ChangeNotComparable
		return out
	}
	delta := new(big.Rat).Sub(cur, prev)
	out.AbsoluteDelta = &MoneyAmount{Amount: canonicaljson.Decimal(formatRat(delta)), CurrencyCode: current.CurrencyCode}
	switch delta.Sign() {
	case 0:
		out.ChangeState = ChangeUnchanged
		z := canonicaljson.Decimal("0")
		out.PctDelta = &z
	default:
		if prev.Sign() == 0 {
			out.ChangeState = ChangeFromZero
			out.PctDelta = nil
			if cur.Sign() == 0 {
				out.ChangeState = ChangeUnchanged
				z := canonicaljson.Decimal("0")
				out.PctDelta = &z
			}
			return out
		}
		if cur.Sign() == 0 {
			out.ChangeState = ChangeToZero
			pct := canonicaljson.Decimal("-100")
			out.PctDelta = &pct
			return out
		}
		pct := new(big.Rat).Quo(delta, prev)
		pct.Mul(pct, big.NewRat(100, 1))
		p := canonicaljson.Decimal(formatRat(pct))
		out.PctDelta = &p
		if delta.Sign() > 0 {
			out.ChangeState = ChangeIncreased
		} else {
			out.ChangeState = ChangeDecreased
		}
	}
	return out
}

// BuildCountComparison mirrors money rules for integer counts.
func BuildCountComparison(current, previous *int64) CountComparison {
	out := CountComparison{Current: current, Previous: previous}
	if current == nil && previous == nil {
		out.ChangeState = ChangeNoCurrent
		return out
	}
	if current == nil {
		out.ChangeState = ChangeNoCurrent
		return out
	}
	if previous == nil {
		out.ChangeState = ChangeNoBase
		return out
	}
	delta := *current - *previous
	out.AbsoluteDelta = &delta
	switch {
	case delta == 0:
		out.ChangeState = ChangeUnchanged
		z := canonicaljson.Decimal("0")
		out.PctDelta = &z
	case *previous == 0:
		out.ChangeState = ChangeFromZero
		out.PctDelta = nil
		if *current == 0 {
			out.ChangeState = ChangeUnchanged
			z := canonicaljson.Decimal("0")
			out.PctDelta = &z
		}
	case *current == 0:
		out.ChangeState = ChangeToZero
		pct := canonicaljson.Decimal("-100")
		out.PctDelta = &pct
	default:
		pct := new(big.Rat).SetFrac64(delta*100, *previous)
		p := canonicaljson.Decimal(formatRat(pct))
		out.PctDelta = &p
		if delta > 0 {
			out.ChangeState = ChangeIncreased
		} else {
			out.ChangeState = ChangeDecreased
		}
	}
	return out
}

// BuildPctPointComparison compares percentage metrics in percentage points.
// Primary delta is delta_pp, never relative %.
func BuildPctPointComparison(current, previous *canonicaljson.Decimal) PercentagePointComparison {
	out := PercentagePointComparison{CurrentPct: current, PreviousPct: previous}
	if current == nil && previous == nil {
		out.ChangeState = ChangeNoCurrent
		return out
	}
	if current == nil {
		out.ChangeState = ChangeNoCurrent
		return out
	}
	if previous == nil {
		out.ChangeState = ChangeNoBase
		return out
	}
	cur, err1 := parseDec(*current)
	prev, err2 := parseDec(*previous)
	if err1 != nil || err2 != nil {
		out.ChangeState = ChangeNotComparable
		return out
	}
	delta := new(big.Rat).Sub(cur, prev)
	d := canonicaljson.Decimal(formatRat(delta))
	out.DeltaPP = &d
	switch delta.Sign() {
	case 0:
		out.ChangeState = ChangeUnchanged
	case 1:
		out.ChangeState = ChangeIncreased
	default:
		out.ChangeState = ChangeDecreased
	}
	return out
}

// BuildRatioComparison compares dimensionless ratios with the same change rules
// as counts (pct of previous), without inventing percentage-point semantics.
func BuildRatioComparison(current, previous *canonicaljson.Decimal) RatioComparison {
	out := RatioComparison{Current: current, Previous: previous}
	if current == nil {
		out.ChangeState = ChangeNoCurrent
		return out
	}
	if previous == nil {
		out.ChangeState = ChangeNoBase
		return out
	}
	cur, err1 := parseDec(*current)
	prev, err2 := parseDec(*previous)
	if err1 != nil || err2 != nil {
		out.ChangeState = ChangeNotComparable
		return out
	}
	delta := new(big.Rat).Sub(cur, prev)
	d := canonicaljson.Decimal(formatRat(delta))
	out.AbsoluteDelta = &d
	switch {
	case delta.Sign() == 0:
		out.ChangeState = ChangeUnchanged
		z := canonicaljson.Decimal("0")
		out.PctDelta = &z
	case prev.Sign() == 0:
		out.ChangeState = ChangeFromZero
		out.PctDelta = nil
	case cur.Sign() == 0:
		out.ChangeState = ChangeToZero
		pct := canonicaljson.Decimal("-100")
		out.PctDelta = &pct
	default:
		pct := new(big.Rat).Quo(delta, prev)
		pct.Mul(pct, big.NewRat(100, 1))
		p := canonicaljson.Decimal(formatRat(pct))
		out.PctDelta = &p
		if delta.Sign() > 0 {
			out.ChangeState = ChangeIncreased
		} else {
			out.ChangeState = ChangeDecreased
		}
	}
	return out
}

func parseDec(d canonicaljson.Decimal) (*big.Rat, error) {
	s := strings.TrimSpace(string(d))
	if s == "" {
		return nil, fmt.Errorf("empty decimal")
	}
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return nil, fmt.Errorf("invalid decimal %q", s)
	}
	return r, nil
}

func formatRat(r *big.Rat) string {
	if r == nil {
		return "0"
	}
	s := r.FloatString(12)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	if s == "" || s == "-0" {
		return "0"
	}
	return s
}
