package alerts

import "math"

func DeltaPercent(current, previous float64) (float64, bool) {
	if previous == 0 {
		return 0, false
	}
	return (current - previous) / previous * 100.0, true
}

func DeltaAbsolute(current, previous float64) float64 {
	return current - previous
}

func IsDropAtOrBelow(deltaPercent, threshold float64) bool {
	return deltaPercent <= threshold
}

func ScoreSeverity(deltaPercent float64) Severity {
	return SeverityFromDropPercent(deltaPercent)
}

func ScoreUrgency(daysOfCover float64) Urgency {
	return UrgencyFromDaysOfCover(daysOfCover)
}

func SeverityFromDropPercent(deltaPercent float64) Severity {
	// Negative delta means drop, positive means growth.
	if deltaPercent <= -50 {
		return SeverityCritical
	}
	if deltaPercent <= -30 {
		return SeverityHigh
	}
	if deltaPercent <= -15 {
		return SeverityMedium
	}
	return SeverityLow
}

func SeverityFromRatio(value float64, criticalAtOrBelow float64, highAtOrBelow float64, mediumAtOrBelow float64) Severity {
	switch {
	case value <= criticalAtOrBelow:
		return SeverityCritical
	case value <= highAtOrBelow:
		return SeverityHigh
	case value <= mediumAtOrBelow:
		return SeverityMedium
	default:
		return SeverityLow
	}
}

func UrgencyFromDaysOfCover(days float64) Urgency {
	if math.IsNaN(days) || math.IsInf(days, 0) {
		return UrgencyLow
	}
	if days <= 1 {
		return UrgencyImmediate
	}
	if days <= 3 {
		return UrgencyHigh
	}
	if days <= 7 {
		return UrgencyMedium
	}
	return UrgencyLow
}
