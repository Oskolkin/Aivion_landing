package price

import "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection"

const (
	CoreEndpoint      = "ozon.product.info.prices.v5"
	PremiumEndpoint   = "ozon.product.prices.details.v1"
	RedactionProfile  = "OZON_PRICE_REDACTION_V1"
	FreshnessPolicyV1 = "price_freshness_policy_v1"
	FieldSemanticsV1  = "price_field_semantics_v1"
)

type FieldSemantic struct {
	ProviderField, CanonicalRole, ZeroMeaning, AcceptanceState string
}

var FieldSemanticsRegistryV1 = map[string]FieldSemantic{
	"price.price":                  {"price.price", "PROVIDER_CURRENT_PRICE", "INVALID_CURRENT_PRICE", "PENDING_ACCEPTANCE"},
	"price.old_price":              {"price.old_price", "OLD_REFERENCE_PRICE", "NOT_SET", "PENDING_ACCEPTANCE"},
	"price.min_price":              {"price.min_price", "MINIMUM_ALLOWED_PRICE", "NOT_SET", "PENDING_ACCEPTANCE"},
	"price.marketing_seller_price": {"price.marketing_seller_price", "SELLER_PROMO_PRICE", "NOT_SET", "PENDING_ACCEPTANCE"},
	"price.marketing_price":        {"price.marketing_price", "RAW_PROVIDER_CONTEXT_ONLY", "UNVERIFIED", "PENDING_ACCEPTANCE"},
}

var CoreContract = collection.NewSourceContract(
	"PRICE.CORE_STATE",
	"ozon.seller.product.info.prices",
	"v5",
	"POST /v5/product/info/prices",
	"price_mapping_v1",
	"price.price is provider current price; old_price is current reference price and zero means unset; min_price is a current floor; marketing_price is retained only in provider context",
)

var PremiumContract = collection.NewSourceContract(
	"PRICE.CUSTOMER_PRICE",
	"ozon.seller.product.prices.details",
	"v1_optional",
	"POST /v1/product/prices/details",
	"price_premium_mapping_v1",
	"optional customer price enrichment; absence never blocks PRICE.CORE_STATE",
)
