// Package analytics contains backend analytics layer services.
//
// Layer composition (MVP):
//   - AccountMetricsService: rebuilds daily_account_metrics.
//   - SKUMetricsService: rebuilds daily_sku_metrics.
//   - DashboardService: builds derived dashboard DTO from daily metrics.
//   - StocksViewService: builds current stocks table view.
//
// This package intentionally stays separated from ingestion layer:
// no sync orchestration, no runtime coupling, no background schedulers.
// Rebuild and checks are triggered manually via dev commands/entrypoints.
package analytics
