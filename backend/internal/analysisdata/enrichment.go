package analysisdata

import (
	"encoding/json"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

// DetectEnrichmentSource derives the product enrichment provenance label.
//
// Behaviour matches the legacy aidatapack helper:
//  1. nil run → ""
//  2. scope == "p0_dev_seed" → "dev_seed"
//  3. string "source" in raw_summary_json → that value
//  4. otherwise → ""
func DetectEnrichmentSource(run *dbgen.OzonEnrichmentSyncRun) string {
	if run == nil {
		return ""
	}
	if run.Scope == "p0_dev_seed" {
		return "dev_seed"
	}
	if len(run.RawSummaryJson) > 0 {
		var raw map[string]any
		if err := json.Unmarshal(run.RawSummaryJson, &raw); err == nil {
			if src, ok := raw["source"].(string); ok && src != "" {
				return src
			}
		}
	}
	return ""
}
