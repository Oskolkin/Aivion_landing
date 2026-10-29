package dashboardfacts

import "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"

const SchemaVersion = "unified_dashboard_facts_v1"

// PackIdentity binds facts to the analysis pack they were derived from.
type PackIdentity struct {
	PackID   string
	PackHash string
}

// DashboardFactsV1 is the immutable factual dashboard layer for Unified Analysis.
type DashboardFactsV1 struct {
	SchemaVersion    string                        `json:"schema_version"`
	SellerAccountID  int64                         `json:"seller_account_id"`
	MetricDate       string                        `json:"metric_date"`
	PackID           string                        `json:"pack_id"`
	PackHash         string                        `json:"pack_hash"`
	FactsHash        string                        `json:"facts_hash"`
	DashboardMetrics analytics.DashboardMetricsDTO `json:"dashboard_metrics"`
	SellerPulse      SellerPulseFacts              `json:"seller_pulse"`
	StockCoverage    StockCoverageFacts            `json:"stock_coverage"`
	AdEfficiency     AdEfficiencyFacts             `json:"ad_efficiency"`
	Funnel           FunnelFacts                   `json:"funnel"`
	SKUFactsIndex    SKUFactsIndex                 `json:"sku_facts_index"`
	KeySKUCohort     KeySKUCohortFacts             `json:"key_sku_cohort"`
	Categories       []CategoryFacts               `json:"categories,omitempty"`
	DataCoverage     DataCoverage                  `json:"data_coverage"`
	ContentCounts    ContentCounts                 `json:"content_counts"`
}

// Key SKU 80% revenue cohort policy (backend-owned; not frontend).
const (
	KeySKUCohortThresholdPct  = 70.0
	KeySKUCohortPolicyVersion = "key_sku_cohort_v1"
)

// KeySKUCohortFacts is the backend-owned 80% revenue cohort over 7 days.
// Status is available | not_enough_data.
type KeySKUCohortFacts struct {
	Status                    string             `json:"status"`
	StoreTotalRevenue7d       float64            `json:"store_total_revenue_7d"`
	KeySKUsTotalRevenue7d     float64            `json:"key_skus_total_revenue_7d"`
	CumulativeRevenueSharePct float64            `json:"cumulative_revenue_share_pct"`
	ThresholdPct              float64            `json:"threshold_pct"`
	PolicyVersion             string             `json:"policy_version"`
	Items                     []KeySKUCohortItem `json:"items"`
}

type KeySKUCohortItem struct {
	OzonProductID             int64   `json:"ozon_product_id"`
	SKU                       *int64  `json:"sku,omitempty"`
	OfferID                   *string `json:"offer_id,omitempty"`
	ProductName               *string `json:"product_name,omitempty"`
	Revenue7d                 float64 `json:"revenue_7d"`
	Orders7d                  int64   `json:"orders_7d"`
	RevenueSharePct           float64 `json:"revenue_share_pct"`
	CumulativeRevenueSharePct float64 `json:"cumulative_revenue_share_pct"`
	Rank                      int     `json:"rank"`
}

type SellerPulseFacts struct {
	KPIs    []KPICardFact   `json:"kpis"`
	Trend7D []TrendDayPoint `json:"trend_7d"`
}

type KPICardFact struct {
	Key         string           `json:"key"`
	Label       string           `json:"label"`
	Value       string           `json:"value"`
	Delta       *KPICardDelta    `json:"delta,omitempty"`
	Sparkline7D []SparklinePoint `json:"sparkline_7d,omitempty"`
	Hint        string           `json:"hint,omitempty"`
}

type KPICardDelta struct {
	Abs     float64  `json:"abs"`
	Pct     *float64 `json:"pct,omitempty"`
	Label   string   `json:"label"`
	Unit    string   `json:"unit,omitempty"`
	Display string   `json:"display,omitempty"`
}

type SparklinePoint struct {
	Date    string   `json:"date"`
	Value   *float64 `json:"value,omitempty"`
	Revenue *float64 `json:"revenue,omitempty"`
	Stock   *int32   `json:"stock,omitempty"`
	HasData bool     `json:"has_data"`
}

type TrendDayPoint struct {
	Date    string   `json:"date"`
	Revenue *float64 `json:"revenue,omitempty"`
	Orders  *int32   `json:"orders,omitempty"`
}

type StockCoverageFacts struct {
	Status    string                 `json:"status"`
	TotalSKUs int                    `json:"total_skus"`
	Segments  []StockCoverageSegment `json:"segments"`
}

type StockCoverageSegment struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	SKUCount int    `json:"sku_count"`
	Severity string `json:"severity"`
}

type AdEfficiencyFacts struct {
	Status           string                   `json:"status"`
	Period           *AdEfficiencyPeriod      `json:"period,omitempty"`
	Summary          *AdEfficiencySummary     `json:"summary,omitempty"`
	Points           []AdEfficiencyPoint      `json:"points,omitempty"`
	HighDRRCampaigns []AdCampaignDisplayRow   `json:"high_drr_campaigns,omitempty"`
	NoOrderCampaigns []AdCampaignDisplayRow   `json:"no_order_campaigns,omitempty"`
	Trend7D          []AdEfficiencyTrendPoint `json:"trend_7d,omitempty"`
}

type AdEfficiencyPeriod struct {
	MetricDate            string `json:"metric_date"`
	DateFrom              string `json:"date_from"`
	DateTo                string `json:"date_to"`
	CurrentCompletedDate  string `json:"current_completed_date"`
	PreviousCompletedDate string `json:"previous_completed_date"`
	Current7dDateFrom     string `json:"current_7d_date_from"`
	Current7dDateTo       string `json:"current_7d_date_to"`
	Previous7dDateFrom    string `json:"previous_7d_date_from"`
	Previous7dDateTo      string `json:"previous_7d_date_to"`
	SampleDays            int    `json:"sample_days"`
	PreviousSampleDays    int    `json:"previous_sample_days"`
}

type AdEfficiencySummary struct {
	// CurrentDRRPct / AverageDRR7dPct = ad spend / attributed ad revenue × 100.
	CurrentDRRPct              *float64 `json:"current_drr_pct,omitempty"`
	PreviousDRRPct             *float64 `json:"previous_drr_pct,omitempty"`
	AverageDRR7dPct            *float64 `json:"average_drr_7d_pct,omitempty"`
	DeltaVsPreviousPp          *float64 `json:"delta_vs_previous_pp,omitempty"`
	DeltaVsAverage7dPp         *float64 `json:"delta_vs_average_7d_pp,omitempty"`
	PreviousDRR7dPct           *float64 `json:"previous_drr_7d_pct,omitempty"`
	DeltaVsPrevious7dPp        *float64 `json:"delta_vs_previous_7d_pp,omitempty"`
	TotalSpendToday            float64  `json:"total_spend_today"`
	TotalRevenueToday          float64  `json:"total_revenue_today"` // attributed ad revenue
	TotalStoreRevenueToday     float64  `json:"total_store_revenue_today"`
	TotalOrdersToday           int64    `json:"total_orders_today"`
	TotalSpend7d               float64  `json:"total_spend_7d"`
	TotalAttributedRevenue7d   float64  `json:"total_attributed_revenue_7d"`
	TotalStoreRevenue7d        float64  `json:"total_store_revenue_7d"`
	CampaignsHighDRRCount      int      `json:"campaigns_high_drr_count"`
	CampaignsNoOrdersCount     int      `json:"campaigns_no_orders_count"`
	HighDRRSpend               float64  `json:"high_drr_spend"`
	SpendWithoutOrders         float64  `json:"spend_without_orders"`
	SpendWithoutOrdersDateFrom string   `json:"spend_without_orders_date_from,omitempty"`
	SpendWithoutOrdersDateTo   string   `json:"spend_without_orders_date_to,omitempty"`
	HighDRRThresholdPct        float64  `json:"high_drr_threshold_pct"`
}

type AdEfficiencyTrendPoint struct {
	Date        string   `json:"date"`
	Spend       float64  `json:"spend"`
	Revenue     float64  `json:"revenue"`
	Orders      int64    `json:"orders"`
	DRRPct      *float64 `json:"drr_pct,omitempty"`
	IsToday     bool     `json:"is_today"`
	IsYesterday bool     `json:"is_yesterday"`
	DataStatus  string   `json:"data_status"`
}

type AdEfficiencyPoint struct {
	CampaignExternalID int64    `json:"campaign_external_id"`
	CampaignName       string   `json:"campaign_name"`
	Spend              float64  `json:"spend"`
	DRR                *float64 `json:"drr,omitempty"`
	Revenue            float64  `json:"revenue"`
	Orders             int64    `json:"orders"`
	EfficiencySignal   string   `json:"efficiency_signal,omitempty"`
}

type AdCampaignDisplayRow struct {
	CampaignExternalID int64    `json:"campaign_external_id"`
	CampaignName       string   `json:"campaign_name"`
	DRRPct             *float64 `json:"drr_pct,omitempty"`
	Spend              float64  `json:"spend"`
	Orders             int64    `json:"orders"`
	Revenue            float64  `json:"revenue"`
	ReasonCode         string   `json:"reason_code,omitempty"`
}

type FunnelFacts struct {
	Status                string             `json:"status"`
	CurrentCompletedDate  string             `json:"current_completed_date"`
	PreviousCompletedDate string             `json:"previous_completed_date"`
	Stages                []FunnelStage      `json:"stages,omitempty"`
	DailySeries           []FunnelDailyPoint `json:"daily_series"`
	Overview7D            FunnelWeeklyFacts  `json:"overview_7d"`
}

type FunnelWeeklyFacts struct {
	Status string              `json:"status"`
	Stages []FunnelWeeklyStage `json:"stages,omitempty"`
}

type FunnelWeeklyStage struct {
	Key                            string   `json:"key"`
	Label                          string   `json:"label"`
	Value                          *int64   `json:"value,omitempty"`
	PreviousPeriodValue            *int64   `json:"previous_period_value"`
	ChangeVsPreviousPeriodPct      *float64 `json:"change_vs_previous_period_pct"`
	ConversionFromPrevious         *float64 `json:"conversion_from_previous,omitempty"`
	PreviousConversionFromPrevious *float64 `json:"previous_conversion_from_previous"`
	ConversionDeltaPp              *float64 `json:"conversion_delta_pp"`
}

// FunnelDailyPoint is one observed, completed account-level funnel day.
// Nullable metrics preserve source absence; zero remains a real observation.
type FunnelDailyPoint struct {
	Date           string `json:"date"`
	Impressions    *int64 `json:"impressions"`
	ProductViews   *int64 `json:"product_views"`
	AddToCart      *int64 `json:"add_to_cart"`
	OrderedUnits   *int64 `json:"ordered_units"`
	DeliveredUnits *int64 `json:"delivered_units"`
}

type FunnelStage struct {
	Key                    string   `json:"key"`
	Label                  string   `json:"label"`
	Value                  *int64   `json:"value,omitempty"`
	ConversionFromPrevious *float64 `json:"conversion_from_previous,omitempty"`
	ConversionFromTop      *float64 `json:"conversion_from_top,omitempty"`
	PreviousDayValue       *int64   `json:"previous_day_value"`
	// PreviousConversionFromPrevious is the same stage conversion in the
	// previous completed day (D-2).
	PreviousConversionFromPrevious *float64 `json:"previous_conversion_from_previous"`
	// ConversionDeltaPp is current − previous conversion in percentage points.
	ConversionDeltaPp      *float64 `json:"conversion_delta_pp"`
	ChangeVsPreviousDayPct *float64 `json:"change_vs_previous_day_pct"`
	// PeriodType documents that Value is one completed day (D-1).
	PeriodType string `json:"period_type,omitempty"`
}

type SKUFactsIndex struct {
	ByOzonProductID map[string]SKUFactRecord `json:"by_ozon_product_id"`
}

type SKUFactRecord struct {
	OzonProductID int64   `json:"ozon_product_id"`
	OfferID       *string `json:"offer_id,omitempty"`
	SKU           *int64  `json:"sku,omitempty"`
	ProductName   *string `json:"product_name,omitempty"`
	Revenue       float64 `json:"revenue"`
	Orders        int32   `json:"orders"`
	Revenue7d     float64 `json:"revenue_7d"`
	Orders7d      int64   `json:"orders_7d"`
	// Previous 7d window (metric_date-13..metric_date-7) for week-over-week
	// order-drop SKU Focus selection.
	RevenuePrev7d        float64  `json:"revenue_prev_7d"`
	OrdersPrev7d         int64    `json:"orders_prev_7d"`
	OrdersDeltaDayToDay  int32    `json:"orders_delta_day_to_day"`
	RevenueDeltaDayToDay float64  `json:"revenue_delta_day_to_day"`
	StockAvailable       int32    `json:"stock_available"`
	DaysOfCover          *float64 `json:"days_of_cover,omitempty"`
	CoverageBucket       string   `json:"coverage_bucket,omitempty"`
	DaysWithoutSales     *int32   `json:"days_without_sales,omitempty"`
	// Sparkline7D covers metric_date-6..metric_date (orders+revenue per
	// day) sourced from snapshot.SKUMetrics. Days with no
	// daily_sku_metrics row for this product are HasData=false with
	// Value/Revenue left nil — missing days are never coerced to zero.
	Sparkline7D []SparklinePoint `json:"sparkline_7d,omitempty"`
}

// CategoryFacts is the immutable per-category factual rollup for the
// current metric_date, built by joining snapshot.Products (ozon_product_id
// -> description_category_id) and snapshot.Categories (id -> name) against
// snapshot.SKUMetrics for the current and previous day. Categories with no
// same-day SKU metrics are omitted (nothing factual to report), even if the
// seller's catalog is categorized (see DataCoverage.CategoryCount).
type CategoryFacts struct {
	CategoryID           int64    `json:"category_id"`
	CategoryName         string   `json:"category_name"`
	SKUCount             int      `json:"sku_count"`
	Revenue              float64  `json:"revenue"`
	Orders               int32    `json:"orders"`
	RevenueShare         *float64 `json:"revenue_share,omitempty"`
	OrdersShare          *float64 `json:"orders_share,omitempty"`
	PreviousRevenue      float64  `json:"previous_revenue"`
	PreviousOrders       int32    `json:"previous_orders"`
	RevenueDeltaDayToDay float64  `json:"revenue_delta_day_to_day"`
	OrdersDeltaDayToDay  int32    `json:"orders_delta_day_to_day"`
	StockRiskSKUCount    int      `json:"stock_risk_sku_count"`
}

type DataCoverage struct {
	AccountMetricsDays int  `json:"account_metrics_days"`
	SKUMetricsDays     int  `json:"sku_metrics_days"`
	FunnelAvailable    bool `json:"funnel_available"`
	AdMetricsDays      int  `json:"ad_metrics_days"`
	StockSnapshotRows  int  `json:"stock_snapshot_rows"`
	// CategoryCount is the number of distinct categories referenced by the
	// seller's product catalog (snapshot.Products x snapshot.Categories),
	// independent of whether those categories have same-day SKU metrics.
	// Used to detect a seller whose catalog is categorized but whose
	// Categories facts came out empty (see validateDashboardFacts).
	CategoryCount int `json:"category_count"`
}

type ContentCounts struct {
	SellerPulseKPIs      int `json:"seller_pulse_kpis"`
	SellerPulseTrendDays int `json:"seller_pulse_trend_days"`
	StockSegments        int `json:"stock_segments"`
	FunnelStages         int `json:"funnel_stages"`
	AdPoints             int `json:"ad_points"`
	SKUIndexEntries      int `json:"sku_index_entries"`
	// SKUsWithSparkline counts SKUFactsIndex entries whose Sparkline7D has
	// at least 2 observed (HasData) days.
	SKUsWithSparkline int `json:"sku_with_sparkline_count"`
	// CategoryCount is len(Categories) — the categories actually built for
	// this metric_date (as opposed to DataCoverage.CategoryCount, which
	// counts catalog-level category availability).
	CategoryCount int `json:"category_count"`
}

type FactsValidationIssue struct {
	Code    string `json:"code"`
	Path    string `json:"path"`
	Message string `json:"message"`
}

type FactsValidationReport struct {
	Valid    bool                   `json:"valid"`
	Errors   []FactsValidationIssue `json:"errors"`
	Warnings []FactsValidationIssue `json:"warnings"`
}
