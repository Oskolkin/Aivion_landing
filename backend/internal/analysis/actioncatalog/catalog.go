package actioncatalog

import "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/discovery"

func def(key, family, domain, target, conflict string, related, caps, ctx []string) Definition {
	return Definition{
		ActionTypeKey: key, Version: CatalogV1, TargetKind: discovery.EntityKind(target),
		ActionFamily: family, SourceDomain: domain, AllowedRelatedEntities: related,
		RequiredCapabilities: caps, RequiredFactualContext: ctx, ConflictGroup: conflict,
		ExecutionSupport: "MANUAL_V1", ExecutionRisk: "STANDARD", ValidationPolicy: ParamSchemaV1,
	}
}

func Catalog() []Definition {
	return []Definition{
		def(TypeAccountRatingRemediation, FamilyAccountHealth, "ACCOUNT_HEALTH", "ACCOUNT", "ACCOUNT_HEALTH",
			[]string{}, []string{"ACCOUNT_HEALTH"}, []string{"ACCOUNT_HEALTH_KEYS"}),
		def(TypeCategoryRepricingPlan, FamilyPricing, "PRICE", "CATEGORY", "CATEGORY_PRICING",
			[]string{"SKU"}, []string{"PRICE", "SALES"}, []string{"CATEGORY_MATERIAL_SKUS"}),
		def(TypeCategoryReplenishmentPlan, FamilyInventory, "STOCK", "CATEGORY", "CATEGORY_INVENTORY",
			[]string{"SKU"}, []string{"STOCK", "SALES"}, []string{"CATEGORY_MATERIAL_SKUS"}),
		def(TypeCategoryAdvertisingReallocation, FamilyAdvertising, "ADVERTISING", "CATEGORY", "CATEGORY_ADS",
			[]string{"SKU", "CAMPAIGN"}, []string{"ADVERTISING"}, []string{"CATEGORY_MATERIAL_SKUS"}),
		def(TypeCategoryPromotionPlan, FamilyPromotion, "PROMOTIONS", "CATEGORY", "CATEGORY_PROMOTION",
			[]string{"SKU", "PROMOTION"}, []string{"PROMOTIONS"}, []string{"CATEGORY_MATERIAL_SKUS"}),
		def(TypeCategoryAssortmentRationalization, FamilyAssortment, "CATALOG", "CATEGORY", "CATEGORY_ASSORTMENT",
			[]string{"SKU"}, []string{"SALES"}, []string{"CATEGORY_MATERIAL_SKUS"}),
		def(TypeSKUAdjustPrice, FamilyPricing, "PRICE", "SKU", "SKU_PRICE",
			[]string{}, []string{"PRICE"}, []string{"PRICE_CURRENT"}),
		def(TypeSKUIncreaseReplenishment, FamilyInventory, "STOCK", "SKU", "SKU_REPLENISHMENT",
			[]string{}, []string{"STOCK", "SALES"}, []string{"STOCK_OR_DEMAND"}),
		def(TypeSKUReduceReplenishment, FamilyInventory, "SUPPLY", "SKU", "SKU_REPLENISHMENT",
			[]string{}, []string{"SUPPLY"}, []string{"REPLENISHMENT_INBOUND"}),
		def(TypeSKUAdjustAdBid, FamilyAdvertising, "ADVERTISING", "SKU", "SKU_ADS",
			[]string{"CAMPAIGN"}, []string{"ADVERTISING"}, []string{"SKU_CAMPAIGN_RELATION", "BID_LIMITS"}),
		def(TypeSKUChangeAdParticipation, FamilyAdvertising, "ADVERTISING", "SKU", "SKU_ADS",
			[]string{"CAMPAIGN"}, []string{"ADVERTISING"}, []string{"SKU_CAMPAIGN_RELATION"}),
		def(TypeSKUJoinPromotion, FamilyPromotion, "PROMOTIONS", "SKU", "SKU_PROMOTION",
			[]string{"PROMOTION"}, []string{"PROMOTIONS"}, []string{"PROMOTION_CANDIDATE"}),
		def(TypeSKULeavePromotion, FamilyPromotion, "PROMOTIONS", "SKU", "SKU_PROMOTION",
			[]string{"PROMOTION"}, []string{"PROMOTIONS"}, []string{"PROMOTION_PARTICIPANT"}),
		def(TypeSKUAdjustPromotionPrice, FamilyPromotion, "PROMOTIONS", "SKU", "SKU_PROMOTION",
			[]string{"PROMOTION"}, []string{"PROMOTIONS"}, []string{"PROMOTION_PARTICIPANT"}),
		def(TypeSKUChangeCatalogState, FamilyAssortment, "CATALOG", "SKU", "SKU_CATALOG",
			[]string{}, []string{"SALES"}, []string{"CATALOG_STATE"}),
		def(TypeCampaignAdjustBudget, FamilyAdvertising, "ADVERTISING", "CAMPAIGN", "CAMPAIGN_BUDGET",
			[]string{}, []string{"ADVERTISING"}, []string{"CAMPAIGN_BUDGET"}),
		def(TypeCampaignChangeState, FamilyAdvertising, "ADVERTISING", "CAMPAIGN", "CAMPAIGN_STATE",
			[]string{}, []string{"ADVERTISING"}, []string{"CAMPAIGN_STATE"}),
		def(TypeCampaignAdjustStrategy, FamilyAdvertising, "ADVERTISING", "CAMPAIGN", "CAMPAIGN_STRATEGY",
			[]string{}, []string{"ADVERTISING"}, []string{"CAMPAIGN_STRATEGY_ENUM"}),
		def(TypeCampaignAdjustProductSet, FamilyAdvertising, "ADVERTISING", "CAMPAIGN", "CAMPAIGN_PRODUCTS",
			[]string{"SKU"}, []string{"ADVERTISING"}, []string{"CAMPAIGN_PRODUCT_SET"}),
	}
}

func ByKey() map[string]Definition {
	out := make(map[string]Definition, 19)
	for _, d := range Catalog() {
		out[d.ActionTypeKey] = d
	}
	return out
}

func MustHave19() []Definition {
	c := Catalog()
	if len(c) != 19 {
		panic("action catalog v1 must contain exactly 19 types")
	}
	return c
}
