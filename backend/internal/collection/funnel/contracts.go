package funnel

import (
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/funnelregistry"
)

const (
	Endpoint         = "ozon.analytics.funnel"
	RedactionProfile = "OZON_FUNNEL_REDACTION_V1"
)

var DomainSourceContract = collection.NewSourceContract(
	"FUNNEL",
	"OZON:FUNNEL:ANALYTICS_DATA",
	"v1",
	"POST /v1/analytics/data",
	funnelregistry.Version(),
	"registry="+funnelregistry.Version()+";missing!=zero;sku=ozon_sku;daily=additive-only;direct-period=provider-only;ordered_units/revenue=forbidden",
)
