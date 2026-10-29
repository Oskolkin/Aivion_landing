package sales

import "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection"

var SKUDailySourceContract = collection.NewSourceContract(
	"SALES_DEMAND_SKU_DAILY", "OZON:SALES_DEMAND:SKU_DAILY", "v1", "/v1/analytics/data",
	"canonical-sales-demand-v1", "grain=seller+sku+provider-day;dimensions=sku,day;metrics=revenue->ordered_amount,ordered_units;money=exact;missing!=zero",
)

var AccountDailySourceContract = collection.NewSourceContract(
	"SALES_DEMAND_ACCOUNT_DAILY", "OZON:SALES_DEMAND:ACCOUNT_DAILY", "v1", "/v1/analytics/data",
	"canonical-sales-demand-v1", "grain=seller+provider-day;dimensions=day;metrics=revenue->ordered_amount,ordered_units;independent-query;totals=forbidden",
)

var DomainSourceContract = collection.NewSourceContract(
	"SALES_DEMAND", "OZON:SALES_DEMAND:SKU_AND_ACCOUNT_DAILY", "v1", "/v1/analytics/data",
	"canonical-sales-demand-v1", SKUDailySourceContract.SemanticMappingHash+":"+AccountDailySourceContract.SemanticMappingHash,
)
