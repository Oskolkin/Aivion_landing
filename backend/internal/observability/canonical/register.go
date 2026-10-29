package canonical

import (
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/deep"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/discovery"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/evidence"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/publication"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/observability"
	"github.com/prometheus/client_golang/prometheus"
)

// Register attaches canonical Analysis / Chat / Eval collectors to a custom registry.
func Register(reg prometheus.Registerer) {
	discovery.Register(reg)
	evidence.Register(reg)
	deep.Register(reg)
	publication.Register(reg)
	observability.RegisterRuntimeMetrics(reg)
}
