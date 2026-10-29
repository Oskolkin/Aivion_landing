package publication

import (
	"context"
	"errors"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// RollbackRequest is the input to Service.RollbackPublication.
type RollbackRequest struct {
	// SellerAccountID identifies whose current visible unified publication
	// should be rolled back. Required.
	SellerAccountID int64
	// TargetPublicationID is a previously published visible (or promotable)
	// publication belonging to the same seller that should become current.
	// Required. The current publication is marked superseded (not deleted);
	// immutable dashboard/actions rows stay intact.
	TargetPublicationID int64
	// Actor identifies who/what triggered the rollback. Recorded on the
	// audit trail.
	Actor string
	// Reason is a free-text explanation recorded on the audit trail.
	Reason string
}

// RollbackPublication switches the seller's current visible unified
// publication pointer to TargetPublicationID. The previously-current
// publication is marked superseded (immutable data is preserved). Both
// seller_current_unified_publications and seller_current_ai_snapshots are
// updated atomically so Dashboard and Actions APIs keep reading one bundle.
func (s *Service) RollbackPublication(ctx context.Context, req RollbackRequest) (*PublishResult, error) {
	if s == nil || s.pool == nil || s.queries == nil {
		return nil, NewError(CodeSettingsInvalid, "publication service is not configured")
	}
	if req.SellerAccountID <= 0 {
		return nil, NewError(CodeRollbackNotAllowed, "seller_account_id is required")
	}
	if req.TargetPublicationID <= 0 {
		return nil, NewError(CodeRollbackNotAllowed, "target_publication_id is required")
	}

	var result *PublishResult
	txErr := s.withSellerTx(ctx, req.SellerAccountID, func(ctx context.Context, tx pgx.Tx, qtx *dbgen.Queries) error {
		current, curErr := qtx.GetSellerCurrentUnifiedPublicationForUpdate(ctx, req.SellerAccountID)
		if curErr != nil {
			if errors.Is(curErr, pgx.ErrNoRows) {
				return NewError(CodeRollbackNotAllowed, "seller has no current unified publication to roll back")
			}
			return WrapError(CodeRollbackNotAllowed, "load seller current unified publication", curErr)
		}
		if current.PublicationID == req.TargetPublicationID {
			return NewError(CodeRollbackNotAllowed, "target publication is already current")
		}

		currentPub, pubErr := qtx.GetUnifiedAnalysisPublicationByIDForUpdate(ctx, current.PublicationID)
		if pubErr != nil {
			return WrapError(CodeRollbackNotAllowed, "load current publication", pubErr)
		}

		target, targetErr := qtx.GetUnifiedAnalysisPublicationByIDForUpdate(ctx, req.TargetPublicationID)
		if targetErr != nil {
			if errors.Is(targetErr, pgx.ErrNoRows) {
				return Errorf(CodeRollbackNotAllowed, "target publication %d not found", req.TargetPublicationID)
			}
			return WrapError(CodeRollbackNotAllowed, "load target publication", targetErr)
		}
		if target.SellerAccountID != req.SellerAccountID {
			return NewError(CodeRollbackNotAllowed, "target publication belongs to a different seller")
		}
		if Status(target.Status) != StatusPublished && Status(target.Status) != StatusSuperseded {
			return Errorf(CodeRollbackNotAllowed, "target publication %d is %s, not publishable", target.ID, target.Status)
		}
		if Visibility(target.Visibility) != VisibilityVisible {
			return Errorf(CodeRollbackNotAllowed, "target publication %d is not visible (shadow cannot be rolled back onto without promote)", target.ID)
		}
		if !target.DashboardRunID.Valid || !target.ActionsRunID.Valid || !target.DashboardDataSnapshotID.Valid {
			return Errorf(CodeRollbackNotAllowed, "target publication %d is missing dashboard/actions projections", target.ID)
		}

		if Status(currentPub.Status) == StatusPublished {
			if _, supErr := qtx.MarkUnifiedAnalysisPublicationSuperseded(ctx, dbgen.MarkUnifiedAnalysisPublicationSupersededParams{
				ID:                        currentPub.ID,
				SupersededByPublicationID: pgtype.Int8{Int64: target.ID, Valid: true},
			}); supErr != nil {
				return WrapError(CodeRollbackNotAllowed, "mark current publication superseded", supErr)
			}
		}

		// Re-activate a previously superseded target so it is again "published".
		if Status(target.Status) != StatusPublished {
			if _, updErr := qtx.UpdateUnifiedAnalysisPublicationStatus(ctx, dbgen.UpdateUnifiedAnalysisPublicationStatusParams{
				ID:     target.ID,
				Status: string(StatusPublished),
			}); updErr != nil {
				return WrapError(CodeRollbackNotAllowed, "reactivate target publication", updErr)
			}
			// Clear superseded metadata with a direct update (no dedicated sqlc).
			if _, execErr := tx.Exec(ctx,
				`UPDATE unified_analysis_publications SET superseded_at = NULL, superseded_by_publication_id = NULL, updated_at = now() WHERE id = $1`,
				target.ID,
			); execErr != nil {
				return WrapError(CodeRollbackNotAllowed, "clear target superseded metadata", execErr)
			}
			reloaded, getErr := qtx.GetUnifiedAnalysisPublicationByID(ctx, target.ID)
			if getErr != nil {
				return WrapError(CodeRollbackNotAllowed, "reload target publication", getErr)
			}
			target = reloaded
		}

		if _, curSetErr := qtx.UpsertSellerCurrentUnifiedPublication(ctx, dbgen.UpsertSellerCurrentUnifiedPublicationParams{
			SellerAccountID: req.SellerAccountID,
			PublicationID:   target.ID,
			MetricDate:      target.MetricDate,
		}); curSetErr != nil {
			return WrapError(CodeRollbackNotAllowed, "set seller current unified publication", curSetErr)
		}
		if snapErr := qtx.UpsertSellerCurrentAISnapshot(ctx, dbgen.UpsertSellerCurrentAISnapshotParams{
			SellerAccountID: req.SellerAccountID,
			PipelineRunID:   target.RunID,
		}); snapErr != nil {
			return WrapError(CodeRollbackNotAllowed, "set seller current ai snapshot pointer", snapErr)
		}

		s.audit(ctx, qtx, req.SellerAccountID, "rollback", req.Actor, target.RunID, target.ID, currentPub.ID, target.ID, req.Reason, nil)
		result = s.resultFromRow(target, OutcomeRolledBack)
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}
	return result, nil
}

// PromoteRequest is the input to Service.PromoteShadowPublication.
type PromoteRequest struct {
	// PublicationID is the unified_analysis_publications.id of a
	// published, shadow-visibility publication to promote. Required.
	PublicationID int64
	// Actor identifies who/what triggered the promotion. Recorded on the
	// audit trail.
	Actor string
	// Reason is a free-text explanation, recorded on the audit trail.
	Reason string
}

// PromoteShadowPublication flips an existing published, shadow-visibility
// publication to visible and makes it the seller's current unified
// publication, superseding whatever was current before. This is intended
// for admin/QA flows that want to manually promote a specific already-run
// shadow publication (e.g. after review) without waiting for a fresh
// PublishRun call.
//
// The dashboard/actions rows the shadow publication already wrote are
// reused as-is; promotion only updates unified_analysis_publications
// visibility/mode and the seller_current_unified_publications pointer.
func (s *Service) PromoteShadowPublication(ctx context.Context, req PromoteRequest) (*PublishResult, error) {
	if s == nil || s.pool == nil || s.queries == nil {
		return nil, NewError(CodeSettingsInvalid, "publication service is not configured")
	}
	if req.PublicationID <= 0 {
		return nil, NewError(CodePromotionNotAllowed, "publication_id is required")
	}

	shadow, err := s.queries.GetUnifiedAnalysisPublicationByID(ctx, req.PublicationID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, Errorf(CodePromotionNotAllowed, "publication %d not found", req.PublicationID)
		}
		return nil, WrapError(CodePromotionNotAllowed, "load publication", err)
	}
	if Status(shadow.Status) != StatusPublished {
		return nil, Errorf(CodePromotionNotAllowed, "publication %d is %s, not published", shadow.ID, shadow.Status)
	}
	if Visibility(shadow.Visibility) == VisibilityVisible {
		return nil, Errorf(CodePromotionNotAllowed, "publication %d is already visible", shadow.ID)
	}

	var result *PublishResult
	txErr := s.withSellerTx(ctx, shadow.SellerAccountID, func(ctx context.Context, tx pgx.Tx, qtx *dbgen.Queries) error {
		locked, lockErr := qtx.GetUnifiedAnalysisPublicationByIDForUpdate(ctx, shadow.ID)
		if lockErr != nil {
			return WrapError(CodePromotionNotAllowed, "reload publication for update", lockErr)
		}
		if Status(locked.Status) != StatusPublished || Visibility(locked.Visibility) == VisibilityVisible {
			return Errorf(CodePromotionNotAllowed, "publication %d is no longer eligible for promotion", locked.ID)
		}

		var previousCurrentID int64
		if current, curErr := qtx.GetSellerCurrentUnifiedPublicationForUpdate(ctx, locked.SellerAccountID); curErr == nil {
			previousCurrentID = current.PublicationID
		} else if !errors.Is(curErr, pgx.ErrNoRows) {
			return WrapError(CodePromotionNotAllowed, "load seller current unified publication", curErr)
		}

		// visibility/mode have no dedicated sqlc mutation (they are normally
		// fixed at creation time by PublishRun), so promotion updates them
		// directly. This is the only raw-SQL mutation in the publication
		// package's rollback/promotion paths.
		if _, execErr := tx.Exec(ctx,
			`UPDATE unified_analysis_publications SET visibility = 'visible', mode = 'unified', updated_at = now() WHERE id = $1`,
			locked.ID,
		); execErr != nil {
			return WrapError(CodePromotionNotAllowed, "promote publication visibility", execErr)
		}
		promoted, getErr := qtx.GetUnifiedAnalysisPublicationByID(ctx, locked.ID)
		if getErr != nil {
			return WrapError(CodePromotionNotAllowed, "reload promoted publication", getErr)
		}

		if _, curErr := qtx.UpsertSellerCurrentUnifiedPublication(ctx, dbgen.UpsertSellerCurrentUnifiedPublicationParams{
			SellerAccountID: locked.SellerAccountID,
			PublicationID:   promoted.ID,
			MetricDate:      promoted.MetricDate,
		}); curErr != nil {
			return WrapError(CodePromotionNotAllowed, "set seller current unified publication", curErr)
		}
		// Mirror PublishRun(visible): existing Dashboard/Actions APIs read
		// seller_current_ai_snapshots, so promotion must switch that pointer
		// onto the unified pipeline run in the same transaction.
		if snapErr := qtx.UpsertSellerCurrentAISnapshot(ctx, dbgen.UpsertSellerCurrentAISnapshotParams{
			SellerAccountID: locked.SellerAccountID,
			PipelineRunID:   promoted.RunID,
		}); snapErr != nil {
			return WrapError(CodePromotionNotAllowed, "set seller current ai snapshot pointer", snapErr)
		}
		if previousCurrentID > 0 && previousCurrentID != promoted.ID {
			if _, supErr := qtx.MarkUnifiedAnalysisPublicationSuperseded(ctx, dbgen.MarkUnifiedAnalysisPublicationSupersededParams{
				ID:                        previousCurrentID,
				SupersededByPublicationID: pgtype.Int8{Int64: promoted.ID, Valid: true},
			}); supErr != nil {
				return WrapError(CodePromotionNotAllowed, "mark previous publication superseded", supErr)
			}
		}

		s.audit(ctx, qtx, locked.SellerAccountID, "promote_shadow", req.Actor, promoted.RunID, promoted.ID, previousCurrentID, promoted.ID, req.Reason, nil)
		result = s.resultFromRow(promoted, OutcomePromoted)
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}
	return result, nil
}
