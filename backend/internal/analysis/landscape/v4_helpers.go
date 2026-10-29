package landscape

import (
	"math/big"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
)

type DemandKnowledge string

const (
	DemandPositive DemandKnowledge = "POSITIVE"
	DemandZero     DemandKnowledge = "ZERO"
	DemandUnknown  DemandKnowledge = "UNKNOWN"
)

func classifySKUCurrentDemand(sk SKULandscape) DemandKnowledge {
	if sk.Sales == nil {
		return DemandUnknown
	}
	var hasKnown bool
	var positive bool
	if units := sk.Sales.OrderedUnits.Current; units != nil {
		hasKnown = true
		if *units > 0 {
			positive = true
		}
	}
	if amount := sk.Sales.OrderedAmount.Current; amount != nil {
		r, err := parseDecLocal(amount.Amount)
		if err != nil {
			return DemandUnknown
		}
		hasKnown = true
		if r.Sign() > 0 {
			positive = true
		}
	}
	if positive {
		return DemandPositive
	}
	if hasKnown {
		return DemandZero
	}
	return DemandUnknown
}

type MoneyAccumulator struct {
	Sum           big.Rat
	KnownCount    int64
	Currency      string
	CurrencyValid bool
}

func (a *MoneyAccumulator) Add(m *MoneyAmount) bool {
	if a == nil || m == nil || m.Amount == "" {
		return false
	}
	r, err := parseDecLocal(m.Amount)
	if err != nil {
		return false
	}
	if a.KnownCount == 0 {
		a.Currency = m.CurrencyCode
		a.CurrencyValid = true
	} else if a.Currency != "" && m.CurrencyCode != "" && a.Currency != m.CurrencyCode {
		a.CurrencyValid = false
		return false
	}
	a.Sum.Add(&a.Sum, r)
	a.KnownCount++
	return true
}

func (a *MoneyAccumulator) Result() *MoneyAmount {
	if a == nil || a.KnownCount == 0 || !a.CurrencyValid {
		return nil
	}
	return &MoneyAmount{
		Amount:       canonicaljson.Decimal(formatRatLocal(&a.Sum)),
		CurrencyCode: a.Currency,
	}
}

type CountAccumulator struct {
	Sum          int64
	KnownCount   int64
	UnknownCount int64
}

func (a *CountAccumulator) Add(v *int64) {
	if a == nil || v == nil {
		return
	}
	a.Sum += *v
	a.KnownCount++
}

func (a *CountAccumulator) Observe(v *int64) {
	if a == nil {
		return
	}
	if v == nil {
		a.UnknownCount++
		return
	}
	a.Add(v)
}

func (a CountAccumulator) Result() *int64 {
	if a.KnownCount == 0 {
		return nil
	}
	v := a.Sum
	return &v
}

func (a CountAccumulator) Incomplete() bool {
	return a.KnownCount > 0 && a.UnknownCount > 0
}

func moneyIsKnownPositive(m *MoneyAmount) bool {
	if m == nil {
		return false
	}
	r, err := parseDecLocal(m.Amount)
	if err != nil {
		return false
	}
	return r.Sign() > 0
}

func cloneDateRange(p *DateRange) *DateRange {
	if p == nil {
		return nil
	}
	cp := *p
	return &cp
}

func cloneTimePtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	cp := *t
	return &cp
}

func dateRangesEqual(a, b *DateRange) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.From == b.From && a.To == b.To
}

func int64PtrKnown(v int64, known bool) *int64 {
	if !known {
		return nil
	}
	x := v
	return &x
}

func calculatedDRRComparison(spendCur, attrCur, spendPrev, attrPrev *MoneyAmount) *PercentagePointComparison {
	cur := calculatedDRRPct(spendCur, attrCur)
	prev := calculatedDRRPct(spendPrev, attrPrev)
	if cur == nil && prev == nil {
		return nil
	}
	cmp := BuildPctPointComparison(cur, prev)
	return &cmp
}

func calculatedDRRPct(spend, attr *MoneyAmount) *canonicaljson.Decimal {
	if spend == nil || attr == nil {
		return nil
	}
	num, err := parseDecLocal(spend.Amount)
	if err != nil || num.Sign() < 0 {
		return nil
	}
	den, err := parseDecLocal(attr.Amount)
	if err != nil || den.Sign() == 0 {
		return nil
	}
	drr := new(big.Rat).Quo(num, den)
	drr.Mul(drr, big.NewRat(100, 1))
	d := canonicaljson.Decimal(formatRatLocal(drr))
	return &d
}

func shareOfKnown(numerator, denominator *MoneyAmount) (*canonicaljson.Decimal, *Limitation) {
	return shareOfAccount(numerator, denominator)
}
