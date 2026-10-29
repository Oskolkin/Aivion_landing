package landscape

import (
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
)

// ChangeState is the unified period-comparison state (Temporal + Landscapes).
type ChangeState string

const (
	ChangeUnchanged     ChangeState = "UNCHANGED"
	ChangeIncreased     ChangeState = "INCREASED"
	ChangeDecreased     ChangeState = "DECREASED"
	ChangeFromZero      ChangeState = "FROM_ZERO"
	ChangeToZero        ChangeState = "TO_ZERO"
	ChangeNoBase        ChangeState = "NO_BASE"
	ChangeNoCurrent     ChangeState = "NO_CURRENT"
	ChangeNotComparable ChangeState = "NOT_COMPARABLE"
)

// BlockState is domain-block readiness (never invent zeros for UNAVAILABLE).
type BlockState string

const (
	BlockReady         BlockState = "READY"
	BlockReadyEmpty    BlockState = "READY_EMPTY"
	BlockPartial       BlockState = "PARTIAL"
	BlockStale         BlockState = "STALE"
	BlockUnavailable   BlockState = "UNAVAILABLE"
	BlockNotApplicable BlockState = "NOT_APPLICABLE"
	BlockUnknown       BlockState = "UNKNOWN"
)

// EntityKind for Landscape primary entities / Action Target allowlist.
type EntityKind string

const (
	EntityAccount  EntityKind = "ACCOUNT"
	EntityCategory EntityKind = "CATEGORY"
	EntitySKU      EntityKind = "SKU"
	EntityCampaign EntityKind = "CAMPAIGN"
)

type MoneyAmount struct {
	Amount       canonicaljson.Decimal `json:"amount"`
	CurrencyCode string                `json:"currency_code"`
}

type MoneyComparison struct {
	Current       *MoneyAmount           `json:"current,omitempty"`
	Previous      *MoneyAmount           `json:"previous,omitempty"`
	AbsoluteDelta *MoneyAmount           `json:"absolute_delta,omitempty"`
	PctDelta      *canonicaljson.Decimal `json:"pct_delta,omitempty"`
	ChangeState   ChangeState            `json:"change_state"`
}

type CountComparison struct {
	Current       *int64                 `json:"current,omitempty"`
	Previous      *int64                 `json:"previous,omitempty"`
	AbsoluteDelta *int64                 `json:"absolute_delta,omitempty"`
	PctDelta      *canonicaljson.Decimal `json:"pct_delta,omitempty"`
	ChangeState   ChangeState            `json:"change_state"`
}

type RatioComparison struct {
	Current       *canonicaljson.Decimal `json:"current,omitempty"`
	Previous      *canonicaljson.Decimal `json:"previous,omitempty"`
	AbsoluteDelta *canonicaljson.Decimal `json:"absolute_delta,omitempty"`
	PctDelta      *canonicaljson.Decimal `json:"pct_delta,omitempty"`
	ChangeState   ChangeState            `json:"change_state"`
}

// PercentagePointComparison compares percentage metrics in p.p. (not relative %).
type PercentagePointComparison struct {
	CurrentPct  *canonicaljson.Decimal `json:"current_pct,omitempty"`
	PreviousPct *canonicaljson.Decimal `json:"previous_pct,omitempty"`
	DeltaPP     *canonicaljson.Decimal `json:"delta_pp,omitempty"`
	ChangeState ChangeState            `json:"change_state"`
}

type SnapshotValue[T any] struct {
	Value *T         `json:"value,omitempty"`
	AsOf  *time.Time `json:"as_of,omitempty"`
	State BlockState `json:"state,omitempty"`
}

// TruncatedList is a bounded supporting-context slice with mandatory metadata.
type TruncatedList[T any] struct {
	Items         []T    `json:"items"`
	TotalKnown    int64  `json:"total_known"`
	Returned      int64  `json:"returned"`
	Truncated     bool   `json:"truncated"`
	SortBasis     string `json:"sort_basis"`
	SemanticScope string `json:"semantic_scope"`
}

// SupportingContext is bounded supporting context with explicit readiness state.
type SupportingContext[T any] struct {
	State         BlockState   `json:"state"`
	WindowRef     string       `json:"window_ref,omitempty"`
	Period        *DateRange   `json:"period,omitempty"`
	AsOf          *time.Time   `json:"as_of,omitempty"`
	Items         []T          `json:"items"`
	TotalCount    int64        `json:"total_count"`
	ReturnedCount int64        `json:"returned_count"`
	Truncated     bool         `json:"truncated"`
	SortBasis     string       `json:"sort_basis"`
	SemanticScope string       `json:"semantic_scope"`
	Limitations   []Limitation `json:"limitations,omitempty"`
}

type DateRange struct {
	From canonicaljson.Date `json:"from"`
	To   canonicaljson.Date `json:"to"`
}

// DomainWindow is the source-native current/previous window (Sales ≠ Search).
type DomainWindow struct {
	Domain                string     `json:"domain"`
	FactFamily            string     `json:"fact_family,omitempty"`
	Current               *DateRange `json:"current,omitempty"`
	Previous              *DateRange `json:"previous,omitempty"`
	AlignedToAnalysisDate bool       `json:"aligned_to_analysis_date"`
	LagDays               *int64     `json:"lag_days,omitempty"`
}

type Limitation struct {
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
}

type CapabilityFlag struct {
	Key, Scope, FinalState, ReasonCode string
}

type LandscapeContext struct {
	AnalysisDate             canonicaljson.Date `json:"analysis_date"`
	AnalysisCutoffAt         time.Time          `json:"analysis_cutoff_at"`
	SellerAccountID          int64              `json:"seller_account_id"`
	FactualUniverseID        string             `json:"factual_universe_id"`
	FactualUniverseHash      string             `json:"factual_universe_hash"`
	ContractBundleHash       string             `json:"contract_bundle_hash"`
	LandscapeSchemaVersion   string             `json:"landscape_schema_version"`
	LandscapeProfileVersion  string             `json:"landscape_profile_version"`
	MaterialitySelectionHash string             `json:"materiality_selection_hash,omitempty"`
	BusinessTimezone         string             `json:"business_timezone"`
	ContinuityMode           string             `json:"continuity_mode,omitempty"`
}

// LandscapeSet is the immutable Core or Detail artifact payload.
type LandscapeSet struct {
	Context            LandscapeContext    `json:"context"`
	DomainWindows      []DomainWindow      `json:"domain_windows"`
	GlobalCapabilities []CapabilityFlag    `json:"global_capabilities,omitempty"`
	GlobalLimitations  []Limitation        `json:"global_limitations,omitempty"`
	Account            AccountLandscape    `json:"account"`
	Categories         []CategoryLandscape `json:"categories"`
	SKUs               []SKULandscape      `json:"skus"`
	Campaigns          []CampaignLandscape `json:"campaigns"`
}

type EntityRef struct {
	Kind EntityKind `json:"kind"`
	ID   int64      `json:"id"`
}

// --- Account ---

type AccountLandscape struct {
	Profile       string                     `json:"profile"`
	Identity      AccountIdentity            `json:"identity"`
	Sales         *AccountSalesBlock         `json:"sales,omitempty"`
	Assortment    *AccountAssortmentBlock    `json:"assortment,omitempty"`
	Stock         *AccountStockBlock         `json:"stock,omitempty"`
	Price         *AccountPriceBlock         `json:"price,omitempty"`
	Advertising   *AccountAdsBlock           `json:"advertising,omitempty"`
	Funnel        *AccountFunnelBlock        `json:"funnel,omitempty"`
	Search        *AccountSearchBlock        `json:"search,omitempty"`
	Finance       *AccountFinanceBlock       `json:"finance,omitempty"`
	Promotions    *AccountPromotionsBlock    `json:"promotions,omitempty"`
	Health        *AccountHealthBlock        `json:"health,omitempty"`
	Supply        *AccountSupplyBlock        `json:"supply,omitempty"`
	Concentration *AccountConcentrationBlock `json:"concentration,omitempty"`
	Limitations   []Limitation               `json:"limitations,omitempty"`
}

type AccountIdentity struct {
	SellerAccountID      int64  `json:"seller_account_id"`
	Marketplace          string `json:"marketplace,omitempty"`
	ActiveSKUCount       *int64 `json:"active_sku_count,omitempty"`
	ArchivedSKUCount     *int64 `json:"archived_sku_count,omitempty"`
	CategoryCount        *int64 `json:"category_count,omitempty"`
	CampaignCurrentCount *int64 `json:"campaign_current_count,omitempty"`
}

type AccountSalesBlock struct {
	State             BlockState       `json:"state"`
	OrderedAmount     MoneyComparison  `json:"ordered_amount"`
	OrderedUnits      CountComparison  `json:"ordered_units"`
	AnalysisDayAmount *MoneyComparison `json:"analysis_day_amount,omitempty"`
	AnalysisDayUnits  *CountComparison `json:"analysis_day_units,omitempty"`
	Trend30DAmount    *MoneyAmount     `json:"trend_30d_amount,omitempty"`
	Trend30DUnits     *int64           `json:"trend_30d_units,omitempty"`
	Limitations       []Limitation     `json:"limitations,omitempty"`
}

type AccountAssortmentBlock struct {
	State                       BlockState             `json:"state"`
	ActiveSKUCount              *int64                 `json:"active_sku_count,omitempty"`
	ArchivedSKUCount            *int64                 `json:"archived_sku_count,omitempty"`
	SKUWithSalesCurrent7        *int64                 `json:"sku_with_sales_current7,omitempty"`
	SKUWithoutSalesCurrent7     *int64                 `json:"sku_without_sales_current7,omitempty"`
	SKUWithUnknownSalesCurrent7 *int64                 `json:"sku_with_unknown_sales_current7,omitempty"`
	CategoryCount               *int64                 `json:"category_count,omitempty"`
	UnmappedSKUCount            *int64                 `json:"unmapped_sku_count,omitempty"`
	UnmappedSalesShare          *canonicaljson.Decimal `json:"unmapped_sales_share,omitempty"`
	Limitations                 []Limitation           `json:"limitations,omitempty"`
}

type AccountStockBlock struct {
	State                       BlockState             `json:"state"`
	TotalSellableUnits          *int64                 `json:"total_sellable_units,omitempty"`
	DemandPositiveSKUCount      *int64                 `json:"demand_positive_sku_count,omitempty"`
	ZeroSellableWithDemandCount *int64                 `json:"zero_sellable_with_demand_count,omitempty"`
	NoDemandSKUCount            *int64                 `json:"no_demand_sku_count,omitempty"`
	DemandUnknownSKUCount       *int64                 `json:"demand_unknown_sku_count,omitempty"`
	CoverageP25                 *canonicaljson.Decimal `json:"coverage_p25,omitempty"`
	CoverageP50                 *canonicaljson.Decimal `json:"coverage_p50,omitempty"`
	CoverageP75                 *canonicaljson.Decimal `json:"coverage_p75,omitempty"`
	SalesExposureZeroStockPct   *canonicaljson.Decimal `json:"sales_exposure_of_zero_stock_skus_pct,omitempty"`
	Limitations                 []Limitation           `json:"limitations,omitempty"`
}

type AccountPriceBlock struct {
	State                     BlockState             `json:"state"`
	SKUWithObservedPriceCount *int64                 `json:"sku_with_observed_price_count,omitempty"`
	PriceIncreasedSKUCount    *int64                 `json:"price_increased_sku_count,omitempty"`
	PriceDecreasedSKUCount    *int64                 `json:"price_decreased_sku_count,omitempty"`
	PriceUnchangedSKUCount    *int64                 `json:"price_unchanged_sku_count,omitempty"`
	PriceChangeCoverage       *canonicaljson.Decimal `json:"price_change_coverage,omitempty"`
	Limitations               []Limitation           `json:"limitations,omitempty"`
}

type AccountAdsTrafficBlock struct {
	State          BlockState                 `json:"state"`
	CurrentPeriod  *DateRange                 `json:"current_period,omitempty"`
	PreviousPeriod *DateRange                 `json:"previous_period,omitempty"`
	PaidSpend      MoneyComparison            `json:"paid_spend"`
	BonusSpend     *MoneyComparison           `json:"bonus_spend,omitempty"`
	Impressions    CountComparison            `json:"impressions"`
	Clicks         CountComparison            `json:"clicks"`
	CTR            *PercentagePointComparison `json:"ctr,omitempty"`
	CPC            *MoneyComparison           `json:"cpc,omitempty"`
	CPM            *MoneyComparison           `json:"cpm,omitempty"`
	Limitations    []Limitation               `json:"limitations,omitempty"`
}

type AccountAdsAttributionBlock struct {
	State                 BlockState                 `json:"state"`
	CurrentPeriod         *DateRange                 `json:"current_period,omitempty"`
	PreviousPeriod        *DateRange                 `json:"previous_period,omitempty"`
	AttributedOrderAmount MoneyComparison            `json:"attributed_order_amount"`
	AttributedOrderUnits  CountComparison            `json:"attributed_order_units"`
	CalculatedDRR         *PercentagePointComparison `json:"calculated_drr,omitempty"`
	Limitations           []Limitation               `json:"limitations,omitempty"`
}

type AccountAdsBlock struct {
	State                  BlockState                  `json:"state"`
	Traffic                *AccountAdsTrafficBlock     `json:"traffic,omitempty"`
	Attribution            *AccountAdsAttributionBlock `json:"attribution,omitempty"`
	CampaignCountRunning   *int64                      `json:"campaign_count_running,omitempty"`
	CampaignCountWithSpend *int64                      `json:"campaign_count_with_spend,omitempty"`
	Limitations            []Limitation                `json:"limitations,omitempty"`
}

type FunnelSurfaceBlock struct {
	Impressions *CountComparison           `json:"impressions,omitempty"`
	CartAdds    *CountComparison           `json:"cart_adds,omitempty"`
	Conversion  *PercentagePointComparison `json:"conversion,omitempty"`
	Limitations []Limitation               `json:"limitations,omitempty"`
}

type AccountFunnelBlock struct {
	State         BlockState          `json:"state"`
	SearchSurface *FunnelSurfaceBlock `json:"search_surface,omitempty"`
	PDPSurface    *FunnelSurfaceBlock `json:"pdp_surface,omitempty"`
	Limitations   []Limitation        `json:"limitations,omitempty"`
}

type AccountSearchBlock struct {
	State                BlockState             `json:"state"`
	SKUWithSearchData    *int64                 `json:"sku_with_search_data,omitempty"`
	PositionP25          *canonicaljson.Decimal `json:"position_p25,omitempty"`
	PositionP50          *canonicaljson.Decimal `json:"position_p50,omitempty"`
	PositionP75          *canonicaljson.Decimal `json:"position_p75,omitempty"`
	SKUPositionUpCount   *int64                 `json:"sku_with_position_up_count,omitempty"`
	SKUPositionDownCount *int64                 `json:"sku_with_position_down_count,omitempty"`
	Limitations          []Limitation           `json:"limitations,omitempty"`
}

type AccountFinanceBlock struct {
	State           BlockState             `json:"state"`
	NetAccrual      MoneyComparison        `json:"net_accrual"`
	MappingCoverage *canonicaljson.Decimal `json:"mapping_coverage,omitempty"`
	Limitations     []Limitation           `json:"limitations,omitempty"`
}

type AccountPromotionsBlock struct {
	State                        BlockState             `json:"state"`
	ActiveProviderPromotionCount *int64                 `json:"active_provider_promotion_count,omitempty"`
	ActiveSellerPromotionCount   *int64                 `json:"active_seller_promotion_count,omitempty"`
	ParticipatingSKUCount        *int64                 `json:"participating_sku_count,omitempty"`
	CandidateSKUCount            *int64                 `json:"candidate_sku_count,omitempty"`
	SalesExposureParticipating   *canonicaljson.Decimal `json:"sales_exposure_of_participating_skus,omitempty"`
	Limitations                  []Limitation           `json:"limitations,omitempty"`
}

type AccountHealthBlock struct {
	State                    BlockState   `json:"state"`
	ProviderOKCount          *int64       `json:"provider_ok_count,omitempty"`
	ProviderWarningCount     *int64       `json:"provider_warning_count,omitempty"`
	ProviderCriticalCount    *int64       `json:"provider_critical_count,omitempty"`
	ProviderBadChangesCount  *int64       `json:"provider_bad_changes_count,omitempty"`
	ProviderGoodChangesCount *int64       `json:"provider_good_changes_count,omitempty"`
	PenaltyScoreExceeded     *bool        `json:"penalty_score_exceeded,omitempty"`
	Limitations              []Limitation `json:"limitations,omitempty"`
}

type AccountSupplyBlock struct {
	State                     BlockState   `json:"state"`
	OpenSupplyOrderCount      *int64       `json:"open_supply_order_count,omitempty"`
	SKUWithOpenInboundCount   *int64       `json:"sku_with_open_inbound_count,omitempty"`
	PendingAcceptanceActCount *int64       `json:"pending_acceptance_act_count,omitempty"`
	OverdueSupplyCount        *int64       `json:"overdue_supply_count,omitempty"`
	Limitations               []Limitation `json:"limitations,omitempty"`
}

type AccountConcentrationBlock struct {
	State                          BlockState             `json:"state"`
	Top5SKUOrderedAmountShare      *canonicaljson.Decimal `json:"top_5_sku_ordered_amount_share,omitempty"`
	Top3CategoryOrderedAmountShare *canonicaljson.Decimal `json:"top_3_category_ordered_amount_share,omitempty"`
	Top5CampaignSpendShare         *canonicaljson.Decimal `json:"top_5_campaign_spend_share,omitempty"`
	Limitations                    []Limitation           `json:"limitations,omitempty"`
}

// --- Category ---

type CategoryLandscape struct {
	Profile     string                   `json:"profile"`
	Identity    CategoryIdentity         `json:"identity"`
	Sales       *CategorySalesBlock      `json:"sales,omitempty"`
	Stock       *CategoryStockBlock      `json:"stock,omitempty"`
	Price       *CategoryPriceBlock      `json:"price,omitempty"`
	Advertising *CategoryAdsBlock        `json:"advertising,omitempty"`
	Funnel      *CategoryFunnelBlock     `json:"funnel,omitempty"`
	Search      *CategorySearchBlock     `json:"search,omitempty"`
	Finance     *CategoryFinanceBlock    `json:"finance,omitempty"`
	Promotions  *CategoryPromotionsBlock `json:"promotions,omitempty"`
	Supply      *CategorySupplyBlock     `json:"supply,omitempty"`
	Limitations []Limitation             `json:"limitations,omitempty"`
}

type CategoryIdentity struct {
	CategoryID       int64  `json:"category_id"`
	Name             string `json:"name,omitempty"`
	IsLeaf           bool   `json:"is_leaf"`
	Unmapped         bool   `json:"unmapped"`
	CurrentSKUCount  *int64 `json:"current_sku_count,omitempty"`
	ProductTypeCount *int64 `json:"product_type_count,omitempty"`
}

type CategorySalesBlock struct {
	State                BlockState             `json:"state"`
	OrderedAmount        MoneyComparison        `json:"ordered_amount"`
	OrderedUnits         CountComparison        `json:"ordered_units"`
	AccountOrderedShare  *canonicaljson.Decimal `json:"account_ordered_amount_share,omitempty"`
	SKUWithSalesCount    *int64                 `json:"sku_with_sales_count,omitempty"`
	SKUWithoutSalesCount *int64                 `json:"sku_without_sales_count,omitempty"`
	OrderedAmountRank    *int64                 `json:"category_ordered_amount_rank,omitempty"`
	Limitations          []Limitation           `json:"limitations,omitempty"`
}

type CategoryStockBlock struct {
	State                    BlockState             `json:"state"`
	SKUCount                 *int64                 `json:"sku_count,omitempty"`
	DemandPositiveSKUCount   *int64                 `json:"demand_positive_sku_count,omitempty"`
	ZeroStockWithDemandCount *int64                 `json:"zero_stock_with_demand_count,omitempty"`
	CoverageP25              *canonicaljson.Decimal `json:"coverage_p25,omitempty"`
	CoverageP50              *canonicaljson.Decimal `json:"coverage_p50,omitempty"`
	CoverageP75              *canonicaljson.Decimal `json:"coverage_p75,omitempty"`
	SalesExposureZeroStock   *canonicaljson.Decimal `json:"sales_exposure_zero_stock,omitempty"`
	SKUWithOpenInboundCount  *int64                 `json:"sku_with_open_inbound_count,omitempty"`
	Limitations              []Limitation           `json:"limitations,omitempty"`
}

type CategoryPriceBlock struct {
	State                  BlockState             `json:"state"`
	SKUPriceIncreasedCount *int64                 `json:"sku_price_increased_count,omitempty"`
	SKUPriceDecreasedCount *int64                 `json:"sku_price_decreased_count,omitempty"`
	SKUPriceUnchangedCount *int64                 `json:"sku_price_unchanged_count,omitempty"`
	MedianPriceChangePct   *canonicaljson.Decimal `json:"median_price_change_pct,omitempty"`
	PriceChangeCoverage    *canonicaljson.Decimal `json:"price_change_coverage,omitempty"`
	Limitations            []Limitation           `json:"limitations,omitempty"`
}

type CategoryAdsTrafficBlock struct {
	State                  BlockState             `json:"state"`
	CurrentPeriod          *DateRange             `json:"current_period,omitempty"`
	PreviousPeriod         *DateRange             `json:"previous_period,omitempty"`
	PaidSpend              MoneyComparison        `json:"paid_spend"`
	SKUWithTrafficActivity *int64                 `json:"sku_with_traffic_activity,omitempty"`
	MappingCoverage        *canonicaljson.Decimal `json:"mapping_coverage,omitempty"`
	Limitations            []Limitation           `json:"limitations,omitempty"`
}

type CategoryAdsAttributionBlock struct {
	State                      BlockState             `json:"state"`
	CurrentPeriod              *DateRange             `json:"current_period,omitempty"`
	PreviousPeriod             *DateRange             `json:"previous_period,omitempty"`
	AttributedOrderAmount      MoneyComparison        `json:"attributed_order_amount"`
	AttributedOrderUnits       CountComparison        `json:"attributed_order_units"`
	SKUWithAttributionActivity *int64                 `json:"sku_with_attribution_activity,omitempty"`
	MappingCoverage            *canonicaljson.Decimal `json:"mapping_coverage,omitempty"`
	Limitations                []Limitation           `json:"limitations,omitempty"`
}

type CategoryAdsBlock struct {
	State       BlockState                   `json:"state"`
	Traffic     *CategoryAdsTrafficBlock     `json:"traffic,omitempty"`
	Attribution *CategoryAdsAttributionBlock `json:"attribution,omitempty"`
	Limitations []Limitation                 `json:"limitations,omitempty"`
}

type CategoryFunnelBlock struct {
	State          BlockState          `json:"state"`
	CurrentPeriod  *DateRange          `json:"current_period,omitempty"`
	PreviousPeriod *DateRange          `json:"previous_period,omitempty"`
	SearchSurface  *FunnelSurfaceBlock `json:"search_surface,omitempty"`
	PDPSurface     *FunnelSurfaceBlock `json:"pdp_surface,omitempty"`
	Limitations    []Limitation        `json:"limitations,omitempty"`
}

type CategorySearchBlock struct {
	State                     BlockState             `json:"state"`
	CurrentPeriod             *DateRange             `json:"current_period,omitempty"`
	PreviousPeriod            *DateRange             `json:"previous_period,omitempty"`
	SKUWithSearchSummaryCount *int64                 `json:"sku_with_search_summary_count,omitempty"`
	PositionP25               *canonicaljson.Decimal `json:"position_p25,omitempty"`
	PositionP50               *canonicaljson.Decimal `json:"position_p50,omitempty"`
	PositionP75               *canonicaljson.Decimal `json:"position_p75,omitempty"`
	Coverage                  *canonicaljson.Decimal `json:"coverage,omitempty"`
	MappingExcludedSKUCount   *int64                 `json:"mapping_excluded_sku_count,omitempty"`
	Limitations               []Limitation           `json:"limitations,omitempty"`
}

type CategoryFinanceBlock struct {
	State                  BlockState             `json:"state"`
	DirectMappedFinanceNet MoneyComparison        `json:"direct_mapped_finance_net"`
	SKUMappingCoverage     *canonicaljson.Decimal `json:"sku_mapping_coverage,omitempty"`
	Limitations            []Limitation           `json:"limitations,omitempty"`
}

type CategoryPromotionsBlock struct {
	State                 BlockState   `json:"state"`
	ParticipatingSKUCount *int64       `json:"participating_sku_count,omitempty"`
	CandidateSKUCount     *int64       `json:"candidate_sku_count,omitempty"`
	ActivePromotionCount  *int64       `json:"active_promotion_count,omitempty"`
	Limitations           []Limitation `json:"limitations,omitempty"`
}

type CategorySupplyBlock struct {
	State                    BlockState   `json:"state"`
	SKUWithOpenInboundCount  *int64       `json:"sku_with_open_inbound_count,omitempty"`
	DeclaredOpenInboundUnits *int64       `json:"declared_open_inbound_units,omitempty"`
	OverdueSupplyCount       *int64       `json:"overdue_supply_count,omitempty"`
	Limitations              []Limitation `json:"limitations,omitempty"`
}

// --- SKU ---

type SKULandscape struct {
	Profile     string              `json:"profile"`
	Identity    SKUIdentity         `json:"identity"`
	Sales       *SKUSalesBlock      `json:"sales,omitempty"`
	Stock       *SKUStockBlock      `json:"stock,omitempty"`
	Price       *SKUPriceBlock      `json:"price,omitempty"`
	Advertising *SKUAdsBlock        `json:"advertising,omitempty"`
	Funnel      *SKUFunnelBlock     `json:"funnel,omitempty"`
	Search      *SKUSearchBlock     `json:"search,omitempty"`
	Finance     *SKUFinanceBlock    `json:"finance,omitempty"`
	Promotions  *SKUPromotionsBlock `json:"promotions,omitempty"`
	Supply      *SKUSupplyBlock     `json:"supply,omitempty"`
	// Detail-only supporting context (nil on Core).
	CampaignContext  *SupportingContext[CampaignContextItem]    `json:"campaign_context,omitempty"`
	PromotionContext *SupportingContext[PromotionContextItem]   `json:"promotion_context,omitempty"`
	QueryContext     *SupportingContext[SearchQueryContextItem] `json:"query_context,omitempty"`
	Limitations      []Limitation                               `json:"limitations,omitempty"`
}

type SKUIdentity struct {
	CanonicalSKUID int64  `json:"canonical_sku_id"`
	ProviderSKU    int64  `json:"provider_sku,omitempty"`
	OfferID        string `json:"offer_id,omitempty"`
	Name           string `json:"name,omitempty"`
	CatalogState   string `json:"catalog_state,omitempty"`
	CategoryID     *int64 `json:"category_id,omitempty"`
	CategoryName   string `json:"category_name,omitempty"`
	Unmapped       bool   `json:"unmapped"`
}

type SKUSalesBlock struct {
	State                BlockState             `json:"state"`
	OrderedAmount        MoneyComparison        `json:"ordered_amount"`
	OrderedUnits         CountComparison        `json:"ordered_units"`
	AccountOrderedShare  *canonicaljson.Decimal `json:"account_ordered_amount_share,omitempty"`
	CategoryOrderedShare *canonicaljson.Decimal `json:"category_ordered_amount_share,omitempty"`
	AccountRank          *int64                 `json:"account_rank,omitempty"`
	CategoryRank         *int64                 `json:"category_rank,omitempty"`
	Trend30DAmount       *MoneyAmount           `json:"ordered_amount_30d,omitempty"`
	Trend30DUnits        *int64                 `json:"ordered_units_30d,omitempty"`
	DaysWithOrders30D    *int64                 `json:"days_with_orders_30d,omitempty"`
	Limitations          []Limitation           `json:"limitations,omitempty"`
}

type SKUStockBlock struct {
	State                  BlockState             `json:"state"`
	SellableUnits          *int64                 `json:"sellable_units,omitempty"`
	CoverageDays7D         *canonicaljson.Decimal `json:"coverage_days_7d,omitempty"`
	CoverageDays30D        *canonicaljson.Decimal `json:"coverage_days_30d,omitempty"`
	SellableStockIsZero    *bool                  `json:"sellable_stock_is_zero,omitempty"`
	StockSnapshotAsOf      *time.Time             `json:"stock_snapshot_as_of,omitempty"`
	WarehousePositionCount *int64                 `json:"warehouse_position_count,omitempty"`
	Limitations            []Limitation           `json:"limitations,omitempty"`
}

type SKUPriceBlock struct {
	State                BlockState             `json:"state"`
	CurrentPrice         *MoneyAmount           `json:"current_price,omitempty"`
	LastObservedChange   *MoneyComparison       `json:"last_observed_price_change,omitempty"`
	LastObservedChangeAt *time.Time             `json:"last_observed_price_change_at,omitempty"`
	PriceIndex           *canonicaljson.Decimal `json:"price_index,omitempty"`
	PriceHistoryCoverage *canonicaljson.Decimal `json:"price_history_coverage,omitempty"`
	Limitations          []Limitation           `json:"limitations,omitempty"`
}

type SKUAdsTrafficBlock struct {
	State          BlockState                 `json:"state"`
	CurrentPeriod  *DateRange                 `json:"current_period,omitempty"`
	PreviousPeriod *DateRange                 `json:"previous_period,omitempty"`
	PaidSpend      MoneyComparison            `json:"paid_spend"`
	BonusSpend     *MoneyComparison           `json:"bonus_spend,omitempty"`
	Impressions    CountComparison            `json:"impressions"`
	Clicks         CountComparison            `json:"clicks"`
	CTR            *PercentagePointComparison `json:"ctr,omitempty"`
	CPC            *MoneyComparison           `json:"cpc,omitempty"`
	CPM            *MoneyComparison           `json:"cpm,omitempty"`
	Limitations    []Limitation               `json:"limitations,omitempty"`
}

type SKUAdsAttributionBlock struct {
	State                 BlockState                 `json:"state"`
	CurrentPeriod         *DateRange                 `json:"current_period,omitempty"`
	PreviousPeriod        *DateRange                 `json:"previous_period,omitempty"`
	AttributedOrderAmount MoneyComparison            `json:"attributed_order_amount"`
	AttributedOrderUnits  CountComparison            `json:"attributed_order_units"`
	CalculatedDRR         *PercentagePointComparison `json:"calculated_drr,omitempty"`
	Limitations           []Limitation               `json:"limitations,omitempty"`
}

type SKUAdsBlock struct {
	State                         BlockState              `json:"state"`
	Traffic                       *SKUAdsTrafficBlock     `json:"traffic,omitempty"`
	Attribution                   *SKUAdsAttributionBlock `json:"attribution,omitempty"`
	CampaignsWithObservedActivity *int64                  `json:"campaigns_with_observed_activity_count,omitempty"`
	Limitations                   []Limitation            `json:"limitations,omitempty"`
}

type SKUFunnelBlock struct {
	State          BlockState          `json:"state"`
	CurrentPeriod  *DateRange          `json:"current_period,omitempty"`
	PreviousPeriod *DateRange          `json:"previous_period,omitempty"`
	SearchSurface  *FunnelSurfaceBlock `json:"search_surface,omitempty"`
	PDPSurface     *FunnelSurfaceBlock `json:"pdp_surface,omitempty"`
	Limitations    []Limitation        `json:"limitations,omitempty"`
}

type SKUSearchBlock struct {
	State                           BlockState             `json:"state"`
	ProviderAveragePosition         *canonicaljson.Decimal `json:"provider_average_position,omitempty"`
	PreviousProviderAveragePosition *canonicaljson.Decimal `json:"previous_provider_average_position,omitempty"`
	UniqueSearchUsers               *int64                 `json:"unique_search_users,omitempty"`
	UniqueViewUsers                 *int64                 `json:"unique_view_users,omitempty"`
	ViewConversion                  *canonicaljson.Decimal `json:"view_conversion,omitempty"`
	Limitations                     []Limitation           `json:"limitations,omitempty"`
}

type SKUFinanceBlock struct {
	State            BlockState             `json:"state"`
	DirectFinanceNet *MoneyComparison       `json:"direct_finance_net,omitempty"`
	MappingCoverage  *canonicaljson.Decimal `json:"mapping_coverage,omitempty"`
	Limitations      []Limitation           `json:"limitations,omitempty"`
}

type SKUPromotionsBlock struct {
	State                   BlockState   `json:"state"`
	ActivePromotionCount    *int64       `json:"active_promotion_count,omitempty"`
	CandidatePromotionCount *int64       `json:"candidate_promotion_count,omitempty"`
	Limitations             []Limitation `json:"limitations,omitempty"`
}

type SKUSupplyBlock struct {
	State                    BlockState   `json:"state"`
	OpenInboundDeclaredUnits *int64       `json:"open_inbound_declared_units,omitempty"`
	OverdueSupplyCount       *int64       `json:"overdue_supply_count,omitempty"`
	OpenSupplyCount          *int64       `json:"open_supply_count,omitempty"`
	Limitations              []Limitation `json:"limitations,omitempty"`
}

type CampaignContextItem struct {
	CampaignID int64        `json:"campaign_id"`
	PaidSpend  *MoneyAmount `json:"paid_spend,omitempty"`
}

type PromotionContextItem struct {
	PromotionID int64  `json:"promotion_id"`
	Origin      string `json:"origin,omitempty"`
	Mechanism   string `json:"mechanism,omitempty"`
	Membership  string `json:"membership_state,omitempty"`
}

type SearchQueryContextItem struct {
	QueryID          int64                  `json:"query_id,omitempty"`
	QueryText        string                 `json:"query_text,omitempty"`
	ProviderPosition *canonicaljson.Decimal `json:"provider_position,omitempty"`
	SearchCount      *int64                 `json:"search_count,omitempty"`
}

// --- Campaign ---

type CampaignLandscape struct {
	Profile     string                 `json:"profile"`
	Identity    CampaignIdentity       `json:"identity"`
	Performance *CampaignPerfBlock     `json:"performance,omitempty"`
	Exposure    *CampaignExposureBlock `json:"exposure,omitempty"`
	Targeting   *CampaignTargetBlock   `json:"targeting,omitempty"`
	Stock       *CampaignStockBlock    `json:"stock,omitempty"`
	// Detail-only.
	SKUContext  *SupportingContext[SKUAdsContextItem] `json:"sku_context,omitempty"`
	Limitations []Limitation                          `json:"limitations,omitempty"`
}

type CampaignIdentity struct {
	CampaignID    int64  `json:"campaign_id"`
	Title         string `json:"title,omitempty"`
	ProviderState string `json:"provider_state,omitempty"`
	PaymentType   string `json:"payment_type,omitempty"`
	AdvObjectType string `json:"adv_object_type,omitempty"`
	TargetScope   string `json:"target_scope,omitempty"`
}

type CampaignPerfBlock struct {
	State       BlockState                `json:"state"`
	Traffic     *CampaignTrafficBlock     `json:"traffic,omitempty"`
	Attribution *CampaignAttributionBlock `json:"attribution,omitempty"`
	Limitations []Limitation              `json:"limitations,omitempty"`
}

type CampaignTrafficBlock struct {
	State          BlockState                 `json:"state"`
	CurrentPeriod  *DateRange                 `json:"current_period,omitempty"`
	PreviousPeriod *DateRange                 `json:"previous_period,omitempty"`
	PaidSpend      MoneyComparison            `json:"paid_spend"`
	BonusSpend     *MoneyComparison           `json:"bonus_spend,omitempty"`
	Impressions    CountComparison            `json:"impressions"`
	Clicks         CountComparison            `json:"clicks"`
	CTR            *PercentagePointComparison `json:"ctr,omitempty"`
	CPC            *MoneyComparison           `json:"cpc,omitempty"`
	CPM            *MoneyComparison           `json:"cpm,omitempty"`
	Limitations    []Limitation               `json:"limitations,omitempty"`
}

type CampaignAttributionBlock struct {
	State                 BlockState                 `json:"state"`
	CurrentPeriod         *DateRange                 `json:"current_period,omitempty"`
	PreviousPeriod        *DateRange                 `json:"previous_period,omitempty"`
	AttributedOrderAmount MoneyComparison            `json:"attributed_order_amount"`
	AttributedOrderUnits  CountComparison            `json:"attributed_order_units"`
	CalculatedDRR         *PercentagePointComparison `json:"calculated_drr,omitempty"`
	ProviderDRR           *PercentagePointComparison `json:"provider_drr,omitempty"`
	Limitations           []Limitation               `json:"limitations,omitempty"`
}

type CampaignExposureBlock struct {
	State                       BlockState             `json:"state"`
	PaidSpendShareOfAccountAds  *canonicaljson.Decimal `json:"campaign_paid_spend_share_of_account_ads,omitempty"`
	AttributedShareOfAccountAds *canonicaljson.Decimal `json:"campaign_attributed_amount_share_of_account_attributed_ads,omitempty"`
	Limitations                 []Limitation           `json:"limitations,omitempty"`
}

type CampaignTargetBlock struct {
	State                      BlockState   `json:"state"`
	TargetScope                string       `json:"target_scope,omitempty"`
	AssignedSKUCount           *int64       `json:"assigned_sku_count,omitempty"`
	SKUWithObservedPerformance *int64       `json:"sku_with_observed_performance_count,omitempty"`
	Limitations                []Limitation `json:"limitations,omitempty"`
}

type CampaignStockBlock struct {
	State                             BlockState             `json:"state"`
	AssignedSKUCount                  *int64                 `json:"assigned_sku_count,omitempty"`
	ContextSKUCount                   *int64                 `json:"context_sku_count,omitempty"`
	ContextTruncated                  bool                   `json:"context_truncated,omitempty"`
	ContextSKUWithKnownStockCount     *int64                 `json:"context_sku_with_known_stock_count,omitempty"`
	ContextSKUWithZeroStockCount      *int64                 `json:"context_sku_with_zero_stock_count,omitempty"`
	AssignedSKUSalesExposureZeroStock *canonicaljson.Decimal `json:"assigned_sku_sales_exposure_zero_stock,omitempty"`
	Limitations                       []Limitation           `json:"limitations,omitempty"`
}

type SKUAdsContextItem struct {
	CanonicalSKUID int64        `json:"canonical_sku_id"`
	PaidSpend      *MoneyAmount `json:"paid_spend,omitempty"`
}

// ContinuityContext is the PREPARE_CONTINUITY artifact for no-base first runs.
type ContinuityContext struct {
	Version         string             `json:"version"`
	SellerAccountID int64              `json:"seller_account_id"`
	AnalysisDate    canonicaljson.Date `json:"analysis_date"`
	Mode            string             `json:"mode"`
	BaseAvailable   bool               `json:"base_available"`
}
