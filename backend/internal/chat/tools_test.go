package chat

import (
	"context"
	"errors"
	"testing"
	"time"
)

type mockToolRepo struct {
	calls []string

	dashboardData *DashboardSummaryToolData
	dashboardErr  error

	recommendations         []RecommendationToolItem
	recommendationsErr      error
	recommendationDetail    *RecommendationDetailToolData
	recommendationDetailErr error

	alerts    []AlertToolItem
	alertsErr error

	criticalSKUs    []CriticalSKUToolItem
	criticalSKUsErr error
	stockRisks      []StockRiskToolItem
	stockRisksErr   error

	advertisingData *AdvertisingToolData
	advertisingErr  error

	skuMetrics    []SKUMetricToolItem
	skuMetricsErr error
	skuContext    *SKUContextToolData
	skuContextErr error
	campaignCtx   *CampaignContextToolData
	campaignErr   error
}

func (m *mockToolRepo) GetDashboardSummary(ctx context.Context, sellerAccountID int64, asOfDate *time.Time) (*DashboardSummaryToolData, error) {
	m.calls = append(m.calls, "GetDashboardSummary")
	return m.dashboardData, m.dashboardErr
}
func (m *mockToolRepo) ListOpenRecommendations(ctx context.Context, sellerAccountID int64, filter RecommendationToolFilter) ([]RecommendationToolItem, error) {
	m.calls = append(m.calls, "ListOpenRecommendations")
	return m.recommendations, m.recommendationsErr
}
func (m *mockToolRepo) GetRecommendationDetail(ctx context.Context, sellerAccountID int64, recommendationID int64) (*RecommendationDetailToolData, error) {
	m.calls = append(m.calls, "GetRecommendationDetail")
	return m.recommendationDetail, m.recommendationDetailErr
}
func (m *mockToolRepo) ListOpenAlerts(ctx context.Context, sellerAccountID int64, filter AlertToolFilter) ([]AlertToolItem, error) {
	m.calls = append(m.calls, "ListOpenAlerts")
	return m.alerts, m.alertsErr
}
func (m *mockToolRepo) ListAlertsByGroup(ctx context.Context, sellerAccountID int64, group string, limit int32) ([]AlertToolItem, error) {
	m.calls = append(m.calls, "ListAlertsByGroup")
	return m.alerts, m.alertsErr
}
func (m *mockToolRepo) ListCriticalSKUs(ctx context.Context, sellerAccountID int64, filter CriticalSKUToolFilter) ([]CriticalSKUToolItem, error) {
	m.calls = append(m.calls, "ListCriticalSKUs")
	return m.criticalSKUs, m.criticalSKUsErr
}
func (m *mockToolRepo) ListStockRisks(ctx context.Context, sellerAccountID int64, filter StockRiskToolFilter) ([]StockRiskToolItem, error) {
	m.calls = append(m.calls, "ListStockRisks")
	return m.stockRisks, m.stockRisksErr
}
func (m *mockToolRepo) GetAdvertisingAnalytics(ctx context.Context, sellerAccountID int64, filter AdvertisingToolFilter) (*AdvertisingToolData, error) {
	m.calls = append(m.calls, "GetAdvertisingAnalytics")
	return m.advertisingData, m.advertisingErr
}
func (m *mockToolRepo) ListSKUMetrics(ctx context.Context, sellerAccountID int64, filter SKUMetricsToolFilter) ([]SKUMetricToolItem, error) {
	m.calls = append(m.calls, "ListSKUMetrics")
	return m.skuMetrics, m.skuMetricsErr
}
func (m *mockToolRepo) GetSKUContext(ctx context.Context, sellerAccountID int64, filter SKUContextToolFilter) (*SKUContextToolData, error) {
	m.calls = append(m.calls, "GetSKUContext")
	return m.skuContext, m.skuContextErr
}
func (m *mockToolRepo) GetCampaignContext(ctx context.Context, sellerAccountID int64, campaignID int64) (*CampaignContextToolData, error) {
	m.calls = append(m.calls, "GetCampaignContext")
	return m.campaignCtx, m.campaignErr
}

func TestExecuteRejectsUnknownTool(t *testing.T) {
	s := NewToolSet(NewDefaultToolRegistry(), &mockToolRepo{})
	_, err := s.Execute(context.Background(), 1, ToolCall{Name: "nope", Args: map[string]any{}})
	if err == nil {
		t.Fatal("expected unknown tool error")
	}
}

func TestExecutePlanRunsInOrderAndPartialOnFailure(t *testing.T) {
	repo := &mockToolRepo{
		dashboardData: &DashboardSummaryToolData{AsOfDate: "2026-05-01", KPI: map[string]any{}, Deltas: map[string]any{}},
		alertsErr:     errors.New("alerts failed"),
	}
	s := NewToolSet(NewDefaultToolRegistry(), repo)
	results, err := s.ExecutePlan(context.Background(), 1, ValidatedToolPlan{
		ToolCalls: []ToolCall{
			{Name: ToolGetDashboardSummary, Args: map[string]any{}},
			{Name: ToolGetOpenAlerts, Args: map[string]any{}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[1].Error == nil {
		t.Fatal("expected second tool error in partial execution")
	}
	if len(repo.calls) != 2 || repo.calls[0] != "GetDashboardSummary" || repo.calls[1] != "ListOpenAlerts" {
		t.Fatalf("unexpected call order: %+v", repo.calls)
	}
}

func TestDashboardSummaryCompactData(t *testing.T) {
	repo := &mockToolRepo{dashboardData: &DashboardSummaryToolData{AsOfDate: "2026-05-01", KPI: map[string]any{"revenue": 123}, Deltas: map[string]any{"revenue_day_to_day_delta": 12}}}
	s := NewToolSet(NewDefaultToolRegistry(), repo)
	res, err := s.Execute(context.Background(), 1, ToolCall{Name: ToolGetDashboardSummary, Args: map[string]any{}})
	if err != nil || res == nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRecommendationAndAlertTools(t *testing.T) {
	repo := &mockToolRepo{
		recommendations:      []RecommendationToolItem{{ID: 11, Title: "rec"}},
		recommendationDetail: &RecommendationDetailToolData{Recommendation: RecommendationToolItem{ID: 11}, RelatedAlerts: []AlertToolItem{{ID: 21}}},
		alerts:               []AlertToolItem{{ID: 7, AlertGroup: "stock"}},
	}
	s := NewToolSet(NewDefaultToolRegistry(), repo)

	calls := []ToolCall{
		{Name: ToolGetOpenRecommendations, Args: map[string]any{"limit": 5}},
		{Name: ToolGetRecommendationDetail, Args: map[string]any{"recommendation_id": 11}},
		{Name: ToolGetOpenAlerts, Args: map[string]any{"groups": []string{"stock"}}},
		{Name: ToolGetAlertsByGroup, Args: map[string]any{"group": "stock", "limit": 5}},
		{Name: ToolGetPriceEconomicsRisks, Args: map[string]any{"limit": 3}},
	}
	for _, call := range calls {
		if _, err := s.Execute(context.Background(), 1, call); err != nil {
			t.Fatalf("unexpected error for %s: %v", call.Name, err)
		}
	}
}

func TestCriticalStockAdsSkuCampaignTools(t *testing.T) {
	repo := &mockToolRepo{
		criticalSKUs:    []CriticalSKUToolItem{{ProductID: 1}},
		stockRisks:      []StockRiskToolItem{{ProductID: 2}},
		advertisingData: &AdvertisingToolData{Summary: map[string]any{"total_spend": 10}},
		skuMetrics:      []SKUMetricToolItem{{ProductID: 3, Revenue: 100, Orders: 10}},
		skuContext:      &SKUContextToolData{Product: map[string]any{"sku": 123}},
		campaignCtx:     &CampaignContextToolData{Campaign: map[string]any{"id": 55}},
	}
	s := NewToolSet(NewDefaultToolRegistry(), repo)
	calls := []ToolCall{
		{Name: ToolGetCriticalSKUs, Args: map[string]any{"limit": 10}},
		{Name: ToolGetStockRisks, Args: map[string]any{"limit": 10, "category_hint": "cosmetics"}},
		{Name: ToolGetAdvertisingAnalytics, Args: map[string]any{"limit": 10}},
		{Name: ToolGetSKUMetrics, Args: map[string]any{"limit": 20, "sort_by": "revenue"}},
		{Name: ToolGetSKUContext, Args: map[string]any{"sku": 123}},
		{Name: ToolGetSKUContext, Args: map[string]any{"offer_id": "abc"}},
		{Name: ToolGetCampaignContext, Args: map[string]any{"campaign_id": 55}},
	}
	for _, call := range calls {
		if _, err := s.Execute(context.Background(), 1, call); err != nil {
			t.Fatalf("unexpected error for %s: %v", call.Name, err)
		}
	}
}

func TestABCAnalysisDeterministicAndEdgeCases(t *testing.T) {
	repo := &mockToolRepo{
		skuMetrics: []SKUMetricToolItem{
			{SKU: int64Ptr(1), Revenue: 80, Orders: 8},
			{SKU: int64Ptr(2), Revenue: 15, Orders: 3},
			{SKU: int64Ptr(3), Revenue: 5, Orders: 1},
		},
	}
	s := NewToolSet(NewDefaultToolRegistry(), repo)
	res, err := s.Execute(context.Background(), 1, ToolCall{Name: ToolRunABCAnalysis, Args: map[string]any{"metric": "revenue", "limit": 200}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data := res.Data.(*ABCAnalysisToolData)
	if len(data.Rows) != 3 {
		t.Fatalf("expected 3 abc rows, got %d", len(data.Rows))
	}

	repo.skuMetrics = []SKUMetricToolItem{}
	res, err = s.Execute(context.Background(), 1, ToolCall{Name: ToolRunABCAnalysis, Args: map[string]any{"metric": "orders"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	empty := res.Data.(*ABCAnalysisToolData)
	if len(empty.Rows) != 0 {
		t.Fatal("expected empty abc rows")
	}
}

func TestSanitizeArgsNoSellerScopeLeak(t *testing.T) {
	repo := &mockToolRepo{alerts: []AlertToolItem{}}
	s := NewToolSet(NewDefaultToolRegistry(), repo)
	res, err := s.Execute(context.Background(), 1, ToolCall{
		Name: ToolGetOpenAlerts,
		Args: map[string]any{"seller_account_id": 99, "user_id": 42, "limit": 10},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := res.Args["seller_account_id"]; ok {
		t.Fatal("seller_account_id leaked into result args")
	}
	if _, ok := res.Args["user_id"]; ok {
		t.Fatal("user_id leaked into result args")
	}
}

func int64Ptr(v int64) *int64 { return &v }
