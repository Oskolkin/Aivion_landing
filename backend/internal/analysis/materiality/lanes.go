package materiality

import (
	"sort"
	"strconv"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
)

// LaneDefinition is the stable registry identity of a Materiality Lane.
type LaneDefinition struct {
	LaneKey    string     `json:"lane_key"`
	EntityKind EntityKind `json:"entity_kind"`
	Domain     Domain     `json:"domain"`
}

// LaneVersion is a versioned ranking semantic for a lane.
type LaneVersion struct {
	LaneKey              string     `json:"lane_key"`
	LaneVersion          string     `json:"lane_version"`
	RankingSemanticKey   string     `json:"ranking_semantic_key"`
	EntityKind           EntityKind `json:"entity_kind"`
	Domain               Domain     `json:"domain"`
	Direction            Direction  `json:"direction"`
	RequiredCapability   string     `json:"required_capability"`
	RequiredCapabilities []string   `json:"required_capabilities,omitempty"`
	CapabilityMatch      string     `json:"capability_match,omitempty"`
	AllowedReadiness     []string   `json:"allowed_readiness"`
	ExposurePolicyKey    string     `json:"exposure_policy_key"`
	CandidateLimit       int        `json:"candidate_limit"`
	Weight               float64    `json:"weight"`
	TypedImplKey         string     `json:"typed_impl_key"`
	PolicyVersion        string     `json:"policy_version"`
}

// Exposure policy keys used by PolicyV1 floors.
const (
	ExposureNone              = "NONE"
	ExposureRelativeSales     = "RELATIVE_SALES"
	ExposureRelativeAdSpend   = "RELATIVE_AD_SPEND"
	ExposureRelativeAttr      = "RELATIVE_ATTRIBUTED"
	ExposureAbsoluteSales     = "ABSOLUTE_SALES" // absolute lanes: soft/no relative floor
	ExposureDemandPositive    = "DEMAND_POSITIVE_SALES"
	ExposureSearchMeaningful  = "SEARCH_WITH_SALES"
	ExposurePriceWithSales    = "PRICE_WITH_SALES"
	ExposureSupplyOverdue     = "SUPPLY_OVERDUE_SALES"
	ExposureCampaignMetricMin = "CAMPAIGN_METRIC_MIN_SPEND"
)

// Typed implementation keys (dispatch in ranking.go). Never SQL expressions.
const (
	ImplSKUSalesCurrentExposure       = "sku.sales_current_exposure"
	ImplSKUSales30DExposure           = "sku.sales_30d_exposure"
	ImplSKUSalesAbsIncrease           = "sku.sales_abs_increase"
	ImplSKUSalesAbsDecrease           = "sku.sales_abs_decrease"
	ImplSKUSalesRelIncrease           = "sku.sales_rel_increase"
	ImplSKUSalesRelDecrease           = "sku.sales_rel_decrease"
	ImplSKUSalesFromZero              = "sku.sales_from_zero"
	ImplSKUSalesToZero                = "sku.sales_to_zero"
	ImplSKUZeroSellableRecentDemand   = "sku.zero_sellable_recent_demand"
	ImplSKUStockCoverageAsc           = "sku.stock_coverage_asc"
	ImplSKUAdPaidSpendExposure        = "sku.ad_paid_spend_exposure"
	ImplSKUAdPaidSpendIncrease        = "sku.ad_paid_spend_increase"
	ImplSKUAdPaidSpendDecrease        = "sku.ad_paid_spend_decrease"
	ImplSKUPriceAbsChange             = "sku.price_abs_change"
	ImplSKUSearchPositionChange       = "sku.search_position_change"
	ImplSKUSupplyOverdueExposure      = "sku.supply_overdue_exposure"
	ImplCatCurrentSalesExposure       = "cat.current_sales_exposure"
	ImplCatSalesAbsIncrease           = "cat.sales_abs_increase"
	ImplCatSalesAbsDecrease           = "cat.sales_abs_decrease"
	ImplCatSalesRelChange             = "cat.sales_rel_change"
	ImplCatZeroStockSalesExposure     = "cat.zero_stock_sales_exposure"
	ImplCatAdPaidSpendExposure        = "cat.ad_paid_spend_exposure"
	ImplCatAdPaidSpendChange          = "cat.ad_paid_spend_change"
	ImplCatSupplyOverdueSalesExposure = "cat.supply_overdue_sales_exposure"
	ImplCampPaidSpendExposure         = "camp.paid_spend_exposure"
	ImplCampPaidSpendIncrease         = "camp.paid_spend_increase"
	ImplCampPaidSpendDecrease         = "camp.paid_spend_decrease"
	ImplCampAttrAmountExposure        = "camp.attr_amount_exposure"
	ImplCampAttrAmountIncrease        = "camp.attr_amount_increase"
	ImplCampAttrAmountDecrease        = "camp.attr_amount_decrease"
	ImplCampDRRAbsPPChange            = "camp.drr_abs_pp_change"
	ImplCampCTRAbsPPChange            = "camp.ctr_abs_pp_change"
	ImplCampPaidSpendFromZero         = "camp.paid_spend_from_zero"
	ImplCampPaidSpendToZero           = "camp.paid_spend_to_zero"
)

// LaneRegistry holds definitions + versions for a registry seal.
type LaneRegistry struct {
	Version     string           `json:"version"`
	Definitions []LaneDefinition `json:"definitions"`
	Versions    []LaneVersion    `json:"versions"`
}

// LaneRegistryV1Current returns the pilot lane registry.
func LaneRegistryV1Current() LaneRegistry {
	defs := []LaneDefinition{
		// SKU
		{LaneKey: "SALES_CURRENT_EXPOSURE", EntityKind: EntitySKU, Domain: DomainSales},
		{LaneKey: "SALES_30D_EXPOSURE", EntityKind: EntitySKU, Domain: DomainSales},
		{LaneKey: "SALES_ABSOLUTE_INCREASE", EntityKind: EntitySKU, Domain: DomainSales},
		{LaneKey: "SALES_ABSOLUTE_DECREASE", EntityKind: EntitySKU, Domain: DomainSales},
		{LaneKey: "SALES_RELATIVE_INCREASE", EntityKind: EntitySKU, Domain: DomainSales},
		{LaneKey: "SALES_RELATIVE_DECREASE", EntityKind: EntitySKU, Domain: DomainSales},
		{LaneKey: "SALES_FROM_ZERO", EntityKind: EntitySKU, Domain: DomainSales},
		{LaneKey: "SALES_TO_ZERO", EntityKind: EntitySKU, Domain: DomainSales},
		{LaneKey: "ZERO_SELLABLE_WITH_RECENT_DEMAND", EntityKind: EntitySKU, Domain: DomainStock},
		{LaneKey: "STOCK_COVERAGE_ASC", EntityKind: EntitySKU, Domain: DomainStock},
		{LaneKey: "AD_PAID_SPEND_EXPOSURE", EntityKind: EntitySKU, Domain: DomainAdvertising},
		{LaneKey: "AD_PAID_SPEND_INCREASE", EntityKind: EntitySKU, Domain: DomainAdvertising},
		{LaneKey: "AD_PAID_SPEND_DECREASE", EntityKind: EntitySKU, Domain: DomainAdvertising},
		{LaneKey: "PRICE_ABSOLUTE_CHANGE", EntityKind: EntitySKU, Domain: DomainPrice},
		{LaneKey: "SEARCH_POSITION_CHANGE", EntityKind: EntitySKU, Domain: DomainSearch},
		{LaneKey: "SUPPLY_OVERDUE_EXPOSURE", EntityKind: EntitySKU, Domain: DomainSupply},
		// Category
		{LaneKey: "CURRENT_SALES_EXPOSURE", EntityKind: EntityCategory, Domain: DomainSales},
		{LaneKey: "SALES_ABSOLUTE_INCREASE", EntityKind: EntityCategory, Domain: DomainSales},
		{LaneKey: "SALES_ABSOLUTE_DECREASE", EntityKind: EntityCategory, Domain: DomainSales},
		{LaneKey: "SALES_RELATIVE_CHANGE", EntityKind: EntityCategory, Domain: DomainSales},
		{LaneKey: "ZERO_STOCK_SALES_EXPOSURE", EntityKind: EntityCategory, Domain: DomainStock},
		{LaneKey: "AD_PAID_SPEND_EXPOSURE", EntityKind: EntityCategory, Domain: DomainAdvertising},
		{LaneKey: "AD_PAID_SPEND_CHANGE", EntityKind: EntityCategory, Domain: DomainAdvertising},
		{LaneKey: "SUPPLY_OVERDUE_SALES_EXPOSURE", EntityKind: EntityCategory, Domain: DomainSupply},
		// Campaign
		{LaneKey: "PAID_SPEND_EXPOSURE", EntityKind: EntityCampaign, Domain: DomainAdvertising},
		{LaneKey: "PAID_SPEND_INCREASE", EntityKind: EntityCampaign, Domain: DomainAdvertising},
		{LaneKey: "PAID_SPEND_DECREASE", EntityKind: EntityCampaign, Domain: DomainAdvertising},
		{LaneKey: "ATTRIBUTED_AMOUNT_EXPOSURE", EntityKind: EntityCampaign, Domain: DomainAdvertising},
		{LaneKey: "ATTRIBUTED_AMOUNT_INCREASE", EntityKind: EntityCampaign, Domain: DomainAdvertising},
		{LaneKey: "ATTRIBUTED_AMOUNT_DECREASE", EntityKind: EntityCampaign, Domain: DomainAdvertising},
		{LaneKey: "DRR_ABSOLUTE_PP_CHANGE", EntityKind: EntityCampaign, Domain: DomainAdvertising},
		{LaneKey: "CTR_ABSOLUTE_PP_CHANGE", EntityKind: EntityCampaign, Domain: DomainAdvertising},
		{LaneKey: "PAID_SPEND_FROM_ZERO", EntityKind: EntityCampaign, Domain: DomainAdvertising},
		{LaneKey: "PAID_SPEND_TO_ZERO", EntityKind: EntityCampaign, Domain: DomainAdvertising},
	}

	ready := []string{ReadinessReady}
	readyPartial := []string{ReadinessReady, ReadinessPartial}
	lim := 40
	w := 1.0

	v := func(key string, kind EntityKind, domain Domain, dir Direction, cap, exp, impl string, allow []string) LaneVersion {
		return LaneVersion{
			LaneKey: key, LaneVersion: "v1", RankingSemanticKey: key + "_V1",
			EntityKind: kind, Domain: domain, Direction: dir,
			RequiredCapability: cap, AllowedReadiness: allow,
			ExposurePolicyKey: exp, CandidateLimit: lim, Weight: w,
			TypedImplKey: impl, PolicyVersion: MaterialityPolicyV1_3,
		}
	}

	versions := []LaneVersion{
		v("SALES_CURRENT_EXPOSURE", EntitySKU, DomainSales, DirectionDesc, "SALES.SKU_DEMAND", ExposureNone, ImplSKUSalesCurrentExposure, readyPartial),
		v("SALES_30D_EXPOSURE", EntitySKU, DomainSales, DirectionDesc, "SALES.SKU_DEMAND", ExposureNone, ImplSKUSales30DExposure, readyPartial),
		v("SALES_ABSOLUTE_INCREASE", EntitySKU, DomainSales, DirectionDesc, "SALES.SKU_DEMAND", ExposureAbsoluteSales, ImplSKUSalesAbsIncrease, readyPartial),
		v("SALES_ABSOLUTE_DECREASE", EntitySKU, DomainSales, DirectionDesc, "SALES.SKU_DEMAND", ExposureAbsoluteSales, ImplSKUSalesAbsDecrease, readyPartial),
		v("SALES_RELATIVE_INCREASE", EntitySKU, DomainSales, DirectionDesc, "SALES.SKU_DEMAND", ExposureRelativeSales, ImplSKUSalesRelIncrease, readyPartial),
		v("SALES_RELATIVE_DECREASE", EntitySKU, DomainSales, DirectionDesc, "SALES.SKU_DEMAND", ExposureRelativeSales, ImplSKUSalesRelDecrease, readyPartial),
		v("SALES_FROM_ZERO", EntitySKU, DomainSales, DirectionDesc, "SALES.SKU_DEMAND", ExposureNone, ImplSKUSalesFromZero, readyPartial),
		v("SALES_TO_ZERO", EntitySKU, DomainSales, DirectionDesc, "SALES.SKU_DEMAND", ExposureNone, ImplSKUSalesToZero, readyPartial),
		v("ZERO_SELLABLE_WITH_RECENT_DEMAND", EntitySKU, DomainStock, DirectionDesc, "STOCK.SKU_CURRENT", ExposureDemandPositive, ImplSKUZeroSellableRecentDemand, ready),
		v("STOCK_COVERAGE_ASC", EntitySKU, DomainStock, DirectionAsc, "STOCK.COVERAGE_7D", ExposureDemandPositive, ImplSKUStockCoverageAsc, ready),
		v("AD_PAID_SPEND_EXPOSURE", EntitySKU, DomainAdvertising, DirectionDesc, "ADVERTISING.SKU_TRAFFIC", ExposureNone, ImplSKUAdPaidSpendExposure, readyPartial),
		v("AD_PAID_SPEND_INCREASE", EntitySKU, DomainAdvertising, DirectionDesc, "ADVERTISING.SKU_TRAFFIC", ExposureAbsoluteSales, ImplSKUAdPaidSpendIncrease, readyPartial),
		v("AD_PAID_SPEND_DECREASE", EntitySKU, DomainAdvertising, DirectionDesc, "ADVERTISING.SKU_TRAFFIC", ExposureAbsoluteSales, ImplSKUAdPaidSpendDecrease, readyPartial),
		v("PRICE_ABSOLUTE_CHANGE", EntitySKU, DomainPrice, DirectionDesc, "PRICE.CUSTOMER_PRICE", ExposurePriceWithSales, ImplSKUPriceAbsChange, ready),
		v("SEARCH_POSITION_CHANGE", EntitySKU, DomainSearch, DirectionDesc, "SEARCH.SELLER_SUMMARY", ExposureSearchMeaningful, ImplSKUSearchPositionChange, readyPartial),
		v("SUPPLY_OVERDUE_EXPOSURE", EntitySKU, DomainSupply, DirectionDesc, "SUPPLY.STATE", ExposureSupplyOverdue, ImplSKUSupplyOverdueExposure, readyPartial),

		v("CURRENT_SALES_EXPOSURE", EntityCategory, DomainSales, DirectionDesc, "SALES.CATEGORY_DEMAND", ExposureNone, ImplCatCurrentSalesExposure, readyPartial),
		v("SALES_ABSOLUTE_INCREASE", EntityCategory, DomainSales, DirectionDesc, "SALES.CATEGORY_DEMAND", ExposureAbsoluteSales, ImplCatSalesAbsIncrease, readyPartial),
		v("SALES_ABSOLUTE_DECREASE", EntityCategory, DomainSales, DirectionDesc, "SALES.CATEGORY_DEMAND", ExposureAbsoluteSales, ImplCatSalesAbsDecrease, readyPartial),
		v("SALES_RELATIVE_CHANGE", EntityCategory, DomainSales, DirectionDesc, "SALES.CATEGORY_DEMAND", ExposureRelativeSales, ImplCatSalesRelChange, readyPartial),
		v("ZERO_STOCK_SALES_EXPOSURE", EntityCategory, DomainStock, DirectionDesc, "STOCK.SKU_CURRENT", ExposureDemandPositive, ImplCatZeroStockSalesExposure, ready),
		v("AD_PAID_SPEND_EXPOSURE", EntityCategory, DomainAdvertising, DirectionDesc, "ADVERTISING.SKU_TRAFFIC", ExposureNone, ImplCatAdPaidSpendExposure, readyPartial),
		v("AD_PAID_SPEND_CHANGE", EntityCategory, DomainAdvertising, DirectionDesc, "ADVERTISING.SKU_TRAFFIC", ExposureRelativeAdSpend, ImplCatAdPaidSpendChange, readyPartial),
		v("SUPPLY_OVERDUE_SALES_EXPOSURE", EntityCategory, DomainSupply, DirectionDesc, "SUPPLY.STATE", ExposureSupplyOverdue, ImplCatSupplyOverdueSalesExposure, readyPartial),

		v("PAID_SPEND_EXPOSURE", EntityCampaign, DomainAdvertising, DirectionDesc, "ADVERTISING.CAMPAIGN_TRAFFIC", ExposureNone, ImplCampPaidSpendExposure, readyPartial),
		v("PAID_SPEND_INCREASE", EntityCampaign, DomainAdvertising, DirectionDesc, "ADVERTISING.CAMPAIGN_TRAFFIC", ExposureAbsoluteSales, ImplCampPaidSpendIncrease, readyPartial),
		v("PAID_SPEND_DECREASE", EntityCampaign, DomainAdvertising, DirectionDesc, "ADVERTISING.CAMPAIGN_TRAFFIC", ExposureAbsoluteSales, ImplCampPaidSpendDecrease, readyPartial),
		v("ATTRIBUTED_AMOUNT_EXPOSURE", EntityCampaign, DomainAdvertising, DirectionDesc, "ADVERTISING.CAMPAIGN_ATTRIBUTION", ExposureNone, ImplCampAttrAmountExposure, readyPartial),
		v("ATTRIBUTED_AMOUNT_INCREASE", EntityCampaign, DomainAdvertising, DirectionDesc, "ADVERTISING.CAMPAIGN_ATTRIBUTION", ExposureAbsoluteSales, ImplCampAttrAmountIncrease, readyPartial),
		v("ATTRIBUTED_AMOUNT_DECREASE", EntityCampaign, DomainAdvertising, DirectionDesc, "ADVERTISING.CAMPAIGN_ATTRIBUTION", ExposureAbsoluteSales, ImplCampAttrAmountDecrease, readyPartial),
		v("DRR_ABSOLUTE_PP_CHANGE", EntityCampaign, DomainAdvertising, DirectionDesc, "ADVERTISING.CAMPAIGN_DRR", ExposureCampaignMetricMin, ImplCampDRRAbsPPChange, readyPartial),
		v("CTR_ABSOLUTE_PP_CHANGE", EntityCampaign, DomainAdvertising, DirectionDesc, "ADVERTISING.CAMPAIGN_TRAFFIC", ExposureCampaignMetricMin, ImplCampCTRAbsPPChange, readyPartial),
		v("PAID_SPEND_FROM_ZERO", EntityCampaign, DomainAdvertising, DirectionDesc, "ADVERTISING.CAMPAIGN_TRAFFIC", ExposureNone, ImplCampPaidSpendFromZero, readyPartial),
		v("PAID_SPEND_TO_ZERO", EntityCampaign, DomainAdvertising, DirectionDesc, "ADVERTISING.CAMPAIGN_TRAFFIC", ExposureNone, ImplCampPaidSpendToZero, readyPartial),
	}

	sort.Slice(defs, func(i, j int) bool {
		if defs[i].EntityKind != defs[j].EntityKind {
			return defs[i].EntityKind < defs[j].EntityKind
		}
		return defs[i].LaneKey < defs[j].LaneKey
	})
	return LaneRegistry{Version: LaneRegistryV1, Definitions: defs, Versions: versions}
}

func LaneRegistryV2Current() LaneRegistry {
	r := LaneRegistryV1Current()
	r.Version = LaneRegistryV2
	for i := range r.Versions {
		r.Versions[i].PolicyVersion = MaterialityPolicyV1_4
		if r.Versions[i].LaneKey == "DRR_ABSOLUTE_PP_CHANGE" && r.Versions[i].EntityKind == EntityCampaign {
			r.Versions[i].RequiredCapability = "ADVERTISING.CAMPAIGN_TRAFFIC"
			r.Versions[i].RequiredCapabilities = []string{"ADVERTISING.CAMPAIGN_TRAFFIC", "ADVERTISING.CAMPAIGN_ATTRIBUTION"}
			r.Versions[i].CapabilityMatch = "ALL"
		}
	}
	return r
}

// Hash returns the canonical lane registry hash for contract sealing.
func (r LaneRegistry) Hash() string {
	versions := make([]laneVersionHashPayload, len(r.Versions))
	for i, v := range r.Versions {
		versions[i] = laneVersionHashPayload{
			LaneKey:              v.LaneKey,
			LaneVersion:          v.LaneVersion,
			RankingSemanticKey:   v.RankingSemanticKey,
			EntityKind:           v.EntityKind,
			Domain:               v.Domain,
			Direction:            v.Direction,
			RequiredCapability:   v.RequiredCapability,
			RequiredCapabilities: append([]string(nil), v.RequiredCapabilities...),
			CapabilityMatch:      v.CapabilityMatch,
			AllowedReadiness:     append([]string(nil), v.AllowedReadiness...),
			ExposurePolicyKey:    v.ExposurePolicyKey,
			CandidateLimit:       v.CandidateLimit,
			Weight:               laneWeightDecimal(v.Weight),
			TypedImplKey:         v.TypedImplKey,
			PolicyVersion:        v.PolicyVersion,
		}
	}
	return canonicaljson.Hash(laneRegistryHashPayload{
		Version:     r.Version,
		Definitions: r.Definitions,
		Versions:    versions,
	})
}

type laneRegistryHashPayload struct {
	Version     string                   `json:"version"`
	Definitions []LaneDefinition         `json:"definitions"`
	Versions    []laneVersionHashPayload `json:"versions"`
}

type laneVersionHashPayload struct {
	LaneKey              string                `json:"lane_key"`
	LaneVersion          string                `json:"lane_version"`
	RankingSemanticKey   string                `json:"ranking_semantic_key"`
	EntityKind           EntityKind            `json:"entity_kind"`
	Domain               Domain                `json:"domain"`
	Direction            Direction             `json:"direction"`
	RequiredCapability   string                `json:"required_capability"`
	RequiredCapabilities []string              `json:"required_capabilities,omitempty"`
	CapabilityMatch      string                `json:"capability_match,omitempty"`
	AllowedReadiness     []string              `json:"allowed_readiness"`
	ExposurePolicyKey    string                `json:"exposure_policy_key"`
	CandidateLimit       int                   `json:"candidate_limit"`
	Weight               canonicaljson.Decimal `json:"weight"`
	TypedImplKey         string                `json:"typed_impl_key"`
	PolicyVersion        string                `json:"policy_version"`
}

func laneWeightDecimal(w float64) canonicaljson.Decimal {
	return canonicaljson.Decimal(strconv.FormatFloat(w, 'f', -1, 64))
}

// LanesForKind returns active lane versions for an entity kind.
func (r LaneRegistry) LanesForKind(kind EntityKind) []LaneVersion {
	out := make([]LaneVersion, 0)
	for _, lv := range r.Versions {
		if lv.EntityKind == kind {
			out = append(out, lv)
		}
	}
	return out
}
