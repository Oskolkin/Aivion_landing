package funnelregistry

import "sort"

const RegistryVersion = "funnel_registry_v1"

type TemporalSemantics string
type EntityAggregationSemantics string
type Surface string
type SemanticStatus string

const (
	AdditiveFlow  TemporalSemantics = "ADDITIVE_FLOW"
	DirectPeriod  TemporalSemantics = "DIRECT_PERIOD"
	ProviderRatio TemporalSemantics = "PROVIDER_RATIO"

	Additive            EntityAggregationSemantics = "ADDITIVE"
	NonAdditive         EntityAggregationSemantics = "NON_ADDITIVE"
	DirectProviderGrain EntityAggregationSemantics = "DIRECT_PROVIDER_GRAIN"

	SearchCatalog Surface = "SEARCH_CATALOG"
	PDP           Surface = "PDP"
	Cart          Surface = "CART"
	Unspecified   Surface = "UNSPECIFIED"

	Available         SemanticStatus = "AVAILABLE"
	PendingAcceptance SemanticStatus = "PENDING_ACCEPTANCE"
	Deprecated        SemanticStatus = "DEPRECATED"
)

type Entry struct {
	CanonicalKey               string
	ProviderMetric             string
	TemporalSemantics          TemporalSemantics
	EntityAggregationSemantics EntityAggregationSemantics
	Surface                    Surface
	Unit                       string
	SemanticStatus             SemanticStatus
	OptionalProviderCandidate  bool
}

var entries = []Entry{
	{"SEARCH_IMPRESSIONS", "hits_view_search", AdditiveFlow, Additive, SearchCatalog, "COUNT", Available, false},
	{"PDP_IMPRESSIONS", "hits_view_pdp", AdditiveFlow, Additive, PDP, "COUNT", Available, false},
	{"SEARCH_CART_ADDS", "hits_tocart_search", AdditiveFlow, Additive, SearchCatalog, "COUNT", Available, false},
	{"PDP_CART_ADDS", "hits_tocart_pdp", AdditiveFlow, Additive, PDP, "COUNT", Available, false},
	{"SEARCH_VIEW_SESSIONS", "session_view_search", DirectPeriod, NonAdditive, SearchCatalog, "COUNT", Available, false},
	{"PDP_VIEW_SESSIONS", "session_view_pdp", DirectPeriod, NonAdditive, PDP, "COUNT", Available, false},
	{"SEARCH_CART_CONVERSION", "conv_tocart_search", ProviderRatio, DirectProviderGrain, SearchCatalog, "RATIO", Available, false},
	{"PDP_CART_CONVERSION", "conv_tocart_pdp", ProviderRatio, DirectProviderGrain, PDP, "RATIO", Available, false},
	// Broad provider metrics are retained as discoverable candidates. They are
	// deliberately not collected until their cross-surface meaning is accepted.
	{"BROAD_IMPRESSIONS", "hits_view", AdditiveFlow, Additive, Unspecified, "COUNT", PendingAcceptance, true},
	{"BROAD_CART_ADDS", "hits_tocart", AdditiveFlow, Additive, Cart, "COUNT", PendingAcceptance, true},
	{"BROAD_VIEW_SESSIONS", "session_view", DirectPeriod, NonAdditive, Unspecified, "COUNT", PendingAcceptance, true},
	{"BROAD_CART_CONVERSION", "conv_tocart", ProviderRatio, DirectProviderGrain, Unspecified, "RATIO", PendingAcceptance, true},
}

func Version() string { return RegistryVersion }

func Entries() []Entry { return append([]Entry(nil), entries...) }

func LookupByProviderMetric(metric string) (Entry, bool) {
	for _, e := range entries {
		if e.ProviderMetric == metric {
			return e, true
		}
	}
	return Entry{}, false
}

func LookupByCanonicalKey(key string) (Entry, bool) {
	for _, e := range entries {
		if e.CanonicalKey == key {
			return e, true
		}
	}
	return Entry{}, false
}

func AdditiveDailyMetrics() []Entry { return byTemporal(AdditiveFlow) }

func DirectPeriodMetrics() []Entry {
	out := append(byTemporal(DirectPeriod), byTemporal(ProviderRatio)...)
	sort.Slice(out, func(i, j int) bool { return out[i].CanonicalKey < out[j].CanonicalKey })
	return out
}

func byTemporal(temporal TemporalSemantics) []Entry {
	var out []Entry
	for _, e := range entries {
		if e.TemporalSemantics == temporal && e.SemanticStatus == Available && !e.OptionalProviderCandidate {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CanonicalKey < out[j].CanonicalKey })
	return out
}
