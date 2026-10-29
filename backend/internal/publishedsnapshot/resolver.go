package publishedsnapshot

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type Snapshot struct {
	PipelineRunID           int64
	SellerAccountID         int64
	MetricDate              time.Time
	DashboardDataSnapshotID int64
	DashboardRunID          string
	ActionsRunID            int64
	DataPackHash            string
	GlobalContextHash       string
	PublishedAt             time.Time
}

type Resolver interface {
	GetCurrentPublishedSnapshot(ctx context.Context, sellerAccountID int64) (Snapshot, error)
	GetPublishedSnapshotByPipelineID(ctx context.Context, sellerAccountID int64, pipelineRunID int64) (Snapshot, error)
}

type SQLCResolver struct {
	q *dbgen.Queries
}

func NewSQLCResolver(q *dbgen.Queries) *SQLCResolver {
	return &SQLCResolver{q: q}
}

func (r *SQLCResolver) GetCurrentPublishedSnapshot(ctx context.Context, sellerAccountID int64) (Snapshot, error) {
	row, err := r.q.GetCurrentPublishedSnapshotBySeller(ctx, sellerAccountID)
	if err != nil {
		return Snapshot{}, err
	}
	return mapCurrentSnapshotRow(row), nil
}

func (r *SQLCResolver) GetPublishedSnapshotByPipelineID(ctx context.Context, sellerAccountID int64, pipelineRunID int64) (Snapshot, error) {
	row, err := r.q.GetPublishedSnapshotBySellerAndPipelineRun(ctx, dbgen.GetPublishedSnapshotBySellerAndPipelineRunParams{
		SellerAccountID: sellerAccountID,
		ID:              pipelineRunID,
	})
	if err != nil {
		return Snapshot{}, err
	}
	return mapPipelineSnapshotRow(row), nil
}

func mapCurrentSnapshotRow(row dbgen.GetCurrentPublishedSnapshotBySellerRow) Snapshot {
	return mapSnapshotFields(
		row.PipelineRunID,
		row.SellerAccountID,
		row.MetricDate.Time.UTC(),
		row.DashboardDataSnapshotID.Int64,
		row.DashboardRunID.Bytes,
		row.ActionsRunID.Int64,
		row.DataPackHash,
		row.GlobalContextHash,
		row.PublishedAt.Time.UTC(),
	)
}

func mapPipelineSnapshotRow(row dbgen.GetPublishedSnapshotBySellerAndPipelineRunRow) Snapshot {
	return mapSnapshotFields(
		row.PipelineRunID,
		row.SellerAccountID,
		row.MetricDate.Time.UTC(),
		row.DashboardDataSnapshotID.Int64,
		row.DashboardRunID.Bytes,
		row.ActionsRunID.Int64,
		row.DataPackHash,
		row.GlobalContextHash,
		row.PublishedAt.Time.UTC(),
	)
}

func mapSnapshotFields(
	pipelineRunID int64,
	sellerAccountID int64,
	metricDate time.Time,
	dashboardDataSnapshotID int64,
	dashboardRunID [16]byte,
	actionsRunID int64,
	dataPackHash string,
	globalContextHash pgtype.Text,
	publishedAt time.Time,
) Snapshot {
	return Snapshot{
		PipelineRunID:           pipelineRunID,
		SellerAccountID:         sellerAccountID,
		MetricDate:              metricDate,
		DashboardDataSnapshotID: dashboardDataSnapshotID,
		DashboardRunID:          uuidFromBytes(dashboardRunID),
		ActionsRunID:            actionsRunID,
		DataPackHash:            strings.TrimSpace(dataPackHash),
		GlobalContextHash:       strings.TrimSpace(textOrEmpty(globalContextHash)),
		PublishedAt:             publishedAt,
	}
}

func uuidFromBytes(b [16]byte) string {
	u, err := uuid.FromBytes(b[:])
	if err != nil {
		return ""
	}
	return u.String()
}

func textOrEmpty(v pgtype.Text) string {
	if !v.Valid {
		return ""
	}
	return v.String
}

func ValidateSnapshot(snapshot Snapshot, sellerAccountID int64) error {
	if sellerAccountID <= 0 {
		return fmt.Errorf("invalid seller_account_id")
	}
	if snapshot.SellerAccountID != sellerAccountID {
		return fmt.Errorf("seller mismatch")
	}
	if snapshot.PipelineRunID <= 0 || snapshot.ActionsRunID <= 0 || strings.TrimSpace(snapshot.DashboardRunID) == "" || snapshot.DashboardDataSnapshotID <= 0 {
		return fmt.Errorf("snapshot is incomplete")
	}
	if strings.TrimSpace(snapshot.DataPackHash) == "" || strings.TrimSpace(snapshot.GlobalContextHash) == "" {
		return fmt.Errorf("snapshot hashes are incomplete")
	}
	return nil
}
