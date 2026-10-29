package devseed

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

const mvpProductDemoImageSlots = 20

// mvpProductDemoImages returns a stable primary URL and JSON gallery for UI testing (no downloads).
// Paths are served from the Next.js public folder (/demo/products/*) so SKU cards load without external CDNs.
// Index-based slots keep top visible SKUs unique; leftover products reuse the pool without colliding in the same view.
func mvpProductDemoImages(p mvpProduct, index int) (primary string, galleryJSON []byte) {
	slot := (index % mvpProductDemoImageSlots) + 1
	primary = fmt.Sprintf("/demo/products/product-demo-%d.jpg", slot)
	altSlot := ((index + 7) % mvpProductDemoImageSlots) + 1
	alt := fmt.Sprintf("/demo/products/product-demo-%d.jpg", altSlot)
	gallery := []map[string]any{
		{"url": primary, "is_primary": true, "source": "dev-seed-mvp"},
		{"url": alt, "is_primary": false, "source": "dev-seed-mvp"},
	}
	galleryJSON, _ = json.Marshal(gallery)
	return primary, galleryJSON
}

// PatchLegacyPicsumProductImages rewrites old external placeholder URLs to frontend-served demo assets.
func PatchLegacyPicsumProductImages(ctx context.Context, pool *pgxpool.Pool, sellerAccountID int64) error {
	_, err := pool.Exec(ctx, `
		WITH ranked AS (
			SELECT id,
			       ROW_NUMBER() OVER (ORDER BY ozon_product_id) AS rn
			FROM products
			WHERE seller_account_id = $1
		)
		UPDATE products p
		SET primary_image_url = '/demo/products/product-demo-' ||
			(((r.rn - 1) % $2) + 1)::text || '.jpg',
		    updated_at = NOW()
		FROM ranked r
		WHERE p.id = r.id
		  AND (
			p.primary_image_url LIKE 'https://picsum.photos/%'
			OR p.primary_image_url LIKE '/demo/products/product-demo-%'
		)
	`, sellerAccountID, mvpProductDemoImageSlots)
	return err
}
