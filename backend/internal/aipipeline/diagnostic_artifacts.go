package aipipeline

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
)

func buildMetricSemanticsSnapshotArtifact(usedCodes []string) map[string]any {
	defs := aidatapack.MetricSemanticsForCodes(usedCodes...)
	if len(defs) == 0 {
		defs = aidatapack.MetricSemanticsSnapshot()
	}
	raw, _ := json.Marshal(defs)
	sum := sha256.Sum256(raw)
	return map[string]any{
		"schema_version":    MetricSemanticsSnapshotSchemaVersion,
		"registry_version":  "metric_semantics_v1",
		"registry_hash":     "sha256:" + hex.EncodeToString(sum[:]),
		"definitions":       defs,
		"used_metric_codes": usedCodes,
	}
}

func collectUsedMetricCodesFromPack(pack *aidatapack.SellerAIDataPack) []string {
	if pack == nil {
		return nil
	}
	seen := map[string]struct{}{}
	var out []string
	for _, fact := range pack.FactRegistry {
		metric := strings.TrimSpace(fact.Metric)
		if metric == "" {
			continue
		}
		if _, ok := seen[metric]; ok {
			continue
		}
		seen[metric] = struct{}{}
		out = append(out, metric)
	}
	return out
}

func buildSharedChunkRefMappingArtifact(chunkSet *aidatapack.ChunkSet, chunkRun *dashboardhome.ChunkAnalysisRunResult) map[string]any {
	chunks := make([]map[string]any, 0)
	if chunkSet != nil {
		for _, ch := range chunkSet.Chunks {
			mapping := dashboardhome.BuildChunkRefMapping(ch)
			entry := map[string]any{
				"chunk_id":    ch.ChunkID,
				"chunk_hash":  ch.ChunkHash,
				"fact_refs":   enrichFactRefMapping(ch, mapping),
				"entity_refs": enrichEntityRefMapping(mapping),
			}
			if chunkRun != nil {
				for _, analysis := range chunkRun.Analyses {
					if analysis.ChunkID != ch.ChunkID {
						continue
					}
					factUnknown, entityUnknown := 0, 0
					if analysis.SchemaVersion == dashboardhome.DashboardChunkAnalysisSchemaVersionV3 {
						_, factUnknown, entityUnknown = dashboardhome.ResolveChunkAnalysisRefStats(&analysis, mapping)
					}
					returnedFacts, returnedEntities := dashboardhome.CountChunkRefUsage(analysis)
					entry["returned_refs"] = map[string]any{
						"fact_refs_total":     returnedFacts,
						"fact_refs_valid":     returnedFacts - factUnknown,
						"fact_refs_unknown":   factUnknown,
						"entity_refs_total":   returnedEntities,
						"entity_refs_valid":   returnedEntities - entityUnknown,
						"entity_refs_unknown": entityUnknown,
					}
				}
			}
			chunks = append(chunks, entry)
		}
	}
	return map[string]any{
		"schema_version": SharedChunkRefMappingSchemaVersion,
		"chunks":         chunks,
	}
}

func enrichFactRefMapping(chunk aidatapack.DataPackChunk, mapping dashboardhome.ChunkRefMapping) map[string]any {
	out := map[string]any{}
	for ref, factID := range mapping.FactRefs {
		entry := map[string]any{"fact_id": factID}
		if fact, ok := chunk.LocalFactCatalog[factID]; ok {
			entry["metric_code"] = fact.Metric
			entry["entity_id"] = fact.EntityID
		}
		out[ref] = entry
	}
	return out
}

func enrichEntityRefMapping(mapping dashboardhome.ChunkRefMapping) map[string]any {
	out := map[string]any{}
	for ref, entityID := range mapping.EntityRefs {
		parts := strings.SplitN(entityID, ":", 2)
		entityType := ""
		if len(parts) > 0 {
			entityType = parts[0]
		}
		out[ref] = map[string]any{
			"entity_id":   entityID,
			"entity_type": entityType,
		}
	}
	return out
}

func buildGlobalContextValidationArtifact(
	beforeStatus, afterStatus, beforeConfidence, afterConfidence string,
	validation GlobalContextValidationResult,
) map[string]any {
	blocking, warnings := 0, 0
	issues := make([]map[string]any, 0, len(validation.Issues))
	for _, issue := range validation.Issues {
		if issue.Severity == "fatal" || issue.Severity == "error" {
			blocking++
		} else {
			warnings++
		}
		issues = append(issues, map[string]any{
			"issue_code": issue.Code,
			"severity":   issue.Severity,
			"field_path": issue.Field,
			"message":    issue.Message,
		})
	}
	return map[string]any{
		"schema_version":               GlobalContextValidationArtifactSchemaVersion,
		"status_before_validation":     beforeStatus,
		"status_after_validation":      afterStatus,
		"confidence_before_validation": beforeConfidence,
		"confidence_after_validation":  afterConfidence,
		"blocking_issue_count":         blocking,
		"warning_count":                warnings,
		"issues":                       issues,
	}
}

func saveDiagnosticArtifacts(
	ctx context.Context,
	recorder *ArtifactRecorder,
	runID int64,
	meta map[string]any,
	debug bool,
	metricSnapshot map[string]any,
	chunkRefMapping map[string]any,
	gcValidation map[string]any,
	finalValidation FinalOutputSemanticValidation,
	pubGuard PublicationGuardDecision,
	actionsExec map[string]any,
) {
	if recorder == nil {
		return
	}
	baseMeta := pipelineArtifactMeta(runID, meta)
	if metricSnapshot != nil {
		_ = recorder.SaveRequiredDocument(ctx, ArtifactKindMetricSemanticsSnapshot, baseMeta, metricSnapshot)
	}
	compactGC := gcValidation
	if !debug && gcValidation != nil {
		compactGC = map[string]any{
			"schema_version":              gcValidation["schema_version"],
			"status_after_validation":     gcValidation["status_after_validation"],
			"confidence_after_validation": gcValidation["confidence_after_validation"],
			"blocking_issue_count":        gcValidation["blocking_issue_count"],
			"warning_count":               gcValidation["warning_count"],
		}
	}
	_ = recorder.SaveRequiredDocument(ctx, ArtifactKindGlobalContextValidation, baseMeta, compactGC)
	compactGuard := map[string]any{
		"schema_version":       pubGuard.SchemaVersion,
		"requested_outputs":    pubGuard.RequestedOutputs,
		"dashboard_validation": pubGuard.DashboardValidation,
		"actions_validation":   pubGuard.ActionsValidation,
		"blocking_issue_codes": pubGuard.BlockingIssueCodes,
		"publication_allowed":  pubGuard.PublicationAllowed,
		"decision":             pubGuard.Decision,
		"decision_reason":      pubGuard.DecisionReason,
	}
	_ = recorder.SaveRequiredDocument(ctx, ArtifactKindPublicationGuardDecision, baseMeta, compactGuard)
	if debug {
		if chunkRefMapping != nil {
			_ = recorder.SaveDocument(ctx, ArtifactKindSharedChunkRefMapping, baseMeta, chunkRefMapping)
		}
		_ = recorder.SaveDocument(ctx, ArtifactKindFinalOutputSemanticValidation, baseMeta, finalValidation)
		_ = recorder.SaveDocument(ctx, ArtifactKindPublicationGuardDecision, baseMeta, pubGuard)
		if gcValidation != nil {
			_ = recorder.SaveDocument(ctx, ArtifactKindGlobalContextValidation, baseMeta, gcValidation)
		}
		if actionsExec != nil {
			_ = recorder.SaveDocument(ctx, ArtifactKindActionsExecutionDiagnostics, baseMeta, actionsExec)
		}
	}
}

func pipelineArtifactMetaWithHashes(runID int64, sellerID int64, metricDate, dataPackHash, gcHash string) map[string]any {
	return map[string]any{
		"pipeline_run_id":     runID,
		"seller_account_id":   sellerID,
		"metric_date":         metricDate,
		"data_pack_hash":      dataPackHash,
		"global_context_hash": gcHash,
		"created_at":          time.Now().UTC().Format(time.RFC3339),
	}
}
