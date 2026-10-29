package publication

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardfacts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome/weeklychangepolicy"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/outputextract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/publication/adapter"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/publication/bundlevalidator"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// unifiedNarrativeGenerator/unifiedNarrativeModel are stamped on
// ai_dashboard_narratives.prompt_version/model for rows produced by Stage 5
// publication (as opposed to the legacy OpenAI-backed narrative generator),
// so debugging/analytics can tell the two paths apart.
const (
	unifiedNarrativeGenerator = adapter.Version
	unifiedNarrativeModel     = "unified_analysis_pipeline"
)

// Service publishes validated unified analysis outputs (Stage 4 artifacts)
// into the seller-facing dashboard/actions surfaces (Stage 5). It never talks
// to the AI provider: by the time PublishRun is called, the run's output has
// already been generated and validated by the unified analysis orchestrator.
type Service struct {
	pool      *pgxpool.Pool
	queries   *dbgen.Queries
	artifacts *unifiedanalysis.ArtifactStore
	policy    PublicationPolicy
	global    GlobalFlags
	now       func() time.Time

	// testFailAt is only ever set via WithTestFailurePoint (hooks.go); it is
	// always empty in production Services built by NewService, making
	// maybeFail a no-op.
	testFailAt FailurePoint
}

// NewService builds a publication Service. global is typically
// GlobalFlagsFromEnv(); tests may pass a fixed GlobalFlags value.
func NewService(pool *pgxpool.Pool, queries *dbgen.Queries, artifacts *unifiedanalysis.ArtifactStore, policy PublicationPolicy, global GlobalFlags) *Service {
	return &Service{
		pool:      pool,
		queries:   queries,
		artifacts: artifacts,
		policy:    policy,
		global:    global,
		now:       func() time.Time { return time.Now().UTC() },
	}
}

// PublishRun projects a publication_ready unified analysis run into the
// seller-facing dashboard/actions tables and, for visible publications,
// updates the seller's current unified publication pointer. See package doc
// in errors.go/status.go for the involved tables and lifecycle.
func (s *Service) PublishRun(ctx context.Context, req PublishRequest) (*PublishResult, error) {
	if s == nil || s.pool == nil || s.queries == nil || s.artifacts == nil {
		return nil, NewError(CodeSettingsInvalid, "publication service is not configured")
	}
	if req.RunID <= 0 {
		return nil, NewError(CodeRunNotFound, "run_id is required")
	}

	run, err := s.queries.GetUnifiedAnalysisRunByID(ctx, req.RunID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, WrapError(CodeRunNotFound, "unified analysis run not found", ErrRunNotFound)
		}
		return nil, WrapError(CodeRunNotFound, "load unified analysis run", err)
	}

	existing, existErr := s.queries.GetUnifiedAnalysisPublicationByRunID(ctx, req.RunID)
	hasExisting := existErr == nil
	if existErr != nil && !errors.Is(existErr, pgx.ErrNoRows) {
		return nil, WrapError(CodeRunNotFound, "load existing publication", existErr)
	}
	if hasExisting {
		switch Status(existing.Status) {
		case StatusPublished:
			// Shadow → visible promotion is a deliberate second step (PromoteShadow
			// or ForceVisibility=visible on a republish). Without this, a visible
			// republish of an already-shadow publication would silently no-op and
			// leave the seller on their previous current pointer.
			if req.ForceVisibility != nil && *req.ForceVisibility == VisibilityVisible &&
				Visibility(existing.Visibility) == VisibilityShadow {
				return s.PromoteShadowPublication(ctx, PromoteRequest{
					PublicationID: existing.ID,
					Actor:         actorOrDefault(req.Actor),
					Reason:        "publish_run_force_visibility_visible",
				})
			}
			// Publication committed but run status may still be publishing (crash
			// after MarkUnifiedAnalysisPublicationPublished / before SetAIPipelinePublishedAt
			// commit boundaries, or a restored partial state). Finalize the run so
			// recovery stops re-enqueueing Publish forever.
			if err := s.finalizeRunIfAlreadyPublished(ctx, run); err != nil {
				return nil, err
			}
			return s.resultFromRow(existing, OutcomeAlreadyPublished), nil
		case StatusRolledBack, StatusSuperseded:
			return nil, Errorf(CodeAlreadyPublished, "publication %d for run %d is %s and cannot be republished directly", existing.ID, req.RunID, existing.Status)
		case StatusPublicationFailed:
			if existing.ErrorCode.Valid && existing.ErrorCode.String == CodeAnalysisFailedOutcome {
				// Permanent: an analysis_failed outcome comes from the run's own
				// validated output, not a transient persistence error, so retrying
				// PublishRun can never make it publishable. Without this early
				// exit, recovery would keep re-invoking PublishRun for a run stuck
				// in publication_ready/publishing forever (see recordAnalysisFailed,
				// which now also moves the RUN itself to publication_failed so
				// classifyRecoveryActions stops scheduling it — this check covers
				// direct/manual PublishRun calls made before that run transition
				// takes effect, or against stale state).
				return s.resultFromRow(existing, OutcomeSkippedAnalysisFailed), nil
			}
			// Any other publication_failed reason (e.g. a transient persistence
			// error) is presumed retryable; fall through to reuse row below.
		}
		// created/projecting/ready/publishing/publication_failed(retryable) -> retry, reuse row below.
	}

	if run.Status != unifiedanalysis.StatusPublicationReady && run.Status != unifiedanalysis.StatusPublishing {
		return nil, Errorf(CodeRunNotReady, "run status %q is not publication_ready", run.Status)
	}

	var settingsPtr *dbgen.SellerAnalysisSetting
	settingsRow, settingsErr := s.queries.GetSellerAnalysisSettings(ctx, run.SellerAccountID)
	if settingsErr == nil {
		settingsPtr = &settingsRow
	} else if !errors.Is(settingsErr, pgx.ErrNoRows) {
		return nil, WrapError(CodeSettingsInvalid, "load seller analysis settings", settingsErr)
	}
	settings := EffectiveSettings(settingsPtr, s.global)

	if req.PublishAsCurrent == nil {
		if art, ok := s.artifacts.LoadDateResolution(ctx, req.RunID); ok {
			pac := art.PublishAsCurrent
			req.PublishAsCurrent = &pac
		}
	}

	visibility := settings.Visibility
	if req.PublishAsCurrent != nil && req.ForceVisibility == nil {
		if *req.PublishAsCurrent {
			vis := VisibilityVisible
			req.ForceVisibility = &vis
			req.AllowOlderMetricDate = true
		} else {
			vis := VisibilityShadow
			req.ForceVisibility = &vis
		}
	}
	if req.ForceVisibility != nil {
		visibility = *req.ForceVisibility
	} else if settings.Mode == AnalysisModeLegacy {
		return nil, NewError(CodeSettingsInvalid, "seller is in legacy analysis mode; unified publication is not allowed")
	}
	if !visibility.IsValid() {
		return nil, Errorf(CodeSettingsInvalid, "invalid visibility %q", visibility)
	}
	pubMode := PublicationModeShadow
	if visibility == VisibilityVisible {
		pubMode = PublicationModeUnified
	}

	payloadJSON, outputHash, ok := s.artifacts.LoadValidatedOutputJSON(ctx, req.RunID)
	if !ok || len(payloadJSON) == 0 {
		return nil, NewError(CodeArtifactMissing, "validated unified analysis output artifact not found")
	}
	outputHash = strings.TrimSpace(outputHash)
	if outputHash == "" {
		return nil, NewError(CodeArtifactIntegrityFailed, "validated output artifact has no content hash")
	}

	out, parseErr := outputextract.ParseOutput(payloadJSON)
	if parseErr != nil {
		return nil, WrapError(CodeOutputParseFailed, "parse validated unified analysis output", parseErr)
	}

	promptVersion := strings.TrimSpace(run.PromptVersion.String)
	_, promptHash, _, schemaHash, _ := s.artifacts.LoadOutputValidationIdentity(ctx, req.RunID)
	if !req.AllowNonCurrentPrompt {
		if pErr := s.policy.Allows(promptVersion, promptHash, out.SchemaVersion, schemaHash, false); pErr != nil {
			return nil, pErr
		}
	}

	metricDateTime := run.MetricDate.Time.UTC()
	metricDateStr := metricDateTime.Format("2006-01-02")
	packID := strings.TrimSpace(run.PackID.String)
	if packID == "" {
		packID = out.AnalysisMeta.PackID
	}
	packHash := strings.TrimSpace(run.PackHash.String)
	if packHash == "" {
		packHash = out.AnalysisMeta.PackHash
	}
	globalContextHash := "unified:" + outputHash

	identity := publicationIdentity{
		PackID:        packID,
		PackHash:      packHash,
		PromptVersion: promptVersion,
		PromptHash:    promptHash,
		SchemaVersion: out.SchemaVersion,
		SchemaHash:    schemaHash,
		OutputHash:    outputHash,
	}

	if out.AnalysisOutcome == contract.AnalysisOutcomeAnalysisFailed {
		return s.recordAnalysisFailed(ctx, run, existing, hasExisting, metricDateStr, identity, pubMode, visibility)
	}

	// Load and verify backend Dashboard Facts BEFORE projecting: facts are
	// the sole source of truth for every factual dashboard field (see
	// docs/current/dashboard-field-ownership.md), so a run with missing or
	// mismatched facts must never be projected/published, regardless of how
	// complete its AI output looks.
	facts, factsOK := s.artifacts.LoadDashboardFacts(ctx, req.RunID)
	factsIdentityOK := factsOK && factsIdentityMatches(facts, run, metricDateStr, packHash)

	var previousFacts *dashboardfacts.DashboardFactsV1
	if factsOK {
		previousFacts = s.loadPreviousDashboardFacts(ctx, run.SellerAccountID, req.RunID)
		if previousFacts == nil && os.Getenv("ALLOW_DEV_SEED") == "1" {
			cloned := weeklychangepolicy.BuildPreviousPeriodFactsClone(facts)
			previousFacts = &cloned
		}
	}

	var dashboardProjection adapter.DashboardProjection
	var mismatchWarnings []adapter.MismatchWarning
	var projectedActions []adapter.ProjectedAction
	if factsOK && factsIdentityOK {
		dashboardProjection, mismatchWarnings = adapter.ProjectDashboardWithPrevious(facts, previousFacts, out, run.SellerAccountID, metricDateStr)
		if dateArt, ok := s.artifacts.LoadDateResolution(ctx, req.RunID); ok {
			adapter.ApplyDateFreshness(&dashboardProjection, dateArt.Resolution)
		}
		projectedActions = adapter.ProjectActions(out, 0, run.SellerAccountID, packID, metricDateTime, adapter.EntityFactsLookupFromDashboardFacts(facts))
	}

	bundleReport := bundlevalidator.ValidatePublicationBundle(bundlevalidator.Input{
		FactsPresent:     factsOK,
		FactsIdentityOK:  factsIdentityOK,
		Facts:            facts,
		Projection:       dashboardProjection,
		Actions:          projectedActions,
		AnalysisOutcome:  out.AnalysisOutcome,
		MismatchWarnings: mismatchWarnings,
	})
	if !bundleReport.Valid {
		return s.recordBundleValidationFailed(ctx, run, existing, hasExisting, identity, pubMode, visibility, out.AnalysisOutcome, bundleReport)
	}

	dashboardHash, hErr := adapter.ProjectionHash(dashboardProjection)
	if hErr != nil {
		return nil, WrapError(CodeProjectionFailed, "hash dashboard projection", hErr)
	}
	actionsHash, hErr := adapter.ProjectionHash(projectedActions)
	if hErr != nil {
		return nil, WrapError(CodeProjectionFailed, "hash actions projection", hErr)
	}
	bundleHash := adapter.BundleHash(outputHash, dashboardHash, actionsHash, adapter.Hash())

	topCount := 0
	recCount := 0
	for _, pa := range projectedActions {
		if pa.IsTop {
			topCount++
		}
		if pa.IsRecommended {
			recCount++
		}
	}

	var result *PublishResult
	txErr := s.withSellerTx(ctx, run.SellerAccountID, func(ctx context.Context, tx pgx.Tx, qtx *dbgen.Queries) error {
		lockedRun, rErr := qtx.GetUnifiedAnalysisRunByIDForUpdate(ctx, req.RunID)
		if rErr != nil {
			return WrapError(CodeRunNotFound, "reload unified analysis run for update", rErr)
		}
		if lockedRun.Status != unifiedanalysis.StatusPublicationReady && lockedRun.Status != unifiedanalysis.StatusPublishing {
			if lockedRun.UnifiedPublicationID.Valid {
				if pub, pErr := qtx.GetUnifiedAnalysisPublicationByID(ctx, lockedRun.UnifiedPublicationID.Int64); pErr == nil && pub.Status == string(StatusPublished) {
					if finErr := s.finalizeRunPublishedLocked(ctx, qtx, lockedRun); finErr != nil {
						return finErr
					}
					result = s.resultFromRow(pub, OutcomeAlreadyPublished)
					return nil
				}
			}
			return Errorf(CodeRunNotReady, "run status %q is not publication_ready", lockedRun.Status)
		}

		// Stale check BEFORE transitioning to publishing so a rejected older
		// candidate does not leave the run stuck in publishing (recovery would
		// otherwise keep re-enqueuing publish forever).
		var previousCurrentPublicationID int64
		if visibility == VisibilityVisible {
			stale, prevID, staleErr := s.checkStale(ctx, qtx, run.SellerAccountID, metricDateStr, lockedRun.ID, req.AllowOlderMetricDate)
			if staleErr != nil {
				return staleErr
			}
			if stale {
				result = &PublishResult{
					Outcome:         OutcomeStale,
					RunID:           req.RunID,
					SellerAccountID: run.SellerAccountID,
					MetricDate:      metricDateStr,
					ErrorCode:       CodeStaleRelativeToCurrent,
				}
				return nil
			}
			previousCurrentPublicationID = prevID
		}

		// Mark the run as actively publishing so a crash mid-publish leaves it in a
		// state that unifiedanalysis recovery (RecoveryActionPublish) will retry, rather
		// than looking like it never started. Retries (lockedRun already publishing)
		// skip straight to re-running the idempotent projection steps below.
		if lockedRun.Status == unifiedanalysis.StatusPublicationReady {
			transitioned, tErr := qtx.TransitionUnifiedAnalysisRunStatus(ctx, dbgen.TransitionUnifiedAnalysisRunStatusParams{
				ID:             lockedRun.ID,
				ExpectedStatus: unifiedanalysis.StatusPublicationReady,
				NextStatus:     unifiedanalysis.StatusPublishing,
			})
			if tErr != nil {
				return WrapError(CodePersistLinkFailed, "transition unified analysis run to publishing", tErr)
			}
			lockedRun = transitioned
		}
		if failErr := s.maybeFail(FailAfterRunPublishing); failErr != nil {
			return failErr
		}

		pubRow, pubErr := s.getOrCreatePublicationRow(ctx, qtx, lockedRun, existing, hasExisting, identity, pubMode, visibility, out.AnalysisOutcome)
		if pubErr != nil {
			return pubErr
		}
		if failErr := s.maybeFail(FailAfterPublicationCreated); failErr != nil {
			return failErr
		}

		// Note: persistence errors below intentionally do NOT call
		// UpdateUnifiedAnalysisPublicationStatus(publication_failed) — any write
		// made before returning an error here is rolled back by withSellerTx
		// along with everything else in this transaction, so marking the
		// publication row failed here would itself be rolled back and is a
		// no-op. A row genuinely stuck in a failed/partial state is picked up
		// by unifiedanalysis recovery (RecoveryActionPublish) retrying
		// PublishRun from scratch.
		narrativeID, snapshotID, dashErr := s.persistDashboard(ctx, qtx, lockedRun, metricDateTime, dashboardProjection, packHash, globalContextHash)
		if dashErr != nil {
			return WrapError(CodePersistDashboardFailed, "persist dashboard projection", dashErr)
		}
		if failErr := s.maybeFail(FailAfterDashboardInserted); failErr != nil {
			return failErr
		}

		actionsRunID, actErr := s.persistActions(ctx, tx, qtx, lockedRun, metricDateTime, out, projectedActions, pubRow.ID, packHash, globalContextHash)
		if actErr != nil {
			return WrapError(CodePersistActionsFailed, "persist actions projection", actErr)
		}
		if failErr := s.maybeFail(FailAfterActionMapInserted); failErr != nil {
			return failErr
		}

		if _, linkErr := qtx.LinkAIPipelineRunDashboardDataSnapshot(ctx, dbgen.LinkAIPipelineRunDashboardDataSnapshotParams{
			ID:                      lockedRun.ID,
			DashboardDataSnapshotID: pgtype.Int8{Int64: snapshotID, Valid: true},
		}); linkErr != nil {
			return WrapError(CodePersistLinkFailed, "link dashboard data snapshot to pipeline run", linkErr)
		}
		if _, updErr := qtx.UpdateAIPipelineRun(ctx, dbgen.UpdateAIPipelineRunParams{
			ID:                lockedRun.ID,
			DashboardRunID:    narrativeID,
			ActionsRunID:      pgtype.Int8{Int64: actionsRunID, Valid: true},
			DataPackHash:      nullableText(packHash),
			GlobalContextHash: nullableText(globalContextHash),
		}); updErr != nil {
			return WrapError(CodePersistLinkFailed, "link dashboard/actions runs to pipeline run", updErr)
		}

		published, markErr := qtx.MarkUnifiedAnalysisPublicationPublished(ctx, dbgen.MarkUnifiedAnalysisPublicationPublishedParams{
			ID:                      pubRow.ID,
			Visibility:              string(visibility),
			DashboardRunID:          narrativeID,
			ActionsRunID:            pgtype.Int8{Int64: actionsRunID, Valid: true},
			DashboardDataSnapshotID: pgtype.Int8{Int64: snapshotID, Valid: true},
			DashboardRecordCount:    1,
			ActionsRecordCount:      int32(len(projectedActions)),
			TopActionsCount:         int32(topCount),
			RecommendedActionsCount: int32(recCount),
			DashboardProjectionHash: dashboardHash,
			ActionsProjectionHash:   actionsHash,
			BundleHash:              bundleHash,
		})
		if markErr != nil {
			return WrapError(CodePersistLinkFailed, "mark publication published", markErr)
		}

		if _, setErr := qtx.SetAIPipelineUnifiedPublicationID(ctx, dbgen.SetAIPipelineUnifiedPublicationIDParams{
			ID:                   lockedRun.ID,
			UnifiedPublicationID: pgtype.Int8{Int64: published.ID, Valid: true},
		}); setErr != nil {
			return WrapError(CodePersistLinkFailed, "set unified_publication_id on pipeline run", setErr)
		}

		// Move the unified_analysis run itself from publishing to published (status
		// "completed" is a legacy-pipeline concept; the unified pipeline's own terminal
		// state is "published"), and stamp published_at/completed_at. Do this for both
		// shadow and visible publications: it is the run's own lifecycle marker, not a
		// visibility decision. NOT SQLCSnapshotPublisher.PublishPipelineSnapshot: that
		// helper is legacy-only (separate tx, sets status="completed") and would
		// conflict with unified_analysis run-status semantics.
		now := s.now()
		if _, publishedAtErr := qtx.SetAIPipelinePublishedAt(ctx, dbgen.SetAIPipelinePublishedAtParams{
			ID:          lockedRun.ID,
			PublishedAt: pgtype.Timestamptz{Time: now, Valid: true},
			Status:      pgtype.Text{String: unifiedanalysis.StatusPublished, Valid: true},
			CompletedAt: pgtype.Timestamptz{Time: now, Valid: true},
		}); publishedAtErr != nil {
			return WrapError(CodePersistLinkFailed, "mark unified analysis run published", publishedAtErr)
		}

		if visibility == VisibilityVisible {
			// Visible publications additionally become the seller's current pointer on
			// BOTH the unified table (seller_current_unified_publications) and the
			// legacy table (seller_current_ai_snapshots) so existing Dashboard/Actions
			// APIs — which still read seller_current_ai_snapshots — see the unified
			// bundle atomically, in the same transaction as everything above.
			if curSnapErr := qtx.UpsertSellerCurrentAISnapshot(ctx, dbgen.UpsertSellerCurrentAISnapshotParams{
				SellerAccountID: run.SellerAccountID,
				PipelineRunID:   lockedRun.ID,
			}); curSnapErr != nil {
				return WrapError(CodePersistLinkFailed, "upsert seller current ai snapshot pointer", curSnapErr)
			}
			// Compat pointer is written BEFORE the unified pointer (see
			// hooks.go FailAfterCompatPointer/FailAfterUnifiedPointer doc):
			// ReconcileCurrentPublicationPointers exists precisely to repair a
			// compat pointer left stale if a crash lands between these two
			// writes outside of a test (both are still inside this same
			// transaction here, so a real mid-write crash rolls back cleanly —
			// the reconcile tool is for defense-in-depth / drift from other
			// causes, e.g. a manual DB fixup).
			if failErr := s.maybeFail(FailAfterCompatPointer); failErr != nil {
				return failErr
			}
			if _, curErr := qtx.UpsertSellerCurrentUnifiedPublication(ctx, dbgen.UpsertSellerCurrentUnifiedPublicationParams{
				SellerAccountID: run.SellerAccountID,
				PublicationID:   published.ID,
				MetricDate:      toDate(metricDateTime),
			}); curErr != nil {
				return WrapError(CodePersistLinkFailed, "upsert seller current unified publication", curErr)
			}
			if failErr := s.maybeFail(FailAfterUnifiedPointer); failErr != nil {
				return failErr
			}
			if previousCurrentPublicationID > 0 && previousCurrentPublicationID != published.ID {
				if _, supErr := qtx.MarkUnifiedAnalysisPublicationSuperseded(ctx, dbgen.MarkUnifiedAnalysisPublicationSupersededParams{
					ID:                        previousCurrentPublicationID,
					SupersededByPublicationID: pgtype.Int8{Int64: published.ID, Valid: true},
				}); supErr != nil {
					return WrapError(CodePersistLinkFailed, "mark previous publication superseded", supErr)
				}
			}
		}

		if failErr := s.maybeFail(FailBeforeAudit); failErr != nil {
			return failErr
		}
		var auditPayload any
		if len(mismatchWarnings) > 0 {
			auditPayload = mismatchWarnings
		}
		s.audit(ctx, qtx, run.SellerAccountID, "publish", req.Actor, req.RunID, published.ID, previousCurrentPublicationID, published.ID,
			fmt.Sprintf("visibility=%s outcome=%s", visibility, out.AnalysisOutcome), auditPayload)

		result = s.resultFromRow(published, OutcomePublished)
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}
	return result, nil
}

type publicationIdentity struct {
	PackID        string
	PackHash      string
	PromptVersion string
	PromptHash    string
	SchemaVersion string
	SchemaHash    string
	OutputHash    string
}

// recordAnalysisFailed persists (or updates) a publication_failed row for a
// run whose validated output outcome is analysis_failed. Nothing is
// projected into dashboard/actions and the seller's current pointer is left
// untouched.
//
// It also transitions the unified_analysis run itself (ai_pipeline_runs.status)
// from publication_ready/publishing to publication_failed in the same
// transaction. Without this, the run would stay in publication_ready/publishing
// forever: classifyRecoveryActions (unifiedanalysis/recovery.go) keeps
// emitting RecoveryActionPublish for both of those statuses, so
// unifiedanalysis recovery would re-enqueue Stage 5 publish for this run on
// every worker restart/recovery sweep indefinitely, re-deriving the same
// analysis_failed outcome each time. publication_failed is not in
// classifyRecoveryActions' switch (nor in ListRecoverableUnifiedAnalysisRuns'
// SQL filter), so once the run reaches it, recovery stops scheduling publish
// for it — see also the StatusPublicationFailed+CodeAnalysisFailedOutcome
// early exit in PublishRun's hasExisting switch, which covers direct/manual
// PublishRun calls made against a run that hasn't made this transition yet.
func (s *Service) recordAnalysisFailed(
	ctx context.Context,
	run dbgen.AiPipelineRun,
	existing dbgen.UnifiedAnalysisPublication,
	hasExisting bool,
	metricDateStr string,
	identity publicationIdentity,
	pubMode PublicationMode,
	visibility Visibility,
) (*PublishResult, error) {
	var result *PublishResult
	txErr := s.withSellerTx(ctx, run.SellerAccountID, func(ctx context.Context, tx pgx.Tx, qtx *dbgen.Queries) error {
		pubRow, err := s.getOrCreatePublicationRow(ctx, qtx, run, existing, hasExisting, identity, pubMode, visibility, contract.AnalysisOutcomeAnalysisFailed)
		if err != nil {
			return err
		}
		updated, err := qtx.UpdateUnifiedAnalysisPublicationStatus(ctx, dbgen.UpdateUnifiedAnalysisPublicationStatusParams{
			ID:           pubRow.ID,
			Status:       string(StatusPublicationFailed),
			ErrorCode:    pgtype.Text{String: CodeAnalysisFailedOutcome, Valid: true},
			ErrorMessage: pgtype.Text{String: "unified analysis outcome is analysis_failed; nothing to publish", Valid: true},
		})
		if err != nil {
			return WrapError(CodePersistLinkFailed, "mark publication analysis_failed", err)
		}

		lockedRun, lockErr := qtx.GetUnifiedAnalysisRunByIDForUpdate(ctx, run.ID)
		if lockErr != nil {
			return WrapError(CodePersistLinkFailed, "reload unified analysis run for update", lockErr)
		}
		if lockedRun.Status == unifiedanalysis.StatusPublicationReady || lockedRun.Status == unifiedanalysis.StatusPublishing {
			if _, tErr := qtx.TransitionUnifiedAnalysisRunStatus(ctx, dbgen.TransitionUnifiedAnalysisRunStatusParams{
				ID:             lockedRun.ID,
				ExpectedStatus: lockedRun.Status,
				NextStatus:     unifiedanalysis.StatusPublicationFailed,
			}); tErr != nil {
				return WrapError(CodePersistLinkFailed, "transition unified analysis run to publication_failed", tErr)
			}
		}

		s.audit(ctx, qtx, run.SellerAccountID, "publish_skip_analysis_failed", "", run.ID, updated.ID, 0, 0, "analysis_outcome=analysis_failed", nil)
		result = s.resultFromRow(updated, OutcomeSkippedAnalysisFailed)
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}
	return result, nil
}

// factsIdentityMatches reports whether loaded Dashboard Facts provably
// belong to the run being published: same seller, same metric_date, and
// (when both sides have one) the same pack_hash. Facts must never be
// projected/published against a run they were not built for.
func factsIdentityMatches(facts dashboardfacts.DashboardFactsV1, run dbgen.AiPipelineRun, metricDateStr, packHash string) bool {
	if facts.SellerAccountID != run.SellerAccountID {
		return false
	}
	if facts.MetricDate != metricDateStr {
		return false
	}
	if packHash != "" && facts.PackHash != "" && facts.PackHash != packHash {
		return false
	}
	return true
}

// formatBundleValidationErrors renders a bundlevalidator.Report's issues
// into a single human-readable error message for
// unified_analysis_publications.error_message.
func formatBundleValidationErrors(report bundlevalidator.Report) string {
	if len(report.Errors) == 0 {
		return ""
	}
	parts := make([]string, 0, len(report.Errors))
	for _, issue := range report.Errors {
		parts = append(parts, fmt.Sprintf("%s: %s (field=%s)", issue.Code, issue.Message, issue.Field))
	}
	return strings.Join(parts, "; ")
}

// recordBundleValidationFailed persists (or updates) a publication_failed
// row for a run whose merged facts+AI bundle failed
// bundlevalidator.ValidatePublicationBundle (missing facts, facts/run
// identity mismatch, or an incomplete/inconsistent projection). Nothing is
// projected into dashboard/actions and the seller's current pointer is left
// untouched — mirrors recordAnalysisFailed's shape/rationale, including the
// run-status transition to publication_failed so recovery stops
// re-enqueuing Stage 5 publish for this run indefinitely.
func (s *Service) recordBundleValidationFailed(
	ctx context.Context,
	run dbgen.AiPipelineRun,
	existing dbgen.UnifiedAnalysisPublication,
	hasExisting bool,
	identity publicationIdentity,
	pubMode PublicationMode,
	visibility Visibility,
	analysisOutcome string,
	report bundlevalidator.Report,
) (*PublishResult, error) {
	errorCode := report.FirstErrorCode()
	if errorCode == "" {
		errorCode = CodeBundleValidationFailed
	}
	errorMessage := formatBundleValidationErrors(report)

	var result *PublishResult
	txErr := s.withSellerTx(ctx, run.SellerAccountID, func(ctx context.Context, tx pgx.Tx, qtx *dbgen.Queries) error {
		pubRow, err := s.getOrCreatePublicationRow(ctx, qtx, run, existing, hasExisting, identity, pubMode, visibility, analysisOutcome)
		if err != nil {
			return err
		}
		updated, err := qtx.UpdateUnifiedAnalysisPublicationStatus(ctx, dbgen.UpdateUnifiedAnalysisPublicationStatusParams{
			ID:           pubRow.ID,
			Status:       string(StatusPublicationFailed),
			ErrorCode:    pgtype.Text{String: errorCode, Valid: errorCode != ""},
			ErrorMessage: pgtype.Text{String: errorMessage, Valid: errorMessage != ""},
		})
		if err != nil {
			return WrapError(CodePersistLinkFailed, "mark publication bundle_validation_failed", err)
		}

		lockedRun, lockErr := qtx.GetUnifiedAnalysisRunByIDForUpdate(ctx, run.ID)
		if lockErr != nil {
			return WrapError(CodePersistLinkFailed, "reload unified analysis run for update", lockErr)
		}
		if lockedRun.Status == unifiedanalysis.StatusPublicationReady || lockedRun.Status == unifiedanalysis.StatusPublishing {
			if _, tErr := qtx.TransitionUnifiedAnalysisRunStatus(ctx, dbgen.TransitionUnifiedAnalysisRunStatusParams{
				ID:             lockedRun.ID,
				ExpectedStatus: lockedRun.Status,
				NextStatus:     unifiedanalysis.StatusPublicationFailed,
			}); tErr != nil {
				return WrapError(CodePersistLinkFailed, "transition unified analysis run to publication_failed", tErr)
			}
		}

		s.audit(ctx, qtx, run.SellerAccountID, "publish_bundle_validation_failed", "", run.ID, updated.ID, 0, 0, errorCode, report.Errors)
		result = s.resultFromRow(updated, OutcomeBundleValidationFailed)
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}
	return result, nil
}

// withSellerTx opens a transaction, acquires the per-seller publication
// advisory lock (shared with the legacy aipipeline publisher's lock key
// convention: pg_advisory_xact_lock(seller_account_id)), runs fn, and commits
// on success. fn's returned error rolls the transaction back.
func (s *Service) withSellerTx(ctx context.Context, sellerAccountID int64, fn func(ctx context.Context, tx pgx.Tx, qtx *dbgen.Queries) error) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return WrapError(CodeConcurrentPublishConflict, "begin publish transaction", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", sellerAccountID); err != nil {
		return WrapError(CodeConcurrentPublishConflict, "acquire seller publication lock", err)
	}
	qtx := s.queries.WithTx(tx)
	if err := fn(ctx, tx, qtx); err != nil {
		return err
	}
	if failErr := s.maybeFail(FailBeforeCommit); failErr != nil {
		return failErr
	}
	return tx.Commit(ctx)
}

// checkStale reports whether metricDateStr/candidateRunID is stale relative
// to the seller's current visible unified publication (in which case
// publishing this run would regress the pointer) and returns the current
// publication id (0 if none exists yet) so callers can supersede it once the
// new one is published.
//
// Two cases make a candidate stale:
//  1. Its metric_date is strictly older than the current publication's
//     metric_date.
//  2. Its metric_date is the SAME as the current publication's, but its
//     run_id is not strictly newer than the current publication's run_id.
//     Same-day re-runs are expected (e.g. a corrected re-analysis of
//     today's data): only a strictly newer run_id may replace the current
//     publication for that date, so an equal-or-older run_id publishing
//     after a newer one already went current must not regress the pointer.
func (s *Service) checkStale(ctx context.Context, qtx *dbgen.Queries, sellerAccountID int64, metricDateStr string, candidateRunID int64, allowOlderMetricDate bool) (stale bool, currentPublicationID int64, err error) {
	current, cErr := qtx.GetSellerCurrentUnifiedPublicationForUpdate(ctx, sellerAccountID)
	if cErr != nil {
		if errors.Is(cErr, pgx.ErrNoRows) {
			return false, 0, nil
		}
		return false, 0, WrapError(CodeStaleRelativeToCurrent, "load seller current unified publication", cErr)
	}
	currentDateStr := current.MetricDate.Time.UTC().Format("2006-01-02")
	if currentDateStr > metricDateStr {
		if allowOlderMetricDate {
			return false, current.PublicationID, nil
		}
		return true, current.PublicationID, nil
	}
	if currentDateStr == metricDateStr {
		currentPub, pubErr := qtx.GetUnifiedAnalysisPublicationByID(ctx, current.PublicationID)
		if pubErr != nil {
			return false, current.PublicationID, WrapError(CodeStaleRelativeToCurrent, "load current publication for run_id comparison", pubErr)
		}
		if currentPub.RunID > candidateRunID {
			return true, current.PublicationID, nil
		}
	}
	return false, current.PublicationID, nil
}

// getOrCreatePublicationRow returns the unified_analysis_publications row
// for this run, creating it (status=publishing) if this is the first publish
// attempt, or resetting it to publishing if a previous attempt for the same
// run_id left it in a non-terminal or failed state (run_id is unique on this
// table, so retries must reuse the same row).
func (s *Service) getOrCreatePublicationRow(
	ctx context.Context,
	qtx *dbgen.Queries,
	run dbgen.AiPipelineRun,
	existing dbgen.UnifiedAnalysisPublication,
	hasExisting bool,
	identity publicationIdentity,
	pubMode PublicationMode,
	visibility Visibility,
	analysisOutcome string,
) (dbgen.UnifiedAnalysisPublication, error) {
	if hasExisting {
		locked, err := qtx.GetUnifiedAnalysisPublicationByIDForUpdate(ctx, existing.ID)
		if err != nil {
			return dbgen.UnifiedAnalysisPublication{}, WrapError(CodeRunNotFound, "reload existing publication for update", err)
		}
		if locked.Status == string(StatusPublished) {
			return locked, nil
		}
		reset, err := qtx.UpdateUnifiedAnalysisPublicationStatus(ctx, dbgen.UpdateUnifiedAnalysisPublicationStatusParams{
			ID:     locked.ID,
			Status: string(StatusPublishing),
		})
		if err != nil {
			return dbgen.UnifiedAnalysisPublication{}, WrapError(CodePersistLinkFailed, "reset publication to publishing", err)
		}
		return reset, nil
	}

	created, err := qtx.CreateUnifiedAnalysisPublication(ctx, dbgen.CreateUnifiedAnalysisPublicationParams{
		SellerAccountID: run.SellerAccountID,
		MetricDate:      toDate(run.MetricDate.Time),
		RunID:           run.ID,
		PackID:          identity.PackID,
		PackHash:        identity.PackHash,
		PromptVersion:   identity.PromptVersion,
		PromptHash:      identity.PromptHash,
		SchemaVersion:   identity.SchemaVersion,
		SchemaHash:      identity.SchemaHash,
		OutputHash:      identity.OutputHash,
		AdapterVersion:  adapter.Version,
		AdapterHash:     adapter.Hash(),
		Mode:            string(pubMode),
		Visibility:      string(visibility),
		Status:          string(StatusPublishing),
		AnalysisOutcome: analysisOutcome,
	})
	if err != nil {
		return dbgen.UnifiedAnalysisPublication{}, WrapError(CodePersistLinkFailed, "create unified analysis publication", err)
	}
	return created, nil
}

// persistDashboard writes the projected dashboard narrative + frozen factual
// snapshot for pipelineRunID, reusing the existing narrative row when this
// pipeline run was already linked to one by a previous (failed) publish
// attempt, so retries never violate the one-narrative-per-pipeline-run
// unique index.
func (s *Service) persistDashboard(
	ctx context.Context,
	qtx *dbgen.Queries,
	run dbgen.AiPipelineRun,
	metricDate time.Time,
	proj adapter.DashboardProjection,
	dataPackHash string,
	globalContextHash string,
) (narrativeID pgtype.UUID, snapshotID int64, err error) {
	validatedPayload, err := json.Marshal(proj.Narrative)
	if err != nil {
		return pgtype.UUID{}, 0, fmt.Errorf("marshal narrative payload: %w", err)
	}
	contextHash := adapter.HashBytes(validatedPayload)
	now := s.now()

	var healthScore pgtype.Int4
	if proj.HealthScore != nil {
		healthScore = pgtype.Int4{Int32: *proj.HealthScore, Valid: true}
	}

	if run.DashboardRunID.Valid {
		narrativeID = run.DashboardRunID
	} else {
		inserted, insErr := qtx.InsertAIDashboardNarrative(ctx, dbgen.InsertAIDashboardNarrativeParams{
			SellerAccountID: run.SellerAccountID,
			MetricDate:      toDate(metricDate),
			Status:          "running",
			HealthScore:     healthScore,
			Severity:        pgtype.Text{String: proj.Severity, Valid: proj.Severity != ""},
			RiskDriversHash: contextHash,
			ContextHash:     contextHash,
			PromptVersion:   unifiedNarrativeGenerator,
			Model:           pgtype.Text{String: unifiedNarrativeModel, Valid: true},
			StartedAt:       pgtype.Timestamptz{Time: now, Valid: true},
		})
		if insErr != nil {
			return pgtype.UUID{}, 0, fmt.Errorf("insert dashboard narrative: %w", insErr)
		}
		narrativeID = inserted.ID
	}

	completed, err := qtx.CompleteAIDashboardNarrative(ctx, dbgen.CompleteAIDashboardNarrativeParams{
		ID:               narrativeID,
		SellerAccountID:  run.SellerAccountID,
		Status:           "completed",
		ValidatedPayload: validatedPayload,
		ValidationErrors: []byte("[]"),
		FinishedAt:       pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		return pgtype.UUID{}, 0, fmt.Errorf("complete dashboard narrative: %w", err)
	}

	if err := qtx.ClearAIDashboardNarrativePipelineLinkExcept(ctx, dbgen.ClearAIDashboardNarrativePipelineLinkExceptParams{
		AiPipelineRunID: pgtype.Int8{Int64: run.ID, Valid: true},
		ID:              completed.ID,
	}); err != nil {
		return pgtype.UUID{}, 0, fmt.Errorf("clear previous dashboard pipeline link: %w", err)
	}
	if _, err := qtx.UpdateAIDashboardNarrativePipelineFields(ctx, dbgen.UpdateAIDashboardNarrativePipelineFieldsParams{
		ID:                completed.ID,
		SellerAccountID:   run.SellerAccountID,
		AiPipelineRunID:   pgtype.Int8{Int64: run.ID, Valid: true},
		DataPackHash:      nullableText(dataPackHash),
		GlobalContextHash: nullableText(globalContextHash),
	}); err != nil {
		return pgtype.UUID{}, 0, fmt.Errorf("link dashboard narrative to pipeline: %w", err)
	}

	frozenJSON, err := json.Marshal(proj.Frozen)
	if err != nil {
		return pgtype.UUID{}, 0, fmt.Errorf("marshal frozen dashboard data: %w", err)
	}
	payloadHash, err := proj.Frozen.CanonicalHash()
	if err != nil {
		return pgtype.UUID{}, 0, fmt.Errorf("hash frozen dashboard data: %w", err)
	}
	snapshot, err := qtx.InsertAIDashboardDataSnapshot(ctx, dbgen.InsertAIDashboardDataSnapshotParams{
		AiPipelineRunID:   run.ID,
		SellerAccountID:   run.SellerAccountID,
		MetricDate:        toDate(metricDate),
		DataPackHash:      dataPackHash,
		GlobalContextHash: globalContextHash,
		SchemaVersion:     dashboardhome.FrozenDashboardDataSchemaVersion,
		PayloadJson:       frozenJSON,
		PayloadHash:       payloadHash,
		Status:            "completed",
		FinishedAt:        pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		return pgtype.UUID{}, 0, fmt.Errorf("insert dashboard data snapshot: %w", err)
	}
	return narrativeID, snapshot.ID, nil
}

// persistActions writes the projected action items and the unified
// publication <-> action_items mapping. It reuses the existing
// action_generation_runs row when this pipeline run already has one linked
// (previous failed publish attempt), so retries never violate the
// one-actions-run-per-pipeline-run unique index.
func (s *Service) persistActions(
	ctx context.Context,
	tx pgx.Tx,
	qtx *dbgen.Queries,
	run dbgen.AiPipelineRun,
	metricDate time.Time,
	out *contract.UnifiedAnalysisOutputV1,
	projected []adapter.ProjectedAction,
	publicationID int64,
	dataPackHash string,
	globalContextHash string,
) (actionsRunID int64, err error) {
	if run.ActionsRunID.Valid {
		actionsRunID = run.ActionsRunID.Int64
	} else {
		created, createErr := qtx.CreateActionGenerationRun(ctx, dbgen.CreateActionGenerationRunParams{
			SellerAccountID: run.SellerAccountID,
			MetricDate:      toDate(metricDate),
			Status:          actions.GenerationRunStatusCompleted,
		})
		if createErr != nil {
			return 0, fmt.Errorf("create action generation run: %w", createErr)
		}
		actionsRunID = created.ID
	}

	topKeys := make([]string, 0, len(projected))
	itemIDByActionID := make(map[string]int64, len(projected))
	for i, pa := range projected {
		params := pa.Params
		params.RunID = actionsRunID
		item, upsertErr := qtx.UpsertActionItem(ctx, toDBUpsertActionItemParams(params))
		if upsertErr != nil {
			return 0, fmt.Errorf("upsert action item %q: %w", params.ActionKey, upsertErr)
		}
		itemIDByActionID[pa.AnalysisActionID] = item.ID
		if pa.IsTop {
			topKeys = append(topKeys, params.ActionKey)
		}
		if i == 0 {
			if failErr := s.maybeFail(FailAfterFirstActionInserted); failErr != nil {
				return 0, failErr
			}
		}
	}
	if failErr := s.maybeFail(FailAfterAllActionsInserted); failErr != nil {
		return 0, failErr
	}

	finalStatus := actions.GenerationFinalStatusCompleted
	if len(projected) == 0 {
		finalStatus = actions.GenerationFinalStatusEmpty
	}
	topKeysJSON, _ := json.Marshal(topKeys)
	notesJSON, _ := json.Marshal(out.Limitations)
	if _, err := qtx.UpdateActionGenerationRun(ctx, dbgen.UpdateActionGenerationRunParams{
		ID:                   actionsRunID,
		Status:               actions.GenerationRunStatusCompleted,
		ContextHash:          nullableText(globalContextHash),
		FinalStatus:          nullableText(finalStatus),
		AcceptedCount:        int32(len(projected)),
		TopActionKeysJson:    topKeysJSON,
		DataQualityNotesJson: notesJSON,
	}); err != nil {
		return 0, fmt.Errorf("update action generation run: %w", err)
	}

	if err := qtx.ClearActionGenerationRunPipelineLinkExcept(ctx, dbgen.ClearActionGenerationRunPipelineLinkExceptParams{
		AiPipelineRunID: pgtype.Int8{Int64: run.ID, Valid: true},
		ID:              actionsRunID,
	}); err != nil {
		return 0, fmt.Errorf("clear previous actions pipeline link: %w", err)
	}
	if _, err := qtx.UpdateActionGenerationRunPipelineFields(ctx, dbgen.UpdateActionGenerationRunPipelineFieldsParams{
		ID:                actionsRunID,
		AiPipelineRunID:   pgtype.Int8{Int64: run.ID, Valid: true},
		DataPackHash:      nullableText(dataPackHash),
		GlobalContextHash: nullableText(globalContextHash),
	}); err != nil {
		return 0, fmt.Errorf("link action generation run to pipeline: %w", err)
	}

	// Retries reuse the same publication_id (run_id is unique on
	// unified_analysis_publications), so clear any action map rows from a
	// previous partial attempt before re-inserting to avoid unique
	// violations on (publication_id, analysis_action_id/action_item_id).
	if _, err := tx.Exec(ctx, "DELETE FROM unified_publication_action_map WHERE publication_id = $1", publicationID); err != nil {
		return 0, fmt.Errorf("clear previous action map rows: %w", err)
	}
	for _, pa := range projected {
		itemID, ok := itemIDByActionID[pa.AnalysisActionID]
		if !ok {
			continue
		}
		if _, err := qtx.InsertUnifiedPublicationActionMap(ctx, dbgen.InsertUnifiedPublicationActionMapParams{
			PublicationID:    publicationID,
			AnalysisActionID: pa.AnalysisActionID,
			ActionItemID:     itemID,
			IsTop:            pa.IsTop,
			IsRecommended:    pa.IsRecommended,
			SortOrder:        int32(pa.SortOrder),
		}); err != nil {
			return 0, fmt.Errorf("insert unified publication action map for %q: %w", pa.AnalysisActionID, err)
		}
	}

	// Reconcile/repair can invent new analysis_action_ids while old keys remain
	// in action_items for the shared generation run. ListActions reads the whole
	// run, so drop anything not mapped to this publication.
	if _, err := tx.Exec(ctx, `
DELETE FROM action_items
WHERE run_id = $1
  AND id NOT IN (
    SELECT action_item_id
    FROM unified_publication_action_map
    WHERE publication_id = $2
  )`, actionsRunID, publicationID); err != nil {
		return 0, fmt.Errorf("delete orphan action items for run: %w", err)
	}

	return actionsRunID, nil
}

// audit writes a best-effort row to unified_analysis_publication_audit.
// Failures are logged-and-swallowed rather than failing the publish: the
// audit trail is diagnostic, not authoritative state.
func (s *Service) audit(
	ctx context.Context,
	qtx *dbgen.Queries,
	sellerAccountID int64,
	operation, actor string,
	runID, publicationID, previousPublicationID, newPublicationID int64,
	reason string,
	payload any,
) {
	var payloadJSON []byte
	if payload != nil {
		payloadJSON, _ = json.Marshal(payload)
	}
	_, _ = qtx.InsertUnifiedAnalysisPublicationAudit(ctx, dbgen.InsertUnifiedAnalysisPublicationAuditParams{
		SellerAccountID:       sellerAccountID,
		Operation:             operation,
		Actor:                 actorOrDefault(actor),
		RunID:                 pgtype.Int8{Int64: runID, Valid: runID > 0},
		PublicationID:         pgtype.Int8{Int64: publicationID, Valid: publicationID > 0},
		PreviousPublicationID: pgtype.Int8{Int64: previousPublicationID, Valid: previousPublicationID > 0},
		NewPublicationID:      pgtype.Int8{Int64: newPublicationID, Valid: newPublicationID > 0},
		Reason:                pgtype.Text{String: reason, Valid: reason != ""},
		PayloadJson:           payloadJSON,
	})
}

// finalizeRunIfAlreadyPublished ensures ai_pipeline_runs reaches status=published
// when a unified_analysis_publications row for the run is already published.
// Safe to call outside a seller TX; uses its own short update.
func (s *Service) finalizeRunIfAlreadyPublished(ctx context.Context, run dbgen.AiPipelineRun) error {
	if run.Status == unifiedanalysis.StatusPublished {
		return nil
	}
	if run.Status != unifiedanalysis.StatusPublishing && run.Status != unifiedanalysis.StatusPublicationReady {
		return nil
	}
	now := s.now()
	_, err := s.queries.SetAIPipelinePublishedAt(ctx, dbgen.SetAIPipelinePublishedAtParams{
		ID:          run.ID,
		PublishedAt: pgtype.Timestamptz{Time: now, Valid: true},
		Status:      pgtype.Text{String: unifiedanalysis.StatusPublished, Valid: true},
		CompletedAt: pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		return WrapError(CodePersistLinkFailed, "finalize already-published run status", err)
	}
	return nil
}

func (s *Service) finalizeRunPublishedLocked(ctx context.Context, qtx *dbgen.Queries, run dbgen.AiPipelineRun) error {
	if run.Status == unifiedanalysis.StatusPublished {
		return nil
	}
	now := s.now()
	_, err := qtx.SetAIPipelinePublishedAt(ctx, dbgen.SetAIPipelinePublishedAtParams{
		ID:          run.ID,
		PublishedAt: pgtype.Timestamptz{Time: now, Valid: true},
		Status:      pgtype.Text{String: unifiedanalysis.StatusPublished, Valid: true},
		CompletedAt: pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		return WrapError(CodePersistLinkFailed, "finalize already-published run status under lock", err)
	}
	return nil
}

func (s *Service) resultFromRow(row dbgen.UnifiedAnalysisPublication, outcome PublishOutcome) *PublishResult {
	result := &PublishResult{
		Outcome:                 outcome,
		PublicationID:           row.ID,
		RunID:                   row.RunID,
		SellerAccountID:         row.SellerAccountID,
		MetricDate:              row.MetricDate.Time.UTC().Format("2006-01-02"),
		Mode:                    PublicationMode(row.Mode),
		Visibility:              Visibility(row.Visibility),
		Status:                  Status(row.Status),
		AnalysisOutcome:         row.AnalysisOutcome,
		OutputHash:              row.OutputHash,
		BundleHash:              row.BundleHash,
		ActionsRunID:            row.ActionsRunID.Int64,
		DashboardDataSnapshotID: row.DashboardDataSnapshotID.Int64,
		DashboardRecordCount:    int(row.DashboardRecordCount),
		ActionsRecordCount:      int(row.ActionsRecordCount),
		TopActionsCount:         int(row.TopActionsCount),
		RecommendedActionsCount: int(row.RecommendedActionsCount),
	}
	result.DashboardRunID = uuidString(row.DashboardRunID)
	if row.ErrorCode.Valid {
		result.ErrorCode = row.ErrorCode.String
	}
	if row.ErrorMessage.Valid {
		result.ErrorMessage = row.ErrorMessage.String
	}
	if row.PublishedAt.Valid {
		result.PublishedAt = row.PublishedAt.Time
	}
	return result
}

func toDBUpsertActionItemParams(p actions.UpsertActionItemParams) dbgen.UpsertActionItemParams {
	return dbgen.UpsertActionItemParams{
		SellerAccountID:        p.SellerAccountID,
		RunID:                  pgtype.Int8{Int64: p.RunID, Valid: p.RunID > 0},
		ActionKey:              p.ActionKey,
		Type:                   string(p.Type),
		Category:               string(p.Category),
		Priority:               string(p.Priority),
		Title:                  p.Title,
		Summary:                nullableText(p.Summary),
		Reason:                 p.Reason,
		NextStep:               p.NextStep,
		ExpectedEffect:         nullableText(p.ExpectedEffect),
		EvidenceJson:           p.EvidenceJSON,
		LimitationsJson:        p.LimitationsJSON,
		CtaJson:                p.CTAJSON,
		AffectedEntitiesJson:   p.AffectedEntitiesJSON,
		EntityGroupRef:         nullableText(p.EntityGroupRef),
		SourceCandidateIdsJson: p.SourceCandidateIDs,
		EvidenceFactIdsJson:    p.EvidenceFactIDs,
		PayloadJson:            p.PayloadJSON,
		MetricDate:             toDate(p.MetricDate),
	}
}

func nullableText(v string) pgtype.Text {
	if v == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: v, Valid: true}
}

func toDate(t time.Time) pgtype.Date {
	t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	return pgtype.Date{Time: t, Valid: true}
}

// loadPreviousDashboardFacts loads Dashboard Facts from the seller's current
// visible publication (when that publication belongs to a different run).
// Returns nil when no prior published facts are available.
func (s *Service) loadPreviousDashboardFacts(ctx context.Context, sellerAccountID, currentRunID int64) *dashboardfacts.DashboardFactsV1 {
	if s == nil || s.queries == nil || s.artifacts == nil || sellerAccountID <= 0 {
		return nil
	}
	current, err := s.queries.GetSellerCurrentUnifiedPublication(ctx, sellerAccountID)
	if err != nil || current.RunID <= 0 || current.RunID == currentRunID {
		return nil
	}
	if Status(current.Status) != StatusPublished || Visibility(current.Visibility) != VisibilityVisible {
		return nil
	}
	facts, ok := s.artifacts.LoadDashboardFacts(ctx, current.RunID)
	if !ok {
		return nil
	}
	return &facts
}

func actorOrDefault(actor string) string {
	actor = strings.TrimSpace(actor)
	if actor == "" {
		return "system"
	}
	return actor
}

func uuidString(v pgtype.UUID) string {
	if !v.Valid {
		return ""
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", v.Bytes[0:4], v.Bytes[4:6], v.Bytes[6:8], v.Bytes[8:10], v.Bytes[10:16])
}
