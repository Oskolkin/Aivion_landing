package recommendations

import (
	"context"
	"time"
)

type Repository interface {
	GetDailyAccountMetricByDate(ctx context.Context, sellerAccountID int64, metricDate time.Time) (*AccountDailyMetric, error)
	ListDailySKUMetricsByDate(ctx context.Context, sellerAccountID int64, metricDate time.Time) ([]SKUDailyMetric, error)
	ListOpenAlerts(ctx context.Context, sellerAccountID int64, limit int, offset int) ([]AlertSignal, error)
	CountOpenAlertsBySeverity(ctx context.Context, sellerAccountID int64) ([]NamedCount, error)
	CountOpenAlertsByGroup(ctx context.Context, sellerAccountID int64) ([]NamedCount, error)
	GetLatestAlertRun(ctx context.Context, sellerAccountID int64) (*RunInfo, error)
	ListAdCampaignSummariesByDateRange(ctx context.Context, sellerAccountID int64, dateFrom time.Time, dateTo time.Time) ([]AdCampaignSummary, error)
	ListSKUEffectiveConstraintsBySellerAccountID(ctx context.Context, sellerAccountID int64) ([]EffectiveConstraint, error)
	ListOpenRecommendations(ctx context.Context, sellerAccountID int64, limit int, offset int) ([]RecommendationDigest, error)
	CountOpenRecommendations(ctx context.Context, sellerAccountID int64) (int64, error)
	CountOpenRecommendationsByPriority(ctx context.Context, sellerAccountID int64) ([]NamedCount, error)
	CountOpenRecommendationsByConfidence(ctx context.Context, sellerAccountID int64) ([]NamedCount, error)
	GetLatestRecommendationRun(ctx context.Context, sellerAccountID int64) (*RunInfo, error)
}

type ContextBuilder struct {
	repo   Repository
	limits ContextBuildLimits
}

func NewContextBuilder(repo Repository) *ContextBuilder {
	return NewContextBuilderWithLimits(repo, ContextBuildLimits{})
}

// NewContextBuilderWithLimits applies MVP context caps before OpenAI (see ApplyRecommendationContextBudget).
func NewContextBuilderWithLimits(repo Repository, limits ContextBuildLimits) *ContextBuilder {
	return &ContextBuilder{repo: repo, limits: limits}
}

type AIRecommendationContext struct {
	ContextVersion  string                 `json:"context_version"`
	SellerAccountID int64                  `json:"seller_account_id"`
	AsOfDate        string                 `json:"as_of_date"`
	GeneratedAt     time.Time              `json:"generated_at"`
	Windows         ContextWindows         `json:"windows"`
	Account         AccountContext         `json:"account"`
	Alerts          AlertsContext          `json:"alerts"`
	Recommendations RecommendationsContext `json:"recommendations"`
	Merchandising   MerchandisingContext   `json:"merchandising"`
	Advertising     AdvertisingContext     `json:"advertising"`
	Pricing         PricingContext         `json:"pricing"`
	// ContextTruncated is true when item caps or byte budget trimming was applied.
	ContextTruncated bool `json:"context_truncated,omitempty"`
	// ContextTruncationReason explains truncation (MVP; no raw payloads).
	ContextTruncationReason string `json:"context_truncation_reason,omitempty"`
	// ContextApproxUncompressedBytes is len(json.Marshal(this context)) after budgeting (approx wire size).
	ContextApproxUncompressedBytes int `json:"context_approx_uncompressed_bytes,omitempty"`
}

type ContextWindows struct {
	PreviousDate string `json:"previous_date"`
	AdsDateFrom  string `json:"ads_date_from"`
	AdsDateTo    string `json:"ads_date_to"`
}

type AccountContext struct {
	Current         *AccountDailyMetric `json:"current,omitempty"`
	Previous        *AccountDailyMetric `json:"previous,omitempty"`
	RevenueDeltaPct *float64            `json:"revenue_delta_pct,omitempty"`
	OrdersDeltaPct  *float64            `json:"orders_delta_pct,omitempty"`
}

type AlertsContext struct {
	OpenTotal             int64         `json:"open_total"`
	OpenCriticalHighCount int           `json:"open_critical_high_count"`
	BySeverity            []NamedCount  `json:"by_severity"`
	ByGroup               []NamedCount  `json:"by_group"`
	TopOpen               []AlertSignal `json:"top_open"`
	LatestRun             *RunInfo      `json:"latest_run,omitempty"`
}

type RecommendationsContext struct {
	OpenTotal    int64                  `json:"open_total"`
	ByPriority   []NamedCount           `json:"by_priority"`
	ByConfidence []NamedCount           `json:"by_confidence"`
	TopOpen      []RecommendationDigest `json:"top_open"`
	LatestRun    *RunInfo               `json:"latest_run,omitempty"`
}

type MerchandisingContext struct {
	TotalSKUs        int              `json:"total_skus"`
	TopRevenueSKUs   []SKUDailyMetric `json:"top_revenue_skus"`
	TopOrderDropSKUs []SKUDailyMetric `json:"top_order_drop_skus"`
	LowStockSKUs     []SKUDailyMetric `json:"low_stock_skus"`
}

type AdvertisingContext struct {
	TopCampaigns []AdCampaignSummary `json:"top_campaigns"`
}

type PricingContext struct {
	EffectiveConstraintsCount int                   `json:"effective_constraints_count"`
	TopConstrainedSKUs        []EffectiveConstraint `json:"top_constrained_skus"`
}

type AccountDailyMetric struct {
	MetricDate              string   `json:"metric_date"`
	Revenue                 float64  `json:"revenue"`
	OrdersCount             int32    `json:"orders_count"`
	ReturnsCount            int32    `json:"returns_count"`
	CancelCount             int32    `json:"cancel_count"`
	AverageOrderValue       *float64 `json:"average_order_value,omitempty"`
	AccountAdSpendToRevenue *float64 `json:"account_ad_spend_to_revenue,omitempty"`
	CancellationRate        *float64 `json:"cancellation_rate,omitempty"`
}

type SKUDailyMetric struct {
	OzonProductID            int64    `json:"ozon_product_id"`
	SKU                      *int64   `json:"sku,omitempty"`
	OfferID                  *string  `json:"offer_id,omitempty"`
	ProductName              *string  `json:"product_name,omitempty"`
	Revenue                  float64  `json:"revenue"`
	OrdersCount              int32    `json:"orders_count"`
	StockAvailable           int32    `json:"stock_available"`
	DaysOfCover              *float64 `json:"days_of_cover,omitempty"`
	SKUOrdersDeltaPercent    *float64 `json:"sku_orders_delta_percent,omitempty"`
	SKUSalesVelocityOrders7D *float64 `json:"sku_sales_velocity_orders_7d,omitempty"`
	StockStatusBucket        *string  `json:"stock_status_bucket,omitempty"`
}

type AlertSignal struct {
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
	LastSeenAt    time.Time      `json:"last_seen_at"`
	Evidence      map[string]any `json:"evidence,omitempty"`
}

type AdCampaignSummary struct {
	CampaignExternalID int64    `json:"campaign_external_id"`
	CampaignName       string   `json:"campaign_name"`
	CampaignType       *string  `json:"campaign_type,omitempty"`
	Status             *string  `json:"status,omitempty"`
	SpendTotal         float64  `json:"spend_total"`
	OrdersTotal        int64    `json:"orders_total"`
	RevenueTotal       float64  `json:"revenue_total"`
	CampaignDRR        *float64 `json:"campaign_drr,omitempty"`
	CampaignROAS       *float64 `json:"campaign_roas,omitempty"`
}

type EffectiveConstraint struct {
	OzonProductID     int64    `json:"ozon_product_id"`
	SKU               *int64   `json:"sku,omitempty"`
	OfferID           *string  `json:"offer_id,omitempty"`
	RuleID            int64    `json:"rule_id"`
	ResolvedFrom      string   `json:"resolved_from"`
	EffectiveMinPrice *float64 `json:"effective_min_price,omitempty"`
	EffectiveMaxPrice *float64 `json:"effective_max_price,omitempty"`
	ReferencePrice    *float64 `json:"reference_price,omitempty"`
	ImpliedCost       *float64 `json:"implied_cost,omitempty"`
}

type RecommendationDigest struct {
	ID                 int64     `json:"id"`
	RecommendationType string    `json:"recommendation_type"`
	Horizon            string    `json:"horizon"`
	EntityType         string    `json:"entity_type"`
	EntityID           *string   `json:"entity_id,omitempty"`
	EntitySKU          *int64    `json:"entity_sku,omitempty"`
	EntityOfferID      *string   `json:"entity_offer_id,omitempty"`
	Title              string    `json:"title"`
	PriorityScore      float64   `json:"priority_score"`
	PriorityLevel      string    `json:"priority_level"`
	Urgency            string    `json:"urgency"`
	ConfidenceLevel    string    `json:"confidence_level"`
	LastSeenAt         time.Time `json:"last_seen_at"`
}

type RunInfo struct {
	ID                            int64      `json:"id"`
	RunType                       string     `json:"run_type"`
	Status                        string     `json:"status"`
	StartedAt                     time.Time  `json:"started_at"`
	FinishedAt                    *time.Time `json:"finished_at,omitempty"`
	AsOfDate                      *string    `json:"as_of_date,omitempty"`
	AIModel                       *string    `json:"ai_model,omitempty"`
	AIPromptVersion               *string    `json:"ai_prompt_version,omitempty"`
	InputTokens                   int        `json:"input_tokens,omitempty"`
	OutputTokens                  int        `json:"output_tokens,omitempty"`
	EstimatedCost                 float64    `json:"estimated_cost,omitempty"`
	GeneratedRecommendationsCount int        `json:"generated_recommendations_count,omitempty"`
	ErrorMessage                  *string    `json:"error_message,omitempty"`
}

type RecommendationFeedbackRating string

const (
	RecommendationFeedbackPositive RecommendationFeedbackRating = "positive"
	RecommendationFeedbackNegative RecommendationFeedbackRating = "negative"
	RecommendationFeedbackNeutral  RecommendationFeedbackRating = "neutral"
)

type RecommendationFeedback struct {
	ID               int64
	RecommendationID int64
	SellerAccountID  int64
	Rating           RecommendationFeedbackRating
	Comment          *string
	CreatedAt        time.Time
}

type AddRecommendationFeedbackInput struct {
	SellerAccountID  int64
	RecommendationID int64
	Rating           RecommendationFeedbackRating
	Comment          *string
}

type NamedCount struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}
