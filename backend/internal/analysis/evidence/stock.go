package evidence

import (
	"context"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

type stockObs struct {
	ObservationAt string `json:"observation_at"`
	SamplingKind  string `json:"sampling_kind"`
	SellableUnits *int64 `json:"sellable_units,omitempty"`
	FactRef       string `json:"fact_ref,omitempty"`
}

func registerStock(r *Registry, api Reads) {
	r.Register(fnResolver{key: "SKU_STOCK_HISTORY", fn: func(ctx context.Context, rc factualapi.FactualReadContext, reqs []NormalizedEvidenceRequest) []MaterializationResult {
		out := make([]MaterializationResult, len(reqs))
		b := CurrentBudget().For("SKU_STOCK_HISTORY")
		for i, req := range reqs {
			id, err := parseID(req.Subject)
			if err != nil {
				out[i] = failResult(req, "INVALID_SUBJECT", err.Error(), "FAILED")
				continue
			}
			page, e := api.GetSKUStockHistory(ctx, rc, id, factualapi.PageRequest{PageSize: b.MaxObservations})
			if e != nil {
				out[i] = failResult(req, "FACTUAL_READ_FAILED", e.Error(), "FAILED")
				continue
			}
			obs := []stockObs{}
			facts := []DeclaredFact{}
			for _, s := range page.Items {
				o := stockObs{ObservationAt: s.SnapshotAt.UTC().Format("2006-01-02T15:04:05Z"), SamplingKind: "LAST_OBSERVED_SNAPSHOT_OF_DAY", SellableUnits: knownInt(s.SellableUnits), FactRef: s.Meta.FactRef}
				obs = append(obs, o)
				if s.Meta.FactRef != "" {
					facts = append(facts, DeclaredFact{SemanticPath: "stock.snapshot." + o.ObservationAt, FactRefs: []string{s.Meta.FactRef}})
				}
			}
			obs, tr := boundList(obs, b.MaxObservations, "snapshot_at_desc", "OBSERVATION_HISTORY", nil)
			if page.NextPageToken != "" {
				tr.Truncated = true
			}
			st := StatusComplete
			if len(obs) == 0 {
				st = StatusEmptyValid
			}
			out[i] = MaterializationResult{Request: req, Status: st, Readiness: "READY", Temporal: temporalFor(mustType("SKU_STOCK_HISTORY"), rc), Truncation: tr, Data: map[string]any{"observations": obs, "not_synthetic_eod": true, "inbound_not_included": true}, Facts: facts, APICalls: 1}
		}
		return out
	}})
	r.Register(fnResolver{key: "SKU_STOCK_LOCATION_CURRENT", fn: func(ctx context.Context, rc factualapi.FactualReadContext, reqs []NormalizedEvidenceRequest) []MaterializationResult {
		ids := []int64{}
		out := make([]MaterializationResult, len(reqs))
		for i, req := range reqs {
			id, err := parseID(req.Subject)
			if err != nil {
				out[i] = failResult(req, "INVALID_SUBJECT", err.Error(), "FAILED")
				continue
			}
			ids = append(ids, id)
		}
		batch, err := api.BatchGetSKUStock(ctx, rc, ids)
		by := map[int64]factualapi.SKUStock{}
		if err == nil {
			for _, it := range batch.Items {
				if it.Stock != nil {
					by[it.ID] = *it.Stock
				}
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
			stck, ok := by[id]
			if !ok {
				out[i] = emptyResult(req, temporalFor(mustType("SKU_STOCK_LOCATION_CURRENT"), rc), "no_stock_snapshot")
				out[i].APICalls = 1
				continue
			}
			pos := stck.Positions
			pos, tr := boundList(pos, CurrentBudget().For(req.EvidenceType).MaxRows, "location", "CURRENT_SNAPSHOT", nil)
			facts := []DeclaredFact{}
			if stck.Meta.FactRef != "" {
				facts = append(facts, DeclaredFact{SemanticPath: "stock.current", FactRefs: []string{stck.Meta.FactRef}})
			}
			out[i] = MaterializationResult{Request: req, Status: StatusComplete, Readiness: stck.Meta.Readiness, Temporal: temporalFor(mustType("SKU_STOCK_LOCATION_CURRENT"), rc), Truncation: tr, Data: map[string]any{"positions": pos, "snapshot_at": stck.SnapshotAt.UTC().Format("2006-01-02T15:04:05Z")}, Facts: facts, APICalls: 1}
		}
		return out
	}})
	r.Register(fnResolver{key: "CATEGORY_STOCK_DISTRIBUTION", fn: func(ctx context.Context, rc factualapi.FactualReadContext, reqs []NormalizedEvidenceRequest) []MaterializationResult {
		out := make([]MaterializationResult, len(reqs))
		b := CurrentBudget().For("CATEGORY_STOCK_DISTRIBUTION")
		for i, req := range reqs {
			id, err := parseID(req.Subject)
			if err != nil {
				out[i] = failResult(req, "INVALID_SUBJECT", err.Error(), "FAILED")
				continue
			}
			page, e := api.ListSKUStock(ctx, rc, factualapi.StockFilter{CategoryIDs: []int64{id}}, factualapi.PageRequest{PageSize: b.MaxRows})
			if e != nil {
				out[i] = failResult(req, "FACTUAL_READ_FAILED", e.Error(), "FAILED")
				continue
			}
			items, tr := boundList(page.Items, b.MaxRows, "sellable_units", "CURRENT_SNAPSHOT", nil)
			if page.NextPageToken != "" {
				tr.Truncated = true
			}
			st := StatusComplete
			if len(items) == 0 {
				st = StatusEmptyValid
			}
			out[i] = MaterializationResult{Request: req, Status: st, Readiness: "READY", Temporal: temporalFor(mustType("CATEGORY_STOCK_DISTRIBUTION"), rc), Truncation: tr, Data: map[string]any{"skus": items}, APICalls: 1}
		}
		return out
	}})
}
