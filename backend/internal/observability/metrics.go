package observability

import (
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	analysisRunsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "analysis_runs_total", Help: "Analysis runs by execution class and status.",
	}, []string{"execution_class", "status"})
	analysisRunsPublishedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "analysis_runs_published_total", Help: "Successfully published production analysis runs.",
	})
	analysisRunsNoopTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "analysis_runs_noop_total", Help: "NO_OP analysis runs (zero AI calls).",
	})
	analysisRunsFailedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "analysis_runs_failed_total", Help: "Failed analysis runs.",
	}, []string{"reason_code"})
	analysisRunDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "analysis_run_duration_seconds", Help: "Analysis run wall time.",
	}, []string{"execution_class"})
	analysisStageDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "analysis_stage_duration_seconds", Help: "Stage wall time.",
	}, []string{"stage"})
	analysisStageRetries = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "analysis_stage_retries_total", Help: "Stage retries.",
	}, []string{"stage", "reason_code"})
	analysisInvariantViolations = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "analysis_invariant_violations_total", Help: "Absolute invariant violations (alertable).",
	}, []string{"invariant"})
	chatTurnsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "chat_turns_total", Help: "Chat turns by status and answer_mode.",
	}, []string{"status", "answer_mode"})
	chatTurnDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "chat_turn_duration_seconds", Help: "Chat turn duration.",
	})
	chatToolCalls = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "chat_tool_calls_total", Help: "Chat tool executions.",
	}, []string{"tool_key", "status"})
	chatToolRounds = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "chat_tool_rounds", Help: "Tool rounds per turn.",
	})
	chatValidationFailures = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "chat_validation_failures_total", Help: "Chat validation failures.",
	}, []string{"reason_code"})
	chatProviderRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "chat_provider_requests_total", Help: "Chat provider HTTP/API requests.",
	}, []string{"provider", "result"})
	chatInputTokens = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "chat_input_tokens_total", Help: "Chat provider input tokens.",
	})
	chatOutputTokens = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "chat_output_tokens_total", Help: "Chat provider output tokens.",
	})
	evalRunsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eval_runs_total", Help: "Eval suite runs.",
	}, []string{"suite_kind", "split", "verdict"})
	evalCaseRunsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eval_case_runs_total", Help: "Eval case runs.",
	}, []string{"suite_kind", "split", "status"})
	evalCriticalFailures = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "eval_critical_failures_total", Help: "Eval critical failures.",
	}, []string{"suite_kind"})
	evalRunDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "eval_run_duration_seconds", Help: "Eval run duration.",
	}, []string{"suite_kind"})
	evalInputTokens = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "eval_input_tokens_total", Help: "Eval provider input tokens.",
	})
	evalOutputTokens = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "eval_output_tokens_total", Help: "Eval provider output tokens.",
	})
)

func runtimeCollectors() []prometheus.Collector {
	return []prometheus.Collector{
		analysisRunsTotal, analysisRunsPublishedTotal, analysisRunsNoopTotal, analysisRunsFailedTotal,
		analysisRunDuration, analysisStageDuration, analysisStageRetries, analysisInvariantViolations,
		chatTurnsTotal, chatTurnDuration, chatToolCalls, chatToolRounds, chatValidationFailures,
		chatProviderRequests, chatInputTokens, chatOutputTokens,
		evalRunsTotal, evalCaseRunsTotal, evalCriticalFailures, evalRunDuration, evalInputTokens, evalOutputTokens,
	}
}

func RegisterRuntimeMetrics(reg prometheus.Registerer) {
	if reg == nil {
		return
	}
	for _, c := range runtimeCollectors() {
		if err := reg.Register(c); err != nil {
			if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
				panic(err)
			}
		}
	}
}

func ObserveAnalysisRun(executionClass, status string, seconds float64) {
	analysisRunsTotal.WithLabelValues(executionClass, status).Inc()
	if seconds > 0 {
		analysisRunDuration.WithLabelValues(executionClass).Observe(seconds)
	}
	switch status {
	case "PUBLISHED":
		analysisRunsPublishedTotal.Inc()
	case "NO_OP":
		analysisRunsNoopTotal.Inc()
	}
}

func ObserveAnalysisFailed(reasonCode string) {
	analysisRunsFailedTotal.WithLabelValues(reasonCode).Inc()
}

func ObserveAnalysisStage(stage string, seconds float64) {
	if stage == "" {
		return
	}
	analysisStageDuration.WithLabelValues(stage).Observe(seconds)
}

func ObserveAnalysisStageRetry(stage, reasonCode string) {
	analysisStageRetries.WithLabelValues(stage, reasonCode).Inc()
}

func ObserveInvariantViolation(invariant string) {
	analysisInvariantViolations.WithLabelValues(invariant).Inc()
}

func ObserveChatTurn(status, answerMode string, seconds float64, rounds int, inTok, outTok int) {
	if answerMode == "" {
		answerMode = "UNKNOWN"
	}
	chatTurnsTotal.WithLabelValues(status, answerMode).Inc()
	if seconds > 0 {
		chatTurnDuration.Observe(seconds)
	}
	if rounds > 0 {
		chatToolRounds.Observe(float64(rounds))
	}
	if inTok > 0 {
		chatInputTokens.Add(float64(inTok))
	}
	if outTok > 0 {
		chatOutputTokens.Add(float64(outTok))
	}
}

func ObserveChatTool(toolKey, status string) {
	if toolKey == "" {
		toolKey = "unknown"
	}
	chatToolCalls.WithLabelValues(toolKey, status).Inc()
}

func ObserveChatValidation(reasonCode string) {
	chatValidationFailures.WithLabelValues(reasonCode).Inc()
}

func ObserveChatProvider(provider, result string) {
	if provider == "" {
		provider = "openai"
	}
	chatProviderRequests.WithLabelValues(provider, result).Inc()
}

func ObserveEvalRun(suiteKind, split, verdict string, seconds float64, critical int, inTok, outTok int64) {
	evalRunsTotal.WithLabelValues(suiteKind, split, verdict).Inc()
	if seconds > 0 {
		evalRunDuration.WithLabelValues(suiteKind).Observe(seconds)
	}
	if critical > 0 {
		evalCriticalFailures.WithLabelValues(suiteKind).Add(float64(critical))
	}
	if inTok > 0 {
		evalInputTokens.Add(float64(inTok))
	}
	if outTok > 0 {
		evalOutputTokens.Add(float64(outTok))
	}
}

func ObserveEvalCase(suiteKind, split, status string) {
	evalCaseRunsTotal.WithLabelValues(suiteKind, split, status).Inc()
}

func ForbiddenMetricLabel(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "seller_id", "run_id", "publication_id", "turn_id", "session_id",
		"sku_id", "campaign_id", "raw_error", "prompt", "question", "provider_request_id":
		return true
	default:
		return false
	}
}
