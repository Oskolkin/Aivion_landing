package advertising

import "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection"

const (
	CampaignEndpoint               = "ozon.performance.campaign.registry"
	DailyEndpoint                  = "ozon.performance.statistics.daily"
	ProductDailyEndpoint           = "ozon.performance.statistics.campaign.product"
	ExpenseEndpoint                = "ozon.performance.statistics.expense"
	TargetEndpoint                 = "ozon.performance.campaign.products"
	CPOReportEndpoint              = "ozon.performance.statistics.cpo.orders"
	RedactionProfile               = "OZON_ADVERTISING_REDACTION_V1"
	CampaignPageLimit              = 100
	CampaignReportBatchLimit       = 10
	TargetPageLimit                = 100
	ReportPollAttempts             = 8
	ReportPollIntervalMs           = 1500
	AttributionMaturityPolicyV1    = "advertising_attribution_maturity_policy_v1"
	AttributionAggregationPolicyV1 = "advertising_attribution_aggregation_policy_v1"
)

var CampaignContract = collection.NewSourceContract("ADVERTISING.CAMPAIGN_UNIVERSE", "ozon.performance.campaign", "current_v1", "GET /api/client/campaign", "advertising_campaign_mapping_v1", "provider campaign id is stable identity; mutable configuration is observed version state; unknown enums are retained")
var DailyContract = collection.NewSourceContract("ADVERTISING.CAMPAIGN_TRAFFIC", "ozon.performance.statistics.daily", "current_json_v2", "GET /api/client/statistics/daily/json", "advertising_daily_mapping_v2", "provider DATE is Moscow business date; spend maps to paid_spend unless bonus keys are present; provider drr retained when present; attribution is pending acceptance")
var ProductDailyContract = collection.NewSourceContract("ADVERTISING.SKU_TRAFFIC", "ozon.performance.statistics.campaign.product", "current_json_v1", "GET /api/client/statistics/campaign/product/json", "advertising_product_daily_mapping_v1", "direct provider campaign/SKU/day rows only; expense is paid spend; modelOrders stays MODEL attribution kind; no campaign allocation")
var ExpenseContract = collection.NewSourceContract("ADVERTISING.ACCOUNT_SPEND", "ozon.performance.statistics.expense", "current_json_v1", "GET /api/client/statistics/expense/json", "advertising_expense_mapping_v1", "expense reconciles paid spend against campaign/day paid spend; never summed into spend")
var TargetContract = collection.NewSourceContract("ADVERTISING.TARGET_SCOPE", "ozon.performance.campaign.products", "v2", "GET /api/client/campaign/{id}/v2/products", "advertising_target_mapping_v1", "only explicit provider membership creates intervals; all-eligible scope is not expanded")
var CPOContract = collection.NewSourceContract("ADVERTISING.CAMPAIGN_ATTRIBUTION", "ozon.performance.statistics.cpo.orders", "async_generate_v1", "GET /api/client/statistics/all_sku_promo/orders/generate", "advertising_cpo_mapping_v1", "async report; promoted and ordered SKU remain distinct; unresolved orders do not fabricate Sales orders")
