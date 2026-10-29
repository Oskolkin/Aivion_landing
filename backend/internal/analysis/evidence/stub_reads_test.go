package evidence

import (
	"context"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

type stubReads struct{}

func (stubReads) GetAccountDemandSeries(context.Context, factualapi.FactualReadContext, factualapi.StandardPeriod, *factualapi.Period) ([]factualapi.DemandPoint, error) {
	return nil, nil
}
func (stubReads) GetCategoryDemandSeries(context.Context, factualapi.FactualReadContext, int64, factualapi.StandardPeriod, *factualapi.Period) ([]factualapi.DemandPoint, error) {
	return nil, nil
}
func (stubReads) BatchGetSKUDemandSeries(context.Context, factualapi.FactualReadContext, []int64, factualapi.StandardPeriod, *factualapi.Period) (factualapi.BatchSKUDemandSeriesResult, error) {
	return factualapi.BatchSKUDemandSeriesResult{}, nil
}
func (stubReads) GetAccountDemand(context.Context, factualapi.FactualReadContext, factualapi.StandardPeriod, *factualapi.Period) (factualapi.Demand, error) {
	return factualapi.Demand{}, nil
}
func (stubReads) GetSKUDemand(context.Context, factualapi.FactualReadContext, int64, factualapi.StandardPeriod, *factualapi.Period) (factualapi.SKUDemand, error) {
	return factualapi.SKUDemand{}, nil
}
func (stubReads) GetCategoryDemand(context.Context, factualapi.FactualReadContext, *int64, factualapi.StandardPeriod, *factualapi.Period) (factualapi.CategoryDemand, error) {
	return factualapi.CategoryDemand{}, nil
}
func (stubReads) ListSKUDemand(context.Context, factualapi.FactualReadContext, factualapi.StandardPeriod, *factualapi.Period, factualapi.PageRequest) (factualapi.Page[factualapi.SKUDemand], error) {
	return factualapi.Page[factualapi.SKUDemand]{}, nil
}
func (stubReads) ListCategoryDemand(context.Context, factualapi.FactualReadContext, factualapi.StandardPeriod, *factualapi.Period, factualapi.PageRequest) (factualapi.Page[factualapi.CategoryDemand], error) {
	return factualapi.Page[factualapi.CategoryDemand]{}, nil
}
func (stubReads) BatchGetSKUTrendContext(context.Context, factualapi.FactualReadContext, []int64) (factualapi.BatchSKUTrendContextResult, error) {
	return factualapi.BatchSKUTrendContextResult{}, nil
}
func (stubReads) ListLifecycleEvents(context.Context, factualapi.FactualReadContext, factualapi.LifecycleEventFilter, factualapi.PageRequest) (factualapi.Page[factualapi.LifecycleEvent], error) {
	return factualapi.Page[factualapi.LifecycleEvent]{}, nil
}
func (stubReads) ListReturns(context.Context, factualapi.FactualReadContext, factualapi.ReturnFilter, factualapi.PageRequest) (factualapi.Page[factualapi.Return], error) {
	return factualapi.Page[factualapi.Return]{}, nil
}
func (stubReads) GetSKUStockHistory(context.Context, factualapi.FactualReadContext, int64, factualapi.PageRequest) (factualapi.Page[factualapi.SKUStock], error) {
	return factualapi.Page[factualapi.SKUStock]{}, nil
}
func (stubReads) BatchGetSKUStock(context.Context, factualapi.FactualReadContext, []int64) (factualapi.BatchSKUStockResult, error) {
	return factualapi.BatchSKUStockResult{}, nil
}
func (stubReads) GetStockLocationBreakdown(context.Context, factualapi.FactualReadContext, int64) ([]factualapi.StockPosition, error) {
	return nil, nil
}
func (stubReads) ListSKUStock(context.Context, factualapi.FactualReadContext, factualapi.StockFilter, factualapi.PageRequest) (factualapi.Page[factualapi.SKUStock], error) {
	return factualapi.Page[factualapi.SKUStock]{}, nil
}
func (stubReads) GetSKUPriceHistory(context.Context, factualapi.FactualReadContext, int64, factualapi.PageRequest) (factualapi.Page[factualapi.SKUPrice], error) {
	return factualapi.Page[factualapi.SKUPrice]{}, nil
}
func (stubReads) GetCampaignSeries(context.Context, factualapi.FactualReadContext, int64, factualapi.StandardPeriod, *factualapi.Period) ([]factualapi.AdvertisingDailyFact, error) {
	return nil, nil
}
func (stubReads) BatchListCampaignTargetSKUs(context.Context, factualapi.FactualReadContext, []int64, int) (factualapi.BatchCampaignTargetSKUResult, error) {
	return factualapi.BatchCampaignTargetSKUResult{}, nil
}
func (stubReads) BatchGetSKUAdvertising(context.Context, factualapi.FactualReadContext, []int64, factualapi.StandardPeriod, *factualapi.Period) (factualapi.BatchSKUAdvertisingResult, error) {
	return factualapi.BatchSKUAdvertisingResult{}, nil
}
func (stubReads) BatchListSKUCampaignPerformance(context.Context, factualapi.FactualReadContext, []int64, factualapi.StandardPeriod, *factualapi.Period, int) (factualapi.BatchSKUCampaignPerformanceResult, error) {
	return factualapi.BatchSKUCampaignPerformanceResult{}, nil
}
func (stubReads) ListCPOAttribution(context.Context, factualapi.FactualReadContext, factualapi.CPOFilter, factualapi.PageRequest) (factualapi.Page[factualapi.CPOAttribution], error) {
	return factualapi.Page[factualapi.CPOAttribution]{}, nil
}
func (stubReads) GetAccountFunnelPeriod(context.Context, factualapi.FactualReadContext, string, factualapi.StandardPeriod, *factualapi.Period) (factualapi.AccountFunnelPeriod, error) {
	return factualapi.AccountFunnelPeriod{}, nil
}
func (stubReads) GetCategoryFunnelPeriod(context.Context, factualapi.FactualReadContext, int64, string, factualapi.StandardPeriod, *factualapi.Period) (factualapi.CategoryFunnelPeriod, error) {
	return factualapi.CategoryFunnelPeriod{}, nil
}
func (stubReads) GetSKUFunnelSeries(context.Context, factualapi.FactualReadContext, int64, string, factualapi.StandardPeriod, *factualapi.Period) ([]factualapi.FunnelPoint, error) {
	return nil, nil
}
func (stubReads) GetFunnelCapabilities(context.Context, factualapi.FactualReadContext) (factualapi.FunnelCapabilities, error) {
	return factualapi.FunnelCapabilities{}, nil
}
func (stubReads) GetSearchSummarySeries(context.Context, factualapi.FactualReadContext, int64, factualapi.StandardPeriod, *factualapi.Period) ([]factualapi.SKUSearchSummary, error) {
	return nil, nil
}
func (stubReads) BatchGetSKUQueryDetails(context.Context, factualapi.FactualReadContext, []int64, factualapi.StandardPeriod, *factualapi.Period, int) (factualapi.BatchSKUQueryDetailsResult, error) {
	return factualapi.BatchSKUQueryDetailsResult{}, nil
}
func (stubReads) ListMarketSearchFacts(context.Context, factualapi.FactualReadContext, factualapi.PageRequest) (factualapi.Page[factualapi.MarketSearchFact], error) {
	return factualapi.Page[factualapi.MarketSearchFact]{}, nil
}
func (stubReads) GetFinanceDailyLedger(context.Context, factualapi.FactualReadContext, factualapi.StandardPeriod, *factualapi.Period) ([]factualapi.FinanceDailyNet, error) {
	return nil, nil
}
func (stubReads) ListFinanceAccruals(context.Context, factualapi.FactualReadContext, factualapi.FinanceAccrualFilter, factualapi.PageRequest) (factualapi.Page[factualapi.FinanceAccrual], error) {
	return factualapi.Page[factualapi.FinanceAccrual]{}, nil
}
func (stubReads) BatchGetSKUPromotions(context.Context, factualapi.FactualReadContext, []int64, int) (factualapi.BatchSKUPromotionsResult, error) {
	return factualapi.BatchSKUPromotionsResult{}, nil
}
func (stubReads) GetPromotionMembershipHistory(context.Context, factualapi.FactualReadContext, int64, factualapi.PromotionMembershipFilter, factualapi.PageRequest) (factualapi.Page[factualapi.PromotionMembershipInterval], error) {
	return factualapi.Page[factualapi.PromotionMembershipInterval]{}, nil
}
func (stubReads) GetPromotionEligibility(context.Context, factualapi.FactualReadContext, int64, factualapi.PageRequest) (factualapi.PromotionEligibility, error) {
	return factualapi.PromotionEligibility{}, nil
}
func (stubReads) BatchGetCategoryPromotionSummary(context.Context, factualapi.FactualReadContext, []int64) (factualapi.BatchCategoryPromotionSummaryResult, error) {
	return factualapi.BatchCategoryPromotionSummaryResult{}, nil
}
func (stubReads) GetPromotion(context.Context, factualapi.FactualReadContext, int64) (factualapi.Promotion, error) {
	return factualapi.Promotion{}, nil
}
func (stubReads) ListPromotions(context.Context, factualapi.FactualReadContext, factualapi.PromotionFilter, factualapi.PageRequest) (factualapi.Page[factualapi.Promotion], error) {
	return factualapi.Page[factualapi.Promotion]{}, nil
}
func (stubReads) GetRatingHistory(context.Context, factualapi.FactualReadContext, int64, factualapi.PageRequest) (factualapi.Page[factualapi.RatingPeriodFact], error) {
	return factualapi.Page[factualapi.RatingPeriodFact]{}, nil
}
func (stubReads) GetRatingThresholds(context.Context, factualapi.FactualReadContext, int64) ([]factualapi.RatingThreshold, error) {
	return nil, nil
}
func (stubReads) GetCurrentRatings(context.Context, factualapi.FactualReadContext) ([]factualapi.AccountRating, error) {
	return nil, nil
}
func (stubReads) GetFBSErrorIndex(context.Context, factualapi.FactualReadContext) (factualapi.FBSErrorIndex, error) {
	return factualapi.FBSErrorIndex{}, nil
}
func (stubReads) ListFBSRatingPostingErrors(context.Context, factualapi.FactualReadContext, factualapi.PageRequest) (factualapi.Page[factualapi.FBSRatingPostingError], error) {
	return factualapi.Page[factualapi.FBSRatingPostingError]{}, nil
}
func (stubReads) BatchGetSKUInbound(context.Context, factualapi.FactualReadContext, []int64) (factualapi.BatchSKUInboundResult, error) {
	return factualapi.BatchSKUInboundResult{}, nil
}
func (stubReads) ListSupplies(context.Context, factualapi.FactualReadContext, factualapi.SupplyFilter, factualapi.PageRequest) (factualapi.Page[factualapi.InboundSupply], error) {
	return factualapi.Page[factualapi.InboundSupply]{}, nil
}
func (stubReads) GetSupplyStateHistory(context.Context, factualapi.FactualReadContext, int64, factualapi.PageRequest) (factualapi.Page[factualapi.SupplyStateObservation], error) {
	return factualapi.Page[factualapi.SupplyStateObservation]{}, nil
}
func (stubReads) ListSupplyPlannedItems(context.Context, factualapi.FactualReadContext, int64, factualapi.PageRequest) (factualapi.Page[factualapi.SupplyPlannedItem], error) {
	return factualapi.Page[factualapi.SupplyPlannedItem]{}, nil
}
func (stubReads) GetSupplyAcceptance(context.Context, factualapi.FactualReadContext, int64) (factualapi.SupplyAcceptance, error) {
	return factualapi.SupplyAcceptance{}, nil
}
func (stubReads) ListSupplyAcceptanceItems(context.Context, factualapi.FactualReadContext, int64, factualapi.PageRequest) (factualapi.Page[factualapi.SupplyAcceptanceItem], error) {
	return factualapi.Page[factualapi.SupplyAcceptanceItem]{}, nil
}
