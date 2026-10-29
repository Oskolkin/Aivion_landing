package metrics

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

func StartMetricsServer(port string, registry *prometheus.Registry, log *zap.Logger, service string) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	addr := fmt.Sprintf(":%s", port)

	go func() {
		log.Info("metrics server starting",
			zap.String("metrics_port", port),
			zap.String("metrics_path", "/metrics"),
			zap.String("metrics_service", service),
		)

		if err := http.ListenAndServe(addr, mux); err != nil {
			log.Error("metrics server failed",
				zap.String("metrics_port", port),
				zap.Error(err),
			)
		}
	}()
}
