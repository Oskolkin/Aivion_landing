package evidence

import (
	"context"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

// Reads is the sealed Factual API surface Evidence Expansion may call.
type Reads interface {
	GetAccountDemandSeries(context.Context, factualapi.FactualReadContext, factualapi.StandardPeriod, *factualapi.Period) ([]factualapi.DemandPoint, error)
	GetCategoryDemandSeries(context.Context, factualapi.FactualReadContext, int64, factualapi.StandardPeriod, *factualapi.Period) ([]factualapi.DemandPoint, error)
	BatchGetSKUDemandSeries(context.Context, factualapi.FactualReadContext, []int64, factualapi.StandardPeriod, *factualapi.Period) (factualapi.BatchSKUDemandSeriesResult, error)
	GetAccountDemand(context.Context, factualapi.FactualReadContext, factualapi.StandardPeriod, *factualapi.Period) (factualapi.Demand, error)
	GetSKUDemand(context.Context, factualapi.FactualReadContext, int64, factualapi.StandardPeriod, *factualapi.Period) (factualapi.SKUDemand, error)
	GetCategoryDemand(context.Context, factualapi.FactualReadContext, *int64, factualapi.StandardPeriod, *factualapi.Period) (factualapi.CategoryDemand, error)
	ListSKUDemand(context.Context, factualapi.FactualReadContext, factualapi.StandardPeriod, *factualapi.Period, factualapi.PageRequest) (factualapi.Page[factualapi.SKUDemand], error)
	ListCategoryDemand(context.Context, factualapi.FactualReadContext, factualapi.StandardPeriod, *factualapi.Period, factualapi.PageRequest) (factualapi.Page[factualapi.CategoryDemand], error)
	BatchGetSKUTrendContext(context.Context, factualapi.FactualReadContext, []int64) (factualapi.BatchSKUTrendContextResult, error)
	ListLifecycleEvents(context.Context, factualapi.FactualReadContext, factualapi.LifecycleEventFilter, factualapi.PageRequest) (factualapi.Page[factualapi.LifecycleEvent], error)
	ListReturns(context.Context, factualapi.FactualReadContext, factualapi.ReturnFilter, factualapi.PageRequest) (factualapi.Page[factualapi.Return], error)
	GetSKUStockHistory(context.Context, factualapi.FactualReadContext, int64, factualapi.PageRequest) (factualapi.Page[factualapi.SKUStock], error)
	BatchGetSKUStock(context.Context, factualapi.FactualReadContext, []int64) (factualapi.BatchSKUStockResult, error)
	GetStockLocationBreakdown(context.Context, factualapi.FactualReadContext, int64) ([]factualapi.StockPosition, error)
	ListSKUStock(context.Context, factualapi.FactualReadContext, factualapi.StockFilter, factualapi.PageRequest) (factualapi.Page[factualapi.SKUStock], error)
	GetSKUPriceHistory(context.Context, factualapi.FactualReadContext, int64, factualapi.PageRequest) (factualapi.Page[factualapi.SKUPrice], error)
	GetCampaignSeries(context.Context, factualapi.FactualReadContext, int64, factualapi.StandardPeriod, *factualapi.Period) ([]factualapi.AdvertisingDailyFact, error)
	BatchListCampaignTargetSKUs(context.Context, factualapi.FactualReadContext, []int64, int) (factualapi.BatchCampaignTargetSKUResult, error)
	BatchGetSKUAdvertising(context.Context, factualapi.FactualReadContext, []int64, factualapi.StandardPeriod, *factualapi.Period) (factualapi.BatchSKUAdvertisingResult, error)
	BatchListSKUCampaignPerformance(context.Context, factualapi.FactualReadContext, []int64, factualapi.StandardPeriod, *factualapi.Period, int) (factualapi.BatchSKUCampaignPerformanceResult, error)
	ListCPOAttribution(context.Context, factualapi.FactualReadContext, factualapi.CPOFilter, factualapi.PageRequest) (factualapi.Page[factualapi.CPOAttribution], error)
	GetAccountFunnelPeriod(context.Context, factualapi.FactualReadContext, string, factualapi.StandardPeriod, *factualapi.Period) (factualapi.AccountFunnelPeriod, error)
	GetCategoryFunnelPeriod(context.Context, factualapi.FactualReadContext, int64, string, factualapi.StandardPeriod, *factualapi.Period) (factualapi.CategoryFunnelPeriod, error)
	GetSKUFunnelSeries(context.Context, factualapi.FactualReadContext, int64, string, factualapi.StandardPeriod, *factualapi.Period) ([]factualapi.FunnelPoint, error)
	GetFunnelCapabilities(context.Context, factualapi.FactualReadContext) (factualapi.FunnelCapabilities, error)
	GetSearchSummarySeries(context.Context, factualapi.FactualReadContext, int64, factualapi.StandardPeriod, *factualapi.Period) ([]factualapi.SKUSearchSummary, error)
	BatchGetSKUQueryDetails(context.Context, factualapi.FactualReadContext, []int64, factualapi.StandardPeriod, *factualapi.Period, int) (factualapi.BatchSKUQueryDetailsResult, error)
	ListMarketSearchFacts(context.Context, factualapi.FactualReadContext, factualapi.PageRequest) (factualapi.Page[factualapi.MarketSearchFact], error)
	GetFinanceDailyLedger(context.Context, factualapi.FactualReadContext, factualapi.StandardPeriod, *factualapi.Period) ([]factualapi.FinanceDailyNet, error)
	ListFinanceAccruals(context.Context, factualapi.FactualReadContext, factualapi.FinanceAccrualFilter, factualapi.PageRequest) (factualapi.Page[factualapi.FinanceAccrual], error)
	BatchGetSKUPromotions(context.Context, factualapi.FactualReadContext, []int64, int) (factualapi.BatchSKUPromotionsResult, error)
	GetPromotionMembershipHistory(context.Context, factualapi.FactualReadContext, int64, factualapi.PromotionMembershipFilter, factualapi.PageRequest) (factualapi.Page[factualapi.PromotionMembershipInterval], error)
	GetPromotionEligibility(context.Context, factualapi.FactualReadContext, int64, factualapi.PageRequest) (factualapi.PromotionEligibility, error)
	BatchGetCategoryPromotionSummary(context.Context, factualapi.FactualReadContext, []int64) (factualapi.BatchCategoryPromotionSummaryResult, error)
	GetPromotion(context.Context, factualapi.FactualReadContext, int64) (factualapi.Promotion, error)
	ListPromotions(context.Context, factualapi.FactualReadContext, factualapi.PromotionFilter, factualapi.PageRequest) (factualapi.Page[factualapi.Promotion], error)
	GetRatingHistory(context.Context, factualapi.FactualReadContext, int64, factualapi.PageRequest) (factualapi.Page[factualapi.RatingPeriodFact], error)
	GetRatingThresholds(context.Context, factualapi.FactualReadContext, int64) ([]factualapi.RatingThreshold, error)
	GetCurrentRatings(context.Context, factualapi.FactualReadContext) ([]factualapi.AccountRating, error)
	GetFBSErrorIndex(context.Context, factualapi.FactualReadContext) (factualapi.FBSErrorIndex, error)
	ListFBSRatingPostingErrors(context.Context, factualapi.FactualReadContext, factualapi.PageRequest) (factualapi.Page[factualapi.FBSRatingPostingError], error)
	BatchGetSKUInbound(context.Context, factualapi.FactualReadContext, []int64) (factualapi.BatchSKUInboundResult, error)
	ListSupplies(context.Context, factualapi.FactualReadContext, factualapi.SupplyFilter, factualapi.PageRequest) (factualapi.Page[factualapi.InboundSupply], error)
	GetSupplyStateHistory(context.Context, factualapi.FactualReadContext, int64, factualapi.PageRequest) (factualapi.Page[factualapi.SupplyStateObservation], error)
	ListSupplyPlannedItems(context.Context, factualapi.FactualReadContext, int64, factualapi.PageRequest) (factualapi.Page[factualapi.SupplyPlannedItem], error)
	GetSupplyAcceptance(context.Context, factualapi.FactualReadContext, int64) (factualapi.SupplyAcceptance, error)
	ListSupplyAcceptanceItems(context.Context, factualapi.FactualReadContext, int64, factualapi.PageRequest) (factualapi.Page[factualapi.SupplyAcceptanceItem], error)
}

type APIReads struct{ API factualapi.API }

func (a APIReads) GetAccountDemandSeries(ctx context.Context, c factualapi.FactualReadContext, sp factualapi.StandardPeriod, ep *factualapi.Period) ([]factualapi.DemandPoint, error) {
	return a.API.GetAccountDemandSeries(ctx, c, sp, ep)
}
func (a APIReads) GetCategoryDemandSeries(ctx context.Context, c factualapi.FactualReadContext, id int64, sp factualapi.StandardPeriod, ep *factualapi.Period) ([]factualapi.DemandPoint, error) {
	return a.API.GetCategoryDemandSeries(ctx, c, id, sp, ep)
}
func (a APIReads) BatchGetSKUDemandSeries(ctx context.Context, c factualapi.FactualReadContext, ids []int64, sp factualapi.StandardPeriod, ep *factualapi.Period) (factualapi.BatchSKUDemandSeriesResult, error) {
	return a.API.BatchGetSKUDemandSeries(ctx, c, ids, sp, ep)
}
func (a APIReads) GetAccountDemand(ctx context.Context, c factualapi.FactualReadContext, sp factualapi.StandardPeriod, ep *factualapi.Period) (factualapi.Demand, error) {
	return a.API.GetAccountDemand(ctx, c, sp, ep)
}
func (a APIReads) GetSKUDemand(ctx context.Context, c factualapi.FactualReadContext, id int64, sp factualapi.StandardPeriod, ep *factualapi.Period) (factualapi.SKUDemand, error) {
	return a.API.GetSKUDemand(ctx, c, id, sp, ep)
}
func (a APIReads) GetCategoryDemand(ctx context.Context, c factualapi.FactualReadContext, id *int64, sp factualapi.StandardPeriod, ep *factualapi.Period) (factualapi.CategoryDemand, error) {
	return a.API.GetCategoryDemand(ctx, c, id, sp, ep)
}
func (a APIReads) ListSKUDemand(ctx context.Context, c factualapi.FactualReadContext, sp factualapi.StandardPeriod, ep *factualapi.Period, p factualapi.PageRequest) (factualapi.Page[factualapi.SKUDemand], error) {
	return a.API.ListSKUDemand(ctx, c, sp, ep, p)
}
func (a APIReads) ListCategoryDemand(ctx context.Context, c factualapi.FactualReadContext, sp factualapi.StandardPeriod, ep *factualapi.Period, p factualapi.PageRequest) (factualapi.Page[factualapi.CategoryDemand], error) {
	return a.API.ListCategoryDemand(ctx, c, sp, ep, p)
}
func (a APIReads) BatchGetSKUTrendContext(ctx context.Context, c factualapi.FactualReadContext, ids []int64) (factualapi.BatchSKUTrendContextResult, error) {
	return a.API.BatchGetSKUTrendContext(ctx, c, ids)
}
func (a APIReads) ListLifecycleEvents(ctx context.Context, c factualapi.FactualReadContext, f factualapi.LifecycleEventFilter, p factualapi.PageRequest) (factualapi.Page[factualapi.LifecycleEvent], error) {
	return a.API.ListLifecycleEvents(ctx, c, f, p)
}
func (a APIReads) ListReturns(ctx context.Context, c factualapi.FactualReadContext, f factualapi.ReturnFilter, p factualapi.PageRequest) (factualapi.Page[factualapi.Return], error) {
	return a.API.ListReturns(ctx, c, f, p)
}
func (a APIReads) GetSKUStockHistory(ctx context.Context, c factualapi.FactualReadContext, id int64, p factualapi.PageRequest) (factualapi.Page[factualapi.SKUStock], error) {
	return a.API.GetSKUStockHistory(ctx, c, id, p)
}
func (a APIReads) BatchGetSKUStock(ctx context.Context, c factualapi.FactualReadContext, ids []int64) (factualapi.BatchSKUStockResult, error) {
	return a.API.BatchGetSKUStock(ctx, c, ids)
}
func (a APIReads) GetStockLocationBreakdown(ctx context.Context, c factualapi.FactualReadContext, id int64) ([]factualapi.StockPosition, error) {
	return a.API.GetStockLocationBreakdown(ctx, c, id)
}
func (a APIReads) ListSKUStock(ctx context.Context, c factualapi.FactualReadContext, f factualapi.StockFilter, p factualapi.PageRequest) (factualapi.Page[factualapi.SKUStock], error) {
	return a.API.ListSKUStock(ctx, c, f, p)
}
func (a APIReads) GetSKUPriceHistory(ctx context.Context, c factualapi.FactualReadContext, id int64, p factualapi.PageRequest) (factualapi.Page[factualapi.SKUPrice], error) {
	return a.API.GetSKUPriceHistory(ctx, c, id, p)
}
func (a APIReads) GetCampaignSeries(ctx context.Context, c factualapi.FactualReadContext, id int64, sp factualapi.StandardPeriod, ep *factualapi.Period) ([]factualapi.AdvertisingDailyFact, error) {
	return a.API.GetCampaignSeries(ctx, c, id, sp, ep)
}
func (a APIReads) BatchListCampaignTargetSKUs(ctx context.Context, c factualapi.FactualReadContext, ids []int64, lim int) (factualapi.BatchCampaignTargetSKUResult, error) {
	return a.API.BatchListCampaignTargetSKUs(ctx, c, ids, lim)
}
func (a APIReads) BatchGetSKUAdvertising(ctx context.Context, c factualapi.FactualReadContext, ids []int64, sp factualapi.StandardPeriod, ep *factualapi.Period) (factualapi.BatchSKUAdvertisingResult, error) {
	return a.API.BatchGetSKUAdvertising(ctx, c, ids, sp, ep)
}
func (a APIReads) BatchListSKUCampaignPerformance(ctx context.Context, c factualapi.FactualReadContext, ids []int64, sp factualapi.StandardPeriod, ep *factualapi.Period, lim int) (factualapi.BatchSKUCampaignPerformanceResult, error) {
	return a.API.BatchListSKUCampaignPerformance(ctx, c, ids, sp, ep, lim)
}
func (a APIReads) ListCPOAttribution(ctx context.Context, c factualapi.FactualReadContext, f factualapi.CPOFilter, p factualapi.PageRequest) (factualapi.Page[factualapi.CPOAttribution], error) {
	return a.API.ListCPOAttribution(ctx, c, f, p)
}
func (a APIReads) GetAccountFunnelPeriod(ctx context.Context, c factualapi.FactualReadContext, metric string, sp factualapi.StandardPeriod, ep *factualapi.Period) (factualapi.AccountFunnelPeriod, error) {
	return a.API.GetAccountFunnelPeriod(ctx, c, metric, sp, ep)
}
func (a APIReads) GetCategoryFunnelPeriod(ctx context.Context, c factualapi.FactualReadContext, id int64, metric string, sp factualapi.StandardPeriod, ep *factualapi.Period) (factualapi.CategoryFunnelPeriod, error) {
	return a.API.GetCategoryFunnelPeriod(ctx, c, id, metric, sp, ep)
}
func (a APIReads) GetSKUFunnelSeries(ctx context.Context, c factualapi.FactualReadContext, id int64, metric string, sp factualapi.StandardPeriod, ep *factualapi.Period) ([]factualapi.FunnelPoint, error) {
	return a.API.GetSKUFunnelSeries(ctx, c, id, metric, sp, ep)
}
func (a APIReads) GetFunnelCapabilities(ctx context.Context, c factualapi.FactualReadContext) (factualapi.FunnelCapabilities, error) {
	return a.API.GetFunnelCapabilities(ctx, c)
}
func (a APIReads) GetSearchSummarySeries(ctx context.Context, c factualapi.FactualReadContext, id int64, sp factualapi.StandardPeriod, ep *factualapi.Period) ([]factualapi.SKUSearchSummary, error) {
	return a.API.GetSearchSummarySeries(ctx, c, id, sp, ep)
}
func (a APIReads) BatchGetSKUQueryDetails(ctx context.Context, c factualapi.FactualReadContext, ids []int64, sp factualapi.StandardPeriod, ep *factualapi.Period, lim int) (factualapi.BatchSKUQueryDetailsResult, error) {
	return a.API.BatchGetSKUQueryDetails(ctx, c, ids, sp, ep, lim)
}
func (a APIReads) ListMarketSearchFacts(ctx context.Context, c factualapi.FactualReadContext, p factualapi.PageRequest) (factualapi.Page[factualapi.MarketSearchFact], error) {
	return a.API.ListMarketSearchFacts(ctx, c, p)
}
func (a APIReads) GetFinanceDailyLedger(ctx context.Context, c factualapi.FactualReadContext, sp factualapi.StandardPeriod, ep *factualapi.Period) ([]factualapi.FinanceDailyNet, error) {
	return a.API.GetFinanceDailyLedger(ctx, c, sp, ep)
}
func (a APIReads) ListFinanceAccruals(ctx context.Context, c factualapi.FactualReadContext, f factualapi.FinanceAccrualFilter, p factualapi.PageRequest) (factualapi.Page[factualapi.FinanceAccrual], error) {
	return a.API.ListFinanceAccruals(ctx, c, f, p)
}
func (a APIReads) BatchGetSKUPromotions(ctx context.Context, c factualapi.FactualReadContext, ids []int64, lim int) (factualapi.BatchSKUPromotionsResult, error) {
	return a.API.BatchGetSKUPromotions(ctx, c, ids, lim)
}
func (a APIReads) GetPromotionMembershipHistory(ctx context.Context, c factualapi.FactualReadContext, id int64, f factualapi.PromotionMembershipFilter, p factualapi.PageRequest) (factualapi.Page[factualapi.PromotionMembershipInterval], error) {
	return a.API.GetPromotionMembershipHistory(ctx, c, id, f, p)
}
func (a APIReads) GetPromotionEligibility(ctx context.Context, c factualapi.FactualReadContext, id int64, p factualapi.PageRequest) (factualapi.PromotionEligibility, error) {
	return a.API.GetPromotionEligibility(ctx, c, id, p)
}
func (a APIReads) BatchGetCategoryPromotionSummary(ctx context.Context, c factualapi.FactualReadContext, ids []int64) (factualapi.BatchCategoryPromotionSummaryResult, error) {
	return a.API.BatchGetCategoryPromotionSummary(ctx, c, ids)
}
func (a APIReads) GetPromotion(ctx context.Context, c factualapi.FactualReadContext, id int64) (factualapi.Promotion, error) {
	return a.API.GetPromotion(ctx, c, id)
}
func (a APIReads) ListPromotions(ctx context.Context, c factualapi.FactualReadContext, f factualapi.PromotionFilter, p factualapi.PageRequest) (factualapi.Page[factualapi.Promotion], error) {
	return a.API.ListPromotions(ctx, c, f, p)
}
func (a APIReads) GetRatingHistory(ctx context.Context, c factualapi.FactualReadContext, id int64, p factualapi.PageRequest) (factualapi.Page[factualapi.RatingPeriodFact], error) {
	return a.API.GetRatingHistory(ctx, c, id, p)
}
func (a APIReads) GetRatingThresholds(ctx context.Context, c factualapi.FactualReadContext, id int64) ([]factualapi.RatingThreshold, error) {
	return a.API.GetRatingThresholds(ctx, c, id)
}
func (a APIReads) GetCurrentRatings(ctx context.Context, c factualapi.FactualReadContext) ([]factualapi.AccountRating, error) {
	return a.API.GetCurrentRatings(ctx, c)
}
func (a APIReads) GetFBSErrorIndex(ctx context.Context, c factualapi.FactualReadContext) (factualapi.FBSErrorIndex, error) {
	return a.API.GetFBSErrorIndex(ctx, c)
}
func (a APIReads) ListFBSRatingPostingErrors(ctx context.Context, c factualapi.FactualReadContext, p factualapi.PageRequest) (factualapi.Page[factualapi.FBSRatingPostingError], error) {
	return a.API.ListFBSRatingPostingErrors(ctx, c, p)
}
func (a APIReads) BatchGetSKUInbound(ctx context.Context, c factualapi.FactualReadContext, ids []int64) (factualapi.BatchSKUInboundResult, error) {
	return a.API.BatchGetSKUInbound(ctx, c, ids)
}
func (a APIReads) ListSupplies(ctx context.Context, c factualapi.FactualReadContext, f factualapi.SupplyFilter, p factualapi.PageRequest) (factualapi.Page[factualapi.InboundSupply], error) {
	return a.API.ListSupplies(ctx, c, f, p)
}
func (a APIReads) GetSupplyStateHistory(ctx context.Context, c factualapi.FactualReadContext, id int64, p factualapi.PageRequest) (factualapi.Page[factualapi.SupplyStateObservation], error) {
	return a.API.GetSupplyStateHistory(ctx, c, id, p)
}
func (a APIReads) ListSupplyPlannedItems(ctx context.Context, c factualapi.FactualReadContext, id int64, p factualapi.PageRequest) (factualapi.Page[factualapi.SupplyPlannedItem], error) {
	return a.API.ListSupplyPlannedItems(ctx, c, id, p)
}
func (a APIReads) GetSupplyAcceptance(ctx context.Context, c factualapi.FactualReadContext, id int64) (factualapi.SupplyAcceptance, error) {
	return a.API.GetSupplyAcceptance(ctx, c, id)
}
func (a APIReads) ListSupplyAcceptanceItems(ctx context.Context, c factualapi.FactualReadContext, id int64, p factualapi.PageRequest) (factualapi.Page[factualapi.SupplyAcceptanceItem], error) {
	return a.API.ListSupplyAcceptanceItems(ctx, c, id, p)
}

func flowPeriod90(rc factualapi.FactualReadContext) factualapi.Period {
	d := rc.AnalysisDate()
	return factualapi.Period{From: d.AddDate(0, 0, -89), To: d}
}

func eventPeriod30(rc factualapi.FactualReadContext) factualapi.Period {
	d := rc.AnalysisDate()
	return factualapi.Period{From: d.AddDate(0, 0, -29), To: d}
}

func cutoffRFC(rc factualapi.FactualReadContext) string {
	return rc.CutoffAt().UTC().Format(time.RFC3339)
}
