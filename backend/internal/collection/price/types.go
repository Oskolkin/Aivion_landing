package price

import (
	"encoding/json"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection/sales"
)

type FrozenSKU struct {
	CanonicalSKUID    int64
	OzonSKU           int64
	ProviderProductID int64
	OfferID           string
}

type Observation struct {
	CanonicalSKUID, SourceSKU, ProviderProductID *int64
	SKUResolutionStatus                          string
	CurrencyCode                                 string
	ProviderCurrentPrice, SellerPromoPrice       *sales.Decimal
	CustomerPrice, OldReferencePrice             *sales.Decimal
	MinimumAllowedPrice, RetailPrice             *sales.Decimal
	VATRate                                      *sales.Decimal
	AutoActionEnabled                            *bool
	CurrentPriceState, PriceIndexColor           string
	PriceIndexJSON, PromotionContextJSON         json.RawMessage
	ProviderFields                               json.RawMessage
	SemanticMappingStatus, ReconciliationStatus  string
	SourcePayloadID, PremiumSourcePayloadID      *int64
}

type Result struct {
	StartedAt, CompletedAt                                time.Time
	UniverseHash                                          string
	ExpectedSKUCount, RequestedSKUCount, ReceivedSKUCount int
	ResolvedSKUCount, UnresolvedSKUCount                  int
	BatchesExpected, BatchesCompleted, BatchesFailed      int
	CompletenessStatus                                    string
	Observations                                          []Observation
	SnapshotID                                            int64
}
