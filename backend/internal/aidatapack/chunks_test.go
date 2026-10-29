package aidatapack

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func testChunkFixturePack(rowCount int) *SellerAIDataPack {
	pack := minimalValidPack()
	pack.EntityRegistry.SKUs = map[string]SKUEntity{}
	pack.EntityRegistry.SKUGroups = map[string]SKUGroupEntity{}
	pack.Tables.SKURollups = nil
	pack.Tables.StockPositions = nil
	pack.Tables.ProductEnrichment = nil
	pack.Tables.CampaignRollups = nil
	pack.Tables.CampaignProductLinks = nil
	pack.Tables.CategoryRollups = nil

	for i := 1; i <= rowCount; i++ {
		sku := int64ToString(int64(801000100000 + i))
		eid := skuEntityID(sku)
		pack.EntityRegistry.SKUs[eid] = SKUEntity{
			EntityID: eid, EntityType: "sku", SKU: sku, OfferID: "OFFER-" + sku, Name: "Product " + sku,
		}
		revFact := "sku." + sku + ".last_7d.revenue"
		pack.FactRegistry[revFact] = Fact{
			FactID: revFact, EntityType: "sku", EntityID: eid, Metric: "revenue", Window: "last_7d",
			Value: float64(i * 1000), Unit: UnitRUB, Source: FactSource{Table: "daily_sku_metrics", Field: "revenue"},
		}
		pack.Tables.SKURollups = append(pack.Tables.SKURollups, SKURollupRow{
			EntityID: eid, SKU: sku, OfferID: "OFFER-" + sku, Name: "Product " + sku,
			Last7d: map[string]any{"revenue": float64(i * 1000)}, FactIDs: []string{revFact},
		})
		pack.Tables.StockPositions = append(pack.Tables.StockPositions, StockPositionRow{
			EntityID: eid, SnapshotDate: "2026-06-25", DataStatus: "available",
			FactIDs: []string{revFact},
		})
		pack.Tables.ProductEnrichment = append(pack.Tables.ProductEnrichment, ProductEnrichmentRow{
			EntityID: eid, SnapshotDate: "2026-06-25", DataStatus: "available", Source: "dev_seed",
		})
	}
	for i := 1; i <= 3; i++ {
		extID := int64ToString(int64(882000001600 + i))
		cid := campaignEntityID(extID)
		pack.EntityRegistry.Campaigns[cid] = CampaignEntity{
			EntityID: cid, EntityType: "campaign", CampaignExternalID: extID, Name: "Campaign " + extID,
		}
		spendFact := "campaign." + extID + ".last_7d.spend"
		pack.FactRegistry[spendFact] = Fact{
			FactID: spendFact, EntityType: "campaign", EntityID: cid, Metric: "spend", Window: "last_7d",
			Value: float64(i * 500), Unit: UnitRUB, Source: FactSource{Table: "ad_metrics", Field: "spend"},
		}
		pack.Tables.CampaignRollups = append(pack.Tables.CampaignRollups, CampaignRollupRow{
			EntityID: cid, CampaignExternalID: extID, Name: "Campaign " + extID,
			Last7d: CampaignMetricsWindow{Spend: float64(i * 500)}, FactIDs: []string{spendFact},
		})
		pack.Tables.CampaignProductLinks = append(pack.Tables.CampaignProductLinks, CampaignProductLinkRow{
			CampaignEntityID: cid, SKUEntityID: skuEntityID(int64ToString(801000100001)),
			CampaignExternalID: extID, SKU: int64ToString(801000100001), IsActive: true,
		})
	}
	return pack
}

func TestBuildChunksCoversAllSKURollups(t *testing.T) {
	pack := testChunkFixturePack(100)
	set, err := BuildChunks(pack, DefaultChunkOptions())
	if err != nil {
		t.Fatal(err)
	}
	if set.Coverage.SKURollups.SourceCount != 100 || set.Coverage.SKURollups.MissingCount != 0 {
		t.Fatalf("sku_rollups coverage=%+v", set.Coverage.SKURollups)
	}
}

func TestBuildChunksCoversAllStockPositions(t *testing.T) {
	pack := testChunkFixturePack(100)
	set, err := BuildChunks(pack, DefaultChunkOptions())
	if err != nil {
		t.Fatal(err)
	}
	if set.Coverage.StockPositions.MissingCount != 0 {
		t.Fatalf("stock coverage=%+v", set.Coverage.StockPositions)
	}
}

func TestBuildChunksCoversAllProductEnrichment(t *testing.T) {
	pack := testChunkFixturePack(100)
	set, err := BuildChunks(pack, DefaultChunkOptions())
	if err != nil {
		t.Fatal(err)
	}
	if set.Coverage.ProductEnrichment.MissingCount != 0 {
		t.Fatalf("enrichment coverage=%+v", set.Coverage.ProductEnrichment)
	}
}

func TestBuildChunksCoversAllCampaignRollups(t *testing.T) {
	pack := testChunkFixturePack(10)
	set, err := BuildChunks(pack, DefaultChunkOptions())
	if err != nil {
		t.Fatal(err)
	}
	if set.Coverage.CampaignRollups.MissingCount != 0 {
		t.Fatalf("campaign coverage=%+v", set.Coverage.CampaignRollups)
	}
}

func TestBuildChunksCoversAllCampaignProductLinks(t *testing.T) {
	pack := testChunkFixturePack(10)
	set, err := BuildChunks(pack, DefaultChunkOptions())
	if err != nil {
		t.Fatal(err)
	}
	if set.Coverage.CampaignProductLinks.MissingCount != 0 {
		t.Fatalf("links coverage=%+v", set.Coverage.CampaignProductLinks)
	}
}

func TestBuildChunksNoDuplicateRows(t *testing.T) {
	pack := testChunkFixturePack(100)
	set, err := BuildChunks(pack, DefaultChunkOptions())
	if err != nil {
		t.Fatal(err)
	}
	if set.Coverage.SKURollups.DuplicateCount != 0 {
		t.Fatalf("duplicates=%d", set.Coverage.SKURollups.DuplicateCount)
	}
}

func TestBuildChunksStableOrdering(t *testing.T) {
	pack := testChunkFixturePack(100)
	a, err := BuildChunks(pack, DefaultChunkOptions())
	if err != nil {
		t.Fatal(err)
	}
	b, err := BuildChunks(pack, DefaultChunkOptions())
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Chunks) != len(b.Chunks) {
		t.Fatalf("chunk count mismatch")
	}
	for i := range a.Chunks {
		if a.Chunks[i].ChunkHash != b.Chunks[i].ChunkHash {
			t.Fatalf("chunk %s hash unstable", a.Chunks[i].ChunkID)
		}
	}
}

func TestBuildChunksNoBusinessSelectionPolicy(t *testing.T) {
	pack := testChunkFixturePack(20)
	set, err := BuildChunks(pack, DefaultChunkOptions())
	if err != nil {
		t.Fatal(err)
	}
	if set.ChunkingPolicy.SelectionPolicy != SelectionPolicyTechnical {
		t.Fatalf("policy=%s", set.ChunkingPolicy.SelectionPolicy)
	}
	if set.ChunkingPolicy.BusinessFilteringAllowed {
		t.Fatal("business filtering must be false")
	}
	for _, ch := range set.Chunks {
		if ch.SelectionPolicy != SelectionPolicyTechnical {
			t.Fatalf("chunk %s policy=%s", ch.ChunkID, ch.SelectionPolicy)
		}
	}
}

func TestLocalFactCatalogContainsReferencedFactsOnly(t *testing.T) {
	pack := testChunkFixturePack(5)
	set, err := BuildChunks(pack, DefaultChunkOptions())
	if err != nil {
		t.Fatal(err)
	}
	for _, ch := range set.Chunks {
		for id := range ch.LocalFactCatalog {
			if _, ok := pack.FactRegistry[id]; !ok {
				t.Fatalf("chunk %s unknown fact %s", ch.ChunkID, id)
			}
		}
	}
}

func TestValidateChunkSetRejectsUnknownFact(t *testing.T) {
	pack := testChunkFixturePack(5)
	set, err := BuildChunks(pack, DefaultChunkOptions())
	if err != nil {
		t.Fatal(err)
	}
	set.Chunks[0].LocalFactCatalog["unknown.fact"] = CompactFact{FactID: "unknown.fact"}
	if err := ValidateChunkSet(pack, set); err == nil {
		t.Fatal("expected unknown fact error")
	}
}

func TestValidateChunkSetRejectsUnknownEntity(t *testing.T) {
	pack := testChunkFixturePack(5)
	set, err := BuildChunks(pack, DefaultChunkOptions())
	if err != nil {
		t.Fatal(err)
	}
	set.Chunks[0].LocalEntityCatalog["sku:missing"] = CompactEntity{EntityID: "sku:missing", EntityType: "sku"}
	if err := ValidateChunkSet(pack, set); err == nil {
		t.Fatal("expected unknown entity error")
	}
}

func TestChunkHashStable(t *testing.T) {
	pack := testChunkFixturePack(10)
	set, err := BuildChunks(pack, DefaultChunkOptions())
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateChunkSet(pack, set); err != nil {
		t.Fatal(err)
	}
}

func TestNoSecretsInChunks(t *testing.T) {
	pack := testChunkFixturePack(5)
	set, err := BuildChunks(pack, DefaultChunkOptions())
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(set)
	lower := strings.ToLower(string(raw))
	if strings.Contains(lower, "api_key_encrypted") {
		t.Fatal("secrets in chunks")
	}
}

func TestAllSKUsStableOrderNoBusinessScoring(t *testing.T) {
	known := map[string]*skuMeta{
		"801000100002": {SKU: "801000100002"},
		"801000100001": {SKU: "801000100001"},
	}
	out, err := allSKUsStableOrder(known)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 || out[0] != "801000100001" {
		t.Fatalf("order=%v", out)
	}
}

func TestBuildChunksManifestContext(t *testing.T) {
	pack := minimalValidPack()
	pack.SellerContext.MetricDate = "2026-06-25"
	pack.DataWindows = ComputeDataWindows(time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC))
	set, err := BuildChunks(pack, DefaultChunkOptions())
	if err != nil {
		t.Fatal(err)
	}
	if set.Chunks[0].ChunkID != "manifest_context" {
		t.Fatalf("first chunk=%s", set.Chunks[0].ChunkID)
	}
}
