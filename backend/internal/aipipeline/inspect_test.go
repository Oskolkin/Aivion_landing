package aipipeline_test

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestInspectPipelineNoReviewerFields(t *testing.T) {
	run := dbgen.AiPipelineRun{
		ID:                7,
		Status:            aipipeline.PipelineStatusCompleted,
		DataPackHash:      "sha256:abc",
		GlobalContextHash: pgtype.Text{String: "sha256:gc", Valid: true},
	}
	artifacts := []dbgen.AiPipelineArtifact{
		{
			ID:          1,
			Kind:        aipipeline.ArtifactKindDataPack,
			Hash:        pgtype.Text{String: "sha256:art", Valid: true},
			PayloadJson: []byte(`{"meta":{"provider":"openai"},"payload":{}}`),
		},
	}
	summary := aipipeline.BuildPipelineInspectSummary(run, artifacts)
	if summary.PipelineRun.ID != 7 {
		t.Fatalf("unexpected run id %d", summary.PipelineRun.ID)
	}
	if len(summary.Artifacts) != 1 || summary.Artifacts[0].Provider != aipipeline.ProviderOpenAI {
		t.Fatalf("artifact inspect mismatch %+v", summary.Artifacts)
	}
}
