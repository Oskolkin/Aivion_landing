package catalog

import "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection"

var CatalogSourceContract = collection.NewSourceContract(
	"CATALOG_CONTENT_PARTIAL",
	"OZON:CATALOG:PRODUCT_LIST_INFO",
	"v2",
	"/v3/product/list+/v3/product/info/list",
	"canonical-catalog-v2",
	"identity=seller_account_id+ozon_sku;universe=ALL+ARCHIVED;product_id=mutable-metadata;conflicting-product-id=error;state-conflict=UNKNOWN;content=partial",
)

var TaxonomySourceContract = collection.NewSourceContract(
	"TAXONOMY_REFERENCE",
	"OZON:TAXONOMY:DESCRIPTION_CATEGORY_TREE",
	"v1",
	"/v1/description-category/tree",
	"ozon-taxonomy-v1",
	"provider-category=OZON+provider_category_id;product-type=provider-category+type_id;seller-category=leaf-presence-only",
)
