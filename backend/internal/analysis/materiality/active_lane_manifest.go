package materiality

import (
	"fmt"
	"sort"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/landscape"
)

// ActiveLaneContract seals one active Materiality lane against its Landscape V3 source contract.
type ActiveLaneContract struct {
	LaneKey        string     `json:"lane_key"`
	EntityKind     EntityKind `json:"entity_kind"`
	ProfileName    string     `json:"profile_name"`
	ProfileField   string     `json:"profile_field"`
	CapabilityKey  string     `json:"capability_key"`
	FactFamily     string     `json:"fact_family"`
	WindowPolicy   string     `json:"window_policy"`
	FactualMethods []string   `json:"factual_methods"`
}

type laneContractSpec struct {
	profileName    string
	profileField   string
	factFamily     string
	windowPolicy   string
	factualMethods []string
}

func laneContractKey(kind EntityKind, laneKey string) string {
	return string(kind) + "|" + laneKey
}

var activeLaneContractSpecs = map[string]laneContractSpec{
	laneContractKey(EntitySKU, "SALES_CURRENT_EXPOSURE"): {
		profileName: landscape.ProfileSKUCoreV4, profileField: "sales.ordered_amount.current",
		factFamily: "SALES.DEMAND", windowPolicy: "CURRENT_7D|PREVIOUS_7D",
		factualMethods: []string{"BatchGetSKUDemand"},
	},
	laneContractKey(EntitySKU, "SALES_30D_EXPOSURE"): {
		profileName: landscape.ProfileSKUCoreV4, profileField: "sales.trend30d_amount",
		factFamily: "SALES.DEMAND", windowPolicy: "TREND_30D",
		factualMethods: []string{"BatchGetSKUDemand", "BatchGetSKUTrendContext"},
	},
	laneContractKey(EntitySKU, "SALES_ABSOLUTE_INCREASE"): {
		profileName: landscape.ProfileSKUCoreV4, profileField: "sales.ordered_amount.delta",
		factFamily: "SALES.DEMAND", windowPolicy: "CURRENT_7D|PREVIOUS_7D",
		factualMethods: []string{"BatchGetSKUDemand"},
	},
	laneContractKey(EntitySKU, "SALES_ABSOLUTE_DECREASE"): {
		profileName: landscape.ProfileSKUCoreV4, profileField: "sales.ordered_amount.delta",
		factFamily: "SALES.DEMAND", windowPolicy: "CURRENT_7D|PREVIOUS_7D",
		factualMethods: []string{"BatchGetSKUDemand"},
	},
	laneContractKey(EntitySKU, "SALES_RELATIVE_INCREASE"): {
		profileName: landscape.ProfileSKUCoreV4, profileField: "sales.ordered_amount.pct_delta",
		factFamily: "SALES.DEMAND", windowPolicy: "CURRENT_7D|PREVIOUS_7D",
		factualMethods: []string{"BatchGetSKUDemand"},
	},
	laneContractKey(EntitySKU, "SALES_RELATIVE_DECREASE"): {
		profileName: landscape.ProfileSKUCoreV4, profileField: "sales.ordered_amount.pct_delta",
		factFamily: "SALES.DEMAND", windowPolicy: "CURRENT_7D|PREVIOUS_7D",
		factualMethods: []string{"BatchGetSKUDemand"},
	},
	laneContractKey(EntitySKU, "SALES_FROM_ZERO"): {
		profileName: landscape.ProfileSKUCoreV4, profileField: "sales.ordered_amount.change_state",
		factFamily: "SALES.DEMAND", windowPolicy: "CURRENT_7D|PREVIOUS_7D",
		factualMethods: []string{"BatchGetSKUDemand"},
	},
	laneContractKey(EntitySKU, "SALES_TO_ZERO"): {
		profileName: landscape.ProfileSKUCoreV4, profileField: "sales.ordered_amount.change_state",
		factFamily: "SALES.DEMAND", windowPolicy: "CURRENT_7D|PREVIOUS_7D",
		factualMethods: []string{"BatchGetSKUDemand"},
	},
	laneContractKey(EntitySKU, "ZERO_SELLABLE_WITH_RECENT_DEMAND"): {
		profileName: landscape.ProfileSKUCoreV4, profileField: "stock.sellable_units",
		factFamily: "STOCK.SNAPSHOT", windowPolicy: "SNAPSHOT",
		factualMethods: []string{"BatchGetSKUStock"},
	},
	laneContractKey(EntitySKU, "STOCK_COVERAGE_ASC"): {
		profileName: landscape.ProfileSKUCoreV4, profileField: "stock.coverage_days",
		factFamily: "STOCK.SNAPSHOT", windowPolicy: "SNAPSHOT",
		factualMethods: []string{"BatchGetStockCoverage"},
	},
	laneContractKey(EntitySKU, "AD_PAID_SPEND_EXPOSURE"): {
		profileName: landscape.ProfileSKUCoreV4, profileField: "advertising.traffic.paid_spend.current",
		factFamily: "ADVERTISING.TRAFFIC", windowPolicy: "CURRENT_7D|PREVIOUS_7D",
		factualMethods: []string{"BatchGetSKUAdvertisingTraffic"},
	},
	laneContractKey(EntitySKU, "AD_PAID_SPEND_INCREASE"): {
		profileName: landscape.ProfileSKUCoreV4, profileField: "advertising.traffic.paid_spend.delta",
		factFamily: "ADVERTISING.TRAFFIC", windowPolicy: "CURRENT_7D|PREVIOUS_7D",
		factualMethods: []string{"BatchGetSKUAdvertisingTraffic"},
	},
	laneContractKey(EntitySKU, "AD_PAID_SPEND_DECREASE"): {
		profileName: landscape.ProfileSKUCoreV4, profileField: "advertising.traffic.paid_spend.delta",
		factFamily: "ADVERTISING.TRAFFIC", windowPolicy: "CURRENT_7D|PREVIOUS_7D",
		factualMethods: []string{"BatchGetSKUAdvertisingTraffic"},
	},
	laneContractKey(EntitySKU, "PRICE_ABSOLUTE_CHANGE"): {
		profileName: landscape.ProfileSKUCoreV4, profileField: "price.customer_price.pct_delta",
		factFamily: "PRICE.SNAPSHOT", windowPolicy: "SNAPSHOT",
		factualMethods: []string{"BatchGetSKUPrice", "BatchGetSKUPriceLastObservedChange"},
	},
	laneContractKey(EntitySKU, "SEARCH_POSITION_CHANGE"): {
		profileName: landscape.ProfileSKUCoreV4, profileField: "search.position.delta",
		factFamily: "SEARCH.SELLER_SUMMARY", windowPolicy: "CURRENT_7D|PREVIOUS_7D",
		factualMethods: []string{"BatchGetSKUSearchSummary"},
	},
	laneContractKey(EntitySKU, "SUPPLY_OVERDUE_EXPOSURE"): {
		profileName: landscape.ProfileSKUCoreV4, profileField: "supply.overdue_state",
		factFamily: "SUPPLY.STATE", windowPolicy: "SNAPSHOT",
		factualMethods: []string{"BatchGetSKUInbound"},
	},

	laneContractKey(EntityCategory, "CURRENT_SALES_EXPOSURE"): {
		profileName: landscape.ProfileCategoryCoreV4, profileField: "sales.ordered_amount.current",
		factFamily: "SALES.DEMAND", windowPolicy: "CURRENT_7D|PREVIOUS_7D",
		factualMethods: []string{"ListCategoryDemand"},
	},
	laneContractKey(EntityCategory, "SALES_ABSOLUTE_INCREASE"): {
		profileName: landscape.ProfileCategoryCoreV4, profileField: "sales.ordered_amount.delta",
		factFamily: "SALES.DEMAND", windowPolicy: "CURRENT_7D|PREVIOUS_7D",
		factualMethods: []string{"ListCategoryDemand"},
	},
	laneContractKey(EntityCategory, "SALES_ABSOLUTE_DECREASE"): {
		profileName: landscape.ProfileCategoryCoreV4, profileField: "sales.ordered_amount.delta",
		factFamily: "SALES.DEMAND", windowPolicy: "CURRENT_7D|PREVIOUS_7D",
		factualMethods: []string{"ListCategoryDemand"},
	},
	laneContractKey(EntityCategory, "SALES_RELATIVE_CHANGE"): {
		profileName: landscape.ProfileCategoryCoreV4, profileField: "sales.ordered_amount.pct_delta",
		factFamily: "SALES.DEMAND", windowPolicy: "CURRENT_7D|PREVIOUS_7D",
		factualMethods: []string{"ListCategoryDemand"},
	},
	laneContractKey(EntityCategory, "ZERO_STOCK_SALES_EXPOSURE"): {
		profileName: landscape.ProfileCategoryCoreV4, profileField: "stock.sales_exposure_zero_stock",
		factFamily: "STOCK.SNAPSHOT", windowPolicy: "SNAPSHOT",
		factualMethods: []string{"BatchGetSKUStock"},
	},
	laneContractKey(EntityCategory, "AD_PAID_SPEND_EXPOSURE"): {
		profileName: landscape.ProfileCategoryCoreV4, profileField: "advertising.traffic.paid_spend.current",
		factFamily: "ADVERTISING.TRAFFIC", windowPolicy: "CURRENT_7D",
		factualMethods: []string{"BatchGetSKUAdvertisingTraffic"},
	},
	laneContractKey(EntityCategory, "AD_PAID_SPEND_CHANGE"): {
		profileName: landscape.ProfileCategoryCoreV4, profileField: "advertising.traffic.paid_spend.delta",
		factFamily: "ADVERTISING.TRAFFIC", windowPolicy: "CURRENT_7D|PREVIOUS_7D",
		factualMethods: []string{"BatchGetSKUAdvertisingTraffic"},
	},
	laneContractKey(EntityCategory, "SUPPLY_OVERDUE_SALES_EXPOSURE"): {
		profileName: landscape.ProfileCategoryCoreV4, profileField: "supply.overdue_sales",
		factFamily: "SUPPLY.STATE", windowPolicy: "SNAPSHOT",
		factualMethods: []string{"BatchGetSKUInbound"},
	},

	laneContractKey(EntityCampaign, "PAID_SPEND_EXPOSURE"): {
		profileName: landscape.ProfileCampaignCoreV4, profileField: "traffic.paid_spend.current",
		factFamily: "ADVERTISING.TRAFFIC", windowPolicy: "CURRENT_7D|PREVIOUS_7D",
		factualMethods: []string{"BatchGetCampaignTrafficPerformance"},
	},
	laneContractKey(EntityCampaign, "PAID_SPEND_INCREASE"): {
		profileName: landscape.ProfileCampaignCoreV4, profileField: "traffic.paid_spend.delta",
		factFamily: "ADVERTISING.TRAFFIC", windowPolicy: "CURRENT_7D|PREVIOUS_7D",
		factualMethods: []string{"BatchGetCampaignTrafficPerformance"},
	},
	laneContractKey(EntityCampaign, "PAID_SPEND_DECREASE"): {
		profileName: landscape.ProfileCampaignCoreV4, profileField: "traffic.paid_spend.delta",
		factFamily: "ADVERTISING.TRAFFIC", windowPolicy: "CURRENT_7D|PREVIOUS_7D",
		factualMethods: []string{"BatchGetCampaignTrafficPerformance"},
	},
	laneContractKey(EntityCampaign, "ATTRIBUTED_AMOUNT_EXPOSURE"): {
		profileName: landscape.ProfileCampaignCoreV4, profileField: "attribution.attributed_order_amount.current",
		factFamily: "ADVERTISING.ATTRIBUTION", windowPolicy: "CURRENT_7D|PREVIOUS_7D",
		factualMethods: []string{"BatchGetCampaignAttributionPerformance"},
	},
	laneContractKey(EntityCampaign, "ATTRIBUTED_AMOUNT_INCREASE"): {
		profileName: landscape.ProfileCampaignCoreV4, profileField: "attribution.attributed_order_amount.delta",
		factFamily: "ADVERTISING.ATTRIBUTION", windowPolicy: "CURRENT_7D|PREVIOUS_7D",
		factualMethods: []string{"BatchGetCampaignAttributionPerformance"},
	},
	laneContractKey(EntityCampaign, "ATTRIBUTED_AMOUNT_DECREASE"): {
		profileName: landscape.ProfileCampaignCoreV4, profileField: "attribution.attributed_order_amount.delta",
		factFamily: "ADVERTISING.ATTRIBUTION", windowPolicy: "CURRENT_7D|PREVIOUS_7D",
		factualMethods: []string{"BatchGetCampaignAttributionPerformance"},
	},
	laneContractKey(EntityCampaign, "DRR_ABSOLUTE_PP_CHANGE"): {
		profileName: landscape.ProfileCampaignCoreV4, profileField: "attribution.calculated_drr.delta",
		factFamily: "ADVERTISING.ATTRIBUTION", windowPolicy: "CURRENT_7D|PREVIOUS_7D",
		factualMethods: []string{"BatchGetCampaignTrafficPerformance", "BatchGetCampaignAttributionPerformance"},
	},
	laneContractKey(EntityCampaign, "CTR_ABSOLUTE_PP_CHANGE"): {
		profileName: landscape.ProfileCampaignCoreV4, profileField: "traffic.ctr.delta",
		factFamily: "ADVERTISING.TRAFFIC", windowPolicy: "CURRENT_7D|PREVIOUS_7D",
		factualMethods: []string{"BatchGetCampaignTrafficPerformance"},
	},
	laneContractKey(EntityCampaign, "PAID_SPEND_FROM_ZERO"): {
		profileName: landscape.ProfileCampaignCoreV4, profileField: "traffic.paid_spend.change_state",
		factFamily: "ADVERTISING.TRAFFIC", windowPolicy: "CURRENT_7D|PREVIOUS_7D",
		factualMethods: []string{"BatchGetCampaignTrafficPerformance"},
	},
	laneContractKey(EntityCampaign, "PAID_SPEND_TO_ZERO"): {
		profileName: landscape.ProfileCampaignCoreV4, profileField: "traffic.paid_spend.change_state",
		factFamily: "ADVERTISING.TRAFFIC", windowPolicy: "CURRENT_7D|PREVIOUS_7D",
		factualMethods: []string{"BatchGetCampaignTrafficPerformance"},
	},
}

// ActiveLaneManifest returns the production active-lane contract manifest (1:1 with LaneRegistry).
func ActiveLaneManifest() []ActiveLaneContract {
	reg := LaneRegistryV2Current()
	out := make([]ActiveLaneContract, 0, len(reg.Versions))
	for _, lv := range reg.Versions {
		key := laneContractKey(lv.EntityKind, lv.LaneKey)
		spec, ok := activeLaneContractSpecs[key]
		if !ok {
			panic(fmt.Sprintf("materiality: active lane manifest missing contract for %s", key))
		}
		methods := append([]string(nil), spec.factualMethods...)
		sort.Strings(methods)
		out = append(out, ActiveLaneContract{
			LaneKey:        lv.LaneKey,
			EntityKind:     lv.EntityKind,
			ProfileName:    spec.profileName,
			ProfileField:   spec.profileField,
			CapabilityKey:  lv.RequiredCapability,
			FactFamily:     spec.factFamily,
			WindowPolicy:   spec.windowPolicy,
			FactualMethods: methods,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].EntityKind != out[j].EntityKind {
			return out[i].EntityKind < out[j].EntityKind
		}
		return out[i].LaneKey < out[j].LaneKey
	})
	return out
}
