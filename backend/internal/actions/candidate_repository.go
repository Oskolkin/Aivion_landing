package actions

import (
	"context"
	"time"
)

// AccountSnapshotRow holds optional account-level KPIs for a metric date.
type AccountSnapshotRow struct {
	Revenue     *float64
	Orders      *int64
	RevenuePrev *float64
	OrdersPrev  *int64
	AdSpend7d   *float64
	Revenue7d   *float64
}

// SKUMetricRow is a per-SKU rollup for candidate builders.
type SKUMetricRow struct {
	OzonProductID int64
	SKU           *int64
	OfferID       *string
	ProductName   string

	HasMetricRow   bool
	OrdersCount    int32
	Revenue        float64
	StockAvailable int32
	DaysOfCover    *float64

	HasPrevMetricRow bool
	PrevOrdersCount  int32
	PrevRevenue      float64

	OrdersDelta  int32
	RevenueDelta float64

	Revenue7d float64
	Orders7d  int64

	RevenueSharePct *float64
	IsKeySKU        bool
}

// SKUMetricsBundle aggregates SKU rows for a metric date.
type SKUMetricsBundle struct {
	MetricDate     time.Time
	Rows           []SKUMetricRow
	StoreRevenue7d float64
	TotalScanned   int
}

// StockReplenishmentRow is a SKU eligible for stock_replenishment.
type StockReplenishmentRow struct {
	SKUMetricRow
	RevenueAtRisk7d  float64
	PriorityCritical bool
}

// NoSalesWithStockRow is a SKU with confirmed zero orders and positive stock.
type NoSalesWithStockRow struct {
	SKUMetricRow
	FrozenStockValue *float64
}

// SKUOrdersDropRow is a SKU with negative day-over-day orders delta.
type SKUOrdersDropRow struct {
	SKUMetricRow
}

// KeySKURiskRow is a key SKU with multiple risk signals.
type KeySKURiskRow struct {
	SKUMetricRow
	RiskTypes []string
	RiskScore int
}

// AdCampaignMetricRow holds 7-day campaign aggregates.
type AdCampaignMetricRow struct {
	CampaignExternalID string
	CampaignName       string
	Spend7d            float64
	Revenue7d          float64
	Orders7d           int64
	DRRPct             *float64
	HasMetricData      bool
}

// FunnelDataBundle holds funnel analytics for weakest-transition detection.
type FunnelDataBundle struct {
	HasAccountScope bool
	SourceStatus    string
	Current         *FunnelDayMetrics
	Previous        *FunnelDayMetrics
	History7d       []FunnelDayMetrics
}

type FunnelDayMetrics struct {
	MetricDate      time.Time
	Impressions     *int64
	ProductViews    *int64
	AddToCart       *int64
	Orders          *int64
	CompletedOrders *int64
}

// FunnelWeakTransitionRow is the weakest funnel transition candidate input.
type FunnelWeakTransitionRow struct {
	TransitionKey      string
	FromStage          string
	ToStage            string
	FromStageKey       string
	ToStageKey         string
	ConversionCurrent  float64
	ConversionPrevious *float64
	ConversionDeltaPP  *float64
	VolumeDeltaPct     *float64
	SKUContributorIDs  []string
}

// CandidateRepository loads data for candidate builders and persists entity groups.
type CandidateRepository interface {
	GetAccountSnapshot(ctx context.Context, sellerAccountID int64, metricDate time.Time) (AccountSnapshotRow, error)
	LoadSKUMetricsBundle(ctx context.Context, sellerAccountID int64, metricDate time.Time) (*SKUMetricsBundle, error)
	ListStockReplenishmentCandidateRows(ctx context.Context, sellerAccountID int64, metricDate time.Time, cfg CandidateBuilderConfig) ([]StockReplenishmentRow, error)
	ListNoSalesWithStockCandidateRows(ctx context.Context, sellerAccountID int64, metricDate time.Time) ([]NoSalesWithStockRow, error)
	ListSKUOrdersDropCandidateRows(ctx context.Context, sellerAccountID int64, metricDate time.Time) ([]SKUOrdersDropRow, error)
	ListKeySKURiskCandidateRows(ctx context.Context, sellerAccountID int64, metricDate time.Time, cfg CandidateBuilderConfig) ([]KeySKURiskRow, error)
	ListAdCampaignMetrics7d(ctx context.Context, sellerAccountID int64, metricDate time.Time) ([]AdCampaignMetricRow, error)
	CountAdCampaigns(ctx context.Context, sellerAccountID int64) (int64, error)
	GetFunnelData(ctx context.Context, sellerAccountID int64, metricDate time.Time) (*FunnelDataBundle, error)
	UpsertActionEntityGroup(ctx context.Context, sellerAccountID int64, metricDate time.Time, groupKey, entityType string, entityIDs []string, summary map[string]any) (*ActionEntityGroupRecord, error)
}
