package dashboardhome

import (
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
)

// SetBuildChunkSetForPackHookForTest overrides chunk set building in tests.
func SetBuildChunkSetForPackHookForTest(fn func(*aidatapack.SellerAIDataPack, aidatapack.ChunkOptions) (*aidatapack.ChunkSet, error)) {
	buildChunkSetForPackHook = fn
}

// BuildChunkSetForPackCallsForTest returns how many times narrative chunk build ran.
func BuildChunkSetForPackCallsForTest() int {
	return int(buildChunkSetForPackCalls.Load())
}

// ResetBuildChunkSetForPackCallsForTest resets the narrative chunk build counter.
func ResetBuildChunkSetForPackCallsForTest() {
	buildChunkSetForPackCalls.Store(0)
}

// ResetBuildChunkSetForPackHookForTest clears the test hook.
func ResetBuildChunkSetForPackHookForTest() {
	buildChunkSetForPackHook = nil
}

// SynthesisInputTokenLimitForTest exposes the configured synthesis input token limit.
func SynthesisInputTokenLimitForTest() int {
	return maxDashboardSynthesisInputTokens()
}

// EstimateSynthesisInputTokensForTest estimates synthesis input size in tokens.
func EstimateSynthesisInputTokensForTest(input DashboardSynthesisInput) (int, error) {
	return estimateSynthesisInputTokens(input)
}

// DashboardSynthesisSystemPromptForTest exposes the synthesis system prompt for tests.
func DashboardSynthesisSystemPromptForTest() string {
	return systemPromptDashboardSynthesis()
}

// DashboardChunkAnalysisSystemPromptForTest exposes the chunk analysis system prompt for tests.
func DashboardChunkAnalysisSystemPromptForTest() string {
	return systemPromptDashboardChunkAnalysis()
}

// EnablePersistFrozenDashboardSnapshotTrackingForTest counts persistFrozenDashboardSnapshot calls.
func EnablePersistFrozenDashboardSnapshotTrackingForTest() {
	trackPersistFrozenDashboardSnapshotCalls.Store(true)
	persistFrozenDashboardSnapshotCalls.Store(0)
}

// PersistFrozenDashboardSnapshotCallsForTest returns persist call count when tracking is enabled.
func PersistFrozenDashboardSnapshotCallsForTest() int {
	return int(persistFrozenDashboardSnapshotCalls.Load())
}

// ResetPersistFrozenDashboardSnapshotTrackingForTest disables snapshot call tracking.
func ResetPersistFrozenDashboardSnapshotTrackingForTest() {
	trackPersistFrozenDashboardSnapshotCalls.Store(false)
	persistFrozenDashboardSnapshotCalls.Store(0)
}

// ShouldPersistFrozenDashboardSnapshotForTest reports when narrative context build persists a snapshot.
func ShouldPersistFrozenDashboardSnapshotForTest(opts NarrativeRunOptions) bool {
	return shouldPersistFrozenDashboardSnapshot(opts)
}

// ResetLegacyNarrativeDataLoadsForTest resets legacy alerts/recommendations load counters.
func ResetLegacyNarrativeDataLoadsForTest() {
	legacyAlertsLoads.Store(0)
	legacyRecommendationsLoads.Store(0)
}

// LegacyAlertsLoadsForTest returns how many legacy alert loads were attempted.
func LegacyAlertsLoadsForTest() int {
	return int(legacyAlertsLoads.Load())
}

// LegacyRecommendationsLoadsForTest returns how many legacy recommendation loads were attempted.
func LegacyRecommendationsLoadsForTest() int {
	return int(legacyRecommendationsLoads.Load())
}

// RecordLegacyNarrativeDataLoadsForTest simulates legacy SQL load attempts for semantics tests.
func RecordLegacyNarrativeDataLoadsForTest(opts NarrativeRunOptions) {
	if UsesUnifiedDataPackContext(opts) {
		return
	}
	legacyAlertsLoads.Add(1)
	legacyRecommendationsLoads.Add(1)
}

// MinimalDashboardBuildForTest builds a chunks-context dashboard build for pipeline integration tests.
func MinimalDashboardBuildForTest(pack *aidatapack.SellerAIDataPack) (dashboardNarrativeBuildResult, error) {
	return BuildPrecomputedSynthesisNarrativeContext(pack)
}
