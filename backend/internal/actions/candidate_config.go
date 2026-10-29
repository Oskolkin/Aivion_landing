package actions

import "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"

// CandidateBuilderConfig holds tunable thresholds for action candidate builders.
type CandidateBuilderConfig struct {
	StockCriticalCoverageDays float64
	StockWarningCoverageDays  float64

	HighDRRThresholdPct float64
	MinAdSpend7d        float64

	MaxCandidatesPerDomain     int
	MaxTopExamplesPerCandidate int
	MaxFactsPerCandidate       int
}

// DefaultCandidateBuilderConfig returns production defaults aligned with dashboard/ad analytics.
func DefaultCandidateBuilderConfig() CandidateBuilderConfig {
	return CandidateBuilderConfig{
		StockCriticalCoverageDays:  7,
		StockWarningCoverageDays:   14,
		HighDRRThresholdPct:        analytics.AdEfficiencyHighDRRThresholdPct,
		MinAdSpend7d:               1000,
		MaxCandidatesPerDomain:     10,
		MaxTopExamplesPerCandidate: 5,
		MaxFactsPerCandidate:       8,
	}
}
