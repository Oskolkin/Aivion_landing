package adsync

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon/performance"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/rawpayloads"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ozonIntegration is the subset of Ozon integration used by advertising sync.
type ozonIntegration interface {
	GetDecryptedCredentials(ctx context.Context, sellerAccountID int64) (ozon.DecryptedCredentials, error)
	GetDecryptedPerformanceBearerToken(ctx context.Context, sellerAccountID int64) (string, error)
}

type Service struct {
	queries         *dbgen.Queries
	ozonConn        ozonIntegration
	performance     *performance.Client
	rawPayloads     *rawpayloads.Service
	initialLookback time.Duration
}

type RunInput struct {
	SellerAccountID int64
	ImportJobID     int64
	SourceCursor    string
}

type RunResult struct {
	RecordsReceived int32
	RecordsImported int32
	RecordsFailed   int32
	NextCursorValue string
}

const (
	campaignTypeSKU         = "SKU"
	campaignTypeSearchPromo = "SEARCH_PROMO"
	campaignTypeBanner      = "BANNER"
	campaignTypeVideoBanner = "VIDEO_BANNER"
)

func NewService(db *pgxpool.Pool, ozonConn ozonIntegration, rawPayloads *rawpayloads.Service) *Service {
	return &Service{
		queries:         dbgen.New(db),
		ozonConn:        ozonConn,
		performance:     performance.NewClient(),
		rawPayloads:     rawPayloads,
		initialLookback: 30 * 24 * time.Hour,
	}
}

func (s *Service) Run(ctx context.Context, input RunInput) (RunResult, error) {
	creds, err := s.ozonConn.GetDecryptedCredentials(ctx, input.SellerAccountID)
	if err != nil {
		return RunResult{}, fmt.Errorf("get decrypted ozon credentials: %w", err)
	}
	bearerToken, err := s.ozonConn.GetDecryptedPerformanceBearerToken(ctx, input.SellerAccountID)
	if err != nil {
		if errors.Is(err, ozon.ErrPerformanceTokenNotConfigured) {
			return RunResult{}, fmt.Errorf("ads sync: %w", ozon.ErrPerformanceTokenNotConfigured)
		}
		return RunResult{}, fmt.Errorf("get decrypted performance bearer token: %w", err)
	}
	if strings.TrimSpace(bearerToken) == "" {
		return RunResult{}, fmt.Errorf("ads sync: %w", ozon.ErrPerformanceTokenNotConfigured)
	}

	from, to, err := s.resolveWindow(input.SourceCursor)
	if err != nil {
		return RunResult{}, fmt.Errorf("resolve advertising window: %w", err)
	}

	campaigns, campaignRowsReceived, err := s.loadCampaigns(ctx, creds.ClientID, bearerToken)
	if err != nil {
		return RunResult{}, err
	}

	received := campaignRowsReceived
	imported := int32(0)

	campaignIDs := make([]int64, 0, len(campaigns))
	for _, campaign := range campaigns {
		campaignType := normalizeCampaignType(campaign.CampaignType)
		if _, err := s.queries.UpsertAdCampaign(ctx, dbgen.UpsertAdCampaignParams{
			SellerAccountID:    input.SellerAccountID,
			CampaignExternalID: campaign.CampaignExternalID,
			CampaignName:       fallbackCampaignName(campaign.CampaignName, campaign.CampaignExternalID),
			CampaignType:       nullableText(campaignType),
			PlacementType:      nullableText(campaign.PlacementType),
			Status:             nullableText(campaign.Status),
			BudgetAmount:       nullableNumeric(campaign.BudgetAmount),
			BudgetDaily:        nullableNumeric(campaign.BudgetDaily),
			PaymentType:        nullableText(campaign.PaymentType),
			RawAttributes:      campaign.Raw,
		}); err != nil {
			return RunResult{}, fmt.Errorf("upsert ad campaign=%d: %w", campaign.CampaignExternalID, err)
		}
		campaignIDs = append(campaignIDs, campaign.CampaignExternalID)
		imported++
	}

	metricsImported, metricsReceived, err := s.importCampaignMetrics(
		ctx,
		creds.ClientID,
		bearerToken,
		input,
		campaignIDs,
		from,
		to,
	)
	if err != nil {
		return RunResult{}, err
	}
	received += metricsReceived
	imported += metricsImported

	linksImported, linksReceived, err := s.importCampaignProducts(
		ctx,
		creds.ClientID,
		bearerToken,
		input,
		campaigns,
	)
	if err != nil {
		return RunResult{}, err
	}
	received += linksReceived
	imported += linksImported

	return RunResult{
		RecordsReceived: received,
		RecordsImported: imported,
		RecordsFailed:   0,
		NextCursorValue: to.Format(time.RFC3339),
	}, nil
}

func (s *Service) loadCampaigns(ctx context.Context, clientID string, bearerToken string) ([]performance.Campaign, int32, error) {
	const pageSize = int64(200)
	var all []performance.Campaign
	var received int32

	for page := int64(1); ; page++ {
		resp, err := s.performance.ListCampaigns(
			ctx,
			clientID,
			bearerToken,
			performance.ListCampaignsRequest{
				Page:     page,
				PageSize: pageSize,
			},
		)
		if err != nil {
			return nil, 0, fmt.Errorf("fetch ad campaigns page=%d: %w", page, err)
		}
		all = append(all, resp.Data.Items...)
		received += int32(len(resp.Data.Items))
		if len(resp.Data.Items) < int(pageSize) {
			break
		}
	}

	return all, received, nil
}

func (s *Service) importCampaignMetrics(
	ctx context.Context,
	clientID string,
	bearerToken string,
	input RunInput,
	campaignIDs []int64,
	from time.Time,
	to time.Time,
) (int32, int32, error) {
	if len(campaignIDs) == 0 {
		return 0, 0, nil
	}

	resp, err := s.performance.GetDailyCampaignStatisticsJSON(
		ctx,
		clientID,
		bearerToken,
		performance.CampaignStatisticsRequest{
			CampaignIDs: campaignIDs,
			DateFrom:    from.Format("2006-01-02"),
			DateTo:      to.Format("2006-01-02"),
		},
	)
	if err != nil {
		return 0, 0, fmt.Errorf("fetch ad daily metrics: %w", err)
	}
	if err := s.saveRaw(ctx, input, "ozon.performance.statistics.daily.json", "ads:daily-stats-json", resp.Raw); err != nil {
		return 0, 0, err
	}

	received := int32(len(resp.Data.Items))
	imported := int32(0)
	for _, metric := range resp.Data.Items {
		if _, err := s.queries.UpsertAdMetricDaily(ctx, dbgen.UpsertAdMetricDailyParams{
			SellerAccountID:    input.SellerAccountID,
			CampaignExternalID: metric.CampaignExternalID,
			MetricDate:         dateValue(metric.MetricDate),
			CampaignName:       nullableText(metric.CampaignName),
			Impressions:        metric.Impressions,
			Clicks:             metric.Clicks,
			Spend:              nullableNumeric(metric.Spend),
			OrdersCount:        int32(metric.OrdersCount),
			Revenue:            nullableNumeric(metric.Revenue),
			RawAttributes:      metric.Raw,
		}); err != nil {
			return 0, 0, fmt.Errorf(
				"upsert ad metric campaign=%d date=%s: %w",
				metric.CampaignExternalID,
				metric.MetricDate.Format("2006-01-02"),
				err,
			)
		}
		imported++
	}
	return imported, received, nil
}

func (s *Service) importCampaignProducts(
	ctx context.Context,
	clientID string,
	bearerToken string,
	input RunInput,
	campaigns []performance.Campaign,
) (int32, int32, error) {
	var received int32
	var imported int32

	for _, campaign := range campaigns {
		var links []performance.CampaignPromotedProduct
		var fetchErr error

		switch normalizeCampaignType(campaign.CampaignType) {
		case campaignTypeSKU:
			links, fetchErr = s.fetchSKUCampaignProducts(ctx, clientID, bearerToken, input, campaign.CampaignExternalID)
		case campaignTypeSearchPromo:
			continue
		case campaignTypeBanner, campaignTypeVideoBanner:
			// For banner-oriented campaigns objects are not reliably SKU-linked.
			continue
		default:
			continue
		}
		if fetchErr != nil {
			return 0, 0, fetchErr
		}
		if normalizeCampaignType(campaign.CampaignType) == campaignTypeSKU && len(links) == 0 {
			return 0, 0, fmt.Errorf("sku campaign=%d has no product links", campaign.CampaignExternalID)
		}

		received += int32(len(links))
		for _, link := range links {
			if link.OzonProductID == 0 {
				continue
			}
			if _, err := s.queries.UpsertAdCampaignSKU(ctx, dbgen.UpsertAdCampaignSKUParams{
				SellerAccountID:    input.SellerAccountID,
				CampaignExternalID: link.CampaignExternalID,
				OzonProductID:      link.OzonProductID,
				OfferID:            pgtype.Text{Valid: false},
				Sku:                nullableInt64(link.SKU),
				IsActive:           link.IsActive,
				Status:             nullableText(link.Status),
				RawAttributes:      link.Raw,
			}); err != nil {
				return 0, 0, fmt.Errorf(
					"upsert ad campaign sku campaign=%d product=%d: %w",
					link.CampaignExternalID,
					link.OzonProductID,
					err,
				)
			}
			imported++
		}
	}

	searchPromoLinks, err := s.fetchSearchPromoCampaignProducts(ctx, clientID, bearerToken, input)
	if err != nil {
		return 0, 0, err
	}
	received += int32(len(searchPromoLinks))
	for _, link := range searchPromoLinks {
		if link.OzonProductID == 0 || link.CampaignExternalID == 0 {
			continue
		}
		if _, err := s.queries.UpsertAdCampaignSKU(ctx, dbgen.UpsertAdCampaignSKUParams{
			SellerAccountID:    input.SellerAccountID,
			CampaignExternalID: link.CampaignExternalID,
			OzonProductID:      link.OzonProductID,
			OfferID:            pgtype.Text{Valid: false},
			Sku:                nullableInt64(link.SKU),
			IsActive:           link.IsActive,
			Status:             nullableText(link.Status),
			RawAttributes:      link.Raw,
		}); err != nil {
			return 0, 0, fmt.Errorf(
				"upsert search_promo campaign sku campaign=%d product=%d: %w",
				link.CampaignExternalID,
				link.OzonProductID,
				err,
			)
		}
		imported++
	}

	return imported, received, nil
}

func (s *Service) fetchSKUCampaignProducts(
	ctx context.Context,
	clientID string,
	bearerToken string,
	input RunInput,
	campaignID int64,
) ([]performance.CampaignPromotedProduct, error) {
	const pageSize = int64(500)
	all := make([]performance.CampaignPromotedProduct, 0)

	for page := int64(1); ; page++ {
		resp, err := s.performance.GetCampaignProductsForSKU(
			ctx,
			clientID,
			bearerToken,
			performance.CampaignProductsRequest{
				CampaignID: campaignID,
				Page:       page,
				PageSize:   pageSize,
			},
		)
		if err != nil {
			return nil, fmt.Errorf("fetch sku campaign products campaign=%d page=%d: %w", campaignID, page, err)
		}
		if err := s.saveRaw(
			ctx,
			input,
			"ozon.performance.campaign.v2.products",
			fmt.Sprintf("ads:campaign-v2-products:%d:page:%d", campaignID, page),
			resp.Raw,
		); err != nil {
			return nil, err
		}
		all = append(all, resp.Data.Items...)
		if len(resp.Data.Items) < int(pageSize) {
			break
		}
	}

	if len(all) > 0 {
		return all, nil
	}

	// Fallback for SKU campaign only.
	resp, err := s.performance.GetCampaignObjects(ctx, clientID, bearerToken, campaignID)
	if err != nil {
		return nil, fmt.Errorf("fetch campaign objects fallback campaign=%d: %w", campaignID, err)
	}
	if err := s.saveRaw(
		ctx,
		input,
		"ozon.performance.campaign.objects",
		fmt.Sprintf("ads:campaign-objects:%d", campaignID),
		resp.Raw,
	); err != nil {
		return nil, err
	}
	return resp.Data.Items, nil
}

func (s *Service) fetchSearchPromoCampaignProducts(
	ctx context.Context,
	clientID string,
	bearerToken string,
	input RunInput,
) ([]performance.CampaignPromotedProduct, error) {
	const pageSize = int64(500)
	all := make([]performance.CampaignPromotedProduct, 0)
	for page := int64(1); ; page++ {
		resp, err := s.performance.GetSearchPromoCampaignProducts(
			ctx,
			clientID,
			bearerToken,
			performance.SearchPromoProductsRequest{
				Page:     page,
				PageSize: pageSize,
			},
		)
		if err != nil {
			return nil, fmt.Errorf("fetch search promo products page=%d: %w", page, err)
		}
		if err := s.saveRaw(
			ctx,
			input,
			"ozon.performance.campaign.search_promo.v2.products",
			fmt.Sprintf("ads:search-promo-products:page:%d", page),
			resp.Raw,
		); err != nil {
			return nil, err
		}
		all = append(all, resp.Data.Items...)
		if len(resp.Data.Items) < int(pageSize) {
			break
		}
	}
	return all, nil
}

func (s *Service) resolveWindow(sourceCursor string) (time.Time, time.Time, error) {
	upperBound := time.Now().UTC().Truncate(time.Second)
	if sourceCursor == "" {
		return upperBound.Add(-s.initialLookback), upperBound, nil
	}
	since, err := time.Parse(time.RFC3339, sourceCursor)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return since.UTC(), upperBound, nil
}

func (s *Service) saveRaw(
	ctx context.Context,
	input RunInput,
	source string,
	requestKey string,
	body []byte,
) error {
	if s.rawPayloads == nil || len(body) == 0 {
		return nil
	}
	if _, err := s.rawPayloads.Save(ctx, rawpayloads.SaveInput{
		SellerAccountID: input.SellerAccountID,
		ImportJobID:     input.ImportJobID,
		Domain:          "ads",
		Source:          source,
		RequestKey:      requestKey,
		Body:            body,
	}); err != nil {
		return fmt.Errorf("save raw advertising payload: %w", err)
	}
	return nil
}

func fallbackCampaignName(name string, campaignID int64) string {
	if name == "" {
		return fmt.Sprintf("campaign_%d", campaignID)
	}
	return name
}

func nullableText(v string) pgtype.Text {
	if v == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: v, Valid: true}
}

func nullableInt64(v int64) pgtype.Int8 {
	if v == 0 {
		return pgtype.Int8{Valid: false}
	}
	return pgtype.Int8{Int64: v, Valid: true}
}

func nullableNumeric(v string) pgtype.Numeric {
	if v == "" {
		return pgtype.Numeric{Valid: false}
	}
	var n pgtype.Numeric
	if err := n.Scan(v); err != nil {
		return pgtype.Numeric{Valid: false}
	}
	return n
}

func dateValue(v time.Time) pgtype.Date {
	normalized := time.Date(v.Year(), v.Month(), v.Day(), 0, 0, 0, 0, time.UTC)
	return pgtype.Date{Time: normalized, Valid: true}
}

func normalizeCampaignType(v string) string {
	return strings.ToUpper(strings.TrimSpace(v))
}
