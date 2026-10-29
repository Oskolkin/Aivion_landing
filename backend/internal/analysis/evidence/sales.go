package evidence

import (
	"context"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/evidencecatalog"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

type fnResolver struct {
	key string
	fn  func(context.Context, factualapi.FactualReadContext, []NormalizedEvidenceRequest) []MaterializationResult
}

func (f fnResolver) EvidenceType() string { return f.key }
func (f fnResolver) ResolveBatch(ctx context.Context, rc factualapi.FactualReadContext, reqs []NormalizedEvidenceRequest) []MaterializationResult {
	return f.fn(ctx, rc, reqs)
}

func registerSales(r *Registry, api Reads) {
	r.Register(fnResolver{key: "ACCOUNT_SALES_HISTORY", fn: func(ctx context.Context, rc factualapi.FactualReadContext, reqs []NormalizedEvidenceRequest) []MaterializationResult {
		out := make([]MaterializationResult, len(reqs))
		p := flowPeriod90(rc)
		pts, err := api.GetAccountDemandSeries(ctx, rc, factualapi.PeriodTrend30D, &p)
		for i, req := range reqs {
			out[i] = demandHistoryResult(req, rc, pts, err, 1)
		}
		return out
	}})
	r.Register(fnResolver{key: "CATEGORY_SALES_HISTORY", fn: func(ctx context.Context, rc factualapi.FactualReadContext, reqs []NormalizedEvidenceRequest) []MaterializationResult {
		out := make([]MaterializationResult, len(reqs))
		p := flowPeriod90(rc)
		for i, req := range reqs {
			id, err := parseID(req.Subject)
			if err != nil {
				out[i] = failResult(req, "INVALID_SUBJECT", err.Error(), "FAILED")
				continue
			}
			pts, e := api.GetCategoryDemandSeries(ctx, rc, id, factualapi.PeriodTrend30D, &p)
			res := demandHistoryResult(req, rc, pts, e, 1)
			res.Coverage.Unmapped = strPtr("UNMAPPED SKU-days excluded; historical SKU→category mapping required")
			out[i] = res
		}
		return out
	}})
	r.Register(fnResolver{key: "SKU_SALES_HISTORY", fn: func(ctx context.Context, rc factualapi.FactualReadContext, reqs []NormalizedEvidenceRequest) []MaterializationResult {
		ids := make([]int64, 0, len(reqs))
		idx := map[int64][]int{}
		out := make([]MaterializationResult, len(reqs))
		for i, req := range reqs {
			id, err := parseID(req.Subject)
			if err != nil {
				out[i] = failResult(req, "INVALID_SUBJECT", err.Error(), "FAILED")
				continue
			}
			ids = append(ids, id)
			idx[id] = append(idx[id], i)
		}
		p := flowPeriod90(rc)
		batch, err := api.BatchGetSKUDemandSeries(ctx, rc, ids, factualapi.PeriodTrend30D, &p)
		if err != nil {
			for i, req := range reqs {
				if out[i].Status == "" {
					out[i] = failResult(req, "FACTUAL_READ_FAILED", err.Error(), "FAILED")
				}
			}
			return out
		}
		byID := map[int64]factualapi.SKUDemandSeries{}
		for _, it := range batch.Items {
			if it.Series != nil {
				byID[it.ID] = *it.Series
			}
		}
		for i, req := range reqs {
			if out[i].Status != "" {
				continue
			}
			id, _ := parseID(req.Subject)
			ser := byID[id]
			out[i] = demandHistoryResult(req, rc, ser.Points, nil, 1)
		}
		return out
	}})
	r.Register(fnResolver{key: "ACCOUNT_SALES_CONTRIBUTORS", fn: func(ctx context.Context, rc factualapi.FactualReadContext, reqs []NormalizedEvidenceRequest) []MaterializationResult {
		b := CurrentBudget().For("ACCOUNT_SALES_CONTRIBUTORS")
		skus, e1 := api.ListSKUDemand(ctx, rc, factualapi.PeriodCurrent7D, nil, factualapi.PageRequest{PageSize: b.MaxRows})
		cats, e2 := api.ListCategoryDemand(ctx, rc, factualapi.PeriodCurrent7D, nil, factualapi.PageRequest{PageSize: b.MaxRows})
		out := make([]MaterializationResult, len(reqs))
		for i, req := range reqs {
			if e1 != nil || e2 != nil {
				msg := "sku/category list failed"
				if e1 != nil {
					msg = e1.Error()
				}
				out[i] = failResult(req, "FACTUAL_READ_FAILED", msg, "FAILED")
				continue
			}
			type row struct {
				Kind, ID, Role string
			}
			rows := []row{}
			for _, s := range skus.Items {
				rows = append(rows, row{Kind: "SKU", ID: itoa(s.CanonicalSKUID), Role: "CURRENT_EXPOSURE"})
			}
			for _, c := range cats.Items {
				if c.CategoryID != nil {
					rows = append(rows, row{Kind: "CATEGORY", ID: itoa(*c.CategoryID), Role: "CURRENT_EXPOSURE"})
				}
			}
			rows, tr := boundList(rows, b.MaxRelatedEntities, "provider_list_order", "bounded_union", nil)
			st := StatusComplete
			if len(rows) == 0 {
				st = StatusEmptyValid
			}
			if tr.Truncated {
				st = StatusPartial
			}
			out[i] = MaterializationResult{Request: req, Status: st, Readiness: "READY", Temporal: temporalFor(mustType("ACCOUNT_SALES_CONTRIBUTORS"), rc), Truncation: tr, Data: map[string]any{"contributors": rows, "roles": []string{"top_leaf_categories_current", "top_sku_current", "top_sku_increase", "top_sku_decrease"}}, APICalls: 2}
		}
		return out
	}})
	r.Register(fnResolver{key: "CATEGORY_SALES_CONTRIBUTORS", fn: func(ctx context.Context, rc factualapi.FactualReadContext, reqs []NormalizedEvidenceRequest) []MaterializationResult {
		b := CurrentBudget().For("CATEGORY_SALES_CONTRIBUTORS")
		skus, err := api.ListSKUDemand(ctx, rc, factualapi.PeriodCurrent7D, nil, factualapi.PageRequest{PageSize: b.MaxRows})
		out := make([]MaterializationResult, len(reqs))
		for i, req := range reqs {
			if err != nil {
				out[i] = failResult(req, "FACTUAL_READ_FAILED", err.Error(), "FAILED")
				continue
			}
			type row struct {
				SKU  string `json:"canonical_sku_id"`
				Role string `json:"role"`
			}
			rows := []row{}
			for _, s := range skus.Items {
				rows = append(rows, row{SKU: itoa(s.CanonicalSKUID), Role: "CURRENT_EXPOSURE"})
			}
			rows, tr := boundList(rows, b.MaxRelatedEntities, "ordered_amount", "bounded_sku_union", nil)
			st := StatusComplete
			if len(rows) == 0 {
				st = StatusEmptyValid
			}
			out[i] = MaterializationResult{Request: req, Status: st, Readiness: "READY", Temporal: temporalFor(mustType("CATEGORY_SALES_CONTRIBUTORS"), rc), Truncation: tr, Data: map[string]any{"skus": rows, "roles": []string{"CURRENT_EXPOSURE", "ABSOLUTE_INCREASE", "ABSOLUTE_DECREASE", "FROM_ZERO", "TO_ZERO"}}, APICalls: 1}
		}
		return out
	}})
}

func demandHistoryResult(req NormalizedEvidenceRequest, rc factualapi.FactualReadContext, pts []factualapi.DemandPoint, err error, calls int) MaterializationResult {
	t, _ := CatalogType(req.EvidenceType)
	temp := temporalFor(t, rc)
	if err != nil {
		return failResult(req, "FACTUAL_READ_FAILED", err.Error(), "FAILED")
	}
	dto, facts, ready := compressDemand(pts, rc.AnalysisDate())
	st := StatusComplete
	if len(dto.Daily) == 0 && len(dto.WeeklyContext) == 0 {
		st = StatusEmptyValid
	}
	if ready == "PARTIAL" || ready == "STALE" {
		st = StatusPartial
	}
	return MaterializationResult{Request: req, Status: st, Readiness: ready, Temporal: temp, Data: dto, Facts: facts, APICalls: calls}
}

func mustType(k string) evidencecatalog.Type {
	t, _ := CatalogType(k)
	return t
}

func strPtr(s string) *string { return &s }

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
