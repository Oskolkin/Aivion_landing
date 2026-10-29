package evidencecatalog

func Catalog() []Type {
	return []Type{
		ev("ACCOUNT_SALES_HISTORY", "sales", "Canonical ordered demand history for the seller account.", []EntityKind{KindAccount}, nil, []string{"SALES.ACCOUNT_DEMAND"}, MatchALL, TemporalFlowHistory30D90D, true, ""),
		ev("CATEGORY_SALES_HISTORY", "sales", "Canonical ordered demand history for a leaf category.", []EntityKind{KindCategory}, nil, []string{"SALES.CATEGORY_DEMAND"}, MatchALL, TemporalFlowHistory30D90D, true, ""),
		ev("SKU_SALES_HISTORY", "sales", "Canonical ordered demand daily history for the SKU.", []EntityKind{KindSKU}, nil, []string{"SALES.SKU_DEMAND"}, MatchALL, TemporalFlowHistory30D90D, true, ""),
		ev("ACCOUNT_SALES_CONTRIBUTORS", "sales", "SKU/category contribution to account ordered demand.", []EntityKind{KindAccount}, []EntityKind{KindSKU, KindCategory}, []string{"SALES.SKU_DEMAND"}, MatchALL, TemporalFlowHistory30D90D, true, ""),
		ev("CATEGORY_SALES_CONTRIBUTORS", "sales", "SKU contribution to category ordered demand.", []EntityKind{KindCategory}, []EntityKind{KindSKU}, []string{"SALES.SKU_DEMAND", "SALES.CATEGORY_DEMAND"}, MatchALL, TemporalFlowHistory30D90D, true, ""),

		ev("SKU_POSTING_LIFECYCLE_CONTEXT", "lifecycle", "Posting lifecycle events for the SKU.", []EntityKind{KindSKU}, nil, []string{"SALES.SKU_DEMAND"}, MatchANY, TemporalEventHistory30D, true, ""),
		ev("SKU_RETURNS_CONTEXT", "returns", "Return facts for the SKU.", []EntityKind{KindSKU}, nil, []string{"SALES.SKU_DEMAND"}, MatchANY, TemporalEventHistory30D, true, ""),

		ev("SKU_STOCK_HISTORY", "stock", "Historical stock snapshots for the SKU.", []EntityKind{KindSKU}, nil, []string{"STOCK.SKU_CURRENT"}, MatchALL, TemporalObservationHistory, true, ""),
		ev("SKU_STOCK_LOCATION_CURRENT", "stock", "Current warehouse/location stock breakdown for the SKU.", []EntityKind{KindSKU}, nil, []string{"STOCK.SKU_CURRENT"}, MatchALL, TemporalCurrentSnapshot, true, ""),
		ev("CATEGORY_STOCK_DISTRIBUTION", "stock", "Stock distribution across SKUs of a leaf category.", []EntityKind{KindCategory}, nil, []string{"STOCK.SKU_CURRENT"}, MatchALL, TemporalCurrentSnapshot, true, ""),

		ev("SKU_PRICE_HISTORY", "price", "Observed canonical price history for the SKU.", []EntityKind{KindSKU}, nil, []string{"PRICE.CUSTOMER_PRICE"}, MatchALL, TemporalObservationHistory, true, ""),
		ev("SKU_PRICING_STRATEGY_CONTEXT", "price", "Pricing strategy / index context for the SKU.", []EntityKind{KindSKU}, nil, []string{"PRICE.CUSTOMER_PRICE"}, MatchALL, TemporalCurrentSnapshot, false, "PriceIndex is not a closed strategy evidence recipe"),
		ev("CATEGORY_PRICE_CHANGE_CONTEXT", "price", "Price-change context across a leaf category.", []EntityKind{KindCategory}, nil, []string{"PRICE.CUSTOMER_PRICE"}, MatchALL, TemporalObservationHistory, false, "no category price-change expansion API"),

		ev("CAMPAIGN_PERFORMANCE_HISTORY", "advertising", "Campaign traffic/attribution time series.", []EntityKind{KindCampaign}, nil, []string{"ADVERTISING.CAMPAIGN_TRAFFIC"}, MatchANY, TemporalFlowHistory30D90D, true, ""),
		ev("CAMPAIGN_SKU_CONTRIBUTION", "advertising", "SKU contribution inside a campaign.", []EntityKind{KindCampaign}, []EntityKind{KindSKU}, []string{"ADVERTISING.CAMPAIGN_TRAFFIC"}, MatchANY, TemporalFlowHistory30D90D, true, ""),
		ev("CAMPAIGN_CONFIGURATION_HISTORY", "advertising", "Campaign configuration/version history.", []EntityKind{KindCampaign}, nil, []string{"ADVERTISING.CAMPAIGN_TRAFFIC"}, MatchANY, TemporalObservationHistory, false, "no campaign configuration history Factual API"),
		ev("CAMPAIGN_TARGETING_CONTEXT", "advertising", "Current targeting / target SKU set for the campaign.", []EntityKind{KindCampaign}, []EntityKind{KindSKU}, []string{"ADVERTISING.CAMPAIGN_TRAFFIC"}, MatchANY, TemporalCurrentSnapshot, true, ""),
		ev("SKU_ADVERTISING_HISTORY", "advertising", "SKU advertising traffic/attribution history.", []EntityKind{KindSKU}, nil, []string{"ADVERTISING.SKU_TRAFFIC", "ADVERTISING.SKU_ATTRIBUTION"}, MatchANY, TemporalFlowHistory30D90D, true, ""),
		ev("SKU_CAMPAIGN_BREAKDOWN", "advertising", "Campaign breakdown of SKU advertising activity.", []EntityKind{KindSKU}, []EntityKind{KindCampaign}, []string{"ADVERTISING.SKU_TRAFFIC"}, MatchANY, TemporalFlowHistory30D90D, true, ""),
		ev("AD_ATTRIBUTION_DETAIL", "advertising", "CPO/attribution detail rows for advertising.", []EntityKind{KindSKU, KindCampaign, KindAccount}, []EntityKind{KindSKU, KindCampaign}, []string{"ADVERTISING.SKU_ATTRIBUTION", "ADVERTISING.CAMPAIGN_ATTRIBUTION"}, MatchANY, TemporalEventHistory30D, true, ""),

		ev("ACCOUNT_FUNNEL_HISTORY", "funnel", "Account funnel surface history.", []EntityKind{KindAccount}, nil, []string{"FUNNEL.ACCOUNT"}, MatchANY, TemporalProviderPeriod, true, ""),
		ev("CATEGORY_FUNNEL_CONTEXT", "funnel", "Category funnel surface context for a period.", []EntityKind{KindCategory}, nil, []string{"FUNNEL.CATEGORY"}, MatchANY, TemporalProviderPeriod, true, ""),
		ev("SKU_FUNNEL_HISTORY", "funnel", "SKU funnel surface history.", []EntityKind{KindSKU}, nil, []string{"FUNNEL.SKU"}, MatchANY, TemporalProviderPeriod, true, ""),

		ev("SKU_SEARCH_HISTORY", "search", "SKU search summary history.", []EntityKind{KindSKU}, nil, []string{"SEARCH.SELLER_SUMMARY"}, MatchALL, TemporalProviderPeriod, true, ""),
		ev("SKU_SEARCH_QUERIES", "search", "Query-level search details for the SKU.", []EntityKind{KindSKU}, nil, []string{"SEARCH.SELLER_SUMMARY"}, MatchALL, TemporalProviderPeriod, true, ""),
		ev("SKU_SEARCH_QUERY_COMPARISON", "search", "Current-vs-previous query comparison for the SKU.", []EntityKind{KindSKU}, nil, []string{"SEARCH.SELLER_SUMMARY"}, MatchALL, TemporalProviderPeriod, true, ""),
		ev("SEARCH_QUERY_MARKET_CONTEXT", "search", "Market/search-query context facts.", []EntityKind{KindSKU, KindAccount}, nil, []string{"SEARCH.SELLER_SUMMARY"}, MatchANY, TemporalProviderPeriod, true, ""),

		ev("ACCOUNT_FINANCE_HISTORY", "finance", "Account finance daily ledger / net accrual history.", []EntityKind{KindAccount}, nil, []string{"FINANCE.ACCOUNT_ACCRUAL"}, MatchALL, TemporalFlowHistory30D90D, true, ""),
		ev("CATEGORY_FINANCE_BREAKDOWN", "finance", "Finance breakdown for a leaf category.", []EntityKind{KindCategory}, nil, []string{"FINANCE.ACCOUNT_ACCRUAL"}, MatchALL, TemporalFlowHistory30D90D, false, "no truthful category finance expansion recipe on current Factual API"),
		ev("SKU_FINANCE_BREAKDOWN", "finance", "Direct-mapped finance for the SKU.", []EntityKind{KindSKU}, nil, []string{"FINANCE.ACCOUNT_ACCRUAL"}, MatchALL, TemporalFlowHistory30D90D, false, "FinanceAccrualFilter has no SKU grain; allocating account-only fees is forbidden"),
		ev("POSTING_FINANCE_DETAIL", "finance", "Posting-level finance accruals.", []EntityKind{KindSKU, KindAccount}, nil, []string{"FINANCE.ACCOUNT_ACCRUAL"}, MatchALL, TemporalEventHistory30D, true, ""),
		ev("CAMPAIGN_FINANCE_CONTEXT", "finance", "Finance context attributed to a campaign.", []EntityKind{KindCampaign}, nil, []string{"FINANCE.ACCOUNT_ACCRUAL"}, MatchALL, TemporalFlowHistory30D90D, false, "campaign finance is not a distinct canonical expansion API"),
		ev("SUPPLY_FINANCE_CONTEXT", "finance", "Finance context for supply/inbound.", []EntityKind{KindSKU, KindAccount}, nil, []string{"FINANCE.ACCOUNT_ACCRUAL"}, MatchALL, TemporalFlowHistory30D90D, false, "no supply-finance expansion API"),

		ev("SKU_PROMOTION_CONTEXT", "promotions", "Current promotion membership context for the SKU.", []EntityKind{KindSKU}, nil, []string{"PROMOTIONS.SKU_MEMBERSHIP"}, MatchANY, TemporalCurrentSnapshot, true, ""),
		ev("SKU_PROMOTION_HISTORY", "promotions", "Promotion membership history for the SKU.", []EntityKind{KindSKU}, nil, []string{"PROMOTIONS.SKU_MEMBERSHIP"}, MatchANY, TemporalObservationHistory, true, ""),
		ev("SKU_PROMOTION_ELIGIBILITY", "promotions", "Promotion eligibility for the SKU.", []EntityKind{KindSKU}, nil, []string{"PROMOTIONS.SKU_MEMBERSHIP"}, MatchANY, TemporalCurrentSnapshot, true, ""),
		ev("CATEGORY_PROMOTION_CONTEXT", "promotions", "Promotion context for a leaf category.", []EntityKind{KindCategory}, nil, []string{"PROMOTIONS.SKU_MEMBERSHIP"}, MatchANY, TemporalCurrentSnapshot, true, ""),
		ev("PROMOTION_DETAIL", "promotions", "Provider promotion detail.", []EntityKind{KindSKU, KindCategory, KindAccount}, nil, []string{"PROMOTIONS.SKU_MEMBERSHIP"}, MatchANY, TemporalCurrentSnapshot, true, ""),

		ev("ACCOUNT_RATING_HISTORY", "account_health", "Account rating history.", []EntityKind{KindAccount}, nil, []string{"ACCOUNT_HEALTH.RATINGS"}, MatchANY, TemporalProviderPeriod, true, ""),
		ev("ACCOUNT_RATING_THRESHOLD_CONTEXT", "account_health", "Rating threshold context.", []EntityKind{KindAccount}, nil, []string{"ACCOUNT_HEALTH.RATINGS"}, MatchANY, TemporalCurrentSnapshot, true, ""),
		ev("ACCOUNT_FBS_ERROR_HISTORY", "account_health", "FBS error index history.", []EntityKind{KindAccount}, nil, []string{"ACCOUNT_HEALTH.FBS_ERRORS"}, MatchANY, TemporalEventHistory30D, true, ""),
		ev("ACCOUNT_FBS_ERROR_POSTINGS", "account_health", "FBS posting error events.", []EntityKind{KindAccount}, nil, []string{"ACCOUNT_HEALTH.FBS_ERRORS"}, MatchANY, TemporalEventHistory30D, true, ""),

		ev("SKU_SUPPLY_CONTEXT", "supply", "Open inbound/supply context for the SKU.", []EntityKind{KindSKU}, nil, []string{"SUPPLY.STATE"}, MatchANY, TemporalCurrentSnapshot, true, ""),
		ev("SUPPLY_LIFECYCLE_DETAIL", "supply", "Supply state history for a supply document.", []EntityKind{KindSKU, KindAccount}, nil, []string{"SUPPLY.STATE"}, MatchANY, TemporalEventHistory30D, true, ""),
		ev("SUPPLY_CONTENT_DETAIL", "supply", "Planned supply content lines.", []EntityKind{KindSKU, KindAccount}, nil, []string{"SUPPLY.STATE"}, MatchANY, TemporalCurrentSnapshot, true, ""),
		ev("SUPPLY_ACCEPTANCE_CONTEXT", "supply", "Supply acceptance act context.", []EntityKind{KindSKU, KindAccount}, nil, []string{"SUPPLY.STATE"}, MatchANY, TemporalCurrentSnapshot, true, ""),
		ev("SKU_ACCEPTANCE_DISCREPANCIES", "supply", "Acceptance discrepancies for the SKU.", []EntityKind{KindSKU}, nil, []string{"SUPPLY.STATE"}, MatchANY, TemporalEventHistory30D, true, ""),
		ev("CATEGORY_SUPPLY_CONTEXT", "supply", "Supply context aggregated to a leaf category.", []EntityKind{KindCategory}, nil, []string{"SUPPLY.STATE"}, MatchANY, TemporalCurrentSnapshot, false, "no category inbound expansion API"),

		ev("SKU_CATEGORY_HISTORY", "catalog", "SKU category assignment history.", []EntityKind{KindSKU}, []EntityKind{KindCategory}, []string{"CATALOG.SKU"}, MatchANY, TemporalObservationHistory, false, "no SKU category history Factual API"),
		ev("SKU_CATALOG_STATE_HISTORY", "catalog", "SKU catalog state history.", []EntityKind{KindSKU}, nil, []string{"CATALOG.SKU"}, MatchANY, TemporalObservationHistory, false, "no SKU catalog state history Factual API"),
	}
}

func ev(key, domain, desc string, subjects, related []EntityKind, caps []string, match CapabilityMatch, temporal string, enabled bool, reason string) Type {
	if related == nil {
		related = []EntityKind{}
	}
	return Type{
		Key: key, Domain: domain, Description: desc,
		AllowedSubjectKinds: subjects, AllowedRelatedKinds: related,
		RequiredCapabilities: caps, CapabilityMatch: match,
		TemporalProfileKey: temporal, EnabledForDiscoveryV1: enabled, DisabledReason: reason,
	}
}

func ByKey() map[string]Type {
	out := map[string]Type{}
	for _, t := range Catalog() {
		out[t.Key] = t
	}
	return out
}

func ExpectedKeys() []string {
	keys := make([]string, 0, 50)
	for _, t := range Catalog() {
		keys = append(keys, t.Key)
	}
	return keys
}

func Enabled() []Type {
	out := make([]Type, 0)
	for _, t := range Catalog() {
		if t.EnabledForDiscoveryV1 {
			out = append(out, t)
		}
	}
	return out
}
