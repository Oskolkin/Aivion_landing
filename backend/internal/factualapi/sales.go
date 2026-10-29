package factualapi

import "time"

type Demand struct {
	OrderedUnits  Value[int64]
	OrderedAmount Value[Money]
	Meta          FactMeta
}
type SKUDemand struct {
	CanonicalSKUID int64
	Demand         Demand
}
type BatchSKUDemandItem struct {
	ID         int64
	Status     BatchStatus
	Demand     *SKUDemand
	Limitation *Limitation
}
type BatchSKUDemandResult struct{ Items []BatchSKUDemandItem }
type SKUTrendContext struct {
	CanonicalSKUID            int64
	OrderedAmount30D          Value[Money]
	OrderedUnits30D           Value[int64]
	DaysWithOrders30D         Value[int64]
	TrailingDaysWithoutOrders Value[int64]
	Period                    Period
	FormulaPolicy             string
	Meta                      FactMeta
}
type BatchSKUTrendContextItem struct {
	ID         int64
	Status     BatchStatus
	Trend      *SKUTrendContext
	Limitation *Limitation
}
type BatchSKUTrendContextResult struct{ Items []BatchSKUTrendContextItem }
type DemandPoint struct {
	BusinessDate  time.Time
	OrderedUnits  Value[int64]
	OrderedAmount Value[Money]
	Meta          FactMeta
}
type SKUDemandSeries struct {
	CanonicalSKUID int64
	Points         []DemandPoint
}
type BatchSKUDemandSeriesItem struct {
	ID         int64
	Status     BatchStatus
	Series     *SKUDemandSeries
	Limitation *Limitation
}
type BatchSKUDemandSeriesResult struct{ Items []BatchSKUDemandSeriesItem }
type CategoryDemandSeries struct {
	CategoryID int64
	Points     []DemandPoint
	Unmapped   Demand
}
type CategoryDemand struct {
	CategoryID *int64
	Bucket     string
	Demand     Demand
}
