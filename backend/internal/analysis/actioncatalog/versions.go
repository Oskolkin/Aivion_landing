package actioncatalog

const (
	CatalogV1     = "action_catalog_v1"
	MenuPolicyV1  = "action_menu_policy_v1"
	ParamSchemaV1 = "action_parameter_schema_v1"
)

const (
	FamilyPricing       = "PRICING"
	FamilyInventory     = "INVENTORY"
	FamilyAdvertising   = "ADVERTISING"
	FamilyPromotion     = "PROMOTION"
	FamilyAssortment    = "ASSORTMENT"
	FamilyAccountHealth = "ACCOUNT_HEALTH"
)

const (
	TypeAccountRatingRemediation          = "ACCOUNT_RATING_REMEDIATION"
	TypeCategoryRepricingPlan             = "CATEGORY_REPRICING_PLAN"
	TypeCategoryReplenishmentPlan         = "CATEGORY_REPLENISHMENT_PLAN"
	TypeCategoryAdvertisingReallocation   = "CATEGORY_ADVERTISING_REALLOCATION"
	TypeCategoryPromotionPlan             = "CATEGORY_PROMOTION_PLAN"
	TypeCategoryAssortmentRationalization = "CATEGORY_ASSORTMENT_RATIONALIZATION"
	TypeSKUAdjustPrice                    = "SKU_ADJUST_PRICE"
	TypeSKUIncreaseReplenishment          = "SKU_INCREASE_REPLENISHMENT"
	TypeSKUReduceReplenishment            = "SKU_REDUCE_REPLENISHMENT"
	TypeSKUAdjustAdBid                    = "SKU_ADJUST_AD_BID"
	TypeSKUChangeAdParticipation          = "SKU_CHANGE_AD_PARTICIPATION"
	TypeSKUJoinPromotion                  = "SKU_JOIN_PROMOTION"
	TypeSKULeavePromotion                 = "SKU_LEAVE_PROMOTION"
	TypeSKUAdjustPromotionPrice           = "SKU_ADJUST_PROMOTION_PRICE"
	TypeSKUChangeCatalogState             = "SKU_CHANGE_CATALOG_STATE"
	TypeCampaignAdjustBudget              = "CAMPAIGN_ADJUST_BUDGET"
	TypeCampaignChangeState               = "CAMPAIGN_CHANGE_STATE"
	TypeCampaignAdjustStrategy            = "CAMPAIGN_ADJUST_STRATEGY"
	TypeCampaignAdjustProductSet          = "CAMPAIGN_ADJUST_PRODUCT_SET"
)
