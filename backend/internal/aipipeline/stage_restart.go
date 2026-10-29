package aipipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicontext"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

func (s *SellerAIPipelineService) runActionsStageRestart(
	ctx context.Context,
	req RunSellerAIPipelineRequest,
) (*RunSellerAIPipelineResult, error) {
	if err := ValidatePipelineConfig(); err != nil {
		return nil, err
	}
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("seller ai pipeline service is not configured")
	}
	if !req.GenerateDashboard || !req.GenerateActions {
		return nil, fmt.Errorf("stage restart requires dashboard and actions generation")
	}
	if s.actions == nil {
		return nil, fmt.Errorf("actions runner is not configured")
	}
	if s.publisher == nil {
		return nil, fmt.Errorf("snapshot publisher is not configured")
	}
	cloneStore, ok := s.store.(DashboardCloneStore)
	if !ok {
		return nil, fmt.Errorf("dashboard clone store is not configured")
	}

	trigger := req.Trigger
	if trigger == "" {
		trigger = "devtool_stage_restart"
	}

	sourceBundle, preflightCode, preflightErr := s.loadActionsStageRestartSource(ctx, req)
	if preflightErr != nil {
		result := &RunSellerAIPipelineResult{
			SellerAccountID:     req.SellerAccountID,
			SourcePipelineRunID: req.SourcePipelineRunID,
			RestartFromStage:    req.RestartFromStage,
			ErrorCode:           preflightCode,
			ErrorMessage:        preflightErr.Error(),
			Status:              PipelineStatusFailed,
		}
		return result, nil
	}
	sourceRun := sourceBundle.Run
	metricDate := stageRestartSourceMetricDate(sourceRun)
	if req.SellerAccountID <= 0 {
		req.SellerAccountID = sourceRun.SellerAccountID
	}
	req.MetricDate = metricDate

	providers := req.ResolvedProviders
	if providers.GlobalContext == "" {
		providers = DefaultProvidersFromRequest(req)
	}

	result := &RunSellerAIPipelineResult{
		SellerAccountID:     req.SellerAccountID,
		MetricDate:          metricDate.Format("2006-01-02"),
		DataPackHash:        sourceBundle.DataPackHash,
		SourcePipelineRunID: req.SourcePipelineRunID,
		RestartFromStage:    req.RestartFromStage,
		ArtifactRefs:        map[string]string{},
		Providers:           &providers,
	}
	diag := newPipelineDiagnostics(providers)
	restartDiag := &StageRestartDiagnostics{
		Requested:            true,
		Mode:                 stageRestartModeDerivedRun,
		SourcePipelineRunID:  req.SourcePipelineRunID,
		RestartFromStage:     req.RestartFromStage,
		DataPackSource:       stageRestartArtifactSourceMeta,
		DataPackReused:       true,
		SharedChunkSource:    stageRestartArtifactSourceMeta,
		SharedChunkReused:    true,
		SharedChunkAICalls:   0,
		GlobalContextSource:  stageRestartArtifactSourceMeta,
		GlobalContextReused:  true,
		GlobalContextAICalls: 0,
		DashboardSource:      "cloned_from_source_pipeline",
		DashboardReused:      true,
		DashboardAICalls:     0,
		ActionsSource:        "generated",
		ActionsReused:        false,
	}
	diag.Restart = restartDiag
	result.Diagnostics = &diag

	if err := s.guardExecutionLease(ctx, req, 0); err != nil {
		return nil, err
	}

	runID, err := s.store.CreatePipelineRun(ctx, CreatePipelineRunParams{
		SellerAccountID:       sourceRun.SellerAccountID,
		MetricDate:            metricDate,
		Mode:                  PipelineModeUnified,
		Trigger:               trigger,
		Status:                PipelineStatusRunning,
		DataPackSchemaVersion: sourceBundle.DataPack.SchemaVersion,
		DataPackHash:          sourceBundle.DataPackHash,
		StartedAt:             time.Now().UTC(),
	})
	if err != nil {
		return nil, err
	}
	result.PipelineRunID = runID
	restartDiag.TargetPipelineRunID = runID

	artifacts := NewArtifactRecorder(s.store, runID, req.DebugArtifacts)
	stageRestartMeta := func(sourceArtifactID int64) map[string]any {
		meta := stageRestartArtifactMeta(runID, req.SourcePipelineRunID, req.RestartFromStage, sourceBundle.DataPackHash)
		if sourceArtifactID > 0 {
			meta["source_artifact_id"] = sourceArtifactID
		}
		return meta
	}

	if err := artifacts.SaveRequiredDocument(ctx, ArtifactKindDataPack, stageRestartMeta(0), sourceBundle.DataPack); err != nil {
		return s.failStageRestartRun(ctx, runID, result, "artifact_save_failed", err, artifacts, restartDiag)
	}

	var sharedChunkSet *aidatapack.ChunkSet
	var sharedChunkRun *dashboardhome.ChunkAnalysisRunResult
	if SharedChunkAnalysisEnabled() {
		sharedChunkSet = sourceBundle.ChunkSet
		sharedChunkRun = sourceBundle.SharedAnalysis
		if sharedChunkSet == nil || sharedChunkRun == nil {
			return s.failStageRestartRun(ctx, runID, result, "stage_restart_shared_analysis_artifact_missing", fmt.Errorf("shared chunk analysis is missing from source bundle"), artifacts, restartDiag)
		}
		if err := s.persistCopiedSharedChunkArtifacts(ctx, req.SourcePipelineRunID, runID, sourceBundle.DataPack, sourceBundle.DataPackHash, metricDate, sharedChunkSet, sharedChunkRun, artifacts, stageRestartMeta); err != nil {
			code := "stage_restart_shared_analysis_artifact_missing"
			if strings.Contains(err.Error(), "persist") {
				code = "artifact_save_failed"
			}
			return s.failStageRestartRun(ctx, runID, result, code, err, artifacts, restartDiag)
		}
		populateSharedChunkDiagnostics(&diag, sharedChunkRun, stageRestartArtifactSourceMeta, "", sharedChunkSet)
		diag.SharedChunkAnalysis.UsageSource = UsageSourceStageRestartSource
	}

	globalCtx := sourceBundle.GlobalContext.Context
	gcHash := sourceBundle.GlobalContext.ContextHash
	gcStatus := GlobalContextStatusCompleted
	if err := s.persistCopiedGlobalContextArtifact(ctx, req.SourcePipelineRunID, runID, sourceBundle.DataPackHash, artifacts, stageRestartMeta); err != nil {
		return s.failStageRestartRun(ctx, runID, result, "stage_restart_global_context_artifact_missing", err, artifacts, restartDiag)
	}
	populateGlobalContextDiagnosticsFromLoad(&diag, sourceBundle.GlobalContext)
	result.GlobalContextHash = gcHash
	result.GlobalContextStatus = gcStatus
	diag.GlobalContext.Status = gcStatus

	sourceDashboardID := uuidString(sourceRun.DashboardRunID)
	restartDiag.SourceDashboardRunID = sourceDashboardID
	newDashboardID, cloneErr := cloneStore.CloneDashboardNarrativeForPipeline(
		ctx,
		sourceDashboardID,
		req.SourcePipelineRunID,
		runID,
		sourceRun.SellerAccountID,
		sourceBundle.DataPackHash,
		gcHash,
	)
	if cloneErr != nil {
		return s.failStageRestartRun(ctx, runID, result, "stage_restart_dashboard_clone_failed", cloneErr, artifacts, restartDiag)
	}
	newSnapshotID, cloneErr := cloneStore.CloneDashboardDataSnapshotForPipeline(
		ctx,
		sourceRun.DashboardDataSnapshotID.Int64,
		req.SourcePipelineRunID,
		runID,
		sourceRun.SellerAccountID,
	)
	if cloneErr != nil {
		return s.failStageRestartRun(ctx, runID, result, "stage_restart_dashboard_clone_failed", cloneErr, artifacts, restartDiag)
	}
	restartDiag.DashboardRunID = newDashboardID
	result.DashboardRunID = newDashboardID
	result.DashboardStatus = textOrEmpty(sourceRun.DashboardStatus)
	diag.Dashboard.Status = result.DashboardStatus
	diag.Dashboard.Reused = true
	diag.Dashboard.ArtifactSource = "cloned_from_source_pipeline"
	diag.Dashboard.SynthesisAICalls = 0
	diag.Dashboard.UsageSource = UsageSourceArtifact
	_ = newSnapshotID

	if err := s.guardExecutionLease(ctx, req, runID); err != nil {
		return nil, err
	}

	actionOpts := actions.GenerateOptionsFromEnv()
	actionOpts.AIFirst = true
	actionOpts.AIEnabled = req.AIEnabled
	actionOpts.ForceActionsRegenerate = true
	actionOpts.PipelineRunID = &runID
	actionOpts.GlobalContextHash = gcHash
	diag.Actions.TimeoutSeconds = s.actionsRequestTimeoutSeconds()
	diag.Actions.MaxOutputTokens = actions.ActionsComposeEffectiveMaxOutputTokens()
	diag.Actions.RequestedMaxOutputTokens = actions.ActionsComposeRequestedMaxOutputTokens()
	diag.Actions.DataPackSource = stageRestartArtifactSourceMeta
	diag.Actions.SharedAnalysisSource = stageRestartArtifactSourceMeta
	diag.Actions.GlobalContextSource = stageRestartArtifactSourceMeta
	diag.Actions.DashboardSource = "cloned_from_source_pipeline"

	pack := sourceBundle.DataPack
	dataPackHash := sourceBundle.DataPackHash
	var input actions.ActionsAIFirstInput
	var buildErr error
	if SharedChunkAnalysisEnabled() {
		limitations := mergeGlobalContextLimitations(pack, sharedChunkRun)
		input, buildErr = s.actions.BuildActionsAIFirstInputFromCandidates(
			pack,
			dataPackHash,
			globalCtx,
			gcHash,
			sharedChunkRun.ValidatedActionCandidates,
			sharedChunkRun.FailedChunkIDs,
			limitations,
			actionOpts,
		)
		populateActionsDiagnosticsFromInput(&diag.Actions, input, "candidates")
	} else {
		input, buildErr = s.actions.BuildActionsAIFirstInputFromDataPack(pack, dataPackHash, globalCtx, actionOpts)
		populateActionsDiagnosticsFromInput(&diag.Actions, input, "legacy_data_pack")
	}
	if buildErr != nil {
		return s.failStageRestartRun(ctx, runID, result, "actions_input_build_failed", buildErr, artifacts, restartDiag)
	}

	wireBytes, wireErr := actions.MarshalActionsAIFirstInputWire(input)
	if wireErr != nil {
		return s.failStageRestartRun(ctx, runID, result, actions.ActionsErrorCode(wireErr), wireErr, artifacts, restartDiag)
	}
	actEst := EstimateBytesTokens(wireBytes)
	actMax := MaxAIInputTokensPerRequest()
	diag.Actions.InputEstimatedTokens = actEst
	diag.Actions.InputTokenLimit = actMax
	if inputHash, hashErr := actions.HashActionsAIFirstInputWire(input); hashErr == nil {
		diag.Actions.InputHash = inputHash
	}
	if req.DebugArtifacts {
		actInputMeta := aiArtifactMeta(runID, providers.Actions, "", actEst, actMax, map[string]any{
			"data_pack_hash":         dataPackHash,
			"global_context_hash":    gcHash,
			"input_schema":           input.SchemaVersion,
			"input_source":           diag.Actions.InputSource,
			"source_pipeline_run_id": req.SourcePipelineRunID,
			"artifact_source":        stageRestartArtifactSourceMeta,
			"restart_from_stage":     req.RestartFromStage,
		})
		_ = artifacts.SaveDocument(ctx, ArtifactKindActionsInput, actInputMeta, json.RawMessage(wireBytes))
	}

	actRes, actErr := s.actions.GenerateActionsAIFirstFromInput(ctx, input, actionOpts)
	composeCalls := 0
	actionsAICalls := 0
	if actRes != nil {
		composeCalls = actRes.ComposeCalls
		actionsAICalls = actRes.ComposeCalls + actRes.ObjectDetailCalls
	}
	restartDiag.ActionsAICalls = actionsAICalls
	diag.Actions.ComposeCalls = composeCalls

	var actionsOK, actionsFailed bool
	if actErr != nil {
		actionsFailed = true
		result.ActionsStatus = StepStatusFailed
		result.ErrorMessage = actErr.Error()
		if code := actions.ActionsErrorCode(actErr); code != "" {
			result.ErrorCode = code
		}
		diag.Actions.Status = StepStatusFailed
	} else if actRes != nil {
		diag.Actions.FinalStatus = actRes.FinalStatus
		diag.Actions.FinalCount = actRes.AcceptedCount
		populateActionsDiagnosticsFromResult(&diag.Actions, actRes)
		if actRes.PipelineDiagnostics != nil {
			actions.ApplyActionsTokenDiagnostics(actRes.PipelineDiagnostics, actEst, actRes.InputTokens, actMax)
		}
		populateFinalTopActionsDiagnostics(&diag.Dashboard, actRes, nil)
		result.ActionsRunID = actRes.RunID
		if actRes.FinalStatus == actions.GenerationFinalStatusFailed {
			actionsFailed = true
			result.ActionsStatus = StepStatusFailed
			diag.Actions.Status = StepStatusFailed
			if actRes.FailureSubreason != "" {
				result.ErrorCode = actRes.FailureSubreason
			}
		} else if actions.IsEmptyActionFinalStatus(actRes.FinalStatus) {
			actionsOK = true
			result.ActionsStatus = "empty"
			diag.Actions.Status = "empty"
		} else if actions.IsPublishableActionFinalStatus(actRes.FinalStatus) {
			actionsOK = true
			if actRes.FinalStatus == actions.GenerationFinalStatusCompletedPartial {
				result.ActionsStatus = actions.GenerationFinalStatusCompletedPartial
				diag.Actions.Status = actions.GenerationFinalStatusCompletedPartial
			} else {
				result.ActionsStatus = StepStatusCompleted
				diag.Actions.Status = StepStatusCompleted
			}
		} else {
			actionsFailed = true
			result.ActionsStatus = StepStatusFailed
			diag.Actions.Status = StepStatusFailed
		}
		if actRes.RunID > 0 {
			_ = s.store.LinkActionsRun(ctx, actRes.RunID, runID, dataPackHash, gcHash)
		}
		if req.DebugArtifacts && actRes != nil {
			actionsMeta := aiArtifactMeta(runID, providers.Actions, "", actEst, actMax, map[string]any{
				"actions_run_id":         actRes.RunID,
				"data_pack_hash":         dataPackHash,
				"global_context_hash":    gcHash,
				"source_pipeline_run_id": req.SourcePipelineRunID,
				"artifact_source":        "generated",
				"restart_from_stage":     req.RestartFromStage,
			})
			_ = artifacts.SaveDocument(ctx, ArtifactKindActionsOutput, actionsMeta, buildActionsOutputArtifact(actRes))
			_ = artifacts.SaveDocument(ctx, ArtifactKindActionsValidation, actionsMeta, buildActionsValidationArtifact(actRes))
			usageStatus := "completed"
			if actRes.FinalStatus == actions.GenerationFinalStatusFailed {
				usageStatus = "validation_failed"
			} else if actRes.FinalStatus == actions.GenerationFinalStatusCompletedPartial {
				usageStatus = "completed_partial"
			}
			_ = artifacts.SaveDocument(ctx, ArtifactKindActionsRawOutput, actionsMeta, buildActionsRawOutputArtifact(actRes, "", usageStatus))
			saveTwoPhaseActionsArtifacts(ctx, artifacts, runID, actionsMeta, actRes)
		}
		if composeCalls > 0 {
			s.recordActionsSynthesisUsage(ctx, req.SellerAccountID, runID, providers.Actions, "", actRawResponse(actRes), result.ActionsStatus)
		}
	}

	dashboardOK := result.DashboardStatus != "" && result.DashboardStatus != StepStatusFailed
	finalStatus := resolvePipelineStatus(req, dashboardOK, actionsOK, false, actionsFailed)
	result.Status = finalStatus
	now := time.Now().UTC()
	if err := s.store.UpdatePipelineRun(ctx, UpdatePipelineRunParams{
		ID:                         runID,
		Status:                     finalStatus,
		GlobalContextSchemaVersion: aicontext.GlobalSemanticContextSchemaVersion,
		GlobalContextHash:          gcHash,
		GlobalContextStatus:        gcStatus,
		DashboardRunID:             result.DashboardRunID,
		ActionsRunID:               result.ActionsRunID,
		DashboardStatus:            result.DashboardStatus,
		ActionsStatus:              result.ActionsStatus,
		ErrorCode:                  result.ErrorCode,
		ErrorMessage:               result.ErrorMessage,
	}); err != nil {
		return nil, err
	}

	if shouldPublishPipeline(req, finalStatus, dashboardOK, actionsOK, false, actionsFailed, result) {
		if err := s.guardExecutionLease(ctx, req, runID); err != nil {
			return nil, err
		}
		if err := validatePipelineHashes(result, dataPackHash, gcHash); err != nil {
			finalStatus = PipelineStatusFailed
			result.Status = finalStatus
			result.ErrorCode = "hash_mismatch"
			result.ErrorMessage = err.Error()
		} else if _, err := s.publisher.PublishPipelineSnapshot(ctx, runID); err != nil {
			finalStatus = PipelineStatusFailed
			result.Status = finalStatus
			result.ErrorCode = "publication_failed"
			result.ErrorMessage = err.Error()
		} else {
			result.Published = true
		}
	}

	summary := buildStageRestartPipelineSummary(finalStatus, req, restartDiag, composeCalls)
	_ = artifacts.SaveDocument(ctx, ArtifactKindPipelineSummary, stageRestartMeta(0), summary)
	result.ArtifactRefs = artifacts.Refs()
	result.Restart = restartDiag

	if result.Published {
		result.ErrorCode = ""
		result.ErrorMessage = ""
	}
	_ = s.store.UpdatePipelineRun(ctx, UpdatePipelineRunParams{
		ID:              runID,
		Status:          finalStatus,
		DashboardRunID:  result.DashboardRunID,
		ActionsRunID:    result.ActionsRunID,
		DashboardStatus: result.DashboardStatus,
		ActionsStatus:   result.ActionsStatus,
		ErrorCode:       result.ErrorCode,
		ErrorMessage:    result.ErrorMessage,
		CompletedAt:     ptrTime(now),
	})
	finalizeStageRestartUsageDiagnostics(&diag)
	return result, nil
}

func stageRestartArtifactMeta(targetRunID, sourceRunID int64, restartFromStage, dataPackHash string) map[string]any {
	return pipelineArtifactMeta(targetRunID, map[string]any{
		"source_pipeline_run_id": sourceRunID,
		"artifact_source":        stageRestartArtifactSourceMeta,
		"restart_from_stage":     restartFromStage,
		"data_pack_hash":         dataPackHash,
	})
}

func (s *SellerAIPipelineService) failStageRestartRun(
	ctx context.Context,
	runID int64,
	result *RunSellerAIPipelineResult,
	code string,
	err error,
	artifacts *ArtifactRecorder,
	restartDiag *StageRestartDiagnostics,
) (*RunSellerAIPipelineResult, error) {
	result.Status = PipelineStatusFailed
	result.ErrorCode = code
	if err != nil {
		result.ErrorMessage = err.Error()
	}
	result.Restart = restartDiag
	now := time.Now().UTC()
	_ = s.store.UpdatePipelineRun(ctx, UpdatePipelineRunParams{
		ID:           runID,
		Status:       PipelineStatusFailed,
		ErrorCode:    code,
		ErrorMessage: result.ErrorMessage,
		CompletedAt:  ptrTime(now),
	})
	if artifacts != nil {
		result.ArtifactRefs = artifacts.Refs()
	}
	return result, nil
}

func (s *SellerAIPipelineService) persistCopiedSharedChunkArtifacts(
	ctx context.Context,
	sourceRunID int64,
	targetRunID int64,
	pack *aidatapack.SellerAIDataPack,
	dataPackHash string,
	metricDate time.Time,
	chunkSet *aidatapack.ChunkSet,
	chunkRun *dashboardhome.ChunkAnalysisRunResult,
	artifacts *ArtifactRecorder,
	stageRestartMeta func(sourceArtifactID int64) map[string]any,
) error {
	sourceArtifactID, doc, err := s.loadLatestArtifactDocument(ctx, sourceRunID, ArtifactKindSharedChunkAnalysis)
	if err != nil {
		return err
	}
	payload := doc.Payload
	if payload == nil {
		chunkHashes, hashErr := orderedChunkHashes(chunkSet)
		if hashErr != nil {
			return hashErr
		}
		limitations := mergeGlobalContextLimitations(pack, chunkRun)
		artifact, buildErr := buildSharedChunkAnalysisArtifact(pack, dataPackHash, chunkSet, chunkHashes, chunkRun, limitations)
		if buildErr != nil {
			return buildErr
		}
		payload = scrubArtifactPayload(artifact)
	}
	meta := stageRestartMeta(sourceArtifactID)
	meta["data_pack_hash"] = dataPackHash
	meta["metric_date"] = metricDate.Format("2006-01-02")
	if err := artifacts.SaveRequiredDocument(ctx, ArtifactKindSharedChunkAnalysis, meta, payload); err != nil {
		return fmt.Errorf("persist copied shared chunk analysis: %w", err)
	}
	_ = artifacts.SaveDocument(ctx, ArtifactKindChunkSet, meta, chunkSet)
	return nil
}

func (s *SellerAIPipelineService) persistCopiedGlobalContextArtifact(
	ctx context.Context,
	sourceRunID int64,
	targetRunID int64,
	dataPackHash string,
	artifacts *ArtifactRecorder,
	stageRestartMeta func(sourceArtifactID int64) map[string]any,
) error {
	sourceArtifactID, doc, err := s.loadLatestArtifactDocument(ctx, sourceRunID, ArtifactKindGlobalContextFinal)
	if err != nil {
		return err
	}
	if doc.Payload == nil {
		return fmt.Errorf("source global context artifact payload is missing")
	}
	meta := stageRestartMeta(sourceArtifactID)
	meta["data_pack_hash"] = dataPackHash
	return artifacts.SaveRequiredDocument(ctx, ArtifactKindGlobalContextFinal, meta, doc.Payload)
}

func (s *SellerAIPipelineService) loadLatestArtifactDocument(
	ctx context.Context,
	pipelineRunID int64,
	kind string,
) (int64, ArtifactDocument, error) {
	arts, err := s.store.ListArtifacts(ctx, pipelineRunID)
	if err != nil {
		return 0, ArtifactDocument{}, err
	}
	var latest *dbgen.AiPipelineArtifact
	for i := range arts {
		if arts[i].Kind == kind {
			cp := arts[i]
			latest = &cp
		}
	}
	if latest == nil {
		return 0, ArtifactDocument{}, fmt.Errorf("artifact kind %q not found for pipeline %d", kind, pipelineRunID)
	}
	doc, err := artifactDocumentPayload(latest.PayloadJson)
	if err != nil {
		return 0, ArtifactDocument{}, err
	}
	return latest.ID, doc, nil
}

func buildStageRestartPipelineSummary(
	finalStatus string,
	req RunSellerAIPipelineRequest,
	restart *StageRestartDiagnostics,
	actionsCalls int,
) map[string]any {
	return map[string]any{
		"status":                 finalStatus,
		"source_pipeline_run_id": req.SourcePipelineRunID,
		"restart_mode":           stageRestartModeDerivedRun,
		"restart_from_stage":     req.RestartFromStage,
		"reused_stages": []string{
			"data_pack",
			"chunk_set",
			"shared_chunk_analysis",
			"global_context",
		},
		"cloned_stages": []string{"dashboard"},
		"executed_stages": []string{
			"actions",
			"publication",
		},
		"ai_calls": map[string]any{
			"shared_chunk_analysis": 0,
			"global_context":        0,
			"dashboard":             0,
			"actions":               actionsCalls,
		},
		"restart": restart,
	}
}
