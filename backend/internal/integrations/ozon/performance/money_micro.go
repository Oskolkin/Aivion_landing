package performance

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

const microRublesPerRuble = 1_000_000

// NormalizeMicroRubles converts Performance API micro-ruble values to ruble decimal strings without float64.
func NormalizeMicroRubles(raw int64) string {
	if raw == 0 {
		return ""
	}
	rat := new(big.Rat).SetFrac(big.NewInt(raw), big.NewInt(microRublesPerRuble))
	return rat.FloatString(2)
}

// ParseMicroRublesFromAny extracts int64 micro-ruble amount from API payload values.
func ParseMicroRublesFromAny(m map[string]any, keys ...string) int64 {
	for _, key := range keys {
		if v, ok := m[key]; ok && v != nil {
			switch val := v.(type) {
			case int64:
				return val
			case int:
				return int64(val)
			case json.Number:
				i, err := val.Int64()
				if err == nil {
					return i
				}
				if s := strings.TrimSpace(val.String()); s != "" {
					if i, err := strconv.ParseInt(s, 10, 64); err == nil {
						return i
					}
				}
			case string:
				if i, err := strconv.ParseInt(strings.TrimSpace(val), 10, 64); err == nil {
					return i
				}
			case float64:
				// Refuse float money/id paths; exact decoding must use json.Number.
				continue
			}
		}
	}
	return 0
}

// ExactNumberString returns a decimal/string form that preserves json.Number digits.
func ExactNumberString(v any) (string, error) {
	switch x := v.(type) {
	case nil:
		return "", nil
	case string:
		return strings.TrimSpace(x), nil
	case json.Number:
		return x.String(), nil
	case int64:
		return strconv.FormatInt(x, 10), nil
	case int:
		return strconv.Itoa(x), nil
	case float64:
		return "", fmt.Errorf("float64 money/id path is forbidden")
	default:
		return "", fmt.Errorf("unsupported numeric type %T", v)
	}
}
