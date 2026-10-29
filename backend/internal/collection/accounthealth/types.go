package accounthealth

import (
	"encoding/json"
	"time"
)

type MetricObservation struct {
	ProviderRatingKey, CanonicalMetricKey, MappingStatus string
	GroupName, DisplayName, Unit                         string
	ValueType, RatingDirection, CanonicalFamily          string
	KnownCalculationWindow                               string
	HistorySupported                                     *bool
	Value, ProviderPreviousValue, ProviderStatus         string
	ProviderChangeDirection, ProviderChangeMeaning       string
	PenaltyAmount, PenaltyCurrency                       string
	ProviderFields                                       json.RawMessage
	SourcePayloadID                                      int64
}

type PeriodFact struct {
	ProviderRatingKey, Value, ProviderStatus string
	PeriodFrom, PeriodTo                     time.Time
	ProviderFields                           json.RawMessage
	SourcePayloadID                          int64
}

type ThresholdObservation struct {
	ProviderRatingKey, ProviderStatus, From, To string
	ProviderFields                              json.RawMessage
	SourcePayloadID                             int64
}

type PenaltyEvent struct {
	ProviderEventKey, ProviderRatingKey, Amount, Currency, Description string
	OccurredAt                                                         *time.Time
	ProviderFields                                                     json.RawMessage
	SourcePayloadID                                                    int64
}

type FBSIndexFact struct {
	Value, ProviderStatus string
	PeriodFrom, PeriodTo  *time.Time
	ProviderFields        json.RawMessage
	SourcePayloadID       int64
}

type FBSErrorEvent struct {
	ProviderEventKey, SourcePostingNumber, ResolutionStatus string
	CanonicalPostingID                                      *int64
	ErrorCode, ErrorMessage                                 string
	OccurredAt                                              *time.Time
	ProviderFields                                          json.RawMessage
	SourcePayloadID                                         int64
}

type Result struct {
	StartedAt, CompletedAt                          time.Time
	Metrics                                         []MetricObservation
	History                                         []PeriodFact
	Thresholds                                      []ThresholdObservation
	Penalties                                       []PenaltyEvent
	FBSIndex                                        *FBSIndexFact
	FBSErrors                                       []FBSErrorEvent
	SummaryPayloadID, HistoryPayloadID              int64
	FBSIndexPayloadID, FBSErrorsLastPayloadID       int64
	SummaryRequests, HistoryRequests, IndexRequests int
	ErrorPages                                      int
	CompletenessStatus                              string
	MetricsCreated, MetricVersionsWritten           int
	SnapshotsWritten, HistoryRevisionsWritten       int
	ThresholdVersionsWritten, PenaltiesWritten      int
	FBSIndexWritten, FBSErrorEventsWritten          int
}
