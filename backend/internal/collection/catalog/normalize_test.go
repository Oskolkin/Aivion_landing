package catalog

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
)

func TestIdentityUsesOzonSKUNotOfferOrProduct(t *testing.T) {
	list := ozon.ProductItem{ProductID: 10, SKU: 77, OfferID: "old", Name: "n"}
	first, err := Normalize(list, ozon.ProductInfoListItem{ID: 10, SKU: 77, OfferID: "old"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Normalize(list, ozon.ProductInfoListItem{ID: 99, SKU: 77, OfferID: "new"})
	if err != nil {
		t.Fatal(err)
	}
	if first.OzonSKU != second.OzonSKU || first.ProductID == second.ProductID || first.OfferID == second.OfferID {
		t.Fatal("identity or mutable metadata handling is incorrect")
	}
}
func TestCrossDomainFieldsAreIgnored(t *testing.T) {
	v, err := Normalize(ozon.ProductItem{ProductID: 1, SKU: 2}, ozon.ProductInfoListItem{ID: 1, SKU: 2, Price: "9", OldPrice: "10", MinPrice: "8", Commissions: []ozon.ProductInfoCommission{{Percent: 5}}, Stocks: ozon.ProductInfoStocks{HasStock: true}})
	if err != nil {
		t.Fatal(err)
	}
	if v.OzonSKU != 2 || v.State != "ACTIVE" {
		t.Fatal(v)
	}
}

func TestProviderArchivedStateIsPreserved(t *testing.T) {
	observation, err := Normalize(ozon.ProductItem{ProductID: 1, SKU: 2, IsArchived: true}, ozon.ProductInfoListItem{ID: 1, SKU: 2})
	if err != nil {
		t.Fatal(err)
	}
	if !observation.Archived || observation.State != "ARCHIVED" {
		t.Fatalf("observation=%+v", observation)
	}
}
func TestNewFullSnapshotAlwaysStartsEmpty(t *testing.T) {
	if got := NewFullSnapshotRequest("ALL").LastID; got != "" {
		t.Fatalf("last_id=%q", got)
	}
}

func TestAmbiguousProviderSKUIsRejected(t *testing.T) {
	_, err := Normalize(ozon.ProductItem{ProductID: 1, SKU: 77}, ozon.ProductInfoListItem{ID: 1, SKU: 88})
	if !errors.Is(err, ErrAmbiguousSKUIdentity) {
		t.Fatalf("err=%v", err)
	}
	observation, err := Normalize(ozon.ProductItem{ProductID: 1, SKU: 77}, ozon.ProductInfoListItem{ID: 1, SKU: 77, Sources: json.RawMessage(`[{"sku":99,"source":"fbo"}]`)})
	if err != nil || observation.OzonSKU != 77 {
		t.Fatalf("nested source SKU affected identity: observation=%+v err=%v", observation, err)
	}
}
