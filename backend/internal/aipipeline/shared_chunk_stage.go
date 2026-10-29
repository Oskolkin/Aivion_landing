package aipipeline

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

func (s *SellerAIPipelineService) chunkSetBuilder() chunkSetBuildFunc {
	if s != nil && s.buildChunkSet != nil {
		return s.buildChunkSet
	}
	return dashboardhome.BuildChunkSetForPack
}

func (s *SellerAIPipelineService) prepareSharedChunkAnalysis(
	ctx context.Context,
	pipelineRunID int64,
	pack *aidatapack.SellerAIDataPack,
	dataPackHash string,
	metricDate time.Time,
	req RunSellerAIPipelineRequest,
	artifacts *ArtifactRecorder,
	diag *PipelineDiagnostics,
	providers PipelineProviders,
) (*aidatapack.ChunkSet, *dashboardhome.ChunkAnalysisRunResult, error) {
	if diag != nil {
		diag.SharedChunkAnalysis.Enabled = true
	}
	chunkSet, err := s.chunkSetBuilder()(pack, aidatapack.DefaultChunkOptions())
	if err != nil {
		return nil, nil, &pipelineStageError{code: "chunk_set_build_failed", err: err}
	}
	if chunkSet == nil {
		return nil, nil, &pipelineStageError{
			code: "chunk_set_build_failed",
			err:  errSharedChunkAnalysisNilChunkSet,
		}
	}
	if len(chunkSet.Chunks) == 0 {
		return nil, nil, &pipelineStageError{
			code: "empty_chunk_set",
			err:  errSharedChunkAnalysisEmptyChunkSet,
		}
	}

	chunkHashes, err := orderedChunkHashes(chunkSet)
	if err != nil {
		return nil, nil, &pipelineStageError{code: "chunk_set_build_failed", err: err}
	}

	metricDateStr := metricDate.Format("2006-01-02")
	loadedRun, artifactHash, missReason, loadErr := s.loadSavedChunkAnalysis(
		ctx,
		pipelineRunID,
		pack.SellerContext.SellerAccountID,
		metricDateStr,
		dataPackHash,
		chunkSet,
		chunkHashes,
	)
	if loadErr != nil {
		return nil, nil, &pipelineStageError{code: "shared_chunk_analysis_load_failed", err: loadErr}
	}
	if diag != nil && missReason != "" {
		diag.SharedChunkAnalysis.ArtifactMissReason = missReason
	}

	var chunkRun *dashboardhome.ChunkAnalysisRunResult
	source := "generated"
	if loadedRun != nil {
		chunkRun = loadedRun
		source = "artifact"
		coverage := EvaluateSharedChunkCoverage(chunkSet, chunkRun)
		if diag != nil {
			populateSharedChunkCoverageDiagnostics(diag, coverage)
		}
		if !coverage.Passed {
			if missReason == "" {
				missReason = "artifact_coverage_insufficient"
			}
			code := "shared_chunk_coverage_insufficient"
			if pipelineResumeMode(req) {
				code = "artifact_coverage_insufficient"
			}
			return nil, nil, &pipelineStageError{
				code: code,
				err:  fmt.Errorf("%s", formatSharedChunkCoverageError(coverage)),
			}
		}
	} else {
		if pipelineResumeMode(req) {
			code := "resume_shared_chunk_artifact_missing"
			if mapped := mapResumeArtifactMissReason(missReason); mapped != "" {
				code = mapped
			}
			return nil, nil, &pipelineStageError{
				code: code,
				err:  fmt.Errorf("resume requires saved shared_chunk_analysis artifact (miss_reason=%s)", missReason),
			}
		}
		if err := ctx.Err(); err != nil {
			return nil, nil, &pipelineStageError{code: "shared_chunk_analysis_failed", err: err}
		}
		opts := dashboardhome.NarrativeRunOptions{
			MetricDate:    &metricDate,
			PipelineRunID: &pipelineRunID,
		}
		chunkRun, err = s.chunkAnalyzer.AnalyzeDataPackChunks(ctx, pack, chunkSet, opts)
		if err != nil {
			return nil, nil, &pipelineStageError{code: "shared_chunk_analysis_failed", err: err}
		}
		if chunkRun == nil {
			return nil, nil, &pipelineStageError{
				code: "shared_chunk_analysis_empty_result",
				err:  errSharedChunkAnalysisEmptyResult,
			}
		}
		if err := validateSharedChunkAnalysisRun(chunkSet, chunkRun); err != nil {
			code := "shared_chunk_analysis_invalid_result"
			if errors.Is(err, errSharedChunkAnalysisAllFailed) {
				code = "all_chunks_failed"
			}
			return nil, nil, &pipelineStageError{code: code, err: err}
		}
		normalized, limitations, err := normalizeSharedChunkRun(chunkRun)
		if err != nil {
			code := "shared_chunk_analysis_invalid_result"
			if errors.Is(err, errSharedChunkAnalysisAllFailed) {
				code = "all_chunks_failed"
			}
			return nil, nil, &pipelineStageError{code: code, err: err}
		}
		chunkRun = normalized

		coverage := EvaluateSharedChunkCoverage(chunkSet, chunkRun)
		if diag != nil {
			populateSharedChunkCoverageDiagnostics(diag, coverage)
		}
		if !coverage.Passed {
			if req.DebugArtifacts {
				saveSharedChunkDebugArtifacts(ctx, artifacts, pipelineRunID, chunkSet, chunkRun, dataPackHash)
			}
			return nil, nil, &pipelineStageError{
				code: "shared_chunk_coverage_insufficient",
				err:  fmt.Errorf("%s", formatSharedChunkCoverageError(coverage)),
			}
		}

		artifact, err := buildSharedChunkAnalysisArtifact(pack, dataPackHash, chunkSet, chunkHashes, chunkRun, limitations)
		if err != nil {
			return nil, nil, &pipelineStageError{code: "shared_chunk_analysis_persist_failed", err: err}
		}
		meta := pipelineArtifactMeta(pipelineRunID, map[string]any{
			"seller_account_id": pack.SellerContext.SellerAccountID,
			"metric_date":       metricDateStr,
			"data_pack_hash":    dataPackHash,
			"analysis_status":   chunkRun.AnalysisStatus,
		})
		if err := artifacts.SaveRequiredDocument(ctx, ArtifactKindSharedChunkAnalysis, meta, artifact); err != nil {
			return nil, nil, &pipelineStageError{code: "shared_chunk_analysis_persist_failed", err: err}
		}
		doc := ArtifactDocument{Meta: meta, Payload: scrubArtifactPayload(artifact)}
		if h, hashErr := HashArtifactPayload(doc); hashErr == nil {
			artifactHash = h
		}
		s.recordChunkAnalysisRunUsage(ctx, pack.SellerContext.SellerAccountID, pipelineRunID, providers.Dashboard, chunkRun)
	}

	if req.DebugArtifacts && chunkSet != nil && chunkRun != nil {
		saveSharedChunkDebugArtifacts(ctx, artifacts, pipelineRunID, chunkSet, chunkRun, dataPackHash)
	}

	if diag != nil {
		populateSharedChunkDiagnostics(diag, chunkRun, source, artifactHash, chunkSet)
		if diag.Resume != nil && source == "artifact" {
			diag.Resume.SharedChunkAnalysisSource = "artifact"
			diag.Resume.SharedChunkAnalysisReused = true
			diag.Resume.SharedChunkAICalls = 0
		}
	}
	return chunkSet, chunkRun, nil
}

func populateSharedChunkDiagnostics(
	diag *PipelineDiagnostics,
	chunkRun *dashboardhome.ChunkAnalysisRunResult,
	source string,
	artifactHash string,
	chunkSet *aidatapack.ChunkSet,
) {
	if diag == nil || chunkRun == nil {
		return
	}
	diag.SharedChunkAnalysis.Enabled = true
	diag.SharedChunkAnalysis.Source = source
	diag.SharedChunkAnalysis.Status = chunkRun.AnalysisStatus
	diag.SharedChunkAnalysis.ChunkCount = chunkRun.ChunkCount
	diag.SharedChunkAnalysis.Completed = chunkRun.Completed
	diag.SharedChunkAnalysis.Failed = chunkRun.Failed
	diag.SharedChunkAnalysis.ProviderFailedCount = chunkRun.ProviderFailedCount
	diag.SharedChunkAnalysis.ValidationFailedCount = chunkRun.ValidationFailedCount
	diag.SharedChunkAnalysis.InsufficientDataCount = chunkRun.InsufficientDataCount
	if chunkRun.ChunkCount > 0 {
		diag.SharedChunkAnalysis.UsablePercent = chunkRun.Completed * 100 / chunkRun.ChunkCount
	}
	diag.SharedChunkAnalysis.ValidatedSignalCount = chunkRun.ValidatedSignalCount
	diag.SharedChunkAnalysis.RejectedSignalCount = chunkRun.RejectedSignalCount
	diag.SharedChunkAnalysis.ValidatedCandidateCount = chunkRun.ValidatedCandidateCount
	diag.SharedChunkAnalysis.RejectedCandidateCount = chunkRun.RejectedCandidateCount
	diag.SharedChunkAnalysis.FailedChunkIDs = append([]string(nil), chunkRun.FailedChunkIDs...)
	if source == "artifact" {
		diag.SharedChunkAnalysis.InputTokens = 0
		diag.SharedChunkAnalysis.OutputTokens = 0
		diag.SharedChunkAnalysis.UsageSource = UsageSourceArtifact
	} else {
		diag.SharedChunkAnalysis.InputTokens = chunkRun.InputTokens
		diag.SharedChunkAnalysis.OutputTokens = chunkRun.OutputTokens
	}
	diag.SharedChunkAnalysis.ChunkCalls = append([]dashboardhome.ChunkCallUsage(nil), chunkRun.ChunkCalls...)
	diag.SharedChunkAnalysis.ArtifactHash = artifactHash
	if chunkRun.AnalysisStatus == dashboardhome.ChunkAnalysisStatusPartial {
		diag.SharedChunkAnalysis.PartialLimitation = sharedChunkAnalysisPartialLimitation
		diag.SharedChunkAnalysis.PartialLimitationCode = sharedChunkAnalysisPartialLimitationCode
	}
	_ = chunkSet
}

func populateSharedChunkCoverageDiagnostics(diag *PipelineDiagnostics, coverage SharedChunkCoverageResult) {
	if diag == nil {
		return
	}
	diag.SharedChunkAnalysis.CoverageGatePassed = coverage.Passed
	diag.SharedChunkAnalysis.CoverageFailureReasons = append([]string(nil), coverage.FailureReasons...)
	if len(coverage.FamilyCoverage) > 0 {
		diag.SharedChunkAnalysis.FamilyCoverage = make(map[string]FamilyCoverage, len(coverage.FamilyCoverage))
		for k, v := range coverage.FamilyCoverage {
			diag.SharedChunkAnalysis.FamilyCoverage[k] = v
		}
	}
	diag.SharedChunkAnalysis.UsablePercent = coverage.UsablePercent
}

func (s *SellerAIPipelineService) loadSavedChunkAnalysis(
	ctx context.Context,
	pipelineRunID int64,
	sellerAccountID int64,
	metricDate string,
	dataPackHash string,
	chunkSet *aidatapack.ChunkSet,
	expectedChunkHashes []string,
) (*dashboardhome.ChunkAnalysisRunResult, string, string, error) {
	artifacts, err := s.store.ListArtifacts(ctx, pipelineRunID)
	if err != nil {
		return nil, "", "", err
	}
	var latest *dbgen.AiPipelineArtifact
	for i := range artifacts {
		if artifacts[i].Kind == ArtifactKindSharedChunkAnalysis {
			cp := artifacts[i]
			latest = &cp
		}
	}
	if latest == nil {
		return nil, "", "artifact_not_found", nil
	}
	doc, err := artifactDocumentPayload(latest.PayloadJson)
	if err != nil {
		return nil, "", "artifact_payload_invalid", nil
	}
	artifact, err := decodeSharedChunkAnalysisArtifactPayload(doc.Payload)
	if err != nil {
		return nil, "", "artifact_payload_invalid", nil
	}
	storedHash := textOrEmpty(latest.Hash)
	computedHash, err := HashArtifactPayload(doc)
	if err != nil {
		return nil, "", "artifact_hash_compute_failed", nil
	}
	missReason, _ := validateStoredSharedChunkAnalysisArtifact(
		artifact,
		sellerAccountID,
		metricDate,
		dataPackHash,
		chunkSet,
		expectedChunkHashes,
		storedHash,
		computedHash,
	)
	if missReason != "" {
		return nil, "", missReason, nil
	}
	return chunkAnalysisRunFromArtifact(artifact), computedHash, "", nil
}

type pipelineStageError struct {
	code string
	err  error
}

func (e *pipelineStageError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *pipelineStageError) Code() string {
	if e == nil {
		return ""
	}
	return e.code
}

func pipelineStageFailure(err error) (string, error) {
	var stageErr *pipelineStageError
	if errors.As(err, &stageErr) {
		return stageErr.Code(), stageErr.err
	}
	return "shared_chunk_analysis_failed", err
}
