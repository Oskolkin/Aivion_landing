package aipipeline

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicontext"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aihash"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

type globalContextRestartSourceBundle struct {
	Run            dbgen.AiPipelineRun
	DataPack       *aidatapack.SellerAIDataPack
	DataPackHash   string
	ChunkSet       *aidatapack.ChunkSet
	SharedAnalysis *dashboardhome.ChunkAnalysisRunResult
}

func (s *SellerAIPipelineService) loadGlobalContextStageRestartSource(
	ctx context.Context,
	req RunSellerAIPipelineRequest,
) (globalContextRestartSourceBundle, string, error) {
	out := globalContextRestartSourceBundle{}
	if s == nil || s.store == nil {
		return out, "", fmt.Errorf("seller ai pipeline service is not configured")
	}
	sourceRun, err := s.store.GetPipelineRun(ctx, req.SourcePipelineRunID)
	if err != nil {
		return out, "stage_restart_source_not_found", fmt.Errorf("source pipeline run %d: %w", req.SourcePipelineRunID, err)
	}
	out.Run = sourceRun
	if sourceRun.SellerAccountID <= 0 {
		return out, "stage_restart_source_not_found", fmt.Errorf("source pipeline run %d has invalid seller_account_id", req.SourcePipelineRunID)
	}
	if req.SellerAccountID > 0 && sourceRun.SellerAccountID != req.SellerAccountID {
		return out, "stage_restart_source_not_found", fmt.Errorf("source seller_account_id mismatch")
	}
	if !sourceRun.MetricDate.Valid {
		return out, "stage_restart_source_not_found", fmt.Errorf("source metric_date missing")
	}
	if strings.TrimSpace(sourceRun.DataPackHash) == "" {
		return out, "stage_restart_data_pack_artifact_missing", fmt.Errorf("source data_pack_hash missing")
	}

	metricDateStr := sourceRun.MetricDate.Time.Format("2006-01-02")
	pack, err := s.tryLoadSavedDataPack(ctx, req.SourcePipelineRunID, sourceRun.SellerAccountID, metricDateStr, sourceRun.DataPackHash)
	if err != nil {
		return out, "stage_restart_data_pack_artifact_missing", err
	}
	if pack == nil {
		return out, "stage_restart_data_pack_artifact_missing", fmt.Errorf("source data_pack artifact missing")
	}
	packHash := aihash.HashDataPack(pack)
	if packHash != strings.TrimSpace(sourceRun.DataPackHash) {
		return out, "stage_restart_hash_mismatch", fmt.Errorf("data_pack_hash mismatch")
	}
	out.DataPack = pack
	out.DataPackHash = packHash

	if !SharedChunkAnalysisEnabled() {
		return out, "stage_restart_shared_analysis_artifact_missing", fmt.Errorf("shared chunk analysis required")
	}
	chunkSet, sharedRun, code, sharedErr := s.loadSourceSharedChunkAnalysis(ctx, req.SourcePipelineRunID, sourceRun, pack, packHash, metricDateStr)
	if sharedErr != nil {
		return out, code, sharedErr
	}
	out.ChunkSet = chunkSet
	out.SharedAnalysis = sharedRun
	return out, "", nil
}

func (s *SellerAIPipelineService) runGlobalContextStageRestart(
	ctx context.Context,
	req RunSellerAIPipelineRequest,
) (*RunSellerAIPipelineResult, error) {
	if err := ValidatePipelineConfig(); err != nil {
		return nil, err
	}
	if s == nil || s.store == nil || s.gcClient == nil {
		return nil, fmt.Errorf("seller ai pipeline service is not configured")
	}
	if !req.GenerateDashboard || !req.GenerateActions {
		return nil, fmt.Errorf("restart from global_context requires dashboard and actions")
	}

	sourceBundle, preflightCode, preflightErr := s.loadGlobalContextStageRestartSource(ctx, req)
	if preflightErr != nil {
		return &RunSellerAIPipelineResult{
			SourcePipelineRunID: req.SourcePipelineRunID,
			RestartFromStage:    req.RestartFromStage,
			ErrorCode:           preflightCode,
			ErrorMessage:        preflightErr.Error(),
			Status:              PipelineStatusFailed,
		}, nil
	}

	metricDate := stageRestartSourceMetricDate(sourceBundle.Run)
	if req.SellerAccountID <= 0 {
		req.SellerAccountID = sourceBundle.Run.SellerAccountID
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
		GlobalContextSource:  "generated",
		GlobalContextReused:  false,
		GlobalContextAICalls: 1,
		DashboardSource:      "generated",
		DashboardReused:      false,
		ActionsSource:        "generated",
		ActionsReused:        false,
	}
	diag.Restart = restartDiag
	result.Diagnostics = &diag

	runID, err := s.store.CreatePipelineRun(ctx, CreatePipelineRunParams{
		SellerAccountID:       sourceBundle.Run.SellerAccountID,
		MetricDate:            metricDate,
		Mode:                  PipelineModeUnified,
		Trigger:               stageRestartTriggerOrDefault(req.Trigger),
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
	if err := s.persistCopiedSharedChunkArtifacts(ctx, req.SourcePipelineRunID, runID, sourceBundle.DataPack, sourceBundle.DataPackHash, metricDate, sourceBundle.ChunkSet, sourceBundle.SharedAnalysis, artifacts, stageRestartMeta); err != nil {
		return s.failStageRestartRun(ctx, runID, result, "stage_restart_shared_analysis_artifact_missing", err, artifacts, restartDiag)
	}
	populateSharedChunkDiagnostics(&diag, sourceBundle.SharedAnalysis, stageRestartArtifactSourceMeta, "", sourceBundle.ChunkSet)

	gcInput, err := BuildGlobalContextGenerationInput(sourceBundle.DataPack, sourceBundle.DataPackHash, sourceBundle.ChunkSet, sourceBundle.SharedAnalysis)
	if err != nil {
		return s.failStageRestartRun(ctx, runID, result, "global_context_input_build_failed", err, artifacts, restartDiag)
	}
	gcInputHash, err := HashGlobalContextGenerationInput(gcInput)
	if err != nil {
		return s.failStageRestartRun(ctx, runID, result, "global_context_input_build_failed", err, artifacts, restartDiag)
	}
	populateGlobalContextDiagnosticsFromInput(&diag, gcInput, gcInputHash)

	globalCtx, gcStatus, gcHash, gcErr := s.generateGlobalContext(ctx, req, runID, sourceBundle.DataPackHash, gcInput, gcInputHash, artifacts, &diag, providers)
	if gcErr != nil {
		result.GlobalContextStatus = gcStatus
		result.ErrorCode = "global_context_failed"
		result.ErrorMessage = gcErr.Error()
		if diag.GlobalContext.FailureSubreason != "" {
			result.ErrorCode = diag.GlobalContext.FailureSubreason
		}
		result.Status = PipelineStatusContextFailed
		_ = s.store.UpdatePipelineRun(ctx, UpdatePipelineRunParams{
			ID: runID, Status: PipelineStatusContextFailed, GlobalContextStatus: gcStatus,
			ErrorCode: result.ErrorCode, ErrorMessage: result.ErrorMessage, CompletedAt: ptrTime(time.Now().UTC()),
		})
		result.ArtifactRefs = artifacts.Refs()
		result.Restart = restartDiag
		return result, nil
	}

	result.GlobalContextHash = gcHash
	result.GlobalContextStatus = gcStatus
	return s.runDerivedDownstreamAfterGlobalContext(ctx, req, runID, sourceBundle.DataPack, sourceBundle.DataPackHash, sourceBundle.ChunkSet, sourceBundle.SharedAnalysis, globalCtx, gcHash, artifacts, providers, &diag, restartDiag, result)
}

func (s *SellerAIPipelineService) runSharedChunkValidationStageRestart(
	ctx context.Context,
	req RunSellerAIPipelineRequest,
) (*RunSellerAIPipelineResult, error) {
	if err := ValidatePipelineConfig(); err != nil {
		return nil, err
	}
	sourceBundle, preflightCode, preflightErr := s.loadGlobalContextStageRestartSource(ctx, req)
	if preflightErr != nil {
		return &RunSellerAIPipelineResult{
			SourcePipelineRunID: req.SourcePipelineRunID,
			RestartFromStage:    req.RestartFromStage,
			ErrorCode:           preflightCode,
			ErrorMessage:        preflightErr.Error(),
			Status:              PipelineStatusFailed,
		}, nil
	}
	rawCalls, err := s.loadSourceChunkRawResponses(ctx, req.SourcePipelineRunID)
	if err != nil {
		return &RunSellerAIPipelineResult{
			SourcePipelineRunID: req.SourcePipelineRunID,
			RestartFromStage:    req.RestartFromStage,
			ErrorCode:           "stage_restart_chunk_raw_response_missing",
			ErrorMessage:        err.Error(),
			Status:              PipelineStatusFailed,
		}, nil
	}
	revalidated, rejectionReport, revalErr := dashboardhome.RevalidateSharedChunkFromRawResponses(sourceBundle.DataPack, sourceBundle.ChunkSet, rawCalls)
	if revalErr != nil {
		return &RunSellerAIPipelineResult{
			SourcePipelineRunID: req.SourcePipelineRunID,
			ErrorCode:           "shared_chunk_revalidation_failed",
			ErrorMessage:        revalErr.Error(),
			Status:              PipelineStatusFailed,
		}, nil
	}
	coverage := EvaluateSharedChunkCoverage(sourceBundle.ChunkSet, &revalidated)
	if !coverage.Passed {
		return &RunSellerAIPipelineResult{
			SourcePipelineRunID: req.SourcePipelineRunID,
			ErrorCode:           "shared_chunk_coverage_insufficient",
			ErrorMessage:        formatSharedChunkCoverageError(coverage),
			Status:              PipelineStatusFailed,
		}, nil
	}
	sourceBundle.SharedAnalysis = &revalidated
	_ = rejectionReport

	req.RestartFromStage = ResumeStageGlobalContext
	return s.runGlobalContextStageRestart(ctx, req)
}

func (s *SellerAIPipelineService) loadSourceChunkRawResponses(ctx context.Context, sourceRunID int64) ([]dashboardhome.ChunkRawCallInput, error) {
	artifacts, err := s.store.ListArtifacts(ctx, sourceRunID)
	if err != nil {
		return nil, err
	}
	for _, art := range artifacts {
		if art.Kind != ArtifactKindChunkRawResponse {
			continue
		}
		doc, docErr := artifactDocumentPayload(art.PayloadJson)
		if docErr != nil {
			return nil, docErr
		}
		payload, ok := doc.Payload.(map[string]any)
		if !ok {
			continue
		}
		callsRaw, ok := payload["calls"].([]any)
		if !ok {
			return nil, fmt.Errorf("chunk_raw_response artifact has no calls")
		}
		out := make([]dashboardhome.ChunkRawCallInput, 0, len(callsRaw))
		for _, item := range callsRaw {
			row, ok := item.(map[string]any)
			if !ok {
				continue
			}
			chunkID, _ := row["chunk_id"].(string)
			rawBody, _ := row["raw_body"].(string)
			if rawBody == "" {
				rawBody, _ = row["body"].(string)
			}
			out = append(out, dashboardhome.ChunkRawCallInput{ChunkID: chunkID, RawBody: rawBody})
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("chunk_raw_response calls empty")
		}
		return out, nil
	}
	return nil, fmt.Errorf("chunk_raw_response artifact not found for run %d", sourceRunID)
}

func (s *SellerAIPipelineService) runDerivedDownstreamAfterGlobalContext(
	ctx context.Context,
	req RunSellerAIPipelineRequest,
	runID int64,
	pack *aidatapack.SellerAIDataPack,
	dataPackHash string,
	chunkSet *aidatapack.ChunkSet,
	sharedChunkRun *dashboardhome.ChunkAnalysisRunResult,
	globalCtx *aicontext.GlobalSemanticContext,
	gcHash string,
	artifacts *ArtifactRecorder,
	providers PipelineProviders,
	diag *PipelineDiagnostics,
	restartDiag *StageRestartDiagnostics,
	result *RunSellerAIPipelineResult,
) (*RunSellerAIPipelineResult, error) {
	metricDate := req.MetricDate
	var dashboardOK, actionsOK bool
	var dashboardFailed, actionsFailed bool
	var dashOutcome *DashboardRunOutcome

	if s.dashboard != nil {
		dashOpts := dashboardhome.NarrativeRunOptions{
			MetricDate:          &metricDate,
			DataPackOverride:    pack,
			DataPackHash:        dataPackHash,
			GlobalContext:       globalCtx,
			GlobalContextHash:   gcHash,
			PipelineRunID:       &runID,
			ForceRegenerate:     true,
			ChunkSetOverride:    chunkSet,
			PrecomputedChunkRun: sharedChunkRun,
		}
		var dashErr error
		dashOutcome, dashErr = s.dashboard.RunForSellerMetricDateWithOptions(ctx, req.SellerAccountID, metricDate, dashOpts)
		populateDashboardDiagnostics(diag, dashOutcome)
		if dashErr != nil || dashOutcome == nil || dashOutcome.Result == nil || !dashOutcome.Result.IsSuccess() {
			dashboardFailed = true
			result.DashboardStatus = StepStatusFailed
			diag.Dashboard.Status = StepStatusFailed
			restartDiag.DashboardAICalls = 0
		} else {
			dashboardOK = true
			result.DashboardRunID = dashOutcome.Result.RunID
			result.DashboardStatus = StepStatusCompleted
			diag.Dashboard.Status = StepStatusCompleted
			restartDiag.DashboardAICalls = 1
			restartDiag.DashboardRunID = dashOutcome.Result.RunID
			if dashOutcome.Result.RunID != "" {
				_ = s.store.LinkDashboardRun(ctx, dashOutcome.Result.RunID, req.SellerAccountID, runID, dataPackHash, gcHash)
			}
		}
	}

	if s.actions != nil && globalCtx != nil {
		actionOpts := actions.GenerateOptionsFromEnv()
		actionOpts.AIFirst = true
		actionOpts.AIEnabled = req.AIEnabled
		actionOpts.ForceRegenerate = true
		actionOpts.PipelineRunID = &runID
		actionOpts.GlobalContextHash = gcHash
		limitations := mergeGlobalContextLimitations(pack, sharedChunkRun)
		input, buildErr := s.actions.BuildActionsAIFirstInputFromCandidates(
			pack, dataPackHash, globalCtx, gcHash,
			sharedChunkRun.ValidatedActionCandidates, sharedChunkRun.FailedChunkIDs, limitations, actionOpts,
		)
		if buildErr != nil {
			actionsFailed = true
			result.ActionsStatus = StepStatusFailed
			result.ErrorMessage = buildErr.Error()
		} else {
			actRes, actErr := s.actions.GenerateActionsAIFirstFromInput(ctx, input, actionOpts)
			actionsAICalls := 0
			if actRes != nil {
				actionsAICalls = actRes.ComposeCalls + actRes.ObjectDetailCalls
			}
			restartDiag.ActionsAICalls = actionsAICalls
			if actErr != nil {
				actionsFailed = true
				result.ActionsStatus = StepStatusFailed
				result.ErrorMessage = actErr.Error()
				if code := actions.ActionsErrorCode(actErr); code != "" {
					result.ErrorCode = code
				}
			} else if actRes != nil {
				result.ActionsRunID = actRes.RunID
				if actRes.FinalStatus == actions.GenerationFinalStatusFailed {
					actionsFailed = true
					result.ActionsStatus = StepStatusFailed
					if actRes.FailureSubreason != "" {
						result.ErrorCode = actRes.FailureSubreason
					}
				} else if actions.IsPublishableActionFinalStatus(actRes.FinalStatus) {
					actionsOK = true
					result.ActionsStatus = actRes.FinalStatus
					if actRes.RunID > 0 {
						_ = s.store.LinkActionsRun(ctx, actRes.RunID, runID, dataPackHash, gcHash)
					}
				}
			}
		}
	}

	finalStatus := resolvePipelineStatus(req, dashboardOK, actionsOK, dashboardFailed, actionsFailed)
	if result.ErrorCode == actions.ActionsErrorAllDropped {
		finalStatus = PipelineStatusActionsAllDropped
	}
	result.Status = finalStatus
	_ = s.store.UpdatePipelineRun(ctx, UpdatePipelineRunParams{
		ID: runID, Status: finalStatus, GlobalContextHash: gcHash, GlobalContextStatus: GlobalContextStatusCompleted,
		DashboardRunID: result.DashboardRunID, ActionsRunID: result.ActionsRunID,
		DashboardStatus: result.DashboardStatus, ActionsStatus: result.ActionsStatus,
		ErrorCode: result.ErrorCode, ErrorMessage: result.ErrorMessage, CompletedAt: ptrTime(time.Now().UTC()),
	})
	result.ArtifactRefs = artifacts.Refs()
	result.Restart = restartDiag
	return result, nil
}

func stageRestartTriggerOrDefault(trigger string) string {
	if strings.TrimSpace(trigger) != "" {
		return trigger
	}
	return "devtool_stage_restart"
}
