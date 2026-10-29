// Package alerts contains the domain foundation for Stage 7 Alerts Engine.
//
// The package is rule-based and evidence-based:
// future rules produce normalized RuleResult values, which are persisted as
// idempotent alerts via fingerprint and accompanied by structured evidence.
//
// Key/top SKU for stock_critical_sku_low_stock and missing_pricing_constraints_for_key_sku
// is determined from daily_sku_metrics over a 7-day window (asOfDate-6 .. asOfDate)
// relative to the seller account: revenue/orders share, ranks among active SKUs
// (revenue_7d > 0 OR orders_7d > 0), top 10% capped at 30, or top 5 when active < 50.
//
// Advertising alerts are campaign-level only (ad_weak_campaign_efficiency): high DRR or
// spend without confirmed revenue/orders. SKU-level ad alert types are legacy and not generated.
//
// Price/economics alerts are disabled in the active MVP (PriceEconomicsGenerationDisabled)
// because reliable cost and margin data is not available. Do not enable without unit economics.
//
// Stock coverage: stock_low_coverage for 7 < days_of_cover <= 14 (FBO planning window);
// stock_oos_risk for 0 < days_of_cover <= 7. Zero stock alone does not create stock alerts.
//
// This package is not a Recommendation Engine and does not include API/UI/jobs.
package alerts
