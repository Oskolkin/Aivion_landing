package outputvalidator

import "math"

// PercentMatchesClaim reports whether a free-text percentage equals a
// structured metric_claim / threshold_claim value under display rounding.
// Supports 0/1/2 decimal display forms of the claim (e.g. 26.234 → 26,2%).
func PercentMatchesClaim(textValue, claimValue float64) bool {
	const eps = 1e-9
	if math.Abs(textValue-claimValue) <= 0.05+eps {
		return true
	}
	for _, places := range []float64{1, 10, 100} {
		rounded := math.Round(claimValue*places) / places
		if math.Abs(textValue-rounded) <= 0.05+eps {
			return true
		}
	}
	return false
}
