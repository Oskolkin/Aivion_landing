package stock

import (
	"encoding/json"
	"time"
)

type FrozenSKU struct {
	CanonicalSKUID int64
	OzonSKU        int64
}

type Position struct {
	CanonicalSKUID                  *int64
	SourceSKU                       int64
	SKUResolutionStatus             string
	ProviderPoolKey                 string
	FulfillmentType                 string
	PoolType                        string
	PoolIndependenceStatus          string
	ProviderLocationKey             string
	LocationType                    string
	LocationName                    string
	ParentProviderLocationKey       string
	SellableUnits                   *int64
	ReservedUnits                   *int64
	NonSellableUnits                *int64
	ProviderValidStockCount         *int64
	ProviderPresentCount            *int64
	ProviderReservedCount           *int64
	QuantitySemanticStatus          string
	AbsenceReason                   string
	ProviderFields                  json.RawMessage
	ProviderADS, ProviderADSCluster string
	ProviderIDC, ProviderIDCCluster string
	ProviderTurnoverGrade           string
	ProviderTurnoverGradeCluster    string
	ProviderDaysWithoutSales        *int64
	ProviderDaysWithoutSalesCluster *int64
	SourcePayloadID                 *int64
	SourceReceivedAt                time.Time
}

type Result struct {
	StartedAt, CompletedAt   time.Time
	UniverseHash             string
	ExpectedSKUCount         int
	RequestedSKUCount        int
	ReceivedSKUCount         int
	MissingResponseSKUCount  int
	BatchUnavailableSKUCount int
	BatchesExpected          int
	BatchesCompleted         int
	BatchesFailed            int
	Positions                []Position
	SnapshotID               int64
	ResolvedSKUs             int
	UnresolvedSKUs           int
	CompletenessStatus       string
}

func (p Position) IsAbsence() bool { return p.AbsenceReason != "" }
