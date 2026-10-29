package aipipeline_test

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
)

func TestResolvePipelineStageRestartPlan_Actions(t *testing.T) {
	plan := aipipeline.ResolvePipelineStageRestartPlanForTest(aipipeline.RunSellerAIPipelineRequest{
		SourcePipelineRunID:   70,
		RestartFromStage:      aipipeline.ResumeStageActions,
		StageRestartRequested: true,
		GenerateDashboard:     true,
		GenerateActions:       true,
	})
	if plan.SourcePipelineRunID != 70 || !plan.CreateNewPipelineRun || plan.RestartFromStage != aipipeline.ResumeStageActions {
		t.Fatalf("plan=%+v", plan)
	}
	if !plan.ReuseDataPack || !plan.ReuseSharedChunkAnalysis || !plan.ReuseGlobalContext || !plan.CloneDashboard {
		t.Fatalf("reuse flags=%+v", plan)
	}
	if plan.ReuseActions || !plan.NeedsActions || !plan.NeedsPublication {
		t.Fatalf("actions/publication=%+v", plan)
	}
}

func TestPipelineStageRestartModeForTest(t *testing.T) {
	if !aipipeline.PipelineStageRestartModeForTest(aipipeline.RunSellerAIPipelineRequest{
		StageRestartRequested: true,
		SourcePipelineRunID:   70,
		RestartFromStage:      aipipeline.ResumeStageActions,
	}) {
		t.Fatal("expected stage restart mode")
	}
	if aipipeline.PipelineStageRestartModeForTest(aipipeline.RunSellerAIPipelineRequest{
		StageRestartRequested: true,
		SourcePipelineRunID:   70,
		RestartFromStage:      aipipeline.ResumeStageActions,
		ForceRegenerate:       true,
	}) {
		t.Fatal("force must disable stage restart mode")
	}
}
