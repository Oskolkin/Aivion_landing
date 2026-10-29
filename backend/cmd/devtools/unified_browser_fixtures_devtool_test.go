package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/publication"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/publication/adapter"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/publication/bundlevalidator"
)

// TestIsSuccessfulPublishOutcome locks down which publication.PublishOutcome
// values findOrPublishFixture treats as "this fixture is now a usable
// published row". This is the fix for the root cause bug where
// findOrPublishFixture treated ANY non-error PublishRun return (including
// OutcomeBundleValidationFailed) as success, silently leaving the seller
// with a publication_failed row and no current unified publication.
func TestIsSuccessfulPublishOutcome(t *testing.T) {
	successOutcomes := []publication.PublishOutcome{
		publication.OutcomePublished,
		publication.OutcomeAlreadyPublished,
	}
	for _, outcome := range successOutcomes {
		if !isSuccessfulPublishOutcome(outcome) {
			t.Errorf("expected outcome %q to be treated as successful", outcome)
		}
	}

	failureOutcomes := []publication.PublishOutcome{
		publication.OutcomeBundleValidationFailed,
		publication.OutcomeSkippedAnalysisFailed,
		publication.OutcomeStale,
		publication.PublishOutcome("unknown_future_outcome"),
	}
	for _, outcome := range failureOutcomes {
		if isSuccessfulPublishOutcome(outcome) {
			t.Errorf("expected outcome %q to NOT be treated as successful", outcome)
		}
	}
}

// TestBuildBrowserDashboardFacts_SatisfiesBundleValidator is a regression
// test for the root cause: seedPublicationReadyFixture used to seed a
// publication_ready run WITHOUT a unified_dashboard_facts_v1 artifact, so
// PublishRun always failed bundlevalidator.ValidatePublicationBundle
// (CodeDashboardFactsMissing) for every browser fixture scenario. This test
// exercises the exact facts+projection+validate pipeline PublishRun runs,
// without needing a database, for every fixture used by
// ensureBrowserFixtures.
func TestBuildBrowserDashboardFacts_SatisfiesBundleValidator(t *testing.T) {
	const sellerID int64 = 9999
	metricDate := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)
	packHash := "test_pack_hash_0000000000000000000000000000000000000000000000000000"

	cases := []string{
		"unified_output_v1.valid.json",
		"unified_output_v1.no_actions.json",
		"unified_output_v1.insufficient_data.json",
	}

	for _, fixtureName := range cases {
		t.Run(fixtureName, func(t *testing.T) {
			raw, err := readContractFixture(fixtureName)
			if err != nil {
				t.Fatalf("readContractFixture: %v", err)
			}
			var out contract.UnifiedAnalysisOutputV1
			if err := json.Unmarshal(raw, &out); err != nil {
				t.Fatalf("unmarshal fixture: %v", err)
			}
			out.AnalysisMeta.MetricDate = metricDate.Format("2006-01-02")

			facts := buildBrowserDashboardFacts(sellerID, metricDate, packHash, &out)
			metricDateStr := metricDate.Format("2006-01-02")

			projection, _ := adapter.ProjectDashboard(facts, &out, sellerID, metricDateStr)
			projectedActions := adapter.ProjectActions(&out, 0, sellerID, facts.PackID, metricDate, nil)

			report := bundlevalidator.ValidatePublicationBundle(bundlevalidator.Input{
				FactsPresent:    true,
				FactsIdentityOK: true,
				Facts:           facts,
				Projection:      projection,
				Actions:         projectedActions,
				AnalysisOutcome: out.AnalysisOutcome,
			})
			if !report.Valid {
				t.Fatalf("bundle validation failed for %s: %+v", fixtureName, report.Errors)
			}
		})
	}
}
