package aipipeline_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aihash"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestResumeRun69Layout_ReusesUpstreamRunsActionsOnce(t *testing.T) {
	t.Setenv("SELLER_AI_PIPELINE_ENABLED", "1")
	t.Setenv("ACTIONS_AI_FIRST", "1")
	t.Setenv("AI_PIPELINE_SHARED_CHUNK_ANALYSIS_ENABLED", "false")

	pack := testSamplePack()
	packHash := aihash.HashDataPack(pack)
	dpDoc, _ := json.Marshal(map[string]any{
		"meta": map[string]any{
			"seller_account_id": int64(1),
			"metric_date":       "2026-06-24",
			"data_pack_hash":    packHash,
		},
		"payload": pack,
	})

	gc := aipipeline.NewDeterministicGlobalContextFakeClient()
	dash := &fakeDashboardRunner{}
	act := &fakeActionsRunner{}
	pub := &recordingSnapshotPublisher{}
	store := &resumePipelineStore{
		run: dbgen.AiPipelineRun{
			ID:                  69,
			SellerAccountID:     1,
			MetricDate:          pgtype.Date{Time: time.Date(2026, 6, 24, 0, 0, 0, 0, time.UTC), Valid: true},
			Status:              aipipeline.PipelineStatusCompletedPartial,
			DataPackHash:        packHash,
			GlobalContextStatus: pgtype.Text{String: aipipeline.GlobalContextStatusCompleted, Valid: true},
			DashboardRunID:      pgtype.UUID{Bytes: [16]byte{0x0a, 0xdf, 0x48, 0xc2}, Valid: true},
			DashboardStatus:     pgtype.Text{String: aipipeline.StepStatusCompleted, Valid: true},
			ActionsRunID:        pgtype.Int8{Int64: 51, Valid: true},
			ActionsStatus:       pgtype.Text{String: aipipeline.StepStatusFailed, Valid: true},
		},
		artifacts: []dbgen.AiPipelineArtifact{{
			ID: 1, PipelineRunID: 69, Kind: aipipeline.ArtifactKindDataPack, PayloadJson: dpDoc,
		}},
	}

	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: pack},
		dash,
		act,
		store,
		gc,
	)
	svc.SetSnapshotPublisher(pub)
	svc.SetActionsTimeoutSeconds(300)

	req := basePipelineRequest()
	req.ExistingPipelineRunID = 69
	req.ResumeRequested = true
	req.DispatchID = 1
	req.LeaseGuard = noopLeaseGuard{}

	res, err := svc.Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if dash.calls != 0 {
		t.Fatalf("dashboard calls=%d", dash.calls)
	}
	if act.calls != 1 {
		t.Fatalf("actions calls=%d", act.calls)
	}
	if pub.calls != 1 {
		t.Fatalf("publication calls=%d", pub.calls)
	}
	if res.Resume == nil || res.Resume.ResumeFromStage != aipipeline.ResumeStageActions {
		t.Fatalf("resume=%+v", res.Resume)
	}
	if res.Diagnostics == nil || res.Diagnostics.Actions.TimeoutSeconds != 300 {
		t.Fatalf("actions timeout=%d", res.Diagnostics.Actions.TimeoutSeconds)
	}
	if res.Resume.DashboardReused != true {
		t.Fatalf("dashboard_reused=false resume=%+v", res.Resume)
	}
}
