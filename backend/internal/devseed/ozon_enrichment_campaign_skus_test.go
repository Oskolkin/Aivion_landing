package devseed

import (
	"encoding/json"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestMergeDevSeedRawAttributesPreservesLegacyFields(t *testing.T) {
	t.Parallel()
	existing, _ := json.Marshal(map[string]any{
		"seeded_by": "dev-seed-mvp",
		"strategy":  "MAX_CLICKS",
	})
	raw := mergeDevSeedRawAttributes(existing, OzonEnrichmentScenarioRich, map[string]any{
		"campaign_external_id": int64(123),
	})
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["seeded_by"] != "dev-seed-mvp" {
		t.Fatalf("seeded_by=%v", payload["seeded_by"])
	}
	if payload["source"] != DevOzonEnrichmentSeedSource {
		t.Fatalf("source=%v", payload["source"])
	}
	if payload["scenario"] != string(OzonEnrichmentScenarioRich) {
		t.Fatalf("scenario=%v", payload["scenario"])
	}
}

func TestBidAmountForSKUInt64Ranges(t *testing.T) {
	t.Parallel()
	var hasNormal, hasHigh bool
	for sku := int64(1); sku < 500; sku++ {
		bid, ok := bidAmountForSKUInt64(sku, campaignKindManualRunning)
		if !ok || bid < 10 || bid > 60 {
			t.Fatalf("sku=%d bid=%v ok=%v", sku, bid, ok)
		}
		if bid >= 10 && bid <= 25 {
			hasNormal = true
		}
		if bid >= 40 && bid <= 60 {
			hasHigh = true
		}
	}
	if !hasNormal || !hasHigh {
		t.Fatalf("expected normal and high bids, normal=%v high=%v", hasNormal, hasHigh)
	}
	_, ok := bidAmountForSKUInt64(12345, campaignKindAutoRunning)
	if ok {
		t.Fatal("auto campaign should not have bid")
	}
}

func TestResolveCampaignSKUProductTitleFallback(t *testing.T) {
	t.Parallel()
	title := resolveCampaignSKUProductTitle(
		dbgen.ListAdCampaignSKUMappingsBySellerAccountIDRow{
			Sku: pgtype.Int8{Int64: 999, Valid: true},
		},
		buildProductTitleLookups(nil),
	)
	if title != "SKU 999" {
		t.Fatalf("title=%q", title)
	}
}
