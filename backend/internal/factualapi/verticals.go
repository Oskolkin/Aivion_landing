package factualapi

import "time"

type StockPosition struct {
	LineID, InventoryPoolID                               int64
	InventoryLocationID                                   *int64
	FulfillmentType, PoolIndependenceStatus, LocationName string
	SellableUnits, ReservedUnits, NonSellableUnits        Value[int64]
	ProviderValidStockCount                               Value[int64]
	ProviderADS, ProviderIDC                              Value[Decimal]
	ProviderTurnoverGrade                                 string
	AbsenceReason                                         string
}
type SKUStock struct {
	CanonicalSKUID     int64
	SnapshotID         int64
	SnapshotAt         time.Time
	TimeQuality        string
	SellableUnits      Value[int64]
	KnownSubtotalUnits int64
	Positions          []StockPosition
	Meta               FactMeta
}
type StockSnapshot struct {
	ID                                                                                                  int64
	SnapshotAt, CollectionStartedAt, CollectionCompletedAt                                              time.Time
	TimeQuality, UniverseHash, CompletenessStatus                                                       string
	ExpectedSKUCount, ReceivedSKUCount, PositionCount, BatchesExpected, BatchesCompleted, BatchesFailed int64
	Meta                                                                                                FactMeta
}
type StockCoverage struct {
	CanonicalSKUID                                   int64
	Stock                                            SKUStock
	CoverageDays7D, CoverageDays30D                  Value[Decimal]
	Demand7DFactRef, Demand30DFactRef, FormulaPolicy string
	Meta                                             FactMeta
}
type StockHistory struct{ Items []SKUStock }
type BatchSKUStockItem struct {
	ID         int64
	Status     BatchStatus
	Stock      *SKUStock
	Limitation *Limitation
}
type BatchSKUStockResult struct{ Items []BatchSKUStockItem }
type BatchStockCoverageItem struct {
	ID         int64
	Status     BatchStatus
	Coverage   *StockCoverage
	Limitation *Limitation
}
type BatchStockCoverageResult struct{ Items []BatchStockCoverageItem }
type StockFilter struct{ CategoryIDs []int64 }

type PriceIndex struct {
	Color   string
	Details map[string]string
}
type SKUPrice struct {
	CanonicalSKUID                                                                                             int64
	SnapshotID                                                                                                 int64
	ObservedAt, SnapshotAt                                                                                     time.Time
	CurrencyCode                                                                                               string
	ProviderCurrentPrice, SellerPromoPrice, CustomerPrice, OldReferencePrice, MinimumAllowedPrice, RetailPrice Value[Money]
	VATRate                                                                                                    Value[Decimal]
	AutoActionEnabled                                                                                          Value[bool]
	Index                                                                                                      PriceIndex
	PromotionContext                                                                                           map[string]string
	SemanticMappingStatus, ReconciliationStatus                                                                string
	Meta                                                                                                       FactMeta
}
type BatchSKUPriceItem struct {
	ID         int64
	Status     BatchStatus
	Price      *SKUPrice
	Limitation *Limitation
}
type BatchSKUPriceResult struct{ Items []BatchSKUPriceItem }
type PriceFilter struct {
	CategoryIDs  []int64
	CurrencyCode string
}

type Campaign struct {
	ID, ProviderCampaignID, VersionNo                                                 int64
	Title, ProviderState, PaymentType, AdvObjectType, CampaignFamily, TargetScopeType string
	Budget, WeeklyBudget                                                              Value[Money]
	ObservedAt                                                                        time.Time
	Meta                                                                              FactMeta
}
type CampaignTarget struct {
	CanonicalSKUID                    *int64
	SourceSKU                         *int64
	ResolutionStatus, AssignmentState string
	Bid                               Value[Money]
	ObservedFrom                      time.Time
	ObservedTo                        *time.Time
}
type AdvertisingDailyFact struct {
	CampaignID                                      int64
	BusinessDate                                    time.Time
	RevisionNo                                      int64
	Impressions, Clicks                             Value[int64]
	PaidSpend, BonusSpend, AttributedOrderAmount    Value[Money]
	AttributedOrderUnits                            Value[int64]
	AttributionKind, TrafficState, AttributionState string
	ProviderDRR                                     Value[Decimal]
	Meta                                            FactMeta
}
type CampaignDRR struct {
	CampaignID                       int64
	PaidSpend, AttributedOrderAmount Value[Money]
	CalculatedDRRPct, ProviderDRRPct Value[Decimal]
	FormulaPolicy                    string
	Meta                             FactMeta
}
type AdvertisingAccountSummary struct {
	Period               Period
	TotalPaidSpend       Value[Money]
	AccountAttributedDRR Value[Decimal]
	Meta                 FactMeta
}
type AdvertisingSpendShare struct {
	Period                                          Period
	PaidSpend, SalesOrderedAmount                   Value[Money]
	SpendSharePct                                   Value[Decimal]
	AdvertisingFactRef, SalesFactRef, FormulaPolicy string
	Meta                                            FactMeta
}
type SKUAdvertising struct {
	CanonicalSKUID                   int64
	Period                           Period
	Impressions, Clicks              Value[int64]
	PaidSpend, AttributedOrderAmount Value[Money]
	AttributionKind                  string
	ByAttributionKind                []SKUAdvertisingKindSlice
	Meta                             FactMeta
}
type SKUAdvertisingKindSlice struct {
	AttributionKind                  string
	Impressions, Clicks              Value[int64]
	PaidSpend, AttributedOrderAmount Value[Money]
}
type BatchSKUAdvertisingItem struct {
	ID          int64
	Status      BatchStatus
	Advertising *SKUAdvertising
	Limitation  *Limitation
}
type BatchSKUAdvertisingResult struct{ Items []BatchSKUAdvertisingItem }
type SKUCampaignPerformance struct {
	CampaignID                       int64
	Period                           Period
	PaidSpend, AttributedOrderAmount Value[Money]
	Meta                             FactMeta
}
type BatchSKUCampaignPerformanceItem struct {
	SKUID   int64
	Status  BatchStatus
	Items   []SKUCampaignPerformance
	HasMore bool
}
type BatchSKUCampaignPerformanceResult struct {
	Items []BatchSKUCampaignPerformanceItem
}
type SKUPriceLastObservedChange struct {
	CanonicalSKUID              int64
	CurrentPrice, PreviousPrice Value[Money]
	ObservedAt                  time.Time
	Meta                        FactMeta
}
type BatchSKUPriceLastObservedChangeItem struct {
	ID     int64
	Status BatchStatus
	Change *SKUPriceLastObservedChange
}
type BatchSKUPriceLastObservedChangeResult struct {
	Items []BatchSKUPriceLastObservedChangeItem
}
type CampaignPeriodPerformance struct {
	CampaignID                                         int64
	Period                                             Period
	Impressions, Clicks                                Value[int64]
	PaidSpend, BonusSpend, AttributedOrderAmount       Value[Money]
	AttributedOrderUnits                               Value[int64]
	CTRPct, CalculatedDRRPct, CPC, CPM, ProviderDRRPct Value[Decimal]
	FormulaPolicy                                      string
	Meta                                               FactMeta
}
type BatchCampaignPeriodPerformanceItem struct {
	ID          int64
	Status      BatchStatus
	Performance *CampaignPeriodPerformance
	Limitation  *Limitation
}
type BatchCampaignPeriodPerformanceResult struct {
	Items []BatchCampaignPeriodPerformanceItem
}

// CampaignTrafficPerformance is the V4 traffic-only campaign period fact.
type CampaignTrafficPerformance struct {
	CampaignID    int64
	Period        Period
	Impressions   Value[int64]
	Clicks        Value[int64]
	PaidSpend     Value[Money]
	BonusSpend    Value[Money]
	CTRPct        Value[Decimal]
	CPC           Value[Money]
	CPM           Value[Money]
	FormulaPolicy string
	Meta          FactMeta
}

type BatchCampaignTrafficItem struct {
	ID         int64
	Status     BatchStatus
	Traffic    *CampaignTrafficPerformance
	Limitation *Limitation
}

type BatchCampaignTrafficResult struct {
	Items []BatchCampaignTrafficItem
}

// CampaignAttributionPerformance is the V4 attribution-only campaign period fact.
type CampaignAttributionPerformance struct {
	CampaignID            int64
	Period                Period
	AttributedOrderAmount Value[Money]
	AttributedOrderUnits  Value[int64]
	ProviderDRRPct        Value[Decimal]
	Meta                  FactMeta
}

type BatchCampaignAttributionItem struct {
	ID          int64
	Status      BatchStatus
	Attribution *CampaignAttributionPerformance
	Limitation  *Limitation
}

type BatchCampaignAttributionResult struct {
	Items []BatchCampaignAttributionItem
}

type SKUAdvertisingTraffic struct {
	CanonicalSKUID int64
	Period         Period
	Impressions    Value[int64]
	Clicks         Value[int64]
	PaidSpend      Value[Money]
	BonusSpend     Value[Money]
	Meta           FactMeta
}

type BatchSKUAdvertisingTrafficItem struct {
	ID         int64
	Status     BatchStatus
	Traffic    *SKUAdvertisingTraffic
	Limitation *Limitation
}

type BatchSKUAdvertisingTrafficResult struct {
	Items []BatchSKUAdvertisingTrafficItem
}

type SKUAdvertisingAttribution struct {
	CanonicalSKUID        int64
	Period                Period
	AttributedOrderAmount Value[Money]
	AttributedOrderUnits  Value[int64]
	Meta                  FactMeta
}

type BatchSKUAdvertisingAttributionItem struct {
	ID          int64
	Status      BatchStatus
	Attribution *SKUAdvertisingAttribution
	Limitation  *Limitation
}

type BatchSKUAdvertisingAttributionResult struct {
	Items []BatchSKUAdvertisingAttributionItem
}

type CampaignTargetingSummary struct {
	CampaignID                      int64
	TargetScope                     string
	ExplicitAssignedSKUCount        Value[int64]
	SKUWithObservedPerformanceCount Value[int64]
	Meta                            FactMeta
}

type BatchCampaignTargetingSummaryItem struct {
	ID         int64
	Status     BatchStatus
	Summary    *CampaignTargetingSummary
	Limitation *Limitation
}

type BatchCampaignTargetingSummaryResult struct {
	Items []BatchCampaignTargetingSummaryItem
}
type CampaignFilter struct{ Families, ProviderStates []string }
type CPOAttribution struct {
	CampaignID                                                      int64
	BusinessDate                                                    time.Time
	PromotedCanonicalSKUID, OrderedCanonicalSKUID, CanonicalOrderID *int64
	SourcePromotedSKU, SourceOrderedSKU, SourceOrderID              *int64
	AttributionKind                                                 string
	AttributedAmount                                                Value[Money]
	Meta                                                            FactMeta
}
type CPOFilter struct {
	CampaignID *int64
	Period     *Period
}
type BatchCampaignItem struct {
	ID         int64
	Status     BatchStatus
	Campaign   *Campaign
	Limitation *Limitation
}
type BatchCampaignResult struct{ Items []BatchCampaignItem }
type BatchCampaignTargetSKUItem struct {
	CampaignID int64
	Status     BatchStatus
	Items      []CampaignTarget
	TotalCount int
	HasMore    bool
	Limitation *Limitation
}
type BatchCampaignTargetSKUResult struct{ Items []BatchCampaignTargetSKUItem }

// SignedMoney uses an exact decimal string because the shared sales Decimal is
// intentionally non-negative and therefore unsuitable for ledger values.
type SignedMoney struct {
	Amount       string
	CurrencyCode string
}
type FinanceAccrual struct {
	ID, DayRevisionID       int64
	BusinessDate            time.Time
	RowOrdinal              int
	ProviderTypeID          string
	ProviderAccruedCategory string
	UnitNumber              string
	Total                   SignedMoney
	MappingStatus           string
	Meta                    FactMeta
}
type FinanceDailyNet struct {
	BusinessDate       time.Time
	Net                Value[SignedMoney]
	CompletenessStatus string
	RevisionNo         int64
	Meta               FactMeta
}
type FinanceAccountSummary struct {
	Period                    Period
	AccountNet                Value[SignedMoney]
	DaysComplete, DaysPartial int64
	FormulaPolicy             string
	Meta                      FactMeta
}
type FinanceComponentAmount struct {
	ProviderTypeID string
	Category       string
	Amount         Value[SignedMoney]
	MappingStatus  string
}
type FinanceAccountBreakdown struct {
	Period          Period
	Components      []FinanceComponentAmount
	MappedAmount    Value[SignedMoney]
	UnmappedAmount  Value[SignedMoney]
	MappingCoverage Value[Decimal]
	FormulaPolicy   string
	Meta            FactMeta
}
type FinanceBalance struct {
	ObservedAt time.Time
	Balance    Value[SignedMoney]
	Meta       FactMeta
}
type FinanceAccrualFilter struct {
	Period          *Period
	ProviderTypeIDs []string
}

type Promotion struct {
	ID, ProviderActionID, VersionNo                          int64
	Origin, Title, ProviderState, DiscountType, CurrencyCode string
	StartsAt, EndsAt                                         *time.Time
	DiscountValue                                            Value[Decimal]
	ObservedAt                                               time.Time
	Meta                                                     FactMeta
}
type PromotionParticipant struct {
	CanonicalSKUID, SourceProductID, SourceSKU   *int64
	SourceOfferID, ResolutionStatus, TimeQuality string
	ActionPrice                                  Value[Money]
	ActionStock                                  Value[int64]
	KnownFrom                                    time.Time
	KnownTo                                      *time.Time
}
type PromotionCandidate struct {
	CanonicalSKUID, SourceProductID, SourceSKU *int64
	SourceOfferID, ResolutionStatus            string
	ActionPrice                                Value[Money]
	ActionStock                                Value[int64]
	ObservedAt                                 time.Time
	CompletenessStatus                         string
}
type SKUPromotion struct {
	Promotion
	MembershipID                                 int64
	CanonicalSKUID, SourceProductID, SourceSKU   *int64
	SourceOfferID, ResolutionStatus, TimeQuality string
	ActionPrice                                  Value[Money]
	ActionStock                                  Value[int64]
	KnownFrom                                    time.Time
	KnownTo                                      *time.Time
}
type BatchSKUPromotionsItem struct {
	SKUID   int64
	Status  BatchStatus
	Items   []SKUPromotion
	HasMore bool
}
type BatchSKUPromotionsResult struct{ Items []BatchSKUPromotionsItem }
type SKUPromotionSummary struct {
	CanonicalSKUID          int64
	ActivePromotionCount    Value[int64]
	CandidatePromotionCount Value[int64]
	Meta                    FactMeta
}
type BatchSKUPromotionSummaryItem struct {
	ID         int64
	Status     BatchStatus
	Summary    *SKUPromotionSummary
	Limitation *Limitation
}
type BatchSKUPromotionSummaryResult struct {
	Items []BatchSKUPromotionSummaryItem
}
type PromotionAccountSummary struct {
	ActiveProviderPromotionCount Value[int64]
	ActiveSellerPromotionCount   Value[int64]
	ParticipatingSKUCount        Value[int64]
	CandidateSKUCount            Value[int64]
	SalesExposureParticipating   Value[Decimal]
	Meta                         FactMeta
}

type CategoryPromotionSummary struct {
	CategoryID            int64
	ActivePromotionCount  Value[int64]
	ParticipatingSKUCount Value[int64]
	CandidateSKUCount     Value[int64]
	Meta                  FactMeta
}

type BatchCategoryPromotionSummaryItem struct {
	ID         int64
	Status     BatchStatus
	Summary    *CategoryPromotionSummary
	Limitation *Limitation
}

type BatchCategoryPromotionSummaryResult struct {
	Items []BatchCategoryPromotionSummaryItem
}
type PromotionMembershipInterval struct {
	ID, PromotionID                              int64
	CanonicalSKUID, SourceProductID, SourceSKU   *int64
	SourceOfferID, ResolutionStatus, TimeQuality string
	ActionPrice                                  Value[Money]
	ActionStock                                  Value[int64]
	KnownFrom                                    time.Time
	KnownTo                                      *time.Time
	Meta                                         FactMeta
}
type PromotionEligibility struct {
	PromotionID, SnapshotID int64
	ObservedAt              time.Time
	CompletenessStatus      string
	Candidates              []PromotionCandidate
	NextPageToken           string
	Meta                    FactMeta
}
type PromotionFilter struct {
	Origins, ProviderStates []string
}
type PromotionMembershipFilter struct {
	CanonicalSKUID *int64
}

type AccountRating struct {
	ID, VersionNo                                        int64
	ProviderRatingKey, CanonicalMetricKey, MappingStatus string
	ProviderGroupName, ProviderDisplayName, ValueUnit    string
	ValueType, RatingDirection, CanonicalFamily          string
	KnownCalculationWindow                               string
	HistorySupported                                     *bool
	ProviderStatus                                       string
	ProviderChangeDirection, ProviderChangeMeaning       string
	CurrentValue, ProviderPreviousValue                  Value[Decimal]
	ObservedAt                                           time.Time
	Meta                                                 FactMeta
}
type AccountHealthSummary struct {
	SnapshotID         int64
	ObservedAt         time.Time
	TimeQuality        string
	CompletenessStatus string
	Ratings            []AccountRating
	PenaltyEvents      []AccountPenaltyEvent
	Meta               FactMeta
}
type RatingPeriodFact struct {
	ID, RatingID, RevisionNo int64
	Period                   Period
	Value                    Value[Decimal]
	ProviderStatus           string
	ObservedAt               time.Time
	Meta                     FactMeta
}
type RatingThreshold struct {
	ID, RatingID, VersionNo    int64
	ProviderStatus             string
	ThresholdFrom, ThresholdTo Value[Decimal]
	ObservedAt                 time.Time
	Meta                       FactMeta
}
type AccountPenaltyEvent struct {
	ID, RevisionNo   int64
	ProviderEventKey string
	RatingID         *int64
	OccurredAt       *time.Time
	Amount           Value[Money]
	Description      string
	ObservedAt       time.Time
	Meta             FactMeta
}
type FBSErrorIndex struct {
	ID             int64
	ProviderPeriod *Period
	Value          Value[Decimal]
	ProviderStatus string
	ObservedAt     time.Time
	Meta           FactMeta
}
type FBSRatingPostingError struct {
	ID, RevisionNo          int64
	ProviderEventKey        string
	CanonicalPostingID      *int64
	SourcePostingNumber     string
	PostingResolutionStatus string
	ErrorCode, ErrorMessage string
	OccurredAt              *time.Time
	ObservedAt              time.Time
	Meta                    FactMeta
}

// Supply facts describe FBO inbound plans and observations. They are not stock
// and intentionally expose no future_stock or expected_stock_at field.
type SupplyOrder struct {
	ID, VersionNo                                  int64
	ProviderOrderID, ProviderState, CanonicalState string
	StateMappingStatus                             string
	DropoffLocationID                              *int64
	TimeslotFrom, TimeslotTo                       *time.Time
	ObservedAt                                     time.Time
	Meta                                           FactMeta
}
type InboundSupply struct {
	ID, SupplyOrderID, VersionNo                    int64
	ProviderSupplyID, ProviderState, CanonicalState string
	StateMappingStatus                              string
	StorageLocationID                               *int64
	ObservedAt                                      time.Time
	Meta                                            FactMeta
}
type SupplyPlannedItem struct {
	ID, SupplyID                               int64
	ProviderBundleID                           string
	BundleVersionNo                            int64
	CanonicalSKUID, SourceSKU, SourceProductID *int64
	SourceOfferID, ResolutionStatus            string
	CanonicalState                             string
	DeclaredUnits                              Value[int64]
	PlanCompleteness                           string
	ObservedAt                                 time.Time
}
type SupplyStateObservation struct {
	ID                                      int64
	SupplyOrderID, SupplyID                 *int64
	ProviderState, CanonicalState           string
	StateMappingStatus, StateMappingVersion string
	ObservedAt                              time.Time
	Meta                                    FactMeta
}
type SupplyAcceptance struct {
	ID, SupplyID                     int64
	ProviderActID, ProviderState     string
	TotalAmount                      Value[Money]
	CurrencyCode, CompletenessStatus string
	ObservedAt                       time.Time
	Meta                             FactMeta
}
type SupplyAcceptanceItem struct {
	ID, AcceptanceActID                        int64
	CanonicalSKUID, SourceSKU                  *int64
	SourceOfferID, ResolutionStatus            string
	AcceptedUnits, ShortageUnits, SurplusUnits Value[int64]
	Amount                                     Value[Money]
	CurrencyCode                               string
}
type SKUInbound struct {
	CanonicalSKUID       int64
	DeclaredInboundUnits Value[int64]
	Supplies             []SupplyPlannedItem
	Meta                 FactMeta
}
type BatchSKUInboundItem struct {
	ID         int64
	Status     BatchStatus
	Inbound    *SKUInbound
	Limitation *Limitation
}
type BatchSKUInboundResult struct{ Items []BatchSKUInboundItem }
type SupplyFilter struct{ CanonicalStates []string }
type SupplyAccountSummary struct {
	OpenSupplyOrderCount      Value[int64]
	SKUWithOpenInboundCount   Value[int64]
	PendingAcceptanceActCount Value[int64]
	OverdueSupplyCount        Value[int64]
	Meta                      FactMeta
}
