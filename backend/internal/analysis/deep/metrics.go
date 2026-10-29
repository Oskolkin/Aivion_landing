package deep

import (
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	packBuildTotal = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "deep_pack_build_total", Help: "Deep pack builds."}, []string{"status"})
	packBytes      = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "deep_pack_bytes", Help: "Deep pack bytes."})
	packTokens     = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "deep_pack_tokens_estimated", Help: "Deep pack estimated tokens."})
	aiAttempts     = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "deep_ai_attempts_total", Help: "Deep AI attempts."}, []string{"status"})
	aiLatency      = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "deep_ai_latency", Help: "Deep AI latency seconds."})
	inputTokens    = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "deep_input_tokens", Help: "Deep input tokens."})
	cachedTokens   = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "deep_cached_input_tokens", Help: "Deep cached input tokens."})
	reasonTokens   = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "deep_reasoning_tokens", Help: "Deep reasoning tokens."})
	outputTokens   = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "deep_output_tokens", Help: "Deep output tokens."})
	threadRes      = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "deep_thread_resolutions_total", Help: "Thread resolutions."}, []string{"resolution"})
	findingsTotal  = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "deep_findings_total", Help: "Findings."}, []string{"direction", "origin"})
	relsTotal      = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "deep_relationships_total", Help: "Relationships."}, []string{"basis"})
	recsTotal      = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "deep_recommendations_total", Help: "Recommendations."}, []string{"actionability"})
	actsTotal      = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "deep_action_proposals_total", Help: "Action proposals."}, []string{"action_type"})
	valFail        = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "deep_validation_failures_total", Help: "Validation failures by reason_code."}, []string{"reason_code"})
	noThreads      = prometheus.NewCounter(prometheus.CounterOpts{Name: "deep_no_meaningful_threads_total", Help: "NO_MEANINGFUL_THREADS packs."})
)

func collectors() []prometheus.Collector {
	return []prometheus.Collector{packBuildTotal, packBytes, packTokens, aiAttempts, aiLatency, inputTokens, cachedTokens, reasonTokens, outputTokens, threadRes, findingsTotal, relsTotal, recsTotal, actsTotal, valFail, noThreads}
}

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

func ObservePack(status string, bytes, tokens, facts, evidence int) {
	packBuildTotal.WithLabelValues(status).Inc()
	packBytes.Observe(float64(bytes))
	packTokens.Observe(float64(tokens))
	_ = facts
	_ = evidence
}

func ObserveAI(status string, latencySec float64, in, out, cached, reason int64) {
	aiAttempts.WithLabelValues(status).Inc()
	aiLatency.Observe(latencySec)
	inputTokens.Observe(float64(in))
	outputTokens.Observe(float64(out))
	cachedTokens.Observe(float64(cached))
	reasonTokens.Observe(float64(reason))
}

func ObserveValidated(out OutputV1, noMeaningful bool) {
	if noMeaningful {
		noThreads.Inc()
	}
	for _, r := range out.ThreadResolutions {
		threadRes.WithLabelValues(r.Resolution).Inc()
	}
	for _, f := range out.Findings {
		findingsTotal.WithLabelValues(f.BusinessDirection, f.OriginType).Inc()
		for _, ra := range f.RelationshipAssessments {
			relsTotal.WithLabelValues(ra.RelationshipBasis).Inc()
		}
	}
	for _, r := range out.Recommendations {
		recsTotal.WithLabelValues(r.Actionability).Inc()
	}
	for _, a := range out.ActionProposals {
		actsTotal.WithLabelValues(a.ActionType).Inc()
	}
}

func ObserveValidationFailure(reason string) {
	valFail.WithLabelValues(metrics.NormalizeReasonCode(reason)).Inc()
}
