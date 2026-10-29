package factualapi

import (
	"context"
	"time"
)

type API interface {
	GetAccount(context.Context, FactualReadContext) (Account, error)
	GetSKU(context.Context, FactualReadContext, int64) (SKU, error)
	BatchGetSKUs(context.Context, FactualReadContext, []int64) (BatchSKUResult, error)
	ListSKUs(context.Context, FactualReadContext, SKUFilter, PageRequest) (Page[SKU], error)
	GetCategory(context.Context, FactualReadContext, int64) (Category, error)
	BatchGetCategories(context.Context, FactualReadContext, []int64) (BatchCategoryResult, error)
	ListCategories(context.Context, FactualReadContext, PageRequest) (Page[Category], error)
	SearchEntities(context.Context, FactualReadContext, EntitySearchRequest) (EntitySearchResult, error)
	GetOrder(context.Context, FactualReadContext, int64) (Order, error)
	GetPosting(context.Context, FactualReadContext, int64) (Posting, error)
	ListPostings(context.Context, FactualReadContext, PostingFilter, PageRequest) (Page[Posting], error)
	ListLifecycleEvents(context.Context, FactualReadContext, LifecycleEventFilter, PageRequest) (Page[LifecycleEvent], error)
	GetReturn(context.Context, FactualReadContext, int64) (Return, error)
	ListReturns(context.Context, FactualReadContext, ReturnFilter, PageRequest) (Page[Return], error)
	GetAccountDemand(context.Context, FactualReadContext, StandardPeriod, *Period) (Demand, error)
	GetSKUDemand(context.Context, FactualReadContext, int64, StandardPeriod, *Period) (SKUDemand, error)
	BatchGetSKUDemand(context.Context, FactualReadContext, []int64, StandardPeriod, *Period) (BatchSKUDemandResult, error)
	BatchGetSKUTrendContext(context.Context, FactualReadContext, []int64) (BatchSKUTrendContextResult, error)
	ListSKUDemand(context.Context, FactualReadContext, StandardPeriod, *Period, PageRequest) (Page[SKUDemand], error)
	GetDemandSeries(context.Context, FactualReadContext, int64, StandardPeriod, *Period) ([]DemandPoint, error)
	GetAccountDemandSeries(context.Context, FactualReadContext, StandardPeriod, *Period) ([]DemandPoint, error)
	GetCategoryDemandSeries(context.Context, FactualReadContext, int64, StandardPeriod, *Period) ([]DemandPoint, error)
	BatchGetSKUDemandSeries(context.Context, FactualReadContext, []int64, StandardPeriod, *Period) (BatchSKUDemandSeriesResult, error)
	GetCategoryDemand(context.Context, FactualReadContext, *int64, StandardPeriod, *Period) (CategoryDemand, error)
	ListCategoryDemand(context.Context, FactualReadContext, StandardPeriod, *Period, PageRequest) (Page[CategoryDemand], error)
	GetSKUStock(context.Context, FactualReadContext, int64) (SKUStock, error)
	BatchGetSKUStock(context.Context, FactualReadContext, []int64) (BatchSKUStockResult, error)
	ListSKUStock(context.Context, FactualReadContext, StockFilter, PageRequest) (Page[SKUStock], error)
	GetStockLocationBreakdown(context.Context, FactualReadContext, int64) ([]StockPosition, error)
	GetStockSnapshot(context.Context, FactualReadContext, int64) (StockSnapshot, error)
	ListStockSnapshots(context.Context, FactualReadContext, PageRequest) (Page[StockSnapshot], error)
	GetSKUStockHistory(context.Context, FactualReadContext, int64, PageRequest) (Page[SKUStock], error)
	GetStockCoverage(context.Context, FactualReadContext, int64) (StockCoverage, error)
	BatchGetStockCoverage(context.Context, FactualReadContext, []int64) (BatchStockCoverageResult, error)
	GetSKUPrice(context.Context, FactualReadContext, int64) (SKUPrice, error)
	BatchGetSKUPrice(context.Context, FactualReadContext, []int64) (BatchSKUPriceResult, error)
	ListSKUPrices(context.Context, FactualReadContext, PriceFilter, PageRequest) (Page[SKUPrice], error)
	GetSKUPriceAsOf(context.Context, FactualReadContext, int64, time.Time) (SKUPrice, error)
	GetSKUPriceHistory(context.Context, FactualReadContext, int64, PageRequest) (Page[SKUPrice], error)
	BatchGetSKUPriceLastObservedChange(context.Context, FactualReadContext, []int64) (BatchSKUPriceLastObservedChangeResult, error)
	GetSKUPriceIndex(context.Context, FactualReadContext, int64) (PriceIndex, error)
	GetCampaign(context.Context, FactualReadContext, int64) (Campaign, error)
	BatchGetCampaigns(context.Context, FactualReadContext, []int64) (BatchCampaignResult, error)
	ListCampaigns(context.Context, FactualReadContext, CampaignFilter, PageRequest) (Page[Campaign], error)
	GetCampaignDailyFacts(context.Context, FactualReadContext, int64, StandardPeriod, *Period) ([]AdvertisingDailyFact, error)
	GetCampaignSeries(context.Context, FactualReadContext, int64, StandardPeriod, *Period) ([]AdvertisingDailyFact, error)
	GetCampaignTargetScope(context.Context, FactualReadContext, int64) (string, error)
	ListCampaignTargetSKUs(context.Context, FactualReadContext, int64, PageRequest) (Page[CampaignTarget], error)
	BatchListCampaignTargetSKUs(context.Context, FactualReadContext, []int64, int) (BatchCampaignTargetSKUResult, error)
	GetCampaignDRR(context.Context, FactualReadContext, int64, StandardPeriod, *Period) (CampaignDRR, error)
	GetAdvertisingAccountSummary(context.Context, FactualReadContext, StandardPeriod, *Period) (AdvertisingAccountSummary, error)
	GetAdvertisingSpendShare(context.Context, FactualReadContext, StandardPeriod, *Period) (AdvertisingSpendShare, error)
	GetSKUAdvertising(context.Context, FactualReadContext, int64, StandardPeriod, *Period) (SKUAdvertising, error)
	BatchGetSKUAdvertising(context.Context, FactualReadContext, []int64, StandardPeriod, *Period) (BatchSKUAdvertisingResult, error)
	ListSKUCampaignPerformance(context.Context, FactualReadContext, int64, StandardPeriod, *Period, PageRequest) (Page[SKUCampaignPerformance], error)
	BatchListSKUCampaignPerformance(context.Context, FactualReadContext, []int64, StandardPeriod, *Period, int) (BatchSKUCampaignPerformanceResult, error)
	BatchGetCampaignPeriodPerformance(context.Context, FactualReadContext, []int64, StandardPeriod, *Period) (BatchCampaignPeriodPerformanceResult, error)
	BatchGetCampaignTrafficPerformance(context.Context, FactualReadContext, []int64, StandardPeriod, *Period) (BatchCampaignTrafficResult, error)
	BatchGetCampaignAttributionPerformance(context.Context, FactualReadContext, []int64, StandardPeriod, *Period) (BatchCampaignAttributionResult, error)
	BatchGetCampaignTargetingSummary(context.Context, FactualReadContext, []int64, StandardPeriod, *Period) (BatchCampaignTargetingSummaryResult, error)
	BatchGetSKUAdvertisingTraffic(context.Context, FactualReadContext, []int64, StandardPeriod, *Period) (BatchSKUAdvertisingTrafficResult, error)
	BatchGetSKUAdvertisingAttribution(context.Context, FactualReadContext, []int64, StandardPeriod, *Period) (BatchSKUAdvertisingAttributionResult, error)
	ListCPOAttribution(context.Context, FactualReadContext, CPOFilter, PageRequest) (Page[CPOAttribution], error)
	GetAccountFunnelPeriod(context.Context, FactualReadContext, string, StandardPeriod, *Period) (AccountFunnelPeriod, error)
	GetSKUFunnelPeriod(context.Context, FactualReadContext, int64, string, StandardPeriod, *Period) (SKUFunnelPeriod, error)
	BatchGetSKUFunnelPeriod(context.Context, FactualReadContext, []int64, string, StandardPeriod, *Period) (BatchSKUFunnelPeriodResult, error)
	ListSKUFunnelPeriod(context.Context, FactualReadContext, string, StandardPeriod, *Period, PageRequest) (Page[SKUFunnelPeriod], error)
	GetSKUFunnelSeries(context.Context, FactualReadContext, int64, string, StandardPeriod, *Period) ([]FunnelPoint, error)
	GetCategoryFunnelPeriod(context.Context, FactualReadContext, int64, string, StandardPeriod, *Period) (CategoryFunnelPeriod, error)
	BatchGetCategoryFunnelPeriod(context.Context, FactualReadContext, []int64, string, StandardPeriod, *Period) (BatchCategoryFunnelPeriodResult, error)
	GetFunnelMetric(context.Context, FactualReadContext, FunnelMetricQuery) (FunnelMetricValue, error)
	GetFunnelCapabilities(context.Context, FactualReadContext) (FunnelCapabilities, error)
	GetFinanceAccountSummary(context.Context, FactualReadContext, StandardPeriod, *Period) (FinanceAccountSummary, error)
	GetFinanceAccountBreakdown(context.Context, FactualReadContext, StandardPeriod, *Period) (FinanceAccountBreakdown, error)
	GetFinanceDailyLedger(context.Context, FactualReadContext, StandardPeriod, *Period) ([]FinanceDailyNet, error)
	ListFinanceAccruals(context.Context, FactualReadContext, FinanceAccrualFilter, PageRequest) (Page[FinanceAccrual], error)
	GetFinanceBalance(context.Context, FactualReadContext) (FinanceBalance, error)
	GetSKUSearchSummary(context.Context, FactualReadContext, int64, StandardPeriod, *Period) (SKUSearchSummary, error)
	BatchGetSKUSearchSummary(context.Context, FactualReadContext, []int64, StandardPeriod, *Period) (BatchSKUSearchSummaryResult, error)
	ListSKUSearchSummary(context.Context, FactualReadContext, StandardPeriod, *Period, PageRequest) (Page[SKUSearchSummary], error)
	GetSearchSummarySeries(context.Context, FactualReadContext, int64, StandardPeriod, *Period) ([]SKUSearchSummary, error)
	GetSKUQueryDetails(context.Context, FactualReadContext, int64, PageRequest) (Page[SearchQueryDetail], error)
	BatchGetSKUQueryDetails(context.Context, FactualReadContext, []int64, StandardPeriod, *Period, int) (BatchSKUQueryDetailsResult, error)
	GetSearchQueryFact(context.Context, FactualReadContext, int64) (SearchQueryFact, error)
	GetCategorySearch(context.Context, FactualReadContext, int64, StandardPeriod, *Period) (SearchCategorySummary, error)
	GetCategorySearchSummary(context.Context, FactualReadContext, int64, StandardPeriod, *Period) (SearchCategorySummary, error)
	BatchGetCategorySearchDistribution(context.Context, FactualReadContext, []int64, StandardPeriod, *Period) (BatchCategorySearchDistributionResult, error)
	ListMarketSearchFacts(context.Context, FactualReadContext, PageRequest) (Page[MarketSearchFact], error)
	GetPromotion(context.Context, FactualReadContext, int64) (Promotion, error)
	ListPromotions(context.Context, FactualReadContext, PromotionFilter, PageRequest) (Page[Promotion], error)
	ListPromotionParticipants(context.Context, FactualReadContext, int64, PageRequest) (Page[PromotionParticipant], error)
	ListPromotionCandidates(context.Context, FactualReadContext, int64, PageRequest) (Page[PromotionCandidate], error)
	GetSKUPromotions(context.Context, FactualReadContext, int64, PageRequest) (Page[SKUPromotion], error)
	BatchGetSKUPromotions(context.Context, FactualReadContext, []int64, int) (BatchSKUPromotionsResult, error)
	BatchGetSKUPromotionSummary(context.Context, FactualReadContext, []int64) (BatchSKUPromotionSummaryResult, error)
	GetPromotionAccountSummary(context.Context, FactualReadContext) (PromotionAccountSummary, error)
	BatchGetCategoryPromotionSummary(context.Context, FactualReadContext, []int64) (BatchCategoryPromotionSummaryResult, error)
	GetPromotionEligibility(context.Context, FactualReadContext, int64, PageRequest) (PromotionEligibility, error)
	GetPromotionMembershipHistory(context.Context, FactualReadContext, int64, PromotionMembershipFilter, PageRequest) (Page[PromotionMembershipInterval], error)
	GetSupplyOrder(context.Context, FactualReadContext, int64) (SupplyOrder, error)
	GetSupply(context.Context, FactualReadContext, int64) (InboundSupply, error)
	ListSupplies(context.Context, FactualReadContext, SupplyFilter, PageRequest) (Page[InboundSupply], error)
	ListSupplyPlannedItems(context.Context, FactualReadContext, int64, PageRequest) (Page[SupplyPlannedItem], error)
	GetSupplyStateHistory(context.Context, FactualReadContext, int64, PageRequest) (Page[SupplyStateObservation], error)
	GetSupplyAcceptance(context.Context, FactualReadContext, int64) (SupplyAcceptance, error)
	GetSupplyAccountSummary(context.Context, FactualReadContext) (SupplyAccountSummary, error)
	ListSupplyAcceptanceItems(context.Context, FactualReadContext, int64, PageRequest) (Page[SupplyAcceptanceItem], error)
	GetSKUInbound(context.Context, FactualReadContext, int64) (SKUInbound, error)
	BatchGetSKUInbound(context.Context, FactualReadContext, []int64) (BatchSKUInboundResult, error)
	GetAccountHealthSummary(context.Context, FactualReadContext) (AccountHealthSummary, error)
	GetCurrentRatings(context.Context, FactualReadContext) ([]AccountRating, error)
	GetRating(context.Context, FactualReadContext, int64) (AccountRating, error)
	GetRatingHistory(context.Context, FactualReadContext, int64, PageRequest) (Page[RatingPeriodFact], error)
	GetRatingThresholds(context.Context, FactualReadContext, int64) ([]RatingThreshold, error)
	GetFBSErrorIndex(context.Context, FactualReadContext) (FBSErrorIndex, error)
	ListFBSRatingPostingErrors(context.Context, FactualReadContext, PageRequest) (Page[FBSRatingPostingError], error)
	TraceFact(context.Context, FactualReadContext, string) (FactTrace, error)
}

type TraceInput struct {
	Kind                   string
	EntityID               int64
	AuxiliaryID            int64
	Period                 *Period
	FilterHash, ResultHash string
}
type TraceSource struct {
	CanonicalFactID                         int64
	RevisionNo                              int64
	SourcePayloadID                         int64
	EndpointKey, ContractHash, SemanticRole string
	SourceReceivedAt, CanonicalObservedAt   string
}
type FactTrace struct {
	Kind, ContextHash, ResultHash, PolicyVersion string
	Sources                                      []TraceSource
	Limitations                                  []Limitation
	Inputs                                       []string
}
