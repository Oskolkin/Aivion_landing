package search

import "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection"

const (
	SummaryEndpoint          = "ozon.analytics.product_queries.v1"
	DetailsEndpoint          = "ozon.analytics.product_queries.details.v1"
	MarketEndpoint           = "ozon.search_queries.top.v1"
	RedactionProfile         = "OZON_SEARCH_REDACTION_V1"
	DetailCollectionPolicyV1 = "search_detail_collection_policy_v1"

	// LimitBySKUDefault is the provider max query rows PER SKU.
	LimitBySKUDefault = 15
	// DetailSKUSelectionLimit caps how many SKUs receive details for cost/rate control.
	// Independent of LimitBySKUDefault / provider limit_by_sku.
	DetailSKUSelectionLimit = 15
	DefaultPageSize         = 100
	DefaultSortBy           = "BY_GMV"
	DefaultSortDir          = "DESCENDING"
	RetrievalReasonTopN     = "TOP_N_PROVIDER_RANKED"
)

var SummaryContract = collection.NewSourceContract(
	"SEARCH.SELLER_SUMMARY", "ozon.seller.analytics.product-queries", "v1",
	"POST /v1/analytics/product-queries", "canonical_search_sku_summary_v1",
	"SKU×period owns provider_search_gmv totals; unique users are non-additive",
)

var DetailsContract = collection.NewSourceContract(
	"SEARCH.QUERY_DETAILS", "ozon.seller.analytics.product-queries-details", "v1",
	"POST /v1/analytics/product-queries/details", DetailCollectionPolicyV1,
	"TOP_N_PROVIDER_RANKED sample via limit_by_sku; not a complete query universe; DetailSKUSelectionLimit is independent",
)
