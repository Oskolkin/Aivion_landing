package factualapi

import "time"

type Account struct {
	SellerAccountID int64
	Meta            FactMeta
}
type CategoryRelation struct {
	CategoryID                                                           *int64
	MappingStatus, CategoryResolutionStatus, ProductTypeResolutionStatus string
	ObservedFrom                                                         time.Time
	ObservedTo                                                           *time.Time
	Limitation                                                           *Limitation
}
type SKU struct {
	ID, OzonSKU                                 int64
	ProviderProductID                           *int64
	OfferID, Name, ProviderStatus, CatalogState string
	ObservedAt                                  time.Time
	Category                                    CategoryRelation
	Meta                                        FactMeta
}
type Category struct {
	ID, SellerCategoryID, ProviderCategoryID int64
	Name, State                              string
	ParentProviderCategoryID                 *int64
	IsLeaf                                   bool
	ObservedAt                               time.Time
	Meta                                     FactMeta
}
type BatchStatus string

const (
	BatchFound      BatchStatus = "FOUND"
	BatchFoundEmpty BatchStatus = "FOUND_EMPTY"
	BatchNotFound   BatchStatus = "NOT_FOUND"
	BatchLimited    BatchStatus = "LIMITED"
)

type BatchSKUItem struct {
	ID         int64
	Status     BatchStatus
	SKU        *SKU
	Limitation *Limitation
}
type BatchSKUResult struct{ Items []BatchSKUItem }

type BatchCategoryItem struct {
	ID         int64
	Status     BatchStatus
	Category   *Category
	Limitation *Limitation
}
type BatchCategoryResult struct{ Items []BatchCategoryItem }

type Order struct {
	ID, OzonOrderID int64
	OrderNumber     string
	FirstObservedAt time.Time
	Meta            FactMeta
}
type PostingProduct struct {
	CanonicalSKUID                  *int64
	SourceSKU                       int64
	ResolutionStatus, OfferID, Name string
	Quantity                        int64
	UnitPrice                       Money
}
type SplitRelation struct {
	ParentPostingID                       *int64
	ParentPostingNumber, ResolutionStatus string
}
type Posting struct {
	ID                                                                                         int64
	PostingNumber, FulfillmentScheme, OrderResolutionStatus, ProviderStatus, ProviderSubstatus string
	CanonicalOrderID                                                                           *int64
	VersionNo                                                                                  int64
	ObservedAt, SourceReceivedAt                                                               time.Time
	Products                                                                                   []PostingProduct
	Splits                                                                                     []SplitRelation
	Meta                                                                                       FactMeta
}
type LifecycleEvent struct {
	ID                                                                      int64
	EventType, TimeQuality                                                  string
	CanonicalOrderID, CanonicalPostingID, CanonicalSKUID, CanonicalReturnID *int64
	EffectiveAt                                                             *time.Time
	ProviderDate, BusinessDate                                              *time.Time
	WindowFrom, WindowTo                                                    *time.Time
	ObservedAt                                                              time.Time
	Meta                                                                    FactMeta
}
type Return struct {
	ID, ProviderReturnID, VersionNo                                            int64
	CanonicalOrderID, CanonicalPostingID, CanonicalSKUID                       *int64
	ProviderType, ProviderSchema, Reason, VisualStatus, VisualStatusSystemName string
	Quantity                                                                   int64
	UnitPrice                                                                  Money
	ProviderReturnDate, VisualChangeAt                                         *time.Time
	ObservedAt, SourceReceivedAt                                               time.Time
	Meta                                                                       FactMeta
}

type SKUFilter struct {
	CatalogState string
	CategoryIDs  []int64
}
type PostingFilter struct {
	OrderID, SKUID                    *int64
	FulfillmentScheme, ProviderStatus string
}
type LifecycleEventFilter struct {
	EventTypes, TimeQualities []string
	OrderID, PostingID, SKUID *int64
	BusinessPeriod            *Period
}
type ReturnFilter struct{ PostingID, OrderID, SKUID *int64 }
type PageRequest struct {
	PageSize  int
	PageToken string
}
type Page[T any] struct {
	Items         []T
	NextPageToken string
}
