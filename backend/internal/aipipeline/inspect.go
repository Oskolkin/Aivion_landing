package aipipeline

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// PipelineInspectStore reads pipeline run summaries for devtools.
type PipelineInspectStore interface {
	GetPipelineRun(ctx context.Context, id int64) (dbgen.AiPipelineRun, error)
	ListPipelineArtifacts(ctx context.Context, pipelineRunID int64) ([]dbgen.AiPipelineArtifact, error)
}

// PipelineInspectSummary is compact inspect output without full payloads.
type PipelineInspectSummary struct {
	PipelineRun PipelineRunInspect `json:"pipeline_run"`
	Artifacts   []ArtifactInspect  `json:"artifacts"`
}

type PipelineRunInspect struct {
	ID                int64  `json:"id"`
	Status            string `json:"status"`
	DataPackHash      string `json:"data_pack_hash,omitempty"`
	GlobalContextHash string `json:"global_context_hash,omitempty"`
	DashboardRunID    string `json:"dashboard_run_id,omitempty"`
	ActionsRunID      int64  `json:"actions_run_id,omitempty"`
}

type ArtifactInspect struct {
	ID         int64  `json:"id"`
	Kind       string `json:"kind"`
	Hash       string `json:"hash,omitempty"`
	Provider   string `json:"provider,omitempty"`
	HasMeta    bool   `json:"has_meta"`
	HasPayload bool   `json:"has_payload"`
}

// BuildPipelineInspectSummary builds compact inspect output.
func BuildPipelineInspectSummary(
	run dbgen.AiPipelineRun,
	artifacts []dbgen.AiPipelineArtifact,
) PipelineInspectSummary {
	out := PipelineInspectSummary{
		PipelineRun: PipelineRunInspect{
			ID:                run.ID,
			Status:            run.Status,
			DataPackHash:      run.DataPackHash,
			GlobalContextHash: pgText(run.GlobalContextHash),
		},
		Artifacts: make([]ArtifactInspect, 0, len(artifacts)),
	}
	if run.DashboardRunID.Valid {
		if u, err := uuid.FromBytes(run.DashboardRunID.Bytes[:]); err == nil {
			out.PipelineRun.DashboardRunID = u.String()
		}
	}
	if run.ActionsRunID.Valid {
		out.PipelineRun.ActionsRunID = run.ActionsRunID.Int64
	}
	for _, art := range artifacts {
		provider, hasMeta, hasPayload := artifactInspectFields(art.PayloadJson)
		out.Artifacts = append(out.Artifacts, ArtifactInspect{
			ID:         art.ID,
			Kind:       art.Kind,
			Hash:       pgText(art.Hash),
			Provider:   provider,
			HasMeta:    hasMeta,
			HasPayload: hasPayload,
		})
	}
	return out
}

func artifactInspectFields(raw []byte) (provider string, hasMeta, hasPayload bool) {
	if len(raw) == 0 {
		return "", false, false
	}
	var doc ArtifactDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		return "", false, false
	}
	hasMeta = doc.Meta != nil
	hasPayload = doc.Payload != nil
	if doc.Meta != nil {
		if p, ok := doc.Meta["provider"].(string); ok {
			provider = p
		}
	}
	return provider, hasMeta, hasPayload
}

func pgText(v pgtype.Text) string {
	if !v.Valid {
		return ""
	}
	return v.String
}

// InspectPipelineRun loads and summarizes a pipeline run.
func InspectPipelineRun(ctx context.Context, store PipelineInspectStore, pipelineRunID int64) (PipelineInspectSummary, error) {
	if store == nil {
		return PipelineInspectSummary{}, fmt.Errorf("inspect store is not configured")
	}
	run, err := store.GetPipelineRun(ctx, pipelineRunID)
	if err != nil {
		return PipelineInspectSummary{}, err
	}
	artifacts, err := store.ListPipelineArtifacts(ctx, pipelineRunID)
	if err != nil {
		return PipelineInspectSummary{}, err
	}
	return BuildPipelineInspectSummary(run, artifacts), nil
}
