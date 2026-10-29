package chat

import (
	"context"
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

// SQLCEntityResolver validates seller entities using lightweight EXISTS queries.
type SQLCEntityResolver struct {
	q *dbgen.Queries
}

func NewSQLCEntityResolver(q *dbgen.Queries) *SQLCEntityResolver {
	return &SQLCEntityResolver{q: q}
}

func (r *SQLCEntityResolver) ProductExistsBySKU(ctx context.Context, sellerAccountID int64, sku int64) (bool, error) {
	exists, err := r.q.ProductExistsBySellerAndSKU(ctx, dbgen.ProductExistsBySellerAndSKUParams{
		SellerAccountID: sellerAccountID,
		Sku:             pgtype.Int8{Int64: sku, Valid: true},
	})
	if err != nil {
		return false, fmt.Errorf("product exists by sku: %w", err)
	}
	return exists, nil
}

func (r *SQLCEntityResolver) ProductExistsByOfferID(ctx context.Context, sellerAccountID int64, offerID string) (bool, error) {
	exists, err := r.q.ProductExistsBySellerAndOfferID(ctx, dbgen.ProductExistsBySellerAndOfferIDParams{
		SellerAccountID: sellerAccountID,
		OfferID:         pgtype.Text{String: offerID, Valid: true},
	})
	if err != nil {
		return false, fmt.Errorf("product exists by offer_id: %w", err)
	}
	return exists, nil
}

func (r *SQLCEntityResolver) AdCampaignExistsByExternalID(ctx context.Context, sellerAccountID int64, campaignExternalID int64) (bool, error) {
	exists, err := r.q.AdCampaignExistsBySellerAndExternalID(ctx, dbgen.AdCampaignExistsBySellerAndExternalIDParams{
		SellerAccountID:    sellerAccountID,
		CampaignExternalID: campaignExternalID,
	})
	if err != nil {
		return false, fmt.Errorf("ad campaign exists: %w", err)
	}
	return exists, nil
}
