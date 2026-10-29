package promotions

import (
	"encoding/json"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection/sales"
)

type DefinitionObservation struct {
	Origin, Title, ProviderState, DiscountType, CurrencyCode string
	ProviderActionID                                         int64
	StartsAt, EndsAt                                         *time.Time
	DiscountValue                                            *sales.Decimal
	ProviderFields                                           json.RawMessage
	RuleComponents                                           []json.RawMessage
	SourcePayloadID                                          int64
}

type SKUObservation struct {
	ProviderActionID    int64
	Origin              string
	CanonicalSKUID      *int64
	SourceProductID     *int64
	SourceSKU           *int64
	SourceOfferID       string
	SKUResolutionStatus string
	ActionPrice         *sales.Decimal
	ActionStock         *int64
	ProviderFields      json.RawMessage
	SourcePayloadID     int64
}

type EligibilitySnapshot struct {
	ProviderActionID   int64
	Origin             string
	CompletenessStatus string
	Rows               []SKUObservation
	SourcePayloadID    *int64
}

type Result struct {
	StartedAt, CompletedAt                                                 time.Time
	Definitions                                                            []DefinitionObservation
	Participants                                                           []SKUObservation
	Eligibility                                                            []EligibilitySnapshot
	CompleteParticipantKeys                                                []PromotionKey
	CompletenessStatus                                                     string
	DefinitionRequests, ParticipantPages, CandidatePages                   int
	PromotionsCreated, VersionsWritten, VersionsReused                     int
	MembershipIntervalsWritten, MembershipIntervalsClosed, EligibilityRows int
}

type PromotionKey struct {
	Origin           string
	ProviderActionID int64
}
