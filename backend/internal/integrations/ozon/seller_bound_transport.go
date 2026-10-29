package ozon

import (
	"context"
	"fmt"
	"time"

	ozonperf "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon/performance"
)

type AccessTokenProvider interface {
	CanonicalAccessToken(context.Context, int64) (string, error)
}

// SellerBoundTransport is the only authentication boundary used by new
// canonical collectors. It has no Client-ID/API-Key or client-credentials
// fallback: every request obtains one seller-bound private OAuth token.
type SellerBoundTransport struct {
	tokens           AccessTokenProvider
	seller           *Client
	performance      *ozonperf.Client
	analyticsLimiter AnalyticsRequestLimiter
}

func NewSellerBoundTransport(tokens AccessTokenProvider, seller *Client, performance *ozonperf.Client) *SellerBoundTransport {
	if seller == nil {
		seller = NewClient()
	}
	if performance == nil {
		performance = ozonperf.NewClient()
	}
	return &SellerBoundTransport{tokens: tokens, seller: seller, performance: performance, analyticsLimiter: NewDefaultAnalyticsLimiter()}
}

func NewSellerBoundTransportWithAnalyticsLimiter(tokens AccessTokenProvider, seller *Client, limiter AnalyticsRequestLimiter) *SellerBoundTransport {
	t := NewSellerBoundTransport(tokens, seller, nil)
	if limiter != nil {
		t.analyticsLimiter = limiter
	}
	return t
}

func (t *SellerBoundTransport) FetchCanonicalAnalyticsData(ctx context.Context, sellerID int64, request AnalyticsDataRequest) (*CanonicalAnalyticsResponse, error) {
	if err := t.analyticsLimiter.Wait(ctx); err != nil {
		return nil, err
	}
	token, err := t.tokens.CanonicalAccessToken(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	return t.seller.analyticsDataBearer(ctx, token, request)
}

func (t *SellerBoundTransport) ListProducts(ctx context.Context, sellerID int64, request ListProductsRequest) (*TypedResponse[ListProductsResult], error) {
	token, err := t.tokens.CanonicalAccessToken(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	return t.seller.listProductsBearer(ctx, token, request)
}

func (t *SellerBoundTransport) ListProductsInfo(ctx context.Context, sellerID int64, request ProductInfoListRequest) (*TypedResponse[ProductInfoListResult], error) {
	token, err := t.tokens.CanonicalAccessToken(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	return t.seller.listProductsInfoBearer(ctx, token, request)
}

func (t *SellerBoundTransport) GetDescriptionCategoryTree(ctx context.Context, sellerID int64) (*TypedResponse[[]DescriptionCategoryTreeJSONNode], error) {
	token, err := t.tokens.CanonicalAccessToken(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	return t.seller.descriptionCategoryTreeBearer(ctx, token)
}

func (t *SellerBoundTransport) ListCanonicalFBOPostings(ctx context.Context, sellerID int64, request PostingListRequest) (*TypedResponse[PostingListResult], error) {
	token, err := t.tokens.CanonicalAccessToken(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	return t.seller.postingListBearer(ctx, token, CanonicalFBOListPath, request)
}
func (t *SellerBoundTransport) GetCanonicalFBOPosting(ctx context.Context, sellerID int64, request PostingDetailRequest) (*TypedResponse[CanonicalPosting], error) {
	token, err := t.tokens.CanonicalAccessToken(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	return t.seller.postingDetailBearer(ctx, token, CanonicalFBODetailPath, request)
}
func (t *SellerBoundTransport) ListCanonicalFBSPostings(ctx context.Context, sellerID int64, request PostingListRequest) (*TypedResponse[PostingListResult], error) {
	token, err := t.tokens.CanonicalAccessToken(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	return t.seller.postingListBearer(ctx, token, CanonicalFBSListPath, request)
}
func (t *SellerBoundTransport) GetCanonicalFBSPosting(ctx context.Context, sellerID int64, request PostingDetailRequest) (*TypedResponse[CanonicalPosting], error) {
	token, err := t.tokens.CanonicalAccessToken(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	return t.seller.postingDetailBearer(ctx, token, CanonicalFBSDetailPath, request)
}
func (t *SellerBoundTransport) ListCanonicalReturns(ctx context.Context, sellerID int64, request ReturnsListRequest) (*TypedResponse[ReturnsListResult], error) {
	token, err := t.tokens.CanonicalAccessToken(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	return t.seller.returnsListBearer(ctx, token, request)
}

func (t *SellerBoundTransport) ListPerformanceCampaigns(ctx context.Context, sellerID int64, request ozonperf.ListCampaignsRequest) (*ozonperf.TypedResponse[ozonperf.ListCampaignsResult], error) {
	token, err := t.tokens.CanonicalAccessToken(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	return t.performance.ListCampaigns(ctx, "", token, request)
}

func (t *SellerBoundTransport) ListCanonicalStocks(ctx context.Context, sellerID int64, request CanonicalStockRequest) (*TypedResponse[[]CanonicalStockRow], error) {
	token, err := t.tokens.CanonicalAccessToken(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	if err = t.analyticsLimiter.Wait(ctx); err != nil {
		return nil, err
	}
	return t.seller.analyticsStocksBearer(ctx, token, request)
}

func (t *SellerBoundTransport) ListCanonicalPrices(ctx context.Context, sellerID int64, request CanonicalPriceRequest) (*TypedResponse[CanonicalPriceResult], error) {
	token, err := t.tokens.CanonicalAccessToken(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	return t.seller.pricesBearer(ctx, token, request)
}

func (t *SellerBoundTransport) ListFinanceAccrualTypes(ctx context.Context, sellerID int64) (*TypedResponse[FinanceAccrualTypesResult], error) {
	token, err := t.tokens.CanonicalAccessToken(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	return t.seller.financeAccrualTypesBearer(ctx, token)
}

func (t *SellerBoundTransport) ListFinanceAccrualsByDay(ctx context.Context, sellerID int64, request FinanceAccrualRequest) (*TypedResponse[FinanceAccrualResult], error) {
	token, err := t.tokens.CanonicalAccessToken(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	return t.seller.financeAccrualBearer(ctx, token, FinanceAccrualByDayPath, request)
}

func (t *SellerBoundTransport) ListFinanceAccrualPostings(ctx context.Context, sellerID int64, request FinanceAccrualRequest) (*TypedResponse[FinanceAccrualResult], error) {
	token, err := t.tokens.CanonicalAccessToken(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	return t.seller.financeAccrualBearer(ctx, token, FinanceAccrualPostingsPath, request)
}

func (t *SellerBoundTransport) GetFinanceBalance(ctx context.Context, sellerID int64) (*TypedResponse[FinanceBalanceResult], error) {
	token, err := t.tokens.CanonicalAccessToken(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	return t.seller.financeBalanceBearer(ctx, token)
}

func (t *SellerBoundTransport) ListPromotionActions(ctx context.Context, sellerID int64) (*TypedResponse[[]PromotionAction], error) {
	token, err := t.tokens.CanonicalAccessToken(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	return t.seller.promotionActionsBearer(ctx, token, ActionsListPath)
}

func (t *SellerBoundTransport) ListPromotionCandidates(ctx context.Context, sellerID int64, request PromotionProductRequest) (*TypedResponse[PromotionProductsResult], error) {
	token, err := t.tokens.CanonicalAccessToken(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	return t.seller.promotionProductsBearer(ctx, token, ActionCandidatesPath, request)
}

func (t *SellerBoundTransport) ListPromotionProducts(ctx context.Context, sellerID int64, request PromotionProductRequest) (*TypedResponse[PromotionProductsResult], error) {
	token, err := t.tokens.CanonicalAccessToken(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	return t.seller.promotionProductsBearer(ctx, token, ActionProductsPath, request)
}

func (t *SellerBoundTransport) ListSellerPromotionActions(ctx context.Context, sellerID int64) (*TypedResponse[[]PromotionAction], error) {
	token, err := t.tokens.CanonicalAccessToken(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	return t.seller.promotionActionsBearer(ctx, token, SellerActionsListPath)
}

func (t *SellerBoundTransport) ListSellerPromotionCandidates(ctx context.Context, sellerID int64, request PromotionProductRequest) (*TypedResponse[PromotionProductsResult], error) {
	token, err := t.tokens.CanonicalAccessToken(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	return t.seller.promotionProductsBearer(ctx, token, SellerActionCandidatesPath, request)
}

func (t *SellerBoundTransport) ListSellerPromotionProducts(ctx context.Context, sellerID int64, request PromotionProductRequest) (*TypedResponse[PromotionProductsResult], error) {
	token, err := t.tokens.CanonicalAccessToken(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	return t.seller.promotionProductsBearer(ctx, token, SellerActionProductsPath, request)
}

func (t *SellerBoundTransport) ListSupplyOrders(ctx context.Context, sellerID int64, request SupplyOrderListRequest) (*TypedResponse[SupplyOrderListResult], error) {
	token, err := t.tokens.CanonicalAccessToken(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	return t.seller.supplyOrderListBearer(ctx, token, request)
}
func (t *SellerBoundTransport) GetSupplyOrder(ctx context.Context, sellerID int64, request SupplyOrderRequest) (*TypedResponse[SupplyOrder], error) {
	token, err := t.tokens.CanonicalAccessToken(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	return t.seller.supplyOrderBearer(ctx, token, SupplyOrderGetPath, request)
}
func (t *SellerBoundTransport) GetSupplyOrderDetails(ctx context.Context, sellerID int64, request SupplyOrderRequest) (*TypedResponse[SupplyOrder], error) {
	token, err := t.tokens.CanonicalAccessToken(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	return t.seller.supplyOrderBearer(ctx, token, SupplyOrderDetailsPath, request)
}
func (t *SellerBoundTransport) GetSupplyBundle(ctx context.Context, sellerID int64, request SupplyBundleRequest) (*TypedResponse[SupplyBundleResult], error) {
	token, err := t.tokens.CanonicalAccessToken(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	return t.seller.supplyBundleBearer(ctx, token, request)
}
func (t *SellerBoundTransport) GetSupplyActSummary(ctx context.Context, sellerID int64, request SupplyActRequest) (*TypedResponse[SupplyActSummary], error) {
	token, err := t.tokens.CanonicalAccessToken(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	return t.seller.supplyActSummaryBearer(ctx, token, request)
}
func (t *SellerBoundTransport) GetSupplyActProducts(ctx context.Context, sellerID int64, request SupplyActRequest) (*TypedResponse[SupplyActProductResult], error) {
	token, err := t.tokens.CanonicalAccessToken(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	return t.seller.supplyActProductBearer(ctx, token, request)
}

func (t *SellerBoundTransport) ListProductQueries(ctx context.Context, sellerID int64, request ProductQueriesRequest) (*TypedResponse[SearchQueryResult], error) {
	token, err := t.tokens.CanonicalAccessToken(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	if len(request.SKUs) == 0 {
		return nil, fmt.Errorf("product-queries requires skus")
	}
	if len(request.SKUs) > ProductQueriesSKULimit {
		return nil, fmt.Errorf("product-queries skus exceed %d", ProductQueriesSKULimit)
	}
	if err = t.analyticsLimiter.Wait(ctx); err != nil {
		return nil, err
	}
	return t.seller.productQueriesBearer(ctx, token, request)
}

func (t *SellerBoundTransport) ListProductQueryDetails(ctx context.Context, sellerID int64, request ProductQueryDetailsRequest) (*TypedResponse[SearchQueryResult], error) {
	token, err := t.tokens.CanonicalAccessToken(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	if len(request.SKUs) == 0 {
		return nil, fmt.Errorf("product-queries/details requires skus")
	}
	if len(request.SKUs) > ProductQueryDetailsSKULimit {
		return nil, fmt.Errorf("product-queries/details skus exceed %d", ProductQueryDetailsSKULimit)
	}
	// limit_by_sku is max query rows PER SKU — not the number of requested SKUs.
	if request.LimitBySKU <= 0 || request.LimitBySKU > ProductQueryDetailsLimitMax {
		return nil, fmt.Errorf("search details limit_by_sku exceeds %d", ProductQueryDetailsLimitMax)
	}
	if request.PageSize > ProductQueryDetailsPageMax {
		return nil, fmt.Errorf("search details page_size exceeds %d", ProductQueryDetailsPageMax)
	}
	if err = t.analyticsLimiter.Wait(ctx); err != nil {
		return nil, err
	}
	return t.seller.productQueryDetailsBearer(ctx, token, request)
}

func (t *SellerBoundTransport) ListTopSearchQueries(ctx context.Context, sellerID int64, request MarketSearchRequest) (*TypedResponse[SearchQueryResult], error) {
	token, err := t.tokens.CanonicalAccessToken(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	return t.seller.marketSearchBearer(ctx, token, SearchQueriesTopPath, request)
}

func (t *SellerBoundTransport) SearchQueryText(ctx context.Context, sellerID int64, request MarketSearchRequest) (*TypedResponse[SearchQueryResult], error) {
	token, err := t.tokens.CanonicalAccessToken(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	return t.seller.marketSearchBearer(ctx, token, SearchQueriesTextPath, request)
}

func (t *SellerBoundTransport) GetAccountRatingSummary(ctx context.Context, sellerID int64, request AccountHealthRequest) (*TypedResponse[AccountHealthDocument], error) {
	token, err := t.tokens.CanonicalAccessToken(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	return t.seller.accountHealthBearer(ctx, token, RatingSummaryPath, request)
}

func (t *SellerBoundTransport) GetAccountRatingHistory(ctx context.Context, sellerID int64, request AccountHealthRequest) (*TypedResponse[AccountHealthDocument], error) {
	token, err := t.tokens.CanonicalAccessToken(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	return t.seller.accountHealthBearer(ctx, token, RatingHistoryPath, request)
}

func (t *SellerBoundTransport) GetFBSRatingIndex(ctx context.Context, sellerID int64, request AccountHealthRequest) (*TypedResponse[AccountHealthDocument], error) {
	token, err := t.tokens.CanonicalAccessToken(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	return t.seller.accountHealthBearer(ctx, token, FBSIndexInfoPath, request)
}

func (t *SellerBoundTransport) ListFBSPostingErrors(ctx context.Context, sellerID int64, request AccountHealthRequest) (*TypedResponse[AccountHealthDocument], error) {
	token, err := t.tokens.CanonicalAccessToken(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	return t.seller.accountHealthBearer(ctx, token, FBSPostingErrorListPath, request)
}

func (t *SellerBoundTransport) GetPerformanceDailyStatistics(ctx context.Context, sellerID int64, request ozonperf.CampaignStatisticsRequest) (*ozonperf.TypedResponse[ozonperf.CampaignStatisticsResult], error) {
	token, err := t.tokens.CanonicalAccessToken(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	return t.performance.GetDailyCampaignStatisticsJSON(ctx, "", token, request)
}

func (t *SellerBoundTransport) GetPerformanceCampaignProducts(ctx context.Context, sellerID int64, request ozonperf.CampaignProductsRequest) (*ozonperf.TypedResponse[ozonperf.CampaignProductsResult], error) {
	token, err := t.tokens.CanonicalAccessToken(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	return t.performance.GetCampaignProductsForSKU(ctx, "", token, request)
}

func (t *SellerBoundTransport) GetPerformanceCampaignProductStatistics(ctx context.Context, sellerID int64, request ozonperf.CampaignStatisticsRequest) (*ozonperf.TypedResponse[ozonperf.CampaignProductStatisticsResult], error) {
	token, err := t.tokens.CanonicalAccessToken(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	return t.performance.GetCampaignProductStatisticsJSON(ctx, "", token, request)
}

func (t *SellerBoundTransport) GetPerformanceExpenseStatistics(ctx context.Context, sellerID int64, request ozonperf.CampaignStatisticsRequest) (*ozonperf.TypedResponse[ozonperf.ExpenseStatisticsResult], error) {
	token, err := t.tokens.CanonicalAccessToken(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	return t.performance.GetExpenseStatisticsJSON(ctx, "", token, request)
}

func (t *SellerBoundTransport) SubmitPerformanceCPOOrdersReport(ctx context.Context, sellerID int64, from, to time.Time) (*ozonperf.TypedResponse[ozonperf.ReportSubmitResult], error) {
	token, err := t.tokens.CanonicalAccessToken(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	return t.performance.SubmitCPOOrdersReport(ctx, "", token, from, to)
}

func (t *SellerBoundTransport) GetPerformanceStatisticsReportStatus(ctx context.Context, sellerID int64, reportUUID string) (*ozonperf.TypedResponse[ozonperf.ReportStatus], error) {
	token, err := t.tokens.CanonicalAccessToken(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	return t.performance.GetStatisticsReportStatus(ctx, "", token, reportUUID)
}

func (t *SellerBoundTransport) DownloadPerformanceStatisticsReport(ctx context.Context, sellerID int64, reportUUID string) (*ozonperf.RawResponse, error) {
	token, err := t.tokens.CanonicalAccessToken(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	return t.performance.DownloadStatisticsReport(ctx, "", token, reportUUID)
}
