package evidence

import (
	"context"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

func registerFunnel(r *Registry, api Reads) {
	r.Register(fnResolver{key: "ACCOUNT_FUNNEL_HISTORY", fn: func(ctx context.Context, rc factualapi.FactualReadContext, reqs []NormalizedEvidenceRequest) []MaterializationResult {
		caps, err := api.GetFunnelCapabilities(ctx, rc)
		out := make([]MaterializationResult, len(reqs))
		metric := "IMPRESSIONS"
		if err == nil && len(caps.Metrics) > 0 {
			metric = caps.Metrics[0].MetricKey
		}
		fact, e := api.GetAccountFunnelPeriod(ctx, rc, metric, factualapi.PeriodCurrent7D, nil)
		for i, req := range reqs {
			if e != nil || err != nil {
				msg := "funnel read failed"
				if e != nil {
					msg = e.Error()
				}
				out[i] = failResult(req, "FACTUAL_READ_FAILED", msg, "FAILED")
				continue
			}
			facts := []DeclaredFact{}
			if fact.Metric.Meta.FactRef != "" {
				facts = append(facts, DeclaredFact{SemanticPath: "funnel.account." + metric, FactRefs: []string{fact.Metric.Meta.FactRef}})
			}
			out[i] = MaterializationResult{Request: req, Status: StatusComplete, Readiness: fact.Metric.Meta.Readiness, Temporal: temporalFor(mustType("ACCOUNT_FUNNEL_HISTORY"), rc), Data: map[string]any{"metric": fact, "non_additive": true, "not_search_pdp_cart_order_chain": true}, Facts: facts, APICalls: 2}
		}
		return out
	}})
	r.Register(fnResolver{key: "CATEGORY_FUNNEL_CONTEXT", fn: func(ctx context.Context, rc factualapi.FactualReadContext, reqs []NormalizedEvidenceRequest) []MaterializationResult {
		out := make([]MaterializationResult, len(reqs))
		for i, req := range reqs {
			id, err := parseID(req.Subject)
			if err != nil {
				out[i] = failResult(req, "INVALID_SUBJECT", err.Error(), "FAILED")
				continue
			}
			fact, e := api.GetCategoryFunnelPeriod(ctx, rc, id, "IMPRESSIONS", factualapi.PeriodCurrent7D, nil)
			if e != nil {
				out[i] = failResult(req, "FACTUAL_READ_FAILED", e.Error(), "FAILED")
				continue
			}
			out[i] = MaterializationResult{Request: req, Status: StatusComplete, Readiness: fact.Metric.Meta.Readiness, Temporal: temporalFor(mustType("CATEGORY_FUNNEL_CONTEXT"), rc), Data: map[string]any{"metric": fact, "non_additive": true}, APICalls: 1}
		}
		return out
	}})
	r.Register(fnResolver{key: "SKU_FUNNEL_HISTORY", fn: func(ctx context.Context, rc factualapi.FactualReadContext, reqs []NormalizedEvidenceRequest) []MaterializationResult {
		out := make([]MaterializationResult, len(reqs))
		for i, req := range reqs {
			id, err := parseID(req.Subject)
			if err != nil {
				out[i] = failResult(req, "INVALID_SUBJECT", err.Error(), "FAILED")
				continue
			}
			pts, e := api.GetSKUFunnelSeries(ctx, rc, id, "IMPRESSIONS", factualapi.PeriodTrend30D, nil)
			if e != nil {
				out[i] = failResult(req, "FACTUAL_READ_FAILED", e.Error(), "FAILED")
				continue
			}
			st := StatusComplete
			if len(pts) == 0 {
				st = StatusEmptyValid
			}
			out[i] = MaterializationResult{Request: req, Status: st, Readiness: "READY", Temporal: temporalFor(mustType("SKU_FUNNEL_HISTORY"), rc), Data: map[string]any{"points": pts, "non_additive_do_not_sum": true}, APICalls: 1}
		}
		return out
	}})
}
