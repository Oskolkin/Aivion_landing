package evidence

import (
	"context"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

func registerSearch(r *Registry, api Reads) {
	r.Register(fnResolver{key: "SKU_SEARCH_HISTORY", fn: func(ctx context.Context, rc factualapi.FactualReadContext, reqs []NormalizedEvidenceRequest) []MaterializationResult {
		out := make([]MaterializationResult, len(reqs))
		for i, req := range reqs {
			id, err := parseID(req.Subject)
			if err != nil {
				out[i] = failResult(req, "INVALID_SUBJECT", err.Error(), "FAILED")
				continue
			}
			pts, e := api.GetSearchSummarySeries(ctx, rc, id, factualapi.PeriodTrend30D, nil)
			if e != nil {
				out[i] = failResult(req, "FACTUAL_READ_FAILED", e.Error(), "FAILED")
				continue
			}
			st := StatusComplete
			if len(pts) == 0 {
				st = StatusEmptyValid
			}
			facts := []DeclaredFact{}
			for _, p := range pts {
				if p.Meta.FactRef != "" {
					facts = append(facts, DeclaredFact{SemanticPath: "search.summary." + p.PeriodKind, FactRefs: []string{p.Meta.FactRef}})
				}
			}
			out[i] = MaterializationResult{Request: req, Status: st, Readiness: "READY", Temporal: temporalFor(mustType("SKU_SEARCH_HISTORY"), rc), Data: map[string]any{"periods": pts, "provider_period_semantics": true, "unique_users_non_additive": true}, Facts: facts, APICalls: 1}
		}
		return out
	}})
	r.Register(fnResolver{key: "SKU_SEARCH_QUERIES", fn: func(ctx context.Context, rc factualapi.FactualReadContext, reqs []NormalizedEvidenceRequest) []MaterializationResult {
		ids := []int64{}
		out := make([]MaterializationResult, len(reqs))
		b := CurrentBudget().For("SKU_SEARCH_QUERIES")
		for i, req := range reqs {
			id, err := parseID(req.Subject)
			if err != nil {
				out[i] = failResult(req, "INVALID_SUBJECT", err.Error(), "FAILED")
				continue
			}
			ids = append(ids, id)
		}
		batch, err := api.BatchGetSKUQueryDetails(ctx, rc, ids, factualapi.PeriodCurrent7D, nil, b.MaxRows)
		by := map[int64]factualapi.BatchSKUQueryDetailsItem{}
		if err == nil {
			for _, it := range batch.Items {
				by[it.SKUID] = it
			}
		}
		for i, req := range reqs {
			if out[i].Status != "" {
				continue
			}
			if err != nil {
				out[i] = failResult(req, "FACTUAL_READ_FAILED", err.Error(), "FAILED")
				continue
			}
			id, _ := parseID(req.Subject)
			it := by[id]
			st := StatusComplete
			if len(it.Items) == 0 {
				st = StatusEmptyValid
			}
			tr := TruncationMeta{Returned: len(it.Items), Truncated: it.HasMore, SortBasis: "provider_rank", SemanticScope: "TOP_N_PROVIDER_RANKED"}
			out[i] = MaterializationResult{Request: req, Status: st, Readiness: "READY", Temporal: temporalFor(mustType("SKU_SEARCH_QUERIES"), rc), Truncation: tr, Data: map[string]any{"queries": it.Items, "semantic_scope": "TOP_N_PROVIDER_RANKED", "not_full_query_universe": true}, APICalls: 1}
		}
		return out
	}})
	r.Register(fnResolver{key: "SKU_SEARCH_QUERY_COMPARISON", fn: func(ctx context.Context, rc factualapi.FactualReadContext, reqs []NormalizedEvidenceRequest) []MaterializationResult {
		ids := []int64{}
		out := make([]MaterializationResult, len(reqs))
		b := CurrentBudget().For("SKU_SEARCH_QUERY_COMPARISON")
		for i, req := range reqs {
			id, err := parseID(req.Subject)
			if err != nil {
				out[i] = failResult(req, "INVALID_SUBJECT", err.Error(), "FAILED")
				continue
			}
			ids = append(ids, id)
		}
		cur, e1 := api.BatchGetSKUQueryDetails(ctx, rc, ids, factualapi.PeriodCurrent7D, nil, b.MaxRows)
		prev, e2 := api.BatchGetSKUQueryDetails(ctx, rc, ids, factualapi.PeriodPrevious7D, nil, b.MaxRows)
		if e1 != nil || e2 != nil {
			for i, req := range reqs {
				if out[i].Status == "" {
					out[i] = failResult(req, "FACTUAL_READ_FAILED", "query comparison read failed", "FAILED")
				}
			}
			return out
		}
		curBy := map[int64]map[int64]bool{}
		prevBy := map[int64]map[int64]bool{}
		for _, it := range cur.Items {
			m := map[int64]bool{}
			for _, q := range it.Items {
				m[q.SearchQueryID] = true
			}
			curBy[it.SKUID] = m
		}
		for _, it := range prev.Items {
			m := map[int64]bool{}
			for _, q := range it.Items {
				m[q.SearchQueryID] = true
			}
			prevBy[it.SKUID] = m
		}
		for i, req := range reqs {
			if out[i].Status != "" {
				continue
			}
			id, _ := parseID(req.Subject)
			appeared, disappeared := []int64{}, []int64{}
			for qid := range curBy[id] {
				if !prevBy[id][qid] {
					appeared = append(appeared, qid)
				}
			}
			for qid := range prevBy[id] {
				if !curBy[id][qid] {
					disappeared = append(disappeared, qid)
				}
			}
			out[i] = MaterializationResult{Request: req, Status: StatusComplete, Readiness: "READY", Temporal: temporalFor(mustType("SKU_SEARCH_QUERY_COMPARISON"), rc), Truncation: TruncationMeta{SemanticScope: "TOP_N_PROVIDER_RANKED", Returned: len(appeared) + len(disappeared)}, Data: map[string]any{"appeared_in_returned_topN": appeared, "disappeared_from_returned_topN": disappeared, "not_market_disappearance": true}, APICalls: 2}
		}
		return out
	}})
	r.Register(fnResolver{key: "SEARCH_QUERY_MARKET_CONTEXT", fn: func(ctx context.Context, rc factualapi.FactualReadContext, reqs []NormalizedEvidenceRequest) []MaterializationResult {
		b := CurrentBudget().For("SEARCH_QUERY_MARKET_CONTEXT")
		page, err := api.ListMarketSearchFacts(ctx, rc, factualapi.PageRequest{PageSize: b.MaxRows})
		out := make([]MaterializationResult, len(reqs))
		for i, req := range reqs {
			if err != nil {
				out[i] = failResult(req, "FACTUAL_READ_FAILED", err.Error(), "FAILED")
				continue
			}
			items, tr := boundList(page.Items, b.MaxRows, "provider_rank", "TOP_N_PROVIDER_RANKED", nil)
			if page.NextPageToken != "" {
				tr.Truncated = true
			}
			st := StatusComplete
			if len(items) == 0 {
				st = StatusEmptyValid
			}
			out[i] = MaterializationResult{Request: req, Status: st, Readiness: "READY", Temporal: temporalFor(mustType("SEARCH_QUERY_MARKET_CONTEXT"), rc), Truncation: tr, Data: map[string]any{"market_facts": items, "market_search_is_not_seller_traffic": true}, APICalls: 1}
		}
		return out
	}})
}
