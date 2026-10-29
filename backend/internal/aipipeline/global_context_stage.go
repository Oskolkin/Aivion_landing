package aipipeline

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicontext"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

type loadedGlobalContext struct {
	Context      *aicontext.GlobalSemanticContext
	ContextHash  string
	ArtifactHash string
	MissReason   string
}

func (s *SellerAIPipelineService) tryLoadSavedGlobalContext(
	ctx context.Context,
	pipelineRunID int64,
	input GlobalContextGenerationInput,
	inputHash string,
	preferredContextHash string,
) (loadedGlobalContext, error) {
	out := loadedGlobalContext{}
	artifacts, err := s.store.ListArtifacts(ctx, pipelineRunID)
	if err != nil {
		return out, err
	}
	var candidates []dbgen.AiPipelineArtifact
	for i := range artifacts {
		if artifacts[i].Kind == ArtifactKindGlobalContextFinal {
			candidates = append(candidates, artifacts[i])
		}
	}
	if len(candidates) == 0 {
		out.MissReason = "artifact_not_found"
		return out, nil
	}
	sortPipelineArtifactsNewestFirst(candidates)
	preferredContextHash = strings.TrimSpace(preferredContextHash)
	if preferredContextHash != "" {
		for i := range candidates {
			loaded, missReason, loadErr := decodeAndValidateGlobalContextArtifactByHashHint(candidates[i], input, preferredContextHash)
			if loadErr != nil {
				return out, loadErr
			}
			if missReason == "" && loaded.Context != nil {
				loaded.MissReason = ""
				return loaded, nil
			}
		}
	}
	for i := range candidates {
		loaded, missReason, loadErr := decodeAndValidateGlobalContextArtifact(candidates[i], input, inputHash, "")
		if loadErr != nil {
			return out, loadErr
		}
		if missReason == "" && loaded.Context != nil {
			loaded.MissReason = ""
			return loaded, nil
		}
		if out.MissReason == "" {
			out.MissReason = missReason
		}
	}
	return out, nil
}

func pipelineArtifactCreatedAt(art dbgen.AiPipelineArtifact) time.Time {
	if art.CreatedAt.Valid {
		return art.CreatedAt.Time.UTC()
	}
	return time.Time{}
}

// sortPipelineArtifactsNewestFirst orders artifacts by created_at DESC, then id DESC.
// Loader policy: evaluate newest compatible artifact first regardless of repository return order.
func sortPipelineArtifactsNewestFirst(artifacts []dbgen.AiPipelineArtifact) {
	slices.SortFunc(artifacts, func(a, b dbgen.AiPipelineArtifact) int {
		ta := pipelineArtifactCreatedAt(a)
		tb := pipelineArtifactCreatedAt(b)
		switch {
		case ta.After(tb):
			return -1
		case ta.Before(tb):
			return 1
		}
		switch {
		case a.ID > b.ID:
			return -1
		case a.ID < b.ID:
			return 1
		default:
			return 0
		}
	})
}

func decodeAndValidateGlobalContextArtifact(
	latest dbgen.AiPipelineArtifact,
	input GlobalContextGenerationInput,
	inputHash string,
	preferredContextHash string,
) (loadedGlobalContext, string, error) {
	out := loadedGlobalContext{}
	doc, err := artifactDocumentPayload(latest.PayloadJson)
	if err != nil {
		return out, "artifact_payload_invalid", nil
	}
	artifact, err := decodeGlobalContextArtifactPayload(doc.Payload)
	if err != nil {
		return out, "artifact_payload_invalid", nil
	}
	if preferredContextHash != "" && artifact.GlobalContextHash != preferredContextHash {
		return out, "global_context_hash_mismatch", nil
	}
	storedHash := textOrEmpty(latest.Hash)
	computedHash, err := HashArtifactPayload(doc)
	if err != nil {
		return out, "artifact_hash_compute_failed", nil
	}
	missReason, _ := validateStoredGlobalContextArtifact(
		artifact,
		input,
		inputHash,
		storedHash,
		computedHash,
	)
	if missReason != "" {
		return out, missReason, nil
	}
	ctx := artifact.GlobalContext
	return loadedGlobalContext{
		Context:      &ctx,
		ContextHash:  artifact.GlobalContextHash,
		ArtifactHash: computedHash,
	}, "", nil
}

func decodeAndValidateGlobalContextArtifactByHashHint(
	latest dbgen.AiPipelineArtifact,
	input GlobalContextGenerationInput,
	preferredContextHash string,
) (loadedGlobalContext, string, error) {
	out := loadedGlobalContext{}
	doc, err := artifactDocumentPayload(latest.PayloadJson)
	if err != nil {
		return out, "artifact_payload_invalid", nil
	}
	artifact, err := decodeGlobalContextArtifactPayload(doc.Payload)
	if err != nil {
		return out, "artifact_payload_invalid", nil
	}
	if artifact.GlobalContextHash != preferredContextHash {
		return out, "global_context_hash_mismatch", nil
	}
	storedHash := textOrEmpty(latest.Hash)
	computedHash, err := HashArtifactPayload(doc)
	if err != nil {
		return out, "artifact_hash_compute_failed", nil
	}
	missReason, _ := validateStoredGlobalContextArtifactResumeHint(
		artifact,
		input,
		storedHash,
		computedHash,
	)
	if missReason != "" {
		return out, missReason, nil
	}
	ctx := artifact.GlobalContext
	return loadedGlobalContext{
		Context:      &ctx,
		ContextHash:  artifact.GlobalContextHash,
		ArtifactHash: computedHash,
	}, "", nil
}

func populateGlobalContextDiagnosticsFromInput(diag *PipelineDiagnostics, input GlobalContextGenerationInput, inputHash string) {
	if diag == nil {
		return
	}
	diag.GlobalContext.InputSchema = input.SchemaVersion
	diag.GlobalContext.InputHash = inputHash
	diag.GlobalContext.SignalCount = len(input.ValidatedSignals)
	diag.GlobalContext.FactCount = len(input.FactCatalog)
	diag.GlobalContext.EntityCount = len(input.EntityCatalog)
	diag.GlobalContext.FailedChunkCount = len(input.FailedChunkIDs)
	diag.GlobalContext.LimitationsCount = len(input.DataQualityLimitations)
}

func populateGlobalContextDiagnosticsFromLoad(diag *PipelineDiagnostics, loaded loadedGlobalContext) {
	if diag == nil {
		return
	}
	diag.GlobalContext.Source = "artifact"
	diag.GlobalContext.ArtifactSource = "artifact"
	diag.GlobalContext.AICalls = 0
	diag.GlobalContext.RetryCalls = 0
	diag.GlobalContext.InputTokens = 0
	diag.GlobalContext.OutputTokens = 0
	diag.GlobalContext.ArtifactHash = loaded.ArtifactHash
	diag.GlobalContext.ArtifactMissReason = ""
}

func populateGlobalContextDiagnosticsFromGeneration(
	diag *PipelineDiagnostics,
	raw *GlobalContextRawResponse,
	validation GlobalContextValidationResult,
) {
	if diag == nil {
		return
	}
	diag.GlobalContext.Source = "generated"
	diag.GlobalContext.AICalls = 1
	diag.GlobalContext.RetryCalls = 0
	diag.GlobalContext.IssuesCount = len(validation.Issues)
	if raw != nil {
		diag.GlobalContext.InputTokens = raw.InputTokens
		diag.GlobalContext.OutputTokens = raw.OutputTokens
		diag.GlobalContext.LatencyMs = raw.LatencyMs
		if raw.Model != "" {
			diag.GlobalContext.Model = raw.Model
		}
	}
}
