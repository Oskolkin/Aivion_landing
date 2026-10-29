package factualapi

type SKUSearchSummary struct {
	ID, SellerAccountID, SourceSKU int64
	CanonicalSKUID                 *int64
	SKUResolutionStatus            string
	Period                         Period
	PeriodKind                     string
	ProviderSearchGMV              Value[Money]
	Orders                         Value[Decimal]
	UniqueSearchUsers              Value[Decimal]
	UniqueViewUsers                Value[Decimal]
	Position                       Value[Decimal]
	ViewConversion                 Value[Decimal]
	UniqueUsersAggregation         string
	Meta                           FactMeta
}

type BatchSKUSearchSummaryItem struct {
	ID     int64
	Status BatchStatus
	Fact   *SKUSearchSummary
}

type BatchSKUSearchSummaryResult struct {
	Items []BatchSKUSearchSummaryItem
}

type SearchQueryFact struct {
	ID, SellerAccountID                        int64
	Provider, Market, QueryText, NormalizedKey string
	DisplayVersionNo                           int64
	Meta                                       FactMeta
}

type BatchSKUQueryDetailsItem struct {
	SKUID   int64
	Status  BatchStatus
	Items   []SearchQueryDetail
	HasMore bool
}
type BatchSKUQueryDetailsResult struct{ Items []BatchSKUQueryDetailsItem }

type SearchQueryDetail struct {
	ID, SearchQueryID, SourceSKU    int64
	CanonicalSKUID                  *int64
	SKUResolutionStatus             string
	ProviderRank                    int
	ProviderSearchGMV               Value[Money]
	Orders                          Value[Decimal]
	RetrievalScope, RetrievalReason string
	TopNLimit, LimitBySKU           int
	SortBy, SortDir                 string
	Page, PageSize                  int
	CollectionPolicy                string
	Period                          Period
	Meta                            FactMeta
}

// SearchQuery is legacy query-primary summary shape (wrong grain). Prefer SKUSearchSummary.
type SearchQuery struct {
	ID, SummaryID, SellerAccountID             int64
	Provider, Market, QueryText, NormalizedKey string
	Period                                     Period
	PeriodKind                                 string
	ProviderSearchGMV                          Value[Money]
	Orders, UniqueUsers                        Value[Decimal]
	UniqueUsersAggregation                     string
	Meta                                       FactMeta
}

type SearchFilter struct {
	Query      string
	PeriodKind string
	CategoryID *int64
	SourceSKU  *int64
}

type SearchCategorySummary struct {
	CategoryID        int64
	Period            Period
	ProviderSearchGMV Value[Money]
	Orders            Value[Decimal]
	Meta              FactMeta
}

type CategorySearchDistribution struct {
	CategoryID                int64
	Period                    Period
	SKUWithSearchSummaryCount Value[int64]
	EligibleStableSKUCount    Value[int64]
	PositionP25               Value[Decimal]
	PositionP50               Value[Decimal]
	PositionP75               Value[Decimal]
	Coverage                  Value[Decimal]
	MappingExcludedSKUCount   Value[int64]
	FormulaPolicy             string
	Meta                      FactMeta
}

type BatchCategorySearchDistributionItem struct {
	ID           int64
	Status       BatchStatus
	Distribution *CategorySearchDistribution
	Limitation   *Limitation
}

type BatchCategorySearchDistributionResult struct {
	Items []BatchCategorySearchDistributionItem
}

type MarketSearchFact struct {
	SearchQueryID int64
	ProviderRank  int
	Period        Period
	Meta          FactMeta
}
