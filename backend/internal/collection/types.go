// Package collection contains the replacement-neutral persistence foundation
// for provider collection. It intentionally has no business fact semantics.
package collection

import "time"

type CollectionRunKind string

const (
	CollectionRunKindInitial CollectionRunKind = "INITIAL"
	CollectionRunKindDaily   CollectionRunKind = "DAILY"
	CollectionRunKindManual  CollectionRunKind = "MANUAL"
	CollectionRunKindRepair  CollectionRunKind = "REPAIR"
)

type CollectionRunStatus string

const (
	CollectionRunStatusPending                  CollectionRunStatus = "PENDING"
	CollectionRunStatusRunning                  CollectionRunStatus = "RUNNING"
	CollectionRunStatusCompleted                CollectionRunStatus = "COMPLETED"
	CollectionRunStatusCompletedWithLimitations CollectionRunStatus = "COMPLETED_WITH_LIMITATIONS"
	CollectionRunStatusFailed                   CollectionRunStatus = "FAILED"
	CollectionRunStatusCancelled                CollectionRunStatus = "CANCELLED"
)

type CollectionDomain string

const (
	DomainCatalog       CollectionDomain = "CATALOG"
	DomainSalesDemand   CollectionDomain = "SALES_DEMAND"
	DomainPostings      CollectionDomain = "POSTINGS"
	DomainReturns       CollectionDomain = "RETURNS"
	DomainStock         CollectionDomain = "STOCK"
	DomainPrice         CollectionDomain = "PRICE"
	DomainAdvertising   CollectionDomain = "ADVERTISING"
	DomainFunnel        CollectionDomain = "FUNNEL"
	DomainFinance       CollectionDomain = "FINANCE"
	DomainSearch        CollectionDomain = "SEARCH"
	DomainPromotions    CollectionDomain = "PROMOTIONS"
	DomainAccountHealth CollectionDomain = "ACCOUNT_HEALTH"
	DomainSupply        CollectionDomain = "SUPPLY"
	DomainTaxonomy      CollectionDomain = "TAXONOMY"
)

type DomainRunStatus string

const (
	DomainRunStatusPending       DomainRunStatus = "PENDING"
	DomainRunStatusRunning       DomainRunStatus = "RUNNING"
	DomainRunStatusCompleted     DomainRunStatus = "COMPLETED"
	DomainRunStatusPartial       DomainRunStatus = "PARTIAL"
	DomainRunStatusFailed        DomainRunStatus = "FAILED"
	DomainRunStatusCancelled     DomainRunStatus = "CANCELLED"
	DomainRunStatusUnavailable   DomainRunStatus = "UNAVAILABLE"
	DomainRunStatusNotApplicable DomainRunStatus = "NOT_APPLICABLE"
)

type CompletenessStatus string

const (
	CompletenessComplete      CompletenessStatus = "COMPLETE"
	CompletenessPartial       CompletenessStatus = "PARTIAL"
	CompletenessUnknown       CompletenessStatus = "UNKNOWN"
	CompletenessUnavailable   CompletenessStatus = "UNAVAILABLE"
	CompletenessNotApplicable CompletenessStatus = "NOT_APPLICABLE"
)

type Provider string

const ProviderOzon Provider = "OZON"

type CollectionRun struct {
	ID, SellerAccountID   int64
	Kind                  CollectionRunKind
	Status                CollectionRunStatus
	StartedAt, FinishedAt *time.Time
}
type DomainRun struct {
	ID, CollectionRunID, SellerAccountID int64
	Domain                               CollectionDomain
	Status                               DomainRunStatus
	CapabilityKey, SourceContractKey     string
	SourceContractVersion                string
	SourceEndpointFamily                 string
	SemanticMappingVersion               string
	SemanticMappingHash                  string
	SourceRevisionHash                   string // provider revision only; NULL/absent when provider exposes none
	StartedAt, FinishedAt                *time.Time
}
type Cursor struct {
	ID, SellerAccountID int64
	Domain              CollectionDomain
	Key                 string
	Value               []byte
	UpdatedAt           time.Time
}
type SourcePayload struct {
	ID, SellerAccountID, DomainRunID                           int64
	Provider                                                   Provider
	EndpointKey, RequestFingerprint, Bucket, ObjectKey, SHA256 string
	RedactionProfile                                           string
	ReceivedAt                                                 time.Time
	Metadata                                                   []byte
	PageSeq                                                    *int64
	HTTPStatus                                                 *int32
}
