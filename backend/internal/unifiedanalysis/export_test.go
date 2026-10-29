package unifiedanalysis

import (
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardfacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

// MustMinimalValidPackZip exposes mustMinimalValidPackZip for external
// integration tests (package unifiedanalysis_test).
func MustMinimalValidPackZip(packID string, sellerID int64, metricDate string) (zipBytes []byte, packHash string) {
	return mustMinimalValidPackZip(packID, sellerID, metricDate)
}

// MinimalTestSnapshot exposes minimalTestSnapshot for external integration tests.
func MinimalTestSnapshot(sellerID int64, metricDate time.Time) *analysisdata.AnalysisDataSnapshot {
	return minimalTestSnapshot(sellerID, metricDate)
}

// BuildFixtureDashboardFacts exposes buildFixtureDashboardFacts for publish-only
// Asynq / worker-restart integration tests that must seed facts before PublishRun.
func BuildFixtureDashboardFacts(sellerID int64, metricDate time.Time, packHash string, out *contract.UnifiedAnalysisOutputV1) dashboardfacts.DashboardFactsV1 {
	return buildFixtureDashboardFacts(sellerID, metricDate, packHash, out)
}
