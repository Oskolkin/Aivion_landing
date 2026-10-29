package landscape

// Profile describes field presence and supporting-list limits for a Landscape build.
type Profile struct {
	Name string

	IncludeSales         bool
	IncludeStock         bool
	IncludePrice         bool
	IncludeAdvertising   bool
	IncludeFunnel        bool
	IncludeSearch        bool
	IncludeFinance       bool
	IncludePromotions    bool
	IncludeHealth        bool
	IncludeSupply        bool
	IncludeAssortment    bool
	IncludeConcentration bool

	// Detail supporting-context limits (K). Zero means omit lists.
	MaxCampaignContext  int
	MaxPromotionContext int
	MaxQueryContext     int
	MaxSKUAdsContext    int
}

func AccountDiscoveryProfile() Profile {
	return Profile{
		Name:         ProfileAccountDiscoveryV1,
		IncludeSales: true, IncludeAssortment: true, IncludeStock: true, IncludePrice: true,
		IncludeAdvertising: true, IncludeFunnel: true, IncludeSearch: true, IncludeFinance: true,
		IncludePromotions: true, IncludeHealth: true, IncludeSupply: true, IncludeConcentration: true,
	}
}

func CategoryCoreProfile() Profile {
	return Profile{
		Name:         ProfileCategoryCoreV1,
		IncludeSales: true, IncludeStock: true, IncludePrice: true, IncludeAdvertising: true,
		IncludeFunnel: true, IncludeSearch: true, IncludeFinance: true, IncludePromotions: true, IncludeSupply: true,
	}
}

func CategoryDetailProfile() Profile {
	p := CategoryCoreProfile()
	p.Name = ProfileCategoryDetailV1
	return p
}

func SKUCoreProfile() Profile {
	return Profile{
		Name:         ProfileSKUCoreV1,
		IncludeSales: true, IncludeStock: true, IncludePrice: true, IncludeAdvertising: true,
		IncludeFunnel: true, IncludeSearch: true, IncludeFinance: true, IncludePromotions: true, IncludeSupply: true,
	}
}

func SKUDetailProfile() Profile {
	p := SKUCoreProfile()
	p.Name = ProfileSKUDetailV1
	p.MaxCampaignContext = 5
	p.MaxPromotionContext = 5
	p.MaxQueryContext = 5
	return p
}

func CampaignCoreProfile() Profile {
	return Profile{
		Name:               ProfileCampaignCoreV1,
		IncludeAdvertising: true, IncludeStock: true,
	}
}

func CampaignDetailProfile() Profile {
	p := CampaignCoreProfile()
	p.Name = ProfileCampaignDetailV1
	p.MaxSKUAdsContext = 10
	return p
}

func ProfileByName(name string) (Profile, bool) {
	switch name {
	case ProfileAccountDiscoveryV1:
		return AccountDiscoveryProfile(), true
	case ProfileCategoryCoreV1:
		return CategoryCoreProfile(), true
	case ProfileCategoryDetailV1:
		return CategoryDetailProfile(), true
	case ProfileSKUCoreV1:
		return SKUCoreProfile(), true
	case ProfileSKUDetailV1:
		return SKUDetailProfile(), true
	case ProfileCampaignCoreV1:
		return CampaignCoreProfile(), true
	case ProfileCampaignDetailV1:
		return CampaignDetailProfile(), true
	default:
		return Profile{}, false
	}
}
