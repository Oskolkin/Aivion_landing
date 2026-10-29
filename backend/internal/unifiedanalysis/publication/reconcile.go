package publication

import (
	"context"
	"errors"
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5"
)

// ReconcileResult reports the outcome of
// Service.ReconcileCurrentPublicationPointers for one seller.
type ReconcileResult struct {
	SellerAccountID int64
	// Checked reports whether there was a current unified publication to
	// reconcile against at all. False (with every other field at its zero
	// value) when the seller has no seller_current_unified_publications row,
	// or that row's publication is not itself published+visible — in both
	// cases there is nothing authoritative to repair the compat pointer
	// from, so this is always a no-op (see ReconcileCurrentPublicationPointers doc).
	Checked bool
	// PublicationID is the seller's current unified publication (0 unless Checked).
	PublicationID int64
	// ExpectedRunID is PublicationID's run_id: what
	// seller_current_ai_snapshots.pipeline_run_id should reference.
	ExpectedRunID int64
	// ActualRunID is what seller_current_ai_snapshots.pipeline_run_id
	// referenced at the time of the check (0 if the seller has no compat
	// pointer row at all).
	ActualRunID int64
	// Mismatched reports whether ActualRunID != ExpectedRunID was observed.
	Mismatched bool
	// Repaired reports whether the compat pointer was actually updated to
	// ExpectedRunID (always false when DryRun, or when !Mismatched).
	Repaired bool
	// DryRun echoes the dryRun argument: when true, Mismatched/Checked are
	// still computed and reported, but Repaired is always false.
	DryRun bool
}

// ReconcileCurrentPublicationPointers detects — and, unless dryRun, repairs
// — a seller_current_ai_snapshots ("compat") pointer that has drifted from
// the seller's current unified publication pointer
// (seller_current_unified_publications, the authoritative Stage 5 pointer).
// Drift is only expected as a defense-in-depth concern: PublishRun writes
// both pointers in the same database transaction (compat pointer first, see
// hooks.go FailAfterCompatPointer/FailAfterUnifiedPointer), so a crash
// between the two writes rolls the whole transaction back rather than
// leaving them inconsistent. This tool exists for cases outside that
// guarantee — e.g. a manual/legacy write to seller_current_ai_snapshots, a
// restored backup, or an as-yet-undiscovered bug — surfaced via the
// `reconcile-unified-publication` devtool for operators to run periodically
// or on-demand.
//
// Deliberately conservative:
//   - Never touches seller_current_unified_publications; the unified
//     pointer is always treated as ground truth and is never itself
//     repaired or replaced by this call.
//   - Never selects a different publication to become current — it only
//     ever makes the compat pointer agree with whatever the unified pointer
//     already says right now.
//   - If the seller has no current unified publication at all, there is
//     nothing authoritative to repair compat *from*, so this is a no-op: it
//     must never invent a pointer.
//   - Only repairs when the current unified publication is itself
//     status=published and visibility=visible; PublishRun itself never
//     projects a shadow or non-published publication onto the compat
//     table, so reconcile must not either.
func (s *Service) ReconcileCurrentPublicationPointers(ctx context.Context, sellerAccountID int64, dryRun bool) (ReconcileResult, error) {
	if s == nil || s.pool == nil || s.queries == nil {
		return ReconcileResult{}, NewError(CodeSettingsInvalid, "publication service is not configured")
	}
	if sellerAccountID <= 0 {
		return ReconcileResult{}, NewError(CodeSelectorInconsistentState, "seller_account_id is required")
	}

	current, err := s.queries.GetSellerCurrentUnifiedPublication(ctx, sellerAccountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Nothing authoritative to repair compat from: never invent a pointer.
			return ReconcileResult{SellerAccountID: sellerAccountID, DryRun: dryRun}, nil
		}
		return ReconcileResult{}, WrapError(CodeSelectorInconsistentState, "load seller current unified publication", err)
	}
	if Status(current.Status) != StatusPublished || Visibility(current.Visibility) != VisibilityVisible {
		// Mirrors PublishRun: only published+visible publications are ever
		// projected onto the compat pointer, so a current row that is
		// (no longer) published+visible has nothing to repair compat from.
		return ReconcileResult{SellerAccountID: sellerAccountID, DryRun: dryRun}, nil
	}

	result := ReconcileResult{
		SellerAccountID: sellerAccountID,
		Checked:         true,
		PublicationID:   current.PublicationID,
		ExpectedRunID:   current.RunID,
		DryRun:          dryRun,
	}

	actualRunID, snapErr := s.queries.GetSellerCurrentPipelineRunID(ctx, sellerAccountID)
	if snapErr != nil && !errors.Is(snapErr, pgx.ErrNoRows) {
		return ReconcileResult{}, WrapError(CodeSelectorInconsistentState, "load seller current ai snapshot pointer", snapErr)
	}
	result.ActualRunID = actualRunID
	result.Mismatched = actualRunID != current.RunID

	if !result.Mismatched || dryRun {
		return result, nil
	}

	txErr := s.withSellerTx(ctx, sellerAccountID, func(ctx context.Context, tx pgx.Tx, qtx *dbgen.Queries) error {
		// Re-check under the seller advisory lock: a concurrent publish may
		// have already repaired (or moved) the pointer since the read above.
		reCurrent, curErr := qtx.GetSellerCurrentUnifiedPublicationForUpdate(ctx, sellerAccountID)
		if curErr != nil {
			if errors.Is(curErr, pgx.ErrNoRows) {
				result.Checked = false
				result.Mismatched = false
				return nil
			}
			return WrapError(CodeSelectorInconsistentState, "reload seller current unified publication for update", curErr)
		}
		pub, pubErr := qtx.GetUnifiedAnalysisPublicationByID(ctx, reCurrent.PublicationID)
		if pubErr != nil {
			return WrapError(CodeSelectorInconsistentState, "reload current publication", pubErr)
		}
		if Status(pub.Status) != StatusPublished || Visibility(pub.Visibility) != VisibilityVisible {
			result.Mismatched = false
			return nil
		}
		result.PublicationID = pub.ID
		result.ExpectedRunID = pub.RunID

		reActual, snapErr2 := qtx.GetSellerCurrentPipelineRunID(ctx, sellerAccountID)
		if snapErr2 != nil && !errors.Is(snapErr2, pgx.ErrNoRows) {
			return WrapError(CodeSelectorInconsistentState, "reload seller current ai snapshot pointer", snapErr2)
		}
		result.ActualRunID = reActual
		if reActual == pub.RunID {
			result.Mismatched = false
			return nil
		}

		if err := qtx.UpsertSellerCurrentAISnapshot(ctx, dbgen.UpsertSellerCurrentAISnapshotParams{
			SellerAccountID: sellerAccountID,
			PipelineRunID:   pub.RunID,
		}); err != nil {
			return WrapError(CodePersistLinkFailed, "repair seller current ai snapshot pointer", err)
		}
		result.Repaired = true

		s.audit(ctx, qtx, sellerAccountID, "reconcile_pointers", "system", pub.RunID, pub.ID, 0, 0,
			fmt.Sprintf("compat pointer run_id %d -> %d", reActual, pub.RunID),
			map[string]any{"previous_compat_run_id": reActual, "new_compat_run_id": pub.RunID, "publication_id": pub.ID})
		return nil
	})
	if txErr != nil {
		return ReconcileResult{}, txErr
	}
	return result, nil
}

// ReconcileAllCurrentPublicationPointers walks sellers that have a current
// unified publication pointer and runs ReconcileCurrentPublicationPointers for
// each. Used by Worker startup recovery so pointer drift is repaired without
// requiring a manual reconcile-unified-publication invocation.
//
// Never invents pointers: sellers without seller_current_unified_publications
// are not listed. Failures for individual sellers are collected and returned
// as a multi-error string while other sellers continue.
func (s *Service) ReconcileAllCurrentPublicationPointers(ctx context.Context, limit int, dryRun bool) ([]ReconcileResult, error) {
	if s == nil || s.pool == nil {
		return nil, NewError(CodeSettingsInvalid, "publication service is not configured")
	}
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
		SELECT seller_account_id
		FROM seller_current_unified_publications
		ORDER BY seller_account_id
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, WrapError(CodeSelectorInconsistentState, "list sellers with current unified publication", err)
	}
	defer rows.Close()

	var sellerIDs []int64
	for rows.Next() {
		var id int64
		if scanErr := rows.Scan(&id); scanErr != nil {
			return nil, WrapError(CodeSelectorInconsistentState, "scan seller_account_id for reconcile", scanErr)
		}
		sellerIDs = append(sellerIDs, id)
	}
	if err := rows.Err(); err != nil {
		return nil, WrapError(CodeSelectorInconsistentState, "iterate sellers for reconcile", err)
	}

	out := make([]ReconcileResult, 0, len(sellerIDs))
	var firstErr error
	for _, sellerID := range sellerIDs {
		res, recErr := s.ReconcileCurrentPublicationPointers(ctx, sellerID, dryRun)
		if recErr != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("reconcile seller %d: %w", sellerID, recErr)
			}
			continue
		}
		out = append(out, res)
	}
	return out, firstErr
}
