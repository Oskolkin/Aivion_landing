package materiality

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
)

// Exact is a base-10 money/metric value stored as a decimal string.
type Exact = canonicaljson.Decimal

func parseRat(d Exact) (*big.Rat, error) {
	s := strings.TrimSpace(string(d))
	if s == "" {
		return nil, fmt.Errorf("materiality: empty exact value")
	}
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return nil, fmt.Errorf("materiality: invalid decimal %q", d)
	}
	return r, nil
}

func compareExact(a, b Exact) (int, error) {
	ar, err := parseRat(a)
	if err != nil {
		return 0, err
	}
	br, err := parseRat(b)
	if err != nil {
		return 0, err
	}
	return ar.Cmp(br), nil
}

func formatRat(r *big.Rat) Exact {
	if r == nil {
		return Exact("0")
	}
	s := r.FloatString(18)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	if s == "" || s == "-0" {
		return Exact("0")
	}
	return Exact(s)
}

func cmpRat(a, b *big.Rat) int {
	if a == nil {
		a = new(big.Rat)
	}
	if b == nil {
		b = new(big.Rat)
	}
	return a.Cmp(b)
}

func addRat(a, b *big.Rat) *big.Rat {
	return new(big.Rat).Add(nonzero(a), nonzero(b))
}

func subRat(a, b *big.Rat) *big.Rat {
	return new(big.Rat).Sub(nonzero(a), nonzero(b))
}

func mulRat(a, b *big.Rat) *big.Rat {
	return new(big.Rat).Mul(nonzero(a), nonzero(b))
}

func quoRat(a, b *big.Rat) *big.Rat {
	bb := nonzero(b)
	if bb.Sign() == 0 {
		return new(big.Rat)
	}
	return new(big.Rat).Quo(nonzero(a), bb)
}

func maxRat(a, b *big.Rat) *big.Rat {
	if cmpRat(a, b) >= 0 {
		return new(big.Rat).Set(nonzero(a))
	}
	return new(big.Rat).Set(nonzero(b))
}

func absRat(a *big.Rat) *big.Rat {
	r := new(big.Rat).Set(nonzero(a))
	if r.Sign() < 0 {
		r.Neg(r)
	}
	return r
}

func nonzero(a *big.Rat) *big.Rat {
	if a == nil {
		return new(big.Rat)
	}
	return a
}

func ratInt(n int64) *big.Rat {
	return new(big.Rat).SetInt64(n)
}

func ratFromFloat64(f float64) *big.Rat {
	r, _ := parseRat(decF(f))
	return r
}

func exactPositive(d Exact) bool {
	r, err := parseRat(d)
	if err != nil {
		return false
	}
	return r.Sign() > 0
}

func exactZero(d Exact) bool {
	r, err := parseRat(d)
	if err != nil {
		return false
	}
	return r.Sign() == 0
}

func formatExact(d Exact) (Exact, error) {
	r, err := parseRat(d)
	if err != nil {
		return Exact(""), err
	}
	return formatRat(r), nil
}

func shareOfRat(part, total *big.Rat) *big.Rat {
	if nonzero(total).Sign() <= 0 {
		return new(big.Rat)
	}
	return quoRat(part, total)
}

// parseExactDecimal parses a canonical decimal string into Exact or returns an error.
func parseExactDecimal(d canonicaljson.Decimal) (Exact, error) {
	s := strings.TrimSpace(string(d))
	if s == "" {
		return Exact(""), fmt.Errorf("invalid decimal %q", d)
	}
	if _, ok := new(big.Rat).SetString(s); !ok {
		return Exact(""), fmt.Errorf("invalid decimal %q", d)
	}
	return Exact(s), nil
}

// optionalExactDecimal returns nil when input is nil; invalid decimals are skipped (nil, error).
func optionalExactDecimal(d *canonicaljson.Decimal) (*Exact, error) {
	if d == nil {
		return nil, nil
	}
	ex, err := parseExactDecimal(*d)
	if err != nil {
		return nil, err
	}
	return &ex, nil
}

func exactInt(n int64) Exact {
	return Exact(big.NewInt(n).String())
}

func optionalExactInt(n *int64) *Exact {
	if n == nil {
		return nil
	}
	ex := exactInt(*n)
	return &ex
}
