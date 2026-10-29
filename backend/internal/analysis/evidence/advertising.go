package evidence

import (
	"context"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

func registerAdvertising(r *Registry, api Reads) {
	r.Register(fnResolver{key: "CAMPAIGN_PERFORMANCE_HISTORY", fn: func(ctx context.Context, rc factualapi.FactualReadContext, reqs []NormalizedEvidenceRequest) []MaterializationResult {
		out := make([]MaterializationResult, len(reqs))
		p := flowPeriod90(rc)
		for i, req := range reqs {
			id, err := parseID(req.Subject)
			if err != nil {
				out[i] = failResult(req, "INVALID_SUBJECT", err.Error(), "FAILED")
				continue
			}
			pts, e := api.GetCampaignSeries(ctx, rc, id, factualapi.PeriodTrend30D, &p)
			if e != nil {
				out[i] = failResult(req, "FACTUAL_READ_FAILED", e.Error(), "FAILED")
				continue
			}
			facts := []DeclaredFact{}
			for _, d := range pts {
				if d.Meta.FactRef != "" {
					facts = append(facts, DeclaredFact{SemanticPath: "ads.campaign.day." + d.BusinessDate.Format("2006-01-02"), FactRefs: []string{d.Meta.FactRef}})
				}
			}
			st := StatusComplete
			if len(pts) == 0 {
				st = StatusEmptyValid
			}
			out[i] = MaterializationResult{Request: req, Status: st, Readiness: "READY", Temporal: temporalFor(mustType("CAMPAIGN_PERFORMANCE_HISTORY"), rc), Data: map[string]any{"daily": pts, "attributed_sales_are_not_canonical_demand": true}, Facts: facts, APICalls: 1}
		}
		return out
	}})
	r.Register(fnResolver{key: "CAMPAIGN_SKU_CONTRIBUTION", fn: func(ctx context.Context, rc factualapi.FactualReadContext, reqs []NormalizedEvidenceRequest) []MaterializationResult {
		out := make([]MaterializationResult, len(reqs))
		b := CurrentBudget().For("CAMPAIGN_SKU_CONTRIBUTION")
		p := eventPeriod30(rc)
		for i, req := range reqs {
			id, err := parseID(req.Subject)
			if err != nil {
				out[i] = failResult(req, "INVALID_SUBJECT", err.Error(), "FAILED")
				continue
			}
			page, e := api.ListCPOAttribution(ctx, rc, factualapi.CPOFilter{CampaignID: &id, Period: &p}, factualapi.PageRequest{PageSize: b.MaxRows})
			out[i] = attrResult(req, rc, e, page)
		}
		return out
	}})
	r.Register(fnResolver{key: "CAMPAIGN_TARGETING_CONTEXT", fn: func(ctx context.Context, rc factualapi.FactualReadContext, reqs []NormalizedEvidenceRequest) []MaterializationResult {
		ids := []int64{}
		out := make([]MaterializationResult, len(reqs))
		b := CurrentBudget().For("CAMPAIGN_TARGETING_CONTEXT")
		for i, req := range reqs {
			id, err := parseID(req.Subject)
			if err != nil {
				out[i] = failResult(req, "INVALID_SUBJECT", err.Error(), "FAILED")
				continue
			}
			ids = append(ids, id)
		}
		batch, err := api.BatchListCampaignTargetSKUs(ctx, rc, ids, b.MaxRows)
		by := map[int64]factualapi.BatchCampaignTargetSKUItem{}
		if err == nil {
			for _, it := range batch.Items {
				by[it.CampaignID] = it
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
			tr := TruncationMeta{Returned: len(it.Items), Truncated: it.HasMore, SortBasis: "assignment", SemanticScope: "CURRENT_SNAPSHOT"}
			out[i] = MaterializationResult{Request: req, Status: st, Readiness: "READY", Temporal: temporalFor(mustType("CAMPAIGN_TARGETING_CONTEXT"), rc), Truncation: tr, Data: map[string]any{"targets": it.Items, "new_sku_does_not_spawn_evidence_requests": true}, APICalls: 1}
		}
		return out
	}})
	r.Register(fnResolver{key: "SKU_ADVERTISING_HISTORY", fn: func(ctx context.Context, rc factualapi.FactualReadContext, reqs []NormalizedEvidenceRequest) []MaterializationResult {
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
		p := flowPeriod90(rc)
		batch, err := api.BatchGetSKUAdvertising(ctx, rc, ids, factualapi.PeriodTrend30D, &p)
		by := map[int64]factualapi.SKUAdvertising{}
		if err == nil {
			for _, it := range batch.Items {
				if it.Advertising != nil {
					by[it.ID] = *it.Advertising
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
			ad, ok := by[id]
			if !ok {
				out[i] = emptyResult(req, temporalFor(mustType("SKU_ADVERTISING_HISTORY"), rc), "no_sku_advertising")
				out[i].APICalls = 1
				continue
			}
			facts := []DeclaredFact{}
			if ad.Meta.FactRef != "" {
				facts = append(facts, DeclaredFact{SemanticPath: "ads.sku.period", FactRefs: []string{ad.Meta.FactRef}})
			}
			out[i] = MaterializationResult{Request: req, Status: StatusComplete, Readiness: ad.Meta.Readiness, Temporal: temporalFor(mustType("SKU_ADVERTISING_HISTORY"), rc), Data: map[string]any{"advertising": ad, "attributed_sales_are_not_canonical_demand": true}, Facts: facts, APICalls: 1}
		}
		return out
	}})
	r.Register(fnResolver{key: "SKU_CAMPAIGN_BREAKDOWN", fn: func(ctx context.Context, rc factualapi.FactualReadContext, reqs []NormalizedEvidenceRequest) []MaterializationResult {
		ids := []int64{}
		out := make([]MaterializationResult, len(reqs))
		b := CurrentBudget().For("SKU_CAMPAIGN_BREAKDOWN")
		for i, req := range reqs {
			id, err := parseID(req.Subject)
			if err != nil {
				out[i] = failResult(req, "INVALID_SUBJECT", err.Error(), "FAILED")
				continue
			}
			ids = append(ids, id)
		}
		p := flowPeriod90(rc)
		batch, err := api.BatchListSKUCampaignPerformance(ctx, rc, ids, factualapi.PeriodTrend30D, &p, b.MaxRows)
		by := map[int64]factualapi.BatchSKUCampaignPerformanceItem{}
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
			tr := TruncationMeta{Returned: len(it.Items), Truncated: it.HasMore, SortBasis: "paid_spend", SemanticScope: "FLOW_HISTORY_30D_90D"}
			out[i] = MaterializationResult{Request: req, Status: st, Readiness: "READY", Temporal: temporalFor(mustType("SKU_CAMPAIGN_BREAKDOWN"), rc), Truncation: tr, Data: it.Items, APICalls: 1}
		}
		return out
	}})
	r.Register(fnResolver{key: "AD_ATTRIBUTION_DETAIL", fn: func(ctx context.Context, rc factualapi.FactualReadContext, reqs []NormalizedEvidenceRequest) []MaterializationResult {
		out := make([]MaterializationResult, len(reqs))
		b := CurrentBudget().For("AD_ATTRIBUTION_DETAIL")
		p := eventPeriod30(rc)
		for i, req := range reqs {
			f := factualapi.CPOFilter{Period: &p}
			if req.Subject.Kind == "CAMPAIGN" {
				id, err := parseID(req.Subject)
				if err != nil {
					out[i] = failResult(req, "INVALID_SUBJECT", err.Error(), "FAILED")
					continue
				}
				f.CampaignID = &id
			}
			page, e := api.ListCPOAttribution(ctx, rc, f, factualapi.PageRequest{PageSize: b.MaxRows})
			out[i] = attrResult(req, rc, e, page)
		}
		return out
	}})
}

func attrResult(req NormalizedEvidenceRequest, rc factualapi.FactualReadContext, err error, page factualapi.Page[factualapi.CPOAttribution]) MaterializationResult {
	if err != nil {
		return failResult(req, "FACTUAL_READ_FAILED", err.Error(), "FAILED")
	}
	items, tr := boundList(page.Items, CurrentBudget().For(req.EvidenceType).MaxRows, "business_date", "AD_ATTRIBUTION_DETAIL", nil)
	if page.NextPageToken != "" {
		tr.Truncated = true
	}
	st := StatusComplete
	if len(items) == 0 {
		st = StatusEmptyValid
	}
	t, _ := CatalogType(req.EvidenceType)
	return MaterializationResult{Request: req, Status: st, Readiness: "READY", Temporal: temporalFor(t, rc), Truncation: tr, Data: map[string]any{"rows": items, "promoted_sku_vs_ordered_sku_preserved": true, "not_canonical_sales": true}, APICalls: 1}
}
