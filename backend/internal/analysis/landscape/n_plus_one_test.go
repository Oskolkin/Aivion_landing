package landscape_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLandscapeUsesBatchFactualMethods is the package-level N+1 gate companion to
// factual/query_shape_test.go (set-based ANY($2) SQL). Landscape Core must call
// batch APIs, not document "UNAVAILABLE because batch missing".
func TestLandscapeUsesBatchFactualMethods(t *testing.T) {
	src := readLandscapeSources(t, "factual_source.go", "factual_source_ads_v4.go")
	for _, want := range []string{
		"BatchGetSKUAdvertisingTraffic",
		"BatchGetSKUAdvertisingAttribution",
		"BatchGetSKUInbound",
		"BatchGetStockCoverage",
		"BatchGetCampaignTrafficPerformance",
		"BatchGetCampaignAttributionPerformance",
		"BatchGetSKUDemand",
		"BatchGetSKUStock",
		"BatchGetSKUPromotionSummary",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("landscape core source must use %s", want)
		}
	}
	for _, banned := range []string{
		"BATCH_SKU_ADVERTISING_MISSING",
		"BATCH_SKU_INBOUND_MISSING",
		"CAMPAIGN_PERIOD_ROLLUP_N_PLUS_ONE",
		"LANDSCAPE_FACTUAL_PARTIAL",
		"BatchGetCampaignPeriodPerformance",
		"BatchGetSKUAdvertising(",
	} {
		if strings.Contains(src, banned) {
			t.Fatalf("production gap marker or mixed V3 ad method %s must be removed", banned)
		}
	}
}

func readLandscapeSources(t *testing.T, names ...string) string {
	t.Helper()
	var b strings.Builder
	for _, name := range names {
		body, err := os.ReadFile(filepath.Join(name))
		if err != nil {
			t.Fatal(err)
		}
		b.Write(body)
		b.WriteByte('\n')
	}
	return b.String()
}
