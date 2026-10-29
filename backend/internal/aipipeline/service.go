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
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aihash"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/openaix"
	"github.com/jackc/pgx/v5/pgtype"
)

type DataPackBuilder interface {
	Build(ctx context.Context, sellerAccountID int64, metricDate time.Time) (*aidatapack.SellerAIDataPack, error)
}

type DashboardRunner interface {
	RunForSellerMetricDateWithOptions(
		ctx context.Context,
		sellerAccountID int64,
		metricDate time.Time,
		opts dashboardhome.NarrativeRunOptions,
	) (*DashboardRunOutcome, error)
}

// ChunkAnalysisRunner analyzes a pre-built Data Pack chunk set without dashboard synthesis.
type ChunkAnalysisRunner interface {
	AnalyzeDataPackChunks(
		ctx context.Context,
		pack *aidatapack.SellerAIDataPack,
		chunkSet *aidatapack.ChunkSet,
		opts dashboardhome.NarrativeRunOptions,
	) (*dashboardhome.ChunkAnalysisRunResult, error)
}

type ActionsRunner interface {
	BuildActionsAIFirstInputFromDataPack(
		pack *aidatapack.SellerAIDataPack,
		dataPackHash string,
		globalContext *aicontext.GlobalSemanticContext,
		opts actions.GenerateActionsOptions,
	) (actions.ActionsAIFirstInput, error)
	BuildActionsAIFirstInputFromCandidates(
		pack *aidatapack.SellerAIDataPack,
		dataPackHash string,
		globalContext *aicontext.GlobalSemanticContext,
		globalContextHash string,
		candidates []actions.AIFirstActionCandidate,
		failedChunkIDs []string,
		dataLimitations []string,
		opts actions.GenerateActionsOptions,
	) (actions.ActionsAIFirstInput, error)
	GenerateActionsAIFirstFromInput(
		ctx context.Context,
		input actions.ActionsAIFirstInput,
		opts actions.GenerateActionsOptions,
	) (*actions.GenerateActionsResult, error)
}

type PipelineStore interface {
	CreatePipelineRun(ctx context.Context, params CreatePipelineRunParams) (int64, error)
	UpdatePipelineRun(ctx context.Context, params UpdatePipelineRunParams) error
	CreateArtifact(ctx context.Context, params CreateArtifactParams) (int64, error)
	LinkDashboardRun(ctx context.Context, narrativeID string, sellerID, pipelineRunID int64, dataPackHash, globalContextHash string) error
	GetDashboardNarrativeHashes(ctx context.Context, narrativeID string, sellerAccountID int64) (dataPackHash, globalContextHash string, err error)
	GetActionGenerationRunHashes(ctx context.Context, actionsRunID, sellerAccountID int64) (dataPackHash, globalContextHash string, err error)
	GetActionGenerationRunFinalStatus(ctx context.Context, actionsRunID, sellerAccountID int64) (string, error)
	LinkActionsRun(ctx context.Context, actionsRunID, pipelineRunID int64, dataPackHash, globalContextHash string) error
	GetPipelineRun(ctx context.Context, id int64) (dbgen.AiPipelineRun, error)
	ListArtifacts(ctx context.Context, pipelineRunID int64) ([]dbgen.AiPipelineArtifact, error)
}

type SellerAIPipelineService struct {
	dataPack              DataPackBuilder
	dashboard             DashboardRunner
	chunkAnalyzer         ChunkAnalysisRunner
	buildChunkSet         chunkSetBuildFunc
	actions               ActionsRunner
	store                 PipelineStore
	publisher             SnapshotPublisher
	gcClient              GlobalContextAIClient
	usage                 aiUsageRecorder
	actionsTimeoutSeconds int
}

func NewSellerAIPipelineService(
	dataPack DataPackBuilder,
	dashboard DashboardRunner,
	actions ActionsRunner,
	store PipelineStore,
	gcClient GlobalContextAIClient,
) *SellerAIPipelineService {
	return &SellerAIPipelineService{
		dataPack:  dataPack,
		dashboard: dashboard,
		actions:   actions,
		store:     store,
		gcClient:  gcClient,
	}
}

func (s *SellerAIPipelineService) SetSnapshotPublisher(publisher SnapshotPublisher) {
	s.publisher = publisher
}

func (s *SellerAIPipelineService) SetChunkAnalysisRunner(runner ChunkAnalysisRunner) {
	s.chunkAnalyzer = runner
}

func (s *SellerAIPipelineService) SetActionsTimeoutSeconds(seconds int) {
	if s == nil {
		return
	}
	s.actionsTimeoutSeconds = seconds
}

func (s *SellerAIPipelineService) actionsRequestTimeoutSeconds() int {
	if s != nil && s.actionsTimeoutSeconds > 0 {
		return s.actionsTimeoutSeconds
	}
	return openaix.DefaultActionsTimeoutSeconds
}

func (s *SellerAIPipelineService) Run(
	ctx context.Context,
	req RunSellerAIPipelineRequest,
) (*RunSellerAIPipelineResult, error) {
	if err := ValidatePipelineConfig(); err != nil {
		return nil, err
	}
	if pipelineStageRestartMode(req) {
		switch req.RestartFromStage {
		case ResumeStageActions:
			return s.runActionsStageRestart(ctx, req)
		case ResumeStageGlobalContext:
			return s.runGlobalContextStageRestart(ctx, req)
		case ResumeStageSharedChunkValidation, ResumeStageSharedChunkAnalysis:
			return s.runSharedChunkValidationStageRestart(ctx, req)
		default:
			return nil, fmt.Errorf("unsupported restart-from-stage %q", req.RestartFromStage)
		}
	}
	if s == nil || s.dataPack == nil || s.store == nil {
		return nil, fmt.Errorf("seller ai pipeline service is not configured")
	}
	if req.SellerAccountID <= 0 {
		return nil, fmt.Errorf("seller_account_id must be > 0")
	}
	metricDate := time.Date(req.MetricDate.Year(), req.MetricDate.Month(), req.MetricDate.Day(), 0, 0, 0, 0, time.UTC)
	trigger := req.Trigger
	if trigger == "" {
		trigger = "manual"
	}

	result := &RunSellerAIPipelineResult{
		SellerAccountID: req.SellerAccountID,
		MetricDate:      metricDate.Format("2006-01-02"),
		ArtifactRefs:    map[string]string{},
	}

	providers := req.ResolvedProviders
	if providers.GlobalContext == "" {
		providers = DefaultProvidersFromRequest(req)
	}
	result.Providers = &providers
	diag := newPipelineDiagnostics(providers)
	result.Diagnostics = &diag

	if SharedChunkAnalysisEnabled() && s.chunkAnalyzer == nil {
		return s.failBeforeRun(
			ctx,
			result,
			"shared_chunk_analysis_runner_not_configured",
			fmt.Errorf("shared chunk analysis is enabled but ChunkAnalysisRunner is not configured"),
		)
	}

	var earlyPersistedRun *dbgen.AiPipelineRun
	var resumeDataPackHashHint string
	var resumeGlobalContextHashHint string
	if req.ExistingPipelineRunID > 0 {
		if err := s.guardExecutionLease(ctx, req, req.ExistingPipelineRunID); err != nil {
			return nil, err
		}
		earlyRun, earlyErr := s.store.GetPipelineRun(ctx, req.ExistingPipelineRunID)
		if earlyErr != nil {
			return nil, earlyErr
		}
		if earlyRun.PublishedAt.Valid {
			result.PipelineRunID = earlyRun.ID
			result.Published = true
			result.Status = earlyRun.Status
			result.Reused = true
			return result, nil
		}
		cp := earlyRun
		earlyPersistedRun = &cp
		if earlyRun.ActionsRunID.Valid && earlyRun.ActionsRunID.Int64 > 0 {
			linkedStatus, statusErr := s.store.GetActionGenerationRunFinalStatus(ctx, earlyRun.ActionsRunID.Int64, req.SellerAccountID)
			if statusErr == nil && actions.IsSuccessfulActionFinalStatus(linkedStatus) {
				dp, gch, actErr := s.store.GetActionGenerationRunHashes(ctx, earlyRun.ActionsRunID.Int64, req.SellerAccountID)
				if actErr == nil {
					resumeDataPackHashHint = strings.TrimSpace(dp)
					resumeGlobalContextHashHint = strings.TrimSpace(gch)
				}
			}
		} else if persistedDashboardCompleted(earlyRun) {
			dashID := uuidString(earlyRun.DashboardRunID)
			if dashID != "" {
				dp, _, hashErr := s.store.GetDashboardNarrativeHashes(ctx, dashID, req.SellerAccountID)
				if hashErr == nil {
					resumeDataPackHashHint = strings.TrimSpace(dp)
				}
			}
		}
	}

	pack, dataPackSource, err := s.loadPipelinePackForRun(ctx, req, metricDate, earlyPersistedRun, resumeDataPackHashHint)
	if err != nil {
		code := "data_pack_build_failed"
		if pipelineResumeMode(req) {
			code = "resume_data_pack_artifact_missing"
		}
		return s.failBeforeRun(ctx, result, code, err)
	}
	if err := aidatapack.ValidateDataPack(pack); err != nil {
		return s.failBeforeRun(ctx, result, "data_pack_invalid", err)
	}
	if !pipelineResumeMode(req) && !req.ForceRegenerate && req.ExistingPipelineRunID > 0 {
		savedPack, loadErr := s.tryLoadSavedDataPack(ctx, req.ExistingPipelineRunID, req.SellerAccountID, metricDate.Format("2006-01-02"), resumeDataPackHashHint)
		if loadErr != nil {
			return s.failBeforeRun(ctx, result, "data_pack_load_failed", loadErr)
		}
		if savedPack != nil {
			pack = savedPack
			dataPackSource = "artifact"
		}
	}
	if pipelineResumeMode(req) && earlyPersistedRun != nil {
		if err := validateResumePipelineRun(req, *earlyPersistedRun, metricDate); err != nil {
			return s.failBeforeRun(ctx, result, "resume_pipeline_run_mismatch", err)
		}
	}

	dataPackHash := aihash.HashDataPack(pack)
	if dataPackHash == "" {
		return s.failBeforeRun(ctx, result, "data_pack_hash_failed", fmt.Errorf("empty data pack hash"))
	}
	if pipelineResumeMode(req) && earlyPersistedRun != nil {
		if err := validateResumeDataPackHash(*earlyPersistedRun, dataPackHash); err != nil {
			return s.failBeforeRun(ctx, result, "resume_data_pack_hash_mismatch", err)
		}
	}
	result.DataPackHash = dataPackHash
	diag.Resume = initResumeDiagnostics(req, dataPackSource)
	syncResumeResult(result, &diag)

	if !req.ForceRegenerate && req.ExistingPipelineRunID <= 0 {
		if existing, ok := s.tryReusePublishedPipeline(ctx, req.SellerAccountID, metricDate, dataPackHash); ok {
			result.Reused = true
			result.Published = true
			result.PipelineRunID = existing.ID
			result.Status = existing.Status
			result.GlobalContextHash = textOrEmpty(existing.GlobalContextHash)
			result.GlobalContextStatus = textOrEmpty(existing.GlobalContextStatus)
			if existing.DashboardRunID.Valid {
				result.DashboardRunID = uuidString(existing.DashboardRunID)
			}
			if existing.ActionsRunID.Valid {
				result.ActionsRunID = existing.ActionsRunID.Int64
			}
			result.DashboardStatus = textOrEmpty(existing.DashboardStatus)
			result.ActionsStatus = textOrEmpty(existing.ActionsStatus)
			result.MetricDate = metricDate.Format("2006-01-02")
			return result, nil
		}
	}

	var runID int64
	var persistedRun dbgen.AiPipelineRun
	var hasPersistedRun bool

	if req.ExistingPipelineRunID > 0 {
		if earlyPersistedRun == nil {
			return nil, fmt.Errorf("resume pipeline run %d was not loaded", req.ExistingPipelineRunID)
		}
		persistedRun = *earlyPersistedRun
		runID = persistedRun.ID
		hasPersistedRun = true
		result.PipelineRunID = runID
		if err := s.store.UpdatePipelineRun(ctx, UpdatePipelineRunParams{
			ID:                    runID,
			Status:                PipelineStatusRunning,
			DataPackSchemaVersion: pack.SchemaVersion,
			DataPackHash:          dataPackHash,
			ErrorCode:             "",
			ErrorMessage:          "",
		}); err != nil {
			return nil, err
		}
	} else {
		runID, err = s.store.CreatePipelineRun(ctx, CreatePipelineRunParams{
			SellerAccountID:       req.SellerAccountID,
			MetricDate:            metricDate,
			Mode:                  PipelineModeUnified,
			Trigger:               trigger,
			Status:                PipelineStatusRunning,
			DataPackSchemaVersion: pack.SchemaVersion,
			DataPackHash:          dataPackHash,
			StartedAt:             time.Now().UTC(),
		})
		if err != nil {
			return nil, err
		}
		result.PipelineRunID = runID
	}

	artifacts := NewArtifactRecorder(s.store, runID, req.DebugArtifacts)
	if err := artifacts.SaveRequiredDocument(ctx, ArtifactKindDataPack, pipelineArtifactMeta(runID, map[string]any{
		"seller_account_id": req.SellerAccountID,
		"metric_date":       metricDate.Format("2006-01-02"),
		"data_pack_hash":    dataPackHash,
	}), pack); err != nil {
		return s.failRun(ctx, runID, result, PipelineStatusFailed, "artifact_save_failed", err, artifacts)
	}
	diagMeta := pipelineArtifactMetaWithHashes(runID, req.SellerAccountID, metricDate.Format("2006-01-02"), dataPackHash, "")
	_ = artifacts.SaveRequiredDocument(ctx, ArtifactKindMetricSemanticsSnapshot, diagMeta, buildMetricSemanticsSnapshotArtifact(collectUsedMetricCodesFromPack(pack)))

	var sharedChunkSet *aidatapack.ChunkSet
	var sharedChunkRun *dashboardhome.ChunkAnalysisRunResult
	if SharedChunkAnalysisEnabled() {
		if err := s.guardExecutionLease(ctx, req, runID); err != nil {
			return nil, err
		}
		var sharedErr error
		sharedChunkSet, sharedChunkRun, sharedErr = s.prepareSharedChunkAnalysis(
			ctx,
			runID,
			pack,
			dataPackHash,
			metricDate,
			req,
			artifacts,
			&diag,
			providers,
		)
		if sharedErr != nil {
			code, underlying := pipelineStageFailure(sharedErr)
			return s.failRun(ctx, runID, result, PipelineStatusFailed, code, underlying, artifacts)
		}
	}

	var gcInput GlobalContextGenerationInput
	var gcInputHash string
	if SharedChunkAnalysisEnabled() {
		if sharedChunkSet == nil || sharedChunkRun == nil {
			return s.failRun(ctx, runID, result, PipelineStatusFailed, "global_context_input_build_failed", fmt.Errorf("shared chunk analysis result is missing"), artifacts)
		}
		var buildErr error
		gcInput, buildErr = BuildGlobalContextGenerationInput(pack, dataPackHash, sharedChunkSet, sharedChunkRun)
		if buildErr != nil {
			return s.failRun(ctx, runID, result, PipelineStatusFailed, "global_context_input_build_failed", buildErr, artifacts)
		}
	} else {
		gcInput = DefaultGlobalContextGenerationInput(pack, dataPackHash)
	}
	gcInputHash, err = HashGlobalContextGenerationInput(gcInput)
	if err != nil {
		return s.failRun(ctx, runID, result, PipelineStatusFailed, "global_context_input_build_failed", err, artifacts)
	}
	populateGlobalContextDiagnosticsFromInput(&diag, gcInput, gcInputHash)

	var globalCtx *aicontext.GlobalSemanticContext
	var gcStatus, gcHash string
	var gcErr error
	loadedGC, loadErr := s.tryLoadSavedGlobalContext(ctx, runID, gcInput, gcInputHash, resumeGlobalContextHashHint)
	if loadErr != nil {
		return s.failRun(ctx, runID, result, PipelineStatusFailed, "global_context_load_failed", loadErr, artifacts)
	}
	if loadedGC.Context != nil {
		globalCtx = loadedGC.Context
		gcHash = loadedGC.ContextHash
		gcStatus = GlobalContextStatusCompleted
		populateGlobalContextDiagnosticsFromLoad(&diag, loadedGC)
	} else {
		if loadedGC.MissReason != "" {
			diag.GlobalContext.ArtifactMissReason = loadedGC.MissReason
		}
		if err := s.guardExecutionLease(ctx, req, runID); err != nil {
			return nil, err
		}
		globalCtx, gcStatus, gcHash, gcErr = s.generateGlobalContext(ctx, req, runID, dataPackHash, gcInput, gcInputHash, artifacts, &diag, providers)
	}
	if gcErr != nil {
		if strings.Contains(gcErr.Error(), "global_context_persist_failed") {
			return s.failRun(ctx, runID, result, PipelineStatusFailed, "global_context_persist_failed", gcErr, artifacts)
		}
		diag.GlobalContext.Status = GlobalContextStatusFailed
		result.GlobalContextStatus = GlobalContextStatusFailed
		result.Status = PipelineStatusContextFailed
		result.ErrorCode = "global_context_failed"
		result.ErrorMessage = gcErr.Error()
		_ = s.store.UpdatePipelineRun(ctx, UpdatePipelineRunParams{
			ID:                  runID,
			Status:              PipelineStatusContextFailed,
			GlobalContextStatus: GlobalContextStatusFailed,
			ErrorCode:           result.ErrorCode,
			ErrorMessage:        result.ErrorMessage,
			CompletedAt:         ptrTime(time.Now().UTC()),
		})
		result.ArtifactRefs = artifacts.Refs()
		syncResumeResult(result, &diag)
		return result, nil
	}
	result.GlobalContextHash = gcHash
	result.GlobalContextStatus = gcStatus
	diag.GlobalContext.Status = gcStatus
	if loadedGC.Context != nil {
		if diag.Resume != nil {
			diag.Resume.GlobalContextSource = "artifact"
			diag.Resume.GlobalContextReused = true
			diag.Resume.GlobalContextAICalls = 0
		}
	} else if gcErr == nil && gcStatus == GlobalContextStatusCompleted {
		if diag.Resume != nil {
			diag.Resume.GlobalContextAICalls = 1
		}
	}

	if pipelineResumeMode(req) && hasPersistedRun {
		gcFromArtifact := loadedGC.Context != nil
		planInput := s.buildResumePlanInput(ctx, req, persistedRun, dataPackHash, gcHash, gcFromArtifact)
		applyResumePlanDiagnostics(&diag, ResolvePipelineResumePlan(planInput), dataPackSource)
	}

	var dashboardOK, actionsOK bool
	var dashboardFailed, actionsFailed bool
	var dashOutcome *DashboardRunOutcome
	var actRes *actions.GenerateActionsResult
	var dashErr, actErr error

	if req.GenerateDashboard {
		if hasPersistedRun && s.canReusePersistedDashboard(ctx, persistedRun, req.SellerAccountID, dataPackHash, gcHash) {
			dashboardOK = true
			result.DashboardRunID = uuidString(persistedRun.DashboardRunID)
			result.DashboardStatus = textOrEmpty(persistedRun.DashboardStatus)
			diag.Dashboard.Status = result.DashboardStatus
			diag.Dashboard.ArtifactSource = "persisted"
			diag.Dashboard.Reused = true
			diag.Dashboard.SynthesisAICalls = 0
			diag.Dashboard.UsageSource = UsageSourceArtifact
			if diag.Resume != nil {
				diag.Resume.DashboardSource = "persisted"
				diag.Resume.DashboardReused = true
				diag.Resume.DashboardAICalls = 0
			}
		} else if s.dashboard == nil {
			dashboardFailed = true
			result.DashboardStatus = StepStatusFailed
			diag.Dashboard.Status = StepStatusFailed
			result.ErrorMessage = "dashboard runner is not configured"
		} else {
			if err := s.guardExecutionLease(ctx, req, runID); err != nil {
				return nil, err
			}
			if SharedChunkAnalysisEnabled() {
				if sharedChunkSet == nil || sharedChunkRun == nil {
					return s.failRun(ctx, runID, result, PipelineStatusFailed, "shared_chunk_analysis_missing_for_dashboard", fmt.Errorf("shared chunk analysis result is missing for dashboard"), artifacts)
				}
			}
			dashOpts := dashboardhome.NarrativeRunOptions{
				DebugOutputDir:    "",
				MetricDate:        &metricDate,
				DataPackOverride:  pack,
				DataPackHash:      dataPackHash,
				GlobalContext:     globalCtx,
				GlobalContextHash: gcHash,
				PipelineRunID:     &runID,
				ForceRegenerate:   req.ForceRegenerate,
			}
			if SharedChunkAnalysisEnabled() {
				dashOpts.ChunkSetOverride = sharedChunkSet
				dashOpts.PrecomputedChunkRun = sharedChunkRun
			}
			dashOutcome, dashErr = s.dashboard.RunForSellerMetricDateWithOptions(ctx, req.SellerAccountID, metricDate, dashOpts)
			populateDashboardDiagnostics(&diag, dashOutcome)
			persistDashboardCallDiagnostics(ctx, artifacts, runID, req, providers, dataPackHash, gcHash, dashOutcome)
			if dashErr != nil {
				dashboardFailed = true
				result.DashboardStatus = StepStatusFailed
				diag.Dashboard.Status = StepStatusFailed
				if result.ErrorMessage == "" {
					result.ErrorMessage = dashErr.Error()
				}
			} else if dashOutcome != nil && dashOutcome.Result != nil && dashOutcome.Result.IsSuccess() {
				dashboardOK = true
				synthIn := dashOutcome.Result.DashboardSynthesisInputTokens
				synthOut := dashOutcome.Result.DashboardSynthesisOutputTokens
				if synthIn == 0 && synthOut == 0 && !dashOutcome.Result.DashboardChunkUsageIncluded {
					synthIn = dashOutcome.Result.InputTokens
					synthOut = dashOutcome.Result.OutputTokens
				}
				dashModel := dashboardArtifactModel(dashOutcome)
				dashRaw := &actions.AIRawResponse{
					Model:        dashModel,
					RequestID:    dashOutcome.Result.DashboardSynthesisRequestID,
					InputTokens:  synthIn,
					OutputTokens: synthOut,
					LatencyMs:    dashOutcome.Result.DashboardSynthesisLatencyMs,
				}
				s.recordDashboardSynthesisUsage(ctx, req.SellerAccountID, runID, providers.Dashboard, dashModel, dashRaw, "completed")
				result.DashboardRunID = dashOutcome.Result.RunID
				result.DashboardStatus = StepStatusCompleted
				diag.Dashboard.Status = StepStatusCompleted
				if diag.Resume != nil {
					diag.Resume.DashboardAICalls = 1
				}
				if dashOutcome.Result.DashboardValidationStatus != "" {
					diag.Dashboard.ValidationStatus = dashOutcome.Result.DashboardValidationStatus
				}
				dashboardMeta := aiArtifactMeta(runID, providers.Dashboard, dashboardArtifactModel(dashOutcome), 0, 0, map[string]any{
					"data_pack_hash":      dataPackHash,
					"global_context_hash": gcHash,
					"mode":                dashboardArtifactMode(dashOutcome),
					"validation_mode":     dashOutcome.Result.DashboardValidationMode,
				})
				_ = artifacts.SaveRequiredDocument(ctx, ArtifactKindDashboardValidation, dashboardMeta, buildDashboardValidationArtifact(dashOutcome))
				if dashOutcome.Result.RunID != "" {
					_ = s.store.LinkDashboardRun(ctx, dashOutcome.Result.RunID, req.SellerAccountID, runID, dataPackHash, gcHash)
				}
				if req.DebugArtifacts {
					dashEst, _ := EstimateJSONTokens(dashOutcome.DebugInput)
					dashMax := MaxAIInputTokensPerRequest()
					dashboardMeta := aiArtifactMeta(runID, providers.Dashboard, dashboardArtifactModel(dashOutcome), dashEst, dashMax, map[string]any{
						"data_pack_hash":      dataPackHash,
						"global_context_hash": gcHash,
						"mode":                dashboardArtifactMode(dashOutcome),
						"validation_mode":     dashOutcome.Result.DashboardValidationMode,
					})
					_ = artifacts.SaveDocument(ctx, ArtifactKindDashboardInput, dashboardMeta, dashOutcome.DebugInput)
					dashboardOutMeta := aiArtifactMeta(runID, providers.Dashboard, dashboardArtifactModel(dashOutcome), dashEst, dashMax, map[string]any{
						"dashboard_run_id":    result.DashboardRunID,
						"data_pack_hash":      dataPackHash,
						"global_context_hash": gcHash,
						"mode":                dashboardArtifactMode(dashOutcome),
					})
					_ = artifacts.SaveDocument(ctx, ArtifactKindDashboardOutput, dashboardOutMeta, dashOutcome.DebugOutput)
					_ = artifacts.SaveDocument(ctx, ArtifactKindDashboardRawOutput, dashboardOutMeta, buildDashboardRawOutputArtifact(
						dashboardArtifactModel(dashOutcome),
						"completed",
						synthIn,
						synthOut,
						dashOutcome.Result.DashboardSynthesisLatencyMs,
						dashOutcome.Result.DashboardSynthesisRequestID,
					))
				}
			} else {
				dashboardFailed = true
				result.DashboardStatus = StepStatusFailed
				diag.Dashboard.Status = StepStatusFailed
				if dashOutcome != nil && dashOutcome.Result != nil {
					if result.ErrorMessage == "" && dashOutcome.Result.ErrorMessage != "" {
						result.ErrorMessage = dashOutcome.Result.ErrorMessage
					}
					if result.ErrorCode == "" && dashOutcome.Result.ErrorCode != "" {
						result.ErrorCode = dashOutcome.Result.ErrorCode
					}
				}
			}
		}
	}

	if req.GenerateActions {
		gcReady := gcStatus == GlobalContextStatusCompleted && globalCtx != nil
		if !gcReady {
			result.ActionsStatus = StepStatusSkipped
			diag.Actions.Status = StepStatusSkipped
		} else if hasPersistedRun && !req.StageRestartRequested && s.persistedActionsReusable(ctx, persistedRun, req.SellerAccountID, dataPackHash, gcHash) {
			actionsOK = true
			result.ActionsRunID = persistedRun.ActionsRunID.Int64
			reusedActionsStatus := textOrEmpty(persistedRun.ActionsStatus)
			if !actions.IsSuccessfulActionFinalStatus(reusedActionsStatus) {
				if linkedStatus, statusErr := s.store.GetActionGenerationRunFinalStatus(ctx, persistedRun.ActionsRunID.Int64, req.SellerAccountID); statusErr == nil && linkedStatus != "" {
					reusedActionsStatus = linkedStatus
				}
			}
			result.ActionsStatus = reusedActionsStatus
			diag.Actions.Status = reusedActionsStatus
			if result.ActionsRunID > 0 {
				_ = s.store.LinkActionsRun(ctx, result.ActionsRunID, runID, dataPackHash, gcHash)
			}
		} else if s.actions == nil {
			actionsFailed = true
			result.ActionsStatus = StepStatusFailed
			if result.ErrorMessage == "" {
				result.ErrorMessage = "actions runner is not configured"
			}
		} else {
			if err := s.guardExecutionLease(ctx, req, runID); err != nil {
				return nil, err
			}
			diag.Actions.TimeoutSeconds = s.actionsRequestTimeoutSeconds()
			diag.Actions.ComposeTimeoutSeconds = actions.ActionsComposeTimeoutSeconds()
			diag.Actions.ObjectDetailTimeoutSeconds = actions.ActionsObjectDetailTimeoutSeconds()
			diag.Actions.MaxOutputTokens = actions.ActionsComposeEffectiveMaxOutputTokens()
			diag.Actions.RequestedMaxOutputTokens = actions.ActionsComposeRequestedMaxOutputTokens()
			if pipelineResumeMode(req) {
				diag.Actions.DataPackSource = dataPackSource
				if SharedChunkAnalysisEnabled() {
					diag.Actions.SharedAnalysisSource = "artifact"
				}
				if diag.GlobalContext.Source == "artifact" || (diag.Resume != nil && diag.Resume.GlobalContextReused) {
					diag.Actions.GlobalContextSource = "artifact"
				}
				if diag.Dashboard.Reused || diag.Dashboard.ArtifactSource == "persisted" {
					diag.Actions.DashboardSource = "persisted"
				}
			}
			actionOpts := actions.GenerateOptionsFromEnv()
			actionOpts.AIFirst = true
			actionOpts.AIEnabled = req.AIEnabled
			actionOpts.ForceRegenerate = req.ForceRegenerate
			actionOpts.PipelineRunID = &runID
			actionOpts.GlobalContextHash = gcHash
			if hasPersistedRun && persistedRun.ActionsRunID.Valid && persistedRun.ActionsRunID.Int64 > 0 && textOrEmpty(persistedRun.ActionsStatus) == StepStatusFailed {
				existingActionsRunID := persistedRun.ActionsRunID.Int64
				actionOpts.ExistingActionsRunID = &existingActionsRunID
			}

			var input actions.ActionsAIFirstInput
			var buildErr error
			if SharedChunkAnalysisEnabled() {
				if sharedChunkRun == nil {
					actionsFailed = true
					result.ActionsStatus = StepStatusFailed
					if result.ErrorCode == "" {
						result.ErrorCode = "shared_chunk_analysis_missing_for_actions"
					}
					if result.ErrorMessage == "" {
						result.ErrorMessage = "shared chunk analysis result is missing for actions"
					}
					diag.Actions.Status = StepStatusFailed
					diag.Actions.InputSource = "candidates"
					diag.Actions.InputSchema = actions.ActionsAIFirstSchemaVersionV2
				} else {
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
				}
			} else {
				input, buildErr = s.actions.BuildActionsAIFirstInputFromDataPack(pack, dataPackHash, globalCtx, actionOpts)
				populateActionsDiagnosticsFromInput(&diag.Actions, input, "legacy_data_pack")
			}
			if buildErr != nil {
				actionsFailed = true
				result.ActionsStatus = StepStatusFailed
				if result.ErrorMessage == "" {
					result.ErrorMessage = buildErr.Error()
				}
			} else if actionsFailed && SharedChunkAnalysisEnabled() && sharedChunkRun == nil {
				// missing shared chunk run already handled above
			} else {
				var actEst int
				var actMax int
				wireBytes, wireErr := actions.MarshalActionsAIFirstInputWire(input)
				if wireErr != nil {
					actionsFailed = true
					result.ActionsStatus = StepStatusFailed
					if result.ErrorMessage == "" {
						result.ErrorMessage = wireErr.Error()
					}
					if result.ErrorCode == "" {
						result.ErrorCode = actions.ActionsErrorCode(wireErr)
					}
				} else {
					actEst = EstimateBytesTokens(wireBytes)
					actMax = MaxAIInputTokensPerRequest()
					diag.Actions.InputEstimatedTokens = actEst
					diag.Actions.InputTokenLimit = actMax
					diag.Actions.TotalRequestBytes = len(wireBytes)
					diag.Actions.TotalEstimatedInputTokens = actEst
					diag.Actions.MaxInputTokens = actMax
					if actMax > actEst {
						diag.Actions.InputHeadroomTokens = actMax - actEst
					}
					diag.Actions.InputDataPackSerialized = input.SchemaVersion != actions.ActionsAIFirstSchemaVersionV2
					if inputHash, hashErr := actions.HashActionsAIFirstInputWire(input); hashErr == nil {
						diag.Actions.InputHash = inputHash
					}
				}
				if !actionsFailed {
					if req.DebugArtifacts && wireBytes != nil {
						actInputMeta := aiArtifactMeta(runID, providers.Actions, "", actEst, actMax, map[string]any{
							"data_pack_hash":      dataPackHash,
							"global_context_hash": gcHash,
							"input_schema":        input.SchemaVersion,
							"input_source":        diag.Actions.InputSource,
						})
						_ = artifacts.SaveDocument(ctx, ArtifactKindActionsInput, actInputMeta, json.RawMessage(wireBytes))
					}
					actRes, actErr = s.actions.GenerateActionsAIFirstFromInput(ctx, input, actionOpts)
					composeCalls := 0
					if actRes != nil {
						composeCalls = actRes.ComposeCalls
					}
					if actErr != nil {
						if composeCalls > 0 && actRes != nil {
							status := "failed"
							s.recordActionsSynthesisUsage(ctx, req.SellerAccountID, runID, providers.Actions, "", actRawResponse(actRes), status)
						}
						actionsFailed = true
						result.ActionsStatus = StepStatusFailed
						if result.ErrorMessage == "" {
							result.ErrorMessage = actErr.Error()
						}
						if code := actions.ActionsErrorCode(actErr); code != "" && result.ErrorCode == "" {
							result.ErrorCode = code
						}
						diag.Actions.ComposeCalls = composeCalls
						diag.Actions.RetryCalls = 0
						if diag.Resume != nil {
							diag.Resume.ActionsAICalls = composeCalls
						}
					} else if actRes != nil {
						if composeCalls > 0 {
							usageStatus := "completed"
							if actRes.FinalStatus == actions.GenerationFinalStatusFailed {
								usageStatus = "validation_failed"
							} else if actRes.FinalStatus == actions.GenerationFinalStatusCompletedPartial {
								usageStatus = "completed_partial"
							}
							s.recordActionsSynthesisUsage(ctx, req.SellerAccountID, runID, providers.Actions, "", actRawResponse(actRes), usageStatus)
						}
						diag.Actions.ComposeCalls = composeCalls
						diag.Actions.RetryCalls = 0
						if diag.Resume != nil {
							diag.Resume.ActionsAICalls = composeCalls
						}
						diag.Actions.FinalStatus = actRes.FinalStatus
						diag.Actions.FinalCount = actRes.AcceptedCount
						populateActionsDiagnosticsFromResult(&diag.Actions, actRes)
						if actRes.PipelineDiagnostics != nil {
							actions.ApplyActionsTokenDiagnostics(actRes.PipelineDiagnostics, actEst, actRes.InputTokens, actMax)
						}
						populateFinalTopActionsDiagnostics(&diag.Dashboard, actRes, dashOutcome)
						result.ActionsRunID = actRes.RunID
						if actRes.FinalStatus == actions.GenerationFinalStatusFailed {
							actionsFailed = true
							result.ActionsStatus = StepStatusFailed
							diag.Actions.Status = StepStatusFailed
							if actRes.FailureSubreason != "" && result.ErrorCode == "" {
								result.ErrorCode = actRes.FailureSubreason
							}
						} else if actions.IsEmptyActionFinalStatus(actRes.FinalStatus) {
							actionsOK = true
							result.ActionsStatus = "empty"
							diag.Actions.Status = "empty"
							diag.Actions.AcceptedCount = 0
						} else if actions.IsPublishableActionFinalStatus(actRes.FinalStatus) {
							actionsOK = true
							if actRes.FinalStatus == actions.GenerationFinalStatusCompletedPartial {
								result.ActionsStatus = actions.GenerationFinalStatusCompletedPartial
								diag.Actions.Status = actions.GenerationFinalStatusCompletedPartial
							} else {
								result.ActionsStatus = StepStatusCompleted
								diag.Actions.Status = StepStatusCompleted
							}
							diag.Actions.AcceptedCount = actRes.AcceptedCount
							diag.Actions.DroppedCount = actRes.DroppedCount
						} else {
							actionsFailed = true
							result.ActionsStatus = StepStatusFailed
							diag.Actions.Status = StepStatusFailed
						}
						if actRes.RunID > 0 {
							_ = s.store.LinkActionsRun(ctx, actRes.RunID, runID, dataPackHash, gcHash)
						}
						if req.DebugArtifacts {
							actionsMeta := aiArtifactMeta(runID, providers.Actions, "", actEst, actMax, map[string]any{
								"actions_run_id":      actRes.RunID,
								"data_pack_hash":      dataPackHash,
								"global_context_hash": gcHash,
								"retry_count":         0,
							})
							_ = artifacts.SaveDocument(ctx, ArtifactKindActionsOutput, actionsMeta, buildActionsOutputArtifact(actRes))
							_ = artifacts.SaveDocument(ctx, ArtifactKindActionsValidation, actionsMeta, buildActionsValidationArtifact(actRes))
							saveTwoPhaseActionsArtifacts(ctx, artifacts, runID, actionsMeta, actRes)
							usageStatus := "completed"
							if actRes.FinalStatus == actions.GenerationFinalStatusFailed {
								usageStatus = "validation_failed"
							}
							_ = artifacts.SaveDocument(ctx, ArtifactKindActionsRawOutput, actionsMeta, buildActionsRawOutputArtifact(actRes, "", usageStatus))
						}
					}
				}
			}
		}
	}

	finalStatus := resolvePipelineStatus(req, dashboardOK, actionsOK, dashboardFailed, actionsFailed)
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
	}); err != nil {
		return nil, err
	}

	if shouldPublishPipeline(req, finalStatus, dashboardOK, actionsOK, dashboardFailed, actionsFailed, result) {
		var dashboardPayload []byte
		if dashOutcome != nil && dashOutcome.Result != nil {
			dashboardPayload = dashOutcome.Result.ValidatedNarrativePayload
		}
		var actionItems []actions.ActionItem
		if actRes != nil {
			actionItems = actRes.Actions
		}
		composeTruncated := actRes != nil && actRes.FailureSubreason == actions.ActionsErrorComposeOutputTruncated
		if actRes != nil && actRes.PipelineDiagnostics != nil && actRes.PipelineDiagnostics.ActionsComposeOutputTruncated {
			composeTruncated = true
		}
		pubGuard := RunPublicationGuard(PublicationGuardInput{
			RequestedDashboard:    req.GenerateDashboard,
			RequestedActions:      req.GenerateActions,
			Pack:                  pack,
			DashboardPayload:      dashboardPayload,
			Actions:               actionItems,
			ComposeTruncated:      composeTruncated,
			GlobalContextBlocking: hasBlockingGlobalContextPlaceholder(diag.GlobalContext.LastValidation),
		})
		pubGuard.NewPipelineRunID = runID
		gcValidationArtifact := globalContextValidationArtifactFromResult(
			diag.GlobalContext.ValidationStatusBefore, diag.GlobalContext.ValidationStatus,
			diag.GlobalContext.ConfidenceBefore, diag.GlobalContext.Confidence,
			diag.GlobalContext.LastValidation,
		)
		saveDiagnosticArtifacts(ctx, artifacts, runID, map[string]any{
			"data_pack_hash": dataPackHash, "global_context_hash": gcHash,
		}, req.DebugArtifacts, nil, buildSharedChunkRefMappingArtifact(sharedChunkSet, sharedChunkRun),
			gcValidationArtifact, pubGuard.FinalSemanticValidation, pubGuard, buildActionsExecutionDiagnostics(actRes, &diag))
		diag.PublicationGuard = pubGuard
		if !pubGuard.PublicationAllowed {
			result.ErrorCode = "publication_blocked"
			result.ErrorMessage = pubGuard.DecisionReason
		} else if err := s.guardExecutionLease(ctx, req, runID); err != nil {
			return nil, err
		} else if err := validatePipelineHashes(result, dataPackHash, gcHash); err != nil {
			finalStatus = PipelineStatusFailed
			result.Status = finalStatus
			result.ErrorCode = "hash_mismatch"
			result.ErrorMessage = err.Error()
		} else if s.publisher == nil {
			finalStatus = PipelineStatusFailed
			result.Status = finalStatus
			result.ErrorCode = "publisher_not_configured"
			result.ErrorMessage = "snapshot publisher is not configured"
		} else if _, err := s.publisher.PublishPipelineSnapshot(ctx, runID); err != nil {
			finalStatus = PipelineStatusFailed
			result.Status = finalStatus
			result.ErrorCode = "publication_failed"
			result.ErrorMessage = err.Error()
		} else {
			result.Published = true
		}
	}

	summary := map[string]any{
		"status":           finalStatus,
		"dashboard_status": result.DashboardStatus,
		"actions_status":   result.ActionsStatus,
		"dashboard_run_id": result.DashboardRunID,
		"actions_run_id":   result.ActionsRunID,
	}
	if diag.PublicationGuard.Decision != "" {
		summary["publication_guard"] = map[string]any{
			"decision":             diag.PublicationGuard.Decision,
			"publication_allowed":  diag.PublicationGuard.PublicationAllowed,
			"decision_reason":      diag.PublicationGuard.DecisionReason,
			"blocking_issue_codes": diag.PublicationGuard.BlockingIssueCodes,
		}
	}
	_ = artifacts.SaveDocument(ctx, ArtifactKindPipelineSummary, pipelineArtifactMeta(runID, map[string]any{
		"seller_account_id":   req.SellerAccountID,
		"metric_date":         metricDate.Format("2006-01-02"),
		"data_pack_hash":      dataPackHash,
		"global_context_hash": gcHash,
	}), summary)
	result.ArtifactRefs = artifacts.Refs()

	if result.Published {
		result.ErrorCode = ""
		result.ErrorMessage = ""
	}
	update := UpdatePipelineRunParams{
		ID:              runID,
		Status:          finalStatus,
		DashboardRunID:  result.DashboardRunID,
		ActionsRunID:    result.ActionsRunID,
		DashboardStatus: result.DashboardStatus,
		ActionsStatus:   result.ActionsStatus,
		ErrorCode:       result.ErrorCode,
		ErrorMessage:    result.ErrorMessage,
		CompletedAt:     ptrTime(now),
	}
	_ = s.store.UpdatePipelineRun(ctx, update)
	finalizePipelineUsageDiagnostics(&diag)
	syncResumeResult(result, &diag)
	return result, nil
}

func (s *SellerAIPipelineService) generateGlobalContext(
	ctx context.Context,
	req RunSellerAIPipelineRequest,
	runID int64,
	dataPackHash string,
	input GlobalContextGenerationInput,
	inputHash string,
	artifacts *ArtifactRecorder,
	diag *PipelineDiagnostics,
	providers PipelineProviders,
) (*aicontext.GlobalSemanticContext, string, string, error) {
	if s.gcClient == nil {
		if req.AIEnabled {
			return nil, "", "", fmt.Errorf("global context ai client is not configured")
		}
		s.gcClient = NewDeterministicGlobalContextFakeClient()
	}

	promptModel := providers.GlobalContext
	if promptModel == "" {
		promptModel = ProviderFakeGlobalContext
	}
	maxInputTokens := MaxAIInputTokensPerRequest()
	maxOutputTokens := GlobalContextRequestedMaxOutputTokens()
	timeoutSeconds := s.globalContextTimeoutSeconds()
	if timeoutSeconds <= 0 {
		timeoutSeconds = openaix.DefaultRequestTimeoutSeconds
	}
	requestMetrics, metricsErr := estimateGlobalContextRequestMetrics(input, promptModel, timeoutSeconds, maxInputTokens, maxOutputTokens)
	if metricsErr == nil {
		populateGlobalContextRequestDiagnostics(diag, requestMetrics)
	}

	wireBytes, err := MarshalGlobalContextGenerationInputWire(input)
	if err != nil {
		return nil, "", "", err
	}
	estTokens, _ := EstimateJSONTokens(json.RawMessage(wireBytes))
	gcMeta := aiArtifactMeta(runID, providers.GlobalContext, promptModel, estTokens, maxInputTokens, map[string]any{
		"data_pack_hash":            dataPackHash,
		"global_context_input_hash": inputHash,
		"input_schema_version":      input.SchemaVersion,
	})
	if metricsErr == nil {
		for k, v := range globalContextRequestMetricsArtifactMeta(requestMetrics, inputHash) {
			gcMeta[k] = v
		}
	}
	_ = artifacts.SaveDocument(ctx, ArtifactKindGlobalContextInput, gcMeta, json.RawMessage(wireBytes))
	_ = artifacts.SaveDocument(ctx, ArtifactKindGlobalContextPrompt, gcMeta, BuildGlobalContextPromptArtifact(promptModel, input))

	draft, raw, err := s.gcClient.GenerateGlobalContext(ctx, input)
	s.recordGlobalContextUsage(ctx, req.SellerAccountID, runID, providers.GlobalContext, promptModel, raw, err)
	var lastRaw *GlobalContextRawResponse = raw

	if err != nil {
		if diag != nil {
			diag.GlobalContext.FailureSubreason = ClassifyGlobalContextFailure(raw, err, "")
			if isInvalidJSONError(err) && diag.GlobalContext.FailureSubreason == GlobalContextFailureProviderError {
				diag.GlobalContext.FailureSubreason = GlobalContextFailureInvalidJSON
			}
			diag.GlobalContext.AICalls = 1
			diag.GlobalContext.RetryCalls = 0
			if raw != nil {
				diag.GlobalContext.MaxOutputTokens = raw.RequestedMaxOutputTokens
				diag.GlobalContext.OutputTokens = raw.OutputTokens
			}
		}
		saveGlobalContextCallArtifacts(ctx, artifacts, runID, gcMeta, providers.GlobalContext, raw, nil, timeoutSeconds, req.DebugArtifacts)
		return nil, GlobalContextStatusFailed, "", err
	}
	if raw != nil && raw.Model != "" {
		promptModel = raw.Model
	}
	RepairGlobalSemanticContext(draft, input)
	if GlobalContextIsEffectivelyEmpty(draft) {
		if diag != nil {
			diag.GlobalContext.FailureSubreason = "empty_context"
		}
		return nil, GlobalContextStatusFailed, "", fmt.Errorf("global context is empty")
	}

	validation := ValidateGlobalSemanticContext(input, *draft)
	confidenceBefore := strings.TrimSpace(draft.Confidence)
	statusBefore := validation.Status
	populateGlobalContextValidationDiagnostics(diag, statusBefore, validation.Status, confidenceBefore, confidenceBefore, validation)
	if diag != nil {
		diag.GlobalContext.LastValidation = validation
	}
	NormalizeGlobalContextConfidenceAfterValidation(draft, validation)
	if diag != nil {
		diag.GlobalContext.Confidence = strings.TrimSpace(draft.Confidence)
		diag.GlobalContext.ValidationStatus = validation.Status
	}
	final := draft
	retryCalls := 0
	if validation.Status == aicontext.ValidationStatusInvalid && !pipelineResumeMode(req) {
		preRetry := *draft
		retryDraft, retryRaw, retryErr := s.gcClient.RetryGlobalContext(ctx, GlobalContextRetryInput{
			Input:            input,
			DraftContext:     preRetry,
			ValidationIssues: validation.Issues,
		})
		retryCalls = 1
		s.recordGlobalContextUsage(ctx, req.SellerAccountID, runID, providers.GlobalContext, promptModel, retryRaw, retryErr)
		if retryErr == nil && retryDraft != nil {
			RepairGlobalSemanticContext(retryDraft, input)
			if !GlobalContextIsEffectivelyEmpty(retryDraft) {
				retryValidation := ValidateGlobalSemanticContext(input, *retryDraft)
				useRetry := globalContextValidationAccepted(retryValidation.Status) ||
					(len(countFatalGlobalContextIssues(retryValidation.Issues)) < len(countFatalGlobalContextIssues(validation.Issues)) &&
						strings.TrimSpace(retryDraft.OverallStory) != "")
				if useRetry {
					draft = retryDraft
					final = retryDraft
					validation = retryValidation
					NormalizeGlobalContextConfidenceAfterValidation(draft, validation)
					if diag != nil {
						diag.GlobalContext.LastValidation = validation
						diag.GlobalContext.ValidationStatus = validation.Status
						diag.GlobalContext.Confidence = strings.TrimSpace(draft.Confidence)
					}
					if retryRaw != nil {
						lastRaw = retryRaw
						if retryRaw.Model != "" {
							promptModel = retryRaw.Model
						}
					}
				}
			}
		}
	}
	populateGlobalContextDiagnosticsFromGeneration(diag, lastRaw, validation)
	if diag != nil {
		diag.GlobalContext.AICalls = 1 + retryCalls
		diag.GlobalContext.RetryCalls = retryCalls
	}

	if !globalContextValidationAccepted(validation.Status) {
		if diag != nil {
			diag.GlobalContext.FailureSubreason = "validation_failed"
		}
		failMeta := aiArtifactMeta(runID, providers.GlobalContext, promptModel, estTokens, maxInputTokens, map[string]any{
			"data_pack_hash":            dataPackHash,
			"global_context_input_hash": inputHash,
			"validation_status":         validation.Status,
		})
		_ = artifacts.SaveDocument(ctx, ArtifactKindGlobalContextValidation, failMeta, validation)
		saveGlobalContextCallArtifacts(ctx, artifacts, runID, failMeta, providers.GlobalContext, lastRaw, draft, timeoutSeconds, req.DebugArtifacts)
		if lastRaw != nil {
			_ = artifacts.SaveDocument(ctx, ArtifactKindGlobalContextRawOutput, failMeta, buildGlobalContextRawOutputArtifact(lastRaw, promptModel, GlobalContextStatusFailed))
		}
		return nil, GlobalContextStatusFailed, "", fmt.Errorf("global context validation failed: %s", validation.Status)
	}

	if diag != nil {
		diag.GlobalContext.EstimatedInputTokens = estTokens
		diag.GlobalContext.MaxInputTokens = maxInputTokens
		diag.GlobalContext.MaxOutputTokens = maxOutputTokens
		diag.GlobalContext.Model = promptModel
	}

	gcHash := aihash.HashGlobalSemanticContext(final)
	gcArtifact, err := buildGlobalContextArtifact(input, final, inputHash, gcHash)
	if err != nil {
		return nil, GlobalContextStatusFailed, "", err
	}
	artifactMeta := pipelineArtifactMeta(runID, map[string]any{
		"seller_account_id":         input.SellerAccountID,
		"metric_date":               input.MetricDate,
		"data_pack_hash":            dataPackHash,
		"global_context_hash":       gcHash,
		"global_context_input_hash": inputHash,
		"input_schema_version":      input.SchemaVersion,
	})
	if err := artifacts.SaveRequiredDocument(ctx, ArtifactKindGlobalContextFinal, artifactMeta, gcArtifact); err != nil {
		return nil, GlobalContextStatusFailed, "", fmt.Errorf("global_context_persist_failed: %w", err)
	}
	if diag != nil {
		doc := ArtifactDocument{Meta: artifactMeta, Payload: scrubArtifactPayload(gcArtifact)}
		if h, hashErr := HashArtifactPayload(doc); hashErr == nil {
			diag.GlobalContext.ArtifactHash = h
		}
	}

	respMeta := aiArtifactMeta(runID, providers.GlobalContext, promptModel, estTokens, maxInputTokens, map[string]any{
		"data_pack_hash":            dataPackHash,
		"global_context_hash":       gcHash,
		"global_context_input_hash": inputHash,
	})
	_ = artifacts.SaveDocument(ctx, ArtifactKindGlobalContextResponse, respMeta, map[string]any{
		"context": final,
		"raw":     lastRaw,
		"model":   promptModel,
	})
	gcStatusLabel := GlobalContextStatusCompleted
	_ = artifacts.SaveDocument(ctx, ArtifactKindGlobalContextRawOutput, respMeta, buildGlobalContextRawOutputArtifact(lastRaw, promptModel, gcStatusLabel))
	gcValidationArtifact := globalContextValidationArtifactFromResult(
		statusBefore, validation.Status, confidenceBefore, strings.TrimSpace(final.Confidence), validation,
	)
	_ = artifacts.SaveRequiredDocument(ctx, ArtifactKindGlobalContextValidation, respMeta, compactGlobalContextValidation(gcValidationArtifact, req.DebugArtifacts))
	saveGlobalContextCallArtifacts(ctx, artifacts, runID, respMeta, providers.GlobalContext, lastRaw, final, timeoutSeconds, req.DebugArtifacts)
	if req.DebugArtifacts {
		_ = artifacts.SaveDocument(ctx, ArtifactKindGlobalContextValidation, respMeta, gcValidationArtifact)
	}
	return final, GlobalContextStatusCompleted, gcHash, nil
}

func compactGlobalContextValidation(full map[string]any, debug bool) map[string]any {
	if debug || full == nil {
		return full
	}
	return map[string]any{
		"schema_version":              full["schema_version"],
		"status_after_validation":     full["status_after_validation"],
		"confidence_after_validation": full["confidence_after_validation"],
		"blocking_issue_count":        full["blocking_issue_count"],
		"warning_count":               full["warning_count"],
	}
}

func resolvePipelineStatus(req RunSellerAIPipelineRequest, dashboardOK, actionsOK, dashboardFailed, actionsFailed bool) string {
	requestedDashboard := req.GenerateDashboard
	requestedActions := req.GenerateActions

	if requestedDashboard && requestedActions {
		switch {
		case dashboardOK && actionsOK:
			return PipelineStatusCompleted
		case dashboardFailed && actionsOK:
			return PipelineStatusCompletedPartial
		case dashboardOK && actionsFailed:
			return PipelineStatusCompletedPartial
		case dashboardFailed && actionsFailed:
			return PipelineStatusFailed
		case dashboardOK:
			return PipelineStatusCompletedPartial
		case actionsOK:
			return PipelineStatusCompletedPartial
		default:
			return PipelineStatusFailed
		}
	}
	if requestedDashboard {
		if dashboardOK {
			return PipelineStatusCompleted
		}
		return PipelineStatusFailed
	}
	if requestedActions {
		if actionsOK {
			return PipelineStatusCompleted
		}
		return PipelineStatusFailed
	}
	return PipelineStatusEmpty
}

func (s *SellerAIPipelineService) failBeforeRun(ctx context.Context, result *RunSellerAIPipelineResult, code string, err error) (*RunSellerAIPipelineResult, error) {
	result.Status = PipelineStatusFailed
	result.ErrorCode = code
	result.ErrorMessage = err.Error()
	return result, nil
}

func (s *SellerAIPipelineService) failRun(
	ctx context.Context,
	runID int64,
	result *RunSellerAIPipelineResult,
	status, code string,
	err error,
	artifacts *ArtifactRecorder,
) (*RunSellerAIPipelineResult, error) {
	result.Status = status
	result.ErrorCode = code
	result.ErrorMessage = err.Error()
	result.ArtifactRefs = artifacts.Refs()
	_ = s.store.UpdatePipelineRun(ctx, UpdatePipelineRunParams{
		ID:           runID,
		Status:       status,
		ErrorCode:    code,
		ErrorMessage: err.Error(),
		CompletedAt:  ptrTime(time.Now().UTC()),
	})
	return result, nil
}

func ptrTime(t time.Time) *time.Time {
	return &t
}

// ActionsGenerationAdapter adapts actions.GenerationService to ActionsRunner.
type ActionsGenerationAdapter struct {
	*actions.GenerationService
}

func (a *ActionsGenerationAdapter) BuildActionsAIFirstInputFromDataPack(
	pack *aidatapack.SellerAIDataPack,
	dataPackHash string,
	globalContext *aicontext.GlobalSemanticContext,
	opts actions.GenerateActionsOptions,
) (actions.ActionsAIFirstInput, error) {
	return actions.BuildActionsAIFirstInputFromDataPack(pack, dataPackHash, globalContext, opts)
}

func (a *ActionsGenerationAdapter) BuildActionsAIFirstInputFromCandidates(
	pack *aidatapack.SellerAIDataPack,
	dataPackHash string,
	globalContext *aicontext.GlobalSemanticContext,
	globalContextHash string,
	candidates []actions.AIFirstActionCandidate,
	failedChunkIDs []string,
	dataLimitations []string,
	opts actions.GenerateActionsOptions,
) (actions.ActionsAIFirstInput, error) {
	return actions.BuildActionsAIFirstInputFromCandidates(
		pack,
		dataPackHash,
		globalContext,
		globalContextHash,
		candidates,
		failedChunkIDs,
		dataLimitations,
		opts,
	)
}

func (a *ActionsGenerationAdapter) GenerateActionsAIFirstFromInput(
	ctx context.Context,
	input actions.ActionsAIFirstInput,
	opts actions.GenerateActionsOptions,
) (*actions.GenerateActionsResult, error) {
	return a.GenerationService.GenerateActionsAIFirstFromInput(ctx, input, opts)
}

// DashboardNarrativeAdapter adapts dashboardhome.NarrativeService to DashboardRunner.
type DashboardNarrativeAdapter struct {
	*dashboardhome.NarrativeService
}

var (
	_ DashboardRunner     = (*DashboardNarrativeAdapter)(nil)
	_ ChunkAnalysisRunner = (*DashboardNarrativeAdapter)(nil)
)

func (a *DashboardNarrativeAdapter) AnalyzeDataPackChunks(
	ctx context.Context,
	pack *aidatapack.SellerAIDataPack,
	chunkSet *aidatapack.ChunkSet,
	opts dashboardhome.NarrativeRunOptions,
) (*dashboardhome.ChunkAnalysisRunResult, error) {
	return a.NarrativeService.AnalyzeDataPackChunks(ctx, pack, chunkSet, opts)
}

func (a *DashboardNarrativeAdapter) RunForSellerMetricDateWithOptions(
	ctx context.Context,
	sellerAccountID int64,
	metricDate time.Time,
	opts dashboardhome.NarrativeRunOptions,
) (*DashboardRunOutcome, error) {
	res, err := a.NarrativeService.RunForSellerMetricDateWithOptions(ctx, sellerAccountID, metricDate, opts)
	if err != nil {
		return nil, err
	}
	debugInput := buildDashboardArtifactInput(sellerAccountID, metricDate, opts, "live")
	debugOutput := map[string]any{
		"status":        res.Status,
		"run_id":        res.RunID,
		"error_message": res.ErrorMessage,
	}
	return &DashboardRunOutcome{
		Result:      res,
		DebugInput:  debugInput,
		DebugOutput: debugOutput,
	}, nil
}

func dashboardArtifactMode(outcome *DashboardRunOutcome) string {
	if outcome == nil || outcome.DebugInput == nil {
		return "live"
	}
	if m, ok := outcome.DebugInput.(map[string]any); ok {
		if mode, ok := m["mode"].(string); ok && mode != "" {
			return mode
		}
	}
	return "live"
}

func dashboardArtifactModel(outcome *DashboardRunOutcome) string {
	if outcome == nil || outcome.DebugOutput == nil {
		return ""
	}
	if m, ok := outcome.DebugOutput.(map[string]any); ok {
		if model, ok := m["model"].(string); ok {
			return model
		}
	}
	return ""
}

func buildDashboardValidationArtifact(outcome *DashboardRunOutcome) map[string]any {
	if outcome == nil || outcome.Result == nil {
		return map[string]any{
			"mode": dashboardhome.CurrentDashboardValidationMode(),
		}
	}
	r := outcome.Result
	return dashboardhome.DashboardValidationArtifactFromResult(
		r.DashboardValidationMode,
		r.DashboardValidationStatus,
		r.DashboardValidationWarningCodes,
		r.DashboardValidationErrorCodes,
		r.DashboardValidationWarningCount,
		r.DashboardValidationErrorCount,
		r.DashboardStage4WarningCount,
	)
}

func buildActionsOutputArtifact(res *actions.GenerateActionsResult) map[string]any {
	if res == nil {
		return nil
	}
	return map[string]any{
		"run_id":          res.RunID,
		"metric_date":     res.MetricDate,
		"final_status":    res.FinalStatus,
		"accepted_count":  res.AcceptedCount,
		"dropped_count":   res.DroppedCount,
		"repaired_count":  res.RepairedCount,
		"fallback_count":  res.FallbackCount,
		"top_action_keys": res.TopActionKeys,
		"actions":         res.Actions,
	}
}

func buildActionsValidationArtifact(res *actions.GenerateActionsResult) map[string]any {
	if res == nil {
		return nil
	}
	out := map[string]any{
		"status":         res.FinalStatus,
		"accepted_count": res.AcceptedCount,
		"dropped_count":  res.DroppedCount,
	}
	if res.AIFirstValidation != nil {
		out["status"] = res.AIFirstValidation.Status
		out["issues"] = res.AIFirstValidation.Issues
		out["focus_issues"] = res.AIFirstValidation.FocusIssues
		out["dropped_actions"] = res.AIFirstValidation.DroppedActions
	}
	return out
}

func isInvalidJSONError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "invalid_json:")
}

type publishedPipelineStore interface {
	GetPublishedAIPipelineRunBySellerDateAndDataPackHash(ctx context.Context, arg dbgen.GetPublishedAIPipelineRunBySellerDateAndDataPackHashParams) (dbgen.AiPipelineRun, error)
}

func (s *SellerAIPipelineService) tryReusePublishedPipeline(ctx context.Context, sellerID int64, metricDate time.Time, dataPackHash string) (dbgen.AiPipelineRun, bool) {
	q, ok := s.store.(publishedPipelineStore)
	if !ok {
		return dbgen.AiPipelineRun{}, false
	}
	row, err := q.GetPublishedAIPipelineRunBySellerDateAndDataPackHash(ctx, dbgen.GetPublishedAIPipelineRunBySellerDateAndDataPackHashParams{
		SellerAccountID: sellerID,
		MetricDate:      pgtype.Date{Time: metricDate, Valid: true},
		DataPackHash:    dataPackHash,
	})
	if err != nil {
		return dbgen.AiPipelineRun{}, false
	}
	return row, true
}

func textOrEmpty(v pgtype.Text) string {
	if !v.Valid {
		return ""
	}
	return strings.TrimSpace(v.String)
}

func uuidString(v pgtype.UUID) string {
	if !v.Valid {
		return ""
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", v.Bytes[0:4], v.Bytes[4:6], v.Bytes[6:8], v.Bytes[8:10], v.Bytes[10:16])
}

func shouldPublishPipeline(
	req RunSellerAIPipelineRequest,
	finalStatus string,
	dashboardOK, actionsOK, dashboardFailed, actionsFailed bool,
	result *RunSellerAIPipelineResult,
) bool {
	if finalStatus == PipelineStatusFailed || finalStatus == PipelineStatusContextFailed {
		return false
	}
	if req.GenerateDashboard && req.GenerateActions {
		return dashboardOK && actionsOK && !dashboardFailed && !actionsFailed
	}
	if req.GenerateDashboard {
		return dashboardOK && !dashboardFailed
	}
	if req.GenerateActions {
		return actionsOK && !actionsFailed
	}
	return false
}

func validatePipelineHashes(result *RunSellerAIPipelineResult, dataPackHash, globalContextHash string) error {
	if result == nil {
		return fmt.Errorf("missing pipeline result")
	}
	if dataPackHash != "" && result.DataPackHash != "" && result.DataPackHash != dataPackHash {
		return fmt.Errorf("data_pack_hash mismatch")
	}
	if globalContextHash != "" && result.GlobalContextHash != "" && result.GlobalContextHash != globalContextHash {
		return fmt.Errorf("global_context_hash mismatch")
	}
	return nil
}

func (s *SellerAIPipelineService) guardExecutionLease(ctx context.Context, req RunSellerAIPipelineRequest, pipelineRunID int64) error {
	if req.LeaseGuard == nil {
		return nil
	}
	return req.LeaseGuard.ValidateLease(ctx, req.DispatchID, pipelineRunID)
}
