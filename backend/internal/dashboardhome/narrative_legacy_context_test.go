package dashboardhome

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
)

func TestUsesUnifiedDataPackContext_UnifiedSharedPathSkipsLegacy(t *testing.T) {
	ResetLegacyNarrativeDataLoadsForTest()
	opts := NarrativeRunOptions{PrecomputedChunkRun: &ChunkAnalysisRunResult{}}
	if !UsesUnifiedDataPackContext(opts) {
		t.Fatal("precomputed path must be unified")
	}
	RecordLegacyNarrativeDataLoadsForTest(opts)
	if LegacyAlertsLoadsForTest() != 0 || LegacyRecommendationsLoadsForTest() != 0 {
		t.Fatalf("alerts=%d recommendations=%d", LegacyAlertsLoadsForTest(), LegacyRecommendationsLoadsForTest())
	}
}

func TestUsesUnifiedDataPackContext_StandaloneOverrideStillLoadsLegacy(t *testing.T) {
	ResetLegacyNarrativeDataLoadsForTest()
	pack := sampleDataPackForNarrativeTest()
	chunkSet := &aidatapack.ChunkSet{}
	opts := NarrativeRunOptions{
		DataPackOverride: pack,
		ChunkSetOverride: chunkSet,
	}
	if UsesUnifiedDataPackContext(opts) {
		t.Fatal("override without precomputed run must remain standalone fallback")
	}
	RecordLegacyNarrativeDataLoadsForTest(opts)
	if LegacyAlertsLoadsForTest() != 1 || LegacyRecommendationsLoadsForTest() != 1 {
		t.Fatalf("alerts=%d recommendations=%d want 1/1", LegacyAlertsLoadsForTest(), LegacyRecommendationsLoadsForTest())
	}
}

func TestUsesUnifiedDataPackContext_LegacyPathLoadsLegacy(t *testing.T) {
	ResetLegacyNarrativeDataLoadsForTest()
	RecordLegacyNarrativeDataLoadsForTest(NarrativeRunOptions{})
	if LegacyAlertsLoadsForTest() != 1 || LegacyRecommendationsLoadsForTest() != 1 {
		t.Fatalf("alerts=%d recommendations=%d", LegacyAlertsLoadsForTest(), LegacyRecommendationsLoadsForTest())
	}
}
