package supply

import "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection"

const (
	ListEndpoint       = "ozon.supply.order.list"
	GetEndpoint        = "ozon.supply.order.get"
	DetailsEndpoint    = "ozon.supply.order.details"
	BundleEndpoint     = "ozon.supply.order.bundle"
	ActSummaryEndpoint = "ozon.supply.act.summary"
	ActProductEndpoint = "ozon.supply.act.product"
	RedactionProfile   = "OZON_SUPPLY_REDACTION_V1"
)

var StateContract = collection.NewSourceContract("SUPPLY.STATE", "ozon.seller.supply-order", "v3-v1", "POST /v3/supply-order/list|get; POST /v1/supply-order/details", "supply_state_mapping_v1", "FBO inbound only; order identity is seller+order_id and supply identity is seller+supply_id; unknown states retained")
var BundleContract = collection.NewSourceContract("SUPPLY.BUNDLE", "ozon.seller.supply-order.bundle", "v1", "POST /v1/supply-order/bundle", "supply_bundle_mapping_v1", "bundle is a versioned plan, never supply identity; partial pagination never closes composition")
var AcceptanceContract = collection.NewSourceContract("SUPPLY.ACCEPTANCE", "ozon.seller.supply-order.act", "v1-beta-readonly", "POST /v1/supply-order/act/summary/get|product/get", "supply_acceptance_mapping_v1", "read-only beta; accepted, shortage and surplus are independent provider facts; money is not Finance")
