package evidence

import (
	"context"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

func registerHealth(r *Registry, api Reads) {
	r.Register(fnResolver{key: "ACCOUNT_RATING_HISTORY", fn: func(ctx context.Context, rc factualapi.FactualReadContext, reqs []NormalizedEvidenceRequest) []MaterializationResult {
		ratings, err := api.GetCurrentRatings(ctx, rc)
		out := make([]MaterializationResult, len(reqs))
		b := CurrentBudget().For("ACCOUNT_RATING_HISTORY")
		var hist factualapi.Page[factualapi.RatingPeriodFact]
		var he error
		if err == nil && len(ratings) > 0 {
			hist, he = api.GetRatingHistory(ctx, rc, ratings[0].ID, factualapi.PageRequest{PageSize: b.MaxObservations})
		}
		for i, req := range reqs {
			if err != nil {
				out[i] = failResult(req, "FACTUAL_READ_FAILED", err.Error(), "FAILED")
				continue
			}
			if len(ratings) == 0 {
				out[i] = emptyResult(req, temporalFor(mustType("ACCOUNT_RATING_HISTORY"), rc), "no_ratings")
				continue
			}
			if he != nil {
				out[i] = failResult(req, "FACTUAL_READ_FAILED", he.Error(), "FAILED")
				continue
			}
			items, tr := boundList(hist.Items, b.MaxObservations, "period", "PROVIDER_PERIOD_HISTORY", nil)
			st := StatusComplete
			if len(items) == 0 {
				st = StatusEmptyValid
			}
			out[i] = MaterializationResult{Request: req, Status: st, Readiness: "READY", Temporal: temporalFor(mustType("ACCOUNT_RATING_HISTORY"), rc), Truncation: tr, Data: map[string]any{"ratings": ratings, "history": items, "provider_period_semantics": true}, APICalls: 2}
		}
		return out
	}})
	r.Register(fnResolver{key: "ACCOUNT_RATING_THRESHOLD_CONTEXT", fn: func(ctx context.Context, rc factualapi.FactualReadContext, reqs []NormalizedEvidenceRequest) []MaterializationResult {
		ratings, err := api.GetCurrentRatings(ctx, rc)
		out := make([]MaterializationResult, len(reqs))
		var th []factualapi.RatingThreshold
		var te error
		if err == nil && len(ratings) > 0 {
			th, te = api.GetRatingThresholds(ctx, rc, ratings[0].ID)
		}
		for i, req := range reqs {
			if err != nil || te != nil {
				out[i] = failResult(req, "FACTUAL_READ_FAILED", "rating thresholds failed", "FAILED")
				continue
			}
			if len(ratings) == 0 {
				out[i] = emptyResult(req, temporalFor(mustType("ACCOUNT_RATING_THRESHOLD_CONTEXT"), rc), "no_ratings")
				continue
			}
			out[i] = MaterializationResult{Request: req, Status: StatusComplete, Readiness: "READY", Temporal: temporalFor(mustType("ACCOUNT_RATING_THRESHOLD_CONTEXT"), rc), Data: map[string]any{"thresholds": th}, APICalls: 2}
		}
		return out
	}})
	r.Register(fnResolver{key: "ACCOUNT_FBS_ERROR_HISTORY", fn: func(ctx context.Context, rc factualapi.FactualReadContext, reqs []NormalizedEvidenceRequest) []MaterializationResult {
		idx, err := api.GetFBSErrorIndex(ctx, rc)
		out := make([]MaterializationResult, len(reqs))
		for i, req := range reqs {
			if err != nil {
				out[i] = failResult(req, "FACTUAL_READ_FAILED", err.Error(), "FAILED")
				continue
			}
			facts := []DeclaredFact{}
			if idx.Meta.FactRef != "" {
				facts = append(facts, DeclaredFact{SemanticPath: "health.fbs_index", FactRefs: []string{idx.Meta.FactRef}})
			}
			out[i] = MaterializationResult{Request: req, Status: StatusComplete, Readiness: idx.Meta.Readiness, Temporal: temporalFor(mustType("ACCOUNT_FBS_ERROR_HISTORY"), rc), Data: idx, Facts: facts, APICalls: 1}
		}
		return out
	}})
	r.Register(fnResolver{key: "ACCOUNT_FBS_ERROR_POSTINGS", fn: func(ctx context.Context, rc factualapi.FactualReadContext, reqs []NormalizedEvidenceRequest) []MaterializationResult {
		b := CurrentBudget().For("ACCOUNT_FBS_ERROR_POSTINGS")
		page, err := api.ListFBSRatingPostingErrors(ctx, rc, factualapi.PageRequest{PageSize: b.MaxRows})
		out := make([]MaterializationResult, len(reqs))
		for i, req := range reqs {
			if err != nil {
				out[i] = failResult(req, "FACTUAL_READ_FAILED", err.Error(), "FAILED")
				continue
			}
			items, tr := boundList(page.Items, b.MaxRows, "observed_at", "EVENT_HISTORY_30D", nil)
			st := StatusComplete
			if len(items) == 0 {
				st = StatusEmptyValid
			}
			out[i] = MaterializationResult{Request: req, Status: st, Readiness: "READY", Temporal: temporalFor(mustType("ACCOUNT_FBS_ERROR_POSTINGS"), rc), Truncation: tr, Data: map[string]any{"posting_errors": items, "pii_excluded": true}, APICalls: 1}
		}
		return out
	}})
}

func registerSupply(r *Registry, api Reads) {
	r.Register(fnResolver{key: "SKU_SUPPLY_CONTEXT", fn: func(ctx context.Context, rc factualapi.FactualReadContext, reqs []NormalizedEvidenceRequest) []MaterializationResult {
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
		batch, err := api.BatchGetSKUInbound(ctx, rc, ids)
		by := map[int64]factualapi.SKUInbound{}
		if err == nil {
			for _, it := range batch.Items {
				if it.Inbound != nil {
					by[it.ID] = *it.Inbound
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
			inb, ok := by[id]
			if !ok {
				out[i] = emptyResult(req, temporalFor(mustType("SKU_SUPPLY_CONTEXT"), rc), "no_open_inbound")
				out[i].APICalls = 1
				continue
			}
			facts := []DeclaredFact{}
			if inb.Meta.FactRef != "" {
				facts = append(facts, DeclaredFact{SemanticPath: "supply.sku.inbound", FactRefs: []string{inb.Meta.FactRef}})
			}
			out[i] = MaterializationResult{Request: req, Status: StatusComplete, Readiness: inb.Meta.Readiness, Temporal: temporalFor(mustType("SKU_SUPPLY_CONTEXT"), rc), Data: map[string]any{"inbound": inb, "inbound_is_not_current_stock": true, "no_future_stock": true}, Facts: facts, APICalls: 1}
		}
		return out
	}})
	r.Register(fnResolver{key: "SUPPLY_LIFECYCLE_DETAIL", fn: func(ctx context.Context, rc factualapi.FactualReadContext, reqs []NormalizedEvidenceRequest) []MaterializationResult {
		page, err := api.ListSupplies(ctx, rc, factualapi.SupplyFilter{}, factualapi.PageRequest{PageSize: CurrentBudget().For("SUPPLY_LIFECYCLE_DETAIL").MaxRows})
		out := make([]MaterializationResult, len(reqs))
		for i, req := range reqs {
			if err != nil {
				out[i] = failResult(req, "FACTUAL_READ_FAILED", err.Error(), "FAILED")
				continue
			}
			if len(page.Items) == 0 {
				out[i] = emptyResult(req, temporalFor(mustType("SUPPLY_LIFECYCLE_DETAIL"), rc), "no_supplies")
				continue
			}
			hist, he := api.GetSupplyStateHistory(ctx, rc, page.Items[0].ID, factualapi.PageRequest{PageSize: CurrentBudget().For(req.EvidenceType).MaxObservations})
			if he != nil {
				out[i] = failResult(req, "FACTUAL_READ_FAILED", he.Error(), "FAILED")
				continue
			}
			items, tr := boundList(hist.Items, CurrentBudget().For(req.EvidenceType).MaxObservations, "observed_at", "EVENT_HISTORY_30D", nil)
			out[i] = MaterializationResult{Request: req, Status: StatusComplete, Readiness: "READY", Temporal: temporalFor(mustType("SUPPLY_LIFECYCLE_DETAIL"), rc), Truncation: tr, Data: map[string]any{"history": items, "fbo_only": true}, APICalls: 2}
		}
		return out
	}})
	r.Register(fnResolver{key: "SUPPLY_CONTENT_DETAIL", fn: func(ctx context.Context, rc factualapi.FactualReadContext, reqs []NormalizedEvidenceRequest) []MaterializationResult {
		page, err := api.ListSupplies(ctx, rc, factualapi.SupplyFilter{}, factualapi.PageRequest{PageSize: 8})
		out := make([]MaterializationResult, len(reqs))
		for i, req := range reqs {
			if err != nil || len(page.Items) == 0 {
				if err != nil {
					out[i] = failResult(req, "FACTUAL_READ_FAILED", err.Error(), "FAILED")
				} else {
					out[i] = emptyResult(req, temporalFor(mustType("SUPPLY_CONTENT_DETAIL"), rc), "no_supplies")
				}
				continue
			}
			items, e := api.ListSupplyPlannedItems(ctx, rc, page.Items[0].ID, factualapi.PageRequest{PageSize: CurrentBudget().For(req.EvidenceType).MaxRows})
			if e != nil {
				out[i] = failResult(req, "FACTUAL_READ_FAILED", e.Error(), "FAILED")
				continue
			}
			rows, tr := boundList(items.Items, CurrentBudget().For(req.EvidenceType).MaxRows, "observed_at", "CURRENT_SNAPSHOT", nil)
			out[i] = MaterializationResult{Request: req, Status: StatusComplete, Readiness: "READY", Temporal: temporalFor(mustType("SUPPLY_CONTENT_DETAIL"), rc), Truncation: tr, Data: map[string]any{"planned_items": rows}, APICalls: 2}
		}
		return out
	}})
	r.Register(fnResolver{key: "SUPPLY_ACCEPTANCE_CONTEXT", fn: func(ctx context.Context, rc factualapi.FactualReadContext, reqs []NormalizedEvidenceRequest) []MaterializationResult {
		page, err := api.ListSupplies(ctx, rc, factualapi.SupplyFilter{}, factualapi.PageRequest{PageSize: 8})
		out := make([]MaterializationResult, len(reqs))
		for i, req := range reqs {
			if err != nil || len(page.Items) == 0 {
				if err != nil {
					out[i] = failResult(req, "FACTUAL_READ_FAILED", err.Error(), "FAILED")
				} else {
					out[i] = emptyResult(req, temporalFor(mustType("SUPPLY_ACCEPTANCE_CONTEXT"), rc), "no_supplies")
				}
				continue
			}
			acc, e := api.GetSupplyAcceptance(ctx, rc, page.Items[0].ID)
			if e != nil {
				out[i] = emptyResult(req, temporalFor(mustType("SUPPLY_ACCEPTANCE_CONTEXT"), rc), "no_acceptance_act")
				out[i].APICalls = 2
				continue
			}
			out[i] = MaterializationResult{Request: req, Status: StatusComplete, Readiness: acc.Meta.Readiness, Temporal: temporalFor(mustType("SUPPLY_ACCEPTANCE_CONTEXT"), rc), Data: map[string]any{"acceptance": acc, "unidentified_accepted_qty_not_allocated": true}, APICalls: 2}
		}
		return out
	}})
	r.Register(fnResolver{key: "SKU_ACCEPTANCE_DISCREPANCIES", fn: func(ctx context.Context, rc factualapi.FactualReadContext, reqs []NormalizedEvidenceRequest) []MaterializationResult {
		page, err := api.ListSupplies(ctx, rc, factualapi.SupplyFilter{}, factualapi.PageRequest{PageSize: 8})
		out := make([]MaterializationResult, len(reqs))
		for i, req := range reqs {
			if err != nil || len(page.Items) == 0 {
				if err != nil {
					out[i] = failResult(req, "FACTUAL_READ_FAILED", err.Error(), "FAILED")
				} else {
					out[i] = emptyResult(req, temporalFor(mustType("SKU_ACCEPTANCE_DISCREPANCIES"), rc), "no_supplies")
				}
				continue
			}
			items, e := api.ListSupplyAcceptanceItems(ctx, rc, page.Items[0].ID, factualapi.PageRequest{PageSize: CurrentBudget().For(req.EvidenceType).MaxRows})
			if e != nil {
				out[i] = emptyResult(req, temporalFor(mustType("SKU_ACCEPTANCE_DISCREPANCIES"), rc), "no_acceptance_items")
				continue
			}
			rows, tr := boundList(items.Items, CurrentBudget().For(req.EvidenceType).MaxRows, "id", "EVENT_HISTORY_30D", nil)
			st := StatusComplete
			if len(rows) == 0 {
				st = StatusEmptyValid
			}
			out[i] = MaterializationResult{Request: req, Status: st, Readiness: "READY", Temporal: temporalFor(mustType("SKU_ACCEPTANCE_DISCREPANCIES"), rc), Truncation: tr, Data: map[string]any{"items": rows}, APICalls: 2}
		}
		return out
	}})
}
