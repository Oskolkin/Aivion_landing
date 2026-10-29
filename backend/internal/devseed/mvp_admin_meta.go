package devseed

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const mvpSystemAdminEmail = "dev-seed-mvp@system.local"

// SeedMVPAdminMetadataInTx inserts completed admin_action_logs that are safe before commit (seed audit).
func SeedMVPAdminMetadataInTx(ctx context.Context, q *dbgen.Queries, sellerAccountID int64, adminUserID int64, adminEmail string, anchorUTC time.Time) error {
	email := mvpSystemAdminEmail
	if adminEmail != "" {
		email = adminEmail
	}
	adminUID := pgtype.Int8{}
	if adminUserID > 0 {
		adminUID = pgtype.Int8{Int64: adminUserID, Valid: true}
	}

	req, _ := json.Marshal(map[string]any{
		"source":      "dev-seed-mvp",
		"anchor_date": anchorUTC.Format("2006-01-02"),
	})
	res, _ := json.Marshal(map[string]any{
		"seeded_by":   "dev-seed-mvp",
		"anchor_date": anchorUTC.Format("2006-01-02"),
		"status":      "source_data_written",
	})

	log, err := q.CreateAdminActionLog(ctx, dbgen.CreateAdminActionLogParams{
		AdminUserID:     adminUID,
		AdminEmail:      email,
		SellerAccountID: sellerAccountID,
		ActionType:      "seed_created",
		TargetType:      pgtype.Text{},
		TargetID:        pgtype.Int8{},
		RequestPayload:  req,
		Status:          pgtype.Text{String: "running", Valid: true},
	})
	if err != nil {
		return fmt.Errorf("create admin seed_created log: %w", err)
	}
	if _, err := q.CompleteAdminActionLog(ctx, dbgen.CompleteAdminActionLogParams{
		ID:            log.ID,
		ResultPayload: res,
	}); err != nil {
		return fmt.Errorf("complete admin seed_created log: %w", err)
	}
	return nil
}

// SeedMVPAdminPostMetricsLog records rerun_metrics after analytics rebuild (runs post-commit).
func SeedMVPAdminPostMetricsLog(ctx context.Context, pool *pgxpool.Pool, sellerAccountID int64, adminUserID int64, adminEmail string) error {
	q := dbgen.New(pool)
	email := mvpSystemAdminEmail
	if adminEmail != "" {
		email = adminEmail
	}
	adminUID := pgtype.Int8{}
	if adminUserID > 0 {
		adminUID = pgtype.Int8{Int64: adminUserID, Valid: true}
	}

	req, _ := json.Marshal(map[string]any{
		"source": "dev-seed-mvp",
		"scope":  "daily_account_metrics+daily_sku_metrics",
	})
	res, _ := json.Marshal(map[string]any{
		"seeded_by": "dev-seed-mvp",
		"outcome":   "metrics_rebuilt",
		"at":        time.Now().UTC().Format(time.RFC3339),
	})

	log, err := q.CreateAdminActionLog(ctx, dbgen.CreateAdminActionLogParams{
		AdminUserID:     adminUID,
		AdminEmail:      email,
		SellerAccountID: sellerAccountID,
		ActionType:      "rerun_metrics",
		TargetType:      pgtype.Text{},
		TargetID:        pgtype.Int8{},
		RequestPayload:  req,
		Status:          pgtype.Text{String: "running", Valid: true},
	})
	if err != nil {
		return fmt.Errorf("create admin rerun_metrics log: %w", err)
	}
	if _, err := q.CompleteAdminActionLog(ctx, dbgen.CompleteAdminActionLogParams{
		ID:            log.ID,
		ResultPayload: res,
	}); err != nil {
		return fmt.Errorf("complete admin rerun_metrics log: %w", err)
	}
	return nil
}
