package canonical

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/deep"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/discovery"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/publication"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/observability"
	"github.com/prometheus/client_golang/prometheus"
)

func TestPrompt16MetricsCustomRegistry(t *testing.T) {
	reg := prometheus.NewRegistry()
	Register(reg)
	discovery.ObserveAI("ok", 0.01, 10, 5, 1)
	deep.ObserveAI("ok", 0.02, 11, 6, 1, 0)
	publication.ObservePublish("ok", 0.01)
	observability.ObserveChatTurn("COMPLETED", "FACTUAL_QUERY", 0.2, 2, 10, 4)
	observability.ObserveEvalRun("FACTUAL_GOLDEN", "HOLDOUT", "PASS", 1, 0, 3, 2)
	observability.ObserveAnalysisRun("PRODUCTION", "PUBLISHED", 1.5)
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, mf := range mfs {
		names[mf.GetName()] = true
	}
	for _, n := range []string{
		"discovery_ai_attempts_total", "deep_ai_attempts_total", "publication_results_total",
		"chat_turns_total", "eval_runs_total", "analysis_runs_total",
	} {
		if !names[n] {
			t.Fatalf("missing metric %s in custom registry", n)
		}
	}
	dmf, _ := prometheus.DefaultGatherer.Gather()
	for _, mf := range dmf {
		if mf.GetName() == "discovery_ai_attempts_total" {
			t.Fatal("canonical collectors must not register on DefaultGatherer")
		}
	}
}
