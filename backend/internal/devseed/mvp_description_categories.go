package devseed

import (
	"context"
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

var mvpCategoryDisplayNames = map[mvpCommerceCategory]string{
	catKitchenStorage:   "Хранение на кухне",
	catHomeTextile:      "Текстиль для дома",
	catCleaning:         "Уборка",
	catBathroom:         "Ванная комната",
	catLightDecor:       "Освещение и декор",
	catHomeOrganization: "Организация пространства",
}

// SeedMVPDescriptionCategories upserts global Ozon description category names used by MVP commerce seed.
// IDs must match categoryDescriptionID() for the same seller so AI summaries and dashboard show real names.
func SeedMVPDescriptionCategories(ctx context.Context, q *dbgen.Queries, sellerAccountID int64) error {
	cats := []mvpCommerceCategory{
		catKitchenStorage,
		catHomeTextile,
		catCleaning,
		catBathroom,
		catLightDecor,
		catHomeOrganization,
	}
	for _, cat := range cats {
		id := categoryDescriptionID(cat, sellerAccountID)
		name := mvpCategoryDisplayNames[cat]
		if name == "" {
			name = string(cat)
		}
		path := fmt.Sprintf("/mvp/%s", cat)
		if err := q.UpsertDescriptionCategory(ctx, dbgen.UpsertDescriptionCategoryParams{
			DescriptionCategoryID: id,
			ParentID:              pgtype.Int8{},
			CategoryName:          name,
			Path:                  pgtype.Text{String: path, Valid: true},
		}); err != nil {
			return fmt.Errorf("upsert description category %d: %w", id, err)
		}
	}
	// Ozon enrichment may reference additional description category IDs outside the MVP hash range.
	orphanCategories := map[int64]string{
		17990001: "Товары для ванной",
	}
	for id, name := range orphanCategories {
		if err := q.UpsertDescriptionCategory(ctx, dbgen.UpsertDescriptionCategoryParams{
			DescriptionCategoryID: id,
			ParentID:              pgtype.Int8{},
			CategoryName:          name,
			Path:                  pgtype.Text{String: fmt.Sprintf("/mvp/orphan/%d", id), Valid: true},
		}); err != nil {
			return fmt.Errorf("upsert orphan description category %d: %w", id, err)
		}
	}
	return nil
}
