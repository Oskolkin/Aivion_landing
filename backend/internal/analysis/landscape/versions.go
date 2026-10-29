package landscape

// Landscape schema / profile / continuity version seals (Prompt №9 / №9.3).
const (
	LandscapeSchemaV1 = "landscape_schema_v1"
	LandscapeSchemaV2 = "landscape_schema_v2"
	LandscapeSchemaV3 = "landscape_schema_v3"
	LandscapeSchemaV4 = "landscape_schema_v4"
	LandscapeSchemaV5 = "landscape_schema_v5"

	ProfileAccountDiscoveryV1 = "ACCOUNT_DISCOVERY_V1"
	ProfileCategoryCoreV1     = "CATEGORY_CORE_V1"
	ProfileCategoryDetailV1   = "CATEGORY_DETAIL_V1"
	ProfileSKUCoreV1          = "SKU_CORE_V1"
	ProfileSKUDetailV1        = "SKU_DETAIL_V1"
	ProfileCampaignCoreV1     = "CAMPAIGN_CORE_V1"
	ProfileCampaignDetailV1   = "CAMPAIGN_DETAIL_V1"

	ProfileAccountDiscoveryV2 = "ACCOUNT_DISCOVERY_V2"
	ProfileCategoryCoreV2     = "CATEGORY_CORE_V2"
	ProfileCategoryDetailV2   = "CATEGORY_DETAIL_V2"
	ProfileSKUCoreV2          = "SKU_CORE_V2"
	ProfileSKUDetailV2        = "SKU_DETAIL_V2"
	ProfileCampaignCoreV2     = "CAMPAIGN_CORE_V2"
	ProfileCampaignDetailV2   = "CAMPAIGN_DETAIL_V2"

	ProfileAccountDiscoveryV3 = "ACCOUNT_DISCOVERY_V3"
	ProfileCategoryCoreV3     = "CATEGORY_CORE_V3"
	ProfileCategoryDetailV3   = "CATEGORY_DETAIL_V3"
	ProfileSKUCoreV3          = "SKU_CORE_V3"
	ProfileSKUDetailV3        = "SKU_DETAIL_V3"
	ProfileCampaignCoreV3     = "CAMPAIGN_CORE_V3"
	ProfileCampaignDetailV3   = "CAMPAIGN_DETAIL_V3"

	ProfileAccountDiscoveryV4 = "ACCOUNT_DISCOVERY_V4"
	ProfileCategoryCoreV4     = "CATEGORY_CORE_V4"
	ProfileCategoryDetailV4   = "CATEGORY_DETAIL_V4"
	ProfileSKUCoreV4          = "SKU_CORE_V4"
	ProfileSKUDetailV4        = "SKU_DETAIL_V4"
	ProfileCampaignCoreV4     = "CAMPAIGN_CORE_V4"
	ProfileCampaignDetailV4   = "CAMPAIGN_DETAIL_V4"

	ProfileAccountDiscoveryV5 = "ACCOUNT_DISCOVERY_V5"
	ProfileCategoryCoreV5     = "CATEGORY_CORE_V5"
	ProfileCategoryDetailV5   = "CATEGORY_DETAIL_V5"
	ProfileSKUCoreV5          = "SKU_CORE_V5"
	ProfileSKUDetailV5        = "SKU_DETAIL_V5"
	ProfileCampaignCoreV5     = "CAMPAIGN_CORE_V5"
	ProfileCampaignDetailV5   = "CAMPAIGN_DETAIL_V5"

	// ErrProfileCapabilityContractViolation is returned when a required profile block
	// capability is absent or unusable in the sealed Factual context (Prompt №9.3 Part M).
	ErrProfileCapabilityContractViolation = "PROFILE_CAPABILITY_CONTRACT_VIOLATION"

	// ContinuityInitialNoBaseV1 is the only continuity mode in Prompt №9:
	// first-run / no prior publication base — never invent a synthetic baseline.
	ContinuityInitialNoBaseV1 = "CONTINUITY_INITIAL_NO_BASE_V1"

	// LandscapeEligibilityV1Seal mirrors eligibility.go policy version for contracts.
	LandscapeEligibilityV1Seal = LandscapeEligibilityV1
)
