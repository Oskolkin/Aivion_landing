package aipipeline

import (
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
)

// DashboardRunOutcome carries dashboard generation result plus optional debug payloads.
type DashboardRunOutcome struct {
	Result      *dashboardhome.NarrativeGenerationResult
	DebugInput  any
	DebugOutput any
}

func buildDashboardArtifactInput(
	sellerAccountID int64,
	metricDate time.Time,
	opts dashboardhome.NarrativeRunOptions,
	mode string,
) map[string]any {
	input := map[string]any{
		"seller_account_id": sellerAccountID,
		"metric_date":       metricDate.Format("2006-01-02"),
		"mode":              mode,
	}
	if opts.DataPackHash != "" {
		input["data_pack_hash"] = opts.DataPackHash
	}
	if opts.GlobalContextHash != "" {
		input["global_context_hash"] = opts.GlobalContextHash
	}
	if opts.GlobalContext != nil {
		input["global_semantic_context"] = opts.GlobalContext
	}
	if opts.DataPackOverride != nil {
		input["data_pack_summary"] = map[string]any{
			"schema_version":    opts.DataPackOverride.SchemaVersion,
			"seller_account_id": opts.DataPackOverride.SellerContext.SellerAccountID,
			"metric_date":       opts.DataPackOverride.SellerContext.MetricDate,
			"fact_count":        len(opts.DataPackOverride.FactRegistry),
		}
	}
	return input
}
