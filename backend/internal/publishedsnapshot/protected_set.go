package publishedsnapshot

import (
	"context"
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5"
)

// ProtectedSnapshotIDs identifies rows that must never be deleted by cleanup/retention.
type ProtectedSnapshotIDs struct {
	PipelineRunID  int64
	DashboardRunID string
	ActionsRunID   int64
}

// CurrentProtectedSnapshotIDs returns protected IDs for the seller's current published snapshot.
func CurrentProtectedSnapshotIDs(ctx context.Context, resolver Resolver, sellerAccountID int64) (ProtectedSnapshotIDs, error) {
	if resolver == nil {
		return ProtectedSnapshotIDs{}, fmt.Errorf("resolver is required")
	}
	snapshot, err := resolver.GetCurrentPublishedSnapshot(ctx, sellerAccountID)
	if err != nil {
		return ProtectedSnapshotIDs{}, err
	}
	return ProtectedSnapshotIDs{
		PipelineRunID:  snapshot.PipelineRunID,
		DashboardRunID: snapshot.DashboardRunID,
		ActionsRunID:   snapshot.ActionsRunID,
	}, nil
}

// ContainsPipelineRun reports whether pipelineRunID belongs to the protected set.
func (p ProtectedSnapshotIDs) ContainsPipelineRun(pipelineRunID int64) bool {
	return pipelineRunID > 0 && p.PipelineRunID == pipelineRunID
}

// ContainsDashboardRun reports whether dashboardRunID belongs to the protected set.
func (p ProtectedSnapshotIDs) ContainsDashboardRun(dashboardRunID string) bool {
	return dashboardRunID != "" && p.DashboardRunID == dashboardRunID
}

// ContainsActionsRun reports whether actionsRunID belongs to the protected set.
func (p ProtectedSnapshotIDs) ContainsActionsRun(actionsRunID int64) bool {
	return actionsRunID > 0 && p.ActionsRunID == actionsRunID
}

// LoadCurrentProtectedSnapshotIDs is a convenience wrapper around SQLC queries.
func LoadCurrentProtectedSnapshotIDs(ctx context.Context, q *dbgen.Queries, sellerAccountID int64) (ProtectedSnapshotIDs, error) {
	return CurrentProtectedSnapshotIDs(ctx, NewSQLCResolver(q), sellerAccountID)
}

// ErrNoCurrentSnapshot indicates the seller has no published snapshot pointer.
var ErrNoCurrentSnapshot = pgx.ErrNoRows
