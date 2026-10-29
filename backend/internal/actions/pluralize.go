package actions

import (
	"fmt"
	"strings"
)

// ruPlural picks the correct Russian plural form for n.
//
//	one  -> 1, 21, 31 ... (but not 11)
//	few  -> 2-4, 22-24 ... (but not 12-14)
//	many -> 0, 5-20, 25-30 ...
func ruPlural(n int, one, few, many string) string {
	if n < 0 {
		n = -n
	}
	mod100 := n % 100
	mod10 := n % 10
	switch {
	case mod100 >= 11 && mod100 <= 14:
		return many
	case mod10 == 1:
		return one
	case mod10 >= 2 && mod10 <= 4:
		return few
	default:
		return many
	}
}

// campaignsNominative returns the noun "кампания/кампании/кампаний" agreeing with n.
func campaignsNominative(n int) string {
	return ruPlural(n, "кампания", "кампании", "кампаний")
}

// campaignsAccusative returns the accusative noun for phrasings like "Проверить N кампанию".
func campaignsAccusative(n int) string {
	return ruPlural(n, "кампанию", "кампании", "кампаний")
}

// campaignsPrepositional returns "по N кампании/кампаниям".
func campaignsPrepositional(n int) string {
	return ruPlural(n, "кампании", "кампаниям", "кампаниям")
}

// campaignsGenitive returns genitive for "у N кампаний".
func campaignsGenitive(n int) string {
	if n%10 == 1 && n%100 != 11 {
		return "кампании"
	}
	return "кампаний"
}

// campaignsDative is an alias for prepositional in operational copy.
func campaignsDative(n int) string {
	return campaignsPrepositional(n)
}

// formatThousands formats an integer rouble amount with non-breaking-free space
// thousands separators (e.g. 21568 -> "21 568"). The validator collapses spaced
// thousands before number extraction, so the figure stays backend-verified.
func formatThousands(v float64) string {
	neg := v < 0
	if neg {
		v = -v
	}
	intPart := int64(v + 0.5)
	s := fmt.Sprintf("%d", intPart)
	var b strings.Builder
	n := len(s)
	for i, ch := range s {
		if i > 0 && (n-i)%3 == 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(ch)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}
