package advertising

import (
	"encoding/json"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection/sales"
)

type CampaignObservation struct {
	ProviderCampaignID                                               int64
	Title, ProviderState, PaymentType, AdvObjectType, CampaignFamily string
	PlacementJSON                                                    json.RawMessage
	Budget, WeeklyBudget                                             *sales.Decimal
	CurrencyCode                                                     string
	FromDate, ToDate                                                 *time.Time
	Strategy, ProductAutopilotStrategy                               string
	IsAutocreated                                                    *bool
	AutostopStatus, TargetScopeType                                  string
	ProviderFields                                                   json.RawMessage
	SourcePayloadID                                                  int64
}
type TargetObservation struct {
	ProviderCampaignID                   int64
	CanonicalSKUID                       *int64
	SourceSKU                            *int64
	SKUResolutionStatus, AssignmentState string
	Bid                                  *sales.Decimal
	BidUnit                              string
	SourcePayloadID                      int64
}
type DailyObservation struct {
	ProviderCampaignID                                                    int64
	BusinessDate                                                          time.Time
	Impressions, Clicks                                                   *int64
	PaidSpendAmount, BonusSpendAmount, AttributedOrderAmount, ProviderDRR *sales.Decimal
	AttributedOrderUnits                                                  *int64
	CurrencyCode, TrafficState, AttributionState, AttributionKind         string
	SourcePayloadID                                                       int64
}
type ProductDailyObservation struct {
	ProviderCampaignID                                                    int64
	BusinessDate                                                          time.Time
	SourcePromotedSKU                                                     int64
	CanonicalSKUID                                                        *int64
	SKUResolutionStatus                                                   string
	Impressions, Clicks, AttributedOrderUnits                             *int64
	PaidSpendAmount, BonusSpendAmount, AttributedOrderAmount, ProviderDRR *sales.Decimal
	CurrencyCode, AttributionKind                                         string
	SourcePayloadID                                                       int64
}
type ExpenseObservation struct {
	BusinessDate     time.Time
	PaidSpendAmount  *sales.Decimal
	BonusSpendAmount *sales.Decimal
	CurrencyCode     string
	SourcePayloadID  int64
}
type CPOObservation struct {
	ProviderCampaignID                                                             int64
	BusinessDate                                                                   time.Time
	SourcePromotedSKU, SourceOrderedSKU, SourceOrderID                             *int64
	PromotedCanonicalSKUID, OrderedCanonicalSKUID, CanonicalOrderID                *int64
	PromotedSKUResolutionStatus, OrderedSKUResolutionStatus, OrderResolutionStatus string
	AttributedOrderUnits                                                           *int64
	AttributedOrderAmount                                                          *sales.Decimal
	CurrencyCode, AttributionKind                                                  string
	SourcePayloadID                                                                int64
}
type ReportRequestObservation struct {
	SourceContractKey, RequestFingerprint, ProviderReportID, Status string
	StatusObservedAt                                                time.Time
	ResultSourcePayloadID                                           *int64
}
type Result struct {
	StartedAt, CompletedAt                                                                                                 time.Time
	RequestedFrom, RequestedTo                                                                                             time.Time
	CampaignPages, DailyBatches, ProductBatches, ExpenseBatches, TargetPages, CPOReports                                   int
	Campaigns                                                                                                              []CampaignObservation
	Targets                                                                                                                []TargetObservation
	Daily                                                                                                                  []DailyObservation
	ProductDaily                                                                                                           []ProductDailyObservation
	Expense                                                                                                                []ExpenseObservation
	CPO                                                                                                                    []CPOObservation
	ReportRequests                                                                                                         []ReportRequestObservation
	AuthoritativeTargetCampaignIDs                                                                                         []int64 // campaigns with successful target enumeration (even empty)
	CompletenessStatus                                                                                                     string
	CampaignsCreated, VersionsWritten, VersionsReused, TargetIntervalsWritten, DailyRevisionsWritten, DailyRevisionsReused int
	ProductRevisionsWritten, ProductRevisionsReused, ExpenseRowsWritten, CPORowsWritten, CPORowsReused                     int
	ReportRequestsWritten                                                                                                  int
}
