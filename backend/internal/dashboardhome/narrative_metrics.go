package dashboardhome

import (
	"log/slog"
	"sync/atomic"
)

type narrativeMetricsSnapshot struct {
	GeneratedTotal             int64
	CompletedTotal             int64
	CompletedWithWarningsTotal int64
	CompletedRepairedTotal     int64
	CompletedPartialTotal      int64
	FailedTotal                int64
	FullFallbackTotal          int64
	BlockFallbackTotal         int64
	StaleTotal                 int64
	OpenAITruncatedTotal       int64
}

var narrativeMetrics narrativeMetricsSnapshot

func recordNarrativeGenerated() {
	atomic.AddInt64(&narrativeMetrics.GeneratedTotal, 1)
}

func recordNarrativeGenerationOutcome(status string, validationErrors []string) {
	switch status {
	case "completed":
		atomic.AddInt64(&narrativeMetrics.CompletedTotal, 1)
	case "completed_with_warnings":
		atomic.AddInt64(&narrativeMetrics.CompletedWithWarningsTotal, 1)
	case "completed_repaired":
		atomic.AddInt64(&narrativeMetrics.CompletedRepairedTotal, 1)
	case "completed_partial":
		atomic.AddInt64(&narrativeMetrics.CompletedPartialTotal, 1)
	case "failed":
		atomic.AddInt64(&narrativeMetrics.FailedTotal, 1)
		for _, err := range validationErrors {
			if err == "openai_output_incomplete_length" {
				atomic.AddInt64(&narrativeMetrics.OpenAITruncatedTotal, 1)
			}
		}
	}
}

func recordNarrativeAcceptance(outcome narrativeAcceptanceOutcome) {
	if outcome.Applied {
		switch outcome.ValidationStatus {
		case "completed":
			atomic.AddInt64(&narrativeMetrics.CompletedTotal, 1)
		case "completed_with_warnings":
			atomic.AddInt64(&narrativeMetrics.CompletedWithWarningsTotal, 1)
		case "completed_repaired":
			atomic.AddInt64(&narrativeMetrics.CompletedRepairedTotal, 1)
		case "completed_partial":
			atomic.AddInt64(&narrativeMetrics.CompletedPartialTotal, 1)
		}
		return
	}
	if outcome.ValidationStatus == "fallback_stale" {
		atomic.AddInt64(&narrativeMetrics.StaleTotal, 1)
	}
	atomic.AddInt64(&narrativeMetrics.FullFallbackTotal, 1)
}

func recordNarrativeBlockFallback(block string) {
	if block == "" {
		return
	}
	atomic.AddInt64(&narrativeMetrics.BlockFallbackTotal, 1)
}

func narrativeGenerationStatusAccepted(status string) bool {
	switch status {
	case "completed", "completed_with_warnings", "completed_repaired", "completed_partial":
		return true
	default:
		return false
	}
}

func narrativeAcceptanceRate() float64 {
	generated := atomic.LoadInt64(&narrativeMetrics.GeneratedTotal)
	if generated == 0 {
		return 0
	}
	accepted := atomic.LoadInt64(&narrativeMetrics.CompletedTotal) +
		atomic.LoadInt64(&narrativeMetrics.CompletedWithWarningsTotal) +
		atomic.LoadInt64(&narrativeMetrics.CompletedRepairedTotal) +
		atomic.LoadInt64(&narrativeMetrics.CompletedPartialTotal)
	return float64(accepted) / float64(generated)
}

func logNarrativeMetrics() {
	slog.Info("dashboard narrative metrics",
		"generated_total", atomic.LoadInt64(&narrativeMetrics.GeneratedTotal),
		"completed_total", atomic.LoadInt64(&narrativeMetrics.CompletedTotal),
		"completed_with_warnings_total", atomic.LoadInt64(&narrativeMetrics.CompletedWithWarningsTotal),
		"completed_repaired_total", atomic.LoadInt64(&narrativeMetrics.CompletedRepairedTotal),
		"completed_partial_total", atomic.LoadInt64(&narrativeMetrics.CompletedPartialTotal),
		"failed_total", atomic.LoadInt64(&narrativeMetrics.FailedTotal),
		"full_fallback_total", atomic.LoadInt64(&narrativeMetrics.FullFallbackTotal),
		"block_fallback_total", atomic.LoadInt64(&narrativeMetrics.BlockFallbackTotal),
		"stale_total", atomic.LoadInt64(&narrativeMetrics.StaleTotal),
		"openai_truncated_total", atomic.LoadInt64(&narrativeMetrics.OpenAITruncatedTotal),
		"acceptance_rate", narrativeAcceptanceRate(),
	)
}

func resetNarrativeMetricsForTest() {
	narrativeMetrics = narrativeMetricsSnapshot{}
}
