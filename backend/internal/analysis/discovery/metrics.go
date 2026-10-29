package discovery

import (
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	packBuildTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "discovery_pack_build_total", Help: "Discovery pack builds by status.",
	}, []string{"status"})
	packBytes = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "discovery_pack_bytes", Help: "Discovery pack artifact size in bytes.",
	})
	packTokens = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "discovery_pack_tokens_estimated", Help: "Estimated Discovery pack input tokens.",
	})
	factHandles = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "discovery_fact_handles_total", Help: "Fact handles per pack.",
	})
	evidenceMenuTypes = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "discovery_evidence_menu_types", Help: "Evidence menu type count.",
	})
	aiAttempts = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "discovery_ai_attempts_total", Help: "Discovery AI attempts by status.",
	}, []string{"status"})
	aiLatency = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "discovery_ai_latency", Help: "Discovery AI provider latency seconds.",
	})
	inputTokens = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "discovery_input_tokens", Help: "Discovery AI input tokens.",
	})
	outputTokens = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "discovery_output_tokens", Help: "Discovery AI output tokens.",
	})
	cachedTokens = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "discovery_cached_tokens", Help: "Discovery AI cached input tokens.",
	})
	validationFailures = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "discovery_validation_failures", Help: "Discovery validation failures by reason_code.",
	}, []string{"reason_code"})
	threadsTotal = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "discovery_threads_total", Help: "Validated Discovery thread count.",
	})
	evidenceRequests = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "discovery_evidence_requests_total", Help: "Validated evidence request count.",
	})
	noThreads = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "discovery_no_meaningful_threads_total", Help: "NO_MEANINGFUL_THREADS outcomes.",
	})
)

func collectors() []prometheus.Collector {
	return []prometheus.Collector{packBuildTotal, packBytes, packTokens, factHandles, evidenceMenuTypes,
		aiAttempts, aiLatency, inputTokens, outputTokens, cachedTokens, validationFailures,
		threadsTotal, evidenceRequests, noThreads}
}

// Register attaches Discovery collectors to the process custom Prometheus registry.
func Register(reg prometheus.Registerer) {
	if reg == nil {
		return
	}
	for _, c := range collectors() {
		if err := reg.Register(c); err != nil {
			if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
				panic(err)
			}
		}
	}
}

func ObservePack(status string, bytes int, tokens, handles, menu int) {
	packBuildTotal.WithLabelValues(status).Inc()
	packBytes.Observe(float64(bytes))
	packTokens.Observe(float64(tokens))
	factHandles.Observe(float64(handles))
	evidenceMenuTypes.Observe(float64(menu))
}

func ObserveAI(status string, latencySec float64, in, out, cached int64) {
	aiAttempts.WithLabelValues(status).Inc()
	aiLatency.Observe(latencySec)
	inputTokens.Observe(float64(in))
	outputTokens.Observe(float64(out))
	cachedTokens.Observe(float64(cached))
}

func ObserveValidation(rep ValidationReportV1, state DiscoveryState) {
	if !rep.Valid {
		reason := metrics.ReasonUnknown
		if len(rep.Errors) > 0 {
			reason = metrics.NormalizeReasonCode(rep.Errors[0])
		}
		validationFailures.WithLabelValues(reason).Inc()
		return
	}
	threadsTotal.Observe(float64(rep.ThreadCount))
	evidenceRequests.Observe(float64(rep.EvidenceRequests))
	if state == StateNoMeaningfulThreads {
		noThreads.Inc()
	}
}
