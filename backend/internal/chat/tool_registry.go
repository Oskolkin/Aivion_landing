package chat

import "sort"

const (
	ToolGetDashboardSummary     = "get_dashboard_summary"
	ToolGetOpenRecommendations  = "get_open_recommendations"
	ToolGetRecommendationDetail = "get_recommendation_detail"
	ToolGetOpenAlerts           = "get_open_alerts"
	ToolGetAlertsByGroup        = "get_alerts_by_group"
	ToolGetCriticalSKUs         = "get_critical_skus"
	ToolGetStockRisks           = "get_stock_risks"
	ToolGetAdvertisingAnalytics = "get_advertising_analytics"
	ToolGetPriceEconomicsRisks  = "get_price_economics_risks"
	ToolGetSKUMetrics           = "get_sku_metrics"
	ToolGetSKUContext           = "get_sku_context"
	ToolGetCampaignContext      = "get_campaign_context"
	ToolRunABCAnalysis          = "run_abc_analysis"
)

const (
	DefaultToolLimit    = 10
	MaxDefaultToolLimit = 20
	MaxSKUMetricsLimit  = 50
	MaxABCAnalysisLimit = 200
	DefaultPeriodDays   = 30
	MaxPeriodDays       = 90
)

type ToolDefinition struct {
	Name             string
	Purpose          string
	Description      string
	ReadOnly         bool
	AllowedArgs      map[string]ToolArgDefinition
	DefaultArgs      map[string]any
	MaxLimit         int
	OutputShape      string
	SupportedIntents []ChatIntent
}

type ToolArgDefinition struct {
	Type          string
	Required      bool
	Default       any
	AllowedValues []string
	MaxInt        *int
	MinInt        *int
	MaxDays       *int
	Description   string
}

type ToolRegistry struct {
	definitions map[string]ToolDefinition
}

func NewDefaultToolRegistry() *ToolRegistry {
	min1 := 1
	max10 := 10
	max20 := 20
	max50 := MaxSKUMetricsLimit
	max200 := MaxABCAnalysisLimit
	max90Days := MaxPeriodDays
	priorityValues := []string{"critical", "high", "medium", "low"}
	horizonValues := []string{"short_term", "medium_term", "long_term"}
	groupValues := []string{"sales", "stock", "advertising", "price_economics"}
	severityValues := []string{"critical", "high", "medium", "low"}
	sortByValues := []string{"revenue", "orders", "revenue_delta", "orders_delta", "contribution"}
	metricValues := []string{"revenue", "orders"}

	defs := []ToolDefinition{
		{
			Name:        ToolGetDashboardSummary,
			Purpose:     "Get account-level dashboard KPI summary and freshness.",
			Description: "Returns account KPI aggregates and freshness metadata for a selected as_of_date.",
			ReadOnly:    true,
			AllowedArgs: map[string]ToolArgDefinition{
				"as_of_date": {Type: "date", Description: "Optional dashboard date in YYYY-MM-DD."},
			},
			DefaultArgs: map[string]any{},
			MaxLimit:    1,
			OutputShape: "revenue, orders, returns, cancels, deltas, freshness, as_of_date",
			SupportedIntents: []ChatIntent{
				ChatIntentPriorities, ChatIntentGeneralOverview, ChatIntentSales, ChatIntentRecommendations, ChatIntentAlerts, ChatIntentUnknown,
				ChatIntentAdvertising, ChatIntentAdLoss, ChatIntentUnsafeAds, ChatIntentStock, ChatIntentPricing, ChatIntentABCAnalysis,
			},
		},
		{
			Name:        ToolGetOpenRecommendations,
			Purpose:     "List top open recommendations for prioritization.",
			Description: "Filters recommendations by priority and horizon and returns compact actionable summary.",
			ReadOnly:    true,
			AllowedArgs: map[string]ToolArgDefinition{
				"limit":           {Type: "integer", Default: 5, MinInt: &min1, MaxInt: &max10, Description: "Max recommendations to return."},
				"priority_levels": {Type: "array<string>", AllowedValues: priorityValues, Description: "Optional priority filter."},
				"horizon":         {Type: "string", AllowedValues: horizonValues, Description: "Optional horizon filter."},
			},
			DefaultArgs: map[string]any{"limit": 5},
			MaxLimit:    max10,
			OutputShape: "recommendation id, title, recommended_action, priority, urgency, confidence, horizon, entity, supporting metrics summary",
			SupportedIntents: []ChatIntent{
				ChatIntentPriorities, ChatIntentRecommendations, ChatIntentExplainRecommendation, ChatIntentGeneralOverview,
				ChatIntentPricing, ChatIntentAdvertising, ChatIntentStock, ChatIntentUnknown,
			},
		},
		{
			Name:        ToolGetRecommendationDetail,
			Purpose:     "Get one recommendation with full context.",
			Description: "Returns recommendation details, supporting metrics, constraints and related alerts.",
			ReadOnly:    true,
			AllowedArgs: map[string]ToolArgDefinition{
				"recommendation_id": {Type: "integer", Required: true, MinInt: &min1, Description: "Recommendation identifier."},
			},
			DefaultArgs: map[string]any{},
			MaxLimit:    1,
			OutputShape: "recommendation detail, supporting metrics, constraints, related alerts, status",
			SupportedIntents: []ChatIntent{
				ChatIntentExplainRecommendation, ChatIntentRecommendations,
			},
		},
		{
			Name:        ToolGetOpenAlerts,
			Purpose:     "List open alerts with filters by severity/group.",
			Description: "Returns compact alert feed with evidence summary for priority assessment.",
			ReadOnly:    true,
			AllowedArgs: map[string]ToolArgDefinition{
				"limit":      {Type: "integer", Default: DefaultToolLimit, MinInt: &min1, MaxInt: &max20, Description: "Max alerts to return."},
				"severities": {Type: "array<string>", AllowedValues: severityValues, Description: "Optional severity filter."},
				"groups":     {Type: "array<string>", AllowedValues: groupValues, Description: "Optional alert group filter."},
			},
			DefaultArgs: map[string]any{"limit": DefaultToolLimit},
			MaxLimit:    MaxDefaultToolLimit,
			OutputShape: "alert id, alert_type, group, severity, urgency, entity, title/message, evidence summary",
			SupportedIntents: []ChatIntent{
				ChatIntentAlerts, ChatIntentPriorities, ChatIntentUnsafeAds, ChatIntentAdLoss, ChatIntentStock,
				ChatIntentPricing, ChatIntentSales, ChatIntentAdvertising, ChatIntentGeneralOverview, ChatIntentUnknown,
			},
		},
		{
			Name:        ToolGetAlertsByGroup,
			Purpose:     "List alerts for a specific operational group.",
			Description: "Returns grouped alerts (sales/stock/advertising/price_economics) with evidence summary.",
			ReadOnly:    true,
			AllowedArgs: map[string]ToolArgDefinition{
				"group": {Type: "string", Required: true, AllowedValues: groupValues, Description: "Alert group selector."},
				"limit": {Type: "integer", Default: DefaultToolLimit, MinInt: &min1, MaxInt: &max20, Description: "Max alerts to return."},
			},
			DefaultArgs: map[string]any{"limit": DefaultToolLimit},
			MaxLimit:    MaxDefaultToolLimit,
			OutputShape: "alerts from selected group, evidence summary",
			SupportedIntents: []ChatIntent{
				ChatIntentSales, ChatIntentStock, ChatIntentAdvertising, ChatIntentPricing, ChatIntentAlerts,
				ChatIntentUnsafeAds, ChatIntentAdLoss, ChatIntentPriorities,
			},
		},
		{
			Name:        ToolGetCriticalSKUs,
			Purpose:     "Fetch critical SKU list ranked by risk/importance.",
			Description: "Returns SKU-level risk scoreboard with revenue/orders/stock context.",
			ReadOnly:    true,
			AllowedArgs: map[string]ToolArgDefinition{
				"limit":      {Type: "integer", Default: DefaultToolLimit, MinInt: &min1, MaxInt: &max20, Description: "Max SKUs to return."},
				"as_of_date": {Type: "date", Description: "Optional as-of date."},
			},
			DefaultArgs: map[string]any{"limit": DefaultToolLimit},
			MaxLimit:    MaxDefaultToolLimit,
			OutputShape: "sku, offer_id, product name, problem score, importance, revenue, orders, stock, days of cover, signals",
			SupportedIntents: []ChatIntent{
				ChatIntentPriorities, ChatIntentStock, ChatIntentSales, ChatIntentGeneralOverview,
			},
		},
		{
			Name:        ToolGetStockRisks,
			Purpose:     "Get SKU stockout and replenishment risk signals.",
			Description: "Returns stock risk metrics and estimated stockout horizon.",
			ReadOnly:    true,
			AllowedArgs: map[string]ToolArgDefinition{
				"limit":         {Type: "integer", Default: DefaultToolLimit, MinInt: &min1, MaxInt: &max20, Description: "Max SKU rows to return."},
				"as_of_date":    {Type: "date", Description: "Optional as-of date."},
				"category_hint": {Type: "string", Description: "Optional product category hint."},
			},
			DefaultArgs: map[string]any{"limit": DefaultToolLimit},
			MaxLimit:    MaxDefaultToolLimit,
			OutputShape: "sku, current stock, days of cover, depletion risk, replenishment priority, estimated stockout date",
			SupportedIntents: []ChatIntent{
				ChatIntentStock, ChatIntentUnsafeAds, ChatIntentPriorities, ChatIntentGeneralOverview, ChatIntentAdvertising,
			},
		},
		{
			Name:        ToolGetAdvertisingAnalytics,
			Purpose:     "Get advertising performance and risk indicators.",
			Description: "Returns spend/revenue/orders/ROAS and campaign-level risk summary.",
			ReadOnly:    true,
			AllowedArgs: map[string]ToolArgDefinition{
				"limit":       {Type: "integer", Default: DefaultToolLimit, MinInt: &min1, MaxInt: &max20, Description: "Max campaigns to return."},
				"date_from":   {Type: "date", MaxDays: &max90Days, Description: "Optional start date."},
				"date_to":     {Type: "date", Description: "Optional end date."},
				"campaign_id": {Type: "integer", MinInt: &min1, Description: "Optional campaign id filter."},
			},
			DefaultArgs: map[string]any{"limit": DefaultToolLimit},
			MaxLimit:    MaxDefaultToolLimit,
			OutputShape: "total spend, campaigns, revenue, orders, ROAS, risk signals, linked SKU summary if available",
			SupportedIntents: []ChatIntent{
				ChatIntentAdvertising, ChatIntentAdLoss, ChatIntentUnsafeAds, ChatIntentPriorities, ChatIntentGeneralOverview,
			},
		},
		{
			Name:        ToolGetPriceEconomicsRisks,
			Purpose:     "Get price/economics risk signals (semantic alias).",
			Description: "Returns pricing and economics risk alerts, including margin and constraints issues.",
			ReadOnly:    true,
			AllowedArgs: map[string]ToolArgDefinition{
				"limit": {Type: "integer", Default: DefaultToolLimit, MinInt: &min1, MaxInt: &max20, Description: "Max risk rows to return."},
			},
			DefaultArgs: map[string]any{"limit": DefaultToolLimit},
			MaxLimit:    MaxDefaultToolLimit,
			OutputShape: "price/economics alerts, current price, min/max constraints, margin risk, missing constraints",
			SupportedIntents: []ChatIntent{
				ChatIntentPricing, ChatIntentPriorities, ChatIntentGeneralOverview, ChatIntentRecommendations,
			},
		},
		{
			Name:        ToolGetSKUMetrics,
			Purpose:     "Get SKU metrics table with sorting and filters.",
			Description: "Returns SKU revenue/orders/deltas/contribution with optional SKU or category filtering.",
			ReadOnly:    true,
			AllowedArgs: map[string]ToolArgDefinition{
				"limit":         {Type: "integer", Default: 20, MinInt: &min1, MaxInt: &max50, Description: "Max SKU rows to return."},
				"date_from":     {Type: "date", Description: "Optional start date."},
				"date_to":       {Type: "date", Description: "Optional end date."},
				"category_hint": {Type: "string", Description: "Optional category hint."},
				"sku":           {Type: "integer", MinInt: &min1, Description: "Optional SKU filter."},
				"offer_id":      {Type: "string", Description: "Optional offer id filter."},
				"sort_by":       {Type: "string", AllowedValues: sortByValues, Description: "Sort metric."},
			},
			DefaultArgs: map[string]any{"limit": 20, "sort_by": "revenue"},
			MaxLimit:    MaxSKUMetricsLimit,
			OutputShape: "sku metrics, revenue, orders, contribution, stock/price identifiers if available",
			SupportedIntents: []ChatIntent{
				ChatIntentSales, ChatIntentABCAnalysis, ChatIntentStock, ChatIntentPricing, ChatIntentGeneralOverview,
			},
		},
		{
			Name:        ToolGetSKUContext,
			Purpose:     "Get consolidated context for one SKU or offer.",
			Description: "Returns product, sales, stock, pricing, related alerts and recommendations; requires sku or offer_id.",
			ReadOnly:    true,
			AllowedArgs: map[string]ToolArgDefinition{
				"sku":      {Type: "integer", MinInt: &min1, Description: "SKU identifier (required if offer_id missing)."},
				"offer_id": {Type: "string", Description: "Offer identifier (required if sku missing)."},
			},
			DefaultArgs: map[string]any{},
			MaxLimit:    1,
			OutputShape: "product info, sales metrics, stock metrics, pricing/economics, related alerts, related recommendations",
			SupportedIntents: []ChatIntent{
				ChatIntentExplainRecommendation, ChatIntentSales, ChatIntentStock, ChatIntentPricing, ChatIntentAdvertising, ChatIntentRecommendations,
			},
		},
		{
			Name:        ToolGetCampaignContext,
			Purpose:     "Get consolidated context for one campaign.",
			Description: "Returns campaign KPI, linked SKU context and related alerts/recommendations.",
			ReadOnly:    true,
			AllowedArgs: map[string]ToolArgDefinition{
				"campaign_id": {Type: "integer", Required: true, MinInt: &min1, Description: "Campaign id."},
			},
			DefaultArgs: map[string]any{},
			MaxLimit:    1,
			OutputShape: "campaign metrics, spend, revenue, orders, ROAS, linked SKU, related alerts/recommendations",
			SupportedIntents: []ChatIntent{
				ChatIntentAdvertising, ChatIntentAdLoss, ChatIntentUnsafeAds,
				ChatIntentSales, ChatIntentPriorities, ChatIntentGeneralOverview,
			},
		},
		{
			Name:        ToolRunABCAnalysis,
			Purpose:     "Run deterministic backend ABC analysis.",
			Description: "Computes A/B/C classes from backend aggregates. Planner must not calculate ABC itself.",
			ReadOnly:    true,
			AllowedArgs: map[string]ToolArgDefinition{
				"category_hint": {Type: "string", Description: "Optional category filter."},
				"date_from":     {Type: "date", Description: "Optional start date."},
				"date_to":       {Type: "date", Description: "Optional end date."},
				"metric":        {Type: "string", Default: "revenue", AllowedValues: metricValues, Description: "ABC metric base."},
				"limit":         {Type: "integer", Default: 100, MinInt: &min1, MaxInt: &max200, Description: "Max SKU rows for analysis."},
			},
			DefaultArgs: map[string]any{"metric": "revenue", "limit": 100, "period_days": DefaultPeriodDays},
			MaxLimit:    MaxABCAnalysisLimit,
			OutputShape: "total SKU count, total revenue/orders, A/B/C class rows, revenue/order share, cumulative share, assumptions",
			SupportedIntents: []ChatIntent{
				ChatIntentABCAnalysis,
			},
		},
	}

	m := make(map[string]ToolDefinition, len(defs))
	for _, def := range defs {
		m[def.Name] = def
	}
	return &ToolRegistry{definitions: m}
}

func (r *ToolRegistry) Get(name string) (ToolDefinition, bool) {
	if r == nil {
		return ToolDefinition{}, false
	}
	def, ok := r.definitions[name]
	return def, ok
}

func (r *ToolRegistry) List() []ToolDefinition {
	if r == nil {
		return nil
	}
	out := make([]ToolDefinition, 0, len(r.definitions))
	for _, def := range r.definitions {
		out = append(out, def)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (r *ToolRegistry) Names() []string {
	defs := r.List()
	out := make([]string, 0, len(defs))
	for _, def := range defs {
		out = append(out, def.Name)
	}
	return out
}
