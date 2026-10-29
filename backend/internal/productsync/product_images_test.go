package productsync

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
)

func TestProductSyncPrimaryImageSelection(t *testing.T) {
	t.Run("primary_image saved", func(t *testing.T) {
		detail := ozon.ProductInfoListItem{
			PrimaryImage: ozon.FlexibleStringList{"https://cdn/primary.jpg"},
			Images:       ozon.FlexibleStringList{"https://cdn/img.jpg"},
		}
		got := ozon.PickPrimaryImageURL(detail.PrimaryImage, detail.Images)
		if got != "https://cdn/primary.jpg" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("fallback images[0]", func(t *testing.T) {
		detail := ozon.ProductInfoListItem{
			Images: ozon.FlexibleStringList{"https://cdn/first.jpg"},
		}
		got := ozon.PickPrimaryImageURL(detail.PrimaryImage, detail.Images)
		if got != "https://cdn/first.jpg" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("no images null", func(t *testing.T) {
		got := ozon.PickPrimaryImageURL(nil, nil)
		if got != "" {
			t.Fatalf("got %q", got)
		}
	})
}
