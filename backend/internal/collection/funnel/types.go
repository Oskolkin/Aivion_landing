package funnel

import "time"

type Observation struct {
	ScopeKind                  string
	CanonicalSKUID             *int64
	SourceSKU                  *int64
	SKUResolutionStatus        string
	MetricKey                  string
	PeriodStart, PeriodEnd     time.Time
	ValueNumeric               string
	Unit                       string
	TemporalSemantics          string
	EntityAggregationSemantics string
	Surface                    string
	ProviderMetricName         string
	RegistryVersion            string
	SourceContractHash         string
	SourcePayloadID            int64
	ObservedAt                 time.Time
}

type Result struct {
	RequestedFrom, RequestedTo                                     time.Time
	Observations                                                   []Observation
	RecordsReceived, RecordsWritten, RevisionsWritten, ReusedFacts int
	Pages, UnresolvedSKUs                                          int
}
