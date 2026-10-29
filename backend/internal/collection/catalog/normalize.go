package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
)

var ErrAmbiguousSKUIdentity = errors.New("ambiguous provider SKU identity")

const ProductListPageSize = 1000
const ProductInfoBatchSize = 1000

type SKUObservation struct {
	OzonSKU, ProductID, CategoryID, ProductTypeID int64
	OfferID, Name, ProviderStatus                 string
	State                                         string
	Archived                                      bool
	PrimaryImage                                  string
	ImageURLs                                     []string
}

// Normalize only admits Catalog metadata. Price, stock, commissions and other
// cross-domain fields present in ProductInfoListItem are intentionally ignored.
func Normalize(list ozon.ProductItem, detail ozon.ProductInfoListItem) (SKUObservation, error) {
	if list.SKU > 0 && detail.SKU > 0 && list.SKU != detail.SKU {
		return SKUObservation{}, fmt.Errorf("%w: product list sku=%d product info sku=%d", ErrAmbiguousSKUIdentity, list.SKU, detail.SKU)
	}
	sku := list.SKU
	if sku == 0 {
		sku = detail.SKU
	}
	if sku <= 0 {
		return SKUObservation{}, fmt.Errorf("catalog identity: missing ozon sku")
	}
	// detail.sources is preserved in raw provenance only. Its nested SKU fields
	// do not participate in canonical identity until provider semantics are
	// confirmed by a separate non-empty seller acceptance.
	productID := list.ProductID
	if detail.ID != 0 {
		productID = detail.ID
	}
	offer := list.OfferID
	if detail.OfferID != "" {
		offer = detail.OfferID
	}
	name := list.Name
	if detail.Name != "" {
		name = detail.Name
	}
	archived := list.IsArchived || list.Archived || detail.IsArchived || detail.Archived
	state := "ACTIVE"
	if archived {
		state = "ARCHIVED"
	}
	status := list.Status
	if detail.Statuses.Status != "" {
		status = detail.Statuses.Status
	}
	if status == "" {
		status = list.State
	}
	images := append([]string(nil), []string(detail.Images)...)
	primary := ""
	if len(detail.PrimaryImage) > 0 {
		primary = detail.PrimaryImage[0]
	} else if len(images) > 0 {
		primary = images[0]
	}
	return SKUObservation{OzonSKU: sku, ProductID: productID, CategoryID: detail.DescriptionCategoryID, ProductTypeID: detail.TypeID, OfferID: offer, Name: name, ProviderStatus: status, State: state, Archived: archived, PrimaryImage: primary, ImageURLs: images}, nil
}

func MetadataHash(v SKUObservation) string {
	b, _ := json.Marshal(struct {
		ProductID                            int64 `json:"product_id"`
		OfferID, Name, ProviderStatus, State string
	}{v.ProductID, v.OfferID, v.Name, v.ProviderStatus, v.State})
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}
func ContentHash(v SKUObservation) string {
	return normalizedHash(struct {
		Primary string   `json:"primary"`
		Images  []string `json:"images"`
	}{v.PrimaryImage, v.ImageURLs})
}
func TaxonomyHash(nodes []ozon.DescriptionCategoryTreeJSONNode) string {
	categories, types, err := NormalizeTaxonomy(nodes)
	if err != nil {
		return ""
	}
	return normalizedHash(struct {
		Categories   []CategoryNode    `json:"categories"`
		ProductTypes []ProductTypeNode `json:"product_types"`
	}{categories, types})
}
func NewFullSnapshotRequest(visibility string) ozon.ListProductsRequest {
	return ozon.ListProductsRequest{Filter: ozon.ProductListFilter{Visibility: visibility}, Limit: ProductListPageSize, LastID: ""}
}
func FingerprintForPage(visibility, lastID string) (string, error) {
	return collection.RequestFingerprint(collection.ProviderOzon, "ozon.product.list", map[string]any{"visibility": visibility, "last_id": lastID, "limit": ProductListPageSize})
}
