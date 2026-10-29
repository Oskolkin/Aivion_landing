package supply

import (
	"encoding/json"
	"strings"
	"time"
)

const StateMappingVersion = "supply_state_mapping_v1"

var canonicalStates = map[string]string{
	"CREATED": "CREATED", "DATA_FILLING": "PLANNING", "READY_TO_SUPPLY": "READY",
	"ACCEPTED_AT_SUPPLY_WAREHOUSE": "ACCEPTING", "IN_TRANSIT": "IN_TRANSIT",
	"ACCEPTANCE_AT_STORAGE_WAREHOUSE": "ACCEPTING", "REPORTS_CONFIRMATION_AWAITING": "ACCEPTING",
	"COMPLETED": "COMPLETED", "CANCELLED": "CANCELLED", "REJECTED": "REJECTED",
}

func MapState(code string) (canonical, status string) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if value := canonicalStates[code]; value != "" {
		return value, "MAPPED"
	}
	return "UNKNOWN", "UNKNOWN"
}

// StateObservationChanged reports whether a new transition row should be written.
// Same normalized provider/canonical state (and mapping status) is change-compressed;
// A→B→A still produces three observations.
func StateObservationChanged(prevProvider, prevCanonical, prevStatus, nextProvider, nextCanonical, nextStatus string) bool {
	return prevProvider != nextProvider || prevCanonical != nextCanonical || prevStatus != nextStatus
}

type OrderObservation struct {
	ProviderOrderID, ProviderState, CanonicalState, StateMappingStatus string
	DropoffProviderLocationID                                          *int64
	TimeslotFrom, TimeslotTo                                           *time.Time
	ProviderFields                                                     json.RawMessage
	SourcePayloadID                                                    int64
}
type SupplyObservation struct {
	ProviderOrderID, ProviderSupplyID, ProviderState, CanonicalState, StateMappingStatus string
	StorageProviderLocationID                                                            *int64
	ProviderFields                                                                       json.RawMessage
	SourcePayloadID                                                                      int64
}
type PlannedItem struct {
	CanonicalSKUID, SourceSKU, SourceProductID *int64
	SourceOfferID, SKUResolutionStatus         string
	DeclaredUnits                              *int64
	ProviderFields                             json.RawMessage
	SourcePayloadID                            int64
}
type BundleObservation struct {
	ProviderSupplyID, ProviderBundleID, CompletenessStatus string
	Items                                                  []PlannedItem
	PageCount                                              int
	SourcePayloadID                                        *int64
}
type AcceptanceItem struct {
	CanonicalSKUID, SourceSKU                  *int64
	SourceOfferID, SKUResolutionStatus         string
	AcceptedUnits, ShortageUnits, SurplusUnits *int64
	Amount, CurrencyCode                       string
	ProviderFields                             json.RawMessage
	SourcePayloadID                            int64
}
type AcceptanceObservation struct {
	ProviderSupplyID, ProviderActID, ProviderState string
	TotalAmount, CurrencyCode                      string
	CompletenessStatus                             string
	Items                                          []AcceptanceItem
	PageCount                                      int
	SourcePayloadID                                int64
}
type Result struct {
	StartedAt, CompletedAt                                          time.Time
	Orders                                                          []OrderObservation
	Supplies                                                        []SupplyObservation
	Bundles                                                         []BundleObservation
	Acceptances                                                     []AcceptanceObservation
	CompletenessStatus                                              string
	AcceptanceSummaryStatus, AcceptanceProductsStatus               string
	ListPages, DetailRequests, BundlePages, AcceptancePages         int
	OrdersCreated, SuppliesCreated, VersionsWritten, BundlesWritten int
	AcceptancesWritten, StateObservationsWritten                    int
}
