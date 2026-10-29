package aipipeline

import (
	"strconv"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
)

func coverageTestChunk(chunkID, sourceTable string) aidatapack.DataPackChunk {
	return aidatapack.DataPackChunk{
		ChunkID:     chunkID,
		ChunkType:   sourceTable,
		SourceTable: sourceTable,
		ChunkHash:   chunkID + "-hash",
	}
}

func run68StyleChunkSet() *aidatapack.ChunkSet {
	chunks := []aidatapack.DataPackChunk{
		coverageTestChunk("account_funnel_quality_constraints", "account_funnel_finance"),
	}
	for i := 1; i <= 4; i++ {
		chunks = append(chunks, coverageTestChunk("sku_rollups_00"+strconv.Itoa(i), "sku_rollups"))
		chunks = append(chunks, coverageTestChunk("stock_positions_00"+strconv.Itoa(i), "stock_positions"))
	}
	chunks = append(chunks, coverageTestChunk("campaign_rollups_001", "campaign_rollups"))
	chunks = append(chunks, coverageTestChunk("category_rollups_001", "category_rollups"))
	chunks = append(chunks, coverageTestChunk("product_enrichment_001", "product_enrichment"))
	chunks = append(chunks, coverageTestChunk("sku_groups_001", "entity_dictionary"))
	chunks = append(chunks, coverageTestChunk("campaign_product_links_001", "campaign_product_links"))
	for i := 14; i <= 23; i++ {
		chunks = append(chunks, coverageTestChunk("optional_"+strconv.Itoa(i), "product_enrichment"))
	}
	return &aidatapack.ChunkSet{Chunks: chunks}
}

func run68StyleFailedRun() *dashboardhome.ChunkAnalysisRunResult {
	failed := []string{
		"account_funnel_quality_constraints",
		"sku_rollups_001", "sku_rollups_002", "sku_rollups_003", "sku_rollups_004",
		"stock_positions_001", "stock_positions_002", "stock_positions_003", "stock_positions_004",
		"campaign_rollups_001",
		"category_rollups_001",
		"optional_14", "optional_15", "optional_16", "optional_17",
	}
	return &dashboardhome.ChunkAnalysisRunResult{
		ChunkCount:            24,
		Completed:             9,
		Failed:                15,
		ProviderFailedCount:   13,
		ValidationFailedCount: 2,
		FailedChunkIDs:        failed,
		FailedChunkTypes:      make([]string, len(failed)),
	}
}

func TestEvaluateSharedChunkCoverage_Run68Fails(t *testing.T) {
	set := run68StyleChunkSet()
	run := run68StyleFailedRun()
	coverage := EvaluateSharedChunkCoverage(set, run)
	if coverage.Passed {
		t.Fatalf("expected coverage failure: %+v", coverage)
	}
	if coverage.UsablePercent != 37 {
		t.Fatalf("usable_percent=%d", coverage.UsablePercent)
	}
	if coverage.FamilyCoverage["sku_rollups"].Passed {
		t.Fatal("sku_rollups should fail")
	}
	if coverage.FamilyCoverage["account_funnel"].Passed {
		t.Fatal("account_funnel should fail")
	}
}

func TestEvaluateSharedChunkCoverage_AllowablePartial(t *testing.T) {
	set := run68StyleChunkSet()
	run := &dashboardhome.ChunkAnalysisRunResult{
		ChunkCount: 24,
		Completed:  23,
		Failed:     1,
		FailedChunkIDs: []string{
			"product_enrichment_001",
		},
		FailedChunkTypes: []string{"product_enrichment"},
	}
	coverage := EvaluateSharedChunkCoverage(set, run)
	if !coverage.Passed {
		t.Fatalf("expected pass: %+v", coverage)
	}
}

func TestEvaluateSharedChunkCoverage_HighPercentButLostStockFails(t *testing.T) {
	set := run68StyleChunkSet()
	failed := []string{"stock_positions_001", "stock_positions_002", "stock_positions_003", "stock_positions_004"}
	run := &dashboardhome.ChunkAnalysisRunResult{
		ChunkCount:       24,
		Completed:        20,
		Failed:           4,
		FailedChunkIDs:   failed,
		FailedChunkTypes: make([]string, len(failed)),
	}
	coverage := EvaluateSharedChunkCoverage(set, run)
	if coverage.Passed {
		t.Fatal("expected failure when all stock positions lost")
	}
}

func TestEvaluateSharedChunkCoverage_NoCampaignDataOptional(t *testing.T) {
	set := &aidatapack.ChunkSet{
		Chunks: []aidatapack.DataPackChunk{
			coverageTestChunk("account_funnel_quality_constraints", "account_funnel_finance"),
			coverageTestChunk("sku_rollups_001", "sku_rollups"),
		},
	}
	run := &dashboardhome.ChunkAnalysisRunResult{
		ChunkCount: 2,
		Completed:  2,
		Failed:     0,
	}
	coverage := EvaluateSharedChunkCoverage(set, run)
	if !coverage.Passed {
		t.Fatalf("expected pass without campaign family: %+v", coverage)
	}
	if _, ok := coverage.FamilyCoverage["campaign_rollups"]; ok {
		t.Fatal("campaign family should be absent")
	}
}

func TestEvaluateSharedChunkCoverage_SingleCampaignChunkFails(t *testing.T) {
	set := &aidatapack.ChunkSet{
		Chunks: []aidatapack.DataPackChunk{
			coverageTestChunk("account_funnel_quality_constraints", "account_funnel_finance"),
			coverageTestChunk("sku_rollups_001", "sku_rollups"),
			coverageTestChunk("campaign_rollups_001", "campaign_rollups"),
		},
	}
	run := &dashboardhome.ChunkAnalysisRunResult{
		ChunkCount:       3,
		Completed:        2,
		Failed:           1,
		FailedChunkIDs:   []string{"campaign_rollups_001"},
		FailedChunkTypes: []string{"campaign_rollups"},
	}
	coverage := EvaluateSharedChunkCoverage(set, run)
	if coverage.Passed {
		t.Fatal("expected failure when only campaign chunk failed")
	}
}

func TestEvaluateSharedChunkCoverage_InsufficientDataDictionaryUsable(t *testing.T) {
	set := &aidatapack.ChunkSet{
		Chunks: []aidatapack.DataPackChunk{
			coverageTestChunk("account_funnel_quality_constraints", "account_funnel_finance"),
			coverageTestChunk("sku_rollups_001", "sku_rollups"),
			coverageTestChunk("sku_groups_001", "entity_dictionary"),
		},
	}
	run := &dashboardhome.ChunkAnalysisRunResult{
		ChunkCount:            3,
		Completed:             3,
		Failed:                0,
		InsufficientDataCount: 1,
	}
	coverage := EvaluateSharedChunkCoverage(set, run)
	if !coverage.Passed {
		t.Fatalf("insufficient_data usable chunk should not break coverage: %+v", coverage)
	}
}

func TestValidateStoredArtifactRejectsInsufficientCoverage(t *testing.T) {
	pack, set, err := PipelineChunkFixtureForTests()
	if err != nil {
		t.Fatal(err)
	}
	hashes, _ := orderedChunkHashes(set)
	run := &dashboardhome.ChunkAnalysisRunResult{
		SellerAccountID: 1,
		MetricDate:      "2026-06-25",
		ChunkCount:      len(set.Chunks),
		Completed:       1,
		Failed:          len(set.Chunks) - 1,
		AnalysisStatus:  dashboardhome.ChunkAnalysisStatusPartial,
		ValidatedSignals: []dashboardhome.DashboardChunkSignal{
			{SignalID: "s1", SignalType: dashboardhome.ChunkSignalObservation, Observation: "ok"},
		},
		ValidatedSignalCount: 1,
		FailedChunkIDs: func() []string {
			ids := make([]string, 0, len(set.Chunks)-1)
			for _, ch := range set.Chunks[1:] {
				ids = append(ids, ch.ChunkID)
			}
			return ids
		}(),
		FailedChunkTypes: func() []string {
			types := make([]string, len(set.Chunks)-1)
			return types
		}(),
	}
	normalized, limitations, err := normalizeSharedChunkRun(run)
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := buildSharedChunkAnalysisArtifact(pack, "hash-1", set, hashes, normalized, limitations)
	if err != nil {
		t.Fatal(err)
	}
	miss, _ := validateStoredSharedChunkAnalysisArtifact(
		artifact, 1, "2026-06-25", "hash-1", set, hashes, "", "",
	)
	if miss != "artifact_coverage_insufficient" {
		t.Fatalf("miss=%q", miss)
	}
}
