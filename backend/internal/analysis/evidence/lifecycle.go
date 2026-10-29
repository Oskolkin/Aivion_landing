package evidence

import (
	"context"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

func registerLifecycle(r *Registry, api Reads) {
	r.Register(fnResolver{key: "SKU_POSTING_LIFECYCLE_CONTEXT", fn: func(ctx context.Context, rc factualapi.FactualReadContext, reqs []NormalizedEvidenceRequest) []MaterializationResult {
		out := make([]MaterializationResult, len(reqs))
		p := eventPeriod30(rc)
		b := CurrentBudget().For("SKU_POSTING_LIFECYCLE_CONTEXT")
		for i, req := range reqs {
			id, err := parseID(req.Subject)
			if err != nil {
				out[i] = failResult(req, "INVALID_SUBJECT", err.Error(), "FAILED")
				continue
			}
			page, e := api.ListLifecycleEvents(ctx, rc, factualapi.LifecycleEventFilter{SKUID: &id, BusinessPeriod: &p}, factualapi.PageRequest{PageSize: b.MaxRows})
			out[i] = eventPageResult(req, rc, e, page, "effective_at", "EVENT_HISTORY_30D")
		}
		return out
	}})
	r.Register(fnResolver{key: "SKU_RETURNS_CONTEXT", fn: func(ctx context.Context, rc factualapi.FactualReadContext, reqs []NormalizedEvidenceRequest) []MaterializationResult {
		out := make([]MaterializationResult, len(reqs))
		b := CurrentBudget().For("SKU_RETURNS_CONTEXT")
		for i, req := range reqs {
			id, err := parseID(req.Subject)
			if err != nil {
				out[i] = failResult(req, "INVALID_SUBJECT", err.Error(), "FAILED")
				continue
			}
			page, e := api.ListReturns(ctx, rc, factualapi.ReturnFilter{SKUID: &id}, factualapi.PageRequest{PageSize: b.MaxRows})
			out[i] = eventPageResult(req, rc, e, page, "observed_at", "EVENT_HISTORY_30D")
		}
		return out
	}})
}

func eventPageResult[T any](req NormalizedEvidenceRequest, rc factualapi.FactualReadContext, err error, page factualapi.Page[T], sortBasis, scope string) MaterializationResult {
	t, _ := CatalogType(req.EvidenceType)
	temp := temporalFor(t, rc)
	if err != nil {
		return failResult(req, "FACTUAL_READ_FAILED", err.Error(), "FAILED")
	}
	items, tr := boundList(page.Items, CurrentBudget().For(req.EvidenceType).MaxRows, sortBasis, scope, nil)
	if page.NextPageToken != "" {
		tr.Truncated = true
	}
	st := StatusComplete
	if len(items) == 0 {
		st = StatusEmptyValid
	} else if tr.Truncated {
		st = StatusPartial
	}
	facts := []DeclaredFact{}
	return MaterializationResult{Request: req, Status: st, Readiness: "READY", Temporal: temp, Truncation: tr, Data: map[string]any{"events": items}, Facts: facts, APICalls: 1}
}
