package aipipeline_test

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

func resumeReq() aipipeline.RunSellerAIPipelineRequest {
	return aipipeline.RunSellerAIPipelineRequest{
		SellerAccountID:       1,
		GenerateDashboard:     true,
		GenerateActions:       true,
		ExistingPipelineRunID: 69,
		ResumeRequested:       true,
	}
}

func completedPersistedRun() dbgen.AiPipelineRun {
	return dbgen.AiPipelineRun{
		ID:                  69,
		Status:              aipipeline.PipelineStatusCompletedPartial,
		DataPackHash:        "sha256:dp",
		GlobalContextHash:   pgtype.Text{String: "sha256:gc", Valid: true},
		GlobalContextStatus: pgtype.Text{String: aipipeline.GlobalContextStatusCompleted, Valid: true},
		DashboardRunID:      pgtype.UUID{Bytes: [16]byte{9}, Valid: true},
		DashboardStatus:     pgtype.Text{String: aipipeline.StepStatusCompleted, Valid: true},
		ActionsRunID:        pgtype.Int8{Int64: 51, Valid: true},
		ActionsStatus:       pgtype.Text{String: aipipeline.StepStatusFailed, Valid: true},
	}
}

func TestResolvePipelineResumePlan_Run69Layout(t *testing.T) {
	plan := aipipeline.ResolvePipelineResumePlanForTest(aipipeline.ResumePlanInputForTest{
		Req:                   resumeReq(),
		Persisted:             completedPersistedRun(),
		DataPackHash:          "sha256:dp",
		GlobalContextHash:     "sha256:gc",
		SharedChunkReusable:   true,
		GlobalContextReusable: true,
		DashboardReusable:     true,
		ActionsReusable:       false,
	})
	if plan.ResumeFromStage != aipipeline.ResumeStageActions {
		t.Fatalf("resume_from_stage=%q", plan.ResumeFromStage)
	}
	if !plan.ReuseDataPack || !plan.ReuseSharedChunkAnalysis || !plan.ReuseGlobalContext || !plan.ReuseDashboard {
		t.Fatalf("reuse flags=%+v", plan)
	}
	if plan.ReuseActions || !plan.NeedsActions || !plan.NeedsPublication {
		t.Fatalf("actions flags=%+v", plan)
	}
}

func TestResolvePipelineResumePlan_GCFailedResumeFromGC(t *testing.T) {
	run := completedPersistedRun()
	run.GlobalContextStatus = pgtype.Text{String: aipipeline.GlobalContextStatusFailed, Valid: true}
	run.GlobalContextHash = pgtype.Text{}
	run.DashboardStatus = pgtype.Text{}
	run.DashboardRunID = pgtype.UUID{}
	plan := aipipeline.ResolvePipelineResumePlanForTest(aipipeline.ResumePlanInputForTest{
		Req:                 resumeReq(),
		Persisted:           run,
		DataPackHash:        "sha256:dp",
		SharedChunkReusable: true,
	})
	if plan.ResumeFromStage != aipipeline.ResumeStageGlobalContext {
		t.Fatalf("resume_from_stage=%q", plan.ResumeFromStage)
	}
	if !plan.ReuseSharedChunkAnalysis || plan.ReuseGlobalContext {
		t.Fatalf("reuse=%+v", plan)
	}
}

func TestResolvePipelineResumePlan_DashboardFailedResumeFromDashboard(t *testing.T) {
	run := completedPersistedRun()
	run.DashboardStatus = pgtype.Text{String: aipipeline.StepStatusFailed, Valid: true}
	run.DashboardRunID = pgtype.UUID{Bytes: [16]byte{9}, Valid: true}
	run.ActionsStatus = pgtype.Text{}
	run.ActionsRunID = pgtype.Int8{}
	plan := aipipeline.ResolvePipelineResumePlanForTest(aipipeline.ResumePlanInputForTest{
		Req:                   resumeReq(),
		Persisted:             run,
		DataPackHash:          "sha256:dp",
		GlobalContextHash:     "sha256:gc",
		SharedChunkReusable:   true,
		GlobalContextReusable: true,
		DashboardReusable:     false,
	})
	if plan.ResumeFromStage != aipipeline.ResumeStageDashboard {
		t.Fatalf("resume_from_stage=%q", plan.ResumeFromStage)
	}
	if !plan.NeedsDashboard {
		t.Fatalf("needs_dashboard=false")
	}
}

func TestResolvePipelineResumePlan_ActionsCompletedNeedsPublication(t *testing.T) {
	run := completedPersistedRun()
	run.ActionsStatus = pgtype.Text{String: "completed_partial", Valid: true}
	plan := aipipeline.ResolvePipelineResumePlanForTest(aipipeline.ResumePlanInputForTest{
		Req:                   resumeReq(),
		Persisted:             run,
		DataPackHash:          "sha256:dp",
		GlobalContextHash:     "sha256:gc",
		SharedChunkReusable:   true,
		GlobalContextReusable: true,
		DashboardReusable:     true,
		ActionsReusable:       true,
	})
	if plan.ResumeFromStage != aipipeline.ResumeStagePublication {
		t.Fatalf("resume_from_stage=%q", plan.ResumeFromStage)
	}
	if !plan.NeedsPublication {
		t.Fatalf("needs_publication=false")
	}
}

func TestResolvePipelineResumePlan_AlreadyPublished(t *testing.T) {
	run := completedPersistedRun()
	run.PublishedAt = pgtype.Timestamptz{Valid: true}
	plan := aipipeline.ResolvePipelineResumePlanForTest(aipipeline.ResumePlanInputForTest{
		Req:       resumeReq(),
		Persisted: run,
		Published: true,
	})
	if plan.ResumeFromStage != aipipeline.ResumeStageCompleted {
		t.Fatalf("resume_from_stage=%q", plan.ResumeFromStage)
	}
}
