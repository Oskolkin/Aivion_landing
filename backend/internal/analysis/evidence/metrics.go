package evidence

import "github.com/prometheus/client_golang/prometheus"

var (
	requestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "evidence_requests_total", Help: "Normalized evidence requests by type.",
	}, []string{"type"})
	itemsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "evidence_items_materialized_total", Help: "Materialized evidence items.",
	}, []string{"type", "status"})
	dedupRatio = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "evidence_dedup_ratio", Help: "Unique/raw evidence request ratio.",
	})
	itemsPerThread = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "evidence_items_per_thread", Help: "Evidence items referenced per thread.",
	})
	bytesTotal = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "evidence_bytes_total", Help: "Evidence bundle size in bytes.",
	})
	tokensEst = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "evidence_tokens_estimated", Help: "Estimated evidence bundle tokens.",
	})
	buildDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "evidence_build_duration", Help: "Evidence expansion duration seconds.",
	})
	partialRate = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "evidence_partial_rate", Help: "Evidence expansions with partial items.",
	})
	failedRate = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "evidence_failed_rate", Help: "Evidence expansions with failed items.",
	})
)

func collectors() []prometheus.Collector {
	return []prometheus.Collector{requestsTotal, itemsTotal, dedupRatio, itemsPerThread, bytesTotal, tokensEst, buildDuration, partialRate, failedRate}
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

func ObserveExpansion(rep ExpansionReportV1, limited bool) {
	for k, n := range rep.ItemsByType {
		requestsTotal.WithLabelValues(k).Add(float64(n))
	}
	itemsTotal.WithLabelValues("all", "COMPLETE").Add(float64(rep.ItemsComplete))
	itemsTotal.WithLabelValues("all", "EMPTY_VALID").Add(float64(rep.ItemsEmptyValid))
	itemsTotal.WithLabelValues("all", "PARTIAL").Add(float64(rep.ItemsPartial))
	itemsTotal.WithLabelValues("all", "FAILED").Add(float64(rep.ItemsFailed))
	if rep.RawRequests > 0 {
		r, _ := strconvParse(rep.DedupRatio)
		dedupRatio.Observe(r)
	}
	bytesTotal.Observe(float64(rep.Bytes))
	tokensEst.Observe(float64(rep.EstimatedTokens))
	if rep.ItemsPartial > 0 {
		partialRate.Inc()
	}
	if rep.ItemsFailed > 0 {
		failedRate.Inc()
	}
	_ = limited
}

func strconvParse(s string) (float64, error) {
	var f float64
	_, err := fmtSscanf(s, &f)
	return f, err
}

func fmtSscanf(s string, f *float64) (int, error) {
	n := 0
	dot := false
	frac := 0.1
	for _, c := range s {
		if c == '.' {
			dot = true
			continue
		}
		if c < '0' || c > '9' {
			break
		}
		d := float64(c - '0')
		if !dot {
			*f = *f*10 + d
		} else {
			*f += d * frac
			frac *= 0.1
		}
		n++
	}
	return n, nil
}
