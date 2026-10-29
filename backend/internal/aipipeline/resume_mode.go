package aipipeline

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

const (
	resumeModeSameRun              = "same_run"
	stageRestartModeDerivedRun     = "derived_run_stage_restart"
	stageRestartArtifactSourceMeta = "stage_restart_source"
)

func pipelineStageRestartMode(req RunSellerAIPipelineRequest) bool {
	return req.StageRestartRequested &&
		req.SourcePipelineRunID > 0 &&
		req.RestartFromStage != "" &&
		!req.ForceRegenerate
}

func IsSupportedStageRestart(stage string) bool {
	return isSupportedStageRestart(stage)
}

func isSupportedStageRestart(stage string) bool {
	switch stage {
	case ResumeStageActions, ResumeStageGlobalContext, ResumeStageSharedChunkValidation, ResumeStageSharedChunkAnalysis:
		return true
	default:
		return false
	}
}

func syncResumeResult(result *RunSellerAIPipelineResult, diag *PipelineDiagnostics) {
	if result == nil || diag == nil || diag.Resume == nil {
		return
	}
	result.Resume = diag.Resume
}

func pipelineResumeMode(req RunSellerAIPipelineRequest) bool {
	return req.ResumeRequested && req.ExistingPipelineRunID > 0 && !req.ForceRegenerate
}

func (s *SellerAIPipelineService) loadPipelinePackForRun(
	ctx context.Context,
	req RunSellerAIPipelineRequest,
	metricDate time.Time,
	persistedRun *dbgen.AiPipelineRun,
	resumeDataPackHashHint string,
) (*aidatapack.SellerAIDataPack, string, error) {
	metricDateStr := metricDate.Format("2006-01-02")
	if pipelineResumeMode(req) {
		savedPack, loadErr := s.tryLoadSavedDataPack(
			ctx,
			req.ExistingPipelineRunID,
			req.SellerAccountID,
			metricDateStr,
			resumeDataPackHashHint,
		)
		if loadErr != nil {
			return nil, "", loadErr
		}
		if savedPack == nil {
			return nil, "", fmt.Errorf("resume requires frozen data_pack artifact from pipeline run %d", req.ExistingPipelineRunID)
		}
		return savedPack, "artifact", nil
	}
	pack, err := s.dataPack.Build(ctx, req.SellerAccountID, metricDate)
	if err != nil {
		return nil, "", err
	}
	return pack, "generated", nil
}

func validateResumePipelineRun(
	req RunSellerAIPipelineRequest,
	persisted dbgen.AiPipelineRun,
	metricDate time.Time,
) error {
	if persisted.SellerAccountID != req.SellerAccountID {
		return fmt.Errorf("resume seller_account_id mismatch: run=%d request=%d", persisted.SellerAccountID, req.SellerAccountID)
	}
	runDate := ""
	if persisted.MetricDate.Valid {
		runDate = persisted.MetricDate.Time.Format("2006-01-02")
	}
	reqDate := metricDate.Format("2006-01-02")
	if runDate != "" && runDate != reqDate {
		return fmt.Errorf("resume metric_date mismatch: run=%s request=%s", runDate, reqDate)
	}
	return nil
}

func validateResumeDataPackHash(persisted dbgen.AiPipelineRun, dataPackHash string) error {
	saved := strings.TrimSpace(persisted.DataPackHash)
	if saved == "" {
		return nil
	}
	if saved != strings.TrimSpace(dataPackHash) {
		return fmt.Errorf("resume data_pack_hash mismatch: run=%s loaded=%s", saved, dataPackHash)
	}
	return nil
}

func initResumeDiagnostics(req RunSellerAIPipelineRequest, dataPackSource string) *ResumeDiagnostics {
	if !pipelineResumeMode(req) {
		return nil
	}
	return &ResumeDiagnostics{
		Requested:           true,
		SourcePipelineRunID: req.ExistingPipelineRunID,
		Mode:                resumeModeSameRun,
		DataPackSource:      dataPackSource,
		SharedChunkAICalls:  0,
	}
}

func mapResumeArtifactMissReason(missReason string) string {
	switch missReason {
	case "artifact_not_found":
		return "resume_shared_chunk_artifact_missing"
	case "schema_version_mismatch", "chunk_analysis_schema_version_mismatch", "chunk_set_schema_version_mismatch":
		return "schema_version_mismatch"
	case "artifact_coverage_insufficient":
		return "artifact_coverage_insufficient"
	case "data_pack_hash_mismatch":
		return "resume_data_pack_hash_mismatch"
	case "chunk_hashes_mismatch":
		return "resume_chunk_hashes_mismatch"
	default:
		if missReason != "" {
			return "resume_shared_chunk_artifact_invalid"
		}
		return ""
	}
}

func (s *SellerAIPipelineService) globalContextTimeoutSeconds() int {
	if s == nil || s.gcClient == nil {
		return 0
	}
	if gc, ok := s.gcClient.(*OpenAIGlobalContextClient); ok && gc.TimeoutSeconds > 0 {
		return gc.TimeoutSeconds
	}
	return 0
}
