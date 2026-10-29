package aidatapack

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

const (
	limitationCodeOzonEnrichmentUnavailable = "ozon_enrichment_unavailable"
	limitationSeverityHigh                  = "high"
)

func buildDataQuality(ctx *loadContext, selectedSKUCount, totalSKUCount, stockMultiRowSKUCount int) DataQuality {
	w := ctx.Windows
	sources := map[string]any{
		"seller": map[string]any{"status": "available"},
	}

	if len(ctx.APICapabilities) > 0 {
		cap := map[string]any{
			"status":        "available",
			"methods_count": countOzonAPICapabilityMethods(ctx.APICapabilities),
		}
		if ctx.EnrichmentSource != "" {
			cap["source"] = ctx.EnrichmentSource
		}
		sources["ozon_api_capabilities"] = cap
	} else {
		sources["ozon_api_capabilities"] = map[string]any{"status": "not_available"}
	}

	if len(ctx.AccountMetrics) > 0 {
		sources["daily_account_metrics"] = map[string]any{
			"status":    "available",
			"rows":      len(ctx.AccountMetrics),
			"date_from": w.Last14d.From,
			"date_to":   w.Last14d.To,
		}
	} else {
		sources["daily_account_metrics"] = map[string]any{"status": "not_available"}
	}

	if len(ctx.SKUMetrics) > 0 {
		sources["daily_sku_metrics"] = map[string]any{
			"status": "available",
			"rows":   len(ctx.SKUMetrics),
		}
	} else {
		sources["daily_sku_metrics"] = map[string]any{"status": "not_available"}
	}

	if enrichmentRunFailed(ctx.EnrichmentRun) {
		src := map[string]any{
			"status":        "unavailable",
			"reason":        limitationCodeOzonEnrichmentUnavailable,
			"snapshot_date": w.CurrentDay,
		}
		if ctx.EnrichmentSource != "" {
			src["source"] = ctx.EnrichmentSource
		}
		sources["product_enrichment"] = src
	} else if len(ctx.ProductInfoSnapshots) > 0 {
		src := map[string]any{
			"status":        "available",
			"rows":          len(ctx.ProductInfoSnapshots),
			"snapshot_date": w.CurrentDay,
		}
		if ctx.EnrichmentSource != "" {
			src["source"] = ctx.EnrichmentSource
		}
		sources["product_enrichment"] = src
	} else {
		sources["product_enrichment"] = map[string]any{"status": "not_available"}
	}

	if enrichmentRunFailed(ctx.EnrichmentRun) {
		src := map[string]any{
			"status":        "unavailable",
			"reason":        limitationCodeOzonEnrichmentUnavailable,
			"snapshot_date": w.CurrentDay,
		}
		if ctx.EnrichmentSource != "" {
			src["source"] = ctx.EnrichmentSource
		}
		sources["stock_analytics"] = src
	} else if len(ctx.StockSnapshots) > 0 {
		src := map[string]any{
			"status":        "available",
			"rows":          len(ctx.StockSnapshots),
			"snapshot_date": w.CurrentDay,
		}
		if ctx.EnrichmentSource != "" {
			src["source"] = ctx.EnrichmentSource
		}
		sources["stock_analytics"] = src
	} else {
		sources["stock_analytics"] = map[string]any{"status": "not_available"}
	}

	if len(ctx.Campaigns) > 0 {
		sources["advertising"] = map[string]any{
			"status":                 "available",
			"campaign_count":         len(ctx.Campaigns),
			"daily_metric_rows":      len(ctx.AdMetrics),
			"campaign_product_links": len(ctx.CampaignSKUs),
		}
	} else {
		sources["advertising"] = map[string]any{"status": "not_available"}
	}

	if len(ctx.FunnelRows) > 0 {
		sources["funnel"] = map[string]any{"status": "available"}
	} else {
		sources["funnel"] = map[string]any{"status": "not_available"}
	}

	sources["finance"] = map[string]any{
		"status": "not_available",
		"reason": "finance ingestion not implemented",
	}

	limitations := []string{
		"Advertising metrics are campaign-level only; SKU-level ad spend attribution is unavailable.",
		"Finance data is unavailable; true profit, payout and margin cannot be calculated.",
		"daily_sku_metrics.stock_available is treated as a current snapshot, not historical stock movement.",
	}
	if stockMultiRowSKUCount > 0 {
		limitations = append(limitations,
			fmt.Sprintf("stock_analytics_snapshots contains multiple rows per SKU for %d SKUs; turnover_grade is aggregated using most_critical_across_rows when grades differ", stockMultiRowSKUCount),
		)
	}

	structured := buildEnrichmentStructuredLimitations(ctx)
	for _, lim := range structured {
		limitations = append(limitations, lim.Message)
	}

	compaction := map[string]any{
		"sku_total":        totalSKUCount,
		"sku_sent":         selectedSKUCount,
		"campaign_total":   len(ctx.Campaigns),
		"campaign_sent":    len(ctx.Campaigns),
		"policy":           "full normalized dataset is split into deterministic technical chunks",
		"selection_policy": "technical_chunking_only_no_business_filtering",
	}
	if totalSKUCount > MaxSKUsTechnicalLimit {
		compaction["technical_limit"] = MaxSKUsTechnicalLimit
		compaction["status"] = "too_large_for_single_pack"
	}

	return DataQuality{
		Sources:               sources,
		Limitations:           limitations,
		StructuredLimitations: structured,
		ContextCompaction:     compaction,
	}
}

func enrichmentRunFailed(run *dbgen.OzonEnrichmentSyncRun) bool {
	return run != nil && run.Status == "failed"
}

func buildEnrichmentStructuredLimitations(ctx *loadContext) []DataQualityLimitation {
	if !enrichmentRunFailed(ctx.EnrichmentRun) {
		return nil
	}
	run := ctx.EnrichmentRun
	return []DataQualityLimitation{{
		Code:     limitationCodeOzonEnrichmentUnavailable,
		Scope:    enrichmentFailureScope(ctx, run),
		Message:  enrichmentFailureMessage(run),
		Severity: limitationSeverityHigh,
	}}
}

func enrichmentFailureScope(ctx *loadContext, run *dbgen.OzonEnrichmentSyncRun) []string {
	scopeSet := map[string]struct{}{
		"product_enrichment": {},
		"stock_analytics":    {},
	}
	if run != nil && !run.PerformanceApiAvailable {
		scopeSet["advertising"] = struct{}{}
	}
	if len(ctx.APICapabilities) == 0 {
		scopeSet["ozon_api_capabilities"] = struct{}{}
	}
	scope := make([]string, 0, len(scopeSet))
	for item := range scopeSet {
		scope = append(scope, item)
	}
	sort.Strings(scope)
	return scope
}

func enrichmentFailureMessage(run *dbgen.OzonEnrichmentSyncRun) string {
	if run == nil {
		return "Ozon P0 enrichment failed; enrichment data is unavailable for this metric date."
	}
	if len(run.ErrorsJson) > 0 {
		var errors []struct {
			Source  string `json:"source"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(run.ErrorsJson, &errors); err == nil && len(errors) > 0 {
			parts := make([]string, 0, len(errors))
			for _, e := range errors {
				parts = append(parts, fmt.Sprintf("%s: %s", e.Source, e.Message))
			}
			return fmt.Sprintf("Ozon P0 enrichment failed: %s", strings.Join(parts, "; "))
		}
	}
	return "Ozon P0 enrichment failed; product and stock enrichment data is unavailable for this metric date."
}

func countOzonAPICapabilityMethods(caps []dbgen.OzonApiCapability) int {
	total := 0
	for _, c := range caps {
		if len(c.MethodsJson) == 0 {
			continue
		}
		var methods []any
		if err := json.Unmarshal(c.MethodsJson, &methods); err == nil {
			total += len(methods)
		}
	}
	return total
}

func detectEnrichmentSource(run *dbgen.OzonEnrichmentSyncRun) string {
	return analysisdata.DetectEnrichmentSource(run)
}
