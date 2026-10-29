package finance

import (
	"encoding/json"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/financeregistry"
)

type Line struct {
	Ordinal             int
	SourceComponentPath string
	SignedAmount        string
	MappingStatus       string
	CanonicalSKUID      *int64
	CanonicalOrderID    *int64
	CanonicalPostingID  *int64
	SourceOzonSKU       *int64
	SourceProductID     *int64
	SourceFragment      json.RawMessage
}

type Record struct {
	RowOrdinal              int
	ProviderTypeID          string
	ProviderAccruedCategory string
	UnitNumber              string
	TotalAmountSigned       string
	CurrencyCode            string
	SourceFragment          json.RawMessage
	Lines                   []Line
}

type DayRevision struct {
	BusinessDate       time.Time
	CompletenessStatus string
	StateHash          string
	SourcePayloadID    int64
	Records            []Record
}

type Balance struct {
	AmountSigned       string
	CurrencyCode       string
	SourcePayloadID    int64
	CompletenessStatus string
	ObservedAt         time.Time
}

type Result struct {
	StartedAt, CompletedAt time.Time
	ProviderTypes          []financeregistry.ProviderType
	DictionaryHash         string
	Days                   []DayRevision
	Balance                *Balance
	PagesReceived          int
	CompletenessStatus     string
}
