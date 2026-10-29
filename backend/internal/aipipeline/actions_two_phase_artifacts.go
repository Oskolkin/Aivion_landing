package aipipeline

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
)

func saveTwoPhaseActionsArtifacts(ctx context.Context, artifacts *ArtifactRecorder, runID int64, meta map[string]any, res *actions.GenerateActionsResult) {
	if artifacts == nil || res == nil {
		return
	}
	if res.ComposeOutput == nil && res.ObjectDetailCalls == 0 && len(res.NormalizationEvents) == 0 {
		return
	}
	if len(res.ComposeWireInput) > 0 {
		_ = artifacts.SaveDocument(ctx, ArtifactKindActionsComposeInput, meta, json.RawMessage(res.ComposeWireInput))
	}

	composeMaxTokens := actions.ActionsComposeRequestedMaxOutputTokens()
	composeTimeout := actions.ActionsComposeTimeoutSeconds()
	composeEnv := BuildAICallDiagnosticEnvelopeFromRaw(
		"actions_compose", AIUsageComponentActionsSynthesis, "openai",
		res.ComposeRawResponse, composeMaxTokens, composeTimeout,
		time.Time{}, time.Time{},
		ParseStatusParsed, "",
	)
	composeEnv.PipelineRunID = runID
	if res.ComposeRawResponse != nil {
		composeEnv.InputSchemaVersion = res.ComposeRawResponse.InputSchemaVersion
		composeEnv.OutputSchemaVersion = actions.ActionsAIFirstSchemaVersionV2
	}
	rawBody := ""
	if res.ComposeRawResponse != nil {
		rawBody = res.ComposeRawResponse.RawText
	}
	_ = artifacts.SaveRequiredDocument(ctx, ArtifactKindActionsComposeCallDiagnostics, meta, composeEnv)
	_ = artifacts.SaveDocument(ctx, ArtifactKindActionsComposeRawResponse, meta,
		BuildAICallRawResponseArtifact(composeEnv, rawBody))

	if res.ComposeOutput != nil {
		_ = artifacts.SaveDocument(ctx, ArtifactKindActionsComposeParsedOutput, meta,
			BuildActionsComposeParsedOutputArtifact(res.ComposeOutput))
		_ = artifacts.SaveDocument(ctx, ArtifactKindActionsComposeOutput, meta, res.ComposeOutput)
	}
	if len(res.BaseActions) > 0 {
		_ = artifacts.SaveDocument(ctx, ArtifactKindActionsComposeValidation, meta, map[string]any{
			"base_actions": res.BaseActions,
		})
	}
	if len(res.NormalizationEvents) > 0 {
		_ = artifacts.SaveDocument(ctx, ArtifactKindActionsNormalization, meta, map[string]any{
			"schema_version": "actions_normalization_v1",
			"events":         res.NormalizationEvents,
		})
	}
	if len(res.DetailMerges) > 0 {
		_ = artifacts.SaveDocument(ctx, ArtifactKindActionsObjectDetailMerge, meta, map[string]any{
			"merges": res.DetailMerges,
		})
	}
	_ = artifacts.SaveDocument(ctx, ArtifactKindActionsFinalOutput, meta, buildActionsOutputArtifact(res))
	if lifecycle := BuildActionsLifecycleArtifactFromResult(res); lifecycle != nil {
		_ = artifacts.SaveDocument(ctx, ArtifactKindActionsLifecycleDiagnostics, meta, lifecycle)
	}
	for ai, merge := range res.DetailMerges {
		for _, br := range merge.BatchResults {
			batchMeta := copyMeta(meta)
			batchMeta["action_index"] = ai
			batchMeta["action_key"] = merge.ActionKey
			batchMeta["batch_index"] = br.BatchIndex
			batchMeta["batch_count"] = br.BatchCount
			batchMeta["entity_count"] = br.EntityCount
			_ = artifacts.SaveDocument(ctx, ArtifactKindActionObjectDetailInput, batchMeta, br.Input)
			if br.Output != nil {
				_ = artifacts.SaveDocument(ctx, ArtifactKindActionObjectDetailOutput, batchMeta, br.Output)
			}
			if br.RawResponse != nil {
				batchIdx := br.BatchIndex
				detailEnv := BuildAICallDiagnosticEnvelopeFromRaw(
					"action_object_detail", "action_object_detail", "openai",
					br.RawResponse, actions.ActionsObjectDetailRequestedMaxOutputTokens(),
					actions.ActionsObjectDetailTimeoutSeconds(),
					time.Time{}, time.Time{}, ParseStatusParsed, "",
				)
				detailEnv.PipelineRunID = runID
				detailEnv.ActionKey = merge.ActionKey
				detailEnv.BatchIndex = &batchIdx
				detailBody := br.RawResponse.RawText
				_ = artifacts.SaveDocument(ctx, ArtifactKindActionObjectDetailCallDiagnostics, batchMeta, detailEnv)
				_ = artifacts.SaveDocument(ctx, ArtifactKindActionObjectDetailRawResponse, batchMeta,
					BuildAICallRawResponseArtifact(detailEnv, detailBody))
			}
			if len(br.Warnings) > 0 || br.Status != "" {
				_ = artifacts.SaveDocument(ctx, ArtifactKindActionObjectDetailValidation, batchMeta, map[string]any{
					"warnings": br.Warnings, "status": br.Status, "mapping": br.Mapping,
				})
			}
		}
	}
	if res.PipelineDiagnostics != nil && res.PipelineDiagnostics.CandidateCoverage != nil {
		_ = artifacts.SaveDocument(ctx, ArtifactKindAIPipelineLineage, meta, buildPipelineLineageArtifact(res))
	}
}

func buildComposeRawOutputArtifact(raw *actions.AIRawResponse) map[string]any {
	if raw == nil {
		return nil
	}
	env := BuildAICallDiagnosticEnvelopeFromRaw(
		"actions_compose", AIUsageComponentActionsSynthesis, "openai",
		raw, raw.RequestedMaxOutputTokens, raw.TimeoutSeconds,
		time.Time{}, time.Time{}, ParseStatusUnknown, raw.ParseError,
	)
	body := raw.RawText
	return BuildAICallRawResponseArtifact(env, body)
}

func buildPipelineLineageArtifact(res *actions.GenerateActionsResult) map[string]any {
	lineage := map[string]any{}
	if res.PipelineDiagnostics != nil && res.PipelineDiagnostics.CandidateCoverage != nil {
		lineage["candidate_coverage"] = res.PipelineDiagnostics.CandidateCoverage
	}
	if len(res.NormalizationEvents) > 0 {
		lineage["normalization_events"] = res.NormalizationEvents
	}
	if len(res.ActionLifecycles) > 0 {
		lineage["action_lifecycle"] = res.ActionLifecycles
		lineage["lifecycle_summary"] = res.LifecycleSummary
	}
	if len(res.BaseActions) > 0 {
		actionsLineage := make(map[string]any, len(res.BaseActions))
		for _, base := range res.BaseActions {
			key := base.ActionKey
			if key == "" {
				continue
			}
			actionsLineage[key] = map[string]any{
				"candidate_ids":            base.ValidSourceCandidates,
				"authoritative_entity_ids": entityIDsFromRefs(base.AuthoritativeEntities),
				"final_status":             baseOutcomeStatus(base),
			}
		}
		lineage["actions"] = actionsLineage
	}
	return lineage
}

func entityIDsFromRefs(refs []actions.AIFirstAffectedEntityRef) []string {
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		if ref.EntityID != "" {
			out = append(out, ref.EntityID)
		}
	}
	return out
}

func baseOutcomeStatus(base actions.AIFirstBaseAction) string {
	if base.Dropped {
		return "dropped"
	}
	return "persisted"
}

func copyMeta(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
