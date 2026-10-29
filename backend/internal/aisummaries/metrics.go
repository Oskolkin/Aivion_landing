package aisummaries

import (
	"log/slog"
	"sync/atomic"
)

var summaryMetrics = struct {
	GeneratedTotal         int64
	CompletedTotal         int64
	CompletedRepairedTotal int64
	CompletedPartialTotal  int64
	FailedTotal            int64
	BlockFallbackTotal     int64
	InvalidJSONTotal       int64
	OpenAITruncatedTotal   int64
}{}

func recordSummaryGenerated() {
	atomic.AddInt64(&summaryMetrics.GeneratedTotal, 1)
}

func recordSummaryOutcome(status string, repair SummaryRepairResult, invalidJSON bool, truncated bool) {
	switch status {
	case "completed":
		atomic.AddInt64(&summaryMetrics.CompletedTotal, 1)
	case "completed_repaired":
		atomic.AddInt64(&summaryMetrics.CompletedRepairedTotal, 1)
	case "completed_partial":
		atomic.AddInt64(&summaryMetrics.CompletedPartialTotal, 1)
	case "failed":
		atomic.AddInt64(&summaryMetrics.FailedTotal, 1)
	}
	if invalidJSON {
		atomic.AddInt64(&summaryMetrics.InvalidJSONTotal, 1)
	}
	if truncated {
		atomic.AddInt64(&summaryMetrics.OpenAITruncatedTotal, 1)
	}
	for range repair.FallbackBlocks {
		atomic.AddInt64(&summaryMetrics.BlockFallbackTotal, 1)
	}
}

func logSummaryMetrics() {
	generated := atomic.LoadInt64(&summaryMetrics.GeneratedTotal)
	completed := atomic.LoadInt64(&summaryMetrics.CompletedTotal)
	repaired := atomic.LoadInt64(&summaryMetrics.CompletedRepairedTotal)
	partial := atomic.LoadInt64(&summaryMetrics.CompletedPartialTotal)
	accepted := completed + repaired + partial
	var acceptanceRate float64
	if generated > 0 {
		acceptanceRate = float64(accepted) / float64(generated) * 100
	}
	slog.Info("ai_daily_summary_metrics",
		"generated_total", generated,
		"completed_total", completed,
		"completed_repaired_total", repaired,
		"completed_partial_total", partial,
		"failed_total", atomic.LoadInt64(&summaryMetrics.FailedTotal),
		"block_fallback_total", atomic.LoadInt64(&summaryMetrics.BlockFallbackTotal),
		"invalid_json_total", atomic.LoadInt64(&summaryMetrics.InvalidJSONTotal),
		"openai_truncated_total", atomic.LoadInt64(&summaryMetrics.OpenAITruncatedTotal),
		"acceptance_rate_pct", acceptanceRate,
	)
}
