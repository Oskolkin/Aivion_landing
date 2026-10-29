package devseed

import (
	"context"
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	mvpDemoOzonClientID          = "DEMO_CLIENT_ID"
	mvpDemoOzonAPIKey            = "DEMO_API_KEY"
	mvpDemoOzonPerformanceClient = "DEMO_PERF_CLIENT_ID"
	mvpDemoOzonPerformanceSecret = "DEMO_PERF_CLIENT_SECRET"
	mvpDemoOzonPerformanceToken  = "DEMO_PERFORMANCE_TOKEN"
)

// SeedOzonConnectionMock upserts validated Seller + Performance API credentials (connection_ready=true).
func SeedOzonConnectionMock(ctx context.Context, q *dbgen.Queries, sellerAccountID int64, anchorUTC time.Time, encryptionKey string) error {
	if len(encryptionKey) != 32 {
		return fmt.Errorf("encryption key must be 32 bytes")
	}
	codec, err := ozon.NewSecretCodec(encryptionKey)
	if err != nil {
		return fmt.Errorf("secret codec: %w", err)
	}

	clientEnc, err := codec.Encrypt(mvpDemoOzonClientID)
	if err != nil {
		return fmt.Errorf("encrypt client id: %w", err)
	}
	apiEnc, err := codec.Encrypt(mvpDemoOzonAPIKey)
	if err != nil {
		return fmt.Errorf("encrypt api key: %w", err)
	}
	perfIDEnc, err := codec.Encrypt(mvpDemoOzonPerformanceClient)
	if err != nil {
		return fmt.Errorf("encrypt performance client id: %w", err)
	}
	perfSecretEnc, err := codec.Encrypt(mvpDemoOzonPerformanceSecret)
	if err != nil {
		return fmt.Errorf("encrypt performance client secret: %w", err)
	}
	pfmEnc, err := codec.Encrypt(mvpDemoOzonPerformanceToken)
	if err != nil {
		return fmt.Errorf("encrypt performance token: %w", err)
	}

	expires := anchorUTC.Add(24 * time.Hour).UTC()
	if _, err := q.UpsertOzonConnectionValidated(ctx, dbgen.UpsertOzonConnectionValidatedParams{
		SellerAccountID:                  sellerAccountID,
		ClientIDEncrypted:                clientEnc,
		ApiKeyEncrypted:                  apiEnc,
		PerformanceClientIDEncrypted:     pgtype.Text{String: perfIDEnc, Valid: true},
		PerformanceClientSecretEncrypted: pgtype.Text{String: perfSecretEnc, Valid: true},
		PerformanceTokenEncrypted:        pgtype.Text{String: pfmEnc, Valid: true},
		PerformanceTokenExpiresAt:        pgtype.Timestamptz{Time: expires, Valid: true},
	}); err != nil {
		return fmt.Errorf("upsert validated ozon connection: %w", err)
	}
	return nil
}
