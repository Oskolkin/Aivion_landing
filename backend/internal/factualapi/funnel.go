package factualapi

import "time"

type FunnelMetricValue struct {
	MetricKey                  string
	Value                      Value[Decimal]
	Unit                       string
	TemporalSemantics          string
	EntityAggregationSemantics string
	Surface                    string
	ProviderMetricName         string
	Meta                       FactMeta
}

type AccountFunnelPeriod struct {
	SellerAccountID int64
	Metric          FunnelMetricValue
}
type SKUFunnelPeriod struct {
	CanonicalSKUID int64
	Metric         FunnelMetricValue
}
type CategoryFunnelPeriod struct {
	CategoryID int64
	Metric     FunnelMetricValue
}
type FunnelPoint struct {
	PeriodStart, PeriodEnd time.Time
	Metric                 FunnelMetricValue
}
type BatchSKUFunnelPeriodItem struct {
	ID         int64
	Status     BatchStatus
	Funnel     *SKUFunnelPeriod
	Limitation *Limitation
}
type BatchSKUFunnelPeriodResult struct{ Items []BatchSKUFunnelPeriodItem }

type BatchCategoryFunnelPeriodItem struct {
	ID         int64
	Status     BatchStatus
	Funnel     *CategoryFunnelPeriod
	Limitation *Limitation
}

type BatchCategoryFunnelPeriodResult struct {
	Items []BatchCategoryFunnelPeriodItem
}

type FunnelMetricQuery struct {
	ScopeKind      string
	CanonicalSKUID int64
	MetricKey      string
	StandardPeriod StandardPeriod
	Period         *Period
}

type FunnelMetricCapability struct {
	MetricKey, ProviderMetricName, TemporalSemantics, EntityAggregationSemantics, Surface, SemanticStatus string
}
type FunnelCapabilities struct {
	RegistryVersion string
	Metrics         []FunnelMetricCapability
}
