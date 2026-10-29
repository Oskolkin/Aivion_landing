package deep

const (
	DomainSales         = "SALES"
	DomainStock         = "STOCK"
	DomainPrice         = "PRICE"
	DomainAdvertising   = "ADVERTISING"
	DomainFunnel        = "FUNNEL"
	DomainSearch        = "SEARCH"
	DomainFinance       = "FINANCE"
	DomainPromotion     = "PROMOTION"
	DomainAccountHealth = "ACCOUNT_HEALTH"
	DomainSupply        = "SUPPLY"
	DomainCatalog       = "CATALOG"
	BucketWorsened      = "WORSENED"
	BucketNewRisk       = "NEW_RISK"
	BucketActiveRisk    = "ACTIVE_RISK"
	BucketImproved      = "IMPROVED"
	BucketNone          = "NONE"
)

func PrimaryDomainOK(v string) bool {
	switch v {
	case DomainSales, DomainStock, DomainPrice, DomainAdvertising, DomainFunnel,
		DomainSearch, DomainFinance, DomainPromotion, DomainAccountHealth, DomainSupply, DomainCatalog:
		return true
	default:
		return false
	}
}

func OverviewBucketOK(v string) bool {
	switch v {
	case BucketWorsened, BucketNewRisk, BucketActiveRisk, BucketImproved, BucketNone:
		return true
	default:
		return false
	}
}

func CanonicalPrimaryDomains() []string {
	return []string{
		DomainSales, DomainStock, DomainPrice, DomainAdvertising, DomainFunnel,
		DomainSearch, DomainFinance, DomainPromotion, DomainAccountHealth, DomainSupply, DomainCatalog,
	}
}

func CanonicalOverviewBuckets() []string {
	return []string{BucketWorsened, BucketNewRisk, BucketActiveRisk, BucketImproved, BucketNone}
}
