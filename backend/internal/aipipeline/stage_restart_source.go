package aipipeline

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aihash"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

type stageRestartSourceBundle struct {
	Run            dbgen.AiPipelineRun
	DataPack       *aidatapack.SellerAIDataPack
	DataPackHash   string
	ChunkSet       *aidatapack.ChunkSet
	SharedAnalysis *dashboardhome.ChunkAnalysisRunResult
	GlobalContext  loadedGlobalContext
}

func (s *SellerAIPipelineService) loadActionsStageRestartSource(
	ctx context.Context,
	req RunSellerAIPipelineRequest,
) (stageRestartSourceBundle, string, error) {
	out := stageRestartSourceBundle{}
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
		return out, "stage_restart_source_not_found", fmt.Errorf("source seller_account_id mismatch: run=%d request=%d", sourceRun.SellerAccountID, req.SellerAccountID)
	}
	if !sourceRun.MetricDate.Valid {
		return out, "stage_restart_source_not_found", fmt.Errorf("source pipeline run %d metric_date is missing", req.SourcePipelineRunID)
	}
	metricDateStr := sourceRun.MetricDate.Time.Format("2006-01-02")
	if strings.TrimSpace(sourceRun.DataPackHash) == "" {
		return out, "stage_restart_data_pack_artifact_missing", fmt.Errorf("source pipeline run %d data_pack_hash is missing", req.SourcePipelineRunID)
	}
	if !sourceRun.GlobalContextHash.Valid || strings.TrimSpace(sourceRun.GlobalContextHash.String) == "" {
		return out, "stage_restart_global_context_artifact_missing", fmt.Errorf("source pipeline run %d global_context_hash is missing", req.SourcePipelineRunID)
	}
	if textOrEmpty(sourceRun.GlobalContextStatus) != GlobalContextStatusCompleted {
		return out, "stage_restart_global_context_artifact_missing", fmt.Errorf("source pipeline global_context_status=%q", textOrEmpty(sourceRun.GlobalContextStatus))
	}
	if !persistedDashboardCompleted(sourceRun) {
		return out, "stage_restart_dashboard_missing", fmt.Errorf("source pipeline dashboard is not completed")
	}
	if !sourceRun.DashboardRunID.Valid {
		return out, "stage_restart_dashboard_missing", fmt.Errorf("source pipeline dashboard_run_id is missing")
	}
	if !sourceRun.DashboardDataSnapshotID.Valid || sourceRun.DashboardDataSnapshotID.Int64 <= 0 {
		return out, "stage_restart_dashboard_snapshot_missing", fmt.Errorf("source pipeline dashboard_data_snapshot_id is missing")
	}

	cloneStore, ok := s.store.(DashboardCloneStore)
	if !ok {
		return out, "stage_restart_dashboard_clone_failed", fmt.Errorf("dashboard clone store is not configured")
	}
	snapshot, err := cloneStore.GetDashboardDataSnapshotByPipelineRunID(ctx, req.SourcePipelineRunID)
	if err != nil {
		return out, "stage_restart_dashboard_snapshot_missing", fmt.Errorf("source dashboard data snapshot: %w", err)
	}
	if snapshot.AiPipelineRunID != req.SourcePipelineRunID {
		return out, "stage_restart_dashboard_snapshot_missing", fmt.Errorf("source dashboard snapshot pipeline mismatch")
	}
	if strings.TrimSpace(snapshot.DataPackHash) != strings.TrimSpace(sourceRun.DataPackHash) {
		return out, "stage_restart_hash_mismatch", fmt.Errorf("source dashboard snapshot data_pack_hash mismatch")
	}
	if strings.TrimSpace(snapshot.GlobalContextHash) != strings.TrimSpace(sourceRun.GlobalContextHash.String) {
		return out, "stage_restart_hash_mismatch", fmt.Errorf("source dashboard snapshot global_context_hash mismatch")
	}
	if !snapshot.MetricDate.Valid || snapshot.MetricDate.Time.Format("2006-01-02") != metricDateStr {
		return out, "stage_restart_hash_mismatch", fmt.Errorf("source dashboard snapshot metric_date mismatch")
	}
	if strings.TrimSpace(snapshot.PayloadHash) == "" {
		return out, "stage_restart_dashboard_snapshot_missing", fmt.Errorf("source dashboard snapshot payload_hash is missing")
	}
	snapStatus := strings.ToLower(strings.TrimSpace(snapshot.Status))
	if snapStatus != "completed" && snapStatus != "available" {
		return out, "stage_restart_dashboard_snapshot_missing", fmt.Errorf("source dashboard snapshot status=%q is not publishable", snapshot.Status)
	}

	pack, err := s.tryLoadSavedDataPack(ctx, req.SourcePipelineRunID, sourceRun.SellerAccountID, metricDateStr, sourceRun.DataPackHash)
	if err != nil {
		return out, "stage_restart_data_pack_artifact_missing", err
	}
	if pack == nil {
		return out, "stage_restart_data_pack_artifact_missing", fmt.Errorf("source pipeline run %d data_pack artifact is missing", req.SourcePipelineRunID)
	}
	packHash := aihash.HashDataPack(pack)
	if packHash != strings.TrimSpace(sourceRun.DataPackHash) {
		return out, "stage_restart_hash_mismatch", fmt.Errorf("loaded data_pack_hash mismatch: run=%s loaded=%s", sourceRun.DataPackHash, packHash)
	}
	out.DataPack = pack
	out.DataPackHash = packHash

	var gcInput GlobalContextGenerationInput
	var gcInputHash string
	if SharedChunkAnalysisEnabled() {
		chunkSet, sharedRun, code, sharedErr := s.loadSourceSharedChunkAnalysis(ctx, req.SourcePipelineRunID, sourceRun, pack, packHash, metricDateStr)
		if sharedErr != nil {
			return out, code, sharedErr
		}
		out.ChunkSet = chunkSet
		out.SharedAnalysis = sharedRun
		gcInput, err = BuildGlobalContextGenerationInput(pack, packHash, chunkSet, sharedRun)
		if err != nil {
			return out, "stage_restart_global_context_artifact_missing", err
		}
	} else {
		gcInput = DefaultGlobalContextGenerationInput(pack, packHash)
	}
	gcInputHash, err = HashGlobalContextGenerationInput(gcInput)
	if err != nil {
		return out, "stage_restart_global_context_artifact_missing", err
	}
	loadedGC, err := s.tryLoadSavedGlobalContext(ctx, req.SourcePipelineRunID, gcInput, gcInputHash, strings.TrimSpace(sourceRun.GlobalContextHash.String))
	if err != nil {
		return out, "stage_restart_global_context_artifact_missing", err
	}
	if loadedGC.Context == nil {
		return out, "stage_restart_global_context_artifact_missing", fmt.Errorf("source global context artifact is missing (miss_reason=%s)", loadedGC.MissReason)
	}
	if loadedGC.ContextHash != strings.TrimSpace(sourceRun.GlobalContextHash.String) {
		return out, "stage_restart_hash_mismatch", fmt.Errorf("loaded global_context_hash mismatch")
	}
	out.GlobalContext = loadedGC
	return out, "", nil
}

func (s *SellerAIPipelineService) loadSourceSharedChunkAnalysis(
	ctx context.Context,
	sourcePipelineRunID int64,
	sourceRun dbgen.AiPipelineRun,
	pack *aidatapack.SellerAIDataPack,
	dataPackHash string,
	metricDateStr string,
) (*aidatapack.ChunkSet, *dashboardhome.ChunkAnalysisRunResult, string, error) {
	chunkSet, err := s.chunkSetBuilder()(pack, aidatapack.DefaultChunkOptions())
	if err != nil {
		return nil, nil, "stage_restart_chunk_set_artifact_missing", fmt.Errorf("build chunk set from source data pack: %w", err)
	}
	chunkHashes, err := orderedChunkHashes(chunkSet)
	if err != nil {
		return nil, nil, "stage_restart_chunk_set_artifact_missing", err
	}
	loadedRun, _, missReason, loadErr := s.loadSavedChunkAnalysis(
		ctx,
		sourcePipelineRunID,
		sourceRun.SellerAccountID,
		metricDateStr,
		dataPackHash,
		chunkSet,
		chunkHashes,
	)
	if loadErr != nil {
		return nil, nil, "stage_restart_shared_analysis_artifact_missing", loadErr
	}
	if loadedRun == nil {
		if missReason == "" {
			missReason = "artifact_not_found"
		}
		return nil, nil, "stage_restart_shared_analysis_artifact_missing", fmt.Errorf("source shared_chunk_analysis artifact missing (miss_reason=%s)", missReason)
	}
	coverage := EvaluateSharedChunkCoverage(chunkSet, loadedRun)
	if !coverage.Passed {
		return nil, nil, "stage_restart_shared_analysis_artifact_missing", fmt.Errorf("source shared_chunk_analysis coverage insufficient: %s", formatSharedChunkCoverageError(coverage))
	}
	return chunkSet, loadedRun, "", nil
}

func stageRestartSourceMetricDate(source dbgen.AiPipelineRun) time.Time {
	if !source.MetricDate.Valid {
		return time.Time{}
	}
	return time.Date(
		source.MetricDate.Time.Year(),
		source.MetricDate.Time.Month(),
		source.MetricDate.Time.Day(),
		0, 0, 0, 0, time.UTC,
	)
}
