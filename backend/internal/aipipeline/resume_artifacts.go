package aipipeline

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aihash"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

func (s *SellerAIPipelineService) tryLoadSavedDataPack(
	ctx context.Context,
	pipelineRunID int64,
	sellerAccountID int64,
	metricDate string,
	preferredDataPackHash string,
) (*aidatapack.SellerAIDataPack, error) {
	if s == nil || s.store == nil || pipelineRunID <= 0 {
		return nil, nil
	}
	artifacts, err := s.store.ListArtifacts(ctx, pipelineRunID)
	if err != nil {
		return nil, err
	}
	var candidates []dbgen.AiPipelineArtifact
	for i := range artifacts {
		if artifacts[i].Kind == ArtifactKindDataPack {
			candidates = append(candidates, artifacts[i])
		}
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	sortPipelineArtifactsNewestFirst(candidates)
	preferredDataPackHash = strings.TrimSpace(preferredDataPackHash)
	var fallback *aidatapack.SellerAIDataPack
	for i := range candidates {
		pack, missReason, decodeErr := decodeSavedDataPackArtifact(candidates[i], sellerAccountID, metricDate)
		if decodeErr != nil {
			return nil, decodeErr
		}
		if missReason != "" || pack == nil {
			continue
		}
		if preferredDataPackHash == "" {
			return pack, nil
		}
		doc, err := artifactDocumentPayload(candidates[i].PayloadJson)
		if err != nil {
			continue
		}
		metaHash := artifactMetaString(doc.Meta, "data_pack_hash")
		packHash := aihash.HashDataPack(pack)
		if metaHash == preferredDataPackHash || packHash == preferredDataPackHash {
			return pack, nil
		}
		if fallback == nil {
			fallback = pack
		}
	}
	if preferredDataPackHash != "" {
		return nil, nil
	}
	return fallback, nil
}

func decodeSavedDataPackArtifact(
	art dbgen.AiPipelineArtifact,
	sellerAccountID int64,
	metricDate string,
) (*aidatapack.SellerAIDataPack, string, error) {
	doc, err := artifactDocumentPayload(art.PayloadJson)
	if err != nil {
		return nil, "artifact_payload_invalid", nil
	}
	raw, err := json.Marshal(doc.Payload)
	if err != nil {
		return nil, "artifact_payload_invalid", nil
	}
	var pack aidatapack.SellerAIDataPack
	if err := json.Unmarshal(raw, &pack); err != nil {
		return nil, "artifact_payload_invalid", nil
	}
	if pack.SellerContext.SellerAccountID != sellerAccountID {
		return nil, "seller_account_id_mismatch", nil
	}
	if pack.SellerContext.MetricDate != metricDate {
		return nil, "metric_date_mismatch", nil
	}
	if err := aidatapack.ValidateDataPack(&pack); err != nil {
		return nil, "data_pack_invalid", nil
	}
	return &pack, "", nil
}

func persistedResumeHashesCompatible(persisted dbgen.AiPipelineRun, dataPackHash, globalContextHash string) bool {
	savedDataPackHash := strings.TrimSpace(persisted.DataPackHash)
	if savedDataPackHash != "" && savedDataPackHash != dataPackHash {
		return false
	}
	savedGlobalContextHash := textOrEmpty(persisted.GlobalContextHash)
	if savedGlobalContextHash != "" && savedGlobalContextHash != globalContextHash {
		return false
	}
	return true
}

func persistedDashboardCompleted(persisted dbgen.AiPipelineRun) bool {
	return persisted.DashboardRunID.Valid && textOrEmpty(persisted.DashboardStatus) == StepStatusCompleted
}

func (s *SellerAIPipelineService) canReusePersistedDashboard(
	ctx context.Context,
	persisted dbgen.AiPipelineRun,
	sellerAccountID int64,
	dataPackHash string,
	globalContextHash string,
) bool {
	if !persistedDashboardCompleted(persisted) || !persistedResumeHashesCompatible(persisted, dataPackHash, globalContextHash) {
		return false
	}
	if s == nil || s.store == nil {
		return false
	}
	dashID := uuidString(persisted.DashboardRunID)
	if dashID == "" {
		return false
	}
	dashDataPackHash, dashGlobalContextHash, err := s.store.GetDashboardNarrativeHashes(ctx, dashID, sellerAccountID)
	if err != nil {
		return false
	}
	if strings.TrimSpace(dashDataPackHash) != strings.TrimSpace(dataPackHash) {
		return false
	}
	if strings.TrimSpace(dashGlobalContextHash) != "" && strings.TrimSpace(dashGlobalContextHash) != strings.TrimSpace(globalContextHash) {
		return false
	}
	return true
}

func (s *SellerAIPipelineService) persistedActionsReusable(
	ctx context.Context,
	persisted dbgen.AiPipelineRun,
	sellerAccountID int64,
	dataPackHash string,
	globalContextHash string,
) bool {
	if !persisted.ActionsRunID.Valid || persisted.ActionsRunID.Int64 <= 0 {
		return false
	}
	if s == nil || s.store == nil {
		return false
	}
	finalStatus := textOrEmpty(persisted.ActionsStatus)
	if !actions.IsSuccessfulActionFinalStatus(finalStatus) {
		linkedStatus, err := s.store.GetActionGenerationRunFinalStatus(ctx, persisted.ActionsRunID.Int64, sellerAccountID)
		if err != nil || !actions.IsSuccessfulActionFinalStatus(linkedStatus) {
			return false
		}
		finalStatus = linkedStatus
	}
	if !actions.IsSuccessfulActionFinalStatus(finalStatus) {
		return false
	}
	dp, gch, err := s.store.GetActionGenerationRunHashes(ctx, persisted.ActionsRunID.Int64, sellerAccountID)
	if err != nil {
		return false
	}
	if strings.TrimSpace(dp) != strings.TrimSpace(dataPackHash) {
		return false
	}
	if strings.TrimSpace(gch) != "" && strings.TrimSpace(gch) != strings.TrimSpace(globalContextHash) {
		return false
	}
	return true
}

func artifactMetaString(meta map[string]any, key string) string {
	if meta == nil {
		return ""
	}
	raw, ok := meta[key]
	if !ok || raw == nil {
		return ""
	}
	if s, ok := raw.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}
