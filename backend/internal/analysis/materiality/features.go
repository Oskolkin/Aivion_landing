package materiality

// CapabilityState is the readiness of a required capability for lane eligibility.
type CapabilityState struct {
	Capability string `json:"capability"`
	Readiness  string `json:"readiness"` // READY | PARTIAL | STALE | UNAVAILABLE | FAILED | ...
}

// CoreFacts is the Materiality input built from Landscape Core facts.
// Landscape packages convert LandscapeSet → CoreFacts; soft dependency only.
// Money/count fields use FeatureValue so missing is never silently zero.
type CoreFacts struct {
	Account      AccountFeatures    `json:"account"`
	Categories   []CategoryFeatures `json:"categories"`
	SKUs         []SKUFeatures      `json:"skus"`
	Campaigns    []CampaignFeatures `json:"campaigns"`
	Capabilities map[string]string  `json:"capabilities"` // capability → readiness
}

// AccountFeatures are account-scale totals used as exposure denominators.
type AccountFeatures struct {
	SemanticKey              string       `json:"semantic_key"`
	CanonicalRefID           string       `json:"canonical_ref_id,omitempty"`
	OrderedAmountCurrent7    MoneyFeature `json:"ordered_amount_current7"`
	OrderedAmountPrevious7   MoneyFeature `json:"ordered_amount_previous7"`
	OrderedAmount30D         MoneyFeature `json:"ordered_amount_30d"`
	PaidSpendCurrent7        MoneyFeature `json:"paid_spend_current7"`
	PaidSpendPrevious7       MoneyFeature `json:"paid_spend_previous7"`
	AttributedAmountCurrent7 MoneyFeature `json:"attributed_amount_current7"`
}

// CategoryFeatures are leaf seller-bearing category Core facts.
// UNMAPPED must not be included by the Landscape → CoreFacts converter.
type CategoryFeatures struct {
	SemanticKey            string       `json:"semantic_key"`
	CanonicalRefID         string       `json:"canonical_ref_id,omitempty"`
	OrderedAmountCurrent7  MoneyFeature `json:"ordered_amount_current7"`
	OrderedAmountPrevious7 MoneyFeature `json:"ordered_amount_previous7"`
	PaidSpendCurrent7      MoneyFeature `json:"paid_spend_current7"`
	PaidSpendPrevious7     MoneyFeature `json:"paid_spend_previous7"`
	ZeroStockSalesAmount   MoneyFeature `json:"zero_stock_sales_amount"` // sales of zero-sellable SKUs in category
	SupplyOverdueSales     MoneyFeature `json:"supply_overdue_sales"`
	HasSupplyOverdue       bool         `json:"has_supply_overdue"`
}

// SKUFeatures are SKU Core facts used by typed lanes.
type SKUFeatures struct {
	SemanticKey            string       `json:"semantic_key"`
	CanonicalRefID         string       `json:"canonical_ref_id,omitempty"`
	CategorySemanticKey    string       `json:"category_semantic_key"`
	OrderedAmountCurrent7  MoneyFeature `json:"ordered_amount_current7"`
	OrderedAmountPrevious7 MoneyFeature `json:"ordered_amount_previous7"`
	OrderedAmount30D       MoneyFeature `json:"ordered_amount_30d"`
	OrderedUnitsCurrent7   CountFeature `json:"ordered_units_current7"`
	OrderedUnitsPrevious7  CountFeature `json:"ordered_units_previous7"`
	SellableStock          MoneyFeature `json:"sellable_stock"`
	CoverageDays           MoneyFeature `json:"coverage_days"`
	PaidSpendCurrent7      MoneyFeature `json:"paid_spend_current7"`
	PaidSpendPrevious7     MoneyFeature `json:"paid_spend_previous7"`
	PriceCurrent           MoneyFeature `json:"price_current"`
	PricePrevious          MoneyFeature `json:"price_previous"`
	SearchPositionCurrent  MoneyFeature `json:"search_position_current"`
	SearchPositionPrevious MoneyFeature `json:"search_position_previous"`
	HasSupplyOverdue       bool         `json:"has_supply_overdue"`
	SalesAvailable         bool         `json:"sales_available"`
	StockAvailable         bool         `json:"stock_available"`
	AdsAvailable           bool         `json:"ads_available"`
	PriceAvailable         bool         `json:"price_available"`
	SearchAvailable        bool         `json:"search_available"`
}

// CampaignFeatures are campaign Core facts (advertising economic basis).
type CampaignFeatures struct {
	SemanticKey               string       `json:"semantic_key"`
	CanonicalRefID            string       `json:"canonical_ref_id,omitempty"`
	PaidSpendCurrent7         MoneyFeature `json:"paid_spend_current7"`
	PaidSpendPrevious7        MoneyFeature `json:"paid_spend_previous7"`
	AttributedAmountCurrent7  MoneyFeature `json:"attributed_amount_current7"`
	AttributedAmountPrevious7 MoneyFeature `json:"attributed_amount_previous7"`
	DRRCurrentPP              MoneyFeature `json:"drr_current_pp"` // percentage points (e.g. 20.0)
	DRRPreviousPP             MoneyFeature `json:"drr_previous_pp"`
	CTRCurrentPP              MoneyFeature `json:"ctr_current_pp"`
	CTRPreviousPP             MoneyFeature `json:"ctr_previous_pp"`
	TrafficAvailable          bool         `json:"traffic_available"`
	AttributionAvailable      bool         `json:"attribution_available"`
	DRRAvailable              bool         `json:"drr_available"`
}

// CapabilityAllows reports whether a capability readiness is in the lane allowlist.
func (c CoreFacts) CapabilityAllows(capability string, allow []string) bool {
	if capability == "" {
		return true
	}
	ready, ok := c.Capabilities[capability]
	if !ok {
		return false
	}
	for _, a := range allow {
		if a == ready {
			return true
		}
	}
	return false
}

// CapabilitiesAllowLane executes RequiredCapabilities with CapabilityMatch (ALL/ANY).
func (c CoreFacts) CapabilitiesAllowLane(lv LaneVersion) bool {
	keys := lv.RequiredCapabilities
	if len(keys) == 0 && lv.RequiredCapability != "" {
		keys = []string{lv.RequiredCapability}
	}
	if len(keys) == 0 {
		return true
	}
	switch lv.CapabilityMatch {
	case "ALL":
		for _, key := range keys {
			if !c.CapabilityAllows(key, lv.AllowedReadiness) {
				return false
			}
		}
		return true
	case "ANY":
		for _, key := range keys {
			if c.CapabilityAllows(key, lv.AllowedReadiness) {
				return true
			}
		}
		return false
	default:
		return c.CapabilityAllows(keys[0], lv.AllowedReadiness)
	}
}
