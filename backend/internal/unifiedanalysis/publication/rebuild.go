package publication

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardfacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const deterministicDashboardRebuildTrigger = "deterministic_dashboard_rebuild"

// RebuildDashboardPublicationRequest rebuilds only the deterministic dashboard
// layer from source data while reusing an immutable validated AI artifact.
// It never starts the unified analysis service or calls a provider.
type RebuildDashboardPublicationRequest struct {
	SellerAccountID int64
	MetricDate      time.Time
	Actor           string
}

type RebuildDashboardPublicationResult struct {
	SellerAccountID       int64
	MetricDate            string
	PreviousPublicationID int64
	PublicationID         int64
	SourceRunID           int64
	DeterministicRunID    int64
	ProviderCallsDelta    int
	ReusedExisting        bool
}

// RebuildDashboardPublication creates a new publication from the current
// seller publication's immutable AI output and freshly rebuilt Dashboard Facts.
// The old publication is never edited or deleted; the normal PublishRun path
// creates the new frozen snapshot and atomically advances the current pointer.
func (s *Service) RebuildDashboardPublication(ctx context.Context, req RebuildDashboardPublicationRequest) (*RebuildDashboardPublicationResult, error) {
	if s == nil || s.pool == nil || s.queries == nil || s.artifacts == nil {
		return nil, NewError(CodeSettingsInvalid, "publication service is not configured")
	}
	if req.SellerAccountID <= 0 {
		return nil, NewError(CodeSettingsInvalid, "seller_account_id is required")
	}
	metricDate := analysisdata.DateOnlyUTC(req.MetricDate)
	if metricDate.IsZero() {
		return nil, NewError(CodeSettingsInvalid, "metric_date is required")
	}

	current, err := s.queries.GetSellerCurrentUnifiedPublication(ctx, req.SellerAccountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, NewError(CodeRunNotFound, "seller has no current unified publication")
		}
		return nil, WrapError(CodeRunNotFound, "load current unified publication", err)
	}
	previousPublication, err := s.queries.GetUnifiedAnalysisPublicationByID(ctx, current.PublicationID)
	if err != nil {
		return nil, WrapError(CodeRunNotFound, "load current publication", err)
	}
	if !previousPublication.MetricDate.Valid || !analysisdata.DateOnlyUTC(previousPublication.MetricDate.Time).Equal(metricDate) {
		return nil, Errorf(CodeSettingsInvalid, "current publication metric_date=%s does not match requested metric_date=%s", previousPublication.MetricDate.Time.Format("2006-01-02"), metricDate.Format("2006-01-02"))
	}
	sourceRun, err := s.queries.GetUnifiedAnalysisRunByID(ctx, previousPublication.RunID)
	if err != nil {
		return nil, WrapError(CodeRunNotFound, "load source unified analysis run", err)
	}

	// Re-running the command after a successful rebuild is a safe no-op. This
	// prevents an operator retry from creating an unbounded chain of equivalent
	// current publications.
	if facts, ok := s.artifacts.LoadDashboardFacts(ctx, sourceRun.ID); ok && hasCanonicalFunnelContract(facts) {
		return &RebuildDashboardPublicationResult{
			SellerAccountID:       req.SellerAccountID,
			MetricDate:            metricDate.Format("2006-01-02"),
			PreviousPublicationID: current.PublicationID,
			PublicationID:         current.PublicationID,
			SourceRunID:           sourceRun.ID,
			DeterministicRunID:    sourceRun.ID,
			ProviderCallsDelta:    0,
			ReusedExisting:        true,
		}, nil
	}

	validated, _, ok := s.artifacts.LoadValidatedOutputJSON(ctx, sourceRun.ID)
	if !ok || len(validated) == 0 {
		return nil, NewError(CodeArtifactMissing, "source validated AI output artifact not found")
	}

	snapshot, err := analysisdata.NewSQLLoader(s.pool).Load(ctx, req.SellerAccountID, metricDate)
	if err != nil {
		return nil, WrapError(CodeProjectionFailed, "load source snapshot for deterministic dashboard rebuild", err)
	}
	packID := strings.TrimSpace(sourceRun.PackID.String)
	packHash := strings.TrimSpace(sourceRun.PackHash.String)
	facts, report, err := dashboardfacts.BuildDashboardFacts(snapshot, dashboardfacts.PackIdentity{PackID: packID, PackHash: packHash})
	if err != nil {
		return nil, WrapError(CodeProjectionFailed, "build dashboard facts", err)
	}
	if !report.Valid {
		return nil, Errorf(CodeProjectionFailed, "dashboard facts validation failed")
	}

	actor := strings.TrimSpace(req.Actor)
	if actor == "" {
		actor = "deterministic-dashboard-rebuild"
	}
	startedAt := time.Now().UTC()
	newRun, err := s.queries.CreateUnifiedAnalysisRun(ctx, dbgen.CreateUnifiedAnalysisRunParams{
		SellerAccountID:       req.SellerAccountID,
		MetricDate:            pgtype.Date{Time: metricDate, Valid: true},
		Mode:                  unifiedanalysis.ModeUnifiedAnalysis,
		Trigger:               deterministicDashboardRebuildTrigger,
		Status:                unifiedanalysis.StatusPublicationReady,
		DataPackSchemaVersion: sourceRun.DataPackSchemaVersion,
		DataPackHash:          sourceRun.DataPackHash,
		PipelineVersion:       pgtype.Text{String: "deterministic-dashboard-rebuild/v1", Valid: true},
		PromptVersion:         sourceRun.PromptVersion,
		Provider:              pgtype.Text{String: "none", Valid: true},
		ProviderModel:         pgtype.Text{String: "none", Valid: true},
		ProviderAttemptCount:  0,
		StartedAt:             pgtype.Timestamptz{Time: startedAt, Valid: true},
	})
	if err != nil {
		return nil, WrapError(CodePersistLinkFailed, "create deterministic dashboard rebuild run", err)
	}
	if _, err := s.pool.Exec(ctx, `
UPDATE ai_pipeline_runs
SET pack_id=$2, pack_hash=$3, provider_status='skipped_deterministic_rebuild',
    cleanup_status='completed', code_interpreter_calls=0, completed_at=now(),
    published_at=NULL
WHERE id=$1`, newRun.ID, packID, packHash); err != nil {
		return nil, WrapError(CodePersistLinkFailed, "stamp deterministic dashboard rebuild run", err)
	}

	// Reuse every immutable source artifact (including validated output and its
	// validation identity), except old Dashboard Facts, which are rebuilt from
	// the current source rows below. No provider artifact is executed or changed.
	artifacts, err := s.artifacts.ListArtifactsWithError(ctx, sourceRun.ID)
	if err != nil {
		return nil, WrapError(CodeArtifactMissing, "list source immutable AI artifacts", err)
	}
	for _, artifact := range artifacts {
		if artifact.Kind == unifiedanalysis.ArtifactUnifiedDashboardFacts {
			continue
		}
		if _, err := s.queries.CreateAIPipelineArtifact(ctx, dbgen.CreateAIPipelineArtifactParams{
			PipelineRunID: newRun.ID,
			Kind:          artifact.Kind,
			Hash:          artifact.Hash,
			Storage:       artifact.Storage,
			ObjectKey:     artifact.ObjectKey,
			PayloadJson:   artifact.PayloadJson,
		}); err != nil {
			return nil, WrapError(CodePersistLinkFailed, "copy immutable AI artifact", err)
		}
	}
	if err := s.artifacts.SaveDashboardFacts(ctx, newRun.ID, facts); err != nil {
		return nil, WrapError(CodePersistDashboardFailed, "save rebuilt dashboard facts", err)
	}

	visible := VisibilityVisible
	published, err := s.PublishRun(ctx, PublishRequest{
		RunID:                 newRun.ID,
		Actor:                 actor,
		AllowNonCurrentPrompt: true,
		ForceVisibility:       &visible,
		PublishAsCurrent:      boolPtr(true),
		AllowOlderMetricDate:  true,
	})
	if err != nil {
		return nil, err
	}
	if published == nil || published.PublicationID == previousPublication.ID {
		return nil, Errorf(CodePersistLinkFailed, "deterministic rebuild did not create a new publication")
	}
	return &RebuildDashboardPublicationResult{
		SellerAccountID:       req.SellerAccountID,
		MetricDate:            metricDate.Format("2006-01-02"),
		PreviousPublicationID: previousPublication.ID,
		PublicationID:         published.PublicationID,
		SourceRunID:           sourceRun.ID,
		DeterministicRunID:    newRun.ID,
		ProviderCallsDelta:    0,
	}, nil
}

func boolPtr(value bool) *bool { return &value }

func hasCanonicalFunnelContract(facts dashboardfacts.DashboardFactsV1) bool {
	return facts.Funnel.CurrentCompletedDate != "" &&
		facts.Funnel.PreviousCompletedDate != "" &&
		facts.Funnel.DailySeries != nil &&
		facts.Funnel.Overview7D.Status != "" &&
		hasCanonicalAdvertisingContract(facts)
}

func hasCanonicalAdvertisingContract(facts dashboardfacts.DashboardFactsV1) bool {
	period := facts.AdEfficiency.Period
	if period == nil || period.CurrentCompletedDate == "" || period.PreviousCompletedDate == "" || period.Current7dDateFrom == "" || period.Current7dDateTo == "" || period.Previous7dDateFrom == "" || period.Previous7dDateTo == "" {
		return false
	}
	if facts.AdEfficiency.Summary == nil || facts.AdEfficiency.Summary.PreviousDRR7dPct == nil || facts.AdEfficiency.Summary.DeltaVsPrevious7dPp == nil || facts.AdEfficiency.Trend7D == nil {
		return false
	}
	for _, point := range facts.AdEfficiency.Trend7D {
		if point.Date == period.MetricDate || point.Date > period.CurrentCompletedDate {
			return false
		}
	}
	return true
}
