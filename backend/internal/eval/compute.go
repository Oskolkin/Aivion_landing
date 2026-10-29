package eval

import (
	"fmt"
	"math/big"
	"strings"
)

func sumSales(days []SalesDay) (amount, units string, err error) {
	am := new(big.Rat)
	un := new(big.Rat)
	for _, d := range days {
		a, e := parseRat(d.OrderedAmount)
		if e != nil {
			return "", "", e
		}
		u, e := parseRat(d.OrderedUnits)
		if e != nil {
			return "", "", e
		}
		am.Add(am, a)
		un.Add(un, u)
	}
	return formatRat(am), formatRat(un), nil
}

func accountDRRPct(spend, ordered string) (string, error) {
	num, err := parseRat(spend)
	if err != nil {
		return "", err
	}
	den, err := parseRat(ordered)
	if err != nil {
		return "", err
	}
	if den.Sign() == 0 {
		return "", fmt.Errorf("drr denominator zero")
	}
	out := new(big.Rat).Quo(num, den)
	out.Mul(out, big.NewRat(100, 1))
	return formatRat(out), nil
}

func conversionDeltaPP(current, prev string) (string, error) {
	c, err := parseRat(current)
	if err != nil {
		return "", err
	}
	p, err := parseRat(prev)
	if err != nil {
		return "", err
	}
	return formatRat(new(big.Rat).Sub(c, p)), nil
}

func parseRat(s string) (*big.Rat, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return big.NewRat(0, 1), nil
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
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if s == "" || s == "-0" {
		return "0"
	}
	return s
}
