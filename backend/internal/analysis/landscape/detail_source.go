package landscape

import (
	"context"
	"errors"
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

// FactualAPIDetailSource loads bounded Detail supporting context via factualapi.API only.
type FactualAPIDetailSource struct {
	API factualapi.API
}

func (s FactualAPIDetailSource) LoadSKUDetailExtras(
	ctx context.Context,
	sealed factualapi.FactualReadContext,
	skuIDs []int64,
	profile ExecutableProfileV3,
) (map[int64]SKUDetailExtras, error) {
	if s.API == nil {
		return nil, fmt.Errorf("landscape: FactualAPIDetailSource requires factualapi.API")
	}
	out := make(map[int64]SKUDetailExtras, len(skuIDs))
	for _, id := range skuIDs {
		out[id] = SKUDetailExtras{}
	}
	if spec, decision, present, err := ProfileBlockDecision(sealed, profile, "sku.funnel"); err != nil {
		return nil, err
	} else if present && decision.Execute {
		if err := s.batchLoadSKUFunnel(ctx, sealed, skuIDs, spec, out); err != nil {
			return nil, err
		}
	}
	if spec, ok := profile.Supporting("sku.detail.campaign_context"); ok && spec.Limit > 0 {
		if _, decision, present, err := ProfileBlockDecision(sealed, profile, "sku.detail.campaign_context"); err != nil {
			return nil, err
		} else if present && decision.Execute {
			if err := s.batchLoadSKUCampaignContext(ctx, sealed, skuIDs, spec, out); err != nil {
				return nil, err
			}
		}
	}
	if spec, ok := profile.Supporting("sku.detail.query_context"); ok && spec.Limit > 0 {
		if _, decision, present, err := ProfileBlockDecision(sealed, profile, "sku.detail.query_context"); err != nil {
			return nil, err
		} else if present && decision.Execute {
			if err := s.batchLoadSKUQueryContext(ctx, sealed, skuIDs, spec, out); err != nil {
				return nil, err
			}
		}
	}
	if spec, ok := profile.Supporting("sku.detail.promotion_context"); ok && spec.Limit > 0 {
		if _, decision, present, err := ProfileBlockDecision(sealed, profile, "sku.detail.promotion_context"); err != nil {
			return nil, err
		} else if present && decision.Execute {
			if err := s.batchLoadSKUPromotionContext(ctx, sealed, skuIDs, spec.Limit, out); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}

func (s FactualAPIDetailSource) LoadCategoryDetailExtras(
	ctx context.Context,
	sealed factualapi.FactualReadContext,
	categoryIDs []int64,
	profile ExecutableProfileV3,
) (map[int64]CategoryDetailExtras, error) {
	if s.API == nil {
		return nil, fmt.Errorf("landscape: FactualAPIDetailSource requires factualapi.API")
	}
	out := make(map[int64]CategoryDetailExtras, len(categoryIDs))
	for _, id := range categoryIDs {
		out[id] = CategoryDetailExtras{}
	}
	for _, blockID := range []string{"category.funnel", "category.search", "category.promotions"} {
		spec, decision, present, err := ProfileBlockDecision(sealed, profile, blockID)
		if err != nil {
			return nil, err
		}
		if !present || !decision.Execute {
			continue
		}
		switch blockID {
		case "category.funnel":
			if err := s.loadCategoryFunnel(ctx, sealed, categoryIDs, spec, out); err != nil {
				return nil, err
			}
		case "category.search":
			if err := s.loadCategorySearch(ctx, sealed, categoryIDs, spec, out); err != nil {
				return nil, err
			}
		case "category.promotions":
			if err := s.loadCategoryPromotions(ctx, sealed, categoryIDs, out); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}

func (s FactualAPIDetailSource) LoadCampaignDetailExtras(
	ctx context.Context,
	sealed factualapi.FactualReadContext,
	campaignIDs []int64,
	profile ExecutableProfileV3,
) (map[int64]CampaignDetailExtras, error) {
	if s.API == nil {
		return nil, fmt.Errorf("landscape: FactualAPIDetailSource requires factualapi.API")
	}
	out := make(map[int64]CampaignDetailExtras, len(campaignIDs))
	spec, ok := profile.Supporting("campaign.detail.sku_context")
	if !ok || spec.Limit <= 0 {
		return out, nil
	}
	limit := spec.Limit
	sortBasis := nz(spec.SortBasis, "CANONICAL_SKU_ID_ASC")
	semanticScope := nz(spec.SemanticScope, "BOUNDED_EXPLICIT_ASSIGNMENTS")
	_, decision, present, err := ProfileBlockDecision(sealed, profile, "campaign.detail.sku_context")
	if err != nil {
		return nil, err
	}
	if !present {
		return out, nil
	}
	if !decision.Execute {
		return out, nil
	}
	batch, err := s.API.BatchListCampaignTargetSKUs(ctx, sealed, campaignIDs, limit)
	if err != nil {
		if isFactualUnavailableErr(err) {
			for _, id := range campaignIDs {
				out[id] = CampaignDetailExtras{
					SKUContext: unavailableSupportingContext[SKUAdsContextItem](sortBasis, semanticScope, factualUnavailableCode(err)),
				}
			}
			return out, nil
		}
		return nil, err
	}
	allSKU := make([]int64, 0)
	asOf := sealed.CutoffAt()
	for _, item := range batch.Items {
		if item.Status == factualapi.BatchNotFound {
			return nil, fmt.Errorf("landscape: selected campaign %d not found in sealed factual context", item.CampaignID)
		}
		items := make([]SKUAdsContextItem, 0, len(item.Items))
		for _, t := range item.Items {
			if t.CanonicalSKUID == nil {
				continue
			}
			items = append(items, SKUAdsContextItem{CanonicalSKUID: *t.CanonicalSKUID})
		}
		if len(items) > limit+1 {
			items = items[:limit+1]
		}
		total := int64(item.TotalCount)
		if total <= 0 {
			total = int64(len(items))
		}
		if item.HasMore && total <= int64(limit) {
			total = int64(limit) + 1
		}
		state := ApplyTruncation(items, total, limit, sortBasis, semanticScope)
		if item.Status == factualapi.BatchFoundEmpty || (len(items) == 0 && !item.HasMore) {
			state = ApplyTruncation([]SKUAdsContextItem(nil), 0, limit, sortBasis, semanticScope)
			total = 0
		}
		truncated := item.HasMore || state.Truncated || (total > int64(len(state.Items)))
		contextIDs := make([]int64, 0, len(state.Items))
		for _, it := range state.Items {
			contextIDs = append(contextIDs, it.CanonicalSKUID)
		}
		out[item.CampaignID] = CampaignDetailExtras{
			SKUContext:       supportingWithAsOf(state, "CAMPAIGN_TARGETS_AS_OF", &asOf),
			AssignedSKUs:     contextIDs,
			AssignedTotal:    total,
			ContextTruncated: truncated,
		}
		allSKU = append(allSKU, contextIDs...)
	}
	if profile.HasBlock("campaign.stock") && len(allSKU) > 0 {
		stocks, err := s.batchSKUStock(ctx, sealed, allSKU)
		if err != nil {
			return nil, err
		}
		for id, ex := range out {
			ex.Stocks = map[int64]factualapi.SKUStock{}
			for _, skuID := range ex.AssignedSKUs {
				if st, ok := stocks[skuID]; ok {
					ex.Stocks[skuID] = st
				}
			}
			out[id] = ex
		}
	}
	return out, nil
}

func (s FactualAPIDetailSource) batchSKUStock(ctx context.Context, sealed factualapi.FactualReadContext, ids []int64) (map[int64]factualapi.SKUStock, error) {
	src := FactualAPISource{API: s.API}
	return src.batchSKUStock(ctx, sealed, ids)
}

func isFactualUnavailableErr(err error) bool {
	var fe *factualapi.Error
	if !errors.As(err, &fe) {
		return false
	}
	return fe.Code == factualapi.CodePeriodUnsupported
}

func factualUnavailableCode(err error) string {
	var fe *factualapi.Error
	if errors.As(err, &fe) && fe.Code != "" {
		return string(fe.Code)
	}
	return "SUPPORTING_CONTEXT_UNAVAILABLE"
}

func factualUnavailableCodeFromDecision(d BlockExecutionDecision) string {
	if d.Limitation != nil && d.Limitation.Code != "" {
		return d.Limitation.Code
	}
	return "CAPABILITY_NOT_AVAILABLE"
}

func intAsDec(v int) *canonicaljson.Decimal {
	if v <= 0 {
		return nil
	}
	d := canonicaljson.Decimal(fmt.Sprintf("%d", v))
	return &d
}
