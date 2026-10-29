package search

import (
	"encoding/json"
	"time"
)

type FrozenSKU struct {
	CanonicalSKUID int64
	OzonSKU        int64
}

type QueryIdentity struct {
	Provider, Market, QueryText, NormalizedQueryKey string
}

// SKUSummary is seller search summary grain: SELLER × SKU × PERIOD × REVISION.
type SKUSummary struct {
	CanonicalSKUID                     *int64
	SourceSKU                          int64
	SKUResolutionStatus                string
	PeriodStart, PeriodEnd             time.Time
	PeriodKind                         string
	ProviderSearchGMV, Orders          string
	UniqueSearchUsers, UniqueViewUsers string
	Position, ViewConversion           string
	UniqueUsersAggregation             string
	SourcePayloadID                    int64
	ObservedAt                         time.Time
	SourceFragment                     json.RawMessage
}

// Detail is SKU × Query × PERIOD × retrieval context × revision.
type Detail struct {
	Identity                  QueryIdentity
	CanonicalSKUID            *int64
	SourceSKU                 int64
	SKUResolutionStatus       string
	ProviderRank              int
	ProviderSearchGMV, Orders string
	RetrievalScope            string
	RetrievalReason           string
	TopNLimit                 int
	SortBy, SortDir           string
	LimitBySKU                int
	Page, PageSize            int
	PeriodStart, PeriodEnd    time.Time
	SourcePayloadID           int64
	ObservedAt                time.Time
	SourceFragment            json.RawMessage
}

type MarketFact struct {
	Identity               QueryIdentity
	ProviderRank           int
	PeriodStart, PeriodEnd time.Time
	SourcePayloadID        int64
	ObservedAt             time.Time
}

type Result struct {
	StartedAt, CompletedAt                  time.Time
	SKUSummaries                            []SKUSummary
	Details                                 []Detail
	MarketFacts                             []MarketFact
	PagesReceived                           int
	SellerSummaryStatus, QueryDetailsStatus string
	MarketSearchStatus, CompletenessStatus  string
}
