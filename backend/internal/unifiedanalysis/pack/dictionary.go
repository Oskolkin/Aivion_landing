package pack

import "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"

// BuildDataDictionary renders data_dictionary.json covering every column that
// appears in any Analysis Pack v1 CSV file (contract.AnalysisPackCSVColumnsV1).
func BuildDataDictionary() contract.DataDictionaryV1 {
	return contract.DataDictionaryV1{
		SchemaVersion: contract.AnalysisPackSchemaVersionV1,
		Fields:        dataDictionaryFields,
	}
}

var dataDictionaryFields = map[string]contract.DataDictionaryFieldV1{
	"row_id":            {Type: "string", Description: "Stable deterministic identifier for this row within its table (not a database primary key).", Nullable: false},
	"seller_account_id": {Type: "integer", Unit: "id", Description: "Aivion seller account identifier.", Nullable: false},
	"metric_date":       {Type: "string", Unit: "date", Description: "Calendar date (YYYY-MM-DD, Europe/Moscow) this row/pack was generated for.", Nullable: false},

	"orders_7d":          {Type: "integer", Unit: "count", Description: "Orders in the current 7-day window inclusive of metric_date.", Period: "current_7d", Nullable: false, Source: "daily_account_metrics / daily_sku_metrics"},
	"orders_previous_7d": {Type: "integer", Unit: "count", Description: "Orders in the previous (non-overlapping) 7-day window.", Period: "previous_7d", Nullable: false, Source: "daily_account_metrics / daily_sku_metrics"},
	"orders_delta_abs":   {Type: "number", Unit: "count", Description: "orders_7d minus orders_previous_7d.", Formula: "orders_7d - orders_previous_7d", Nullable: false},
	"orders_delta_pct":   {Type: "number", Unit: "percent", Description: "Percent change of orders_7d vs orders_previous_7d.", Formula: "(orders_7d - orders_previous_7d) / orders_previous_7d * 100", Nullable: true, NullSemantics: MetricNullSemanticsDeltaPct},

	"revenue_7d":          {Type: "number", Unit: "RUB", Description: "Total store (cabinet) revenue in the current 7-day window. Use this as total_store_revenue — never confuse with ad_revenue_7d.", Period: "current_7d", Nullable: false, Source: "daily_account_metrics / daily_sku_metrics"},
	"revenue_previous_7d": {Type: "number", Unit: "RUB", Description: "Total store (cabinet) revenue in the previous (non-overlapping) 7-day window.", Period: "previous_7d", Nullable: false, Source: "daily_account_metrics / daily_sku_metrics"},
	"revenue_delta_abs":   {Type: "number", Unit: "RUB", Description: "revenue_7d minus revenue_previous_7d.", Formula: "revenue_7d - revenue_previous_7d", Nullable: false},
	"revenue_delta_pct":   {Type: "number", Unit: "percent", Description: "Percent change of revenue_7d vs revenue_previous_7d.", Formula: "(revenue_7d - revenue_previous_7d) / revenue_previous_7d * 100", Nullable: true, NullSemantics: MetricNullSemanticsDeltaPct},
	"revenue_share_pct":   {Type: "number", Unit: "percent", Description: "This SKU's share of total revenue_7d across all known SKUs.", Formula: "sku.revenue_7d / sum(all_skus.revenue_7d) * 100", Nullable: true, NullSemantics: "Null when total revenue_7d across all SKUs is 0."},

	"delivered_7d": {Type: "integer", Unit: "count", Description: "Orders in the current 7-day window minus cancelled_7d and returns_7d, floored at 0. Account-level only.", Formula: "max(orders_7d - cancelled_7d - returns_7d, 0)", Nullable: true, NullSemantics: LimitationDescription(LimitationSKUOrderQualityUnavailable), Source: "daily_account_metrics"},
	"cancelled_7d": {Type: "integer", Unit: "count", Description: "Cancelled order postings in the current 7-day window. Account-level only.", Period: "current_7d", Nullable: true, NullSemantics: LimitationDescription(LimitationSKUOrderQualityUnavailable), Source: "daily_account_metrics"},
	"returns_7d":   {Type: "integer", Unit: "count", Description: "Returned order postings in the current 7-day window. Account-level only.", Period: "current_7d", Nullable: true, NullSemantics: LimitationDescription(LimitationSKUOrderQualityUnavailable), Source: "daily_account_metrics"},

	"average_order_value_7d": {Type: "number", Unit: "RUB", Description: "Average order value over the current 7-day window.", Formula: "revenue_7d / orders_7d", Nullable: true, NullSemantics: "Null when orders_7d is 0.", Source: "daily_account_metrics"},

	"impressions_7d":   {Type: "integer", Unit: "count", Description: "Account-level funnel impressions in the current 7-day window.", Period: "current_7d", Nullable: true, NullSemantics: LimitationDescription(LimitationSKUFunnelUnavailable), Source: "product_analytics_daily"},
	"product_views_7d": {Type: "integer", Unit: "count", Description: "Account-level funnel product views in the current 7-day window.", Period: "current_7d", Nullable: false, Source: "product_analytics_daily"},
	"add_to_cart_7d":   {Type: "integer", Unit: "count", Description: "Account-level funnel add-to-cart events in the current 7-day window.", Period: "current_7d", Nullable: false, Source: "product_analytics_daily"},
	"funnel_orders_7d": {Type: "integer", Unit: "count", Description: "Orders recorded by the funnel table (may differ slightly from orders_7d, which uses daily_account_metrics).", Period: "current_7d", Nullable: false, Source: "product_analytics_daily"},

	"impression_to_view_pct": {Type: "number", Unit: "percent", Description: "Conversion from impressions to product views.", Formula: "product_views_7d / impressions_7d * 100", Nullable: true, NullSemantics: MetricNullSemanticsConversionPct},
	"view_to_cart_pct":       {Type: "number", Unit: "percent", Description: "Conversion from product views to add-to-cart.", Formula: "add_to_cart_7d / product_views_7d * 100", Nullable: true, NullSemantics: MetricNullSemanticsConversionPct},
	"cart_to_order_pct":      {Type: "number", Unit: "percent", Description: "Conversion from add-to-cart to funnel orders.", Formula: "funnel_orders_7d / add_to_cart_7d * 100", Nullable: true, NullSemantics: MetricNullSemanticsConversionPct},
	"order_to_delivered_pct": {Type: "number", Unit: "percent", Description: "Conversion from funnel orders to funnel completed orders.", Formula: "funnel_completed_orders_7d / funnel_orders_7d * 100", Nullable: true, NullSemantics: MetricNullSemanticsConversionPct},

	"active_sku_count":          {Type: "integer", Unit: "count", Description: "Count of SKUs with revenue_7d>0 or orders_7d>0 (same semantics as alerts.sku_importance).", Nullable: false},
	"stockout_sku_count":        {Type: "integer", Unit: "count", Description: "Count of known SKUs with available_stock_units == 0.", Nullable: false},
	"deficit_sku_count":         {Type: "integer", Unit: "count", Description: "Count of analysis-eligible SKUs whose canonical stock_status is critical_shortage or out_of_stock.", Nullable: false},
	"surplus_sku_count":         {Type: "integer", Unit: "count", Description: "Count of analysis-eligible SKUs whose canonical stock_status is surplus_120d.", Nullable: false},
	"no_sales_with_stock_count": {Type: "integer", Unit: "count", Description: "Count of SKUs with available stock but zero orders in the current 7-day window.", Nullable: false},

	"ad_spend_7d":                              {Type: "number", Unit: "RUB", Description: "Total advertising spend (total_ad_spend) in the current 7-day window across ALL campaigns (including no-order and high-DRR). Account scope only; sku: always null.", Period: "current_7d", Nullable: true, NullSemantics: LimitationDescription(LimitationSKUAdSpendNotAttributed), Source: "ad_metrics_daily"},
	"ad_revenue_7d":                            {Type: "number", Unit: "RUB", Description: "Attributed ad revenue (attributed_ad_revenue) in the current 7-day window: sum of campaign-attributed revenue across ALL campaigns. NOT total store revenue.", Period: "current_7d", Nullable: true, NullSemantics: LimitationDescription(LimitationSKUAdSpendNotAttributed), Source: "ad_metrics_daily"},
	"ad_orders_7d":                             {Type: "integer", Unit: "count", Description: "Advertising-attributed orders in the current 7-day window (account: sum across all campaigns; sku: not attributed, always null).", Period: "current_7d", Nullable: true, NullSemantics: LimitationDescription(LimitationSKUAdSpendNotAttributed), Source: "ad_metrics_daily"},
	"account_drr_pct":                          {Type: "number", Unit: "percent", Description: "ДРР в общей выручке (Seller Pulse): all campaign ad spend / total store (cabinet) revenue * 100. Same value as ad_spend_share_of_total_revenue_pct.", Formula: "ad_spend_7d / revenue_7d * 100", Nullable: true, NullSemantics: contract.MetricNullSemanticsV1["drr_pct"]},
	"ad_spend_share_of_total_revenue_pct":      {Type: "number", Unit: "percent", Description: "ДРР в общей выручке: все рекламные расходы / вся выручка кабинета × 100. Used by Seller Pulse.", Formula: "total_ad_spend / total_store_revenue * 100 = ad_spend_7d / revenue_7d * 100", Nullable: true, NullSemantics: contract.MetricNullSemanticsV1["drr_pct"]},
	"ad_spend_share_of_attributed_revenue_pct": {Type: "number", Unit: "percent", Description: "ДРР в выручке с рекламных кампаний: все рекламные расходы / выручка, атрибутированная рекламе × 100. Used by Advertising efficiency block. Ratio of sums — never mean of campaign or daily percentages.", Formula: "total_ad_spend / attributed_ad_revenue * 100 = ad_spend_7d / ad_revenue_7d * 100", Nullable: true, NullSemantics: contract.MetricNullSemanticsV1["drr_pct"]},
	"drr_pct":                                  {Type: "number", Unit: "percent", Description: "Campaign-level ad spend / attributed campaign revenue * 100 (campaign scope) or always-null placeholder (sku/campaign_sku scope).", Formula: "spend / revenue * 100 when revenue > 0", Nullable: true, NullSemantics: contract.MetricNullSemanticsV1["drr_pct"]},

	"orders":        {Type: "integer", Unit: "count", Description: "Orders on this calendar day.", Nullable: false, Source: "daily_account_metrics / daily_sku_metrics"},
	"revenue":       {Type: "number", Unit: "RUB", Description: "Total store (cabinet) revenue on this calendar day (total_store_revenue). Do not confuse with ad_revenue.", Nullable: false, Source: "daily_account_metrics / daily_sku_metrics"},
	"delivered":     {Type: "integer", Unit: "count", Description: "orders minus cancelled and returns for this day, floored at 0. Account-level only.", Formula: "max(orders - cancelled - returns, 0)", Nullable: true, NullSemantics: LimitationDescription(LimitationSKUOrderQualityUnavailable), Source: "daily_account_metrics"},
	"cancelled":     {Type: "integer", Unit: "count", Description: "Cancelled order postings on this day. Account-level only.", Nullable: true, NullSemantics: LimitationDescription(LimitationSKUOrderQualityUnavailable), Source: "daily_account_metrics"},
	"returns":       {Type: "integer", Unit: "count", Description: "Returned order postings on this day. Account-level only.", Nullable: true, NullSemantics: LimitationDescription(LimitationSKUOrderQualityUnavailable), Source: "daily_account_metrics"},
	"impressions":   {Type: "integer", Unit: "count", Description: "Account-level funnel impressions on this day.", Nullable: false, Source: "product_analytics_daily"},
	"product_views": {Type: "integer", Unit: "count", Description: "Account-level funnel product views on this day.", Nullable: false, Source: "product_analytics_daily"},
	"add_to_cart":   {Type: "integer", Unit: "count", Description: "Account-level funnel add-to-cart events on this day.", Nullable: false, Source: "product_analytics_daily"},
	"ad_spend":      {Type: "number", Unit: "RUB", Description: "Total advertising spend (total_ad_spend) on this calendar day across ALL campaigns.", Nullable: true, NullSemantics: LimitationDescription(LimitationSKUAdSpendNotAttributed), Source: "ad_metrics_daily"},
	"ad_revenue":    {Type: "number", Unit: "RUB", Description: "Attributed ad revenue (attributed_ad_revenue) on this calendar day — NOT total store revenue.", Nullable: true, NullSemantics: LimitationDescription(LimitationSKUAdSpendNotAttributed), Source: "ad_metrics_daily"},
	"ad_orders":     {Type: "integer", Unit: "count", Description: "Advertising-attributed orders on this calendar day.", Nullable: true, NullSemantics: LimitationDescription(LimitationSKUAdSpendNotAttributed), Source: "ad_metrics_daily"},

	"sku_id":        {Type: "string", Unit: "id", Description: "Ozon SKU identifier.", Nullable: false},
	"offer_id":      {Type: "string", Description: "Seller-defined offer identifier for this SKU.", Nullable: true, NullSemantics: "Null when no source row carried an offer_id for this SKU."},
	"product_id":    {Type: "string", Unit: "id", Description: "Ozon product identifier this SKU belongs to.", Nullable: true, NullSemantics: "Null when no source row carried a product id for this SKU."},
	"title":         {Type: "string", Description: "Product title.", Nullable: false},
	"category_id":   {Type: "string", Unit: "id", Description: "Category entity reference (category:<description_category_id>).", Nullable: true, NullSemantics: "Null when no source row carried a category for this SKU."},
	"category_name": {Type: "string", Description: "Human-readable category name.", Nullable: true, NullSemantics: "Null when category_id is null or the category lookup found no name."},

	"views_7d": {Type: "integer", Unit: "count", Description: "Per-SKU funnel product views in the current 7-day window. Not tracked at SKU granularity; always null.", Nullable: true, NullSemantics: LimitationDescription(LimitationSKUFunnelUnavailable)},
	"views":    {Type: "integer", Unit: "count", Description: "Per-SKU funnel product views on this day. Not tracked at SKU granularity; always null.", Nullable: true, NullSemantics: LimitationDescription(LimitationSKUFunnelUnavailable)},

	"available_stock_units": {Type: "integer", Unit: "count", Description: "Currently available stock units for this SKU (stock_analytics_snapshots aggregate, falling back to daily_sku_metrics.stock_available).", Nullable: true, NullSemantics: "Null when neither source has a stock row for this SKU.", Source: "stock_analytics_snapshots / daily_sku_metrics"},
	"reserved_stock_units":  {Type: "integer", Unit: "count", Description: "Reserved stock units. Not tracked as a distinct field in the current schema; always null.", Nullable: true, NullSemantics: LimitationDescription(LimitationSKUReservedStockUnknown)},
	"incoming_stock_units":  {Type: "integer", Unit: "count", Description: "In-transit (incoming) stock units.", Nullable: true, NullSemantics: "Null when no stock_analytics_snapshots row exists for this SKU.", Source: "stock_analytics_snapshots"},
	"available_stock":       {Type: "integer", Unit: "count", Description: "Available stock units for this SKU on this calendar day (daily_sku_metrics.stock_available).", Nullable: false, Source: "daily_sku_metrics"},
	"reserved_stock":        {Type: "integer", Unit: "count", Description: "Reserved stock units for this SKU on this day. Not tracked at daily granularity; always null.", Nullable: true, NullSemantics: LimitationDescription(LimitationSKUReservedStockUnknown)},
	"incoming_stock":        {Type: "integer", Unit: "count", Description: "In-transit stock units for this SKU on this day. Not tracked at daily granularity; always null.", Nullable: true, NullSemantics: LimitationDescription(LimitationSKUReservedStockUnknown)},

	"average_daily_sales_7d":  {Type: "number", Unit: "units/day", Description: "Average daily order velocity over the current 7-day window.", Formula: "orders_7d / 7", Nullable: true, NullSemantics: contract.MetricNullSemanticsV1["average_daily_sales"]},
	"average_daily_sales_30d": {Type: "number", Unit: "units/day", Description: "Average daily order velocity over the 30-day trend window.", Formula: "orders_30d / 30", Nullable: true, NullSemantics: contract.MetricNullSemanticsV1["average_daily_sales_30d"]},
	"stock_coverage_days_7d":  {Type: "number", Unit: "days", Description: "Stock coverage using the 7-day average daily sales.", Formula: "available_stock_units / average_daily_sales_7d", Nullable: true, NullSemantics: contract.MetricNullSemanticsV1["stock_coverage_days"]},
	"stock_coverage_days_30d": {Type: "number", Unit: "days", Description: "Stock coverage using the 30-day average daily sales.", Formula: "available_stock_units / average_daily_sales_30d", Nullable: true, NullSemantics: contract.MetricNullSemanticsV1["stock_coverage_days_30d"]},
	"stock_status":            {Type: "string", Unit: "enum", Description: "Canonical Aivion stock health bucket derived from available stock, coverage days, and days_without_sales (out_of_stock, critical_shortage, shortage_7d, planning_14d, healthy, surplus_120d, no_sales, unknown, collecting_data). Raw Ozon turnover_grade is not used as the final status.", Nullable: true, NullSemantics: "Null when no stock signal (snapshot or daily metric) exists for this SKU."},
	"days_without_sales":      {Type: "integer", Unit: "days", Description: "Days without any recorded order, from stock_analytics_snapshots.days_without_sales when present, else computed by scanning back through the 30-day trend window.", Nullable: true, NullSemantics: "Null when no stock or sales history exists for this SKU."},

	"price":                    {Type: "number", Unit: "RUB", Description: "Current selling price.", Nullable: true, NullSemantics: "Null when no product info snapshot or product row carries a price for this SKU.", Source: "ozon_product_info_snapshots / products"},
	"old_price":                {Type: "number", Unit: "RUB", Description: "Pre-discount reference price.", Nullable: true, NullSemantics: "Null when no product info snapshot or product row carries an old_price for this SKU.", Source: "ozon_product_info_snapshots / products"},
	"market_price_index":       {Type: "number", Unit: "ratio", Description: "Ozon/external market price index value (lower is more competitive).", Nullable: true, NullSemantics: contract.MetricNullSemanticsV1["market_price_index"], Source: "ozon_product_info_snapshots"},
	"enrichment_status":        {Type: "string", Unit: "enum", Description: "Whether Ozon product enrichment data is available for this SKU (available | not_available).", Nullable: false},
	"content_score":            {Type: "number", Unit: "ratio", Description: "Content quality score. Not computed by any upstream source; always null.", Nullable: true, NullSemantics: LimitationDescription(LimitationContentScoreUnavailable)},
	"has_content_warnings":     {Type: "boolean", Description: "Whether the product enrichment snapshot recorded any error/warning entries for this SKU.", Nullable: true, NullSemantics: "Null when no product info snapshot exists for this SKU."},
	"image_count":              {Type: "integer", Unit: "count", Description: "Number of product images recorded in the enrichment snapshot.", Nullable: true, NullSemantics: "Null when no product info snapshot exists for this SKU."},
	"attribute_completion_pct": {Type: "number", Unit: "percent", Description: "Percentage of catalog attributes filled in. Not computed by any upstream source; always null.", Nullable: true, NullSemantics: LimitationDescription(LimitationAttributeCompletionUnknown)},

	"campaign_id":     {Type: "string", Unit: "id", Description: "Ozon campaign external identifier.", Nullable: false},
	"campaign_name":   {Type: "string", Description: "Campaign display name.", Nullable: true, NullSemantics: "Null when the campaign row itself has no name recorded."},
	"campaign_type":   {Type: "string", Unit: "enum", Description: "Campaign advertising object type (e.g. SKU).", Nullable: true, NullSemantics: "Null when not recorded on the campaign row."},
	"campaign_status": {Type: "string", Unit: "enum", Description: "Campaign lifecycle status (e.g. CAMPAIGN_STATE_RUNNING).", Nullable: true, NullSemantics: "Null when not recorded on the campaign row."},

	"spend_7d":          {Type: "number", Unit: "RUB", Description: "Campaign ad spend in the current 7-day window.", Period: "current_7d", Nullable: false, Source: "ad_metrics_daily"},
	"spend_previous_7d": {Type: "number", Unit: "RUB", Description: "Campaign ad spend in the previous 7-day window.", Period: "previous_7d", Nullable: false, Source: "ad_metrics_daily"},
	"spend_delta_abs":   {Type: "number", Unit: "RUB", Description: "spend_7d minus spend_previous_7d.", Formula: "spend_7d - spend_previous_7d", Nullable: false},
	"spend_delta_pct":   {Type: "number", Unit: "percent", Description: "Percent change of spend_7d vs spend_previous_7d.", Nullable: true, NullSemantics: MetricNullSemanticsDeltaPct},
	"spend":             {Type: "number", Unit: "RUB", Description: "Campaign ad spend on this calendar day.", Nullable: false, Source: "ad_metrics_daily"},

	"clicks_7d": {Type: "integer", Unit: "count", Description: "Campaign ad clicks in the current 7-day window.", Period: "current_7d", Nullable: false, Source: "ad_metrics_daily"},
	"clicks":    {Type: "integer", Unit: "count", Description: "Campaign ad clicks on this calendar day.", Nullable: false, Source: "ad_metrics_daily"},
	"ctr_pct":   {Type: "number", Unit: "percent", Description: "Click-through rate: clicks / impressions * 100.", Formula: "clicks / impressions * 100", Nullable: true, NullSemantics: contract.MetricNullSemanticsV1["ctr_pct"]},
	"cpc":       {Type: "number", Unit: "RUB", Description: "Cost per click: spend / clicks.", Formula: "spend / clicks", Nullable: true, NullSemantics: "Null when clicks is 0."},

	"conversion_pct":      {Type: "number", Unit: "percent", Description: "Click-to-order conversion rate: orders / clicks * 100.", Formula: "orders / clicks * 100", Nullable: true, NullSemantics: contract.MetricNullSemanticsV1["conversion_pct"]},
	"connected_sku_count": {Type: "integer", Unit: "count", Description: "Count of distinct SKUs linked to this campaign via ad_campaign_skus.", Nullable: false},

	"validation_status":          {Type: "string", Unit: "enum", Description: "Ozon product validation status from the enrichment snapshot.", Nullable: true, NullSemantics: LimitationDescription(LimitationEnrichmentUnavailable)},
	"price_index":                {Type: "string", Unit: "enum", Description: "Ozon price index color (e.g. green/yellow/red) from the enrichment snapshot.", Nullable: true, NullSemantics: LimitationDescription(LimitationEnrichmentUnavailable)},
	"has_stock":                  {Type: "boolean", Description: "Whether Ozon reports visibility.has_stock=true for this SKU.", Nullable: true, NullSemantics: LimitationDescription(LimitationEnrichmentUnavailable)},
	"title_quality_status":       {Type: "string", Unit: "enum", Description: "Title content-quality classification. Not computed by any upstream source; always null.", Nullable: true, NullSemantics: LimitationDescription(LimitationContentScoreUnavailable)},
	"description_quality_status": {Type: "string", Unit: "enum", Description: "Description content-quality classification. Not computed by any upstream source; always null.", Nullable: true, NullSemantics: LimitationDescription(LimitationContentScoreUnavailable)},
	"warning_codes":              {Type: "string", Description: "Pipe-separated (|) list of warning/error codes recorded in the enrichment snapshot.", Nullable: true, NullSemantics: "Null/empty when no warnings were recorded or no snapshot exists."},
}

// Shared null-semantics phrasing reused by multiple dictionary entries.
const (
	MetricNullSemanticsDeltaPct      = "Null when the previous-period denominator is 0 and the current value is > 0 (undefined direction); 0 when both periods are 0."
	MetricNullSemanticsConversionPct = "Null when the from-stage denominator is 0."
)

// LimitationDescription returns the human-readable description for a frozen limitation code,
// for reuse in data dictionary null_semantics text.
func LimitationDescription(code string) string {
	if def, ok := limitationDefaults[code]; ok {
		return def.Description
	}
	return ""
}
