package landscape

// BlockSpecV5 / ExecutableProfileV5 reuse the V4 executable contract shape.
// V4 profile names and hashes remain immutable; production uses V5 names so
// short-history Sales block semantics (PARTIAL vs READY_EMPTY) cannot replay
// under landscape_schema_v4.
type BlockSpecV5 = BlockSpecV4
type ExecutableProfileV5 = ExecutableProfileV4
type CoreProfileBundleV5 = CoreProfileBundleV4
type DetailProfileBundleV5 = DetailProfileBundleV4

func stampProfileV5(p ExecutableProfileV4, name string) ExecutableProfileV4 {
	p.Name = name
	p.SchemaVersion = LandscapeSchemaV5
	return p
}

func DefaultCoreProfilesV5() CoreProfileBundleV5 {
	b := DefaultCoreProfilesV4()
	return CoreProfileBundleV5{
		Account:  stampProfileV5(b.Account, ProfileAccountDiscoveryV5),
		Category: stampProfileV5(b.Category, ProfileCategoryCoreV5),
		SKU:      stampProfileV5(b.SKU, ProfileSKUCoreV5),
		Campaign: stampProfileV5(b.Campaign, ProfileCampaignCoreV5),
	}
}

func DefaultDetailProfilesV5() DetailProfileBundleV5 {
	b := DefaultDetailProfilesV4()
	return DetailProfileBundleV5{
		Account:  stampProfileV5(b.Account, ProfileAccountDiscoveryV5),
		Category: stampProfileV5(b.Category, ProfileCategoryDetailV5),
		SKU:      stampProfileV5(b.SKU, ProfileSKUDetailV5),
		Campaign: stampProfileV5(b.Campaign, ProfileCampaignDetailV5),
	}
}

func CoreProfileVersionSealV5() string {
	b := DefaultCoreProfilesV5()
	return sealExecutableProfileV3(b.Account) + "|" +
		sealExecutableProfileV3(b.Category) + "|" +
		sealExecutableProfileV3(b.SKU) + "|" +
		sealExecutableProfileV3(b.Campaign) + "|" +
		LandscapeEligibilityV1Seal
}

func DetailProfileVersionSealV5() string {
	b := DefaultDetailProfilesV5()
	return sealExecutableProfileV3(b.Account) + "|" +
		sealExecutableProfileV3(b.Category) + "|" +
		sealExecutableProfileV3(b.SKU) + "|" +
		sealExecutableProfileV3(b.Campaign) + "|" +
		LandscapeEligibilityV1Seal
}
