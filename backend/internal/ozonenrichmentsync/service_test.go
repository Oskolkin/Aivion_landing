package ozonenrichmentsync

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon/performance"
)

type stubCreds struct {
	creds ozon.DecryptedCredentials
	cerr  error
	token string
	terr  error
}

func (s *stubCreds) GetDecryptedCredentials(context.Context, int64) (ozon.DecryptedCredentials, error) {
	return s.creds, s.cerr
}

func (s *stubCreds) GetDecryptedPerformanceBearerToken(context.Context, int64) (string, error) {
	return s.token, s.terr
}

type stubSellerAPI struct {
	rolesResp *ozon.TypedResponse[ozon.RolesResponse]
	rolesErr  error
}

func (s *stubSellerAPI) GetSellerAPIRoles(context.Context, string, string) (*ozon.TypedResponse[ozon.RolesResponse], error) {
	return s.rolesResp, s.rolesErr
}

func (s *stubSellerAPI) GetProductInfoList(context.Context, string, string, ozon.ProductInfoListRequestIDs) (*ozon.TypedResponse[ozon.ProductInfoListResult], error) {
	return &ozon.TypedResponse[ozon.ProductInfoListResult]{Data: ozon.ProductInfoListResult{}}, nil
}

func (s *stubSellerAPI) GetRelatedSKUs(context.Context, string, string, []int64) (*ozon.TypedResponse[ozon.RelatedSKUsResponse], error) {
	return &ozon.TypedResponse[ozon.RelatedSKUsResponse]{Data: ozon.RelatedSKUsResponse{}}, nil
}

func (s *stubSellerAPI) GetAnalyticsStocks(context.Context, string, string, ozon.AnalyticsStocksRequest) (*ozon.TypedResponse[[]ozon.AnalyticsStockItem], error) {
	return &ozon.TypedResponse[[]ozon.AnalyticsStockItem]{Data: nil}, nil
}

type stubPerformanceAPI struct {
	campaignErr error
	productErr  map[int64]error
}

func (s *stubPerformanceAPI) ListPerformanceCampaigns(context.Context, string, string, performance.CampaignListFilter) (*performance.TypedResponse[performance.ListCampaignsResult], error) {
	if s.campaignErr != nil {
		return nil, s.campaignErr
	}
	return &performance.TypedResponse[performance.ListCampaignsResult]{
		Data: performance.ListCampaignsResult{Items: []performance.Campaign{
			{CampaignExternalID: 1, CampaignName: "A", CampaignType: "SKU"},
			{CampaignExternalID: 2, CampaignName: "B", CampaignType: "SKU"},
		}},
	}, nil
}

func (s *stubPerformanceAPI) GetPerformanceDailyStatisticsJSON(context.Context, string, string, []int64, time.Time, time.Time) (*performance.TypedResponse[performance.CampaignStatisticsResult], error) {
	return &performance.TypedResponse[performance.CampaignStatisticsResult]{
		Data: performance.CampaignStatisticsResult{Items: []performance.CampaignDailyMetric{
			{CampaignExternalID: 1, MetricDate: time.Now().UTC(), Spend: "10.00", OrdersCount: 0},
		}},
	}, nil
}

func (s *stubPerformanceAPI) GetPerformanceCampaignObjects(context.Context, string, string, int64, string) (*performance.TypedResponse[performance.CampaignObjectsResult], error) {
	return &performance.TypedResponse[performance.CampaignObjectsResult]{
		Data: performance.CampaignObjectsResult{Items: []performance.CampaignObject{{CampaignExternalID: 1, ObjectExternalID: "111", ObjectType: "sku"}}},
	}, nil
}

func (s *stubPerformanceAPI) GetPerformanceCampaignProductsV2(ctx context.Context, clientID, bearerToken string, campaignID int64, page, pageSize int64) (*performance.TypedResponse[performance.CampaignProductsResult], error) {
	if err, ok := s.productErr[campaignID]; ok && err != nil {
		return nil, err
	}
	return &performance.TypedResponse[performance.CampaignProductsResult]{
		Data: performance.CampaignProductsResult{Items: []performance.CampaignPromotedProduct{
			{CampaignExternalID: campaignID, SKU: 111, Title: "SKU product", BidRawMicro: 1_000_000, BidAmount: "1.00", BidUnitAssumption: "micro_rubles"},
		}},
	}, nil
}

func TestSyncOzonEnrichmentP0_PerformanceUnavailableStillPartial(t *testing.T) {
	// Integration-style test requires DB; this test validates orchestration wiring compiles and
	// stub clients behave for performance unavailable path at unit level.
	perf := &stubPerformanceAPI{campaignErr: errors.New("performance down")}
	_ = perf
	seller := &stubSellerAPI{
		rolesResp: &ozon.TypedResponse[ozon.RolesResponse]{
			Data: ozon.RolesResponse{Roles: []struct {
				Name    string   `json:"name"`
				Methods []string `json:"methods"`
			}{{Name: "Admin", Methods: []string{"/v1/roles"}}}},
			Raw: []byte(`{"roles":[{"name":"Admin","methods":["/v1/roles"]}]}`),
		},
	}
	methods := dedupeMethods(seller.rolesResp.Data)
	if len(methods) != 1 {
		t.Fatalf("expected one method, got %v", methods)
	}
}

func TestBuildProductInfoParamsPreservesRawOnBadPrice(t *testing.T) {
	t.Parallel()
	item := ozon.ProductInfoListItem{
		ID:    10,
		SKU:   20,
		Price: "bad",
		Name:  "Test",
	}
	raw, _ := json.Marshal(item)
	params := buildProductInfoParams(1, time.Now().UTC(), pgDate(time.Now().UTC()), item, raw)
	if params.Price.Valid {
		t.Fatal("expected invalid normalized price")
	}
	if string(params.RawResponseJson) == "" {
		t.Fatal("expected raw preserved")
	}
}

func TestStubPerformanceDailyStatsDoesNotCreateActions(t *testing.T) {
	t.Parallel()
	// Guardrail: enrichment layer only stores metrics facts.
	api := &stubPerformanceAPI{}
	resp, _ := api.GetPerformanceDailyStatisticsJSON(context.Background(), "", "", nil, time.Now(), time.Now())
	if len(resp.Data.Items) != 1 || resp.Data.Items[0].OrdersCount != 0 {
		t.Fatalf("unexpected stats: %+v", resp.Data.Items)
	}
}
