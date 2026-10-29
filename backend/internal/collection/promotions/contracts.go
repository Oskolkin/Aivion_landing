package promotions

import "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection"

const (
	ActionsEndpoint          = "ozon.promotions.actions"
	CandidatesEndpoint       = "ozon.promotions.candidates"
	ProductsEndpoint         = "ozon.promotions.products"
	SellerActionsEndpoint    = "ozon.promotions.seller_actions"
	SellerCandidatesEndpoint = "ozon.promotions.seller_candidates"
	SellerProductsEndpoint   = "ozon.promotions.seller_products"
	RedactionProfile         = "OZON_PROMOTIONS_REDACTION_V1"
)

var ActionsContract = collection.NewSourceContract("PROMOTIONS.DEFINITIONS", "ozon.seller.actions", "v1", "GET /v1/actions", "promotions_definition_mapping_v1", "identity is seller+origin+provider action id; mutable fields are versioned")
var CandidatesContract = collection.NewSourceContract("PROMOTIONS.ELIGIBILITY", "ozon.seller.actions.candidates", "v1", "POST /v1/actions/candidates", "promotions_eligibility_mapping_v1", "candidate is eligibility only and never implies participation")
var ProductsContract = collection.NewSourceContract("PROMOTIONS.MEMBERSHIP", "ozon.seller.actions.products", "v1", "POST /v1/actions/products", "promotions_membership_mapping_v1", "membership closes only after complete participant enumeration; known_from is observation time")
