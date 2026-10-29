package landscape

// BlockSpecV4 / ExecutableProfileV4 reuse the V3 executable contract shape.
// V3 profile names and hashes remain immutable legacy; production uses V4 names.
type BlockSpecV4 = BlockSpecV3
type ExecutableProfileV4 = ExecutableProfileV3
type CoreProfileBundleV4 = CoreProfileBundleV3
type DetailProfileBundleV4 = DetailProfileBundleV3

func derivedCapabilityBlockV4(
	id string,
	factFamilies []string,
	capabilityKeys []string,
	match CapabilityMatch,
	formulaPolicy string,
	periods ...PeriodSpec,
) BlockSpecV4 {
	if match == "" {
		match = CapabilityAll
	}
	return BlockSpecV4{
		BlockID:         id,
		Mode:            BlockDerived,
		FactFamilies:    factFamilies,
		CapabilityKeys:  capabilityKeys,
		CapabilityMatch: match,
		FormulaPolicy:   formulaPolicy,
		Periods:         periods,
		Serialization:   OmitWhenGlobalCapabilityUnavailable,
	}
}

func DefaultCoreProfilesV4() CoreProfileBundleV4 {
	return CoreProfileBundleV4{
		Account:  AccountDiscoveryV4(),
		Category: CategoryCoreV4(),
		SKU:      SKUCoreV4(),
		Campaign: CampaignCoreV4(),
	}
}

func DefaultDetailProfilesV4() DetailProfileBundleV4 {
	return DetailProfileBundleV4{
		Account:  AccountDiscoveryV4(),
		Category: CategoryDetailV4(),
		SKU:      SKUDetailV4(),
		Campaign: CampaignDetailV4(),
	}
}

func CoreProfileVersionSealV4() string {
	b := DefaultCoreProfilesV4()
	return sealExecutableProfileV3(b.Account) + "|" +
		sealExecutableProfileV3(b.Category) + "|" +
		sealExecutableProfileV3(b.SKU) + "|" +
		sealExecutableProfileV3(b.Campaign) + "|" +
		LandscapeEligibilityV1Seal
}

func DetailProfileVersionSealV4() string {
	b := DefaultDetailProfilesV4()
	return sealExecutableProfileV3(b.Account) + "|" +
		sealExecutableProfileV3(b.Category) + "|" +
		sealExecutableProfileV3(b.SKU) + "|" +
		sealExecutableProfileV3(b.Campaign) + "|" +
		LandscapeEligibilityV1Seal
}

func AccountDiscoveryV4() ExecutableProfileV4 {
	p := AccountDiscoveryV3()
	p.Name = ProfileAccountDiscoveryV4
	p.SchemaVersion = LandscapeSchemaV4
	blocks := make([]BlockSpecV3, 0, len(p.Blocks)+2)
	for _, b := range p.Blocks {
		if b.BlockID == "account.advertising" {
			blocks = append(blocks,
				derivedCapabilityBlockV4(
					"account.advertising.traffic",
					[]string{"ADVERTISING.TRAFFIC"},
					[]string{"ADVERTISING.CAMPAIGN_TRAFFIC"},
					CapabilityAll,
					"account_ads_traffic_from_campaign_v4",
					sourceNativePeriod("CURRENT_7D", "ADVERTISING.TRAFFIC"),
					sourceNativePeriod("PREVIOUS_7D", "ADVERTISING.TRAFFIC"),
				),
				derivedCapabilityBlockV4(
					"account.advertising.attribution",
					[]string{"ADVERTISING.ATTRIBUTION"},
					[]string{"ADVERTISING.CAMPAIGN_ATTRIBUTION"},
					CapabilityAll,
					"account_ads_attribution_from_campaign_v4",
					sourceNativePeriod("CURRENT_7D", "ADVERTISING.ATTRIBUTION"),
					sourceNativePeriod("PREVIOUS_7D", "ADVERTISING.ATTRIBUTION"),
				),
			)
			continue
		}
		blocks = append(blocks, b)
	}
	p.Blocks = blocks
	return p
}

func CategoryCoreV4() ExecutableProfileV4 {
	p := CategoryCoreV3()
	p.Name = ProfileCategoryCoreV4
	p.SchemaVersion = LandscapeSchemaV4
	blocks := make([]BlockSpecV3, 0, len(p.Blocks)+2)
	for _, b := range p.Blocks {
		if b.BlockID == "category.advertising" {
			blocks = append(blocks,
				derivedCapabilityBlockV4(
					"category.advertising.traffic",
					[]string{"ADVERTISING.TRAFFIC"},
					[]string{"ADVERTISING.SKU_TRAFFIC"},
					CapabilityAll,
					"category_ads_traffic_from_sku_v4",
					sourceNativePeriod("CURRENT_7D", "ADVERTISING.TRAFFIC"),
					sourceNativePeriod("PREVIOUS_7D", "ADVERTISING.TRAFFIC"),
				),
				derivedCapabilityBlockV4(
					"category.advertising.attribution",
					[]string{"ADVERTISING.ATTRIBUTION"},
					[]string{"ADVERTISING.SKU_ATTRIBUTION"},
					CapabilityAll,
					"category_ads_attribution_from_sku_v4",
					sourceNativePeriod("CURRENT_7D", "ADVERTISING.ATTRIBUTION"),
					sourceNativePeriod("PREVIOUS_7D", "ADVERTISING.ATTRIBUTION"),
				),
			)
			continue
		}
		blocks = append(blocks, b)
	}
	p.Blocks = blocks
	return p
}

func CategoryDetailV4() ExecutableProfileV4 {
	p := CategoryCoreV4()
	p.Name = ProfileCategoryDetailV4
	p.Blocks = append(append([]BlockSpecV3(nil), p.Blocks...),
		optionalBlock(
			"category.funnel",
			[]string{"FUNNEL.DAILY_ADDITIVE"},
			[]string{"BatchGetCategoryFunnelPeriod"},
			[]string{"FUNNEL.CATEGORY_ADDITIVE"},
			CapabilityAll,
			sourceNativePeriod("CURRENT_7D", "FUNNEL.DAILY_ADDITIVE"),
			sourceNativePeriod("PREVIOUS_7D", "FUNNEL.DAILY_ADDITIVE"),
		),
		optionalBlock(
			"category.search",
			[]string{"SEARCH.SELLER_SUMMARY"},
			[]string{"BatchGetCategorySearchDistribution"},
			[]string{"SEARCH.CATEGORY_SUMMARY"},
			CapabilityAll,
			sourceNativePeriod("CURRENT_7D", "SEARCH.SELLER_SUMMARY"),
			sourceNativePeriod("PREVIOUS_7D", "SEARCH.SELLER_SUMMARY"),
		),
		optionalBlock(
			"category.promotions",
			[]string{"PROMOTIONS.MEMBERSHIP"},
			[]string{"BatchGetCategoryPromotionSummary"},
			[]string{"PROMOTIONS.MEMBERSHIP"},
			CapabilityAll,
			snapshotPeriod(),
		),
		notSupportedBlockV3("category.finance"),
	)
	return p
}

func SKUCoreV4() ExecutableProfileV4 {
	p := SKUCoreV3()
	p.Name = ProfileSKUCoreV4
	p.SchemaVersion = LandscapeSchemaV4
	blocks := make([]BlockSpecV3, 0, len(p.Blocks)+2)
	for _, b := range p.Blocks {
		if b.BlockID == "sku.advertising" {
			blocks = append(blocks,
				optionalBlock(
					"sku.advertising.traffic",
					[]string{"ADVERTISING.TRAFFIC"},
					[]string{"BatchGetSKUAdvertisingTraffic"},
					[]string{"ADVERTISING.SKU_TRAFFIC"},
					CapabilityAll,
					sourceNativePeriod("CURRENT_7D", "ADVERTISING.TRAFFIC"),
					sourceNativePeriod("PREVIOUS_7D", "ADVERTISING.TRAFFIC"),
				),
				optionalBlock(
					"sku.advertising.attribution",
					[]string{"ADVERTISING.ATTRIBUTION"},
					[]string{"BatchGetSKUAdvertisingAttribution"},
					[]string{"ADVERTISING.SKU_ATTRIBUTION"},
					CapabilityAll,
					sourceNativePeriod("CURRENT_7D", "ADVERTISING.ATTRIBUTION"),
					sourceNativePeriod("PREVIOUS_7D", "ADVERTISING.ATTRIBUTION"),
				),
			)
			continue
		}
		blocks = append(blocks, b)
	}
	p.Blocks = blocks
	return p
}

func SKUDetailV4() ExecutableProfileV4 {
	p := SKUCoreV4()
	p.Name = ProfileSKUDetailV4
	p.Blocks = append(append([]BlockSpecV3(nil), p.Blocks...),
		optionalBlock(
			"sku.funnel",
			[]string{"FUNNEL.DAILY_ADDITIVE"},
			[]string{"BatchGetSKUFunnelPeriod"},
			[]string{"FUNNEL.SKU"},
			CapabilityAll,
			sourceNativePeriod("CURRENT_7D", "FUNNEL.DAILY_ADDITIVE"),
			sourceNativePeriod("PREVIOUS_7D", "FUNNEL.DAILY_ADDITIVE"),
		),
		notSupportedBlockV3("sku.finance"),
		supportingBlock(
			"sku.detail.campaign_context",
			[]string{"ADVERTISING.TRAFFIC"},
			[]string{"BatchListSKUCampaignPerformance"},
			[]string{"ADVERTISING.SKU_TRAFFIC"},
			SupportingSpec{
				Limit:         5,
				SortBasis:     "PAID_SPEND_DESC",
				SemanticScope: "TOP_N_BY_PAID_SPEND",
				Period:        sourceNativePeriod("CURRENT_7D", "ADVERTISING.TRAFFIC"),
			},
			sourceNativePeriod("CURRENT_7D", "ADVERTISING.TRAFFIC"),
		),
		supportingBlock(
			"sku.detail.promotion_context",
			[]string{"PROMOTIONS.MEMBERSHIP"},
			[]string{"BatchGetSKUPromotions"},
			[]string{"PROMOTIONS.MEMBERSHIP"},
			SupportingSpec{
				Limit:         5,
				SortBasis:     "PROMOTION_ID_ASC",
				SemanticScope: "TOP_N_ACTIVE_OR_CANDIDATE",
				Period:        snapshotPeriod(),
			},
		),
		supportingBlock(
			"sku.detail.query_context",
			[]string{"SEARCH.QUERY_DETAILS"},
			[]string{"BatchGetSKUQueryDetails"},
			[]string{"SEARCH.QUERY_DETAILS"},
			SupportingSpec{
				Limit:         5,
				SortBasis:     "PROVIDER_SEARCH_RANK",
				SemanticScope: "TOP_N_PROVIDER_RANKED",
				Period:        sourceNativePeriod("CURRENT_7D", "SEARCH.QUERY_DETAILS"),
			},
			sourceNativePeriod("CURRENT_7D", "SEARCH.QUERY_DETAILS"),
		),
	)
	return p
}

func CampaignCoreV4() ExecutableProfileV4 {
	return ExecutableProfileV4{
		Name:          ProfileCampaignCoreV4,
		SchemaVersion: LandscapeSchemaV4,
		Blocks: []BlockSpecV3{
			requiredBlock("campaign.identity", nil, nil, nil),
			requiredBlock(
				"campaign.traffic",
				[]string{"ADVERTISING.TRAFFIC"},
				[]string{"BatchGetCampaignTrafficPerformance"},
				[]string{"ADVERTISING.CAMPAIGN_TRAFFIC"},
				sourceNativePeriod("CURRENT_7D", "ADVERTISING.TRAFFIC"),
				sourceNativePeriod("PREVIOUS_7D", "ADVERTISING.TRAFFIC"),
			),
			optionalBlock(
				"campaign.attribution",
				[]string{"ADVERTISING.ATTRIBUTION"},
				[]string{"BatchGetCampaignAttributionPerformance"},
				[]string{"ADVERTISING.CAMPAIGN_ATTRIBUTION"},
				CapabilityAll,
				sourceNativePeriod("CURRENT_7D", "ADVERTISING.ATTRIBUTION"),
				sourceNativePeriod("PREVIOUS_7D", "ADVERTISING.ATTRIBUTION"),
			),
			derivedBlockV3("campaign.exposure", "ADVERTISING.TRAFFIC"),
			optionalBlock(
				"campaign.targeting",
				[]string{"ADVERTISING.TRAFFIC"},
				[]string{"BatchGetCampaignTargetingSummary"},
				[]string{"ADVERTISING.CAMPAIGN_TRAFFIC"},
				CapabilityAll,
				sourceNativePeriod("CURRENT_7D", "ADVERTISING.TRAFFIC"),
			),
		},
	}
}

func CampaignDetailV4() ExecutableProfileV4 {
	p := CampaignCoreV4()
	p.Name = ProfileCampaignDetailV4
	p.Blocks = append(append([]BlockSpecV3(nil), p.Blocks...),
		supportingBlock(
			"campaign.detail.sku_context",
			[]string{"ADVERTISING.TRAFFIC"},
			[]string{"BatchListCampaignTargetSKUs"},
			[]string{"ADVERTISING.CAMPAIGN_CONFIG"},
			SupportingSpec{
				Limit:         10,
				SortBasis:     "CANONICAL_SKU_ID_ASC",
				SemanticScope: "BOUNDED_EXPLICIT_ASSIGNMENTS",
				Period:        snapshotPeriod(),
			},
			snapshotPeriod(),
		),
		derivedBlockV3("campaign.stock"),
	)
	return p
}
