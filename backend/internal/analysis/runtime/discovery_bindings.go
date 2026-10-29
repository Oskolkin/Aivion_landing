package runtime

import (
	"context"
	"errors"
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/artifacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/deep"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/discovery"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/stages"
)

var ErrArtifactBindingMismatch = errors.New("runtime: ARTIFACT_BINDING_MISMATCH")

func (e *Executor) loadAcceptedDiscoveryBindings(ctx context.Context, runID int64, packOut string) (discovery.AuditEnvelope, string, string, error) {
	if e.Repo == nil || e.Store == nil {
		return discovery.AuditEnvelope{}, "", "", fmt.Errorf("runtime: repository and artifact store required")
	}
	stage, err := e.Repo.GetStage(ctx, runID, string(stages.BuildDiscoveryPack))
	if err != nil {
		return discovery.AuditEnvelope{}, "", "", err
	}
	if stage.AcceptedAttemptID == nil || *stage.AcceptedAttemptID <= 0 {
		return discovery.AuditEnvelope{}, "", "", fmt.Errorf("runtime: BUILD_DISCOVERY_PACK accepted attempt missing")
	}
	key, err := e.Repo.ArtifactStorageKey(ctx, runID, packOut)
	if err != nil {
		return discovery.AuditEnvelope{}, "", "", err
	}
	body, err := e.Store.Get(ctx, key)
	if err != nil {
		return discovery.AuditEnvelope{}, "", "", err
	}
	envelope, _, err := discovery.UnmarshalPackWire(body)
	if err != nil {
		return discovery.AuditEnvelope{}, "", "", err
	}
	_, _, mapSHA, err := e.Repo.ArtifactByStageAttemptAndKind(ctx, runID, *stage.AcceptedAttemptID, string(artifacts.DiscoveryFactHandleMap))
	if err != nil {
		return discovery.AuditEnvelope{}, "", "", err
	}
	if mapSHA != envelope.FactHandleMapHash {
		return discovery.AuditEnvelope{}, "", "", fmt.Errorf("%w: fact handle map", ErrArtifactBindingMismatch)
	}
	_, _, manSHA, err := e.Repo.ArtifactByStageAttemptAndKind(ctx, runID, *stage.AcceptedAttemptID, string(artifacts.DiscoveryRequestManifest))
	if err != nil {
		return discovery.AuditEnvelope{}, "", "", err
	}
	return envelope, mapSHA, manSHA, nil
}

func (e *Executor) loadAcceptedEvidenceRegistrySHA(ctx context.Context, runID int64) (string, error) {
	if e.Repo == nil {
		return "", fmt.Errorf("runtime: repository required")
	}
	stage, err := e.Repo.GetStage(ctx, runID, string(stages.ExpandEvidence))
	if err != nil {
		return "", err
	}
	if stage.AcceptedAttemptID == nil || *stage.AcceptedAttemptID <= 0 {
		return "", fmt.Errorf("runtime: EXPAND_EVIDENCE accepted attempt missing")
	}
	_, _, sha, err := e.Repo.ArtifactByStageAttemptAndKind(ctx, runID, *stage.AcceptedAttemptID, string(artifacts.AnalysisFactHandleRegistry))
	return sha, err
}

func (e *Executor) loadAcceptedDeepBindings(ctx context.Context, runID int64, packOut string) (deep.AuditEnvelope, string, string, error) {
	if e.Repo == nil || e.Store == nil {
		return deep.AuditEnvelope{}, "", "", fmt.Errorf("runtime: repository and artifact store required")
	}
	stage, err := e.Repo.GetStage(ctx, runID, string(stages.BuildDeepPack))
	if err != nil {
		return deep.AuditEnvelope{}, "", "", err
	}
	if stage.AcceptedAttemptID == nil || *stage.AcceptedAttemptID <= 0 {
		return deep.AuditEnvelope{}, "", "", fmt.Errorf("runtime: BUILD_DEEP_PACK accepted attempt missing")
	}
	key, err := e.Repo.ArtifactStorageKey(ctx, runID, packOut)
	if err != nil {
		return deep.AuditEnvelope{}, "", "", err
	}
	body, err := e.Store.Get(ctx, key)
	if err != nil {
		return deep.AuditEnvelope{}, "", "", err
	}
	envelope, _, err := deep.UnmarshalPackWire(body)
	if err != nil {
		return deep.AuditEnvelope{}, "", "", err
	}
	_, _, menuSHA, err := e.Repo.ArtifactByStageAttemptAndKind(ctx, runID, *stage.AcceptedAttemptID, string(artifacts.DeepActionMenu))
	if err != nil {
		return deep.AuditEnvelope{}, "", "", err
	}
	if menuSHA != envelope.ActionMenuHash {
		return deep.AuditEnvelope{}, "", "", fmt.Errorf("%w: action menu", ErrArtifactBindingMismatch)
	}
	_, _, polSHA, err := e.Repo.ArtifactByStageAttemptAndKind(ctx, runID, *stage.AcceptedAttemptID, string(artifacts.DeepRelationshipPolicy))
	if err != nil {
		return deep.AuditEnvelope{}, "", "", err
	}
	return envelope, menuSHA, polSHA, nil
}
