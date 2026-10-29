package dashboardhome

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestShouldReuseExistingDashboardNarrative_RejectsDifferentPipeline(t *testing.T) {
	pipelineID := int64(99)
	existing := dbgen.AiDashboardNarrative{
		Status:        "completed",
		PromptVersion: DashboardNarrativePromptVersion,
		ContextHash:   "ctx",
		AiPipelineRunID: pgtype.Int8{
			Int64: 1,
			Valid: true,
		},
	}
	build := dashboardNarrativeBuildResult{FactualContextHash: "ctx"}
	opts := NarrativeRunOptions{PipelineRunID: &pipelineID}
	if shouldReuseExistingDashboardNarrative(existing, opts, build) {
		t.Fatal("must not reuse dashboard narrative from another pipeline")
	}
}

func TestShouldRetryDashboardNarrativeOnSameRow_FailedPipelineRun(t *testing.T) {
	svc := &NarrativeService{}
	pipelineID := int64(42)
	existing := dbgen.AiDashboardNarrative{
		Status:          "failed",
		AiPipelineRunID: pgtype.Int8{Int64: pipelineID, Valid: true},
	}
	opts := NarrativeRunOptions{PipelineRunID: &pipelineID}
	if !svc.shouldRetryDashboardNarrativeOnSameRow(existing, opts) {
		t.Fatal("failed pipeline dashboard must retry on same row")
	}
}
