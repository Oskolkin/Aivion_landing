package aidatapack

import "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"

const SchemaVersion = "ai_data_pack_v1"

const (
	MarketplaceOzon = "ozon"
	DefaultTimezone = "Europe/Moscow"
	// MaxSKUsTechnicalLimit is a technical in-memory safeguard only - not business selection.
	MaxSKUsTechnicalLimit = 50000
	StockSemanticCurrent  = "current_snapshot"
)

// SellerAIDataPack is the normalized JSON-ready data layer for AI consumers.
type SellerAIDataPack struct {
	SchemaVersion  string           `json:"schema_version"`
	SellerContext  SellerContext    `json:"seller_context"`
	DataWindows    DataWindows      `json:"data_windows"`
	DataQuality    DataQuality      `json:"data_quality"`
	EntityRegistry EntityRegistry   `json:"entity_registry"`
	FactRegistry   map[string]Fact  `json:"fact_registry"`
	Tables         Tables           `json:"tables"`
	Constraints    ConstraintsBlock `json:"constraints"`
}

type SellerContext struct {
	SellerAccountID int64  `json:"seller_account_id"`
	Marketplace     string `json:"marketplace"`
	MetricDate      string `json:"metric_date"`
	Timezone        string `json:"timezone"`
}

// DateRange is an alias for analysisdata.DateRange (shared window contract).
type DateRange = analysisdata.DateRange

// DataWindows is an alias for analysisdata.DataWindows (shared window contract).
type DataWindows = analysisdata.DataWindows

type DataQualityLimitation struct {
	Code     string   `json:"code"`
	Scope    []string `json:"scope"`
	Message  string   `json:"message"`
	Severity string   `json:"severity"`
}

type DataQuality struct {
	Sources               map[string]any          `json:"sources"`
	Limitations           []string                `json:"limitations"`
	StructuredLimitations []DataQualityLimitation `json:"structured_limitations,omitempty"`
	ContextCompaction     map[string]any          `json:"context_compaction,omitempty"`
}

type EntityRegistry struct {
	SKUs       map[string]SKUEntity       `json:"skus"`
	SKUGroups  map[string]SKUGroupEntity  `json:"sku_groups"`
	Campaigns  map[string]CampaignEntity  `json:"campaigns"`
	Warehouses map[string]WarehouseEntity `json:"warehouses"`
	Categories map[string]CategoryEntity  `json:"categories"`
}

type SKUEntity struct {
	EntityID      string   `json:"entity_id"`
	EntityType    string   `json:"entity_type"`
	SKU           string   `json:"sku"`
	OzonProductID string   `json:"ozon_product_id,omitempty"`
	OfferID       string   `json:"offer_id,omitempty"`
	Name          string   `json:"name,omitempty"`
	CategoryID    string   `json:"category_id,omitempty"`
	Status        string   `json:"status,omitempty"`
	IsArchived    bool     `json:"is_archived,omitempty"`
	ImageURL      *string  `json:"image_url"`
	Sources       []string `json:"sources,omitempty"`
}

type RelatedSKUEntry struct {
	SKU               string  `json:"sku"`
	DeliverySchema    string  `json:"delivery_schema,omitempty"`
	Availability      string  `json:"availability,omitempty"`
	DeletedAt         *string `json:"deleted_at"`
	IsKnownProductSKU bool    `json:"is_known_product_sku"`
}

type SKUGroupEntity struct {
	EntityID    string            `json:"entity_id"`
	EntityType  string            `json:"entity_type"`
	ProductID   string            `json:"product_id,omitempty"`
	PrimarySKU  string            `json:"primary_sku,omitempty"`
	RelatedSKUs []RelatedSKUEntry `json:"related_skus"`
}

type CampaignBudget struct {
	DailyAmount  *float64 `json:"daily_amount,omitempty"`
	WeeklyAmount *float64 `json:"weekly_amount,omitempty"`
	TotalAmount  *float64 `json:"total_amount,omitempty"`
	Currency     string   `json:"currency,omitempty"`
}

type CampaignEntity struct {
	EntityID                 string         `json:"entity_id"`
	EntityType               string         `json:"entity_type"`
	CampaignExternalID       string         `json:"campaign_external_id"`
	Name                     string         `json:"name,omitempty"`
	State                    string         `json:"state,omitempty"`
	PaymentType              string         `json:"payment_type,omitempty"`
	AdvObjectType            string         `json:"adv_object_type,omitempty"`
	Placement                []string       `json:"placement,omitempty"`
	ProductCampaignMode      string         `json:"product_campaign_mode,omitempty"`
	ProductAutopilotStrategy string         `json:"product_autopilot_strategy,omitempty"`
	Budget                   CampaignBudget `json:"budget,omitempty"`
}

type WarehouseEntity struct {
	EntityID    string `json:"entity_id"`
	EntityType  string `json:"entity_type"`
	WarehouseID string `json:"warehouse_id"`
	Name        string `json:"name,omitempty"`
}

type CategoryEntity struct {
	EntityID   string  `json:"entity_id"`
	EntityType string  `json:"entity_type"`
	CategoryID string  `json:"category_id"`
	Name       string  `json:"name,omitempty"`
	Path       *string `json:"path"`
}

type Fact struct {
	FactID         string     `json:"fact_id"`
	EntityType     string     `json:"entity_type"`
	EntityID       string     `json:"entity_id"`
	Metric         string     `json:"metric"`
	Window         string     `json:"window,omitempty"`
	Date           string     `json:"date,omitempty"`
	DateFrom       string     `json:"date_from,omitempty"`
	DateTo         string     `json:"date_to,omitempty"`
	Value          any        `json:"value"`
	Unit           string     `json:"unit"`
	FormattedValue string     `json:"formatted_value,omitempty"`
	Source         FactSource `json:"source"`
}

type FactSource struct {
	Table       string `json:"table"`
	Field       string `json:"field"`
	Calculation string `json:"calculation,omitempty"`
}

type Tables struct {
	AccountRollup        AccountRollupTable       `json:"account_rollup"`
	SKURollups           []SKURollupRow           `json:"sku_rollups"`
	ProductEnrichment    []ProductEnrichmentRow   `json:"product_enrichment"`
	StockPositions       []StockPositionRow       `json:"stock_positions"`
	CampaignRollups      []CampaignRollupRow      `json:"campaign_rollups"`
	CampaignProductLinks []CampaignProductLinkRow `json:"campaign_product_links"`
	FunnelRollup         FunnelRollupTable        `json:"funnel_rollup"`
	CategoryRollups      []CategoryRollupRow      `json:"category_rollups"`
	FinanceRollup        FinanceRollupTable       `json:"finance_rollup"`
}

type AccountRollupTable struct {
	DataStatus string         `json:"data_status"`
	Reason     string         `json:"reason,omitempty"`
	CurrentDay map[string]any `json:"current_day,omitempty"`
	Yesterday  map[string]any `json:"yesterday,omitempty"`
	Last7d     map[string]any `json:"last_7d,omitempty"`
	Previous7d map[string]any `json:"previous_7d,omitempty"`
	Deltas     map[string]any `json:"deltas,omitempty"`
	FactIDs    []string       `json:"fact_ids,omitempty"`
}

type SKURollupRow struct {
	EntityID                string         `json:"entity_id"`
	SKU                     string         `json:"sku"`
	OfferID                 string         `json:"offer_id,omitempty"`
	Name                    string         `json:"name,omitempty"`
	CurrentDay              map[string]any `json:"current_day,omitempty"`
	Last7d                  map[string]any `json:"last_7d,omitempty"`
	Previous7d              map[string]any `json:"previous_7d,omitempty"`
	Deltas                  map[string]any `json:"deltas,omitempty"`
	Shares                  map[string]any `json:"shares,omitempty"`
	ProductEnrichmentStatus string         `json:"product_enrichment_status,omitempty"`
	FactIDs                 []string       `json:"fact_ids,omitempty"`
}

type ProductEnrichmentRow struct {
	EntityID     string         `json:"entity_id"`
	SnapshotDate string         `json:"snapshot_date"`
	DataStatus   string         `json:"data_status"`
	Price        *float64       `json:"price,omitempty"`
	OldPrice     *float64       `json:"old_price,omitempty"`
	MinPrice     *float64       `json:"min_price,omitempty"`
	Currency     string         `json:"currency,omitempty"`
	Status       map[string]any `json:"status,omitempty"`
	Visibility   map[string]any `json:"visibility,omitempty"`
	PriceIndexes map[string]any `json:"price_indexes,omitempty"`
	Commissions  []any          `json:"commissions"`
	Promotions   []any          `json:"promotions"`
	Errors       []any          `json:"errors"`
	Source       string         `json:"source,omitempty"`
	FactIDs      []string       `json:"fact_ids,omitempty"`
}

type StockPositionRow struct {
	EntityID                       string   `json:"entity_id"`
	SnapshotDate                   string   `json:"snapshot_date"`
	DataStatus                     string   `json:"data_status"`
	AvailableStockCount            int32    `json:"available_stock_count"`
	ValidStockCount                int32    `json:"valid_stock_count"`
	RequestedStockCount            int32    `json:"requested_stock_count"`
	TransitStockCount              int32    `json:"transit_stock_count"`
	WaitingDocsStockCount          int32    `json:"waiting_docs_stock_count"`
	ExcessStockCount               int32    `json:"excess_stock_count"`
	ExpiringStockCount             int32    `json:"expiring_stock_count"`
	ReturnFromCustomerStockCount   int32    `json:"return_from_customer_stock_count"`
	ReturnToSellerStockCount       int32    `json:"return_to_seller_stock_count"`
	StockDefectStockCount          int32    `json:"stock_defect_stock_count"`
	TransitDefectStockCount        int32    `json:"transit_defect_stock_count"`
	DaysWithoutSales               int32    `json:"days_without_sales"`
	Ads28d                         *float64 `json:"ads_28d,omitempty"`
	IDCDays                        *float64 `json:"idc_days,omitempty"`
	TurnoverGrade                  string   `json:"turnover_grade,omitempty"`
	SourceRowsCount                int      `json:"source_rows_count,omitempty"`
	SourceTurnoverGrades           []string `json:"source_turnover_grades,omitempty"`
	TurnoverGradeAggregationMethod string   `json:"turnover_grade_aggregation_method,omitempty"`
	WarehouseBreakdown             []any    `json:"warehouse_breakdown"`
	Source                         string   `json:"source,omitempty"`
	FactIDs                        []string `json:"fact_ids,omitempty"`
}

// CampaignMetricsWindow holds campaign ad metrics for a time window.
// Base metrics always serialize as numbers (including 0). Derived ratios may be omitted when null.
type CampaignMetricsWindow struct {
	Impressions         int64    `json:"impressions"`
	Clicks              int64    `json:"clicks"`
	Spend               float64  `json:"spend"`
	Orders              int64    `json:"orders"`
	Revenue             float64  `json:"revenue"`
	CTRPct              *float64 `json:"ctr_pct,omitempty"`
	CPC                 *float64 `json:"cpc,omitempty"`
	CPO                 *float64 `json:"cpo,omitempty"`
	ROAS                *float64 `json:"roas,omitempty"`
	DRRPct              *float64 `json:"drr_pct,omitempty"`
	SpendWithoutOrders  bool     `json:"spend_without_orders"`
	SpendWithoutRevenue bool     `json:"spend_without_revenue"`
}

type CampaignRollupRow struct {
	EntityID            string                `json:"entity_id"`
	CampaignExternalID  string                `json:"campaign_external_id"`
	Name                string                `json:"name,omitempty"`
	State               string                `json:"state,omitempty"`
	PaymentType         string                `json:"payment_type,omitempty"`
	AdvObjectType       string                `json:"adv_object_type,omitempty"`
	ProductCampaignMode string                `json:"product_campaign_mode,omitempty"`
	Placement           []string              `json:"placement"`
	Budget              CampaignBudget        `json:"budget,omitempty"`
	CurrentDay          CampaignMetricsWindow `json:"current_day,omitempty"`
	Last7d              CampaignMetricsWindow `json:"last_7d,omitempty"`
	Previous7d          CampaignMetricsWindow `json:"previous_7d,omitempty"`
	Last14d             CampaignMetricsWindow `json:"last_14d,omitempty"`
	Deltas              map[string]any        `json:"deltas,omitempty"`
	DailySeries         []CampaignDailyPoint  `json:"daily_series,omitempty"`
	FactIDs             []string              `json:"fact_ids,omitempty"`
}

type CampaignDailyPoint struct {
	Date string `json:"date"`
	CampaignMetricsWindow
}

type CampaignBid struct {
	Amount         *float64 `json:"amount,omitempty"`
	RawMicro       *int64   `json:"raw_micro,omitempty"`
	UnitAssumption string   `json:"unit_assumption,omitempty"`
	Applicability  string   `json:"applicability,omitempty"`
}

type CampaignProductLinkRow struct {
	CampaignEntityID    string         `json:"campaign_entity_id"`
	SKUEntityID         string         `json:"sku_entity_id,omitempty"`
	CampaignExternalID  string         `json:"campaign_external_id"`
	SKU                 string         `json:"sku"`
	OfferID             string         `json:"offer_id,omitempty"`
	ProductTitle        string         `json:"product_title,omitempty"`
	IsActive            bool           `json:"is_active"`
	Status              string         `json:"status,omitempty"`
	Bid                 *CampaignBid   `json:"bid,omitempty"`
	EnrichmentStatus    string         `json:"enrichment_status,omitempty"`
	LinkedStockSummary  map[string]any `json:"linked_stock_summary,omitempty"`
	LinkedProductStatus map[string]any `json:"linked_product_status,omitempty"`
	Partial             bool           `json:"partial,omitempty"`
}

type FunnelRollupTable struct {
	DataStatus string         `json:"data_status"`
	Reason     string         `json:"reason,omitempty"`
	Scope      map[string]any `json:"scope,omitempty"`
	CurrentDay map[string]any `json:"current_day,omitempty"`
	Last7d     map[string]any `json:"last_7d,omitempty"`
	FactIDs    []string       `json:"fact_ids,omitempty"`
}

type CategoryRollupRow struct {
	EntityID               string   `json:"entity_id"`
	CategoryID             string   `json:"category_id"`
	Name                   string   `json:"name,omitempty"`
	Path                   *string  `json:"path"`
	ActiveSKUCount         int      `json:"active_sku_count"`
	ProductsWithStockCount int      `json:"products_with_stock_count"`
	Revenue7d              float64  `json:"revenue_7d"`
	Orders7d               int64    `json:"orders_7d"`
	Units7d                int64    `json:"units_7d"`
	DeficitSKUCount        int      `json:"deficit_sku_count"`
	NoSalesSKUCount        int      `json:"no_sales_sku_count"`
	SurplusSKUCount        int      `json:"surplus_sku_count"`
	CampaignLinkedSKUCount int      `json:"campaign_linked_sku_count"`
	FactIDs                []string `json:"fact_ids,omitempty"`
}

type FinanceRollupTable struct {
	DataStatus string `json:"data_status"`
	Reason     string `json:"reason,omitempty"`
}

type ConstraintsBlock struct {
	PriceConstraintsAvailable bool     `json:"price_constraints_available"`
	SKUPriceConstraints       []any    `json:"sku_price_constraints"`
	Limitations               []string `json:"limitations,omitempty"`
}
