package evidence

import (
	"context"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

func registerFinance(r *Registry, api Reads) {
	r.Register(fnResolver{key: "ACCOUNT_FINANCE_HISTORY", fn: func(ctx context.Context, rc factualapi.FactualReadContext, reqs []NormalizedEvidenceRequest) []MaterializationResult {
		p := flowPeriod90(rc)
		pts, err := api.GetFinanceDailyLedger(ctx, rc, factualapi.PeriodTrend30D, &p)
		out := make([]MaterializationResult, len(reqs))
		for i, req := range reqs {
			if err != nil {
				out[i] = failResult(req, "FACTUAL_READ_FAILED", err.Error(), "FAILED")
				continue
			}
			st := StatusComplete
			if len(pts) == 0 {
				st = StatusEmptyValid
			}
			facts := []DeclaredFact{}
			for _, d := range pts {
				if d.Meta.FactRef != "" {
					facts = append(facts, DeclaredFact{SemanticPath: "finance.day." + d.BusinessDate.Format("2006-01-02"), FactRefs: []string{d.Meta.FactRef}})
				}
			}
			out[i] = MaterializationResult{Request: req, Status: st, Readiness: "READY", Temporal: temporalFor(mustType("ACCOUNT_FINANCE_HISTORY"), rc), Data: map[string]any{"ledger": pts, "accrual_is_not_sales_revenue": true, "accrual_is_not_profit": true}, Facts: facts, APICalls: 1}
		}
		return out
	}})
	r.Register(fnResolver{key: "POSTING_FINANCE_DETAIL", fn: func(ctx context.Context, rc factualapi.FactualReadContext, reqs []NormalizedEvidenceRequest) []MaterializationResult {
		p := eventPeriod30(rc)
		b := CurrentBudget().For("POSTING_FINANCE_DETAIL")
		page, err := api.ListFinanceAccruals(ctx, rc, factualapi.FinanceAccrualFilter{Period: &p}, factualapi.PageRequest{PageSize: b.MaxRows})
		out := make([]MaterializationResult, len(reqs))
		for i, req := range reqs {
			if err != nil {
				out[i] = failResult(req, "FACTUAL_READ_FAILED", err.Error(), "FAILED")
				continue
			}
			items, tr := boundList(page.Items, b.MaxRows, "business_date", "direct_mapped_accruals", nil)
			if page.NextPageToken != "" {
				tr.Truncated = true
			}
			st := StatusComplete
			if len(items) == 0 {
				st = StatusEmptyValid
			}
			out[i] = MaterializationResult{Request: req, Status: st, Readiness: "READY", Temporal: temporalFor(mustType("POSTING_FINANCE_DETAIL"), rc), Truncation: tr, Coverage: Coverage{Notes: []string{"account_only_fees_not_allocated"}}, Data: map[string]any{"accruals": items, "direct_mapped_only": true}, APICalls: 1}
		}
		return out
	}})
}

func registerPromotions(r *Registry, api Reads) {
	r.Register(fnResolver{key: "SKU_PROMOTION_CONTEXT", fn: func(ctx context.Context, rc factualapi.FactualReadContext, reqs []NormalizedEvidenceRequest) []MaterializationResult {
		ids := []int64{}
		out := make([]MaterializationResult, len(reqs))
		b := CurrentBudget().For("SKU_PROMOTION_CONTEXT")
		for i, req := range reqs {
			id, err := parseID(req.Subject)
			if err != nil {
				out[i] = failResult(req, "INVALID_SUBJECT", err.Error(), "FAILED")
				continue
			}
			ids = append(ids, id)
		}
		batch, err := api.BatchGetSKUPromotions(ctx, rc, ids, b.MaxRows)
		by := map[int64]factualapi.BatchSKUPromotionsItem{}
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
			tr := TruncationMeta{Returned: len(it.Items), Truncated: it.HasMore, SemanticScope: "CURRENT_SNAPSHOT"}
			out[i] = MaterializationResult{Request: req, Status: st, Readiness: "READY", Temporal: temporalFor(mustType("SKU_PROMOTION_CONTEXT"), rc), Truncation: tr, Data: map[string]any{"memberships": it.Items, "candidate_is_not_participant": true}, APICalls: 1}
		}
		return out
	}})
	r.Register(fnResolver{key: "SKU_PROMOTION_HISTORY", fn: func(ctx context.Context, rc factualapi.FactualReadContext, reqs []NormalizedEvidenceRequest) []MaterializationResult {
		out := make([]MaterializationResult, len(reqs))
		b := CurrentBudget().For("SKU_PROMOTION_HISTORY")
		for i, req := range reqs {
			id, err := parseID(req.Subject)
			if err != nil {
				out[i] = failResult(req, "INVALID_SUBJECT", err.Error(), "FAILED")
				continue
			}
			page, e := api.GetPromotionMembershipHistory(ctx, rc, id, factualapi.PromotionMembershipFilter{CanonicalSKUID: &id}, factualapi.PageRequest{PageSize: b.MaxObservations})
			if e != nil {
				out[i] = failResult(req, "FACTUAL_READ_FAILED", e.Error(), "FAILED")
				continue
			}
			items, tr := boundList(page.Items, b.MaxObservations, "known_from", "OBSERVATION_HISTORY", nil)
			st := StatusComplete
			if len(items) == 0 {
				st = StatusEmptyValid
			}
			out[i] = MaterializationResult{Request: req, Status: st, Readiness: "READY", Temporal: temporalFor(mustType("SKU_PROMOTION_HISTORY"), rc), Truncation: tr, Data: map[string]any{"intervals": items, "no_backfill_from_promotion_start": true}, APICalls: 1}
		}
		return out
	}})
	r.Register(fnResolver{key: "SKU_PROMOTION_ELIGIBILITY", fn: func(ctx context.Context, rc factualapi.FactualReadContext, reqs []NormalizedEvidenceRequest) []MaterializationResult {
		out := make([]MaterializationResult, len(reqs))
		for i, req := range reqs {
			id, err := parseID(req.Subject)
			if err != nil {
				out[i] = failResult(req, "INVALID_SUBJECT", err.Error(), "FAILED")
				continue
			}
			el, e := api.GetPromotionEligibility(ctx, rc, id, factualapi.PageRequest{PageSize: CurrentBudget().For(req.EvidenceType).MaxRows})
			if e != nil {
				out[i] = failResult(req, "FACTUAL_READ_FAILED", e.Error(), "FAILED")
				continue
			}
			out[i] = MaterializationResult{Request: req, Status: StatusComplete, Readiness: el.Meta.Readiness, Temporal: temporalFor(mustType("SKU_PROMOTION_ELIGIBILITY"), rc), Data: map[string]any{"eligibility": el, "candidate_is_not_participant": true}, APICalls: 1}
		}
		return out
	}})
	r.Register(fnResolver{key: "CATEGORY_PROMOTION_CONTEXT", fn: func(ctx context.Context, rc factualapi.FactualReadContext, reqs []NormalizedEvidenceRequest) []MaterializationResult {
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
		batch, err := api.BatchGetCategoryPromotionSummary(ctx, rc, ids)
		by := map[int64]factualapi.CategoryPromotionSummary{}
		if err == nil {
			for _, it := range batch.Items {
				if it.Summary != nil {
					by[it.ID] = *it.Summary
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
			sum, ok := by[id]
			if !ok {
				out[i] = emptyResult(req, temporalFor(mustType("CATEGORY_PROMOTION_CONTEXT"), rc), "no_category_promotion_summary")
				out[i].APICalls = 1
				continue
			}
			out[i] = MaterializationResult{Request: req, Status: StatusComplete, Readiness: sum.Meta.Readiness, Temporal: temporalFor(mustType("CATEGORY_PROMOTION_CONTEXT"), rc), Data: sum, APICalls: 1}
		}
		return out
	}})
	r.Register(fnResolver{key: "PROMOTION_DETAIL", fn: func(ctx context.Context, rc factualapi.FactualReadContext, reqs []NormalizedEvidenceRequest) []MaterializationResult {
		page, err := api.ListPromotions(ctx, rc, factualapi.PromotionFilter{}, factualapi.PageRequest{PageSize: CurrentBudget().For("PROMOTION_DETAIL").MaxRows})
		out := make([]MaterializationResult, len(reqs))
		for i, req := range reqs {
			if err != nil {
				out[i] = failResult(req, "FACTUAL_READ_FAILED", err.Error(), "FAILED")
				continue
			}
			items, tr := boundList(page.Items, CurrentBudget().For(req.EvidenceType).MaxRows, "observed_at", "CURRENT_SNAPSHOT", nil)
			st := StatusComplete
			if len(items) == 0 {
				st = StatusEmptyValid
			}
			out[i] = MaterializationResult{Request: req, Status: st, Readiness: "READY", Temporal: temporalFor(mustType("PROMOTION_DETAIL"), rc), Truncation: tr, Data: map[string]any{"promotions": items}, APICalls: 1}
		}
		return out
	}})
}
