package landscape

import (
	"context"
	"fmt"
	"sort"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

func (s FactualAPIDetailSource) batchLoadSKUFunnel(
	ctx context.Context,
	sealed factualapi.FactualReadContext,
	skuIDs []int64,
	spec BlockSpecV3,
	out map[int64]SKUDetailExtras,
) error {
	curPeriod, err := periodFromBlock(sealed, spec, "CURRENT_7D")
	if err != nil {
		return err
	}
	prevPeriod, err := periodFromBlock(sealed, spec, "PREVIOUS_7D")
	if err != nil {
		return err
	}
	metrics := []string{
		"SEARCH_IMPRESSIONS", "SEARCH_CART_ADDS", "SEARCH_CART_CONVERSION",
		"PDP_IMPRESSIONS", "PDP_CART_ADDS", "PDP_CART_CONVERSION",
	}
	curMaps := make(map[string]map[int64]skuFunnelObs, len(metrics))
	prevMaps := make(map[string]map[int64]skuFunnelObs, len(metrics))
	for _, metric := range metrics {
		curMaps[metric], err = s.batchSKUFunnelMetric(ctx, sealed, skuIDs, metric, &curPeriod)
		if err != nil {
			return err
		}
		prevMaps[metric], err = s.batchSKUFunnelMetric(ctx, sealed, skuIDs, metric, &prevPeriod)
		if err != nil {
			return err
		}
	}
	for _, id := range skuIDs {
		ex := out[id]
		ex.Funnel = buildFunnelBlockFromSKUBatches(id, curMaps, prevMaps, periodToDateRange(curPeriod), periodToDateRange(prevPeriod))
		out[id] = ex
	}
	return nil
}

func (s FactualAPIDetailSource) batchSKUFunnelMetric(
	ctx context.Context,
	sealed factualapi.FactualReadContext,
	skuIDs []int64,
	metric string,
	period *factualapi.Period,
) (map[int64]skuFunnelObs, error) {
	out := map[int64]skuFunnelObs{}
	if len(skuIDs) == 0 {
		return out, nil
	}
	batch, err := s.API.BatchGetSKUFunnelPeriod(ctx, sealed, skuIDs, metric, "", period)
	if err != nil {
		return nil, err
	}
	for _, item := range batch.Items {
		if item.Status == factualapi.BatchNotFound {
			return nil, fmt.Errorf("landscape: selected sku %d not found in sealed factual context", item.ID)
		}
		obs := skuFunnelObs{Status: item.Status}
		if item.Funnel != nil {
			row := *item.Funnel
			obs.Row = &row
		}
		if item.Limitation != nil {
			obs.Limitation = item.Limitation
		}
		out[item.ID] = obs
	}
	return out, nil
}

func (s FactualAPIDetailSource) loadCategoryFunnel(
	ctx context.Context,
	sealed factualapi.FactualReadContext,
	categoryIDs []int64,
	spec BlockSpecV3,
	out map[int64]CategoryDetailExtras,
) error {
	ids := mappedCategoryIDs(categoryIDs)
	curPeriod, err := periodFromBlock(sealed, spec, "CURRENT_7D")
	if err != nil {
		return err
	}
	prevPeriod, err := periodFromBlock(sealed, spec, "PREVIOUS_7D")
	if err != nil {
		return err
	}
	metrics := []string{
		"SEARCH_IMPRESSIONS", "SEARCH_CART_ADDS", "SEARCH_CART_CONVERSION",
		"PDP_IMPRESSIONS", "PDP_CART_ADDS", "PDP_CART_CONVERSION",
	}
	curMaps := make(map[string]map[int64]categoryFunnelObs, len(metrics))
	prevMaps := make(map[string]map[int64]categoryFunnelObs, len(metrics))
	for _, metric := range metrics {
		curMaps[metric], err = s.batchCategoryFunnelMetric(ctx, sealed, ids, metric, &curPeriod)
		if err != nil {
			return err
		}
		prevMaps[metric], err = s.batchCategoryFunnelMetric(ctx, sealed, ids, metric, &prevPeriod)
		if err != nil {
			return err
		}
	}
	for _, id := range ids {
		ex := out[id]
		ex.Funnel = buildFunnelBlockFromCategoryBatches(id, curMaps, prevMaps, periodToDateRange(curPeriod), periodToDateRange(prevPeriod))
		out[id] = ex
	}
	return nil
}

func (s FactualAPIDetailSource) batchCategoryFunnelMetric(
	ctx context.Context,
	sealed factualapi.FactualReadContext,
	ids []int64,
	metric string,
	period *factualapi.Period,
) (map[int64]categoryFunnelObs, error) {
	out := map[int64]categoryFunnelObs{}
	if len(ids) == 0 {
		return out, nil
	}
	batch, err := s.API.BatchGetCategoryFunnelPeriod(ctx, sealed, ids, metric, "", period)
	if err != nil {
		if isFactualUnavailableErr(err) {
			return out, nil
		}
		return nil, err
	}
	for _, item := range batch.Items {
		if item.Status == factualapi.BatchNotFound {
			return nil, fmt.Errorf("landscape: selected category %d not found in sealed factual context", item.ID)
		}
		obs := categoryFunnelObs{Status: item.Status}
		if item.Funnel != nil {
			row := *item.Funnel
			obs.Row = &row
		}
		if item.Limitation != nil {
			obs.Limitation = item.Limitation
		}
		out[item.ID] = obs
	}
	return out, nil
}

func (s FactualAPIDetailSource) loadCategorySearch(
	ctx context.Context,
	sealed factualapi.FactualReadContext,
	categoryIDs []int64,
	spec BlockSpecV3,
	out map[int64]CategoryDetailExtras,
) error {
	ids := mappedCategoryIDs(categoryIDs)
	curPeriod, err := periodFromBlock(sealed, spec, "CURRENT_7D")
	if err != nil {
		return err
	}
	prevPeriod, err := periodFromBlock(sealed, spec, "PREVIOUS_7D")
	if err != nil {
		return err
	}
	cur, err := s.batchCategorySearchDistribution(ctx, sealed, ids, &curPeriod)
	if err != nil {
		return err
	}
	prev, err := s.batchCategorySearchDistribution(ctx, sealed, ids, &prevPeriod)
	if err != nil {
		return err
	}
	for _, id := range ids {
		ex := out[id]
		ex.Search = buildCategorySearchBlock(id, cur, prev, periodToDateRange(curPeriod), periodToDateRange(prevPeriod))
		out[id] = ex
	}
	return nil
}

func (s FactualAPIDetailSource) batchCategorySearchDistribution(
	ctx context.Context,
	sealed factualapi.FactualReadContext,
	ids []int64,
	period *factualapi.Period,
) (map[int64]factualapi.CategorySearchDistribution, error) {
	out := map[int64]factualapi.CategorySearchDistribution{}
	if len(ids) == 0 {
		return out, nil
	}
	batch, err := s.API.BatchGetCategorySearchDistribution(ctx, sealed, ids, "", period)
	if err != nil {
		if isFactualUnavailableErr(err) {
			return out, nil
		}
		return nil, err
	}
	for _, item := range batch.Items {
		if item.Status == factualapi.BatchNotFound {
			return nil, fmt.Errorf("landscape: selected category %d not found in sealed factual context", item.ID)
		}
		if item.Status == factualapi.BatchFound && item.Distribution != nil {
			out[item.ID] = *item.Distribution
		}
	}
	return out, nil
}

func buildCategorySearchBlock(
	id int64,
	cur, prev map[int64]factualapi.CategorySearchDistribution,
	curPeriod, prevPeriod *DateRange,
) *CategorySearchBlock {
	c, hasCur := cur[id]
	p, hasPrev := prev[id]
	if !hasCur && !hasPrev {
		return &CategorySearchBlock{State: BlockReadyEmpty, CurrentPeriod: curPeriod, PreviousPeriod: prevPeriod}
	}
	block := &CategorySearchBlock{State: BlockReady, CurrentPeriod: curPeriod, PreviousPeriod: prevPeriod}
	src := c
	if !hasCur {
		src = p
	}
	block.SKUWithSearchSummaryCount = intFromValue(src.SKUWithSearchSummaryCount)
	block.PositionP25 = decFromValue(src.PositionP25)
	block.PositionP50 = decFromValue(src.PositionP50)
	block.PositionP75 = decFromValue(src.PositionP75)
	block.Coverage = decFromValue(src.Coverage)
	block.MappingExcludedSKUCount = intFromValue(src.MappingExcludedSKUCount)
	return block
}

func (s FactualAPIDetailSource) loadCategoryPromotions(
	ctx context.Context,
	sealed factualapi.FactualReadContext,
	categoryIDs []int64,
	out map[int64]CategoryDetailExtras,
) error {
	ids := mappedCategoryIDs(categoryIDs)
	if len(ids) == 0 {
		return nil
	}
	batch, err := s.API.BatchGetCategoryPromotionSummary(ctx, sealed, ids)
	if err != nil {
		if isFactualUnavailableErr(err) {
			for _, id := range ids {
				ex := out[id]
				ex.Promotions = &CategoryPromotionsBlock{State: BlockUnavailable, Limitations: unavailableBlock(factualUnavailableCode(err), err.Error())}
				out[id] = ex
			}
			return nil
		}
		return err
	}
	for _, item := range batch.Items {
		ex := out[item.ID]
		if item.Status == factualapi.BatchNotFound {
			return fmt.Errorf("landscape: selected category %d not found in sealed factual context", item.ID)
		}
		if item.Status == factualapi.BatchFoundEmpty || item.Summary == nil {
			ex.Promotions = &CategoryPromotionsBlock{State: BlockReadyEmpty}
			out[item.ID] = ex
			continue
		}
		ex.Promotions = &CategoryPromotionsBlock{
			State:                 BlockReady,
			ActivePromotionCount:  intFromValue(item.Summary.ActivePromotionCount),
			ParticipatingSKUCount: intFromValue(item.Summary.ParticipatingSKUCount),
			CandidateSKUCount:     intFromValue(item.Summary.CandidateSKUCount),
		}
		out[item.ID] = ex
	}
	return nil
}

func mappedCategoryIDs(ids []int64) []int64 {
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id == UnmappedCategoryID {
			continue
		}
		out = append(out, id)
	}
	return out
}

type skuFunnelObs struct {
	Status     factualapi.BatchStatus
	Row        *factualapi.SKUFunnelPeriod
	Limitation *factualapi.Limitation
}

type categoryFunnelObs struct {
	Status     factualapi.BatchStatus
	Row        *factualapi.CategoryFunnelPeriod
	Limitation *factualapi.Limitation
}

func buildFunnelBlockFromSKUBatches(
	id int64,
	cur, prev map[string]map[int64]skuFunnelObs,
	curPeriod, prevPeriod *DateRange,
) *SKUFunnelBlock {
	metrics := []string{
		"SEARCH_IMPRESSIONS", "SEARCH_CART_ADDS", "SEARCH_CART_CONVERSION",
		"PDP_IMPRESSIONS", "PDP_CART_ADDS", "PDP_CART_CONVERSION",
	}
	var st FunnelBuildState
	for _, metric := range metrics {
		observeSKUFunnel(id, cur, metric, &st)
		observeSKUFunnel(id, prev, metric, &st)
	}
	search := buildFunnelSurfaceFromSKU(id, cur, prev, "SEARCH_IMPRESSIONS", "SEARCH_CART_ADDS", "SEARCH_CART_CONVERSION")
	pdp := buildFunnelSurfaceFromSKU(id, cur, prev, "PDP_IMPRESSIONS", "PDP_CART_ADDS", "PDP_CART_CONVERSION")
	state, lims := finalizeFunnelState(st)
	return &SKUFunnelBlock{
		State: state, CurrentPeriod: curPeriod, PreviousPeriod: prevPeriod,
		SearchSurface: search, PDPSurface: pdp, Limitations: lims,
	}
}

func buildFunnelBlockFromCategoryBatches(
	id int64,
	cur, prev map[string]map[int64]categoryFunnelObs,
	curPeriod, prevPeriod *DateRange,
) *CategoryFunnelBlock {
	metrics := []string{
		"SEARCH_IMPRESSIONS", "SEARCH_CART_ADDS", "SEARCH_CART_CONVERSION",
		"PDP_IMPRESSIONS", "PDP_CART_ADDS", "PDP_CART_CONVERSION",
	}
	var st FunnelBuildState
	for _, metric := range metrics {
		observeCategoryFunnel(id, cur, metric, &st)
		observeCategoryFunnel(id, prev, metric, &st)
	}
	search := buildFunnelSurfaceFromCategory(id, cur, prev, "SEARCH_IMPRESSIONS", "SEARCH_CART_ADDS", "SEARCH_CART_CONVERSION")
	pdp := buildFunnelSurfaceFromCategory(id, cur, prev, "PDP_IMPRESSIONS", "PDP_CART_ADDS", "PDP_CART_CONVERSION")
	state, lims := finalizeFunnelState(st)
	return &CategoryFunnelBlock{
		State: state, CurrentPeriod: curPeriod, PreviousPeriod: prevPeriod,
		SearchSurface: search, PDPSurface: pdp, Limitations: lims,
	}
}

func observeSKUFunnel(id int64, m map[string]map[int64]skuFunnelObs, metric string, st *FunnelBuildState) {
	obs, ok := m[metric][id]
	if !ok {
		st.MissingMetricCount++
		return
	}
	known := false
	if obs.Row != nil {
		known = obs.Row.Metric.Value.State == factualapi.ValueKnown && obs.Row.Metric.Value.Value != nil
	}
	observeFunnelStatus(st, obs.Status, known, limCode(obs.Limitation), limMsg(obs.Limitation))
}

func observeCategoryFunnel(id int64, m map[string]map[int64]categoryFunnelObs, metric string, st *FunnelBuildState) {
	obs, ok := m[metric][id]
	if !ok {
		st.MissingMetricCount++
		return
	}
	known := false
	if obs.Row != nil {
		known = obs.Row.Metric.Value.State == factualapi.ValueKnown && obs.Row.Metric.Value.Value != nil
	}
	observeFunnelStatus(st, obs.Status, known, limCode(obs.Limitation), limMsg(obs.Limitation))
}

func limCode(lim *factualapi.Limitation) string {
	if lim == nil {
		return ""
	}
	return lim.Code
}

func limMsg(lim *factualapi.Limitation) string {
	if lim == nil {
		return ""
	}
	return lim.Message
}

func buildFunnelSurfaceFromSKU(
	id int64,
	cur, prev map[string]map[int64]skuFunnelObs,
	impressionsKey, cartAddsKey, conversionKey string,
) *FunnelSurfaceBlock {
	impCur, impPrev := funnelCountFromSKU(cur, prev, id, impressionsKey)
	cartCur, cartPrev := funnelCountFromSKU(cur, prev, id, cartAddsKey)
	convCur, convPrev := funnelPctFromSKU(cur, prev, id, conversionKey)
	if impCur == nil && impPrev == nil && cartCur == nil && cartPrev == nil && convCur == nil && convPrev == nil {
		return nil
	}
	surface := &FunnelSurfaceBlock{}
	if impCur != nil || impPrev != nil {
		cmp := BuildCountComparison(impCur, impPrev)
		surface.Impressions = &cmp
	}
	if cartCur != nil || cartPrev != nil {
		cmp := BuildCountComparison(cartCur, cartPrev)
		surface.CartAdds = &cmp
	}
	if convCur != nil || convPrev != nil {
		cmp := BuildPctPointComparison(convCur, convPrev)
		surface.Conversion = &cmp
	}
	return surface
}

func buildFunnelSurfaceFromCategory(
	id int64,
	cur, prev map[string]map[int64]categoryFunnelObs,
	impressionsKey, cartAddsKey, conversionKey string,
) *FunnelSurfaceBlock {
	impCur, impPrev := funnelCountFromCategory(cur, prev, id, impressionsKey)
	cartCur, cartPrev := funnelCountFromCategory(cur, prev, id, cartAddsKey)
	convCur, convPrev := funnelPctFromCategory(cur, prev, id, conversionKey)
	if impCur == nil && impPrev == nil && cartCur == nil && cartPrev == nil && convCur == nil && convPrev == nil {
		return nil
	}
	surface := &FunnelSurfaceBlock{}
	if impCur != nil || impPrev != nil {
		cmp := BuildCountComparison(impCur, impPrev)
		surface.Impressions = &cmp
	}
	if cartCur != nil || cartPrev != nil {
		cmp := BuildCountComparison(cartCur, cartPrev)
		surface.CartAdds = &cmp
	}
	if convCur != nil || convPrev != nil {
		cmp := BuildPctPointComparison(convCur, convPrev)
		surface.Conversion = &cmp
	}
	return surface
}

func funnelCountFromSKU(cur, prev map[string]map[int64]skuFunnelObs, id int64, metric string) (*int64, *int64) {
	return funnelCountFromMetric(skuFunnelValue(cur, id, metric), skuFunnelValue(prev, id, metric))
}

func funnelPctFromSKU(cur, prev map[string]map[int64]skuFunnelObs, id int64, metric string) (*canonicaljson.Decimal, *canonicaljson.Decimal) {
	return funnelPctFromMetric(skuFunnelValue(cur, id, metric), skuFunnelValue(prev, id, metric))
}

func funnelCountFromCategory(cur, prev map[string]map[int64]categoryFunnelObs, id int64, metric string) (*int64, *int64) {
	return funnelCountFromMetric(categoryFunnelValue(cur, id, metric), categoryFunnelValue(prev, id, metric))
}

func funnelPctFromCategory(cur, prev map[string]map[int64]categoryFunnelObs, id int64, metric string) (*canonicaljson.Decimal, *canonicaljson.Decimal) {
	return funnelPctFromMetric(categoryFunnelValue(cur, id, metric), categoryFunnelValue(prev, id, metric))
}

func skuFunnelValue(m map[string]map[int64]skuFunnelObs, id int64, metric string) factualapi.Value[factualapi.Decimal] {
	if mm, ok := m[metric]; ok {
		if row, ok := mm[id]; ok && row.Row != nil {
			return row.Row.Metric.Value
		}
	}
	return factualapi.Value[factualapi.Decimal]{}
}

func categoryFunnelValue(m map[string]map[int64]categoryFunnelObs, id int64, metric string) factualapi.Value[factualapi.Decimal] {
	if mm, ok := m[metric]; ok {
		if row, ok := mm[id]; ok && row.Row != nil {
			return row.Row.Metric.Value
		}
	}
	return factualapi.Value[factualapi.Decimal]{}
}

func funnelCountFromMetric(cur, prev factualapi.Value[factualapi.Decimal]) (*int64, *int64) {
	return decIntFromValue(cur), decIntFromValue(prev)
}

func funnelPctFromMetric(cur, prev factualapi.Value[factualapi.Decimal]) (*canonicaljson.Decimal, *canonicaljson.Decimal) {
	return decFromValue(cur), decFromValue(prev)
}

func (s FactualAPIDetailSource) batchLoadSKUCampaignContext(
	ctx context.Context,
	sealed factualapi.FactualReadContext,
	skuIDs []int64,
	spec SupportingSpec,
	out map[int64]SKUDetailExtras,
) error {
	sortBasis := nz(spec.SortBasis, "PAID_SPEND_DESC")
	semanticScope := nz(spec.SemanticScope, "TOP_N_BY_PAID_SPEND")
	periods, err := ResolveBlockPeriodsV3(sealed, BlockSpecV3{
		BlockID: "sku.detail.campaign_context",
		Periods: []PeriodSpec{spec.Period},
	})
	if err != nil {
		return err
	}
	period := periods["CURRENT_7D"]
	batch, err := s.API.BatchListSKUCampaignPerformance(ctx, sealed, skuIDs, "", &period, spec.Limit)
	if err != nil {
		if isFactualUnavailableErr(err) {
			code := factualUnavailableCode(err)
			for _, id := range skuIDs {
				ex := out[id]
				ex.CampaignContext = unavailableSupportingContext[CampaignContextItem](sortBasis, semanticScope, code)
				out[id] = ex
			}
			return nil
		}
		return err
	}
	for _, item := range batch.Items {
		ex := out[item.SKUID]
		if item.Status == factualapi.BatchNotFound {
			return fmt.Errorf("landscape: selected sku %d not found in sealed factual context", item.SKUID)
		}
		if item.Status == factualapi.BatchFoundEmpty {
			ex.CampaignContext = emptySupportingContext[CampaignContextItem](sortBasis, semanticScope)
			sc := supportingWithPeriod(ApplyTruncation([]CampaignContextItem(nil), 0, spec.Limit, sortBasis, semanticScope), "ADVERTISING_TRAFFIC_CURRENT", periodToDateRange(period))
			ex.CampaignContext = sc
			out[item.SKUID] = ex
			continue
		}
		items := make([]CampaignContextItem, 0, len(item.Items))
		for _, p := range item.Items {
			items = append(items, CampaignContextItem{
				CampaignID: p.CampaignID,
				PaidSpend:  moneyFromValue(p.PaidSpend),
			})
		}
		if len(items) > spec.Limit+1 {
			items = items[:spec.Limit+1]
		}
		total := int64(len(items))
		if item.HasMore {
			total = int64(spec.Limit) + 1
		}
		ex.CampaignContext = supportingWithPeriod(ApplyTruncation(items, total, spec.Limit, sortBasis, semanticScope), "ADVERTISING_TRAFFIC_CURRENT", periodToDateRange(period))
		out[item.SKUID] = ex
	}
	return nil
}

func (s FactualAPIDetailSource) batchLoadSKUQueryContext(
	ctx context.Context,
	sealed factualapi.FactualReadContext,
	skuIDs []int64,
	spec SupportingSpec,
	out map[int64]SKUDetailExtras,
) error {
	sortBasis := nz(spec.SortBasis, "PROVIDER_SEARCH_RANK")
	semanticScope := nz(spec.SemanticScope, "TOP_N_PROVIDER_RANKED")
	pair, err := ResolveFactFamilyPeriodPair(sealed, "SEARCH", "SEARCH.QUERY_DETAILS")
	if err != nil {
		return err
	}
	period := periodToDateRange(pair.Current)
	batch, err := s.API.BatchGetSKUQueryDetails(ctx, sealed, skuIDs, "", &pair.Current, spec.Limit)
	if err != nil {
		if isFactualUnavailableErr(err) {
			code := factualUnavailableCode(err)
			for _, id := range skuIDs {
				ex := out[id]
				ex.QueryContext = unavailableSupportingContext[SearchQueryContextItem](sortBasis, semanticScope, code)
				out[id] = ex
			}
			return nil
		}
		return err
	}
	for _, item := range batch.Items {
		ex := out[item.SKUID]
		if item.Status == factualapi.BatchNotFound {
			return fmt.Errorf("landscape: selected sku %d not found in sealed factual context", item.SKUID)
		}
		items := make([]SearchQueryContextItem, 0, len(item.Items))
		for _, q := range item.Items {
			items = append(items, SearchQueryContextItem{
				QueryID:          q.SearchQueryID,
				ProviderPosition: intAsDec(q.ProviderRank),
			})
		}
		if len(items) > spec.Limit+1 {
			items = items[:spec.Limit+1]
		}
		total := int64(len(items))
		if item.HasMore {
			total = int64(spec.Limit) + 1
		}
		if item.Status == factualapi.BatchFoundEmpty || (len(items) == 0 && !item.HasMore) {
			sc := supportingWithPeriod(ApplyTruncation([]SearchQueryContextItem(nil), 0, spec.Limit, sortBasis, semanticScope), "SEARCH_QUERY_DETAILS_CURRENT", period)
			ex.QueryContext = sc
			out[item.SKUID] = ex
			continue
		}
		sc := supportingWithPeriod(ApplyTruncation(items, total, spec.Limit, sortBasis, semanticScope), "SEARCH_QUERY_DETAILS_CURRENT", period)
		ex.QueryContext = sc
		out[item.SKUID] = ex
	}
	return nil
}

func (s FactualAPIDetailSource) batchLoadSKUPromotionContext(
	ctx context.Context,
	sealed factualapi.FactualReadContext,
	skuIDs []int64,
	limit int,
	out map[int64]SKUDetailExtras,
) error {
	sortBasis := "PROMOTION_ID_ASC"
	semanticScope := "TOP_N_ACTIVE_OR_CANDIDATE"
	asOf := sealed.CutoffAt()
	batch, err := s.API.BatchGetSKUPromotions(ctx, sealed, skuIDs, limit)
	if err != nil {
		if isFactualUnavailableErr(err) {
			code := factualUnavailableCode(err)
			for _, id := range skuIDs {
				ex := out[id]
				ex.PromotionContext = unavailableSupportingContext[PromotionContextItem](sortBasis, semanticScope, code)
				out[id] = ex
			}
			return nil
		}
		return err
	}
	for _, item := range batch.Items {
		ex := out[item.SKUID]
		if item.Status == factualapi.BatchNotFound {
			return fmt.Errorf("landscape: selected sku %d not found in sealed factual context", item.SKUID)
		}
		items := make([]PromotionContextItem, 0, len(item.Items))
		for _, p := range item.Items {
			items = append(items, PromotionContextItem{
				PromotionID: p.ID,
				Origin:      p.Origin,
				Mechanism:   p.DiscountType,
				Membership:  "MEMBER",
			})
		}
		sort.Slice(items, func(i, j int) bool { return items[i].PromotionID < items[j].PromotionID })
		if len(items) > limit+1 {
			items = items[:limit+1]
		}
		total := int64(len(items))
		if item.HasMore {
			total = int64(limit) + 1
		}
		ex.PromotionContext = supportingWithAsOf(ApplyTruncation(items, total, limit, sortBasis, semanticScope), "PROMOTIONS_AS_OF", &asOf)
		out[item.SKUID] = ex
	}
	return nil
}
