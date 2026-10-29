package discovery

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/landscape"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

type GroundingCache struct {
	AccountDemandCur, AccountDemandPrev, AccountDemandDay, AccountDemandPrevDay factualapi.Demand
	AccountFinanceCur, AccountFinancePrev                                       factualapi.FinanceAccountSummary

	SKUDemandCur, SKUDemandPrev map[int64]factualapi.SKUDemand
	SKUStock                    map[int64]factualapi.SKUStock
	SKUCoverage                 map[int64]factualapi.StockCoverage
	SKUPrice                    map[int64]factualapi.SKUPrice
	SKUPriceChange              map[int64]factualapi.SKUPriceLastObservedChange
	SKUInbound                  map[int64]factualapi.SKUInbound
	SKUAdsTrafficCur            map[int64]factualapi.SKUAdvertisingTraffic
	SKUAdsTrafficPrev           map[int64]factualapi.SKUAdvertisingTraffic
	SKUAdsAttrCur               map[int64]factualapi.SKUAdvertisingAttribution
	SKUAdsAttrPrev              map[int64]factualapi.SKUAdvertisingAttribution
	SKUFunnelSearchCur          map[int64]factualapi.SKUFunnelPeriod
	SKUFunnelSearchPrev         map[int64]factualapi.SKUFunnelPeriod
	SKUSearchCur                map[int64]factualapi.SKUSearchSummary
	SKUSearchPrev               map[int64]factualapi.SKUSearchSummary

	CatDemandCur  map[int64]factualapi.CategoryDemand
	CatDemandPrev map[int64]factualapi.CategoryDemand

	CampTrafficCur, CampTrafficPrev map[int64]factualapi.CampaignTrafficPerformance
	CampAttrCur, CampAttrPrev       map[int64]factualapi.CampaignAttributionPerformance

	APICallCount int
}

type packGroundingAPI interface {
	GetAccountDemand(context.Context, factualapi.FactualReadContext, factualapi.StandardPeriod, *factualapi.Period) (factualapi.Demand, error)
	GetFinanceAccountSummary(context.Context, factualapi.FactualReadContext, factualapi.StandardPeriod, *factualapi.Period) (factualapi.FinanceAccountSummary, error)
	BatchGetSKUDemand(context.Context, factualapi.FactualReadContext, []int64, factualapi.StandardPeriod, *factualapi.Period) (factualapi.BatchSKUDemandResult, error)
	BatchGetSKUStock(context.Context, factualapi.FactualReadContext, []int64) (factualapi.BatchSKUStockResult, error)
	BatchGetStockCoverage(context.Context, factualapi.FactualReadContext, []int64) (factualapi.BatchStockCoverageResult, error)
	BatchGetSKUPrice(context.Context, factualapi.FactualReadContext, []int64) (factualapi.BatchSKUPriceResult, error)
	BatchGetSKUPriceLastObservedChange(context.Context, factualapi.FactualReadContext, []int64) (factualapi.BatchSKUPriceLastObservedChangeResult, error)
	BatchGetSKUInbound(context.Context, factualapi.FactualReadContext, []int64) (factualapi.BatchSKUInboundResult, error)
	BatchGetSKUAdvertisingTraffic(context.Context, factualapi.FactualReadContext, []int64, factualapi.StandardPeriod, *factualapi.Period) (factualapi.BatchSKUAdvertisingTrafficResult, error)
	BatchGetSKUAdvertisingAttribution(context.Context, factualapi.FactualReadContext, []int64, factualapi.StandardPeriod, *factualapi.Period) (factualapi.BatchSKUAdvertisingAttributionResult, error)
	BatchGetSKUFunnelPeriod(context.Context, factualapi.FactualReadContext, []int64, string, factualapi.StandardPeriod, *factualapi.Period) (factualapi.BatchSKUFunnelPeriodResult, error)
	BatchGetSKUSearchSummary(context.Context, factualapi.FactualReadContext, []int64, factualapi.StandardPeriod, *factualapi.Period) (factualapi.BatchSKUSearchSummaryResult, error)
	BatchGetCampaignTrafficPerformance(context.Context, factualapi.FactualReadContext, []int64, factualapi.StandardPeriod, *factualapi.Period) (factualapi.BatchCampaignTrafficResult, error)
	BatchGetCampaignAttributionPerformance(context.Context, factualapi.FactualReadContext, []int64, factualapi.StandardPeriod, *factualapi.Period) (factualapi.BatchCampaignAttributionResult, error)
	ListCategoryDemand(context.Context, factualapi.FactualReadContext, factualapi.StandardPeriod, *factualapi.Period, factualapi.PageRequest) (factualapi.Page[factualapi.CategoryDemand], error)
}

type groundingPlan struct {
	AccountDemand, AccountDemandDay, AccountFinance                        bool
	SKUDemand, SKUStock, SKUCoverage, SKUPrice, SKUPriceChange, SKUInbound bool
	SKUAdsTraffic, SKUAdsAttr, SKUFunnel, SKUSearch                        bool
	CatDemand, CampTraffic, CampAttr                                       bool
}

func (p groundingPlan) requiresAPI() bool {
	return p.AccountDemand || p.AccountDemandDay || p.AccountFinance ||
		p.SKUDemand || p.SKUStock || p.SKUCoverage || p.SKUPrice || p.SKUPriceChange || p.SKUInbound ||
		p.SKUAdsTraffic || p.SKUAdsAttr || p.SKUFunnel || p.SKUSearch ||
		p.CatDemand || p.CampTraffic || p.CampAttr
}

func planGrounding(detail landscape.LandscapeSet) groundingPlan {
	var p groundingPlan
	if detail.Account.Sales != nil && usableBlock(detail.Account.Sales.State) {
		if moneyPresent(detail.Account.Sales.OrderedAmount) || countPresent(detail.Account.Sales.OrderedUnits) {
			p.AccountDemand = true
		}
		if detail.Account.Sales.AnalysisDayAmount != nil && moneyPresent(*detail.Account.Sales.AnalysisDayAmount) {
			p.AccountDemandDay = true
		}
		if detail.Account.Sales.AnalysisDayUnits != nil && countPresent(*detail.Account.Sales.AnalysisDayUnits) {
			p.AccountDemandDay = true
		}
	}
	if detail.Account.Finance != nil && usableBlock(detail.Account.Finance.State) && moneyPresent(detail.Account.Finance.NetAccrual) {
		p.AccountFinance = true
	}
	for _, cat := range detail.Categories {
		if cat.Sales != nil && usableBlock(cat.Sales.State) && (moneyPresent(cat.Sales.OrderedAmount) || countPresent(cat.Sales.OrderedUnits)) {
			p.CatDemand = true
			break
		}
	}
	for _, camp := range detail.Campaigns {
		if camp.Performance == nil {
			continue
		}
		if camp.Performance.Traffic != nil && usableBlock(camp.Performance.Traffic.State) && moneyPresent(camp.Performance.Traffic.PaidSpend) {
			p.CampTraffic = true
		}
		if camp.Performance.Attribution != nil && usableBlock(camp.Performance.Attribution.State) && moneyPresent(camp.Performance.Attribution.AttributedOrderAmount) {
			p.CampAttr = true
		}
	}
	for _, sku := range detail.SKUs {
		if sku.Sales != nil && usableBlock(sku.Sales.State) && (moneyPresent(sku.Sales.OrderedAmount) || countPresent(sku.Sales.OrderedUnits)) {
			p.SKUDemand = true
		}
		if sku.Stock != nil && usableBlock(sku.Stock.State) {
			p.SKUStock = true
			if sku.Stock.CoverageDays7D != nil {
				p.SKUCoverage = true
			}
		}
		if sku.Price != nil && usableBlock(sku.Price.State) {
			if sku.Price.CurrentPrice != nil {
				p.SKUPrice = true
			}
			if sku.Price.LastObservedChange != nil && moneyPresent(*sku.Price.LastObservedChange) {
				p.SKUPriceChange = true
			}
		}
		if sku.Advertising != nil {
			if sku.Advertising.Traffic != nil && usableBlock(sku.Advertising.Traffic.State) && moneyPresent(sku.Advertising.Traffic.PaidSpend) {
				p.SKUAdsTraffic = true
			}
			if sku.Advertising.Attribution != nil && usableBlock(sku.Advertising.Attribution.State) && moneyPresent(sku.Advertising.Attribution.AttributedOrderAmount) {
				p.SKUAdsAttr = true
			}
		}
		if sku.Funnel != nil && usableBlock(sku.Funnel.State) && sku.Funnel.SearchSurface != nil && sku.Funnel.SearchSurface.Impressions != nil && countPresent(*sku.Funnel.SearchSurface.Impressions) {
			p.SKUFunnel = true
		}
		if sku.Search != nil && usableBlock(sku.Search.State) {
			p.SKUSearch = true
		}
		if sku.Supply != nil && usableBlock(sku.Supply.State) {
			p.SKUInbound = true
		}
	}
	return p
}

func newGroundingCache() GroundingCache {
	return GroundingCache{
		SKUDemandCur: map[int64]factualapi.SKUDemand{}, SKUDemandPrev: map[int64]factualapi.SKUDemand{},
		SKUStock: map[int64]factualapi.SKUStock{}, SKUCoverage: map[int64]factualapi.StockCoverage{},
		SKUPrice: map[int64]factualapi.SKUPrice{}, SKUPriceChange: map[int64]factualapi.SKUPriceLastObservedChange{},
		SKUInbound:       map[int64]factualapi.SKUInbound{},
		SKUAdsTrafficCur: map[int64]factualapi.SKUAdvertisingTraffic{}, SKUAdsTrafficPrev: map[int64]factualapi.SKUAdvertisingTraffic{},
		SKUAdsAttrCur: map[int64]factualapi.SKUAdvertisingAttribution{}, SKUAdsAttrPrev: map[int64]factualapi.SKUAdvertisingAttribution{},
		SKUFunnelSearchCur: map[int64]factualapi.SKUFunnelPeriod{}, SKUFunnelSearchPrev: map[int64]factualapi.SKUFunnelPeriod{},
		SKUSearchCur: map[int64]factualapi.SKUSearchSummary{}, SKUSearchPrev: map[int64]factualapi.SKUSearchSummary{},
		CatDemandCur: map[int64]factualapi.CategoryDemand{}, CatDemandPrev: map[int64]factualapi.CategoryDemand{},
		CampTrafficCur: map[int64]factualapi.CampaignTrafficPerformance{}, CampTrafficPrev: map[int64]factualapi.CampaignTrafficPerformance{},
		CampAttrCur: map[int64]factualapi.CampaignAttributionPerformance{}, CampAttrPrev: map[int64]factualapi.CampaignAttributionPerformance{},
	}
}

func LoadGrounding(ctx context.Context, api packGroundingAPI, sealed factualapi.FactualReadContext, detail landscape.LandscapeSet) (GroundingCache, error) {
	c := newGroundingCache()
	plan := planGrounding(detail)
	if api == nil {
		if plan.requiresAPI() {
			return GroundingCache{}, packErr(CodePackInvalid, "Factual API required to ground AI-visible facts")
		}
		return c, nil
	}

	call := func() { c.APICallCount++ }

	if plan.AccountDemand {
		call()
		d, err := api.GetAccountDemand(ctx, sealed, factualapi.PeriodCurrent7D, nil)
		if err != nil {
			return GroundingCache{}, fmt.Errorf("ground account demand current: %w", err)
		}
		c.AccountDemandCur = d
		call()
		d, err = api.GetAccountDemand(ctx, sealed, factualapi.PeriodPrevious7D, nil)
		if err != nil {
			return GroundingCache{}, fmt.Errorf("ground account demand previous: %w", err)
		}
		c.AccountDemandPrev = d
	}
	if plan.AccountDemandDay {
		call()
		d, err := api.GetAccountDemand(ctx, sealed, factualapi.PeriodAnalysisDay, nil)
		if err != nil {
			return GroundingCache{}, fmt.Errorf("ground account demand analysis day: %w", err)
		}
		c.AccountDemandDay = d
		call()
		d, err = api.GetAccountDemand(ctx, sealed, factualapi.PeriodPreviousDay, nil)
		if err != nil {
			return GroundingCache{}, fmt.Errorf("ground account demand previous day: %w", err)
		}
		c.AccountDemandPrevDay = d
	}
	if plan.AccountFinance {
		call()
		d, err := api.GetFinanceAccountSummary(ctx, sealed, factualapi.PeriodCurrent7D, nil)
		if err != nil {
			return GroundingCache{}, fmt.Errorf("ground account finance current: %w", err)
		}
		c.AccountFinanceCur = d
		call()
		d, err = api.GetFinanceAccountSummary(ctx, sealed, factualapi.PeriodPrevious7D, nil)
		if err != nil {
			return GroundingCache{}, fmt.Errorf("ground account finance previous: %w", err)
		}
		c.AccountFinancePrev = d
	}

	skuIDs := make([]int64, 0, len(detail.SKUs))
	for _, s := range detail.SKUs {
		skuIDs = append(skuIDs, s.Identity.CanonicalSKUID)
	}
	campIDs := make([]int64, 0, len(detail.Campaigns))
	for _, x := range detail.Campaigns {
		campIDs = append(campIDs, x.Identity.CampaignID)
	}

	if len(skuIDs) > 0 {
		if plan.SKUDemand {
			call()
			res, err := api.BatchGetSKUDemand(ctx, sealed, skuIDs, factualapi.PeriodCurrent7D, nil)
			if err != nil {
				return GroundingCache{}, fmt.Errorf("ground SKU demand current: %w", err)
			}
			for _, it := range res.Items {
				if it.Demand != nil {
					c.SKUDemandCur[it.ID] = *it.Demand
				}
			}
			call()
			res, err = api.BatchGetSKUDemand(ctx, sealed, skuIDs, factualapi.PeriodPrevious7D, nil)
			if err != nil {
				return GroundingCache{}, fmt.Errorf("ground SKU demand previous: %w", err)
			}
			for _, it := range res.Items {
				if it.Demand != nil {
					c.SKUDemandPrev[it.ID] = *it.Demand
				}
			}
		}
		if plan.SKUStock {
			call()
			res, err := api.BatchGetSKUStock(ctx, sealed, skuIDs)
			if err != nil {
				return GroundingCache{}, fmt.Errorf("ground SKU stock: %w", err)
			}
			for _, it := range res.Items {
				if it.Stock != nil {
					c.SKUStock[it.ID] = *it.Stock
				}
			}
		}
		if plan.SKUCoverage {
			call()
			res, err := api.BatchGetStockCoverage(ctx, sealed, skuIDs)
			if err != nil {
				return GroundingCache{}, fmt.Errorf("ground SKU coverage: %w", err)
			}
			for _, it := range res.Items {
				if it.Coverage != nil {
					c.SKUCoverage[it.ID] = *it.Coverage
				}
			}
		}
		if plan.SKUPrice {
			call()
			res, err := api.BatchGetSKUPrice(ctx, sealed, skuIDs)
			if err != nil {
				return GroundingCache{}, fmt.Errorf("ground SKU price: %w", err)
			}
			for _, it := range res.Items {
				if it.Price != nil {
					c.SKUPrice[it.ID] = *it.Price
				}
			}
		}
		if plan.SKUPriceChange {
			call()
			res, err := api.BatchGetSKUPriceLastObservedChange(ctx, sealed, skuIDs)
			if err != nil {
				return GroundingCache{}, fmt.Errorf("ground SKU price change: %w", err)
			}
			for _, it := range res.Items {
				if it.Change != nil {
					c.SKUPriceChange[it.ID] = *it.Change
				}
			}
		}
		if plan.SKUInbound {
			call()
			res, err := api.BatchGetSKUInbound(ctx, sealed, skuIDs)
			if err != nil {
				return GroundingCache{}, fmt.Errorf("ground SKU inbound: %w", err)
			}
			for _, it := range res.Items {
				if it.Inbound != nil {
					c.SKUInbound[it.ID] = *it.Inbound
				}
			}
		}
		if plan.SKUAdsTraffic {
			call()
			res, err := api.BatchGetSKUAdvertisingTraffic(ctx, sealed, skuIDs, factualapi.PeriodCurrent7D, nil)
			if err != nil {
				return GroundingCache{}, fmt.Errorf("ground SKU ads traffic current: %w", err)
			}
			for _, it := range res.Items {
				if it.Traffic != nil {
					c.SKUAdsTrafficCur[it.ID] = *it.Traffic
				}
			}
			call()
			res, err = api.BatchGetSKUAdvertisingTraffic(ctx, sealed, skuIDs, factualapi.PeriodPrevious7D, nil)
			if err != nil {
				return GroundingCache{}, fmt.Errorf("ground SKU ads traffic previous: %w", err)
			}
			for _, it := range res.Items {
				if it.Traffic != nil {
					c.SKUAdsTrafficPrev[it.ID] = *it.Traffic
				}
			}
		}
		if plan.SKUAdsAttr {
			call()
			res, err := api.BatchGetSKUAdvertisingAttribution(ctx, sealed, skuIDs, factualapi.PeriodCurrent7D, nil)
			if err != nil {
				return GroundingCache{}, fmt.Errorf("ground SKU ads attribution current: %w", err)
			}
			for _, it := range res.Items {
				if it.Attribution != nil {
					c.SKUAdsAttrCur[it.ID] = *it.Attribution
				}
			}
			call()
			res, err = api.BatchGetSKUAdvertisingAttribution(ctx, sealed, skuIDs, factualapi.PeriodPrevious7D, nil)
			if err != nil {
				return GroundingCache{}, fmt.Errorf("ground SKU ads attribution previous: %w", err)
			}
			for _, it := range res.Items {
				if it.Attribution != nil {
					c.SKUAdsAttrPrev[it.ID] = *it.Attribution
				}
			}
		}
		if plan.SKUFunnel {
			call()
			res, err := api.BatchGetSKUFunnelPeriod(ctx, sealed, skuIDs, "impressions", factualapi.PeriodCurrent7D, nil)
			if err != nil {
				return GroundingCache{}, fmt.Errorf("ground SKU funnel current: %w", err)
			}
			for _, it := range res.Items {
				if it.Funnel != nil {
					c.SKUFunnelSearchCur[it.ID] = *it.Funnel
				}
			}
			call()
			res, err = api.BatchGetSKUFunnelPeriod(ctx, sealed, skuIDs, "impressions", factualapi.PeriodPrevious7D, nil)
			if err != nil {
				return GroundingCache{}, fmt.Errorf("ground SKU funnel previous: %w", err)
			}
			for _, it := range res.Items {
				if it.Funnel != nil {
					c.SKUFunnelSearchPrev[it.ID] = *it.Funnel
				}
			}
		}
		if plan.SKUSearch {
			call()
			res, err := api.BatchGetSKUSearchSummary(ctx, sealed, skuIDs, factualapi.PeriodCurrent7D, nil)
			if err != nil {
				return GroundingCache{}, fmt.Errorf("ground SKU search current: %w", err)
			}
			for _, it := range res.Items {
				if it.Fact != nil {
					c.SKUSearchCur[it.ID] = *it.Fact
				}
			}
			call()
			res, err = api.BatchGetSKUSearchSummary(ctx, sealed, skuIDs, factualapi.PeriodPrevious7D, nil)
			if err != nil {
				return GroundingCache{}, fmt.Errorf("ground SKU search previous: %w", err)
			}
			for _, it := range res.Items {
				if it.Fact != nil {
					c.SKUSearchPrev[it.ID] = *it.Fact
				}
			}
		}
	}

	if len(campIDs) > 0 {
		if plan.CampTraffic {
			call()
			res, err := api.BatchGetCampaignTrafficPerformance(ctx, sealed, campIDs, factualapi.PeriodCurrent7D, nil)
			if err != nil {
				return GroundingCache{}, fmt.Errorf("ground campaign traffic current: %w", err)
			}
			for _, it := range res.Items {
				if it.Traffic != nil {
					c.CampTrafficCur[it.ID] = *it.Traffic
				}
			}
			call()
			res, err = api.BatchGetCampaignTrafficPerformance(ctx, sealed, campIDs, factualapi.PeriodPrevious7D, nil)
			if err != nil {
				return GroundingCache{}, fmt.Errorf("ground campaign traffic previous: %w", err)
			}
			for _, it := range res.Items {
				if it.Traffic != nil {
					c.CampTrafficPrev[it.ID] = *it.Traffic
				}
			}
		}
		if plan.CampAttr {
			call()
			res, err := api.BatchGetCampaignAttributionPerformance(ctx, sealed, campIDs, factualapi.PeriodCurrent7D, nil)
			if err != nil {
				return GroundingCache{}, fmt.Errorf("ground campaign attribution current: %w", err)
			}
			for _, it := range res.Items {
				if it.Attribution != nil {
					c.CampAttrCur[it.ID] = *it.Attribution
				}
			}
			call()
			res, err = api.BatchGetCampaignAttributionPerformance(ctx, sealed, campIDs, factualapi.PeriodPrevious7D, nil)
			if err != nil {
				return GroundingCache{}, fmt.Errorf("ground campaign attribution previous: %w", err)
			}
			for _, it := range res.Items {
				if it.Attribution != nil {
					c.CampAttrPrev[it.ID] = *it.Attribution
				}
			}
		}
	}

	if plan.CatDemand {
		if err := indexCategoryDemand(ctx, api, sealed, factualapi.PeriodCurrent7D, c.CatDemandCur, call); err != nil {
			return GroundingCache{}, fmt.Errorf("ground category demand current: %w", err)
		}
		if err := indexCategoryDemand(ctx, api, sealed, factualapi.PeriodPrevious7D, c.CatDemandPrev, call); err != nil {
			return GroundingCache{}, fmt.Errorf("ground category demand previous: %w", err)
		}
	}
	return c, nil
}

func indexCategoryDemand(
	ctx context.Context,
	api packGroundingAPI,
	sealed factualapi.FactualReadContext,
	period factualapi.StandardPeriod,
	dest map[int64]factualapi.CategoryDemand,
	call func(),
) error {
	token := ""
	for {
		call()
		page, err := api.ListCategoryDemand(ctx, sealed, period, nil, factualapi.PageRequest{PageSize: 200, PageToken: token})
		if err != nil {
			return err
		}
		for _, d := range page.Items {
			if d.CategoryID != nil {
				dest[*d.CategoryID] = d
			}
		}
		if page.NextPageToken == "" {
			return nil
		}
		token = page.NextPageToken
	}
}

func BuildFactHandleMap(detail landscape.LandscapeSet, cache GroundingCache) FactHandleMapV1 {
	m := FactHandleMapV1{SchemaVersion: FactHandleMapSchemaV2, RegistryVersion: FactHandleRegistryV2, Entries: []HandleEntry{}}
	n := 1
	add := func(e HandleEntry) {
		if e.SemanticPath == "" {
			return
		}
		if e.FactRefs == nil {
			e.FactRefs = []string{}
		}
		if len(e.FactRefs) == 0 && (e.Derivation == nil || len(e.Derivation.InputFactRefs) == 0) {
			return
		}
		e.Handle = formatHandle(n)
		n++
		m.Entries = append(m.Entries, e)
	}

	acct := PackEntity{Kind: KindAccount, ID: "account"}
	if detail.Account.Sales != nil && usableBlock(detail.Account.Sales.State) {
		if moneyPresent(detail.Account.Sales.OrderedAmount) {
			add(derivedComparison(acct, "sales.ordered_amount_comparison", cache.AccountDemandCur.Meta, cache.AccountDemandPrev.Meta, FormulaMoneyComparisonV1))
		}
		if countPresent(detail.Account.Sales.OrderedUnits) {
			add(derivedComparison(acct, "sales.ordered_units_comparison", cache.AccountDemandCur.Meta, cache.AccountDemandPrev.Meta, FormulaCountComparisonV1))
		}
		if detail.Account.Sales.AnalysisDayAmount != nil && moneyPresent(*detail.Account.Sales.AnalysisDayAmount) {
			add(derivedComparison(acct, "sales.analysis_day_amount_comparison", cache.AccountDemandDay.Meta, cache.AccountDemandPrevDay.Meta, FormulaMoneyComparisonV1))
		}
		if detail.Account.Sales.AnalysisDayUnits != nil && countPresent(*detail.Account.Sales.AnalysisDayUnits) {
			add(derivedComparison(acct, "sales.analysis_day_units_comparison", cache.AccountDemandDay.Meta, cache.AccountDemandPrevDay.Meta, FormulaCountComparisonV1))
		}
	}
	if detail.Account.Finance != nil && usableBlock(detail.Account.Finance.State) && moneyPresent(detail.Account.Finance.NetAccrual) {
		add(derivedComparison(acct, "finance.net_accrual_comparison", cache.AccountFinanceCur.Meta, cache.AccountFinancePrev.Meta, FormulaMoneyComparisonV1))
	}

	for _, cat := range detail.Categories {
		ent := PackEntity{Kind: KindCategory, ID: strconv.FormatInt(cat.Identity.CategoryID, 10)}
		cur := cache.CatDemandCur[cat.Identity.CategoryID]
		prev := cache.CatDemandPrev[cat.Identity.CategoryID]
		if cat.Sales != nil && usableBlock(cat.Sales.State) {
			if moneyPresent(cat.Sales.OrderedAmount) {
				add(derivedComparison(ent, "sales.ordered_amount_comparison", cur.Demand.Meta, prev.Demand.Meta, FormulaMoneyComparisonV1))
			}
			if countPresent(cat.Sales.OrderedUnits) {
				add(derivedComparison(ent, "sales.ordered_units_comparison", cur.Demand.Meta, prev.Demand.Meta, FormulaCountComparisonV1))
			}
		}
	}

	for _, camp := range detail.Campaigns {
		ent := PackEntity{Kind: KindCampaign, ID: strconv.FormatInt(camp.Identity.CampaignID, 10)}
		if camp.Performance != nil && camp.Performance.Traffic != nil && usableBlock(camp.Performance.Traffic.State) && moneyPresent(camp.Performance.Traffic.PaidSpend) {
			cur := cache.CampTrafficCur[camp.Identity.CampaignID]
			prev := cache.CampTrafficPrev[camp.Identity.CampaignID]
			add(derivedComparison(ent, "advertising.traffic.paid_spend_comparison", cur.Meta, prev.Meta, FormulaMoneyComparisonV1))
		}
		if camp.Performance != nil && camp.Performance.Attribution != nil && usableBlock(camp.Performance.Attribution.State) && moneyPresent(camp.Performance.Attribution.AttributedOrderAmount) {
			cur := cache.CampAttrCur[camp.Identity.CampaignID]
			prev := cache.CampAttrPrev[camp.Identity.CampaignID]
			add(derivedComparison(ent, "advertising.attribution.attributed_order_amount_comparison", cur.Meta, prev.Meta, FormulaMoneyComparisonV1))
		}
	}

	for _, sku := range detail.SKUs {
		id := sku.Identity.CanonicalSKUID
		ent := PackEntity{Kind: KindSKU, ID: strconv.FormatInt(id, 10)}
		if sku.Sales != nil && usableBlock(sku.Sales.State) {
			cur := cache.SKUDemandCur[id]
			prev := cache.SKUDemandPrev[id]
			if moneyPresent(sku.Sales.OrderedAmount) {
				add(derivedComparison(ent, "sales.ordered_amount_comparison", cur.Demand.Meta, prev.Demand.Meta, FormulaMoneyComparisonV1))
			}
			if countPresent(sku.Sales.OrderedUnits) {
				add(derivedComparison(ent, "sales.ordered_units_comparison", cur.Demand.Meta, prev.Demand.Meta, FormulaCountComparisonV1))
			}
		}
		if sku.Price != nil && usableBlock(sku.Price.State) {
			if p, ok := cache.SKUPrice[id]; ok {
				add(singleFact(ent, "price.observed_price", p.Meta))
			}
			if sku.Price.LastObservedChange != nil && moneyPresent(*sku.Price.LastObservedChange) {
				if ch, ok := cache.SKUPriceChange[id]; ok {
					add(singleFact(ent, "price.last_observed_price_change", ch.Meta))
				}
			}
		}
		if sku.Stock != nil && usableBlock(sku.Stock.State) {
			if st, ok := cache.SKUStock[id]; ok {
				add(singleFact(ent, "stock.current_state", st.Meta))
			}
			if cov, ok := cache.SKUCoverage[id]; ok {
				u, okU := requiredSideUsability(cov.Meta)
				refs := collectRefs(cov.Meta.FactRef, cov.Demand7DFactRef, cov.Demand30DFactRef)
				if okU && len(refs) > 0 {
					fv := cov.FormulaPolicy
					if fv == "" {
						fv = "stock_coverage_v1"
					}
					add(HandleEntry{Entity: ent, SemanticPath: "stock.coverage", AnalyticalUsability: u, FactRefs: refs,
						Derivation: &Derivation{FormulaKey: "stock.coverage_days", FormulaVersion: fv, InputFactRefs: refs}})
				}
			}
		}
		if sku.Advertising != nil && sku.Advertising.Traffic != nil && usableBlock(sku.Advertising.Traffic.State) && moneyPresent(sku.Advertising.Traffic.PaidSpend) {
			cur := cache.SKUAdsTrafficCur[id]
			prev := cache.SKUAdsTrafficPrev[id]
			add(derivedComparison(ent, "advertising.traffic.paid_spend_comparison", cur.Meta, prev.Meta, FormulaMoneyComparisonV1))
		}
		if sku.Advertising != nil && sku.Advertising.Attribution != nil && usableBlock(sku.Advertising.Attribution.State) && moneyPresent(sku.Advertising.Attribution.AttributedOrderAmount) {
			cur := cache.SKUAdsAttrCur[id]
			prev := cache.SKUAdsAttrPrev[id]
			add(derivedComparison(ent, "advertising.attribution.attributed_order_amount_comparison", cur.Meta, prev.Meta, FormulaMoneyComparisonV1))
		}
		if sku.Funnel != nil && usableBlock(sku.Funnel.State) && sku.Funnel.SearchSurface != nil && sku.Funnel.SearchSurface.Impressions != nil && countPresent(*sku.Funnel.SearchSurface.Impressions) {
			cur := cache.SKUFunnelSearchCur[id]
			prev := cache.SKUFunnelSearchPrev[id]
			add(derivedComparison(ent, "funnel.search.impressions_comparison", cur.Metric.Meta, prev.Metric.Meta, FormulaCountComparisonV1))
		}
		if sku.Search != nil && usableBlock(sku.Search.State) {
			if cur, ok := cache.SKUSearchCur[id]; ok {
				add(singleFact(ent, "search.provider_average_position", cur.Meta))
			}
		}
		if sku.Supply != nil && usableBlock(sku.Supply.State) {
			if inb, ok := cache.SKUInbound[id]; ok {
				add(singleFact(ent, "supply.open_inbound", inb.Meta))
			}
		}
	}
	return m
}

func usableBlock(s landscape.BlockState) bool {
	return s != landscape.BlockUnavailable && s != landscape.BlockUnknown && s != landscape.BlockNotApplicable
}

func singleFact(ent PackEntity, path string, meta factualapi.FactMeta) HandleEntry {
	u, ok := requiredSideUsability(meta)
	if !ok {
		return HandleEntry{}
	}
	if u == UsabilityPartial && !allowedPartial(u, meta.Limitations) {
		return HandleEntry{}
	}
	return HandleEntry{Entity: ent, SemanticPath: path, AnalyticalUsability: u, FactRefs: collectRefs(meta.FactRef)}
}

func derivedComparison(ent PackEntity, path string, cur, prev factualapi.FactMeta, formula string) HandleEntry {
	u, ok := comparisonUsability(cur, prev)
	if !ok {
		return HandleEntry{}
	}
	if cur.FactRef == "" || prev.FactRef == "" {
		return HandleEntry{}
	}
	refs := collectRefs(cur.FactRef, prev.FactRef)
	if len(refs) < 2 {
		return HandleEntry{}
	}
	if u == UsabilityPartial && !allowedPartial(u, append(append([]factualapi.Limitation{}, cur.Limitations...), prev.Limitations...)) {
		return HandleEntry{}
	}
	return HandleEntry{
		Entity: ent, SemanticPath: path, AnalyticalUsability: u, FactRefs: refs,
		Derivation: &Derivation{FormulaKey: formula, FormulaVersion: formula, InputFactRefs: refs},
	}
}
