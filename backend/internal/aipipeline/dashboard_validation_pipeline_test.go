package aipipeline_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
)

type validationDashboardRunner struct {
	result *dashboardhome.NarrativeGenerationResult
}

func (r *validationDashboardRunner) RunForSellerMetricDateWithOptions(
	ctx context.Context,
	sellerAccountID int64,
	metricDate time.Time,
	opts dashboardhome.NarrativeRunOptions,
) (*aipipeline.DashboardRunOutcome, error) {
	return &aipipeline.DashboardRunOutcome{
		Result: r.result,
		DebugInput: map[string]any{
			"seller_account_id": sellerAccountID,
			"metric_date":       metricDate.Format("2006-01-02"),
		},
		DebugOutput: map[string]any{"payload": "synthetic"},
	}, nil
}

func TestDashboardValidationPilotMode_ResumeWithWarningsContinuesPipeline(t *testing.T) {
	sharedChunkPipelineEnv(t)
	t.Setenv("DASHBOARD_VALIDATION_MODE", dashboardhome.DashboardValidationModePilotStructural)

	pack := sharedChunkTestPack(t)
	runner := &stubChunkAnalyzer{}
	gc := aipipeline.NewDeterministicGlobalContextFakeClient()
	dash := &validationDashboardRunner{
		result: &dashboardhome.NarrativeGenerationResult{
			Status:                          "completed_with_warnings",
			RunID:                           "dash-run-warnings",
			DashboardSynthesisAICalls:       1,
			DashboardSynthesisInputTokens:   100,
			DashboardSynthesisOutputTokens:  20,
			DashboardSynthesisRequestID:     "req-dash-warn",
			DashboardValidationMode:         dashboardhome.DashboardValidationModePilotStructural,
			DashboardValidationStatus:       "completed_with_warnings",
			DashboardValidationWarningCount: 1,
			DashboardValidationWarningCodes: []string{"headline_mismatch_between_root_and_account_health"},
			DashboardStage4WarningCount:     1,
		},
	}
	act := &fakeActionsRunner{}
	pub := &recordingSnapshotPublisher{}
	store := &resumeTrackingStore{}

	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: pack},
		dash,
		act,
		store,
		gc,
	)
	svc.SetSnapshotPublisher(pub)
	svc.SetChunkAnalysisRunner(runner)

	first, err := svc.Run(context.Background(), sharedChunkPipelineRequest())
	if err != nil {
		t.Fatal(err)
	}
	runID := first.PipelineRunID

	secondReq := sharedChunkPipelineRequest()
	secondReq.ExistingPipelineRunID = runID
	secondReq.ResumeRequested = true
	second, err := svc.Run(context.Background(), secondReq)
	if err != nil {
		t.Fatal(err)
	}
	if second.DashboardStatus != aipipeline.StepStatusCompleted {
		t.Fatalf("dashboard_status=%s", second.DashboardStatus)
	}
	if second.ActionsStatus != aipipeline.StepStatusCompleted {
		t.Fatalf("actions_status=%s", second.ActionsStatus)
	}
	if !second.Published {
		t.Fatal("expected publication after dashboard warnings")
	}
	if runner.calls != 1 {
		t.Fatalf("chunk calls=%d want 1", runner.calls)
	}
	if second.Diagnostics == nil {
		t.Fatal("missing diagnostics")
	}
	if second.Diagnostics.Dashboard.Status != aipipeline.StepStatusCompleted {
		t.Fatalf("diag dashboard status=%q", second.Diagnostics.Dashboard.Status)
	}
	if second.Diagnostics.Dashboard.ValidationMode != dashboardhome.DashboardValidationModePilotStructural {
		t.Fatalf("validation_mode=%q", second.Diagnostics.Dashboard.ValidationMode)
	}
	if second.Diagnostics.Dashboard.ValidationStatus != "completed_with_warnings" {
		t.Fatalf("validation_status=%q", second.Diagnostics.Dashboard.ValidationStatus)
	}
	if second.Diagnostics.Dashboard.ValidationWarningCount == 0 {
		t.Fatal("expected warning count in diagnostics")
	}
}

func TestDashboardValidationPilotMode_StructuralFailureSkipsActions(t *testing.T) {
	sharedChunkPipelineEnv(t)
	t.Setenv("DASHBOARD_VALIDATION_MODE", dashboardhome.DashboardValidationModePilotStructural)

	pack := sharedChunkTestPack(t)
	runner := &stubChunkAnalyzer{}
	gc := aipipeline.NewDeterministicGlobalContextFakeClient()
	dash := &validationDashboardRunner{
		result: &dashboardhome.NarrativeGenerationResult{
			Status:       "failed",
			ErrorMessage: "stage4 block validation failed: account_health:missing_account_health",
		},
	}
	act := &fakeActionsRunner{}
	pub := &recordingSnapshotPublisher{}

	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: pack},
		dash,
		act,
		&persistingArtifactStore{},
		gc,
	)
	svc.SetSnapshotPublisher(pub)
	svc.SetChunkAnalysisRunner(runner)

	res, err := svc.Run(context.Background(), sharedChunkPipelineRequest())
	if err != nil {
		t.Fatal(err)
	}
	if res.DashboardStatus != aipipeline.StepStatusFailed {
		t.Fatalf("dashboard_status=%s", res.DashboardStatus)
	}
	if res.ActionsStatus == aipipeline.StepStatusSkipped {
		t.Fatalf("actions should run independently after dashboard failure, got skipped")
	}
	if res.Published {
		t.Fatal("expected no publication")
	}
	if res.Diagnostics == nil || res.Diagnostics.Dashboard.Status != aipipeline.StepStatusFailed {
		t.Fatalf("diag=%+v", res.Diagnostics)
	}
}

func TestDashboardValidationStrictMode_HeadlineMismatchFailsPipeline(t *testing.T) {
	sharedChunkPipelineEnv(t)
	t.Setenv("DASHBOARD_VALIDATION_MODE", dashboardhome.DashboardValidationModeStrict)

	pack := sharedChunkTestPack(t)
	runner := &stubChunkAnalyzer{}
	gc := aipipeline.NewDeterministicGlobalContextFakeClient()
	dash := &validationDashboardRunner{
		result: &dashboardhome.NarrativeGenerationResult{
			Status:       "failed",
			ErrorMessage: "stage4 cross-block review failed: headline_mismatch_between_root_and_account_health",
		},
	}
	act := &fakeActionsRunner{}
	pub := &recordingSnapshotPublisher{}

	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: pack},
		dash,
		act,
		&persistingArtifactStore{},
		gc,
	)
	svc.SetSnapshotPublisher(pub)
	svc.SetChunkAnalysisRunner(runner)

	res, err := svc.Run(context.Background(), sharedChunkPipelineRequest())
	if err != nil {
		t.Fatal(err)
	}
	if res.DashboardStatus != aipipeline.StepStatusFailed {
		t.Fatalf("dashboard_status=%s", res.DashboardStatus)
	}
	if res.ActionsStatus == aipipeline.StepStatusSkipped {
		t.Fatalf("actions should run independently after dashboard failure, got skipped")
	}
	if res.Published {
		t.Fatal("expected no publication in strict mode")
	}
}

func TestDashboardValidationArtifactPersistedOnSuccess(t *testing.T) {
	sharedChunkPipelineEnv(t)
	t.Setenv("DASHBOARD_VALIDATION_MODE", dashboardhome.DashboardValidationModePilotStructural)

	pack := sharedChunkTestPack(t)
	store := &persistingArtifactStore{}
	dash := &validationDashboardRunner{
		result: &dashboardhome.NarrativeGenerationResult{
			Status:                          "completed_with_warnings",
			RunID:                           "dash-run-artifact",
			DashboardValidationMode:         dashboardhome.DashboardValidationModePilotStructural,
			DashboardValidationStatus:       "completed_with_warnings",
			DashboardValidationWarningCount: 1,
			DashboardValidationWarningCodes: []string{"headline_mismatch_between_root_and_account_health"},
		},
	}

	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: pack},
		dash,
		&fakeActionsRunner{},
		store,
		aipipeline.NewDeterministicGlobalContextFakeClient(),
	)
	svc.SetSnapshotPublisher(&recordingSnapshotPublisher{})
	svc.SetChunkAnalysisRunner(&stubChunkAnalyzer{})

	req := sharedChunkPipelineRequest()
	res, err := svc.Run(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	arts, err := store.ListArtifacts(context.Background(), res.PipelineRunID)
	if err != nil {
		t.Fatal(err)
	}
	var validationRaw json.RawMessage
	for _, art := range arts {
		if art.Kind == aipipeline.ArtifactKindDashboardValidation {
			validationRaw = art.PayloadJson
			break
		}
	}
	if len(validationRaw) == 0 {
		t.Fatalf("artifacts=%v", arts)
	}
	payload := decodeArtifactPayload(t, validationRaw)
	if payload["mode"] != dashboardhome.DashboardValidationModePilotStructural {
		t.Fatalf("mode=%v", payload["mode"])
	}
	if payload["valid"] != true {
		t.Fatalf("valid=%v", payload["valid"])
	}
	warnings, _ := payload["warnings"].([]any)
	if len(warnings) == 0 {
		t.Fatalf("payload=%v", payload)
	}
}

func TestDashboardValidationPilotMode_MultipleSemanticIssuesContinuePipeline(t *testing.T) {
	sharedChunkPipelineEnv(t)
	t.Setenv("DASHBOARD_VALIDATION_MODE", dashboardhome.DashboardValidationModePilotStructural)

	pack := sharedChunkTestPack(t)
	runner := &stubChunkAnalyzer{}
	gc := aipipeline.NewDeterministicGlobalContextFakeClient()
	dash := &validationDashboardRunner{
		result: &dashboardhome.NarrativeGenerationResult{
			Status:                          "completed_with_warnings",
			RunID:                           "dash-run-multi-warn",
			DashboardSynthesisAICalls:       1,
			DashboardValidationMode:         dashboardhome.DashboardValidationModePilotStructural,
			DashboardValidationStatus:       "completed_with_warnings",
			DashboardValidationWarningCount: 5,
			DashboardValidationWarningCodes: []string{
				"headline_mismatch_between_root_and_account_health",
				"top_actions_lt_3",
				"top_action_missing_fact_ids",
				"top_action_stock_classified_as_sales",
				"category_summary_incomplete",
			},
		},
	}
	act := &fakeActionsRunner{}
	pub := &recordingSnapshotPublisher{}

	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: pack},
		dash,
		act,
		&persistingArtifactStore{},
		gc,
	)
	svc.SetSnapshotPublisher(pub)
	svc.SetChunkAnalysisRunner(runner)

	res, err := svc.Run(context.Background(), sharedChunkPipelineRequest())
	if err != nil {
		t.Fatal(err)
	}
	if res.DashboardStatus != aipipeline.StepStatusCompleted {
		t.Fatalf("dashboard_status=%s", res.DashboardStatus)
	}
	if res.ActionsStatus != aipipeline.StepStatusCompleted {
		t.Fatalf("actions_status=%s", res.ActionsStatus)
	}
	if !res.Published {
		t.Fatal("expected publication")
	}
	if runner.calls != 1 {
		t.Fatalf("chunk calls=%d", runner.calls)
	}
}
