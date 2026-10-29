package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

var (
	ErrUnknownTool                = errors.New("unknown tool")
	ErrToolDataRepositoryRequired = errors.New("tool data repository is required")
)

type ToolExecutor interface {
	Execute(ctx context.Context, sellerAccountID int64, call ToolCall) (*ToolResult, error)
	ExecutePlan(ctx context.Context, sellerAccountID int64, plan ValidatedToolPlan) ([]ToolResult, error)
}

type RecommendationToolFilter struct {
	Limit          int32
	PriorityLevels []string
	Horizon        *string
}

type AlertToolFilter struct {
	Limit      int32
	Severities []string
	Groups     []string
}

type CriticalSKUToolFilter struct {
	Limit    int32
	AsOfDate *time.Time
}

type StockRiskToolFilter struct {
	Limit        int32
	AsOfDate     *time.Time
	CategoryHint *string
}

type AdvertisingToolFilter struct {
	Limit      int32
	DateFrom   *time.Time
	DateTo     *time.Time
	CampaignID *int64
}

type SKUMetricsToolFilter struct {
	Limit        int32
	DateFrom     *time.Time
	DateTo       *time.Time
	CategoryHint *string
	SKU          *int64
	OfferID      *string
	SortBy       string
}

type SKUContextToolFilter struct {
	SKU     *int64
	OfferID *string
}

type ABCAnalysisToolFilter struct {
	CategoryHint *string
	DateFrom     *time.Time
	DateTo       *time.Time
	Metric       string
	Limit        int32
}

type DashboardSummaryToolData struct {
	AsOfDate             string         `json:"as_of_date"`
	AsOfDateSource       string         `json:"as_of_date_source"`
	DataFreshness        string         `json:"data_freshness"`
	LastSuccessfulUpdate *string        `json:"last_successful_update,omitempty"`
	KPI                  map[string]any `json:"kpi"`
	Deltas               map[string]any `json:"deltas"`
}

type RecommendationToolItem struct {
	ID                 int64          `json:"id"`
	RecommendationType string         `json:"recommendation_type"`
	Horizon            string         `json:"horizon"`
	EntityType         string         `json:"entity_type"`
	EntityID           *string        `json:"entity_id,omitempty"`
	EntitySKU          *int64         `json:"entity_sku,omitempty"`
	EntityOfferID      *string        `json:"entity_offer_id,omitempty"`
	Title              string         `json:"title"`
	WhatHappened       string         `json:"what_happened"`
	WhyItMatters       string         `json:"why_it_matters"`
	RecommendedAction  string         `json:"recommended_action"`
	ExpectedEffect     *string        `json:"expected_effect,omitempty"`
	PriorityScore      float64        `json:"priority_score"`
	PriorityLevel      string         `json:"priority_level"`
	Urgency            string         `json:"urgency"`
	ConfidenceLevel    string         `json:"confidence_level"`
	Status             string         `json:"status"`
	SupportingMetrics  map[string]any `json:"supporting_metrics,omitempty"`
	Constraints        map[string]any `json:"constraints,omitempty"`
	LastSeenAt         *string        `json:"last_seen_at,omitempty"`
}

type RecommendationDetailToolData struct {
	Recommendation RecommendationToolItem `json:"recommendation"`
	RelatedAlerts  []AlertToolItem        `json:"related_alerts"`
}

type AlertToolItem struct {
	ID            int64          `json:"id"`
	AlertType     string         `json:"alert_type"`
	AlertGroup    string         `json:"alert_group"`
	EntityType    string         `json:"entity_type"`
	EntityID      *string        `json:"entity_id,omitempty"`
	EntitySKU     *int64         `json:"entity_sku,omitempty"`
	EntityOfferID *string        `json:"entity_offer_id,omitempty"`
	Title         string         `json:"title"`
	Message       string         `json:"message"`
	Severity      string         `json:"severity"`
	Urgency       string         `json:"urgency"`
	Status        string         `json:"status"`
	Evidence      map[string]any `json:"evidence,omitempty"`
	FirstSeenAt   *string        `json:"first_seen_at,omitempty"`
	LastSeenAt    *string        `json:"last_seen_at,omitempty"`
}

type CriticalSKUToolItem struct {
	SKU             *int64   `json:"sku,omitempty"`
	OfferID         *string  `json:"offer_id,omitempty"`
	ProductID       int64    `json:"product_id"`
	ProductName     *string  `json:"product_name,omitempty"`
	ProblemScore    float64  `json:"problem_score"`
	ImportanceScore float64  `json:"importance_score"`
	Revenue         float64  `json:"revenue"`
	Orders          int32    `json:"orders"`
	CurrentStock    int32    `json:"current_stock"`
	DaysOfCover     *float64 `json:"days_of_cover,omitempty"`
	Signals         []string `json:"signals"`
}

type StockRiskToolItem struct {
	SKU                   *int64   `json:"sku,omitempty"`
	OfferID               *string  `json:"offer_id,omitempty"`
	ProductID             int64    `json:"product_id"`
	ProductName           *string  `json:"product_name,omitempty"`
	CurrentStock          int32    `json:"current_stock"`
	DaysOfCover           *float64 `json:"days_of_cover,omitempty"`
	DepletionRisk         string   `json:"depletion_risk"`
	ReplenishmentPriority string   `json:"replenishment_priority"`
	EstimatedStockoutDate *string  `json:"estimated_stockout_date,omitempty"`
	Reason                string   `json:"reason,omitempty"`
	Revenue               *float64 `json:"revenue,omitempty"`
	Orders                *int32   `json:"orders,omitempty"`
}

type AdvertisingCampaignToolItem struct {
	CampaignID int64    `json:"campaign_id"`
	Name       string   `json:"campaign_name"`
	Type       *string  `json:"campaign_type,omitempty"`
	Spend      float64  `json:"spend"`
	Revenue    float64  `json:"revenue"`
	Orders     int64    `json:"orders"`
	ROAS       *float64 `json:"roas,omitempty"`
	RiskSignal string   `json:"risk_signal"`
}

type AdvertisingToolData struct {
	Summary   map[string]any                `json:"summary"`
	Campaigns []AdvertisingCampaignToolItem `json:"campaigns"`
	SKURisks  []map[string]any              `json:"sku_risks"`
}

type SKUMetricToolItem struct {
	ProductID    int64    `json:"product_id"`
	SKU          *int64   `json:"sku,omitempty"`
	OfferID      *string  `json:"offer_id,omitempty"`
	ProductName  *string  `json:"product_name,omitempty"`
	Revenue      float64  `json:"revenue"`
	Orders       int32    `json:"orders"`
	Returns      *int32   `json:"returns,omitempty"`
	Cancels      *int32   `json:"cancels,omitempty"`
	CurrentStock *int32   `json:"current_stock,omitempty"`
	DaysOfCover  *float64 `json:"days_of_cover,omitempty"`
	Contribution *float64 `json:"contribution,omitempty"`
}

type SKUContextToolData struct {
	Product         map[string]any           `json:"product"`
	Sales           map[string]any           `json:"sales"`
	Stock           map[string]any           `json:"stock"`
	Pricing         map[string]any           `json:"pricing"`
	Alerts          []AlertToolItem          `json:"alerts"`
	Recommendations []RecommendationToolItem `json:"recommendations"`
	Advertising     []map[string]any         `json:"advertising,omitempty"`
}

type CampaignContextToolData struct {
	Campaign        map[string]any           `json:"campaign"`
	Metrics         map[string]any           `json:"metrics"`
	LinkedSKUs      []map[string]any         `json:"linked_skus"`
	Alerts          []AlertToolItem          `json:"alerts"`
	Recommendations []RecommendationToolItem `json:"recommendations"`
}

type ABCRowToolData struct {
	SKU             *int64   `json:"sku,omitempty"`
	OfferID         *string  `json:"offer_id,omitempty"`
	ProductName     *string  `json:"product_name,omitempty"`
	Revenue         float64  `json:"revenue"`
	Orders          int32    `json:"orders"`
	MetricValue     float64  `json:"metric_value"`
	MetricShare     float64  `json:"metric_share"`
	CumulativeShare float64  `json:"cumulative_share"`
	ABCClass        string   `json:"abc_class"`
	DaysOfCover     *float64 `json:"days_of_cover,omitempty"`
}

type ABCAnalysisToolData struct {
	Summary map[string]any   `json:"summary"`
	Rows    []ABCRowToolData `json:"rows"`
}

type ToolDataRepository interface {
	GetDashboardSummary(ctx context.Context, sellerAccountID int64, asOfDate *time.Time) (*DashboardSummaryToolData, error)
	ListOpenRecommendations(ctx context.Context, sellerAccountID int64, filter RecommendationToolFilter) ([]RecommendationToolItem, error)
	GetRecommendationDetail(ctx context.Context, sellerAccountID int64, recommendationID int64) (*RecommendationDetailToolData, error)
	ListOpenAlerts(ctx context.Context, sellerAccountID int64, filter AlertToolFilter) ([]AlertToolItem, error)
	ListAlertsByGroup(ctx context.Context, sellerAccountID int64, group string, limit int32) ([]AlertToolItem, error)
	ListCriticalSKUs(ctx context.Context, sellerAccountID int64, filter CriticalSKUToolFilter) ([]CriticalSKUToolItem, error)
	ListStockRisks(ctx context.Context, sellerAccountID int64, filter StockRiskToolFilter) ([]StockRiskToolItem, error)
	GetAdvertisingAnalytics(ctx context.Context, sellerAccountID int64, filter AdvertisingToolFilter) (*AdvertisingToolData, error)
	ListSKUMetrics(ctx context.Context, sellerAccountID int64, filter SKUMetricsToolFilter) ([]SKUMetricToolItem, error)
	GetSKUContext(ctx context.Context, sellerAccountID int64, filter SKUContextToolFilter) (*SKUContextToolData, error)
	GetCampaignContext(ctx context.Context, sellerAccountID int64, campaignID int64) (*CampaignContextToolData, error)
}

type ToolSet struct {
	registry *ToolRegistry
	repo     ToolDataRepository
}

func NewToolSet(registry *ToolRegistry, repo ToolDataRepository) *ToolSet {
	return &ToolSet{registry: registry, repo: repo}
}

func (s *ToolSet) Execute(ctx context.Context, sellerAccountID int64, call ToolCall) (*ToolResult, error) {
	if s.registry == nil {
		return nil, ErrUnknownTool
	}
	if _, ok := s.registry.Get(call.Name); !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownTool, call.Name)
	}
	if s.repo == nil {
		return nil, ErrToolDataRepositoryRequired
	}

	switch call.Name {
	case ToolGetDashboardSummary:
		return s.execDashboardSummary(ctx, sellerAccountID, call)
	case ToolGetOpenRecommendations:
		return s.execOpenRecommendations(ctx, sellerAccountID, call)
	case ToolGetRecommendationDetail:
		return s.execRecommendationDetail(ctx, sellerAccountID, call)
	case ToolGetOpenAlerts:
		return s.execOpenAlerts(ctx, sellerAccountID, call)
	case ToolGetAlertsByGroup:
		return s.execAlertsByGroup(ctx, sellerAccountID, call)
	case ToolGetCriticalSKUs:
		return s.execCriticalSKUs(ctx, sellerAccountID, call)
	case ToolGetStockRisks:
		return s.execStockRisks(ctx, sellerAccountID, call)
	case ToolGetAdvertisingAnalytics:
		return s.execAdvertisingAnalytics(ctx, sellerAccountID, call)
	case ToolGetPriceEconomicsRisks:
		return s.execPriceEconomicsRisks(ctx, sellerAccountID, call)
	case ToolGetSKUMetrics:
		return s.execSKUMetrics(ctx, sellerAccountID, call)
	case ToolGetSKUContext:
		return s.execSKUContext(ctx, sellerAccountID, call)
	case ToolGetCampaignContext:
		return s.execCampaignContext(ctx, sellerAccountID, call)
	case ToolRunABCAnalysis:
		return s.execABCAnalysis(ctx, sellerAccountID, call)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnknownTool, call.Name)
	}
}

func (s *ToolSet) ExecutePlan(ctx context.Context, sellerAccountID int64, plan ValidatedToolPlan) ([]ToolResult, error) {
	results := make([]ToolResult, 0, len(plan.ToolCalls))
	for _, call := range plan.ToolCalls {
		res, err := s.Execute(ctx, sellerAccountID, call)
		if err != nil {
			msg := err.Error()
			results = append(results, ToolResult{
				Name:        call.Name,
				Args:        sanitizeArgs(call.Args),
				Data:        map[string]any{},
				Error:       &msg,
				Limitations: []string{"Tool execution failed; partial context only."},
			})
			continue
		}
		results = append(results, *res)
	}
	if len(results) == 0 {
		return nil, errors.New("no tool results generated")
	}
	return results, nil
}

func (s *ToolSet) execDashboardSummary(ctx context.Context, sellerAccountID int64, call ToolCall) (*ToolResult, error) {
	asOfDate, _ := getDateArg(call.Args, "as_of_date")
	data, err := s.repo.GetDashboardSummary(ctx, sellerAccountID, asOfDate)
	if err != nil {
		return nil, err
	}
	if data == nil {
		data = &DashboardSummaryToolData{
			KPI:    map[string]any{},
			Deltas: map[string]any{},
		}
	}
	return &ToolResult{
		Name:        call.Name,
		Args:        sanitizeArgs(call.Args),
		Data:        data,
		Limitations: emptyLimitationsIfAny(data.AsOfDate == ""),
	}, nil
}

func (s *ToolSet) execOpenRecommendations(ctx context.Context, sellerAccountID int64, call ToolCall) (*ToolResult, error) {
	limit := getIntArg(call.Args, "limit", 5, 10)
	priorityLevels := getStringArrayArg(call.Args, "priority_levels")
	horizon := getStringArgPtr(call.Args, "horizon")
	items, err := s.repo.ListOpenRecommendations(ctx, sellerAccountID, RecommendationToolFilter{
		Limit:          int32(limit),
		PriorityLevels: priorityLevels,
		Horizon:        horizon,
	})
	if err != nil {
		return nil, err
	}
	data := map[string]any{
		"items": compactRecommendations(items),
		"count": len(items),
		"limit": limit,
	}
	limitations := []string{}
	if len(items) == 0 {
		limitations = append(limitations, "No open recommendations found for the selected filters.")
	}
	return &ToolResult{Name: call.Name, Args: sanitizeArgs(call.Args), Data: data, Limitations: limitations}, nil
}

func (s *ToolSet) execRecommendationDetail(ctx context.Context, sellerAccountID int64, call ToolCall) (*ToolResult, error) {
	recommendationID := getIntArg(call.Args, "recommendation_id", 0, 1_000_000_000)
	detail, err := s.repo.GetRecommendationDetail(ctx, sellerAccountID, int64(recommendationID))
	if err != nil {
		return nil, err
	}
	if detail == nil {
		return &ToolResult{
			Name:        call.Name,
			Args:        sanitizeArgs(call.Args),
			Data:        map[string]any{},
			Limitations: []string{"Recommendation was not found."},
		}, nil
	}
	return &ToolResult{
		Name: call.Name,
		Args: sanitizeArgs(call.Args),
		Data: map[string]any{
			"recommendation": detail.Recommendation,
			"related_alerts": detail.RelatedAlerts,
		},
		Limitations: []string{"Raw AI response is not included in chat context."},
	}, nil
}

func (s *ToolSet) execOpenAlerts(ctx context.Context, sellerAccountID int64, call ToolCall) (*ToolResult, error) {
	limit := getIntArg(call.Args, "limit", DefaultToolLimit, MaxDefaultToolLimit)
	items, err := s.repo.ListOpenAlerts(ctx, sellerAccountID, AlertToolFilter{
		Limit:      int32(limit),
		Severities: getStringArrayArg(call.Args, "severities"),
		Groups:     getStringArrayArg(call.Args, "groups"),
	})
	if err != nil {
		return nil, err
	}
	data := map[string]any{"items": compactAlerts(items), "count": len(items), "limit": limit}
	limitations := []string{}
	if len(items) == 0 {
		limitations = append(limitations, "No open alerts found for the selected filters.")
	}
	return &ToolResult{Name: call.Name, Args: sanitizeArgs(call.Args), Data: data, Limitations: limitations}, nil
}

func (s *ToolSet) execAlertsByGroup(ctx context.Context, sellerAccountID int64, call ToolCall) (*ToolResult, error) {
	group := strings.TrimSpace(getStringArg(call.Args, "group"))
	limit := int32(getIntArg(call.Args, "limit", DefaultToolLimit, MaxDefaultToolLimit))
	items, err := s.repo.ListAlertsByGroup(ctx, sellerAccountID, group, limit)
	if err != nil {
		return nil, err
	}
	data := map[string]any{"group": group, "items": compactAlerts(items), "count": len(items), "limit": limit}
	return &ToolResult{Name: call.Name, Args: sanitizeArgs(call.Args), Data: data, Limitations: emptyLimitationsIfAny(len(items) == 0)}, nil
}

func (s *ToolSet) execCriticalSKUs(ctx context.Context, sellerAccountID int64, call ToolCall) (*ToolResult, error) {
	limit := int32(getIntArg(call.Args, "limit", DefaultToolLimit, MaxDefaultToolLimit))
	asOf, _ := getDateArg(call.Args, "as_of_date")
	items, err := s.repo.ListCriticalSKUs(ctx, sellerAccountID, CriticalSKUToolFilter{Limit: limit, AsOfDate: asOf})
	if err != nil {
		return nil, err
	}
	if len(items) > int(limit) {
		items = items[:limit]
	}
	data := map[string]any{"items": items, "count": len(items), "limit": limit}
	limitations := []string{"Only top rows are included."}
	if len(items) == 0 {
		limitations = append(limitations, "No data found for the selected period.")
	}
	return &ToolResult{Name: call.Name, Args: sanitizeArgs(call.Args), Data: data, Limitations: limitations}, nil
}

func (s *ToolSet) execStockRisks(ctx context.Context, sellerAccountID int64, call ToolCall) (*ToolResult, error) {
	limit := int32(getIntArg(call.Args, "limit", DefaultToolLimit, MaxDefaultToolLimit))
	asOf, _ := getDateArg(call.Args, "as_of_date")
	category := getStringArgPtr(call.Args, "category_hint")
	items, err := s.repo.ListStockRisks(ctx, sellerAccountID, StockRiskToolFilter{
		Limit:        limit,
		AsOfDate:     asOf,
		CategoryHint: category,
	})
	if err != nil {
		return nil, err
	}
	if len(items) > int(limit) {
		items = items[:limit]
	}
	limitations := []string{}
	if category != nil {
		limitations = append(limitations, "Category filtering depends on available catalog attributes.")
	}
	if len(items) == 0 {
		limitations = append(limitations, "No stock risk data found for the selected filters.")
	}
	return &ToolResult{Name: call.Name, Args: sanitizeArgs(call.Args), Data: map[string]any{"items": items, "count": len(items), "limit": limit}, Limitations: limitations}, nil
}

func (s *ToolSet) execAdvertisingAnalytics(ctx context.Context, sellerAccountID int64, call ToolCall) (*ToolResult, error) {
	limit := int32(getIntArg(call.Args, "limit", DefaultToolLimit, MaxDefaultToolLimit))
	dateFrom, dateTo := getDateRangeArgs(call.Args, DefaultChatDateRangeDays)
	campaignID := getInt64ArgPtr(call.Args, "campaign_id")
	data, err := s.repo.GetAdvertisingAnalytics(ctx, sellerAccountID, AdvertisingToolFilter{
		Limit:      limit,
		DateFrom:   dateFrom,
		DateTo:     dateTo,
		CampaignID: campaignID,
	})
	if err != nil {
		return nil, err
	}
	if data == nil {
		data = &AdvertisingToolData{Summary: map[string]any{}, Campaigns: []AdvertisingCampaignToolItem{}, SKURisks: []map[string]any{}}
	}
	limitations := []string{}
	if campaignID != nil {
		limitations = append(limitations, "Campaign filter applied; summary reflects filtered scope.")
	}
	return &ToolResult{Name: call.Name, Args: sanitizeArgs(call.Args), Data: data, Limitations: limitations}, nil
}

func (s *ToolSet) execPriceEconomicsRisks(ctx context.Context, sellerAccountID int64, call ToolCall) (*ToolResult, error) {
	limit := int32(getIntArg(call.Args, "limit", DefaultToolLimit, MaxDefaultToolLimit))
	items, err := s.repo.ListAlertsByGroup(ctx, sellerAccountID, "price_economics", limit)
	if err != nil {
		return nil, err
	}
	return &ToolResult{
		Name:        call.Name,
		Args:        sanitizeArgs(call.Args),
		Data:        map[string]any{"group": "price_economics", "items": compactAlerts(items), "count": len(items)},
		Limitations: []string{"This tool is a semantic alias over price_economics alerts."},
	}, nil
}

func (s *ToolSet) execSKUMetrics(ctx context.Context, sellerAccountID int64, call ToolCall) (*ToolResult, error) {
	limit := int32(getIntArg(call.Args, "limit", 20, MaxSKUMetricsLimit))
	dateFrom, dateTo := getDateRangeArgs(call.Args, DefaultChatDateRangeDays)
	sortBy := getStringArg(call.Args, "sort_by")
	if sortBy == "" {
		sortBy = "revenue"
	}
	filter := SKUMetricsToolFilter{
		Limit:        limit,
		DateFrom:     dateFrom,
		DateTo:       dateTo,
		CategoryHint: getStringArgPtr(call.Args, "category_hint"),
		SKU:          getInt64ArgPtr(call.Args, "sku"),
		OfferID:      getStringArgPtr(call.Args, "offer_id"),
		SortBy:       sortBy,
	}
	items, err := s.repo.ListSKUMetrics(ctx, sellerAccountID, filter)
	if err != nil {
		return nil, err
	}
	limitations := []string{}
	if filter.CategoryHint != nil {
		limitations = append(limitations, "Category filtering may be approximate depending on available product attributes.")
	}
	if len(items) > int(limit) {
		items = items[:limit]
	}
	data := map[string]any{"items": items, "count": len(items), "limit": limit, "sort_by": sortBy}
	return &ToolResult{Name: call.Name, Args: sanitizeArgs(call.Args), Data: data, Limitations: limitations}, nil
}

func (s *ToolSet) execSKUContext(ctx context.Context, sellerAccountID int64, call ToolCall) (*ToolResult, error) {
	filter := SKUContextToolFilter{SKU: getInt64ArgPtr(call.Args, "sku"), OfferID: getStringArgPtr(call.Args, "offer_id")}
	data, err := s.repo.GetSKUContext(ctx, sellerAccountID, filter)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return &ToolResult{
			Name:        call.Name,
			Args:        sanitizeArgs(call.Args),
			Data:        map[string]any{},
			Limitations: []string{"SKU context was not found for the provided identifier."},
		}, nil
	}
	return &ToolResult{Name: call.Name, Args: sanitizeArgs(call.Args), Data: data, Limitations: []string{}}, nil
}

func (s *ToolSet) execCampaignContext(ctx context.Context, sellerAccountID int64, call ToolCall) (*ToolResult, error) {
	campaignID := int64(getIntArg(call.Args, "campaign_id", 0, 1_000_000_000))
	data, err := s.repo.GetCampaignContext(ctx, sellerAccountID, campaignID)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return &ToolResult{
			Name:        call.Name,
			Args:        sanitizeArgs(call.Args),
			Data:        map[string]any{},
			Limitations: []string{"Campaign context was not found."},
		}, nil
	}
	return &ToolResult{Name: call.Name, Args: sanitizeArgs(call.Args), Data: data, Limitations: []string{}}, nil
}

func (s *ToolSet) execABCAnalysis(ctx context.Context, sellerAccountID int64, call ToolCall) (*ToolResult, error) {
	metric := getStringArg(call.Args, "metric")
	if metric == "" {
		metric = "revenue"
	}
	limit := int32(getIntArg(call.Args, "limit", 100, MaxABCAnalysisLimit))
	dateFrom, dateTo := getDateRangeArgs(call.Args, DefaultChatDateRangeDays)
	filter := ABCAnalysisToolFilter{
		CategoryHint: getStringArgPtr(call.Args, "category_hint"),
		DateFrom:     dateFrom,
		DateTo:       dateTo,
		Metric:       metric,
		Limit:        limit,
	}
	skuRows, err := s.repo.ListSKUMetrics(ctx, sellerAccountID, SKUMetricsToolFilter{
		Limit:        MaxABCAnalysisLimit,
		DateFrom:     filter.DateFrom,
		DateTo:       filter.DateTo,
		CategoryHint: filter.CategoryHint,
		SortBy:       metric,
	})
	if err != nil {
		return nil, err
	}
	analysis := calculateABC(metric, skuRows, int(limit))
	limitations := []string{}
	if len(analysis.Rows) == 0 {
		limitations = append(limitations, "No data found for ABC analysis in selected period.")
	}
	if filter.CategoryHint != nil {
		limitations = append(limitations, "Category filtering may be approximate depending on available catalog attributes.")
	}
	return &ToolResult{Name: call.Name, Args: sanitizeArgs(call.Args), Data: analysis, Limitations: limitations}, nil
}

func calculateABC(metric string, rows []SKUMetricToolItem, limit int) *ABCAnalysisToolData {
	candidates := make([]ABCRowToolData, 0, len(rows))
	total := 0.0
	for _, row := range rows {
		value := row.Revenue
		if metric == "orders" {
			value = float64(row.Orders)
		}
		if value <= 0 {
			continue
		}
		total += value
		candidates = append(candidates, ABCRowToolData{
			SKU:         row.SKU,
			OfferID:     row.OfferID,
			ProductName: row.ProductName,
			Revenue:     row.Revenue,
			Orders:      row.Orders,
			MetricValue: value,
			DaysOfCover: row.DaysOfCover,
		})
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].MetricValue > candidates[j].MetricValue })
	if limit > 0 && len(candidates) > limit {
		candidates = candidates[:limit]
	}
	if total <= 0 || len(candidates) == 0 {
		return &ABCAnalysisToolData{Summary: map[string]any{
			"metric": metric, "total_skus": 0, "total_metric": 0.0, "a_count": 0, "b_count": 0, "c_count": 0,
		}, Rows: []ABCRowToolData{}}
	}
	var cumulative float64
	var aCount, bCount, cCount int
	var aShare, bShare, cShare float64
	for i := range candidates {
		share := candidates[i].MetricValue / total
		cumulative += share
		class := "C"
		switch {
		case cumulative <= 0.80:
			class = "A"
		case cumulative <= 0.95:
			class = "B"
		}
		candidates[i].MetricShare = roundFloat(share, 6)
		candidates[i].CumulativeShare = roundFloat(cumulative, 6)
		candidates[i].ABCClass = class
		switch class {
		case "A":
			aCount++
			aShare += share
		case "B":
			bCount++
			bShare += share
		default:
			cCount++
			cShare += share
		}
	}
	return &ABCAnalysisToolData{
		Summary: map[string]any{
			"metric":       metric,
			"total_skus":   len(candidates),
			"total_metric": roundFloat(total, 2),
			"a_count":      aCount,
			"b_count":      bCount,
			"c_count":      cCount,
			"a_share":      roundFloat(aShare, 6),
			"b_share":      roundFloat(bShare, 6),
			"c_share":      roundFloat(cShare, 6),
		},
		Rows: candidates,
	}
}

func roundFloat(v float64, scale int) float64 {
	if scale <= 0 {
		return v
	}
	factor := 1.0
	for i := 0; i < scale; i++ {
		factor *= 10
	}
	return float64(int64(v*factor+0.5)) / factor
}

func getIntArg(args map[string]any, name string, defaultValue int, max int) int {
	raw, ok := args[name]
	if !ok {
		return defaultValue
	}
	switch v := raw.(type) {
	case int:
		if v > max {
			return max
		}
		return v
	case int32:
		return getIntArg(argsFromSingle(name, int(v)), name, defaultValue, max)
	case int64:
		return getIntArg(argsFromSingle(name, int(v)), name, defaultValue, max)
	case float64:
		return getIntArg(argsFromSingle(name, int(v)), name, defaultValue, max)
	default:
		return defaultValue
	}
}

func getInt64ArgPtr(args map[string]any, name string) *int64 {
	raw, ok := args[name]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case int:
		x := int64(v)
		return &x
	case int64:
		x := v
		return &x
	case int32:
		x := int64(v)
		return &x
	case float64:
		x := int64(v)
		return &x
	default:
		return nil
	}
}

func getStringArg(args map[string]any, name string) string {
	raw, ok := args[name]
	if !ok {
		return ""
	}
	s, ok := raw.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

func getStringArgPtr(args map[string]any, name string) *string {
	s := getStringArg(args, name)
	if s == "" {
		return nil
	}
	return &s
}

func getStringArrayArg(args map[string]any, name string) []string {
	raw, ok := args[name]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if ok && strings.TrimSpace(s) != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func getDateArg(args map[string]any, name string) (*time.Time, bool) {
	value := getStringArg(args, name)
	if value == "" {
		return nil, false
	}
	t, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, false
	}
	t = t.UTC()
	return &t, true
}

func getDateRangeArgs(args map[string]any, defaultDays int) (*time.Time, *time.Time) {
	from, hasFrom := getDateArg(args, "date_from")
	to, hasTo := getDateArg(args, "date_to")
	if hasFrom || hasTo {
		return from, to
	}
	end := time.Now().UTC().Truncate(24 * time.Hour)
	start := end.AddDate(0, 0, -defaultDays)
	return &start, &end
}

func argsFromSingle(k string, v any) map[string]any { return map[string]any{k: v} }

func compactAlerts(items []AlertToolItem) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, map[string]any{
			"id":         item.ID,
			"alert_type": item.AlertType,
			"group":      item.AlertGroup,
			"severity":   item.Severity,
			"urgency":    item.Urgency,
			"title":      item.Title,
			"entity": map[string]any{
				"entity_type":  item.EntityType,
				"entity_id":    item.EntityID,
				"entity_sku":   item.EntitySKU,
				"entity_offer": item.EntityOfferID,
			},
			"evidence_summary": compactJSONMap(item.Evidence, []string{
				"current_price", "effective_min_price", "effective_max_price", "expected_margin",
				"days_of_cover", "available_stock", "spend", "revenue", "orders_count",
			}, 4096),
		})
	}
	return out
}

func compactRecommendations(items []RecommendationToolItem) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, map[string]any{
			"id":                 item.ID,
			"title":              item.Title,
			"recommended_action": item.RecommendedAction,
			"priority_level":     item.PriorityLevel,
			"priority_score":     item.PriorityScore,
			"urgency":            item.Urgency,
			"confidence_level":   item.ConfidenceLevel,
			"horizon":            item.Horizon,
			"entity": map[string]any{
				"entity_type": item.EntityType,
				"entity_id":   item.EntityID,
				"entity_sku":  item.EntitySKU,
				"offer_id":    item.EntityOfferID,
			},
			"supporting_metrics_summary": summarizeRecommendationMetrics(item.SupportingMetrics),
			"constraints_summary":        summarizeConstraints(item.Constraints),
			"last_seen_at":               item.LastSeenAt,
		})
	}
	return out
}

func summarizeRecommendationMetrics(payload map[string]any) map[string]any {
	return compactJSONMap(payload, []string{
		"revenue", "orders", "revenue_delta", "orders_delta", "share", "days_of_cover",
	}, 4096)
}

func summarizeConstraints(payload map[string]any) map[string]any {
	return compactJSONMap(payload, []string{
		"effective_min_price", "effective_max_price", "reference_price", "implied_cost", "expected_margin",
	}, 4096)
}

func compactJSONMap(input map[string]any, allowedKeys []string, maxBytes int) map[string]any {
	if len(input) == 0 {
		return map[string]any{}
	}
	allowed := make(map[string]struct{}, len(allowedKeys))
	for _, key := range allowedKeys {
		allowed[key] = struct{}{}
	}
	out := map[string]any{}
	for key, value := range input {
		if _, ok := allowed[key]; ok {
			out[key] = value
		}
	}
	raw, err := json.Marshal(out)
	if err != nil || len(raw) <= maxBytes {
		return out
	}
	return map[string]any{"truncated": true, "keys": keysOf(out)}
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for key := range m {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func sanitizeArgs(args map[string]any) map[string]any {
	out := make(map[string]any, len(args))
	for key, value := range args {
		if key == "seller_account_id" || key == "user_id" {
			continue
		}
		out[key] = value
	}
	return out
}

func emptyLimitationsIfAny(empty bool) []string {
	if empty {
		return []string{"No data found for the selected period."}
	}
	return []string{}
}
