package sales

import (
	"encoding/json"
	"fmt"
	"math/big"
	"regexp"
	"strings"
)

var decimalPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)(?:\.([0-9]{1,6}))?$`)
var ErrInvalidSalesAmount = fmt.Errorf("INVALID_SALES_AMOUNT")
var ErrInvalidOrderedUnits = fmt.Errorf("INVALID_ORDERED_UNITS")

// Decimal is non-negative fixed-point money with six fractional digits.
type Decimal struct{ micros *big.Int }

func ParseAmount(n json.Number) (Decimal, error) {
	s := n.String()
	m := decimalPattern.FindStringSubmatch(s)
	if m == nil {
		return Decimal{}, fmt.Errorf("%w: %q", ErrInvalidSalesAmount, s)
	}
	frac := m[2] + strings.Repeat("0", 6-len(m[2]))
	whole, _ := new(big.Int).SetString(m[1], 10)
	whole.Mul(whole, big.NewInt(1_000_000))
	fraction := new(big.Int)
	if frac != "" {
		fraction.SetString(frac, 10)
	}
	return Decimal{micros: whole.Add(whole, fraction)}, nil
}

func ParseUnits(n json.Number) (int64, error) {
	rat, ok := new(big.Rat).SetString(n.String())
	if !ok || rat.Sign() < 0 || !rat.IsInt() || !rat.Num().IsInt64() {
		return 0, fmt.Errorf("%w: %q", ErrInvalidOrderedUnits, n)
	}
	return rat.Num().Int64(), nil
}

func Zero() Decimal { return Decimal{micros: new(big.Int)} }
func (d Decimal) Add(v Decimal) Decimal {
	return Decimal{micros: new(big.Int).Add(d.value(), v.value())}
}
func (d Decimal) Equal(v Decimal) bool { return d.value().Cmp(v.value()) == 0 }
func (d Decimal) IsZero() bool         { return d.value().Sign() == 0 }
func (d Decimal) value() *big.Int {
	if d.micros == nil {
		return new(big.Int)
	}
	return d.micros
}
func (d Decimal) String() string {
	return formatMicros(d.value())
}
func (d Decimal) SubString(v Decimal) string {
	return formatMicros(new(big.Int).Sub(d.value(), v.value()))
}
func formatMicros(n *big.Int) string {
	sign := ""
	value := new(big.Int).Set(n)
	if value.Sign() < 0 {
		sign = "-"
		value.Abs(value)
	}
	v := value.String()
	for len(v) < 7 {
		v = "0" + v
	}
	whole, frac := v[:len(v)-6], strings.TrimRight(v[len(v)-6:], "0")
	if frac == "" {
		return sign + whole
	}
	return sign + whole + "." + frac
}
