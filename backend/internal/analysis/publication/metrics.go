package publication

import "github.com/prometheus/client_golang/prometheus"

var (
	publishTotal = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "publication_publish_total", Help: "Publish attempts."}, []string{"status"})
	staleTotal   = prometheus.NewCounter(prometheus.CounterOpts{Name: "publication_stale_runs_total", Help: "Stale publish attempts."})
	txLatency    = prometheus.NewHistogram(prometheus.HistogramOpts{Name: "publication_transaction_latency", Help: "Publish transaction seconds."})
	resultsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "publication_results_total", Help: "Publication outcomes."}, []string{"result"})
)

func collectors() []prometheus.Collector {
	return []prometheus.Collector{publishTotal, staleTotal, txLatency, resultsTotal}
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

func ObservePublish(status string, seconds float64) {
	publishTotal.WithLabelValues(status).Inc()
	resultsTotal.WithLabelValues(status).Inc()
	if seconds > 0 {
		txLatency.Observe(seconds)
	}
	if status == "stale" {
		staleTotal.Inc()
	}
}
