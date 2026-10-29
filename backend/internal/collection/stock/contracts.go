package stock

import "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection"

const (
	PrimaryEndpoint  = "ozon.analytics.stocks.v1"
	RedactionProfile = "OZON_STOCK_REDACTION_V1"
)

var PrimaryContract = collection.NewSourceContract(
	"STOCK.SKU_CURRENT",
	"ozon.seller.analytics.stocks",
	"v1",
	"POST /v1/analytics/stocks",
	"stock_mapping_v1",
	"sku is canonical provider SKU; warehouse/cluster are dimensions; valid_stock_count remains provider-derived pending acceptance; missing response rows are unknown",
)

const FreshnessPolicyV1 = "stock_freshness_policy_v1"
