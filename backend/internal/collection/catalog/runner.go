package catalog

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
)

type ProductAPI interface {
	ListProducts(context.Context, int64, ozon.ListProductsRequest) (*ozon.TypedResponse[ozon.ListProductsResult], error)
	ListProductsInfo(context.Context, int64, ozon.ProductInfoListRequest) (*ozon.TypedResponse[ozon.ProductInfoListResult], error)
}

type PayloadRegistrar interface {
	StoreAndRegisterPayload(context.Context, *collection.SourcePayloadStore, collection.RegisterPayloadInput) (collection.SourcePayload, error)
}

type Runner struct {
	API           ProductAPI
	Collection    PayloadRegistrar
	Payloads      *collection.SourcePayloadStore
	TaxonomyReady bool
	Now           func() time.Time
}

type Result struct {
	RecordsReceived, RecordsWritten, Pages int
	AssignmentsWritten, MarkedUnobserved   int
	AllPages, ArchivedPages                int
	AllItems, ArchivedItems                int
	UnionSKUs, CrossStreamDuplicates       int
	ObservedSKUs                           []int64
	observations                           []SKUObservation
	taxonomyReady                          bool
}

type listObservation struct {
	item       ozon.ProductItem
	visibility string
}

// RunCatalog builds a complete in-memory ALL + ARCHIVED provider universe.
// Canonical persistence is deliberately deferred to FinalizeCompleteRun so
// incomplete pagination, enrichment or normalization cannot change current
// canonical state or reconcile UNOBSERVED.
func (r *Runner) RunCatalog(ctx context.Context, sellerID, domainRunID int64) (Result, error) {
	if r.Now == nil {
		r.Now = func() time.Time { return time.Now().UTC() }
	}
	var out Result
	out.taxonomyReady = r.TaxonomyReady
	all, pages, err := r.enumerateStream(ctx, sellerID, domainRunID, "ALL")
	out.AllPages, out.AllItems = pages, len(all)
	out.Pages += pages
	if err != nil {
		return out, err
	}
	archived, pages, err := r.enumerateStream(ctx, sellerID, domainRunID, "ARCHIVED")
	out.ArchivedPages, out.ArchivedItems = pages, len(archived)
	out.Pages += pages
	if err != nil {
		return out, err
	}

	items := append(all, archived...)
	bySKU := make(map[int64][]listObservation, len(items))
	for _, observation := range items {
		bySKU[observation.item.SKU] = append(bySKU[observation.item.SKU], observation)
	}
	for sku, providerObservations := range bySKU {
		if err := validateSKUProductIdentity(sku, providerObservations); err != nil {
			return out, err
		}
	}
	productIDs := make([]int64, 0, len(items))
	seenProductIDs := make(map[int64]struct{}, len(items))
	for _, observation := range items {
		if observation.item.ProductID <= 0 {
			return out, fmt.Errorf("catalog identity: missing provider product_id for sku=%d", observation.item.SKU)
		}
		if observation.item.SKU <= 0 {
			return out, fmt.Errorf("catalog identity: missing ozon sku for product_id=%d", observation.item.ProductID)
		}
		if _, exists := seenProductIDs[observation.item.ProductID]; !exists {
			seenProductIDs[observation.item.ProductID] = struct{}{}
			productIDs = append(productIDs, observation.item.ProductID)
		}
	}
	sort.Slice(productIDs, func(i, j int) bool { return productIDs[i] < productIDs[j] })
	details, err := r.enrichProducts(ctx, sellerID, domainRunID, productIDs)
	if err != nil {
		return out, err
	}

	skus := make([]int64, 0, len(bySKU))
	for sku := range bySKU {
		skus = append(skus, sku)
	}
	sort.Slice(skus, func(i, j int) bool { return skus[i] < skus[j] })

	for _, sku := range skus {
		providerObservations := bySKU[sku]
		streams := map[string]struct{}{}
		candidates := make([]SKUObservation, 0, len(providerObservations))
		for _, providerObservation := range providerObservations {
			streams[providerObservation.visibility] = struct{}{}
			detail, found := details[providerObservation.item.ProductID]
			if !found {
				return out, fmt.Errorf("required product info missing for product_id=%d", providerObservation.item.ProductID)
			}
			item := providerObservation.item
			if providerObservation.visibility == "ARCHIVED" {
				item.Archived = true
			}
			normalized, normalizeErr := Normalize(item, detail)
			if normalizeErr != nil {
				return out, normalizeErr
			}
			candidates = append(candidates, normalized)
		}
		if len(streams) > 1 {
			out.CrossStreamDuplicates++
		}
		merged := mergeSKUObservations(candidates)
		out.observations = append(out.observations, merged)
		out.ObservedSKUs = append(out.ObservedSKUs, sku)
	}
	out.UnionSKUs = len(out.ObservedSKUs)
	out.RecordsReceived = out.UnionSKUs
	return out, nil
}

func (r *Runner) enumerateStream(ctx context.Context, sellerID, domainRunID int64, visibility string) ([]listObservation, int, error) {
	lastID := ""
	seenCursors := map[string]struct{}{}
	var observations []listObservation
	for page := 1; ; page++ {
		request := NewFullSnapshotRequest(visibility)
		request.LastID = lastID
		response, err := r.API.ListProducts(ctx, sellerID, request)
		if err != nil {
			return observations, page - 1, fmt.Errorf("product list %s page %d: %w", visibility, page, err)
		}
		if r.Collection != nil && r.Payloads != nil {
			pageSeq := int64(page)
			status := int32(response.Meta.StatusCode)
			_, err = r.Collection.StoreAndRegisterPayload(ctx, r.Payloads, collection.RegisterPayloadInput{
				SellerAccountID: sellerID,
				DomainRunID:     domainRunID,
				Provider:        collection.ProviderOzon,
				EndpointKey:     "ozon.product.list",
				RequestIdentity: map[string]any{"visibility": visibility, "last_id": lastID, "limit": ProductListPageSize},
				Metadata:        map[string]any{"visibility_stream": visibility},
				Body:            response.Raw,
				ContentType:     "application/json",
				PageSeq:         &pageSeq,
				HTTPStatus:      &status,
			})
			if err != nil {
				return observations, page - 1, err
			}
		}
		for _, item := range response.Data.Items {
			observations = append(observations, listObservation{item: item, visibility: visibility})
		}
		if len(response.Data.Items) == 0 && response.Data.LastID != "" {
			return observations, page, fmt.Errorf("catalog %s pagination returned empty page with non-empty cursor", visibility)
		}
		if response.Data.LastID == "" {
			return observations, page, nil
		}
		if response.Data.LastID == lastID {
			return observations, page, fmt.Errorf("catalog %s pagination cursor did not advance at page %d", visibility, page)
		}
		if _, duplicate := seenCursors[response.Data.LastID]; duplicate {
			return observations, page, fmt.Errorf("catalog %s pagination cursor repeated at page %d", visibility, page)
		}
		seenCursors[response.Data.LastID] = struct{}{}
		lastID = response.Data.LastID
	}
}

func (r *Runner) enrichProducts(ctx context.Context, sellerID, domainRunID int64, productIDs []int64) (map[int64]ozon.ProductInfoListItem, error) {
	details := make(map[int64]ozon.ProductInfoListItem, len(productIDs))
	for start, batch := 0, 1; start < len(productIDs); start, batch = start+ProductInfoBatchSize, batch+1 {
		end := start + ProductInfoBatchSize
		if end > len(productIDs) {
			end = len(productIDs)
		}
		ids := append([]int64(nil), productIDs[start:end]...)
		response, err := r.API.ListProductsInfo(ctx, sellerID, ozon.ProductInfoListRequest{ProductID: ids})
		if err != nil {
			return nil, fmt.Errorf("product info batch %d: %w", batch, err)
		}
		if r.Collection != nil && r.Payloads != nil {
			pageSeq := int64(batch)
			status := int32(response.Meta.StatusCode)
			_, err = r.Collection.StoreAndRegisterPayload(ctx, r.Payloads, collection.RegisterPayloadInput{
				SellerAccountID: sellerID,
				DomainRunID:     domainRunID,
				Provider:        collection.ProviderOzon,
				EndpointKey:     "ozon.product.info.list",
				RequestIdentity: map[string]any{"product_id": ids},
				Metadata:        map[string]any{"visibility_stream": "UNION"},
				Body:            response.Raw,
				ContentType:     "application/json",
				PageSeq:         &pageSeq,
				HTTPStatus:      &status,
			})
			if err != nil {
				return nil, err
			}
		}
		for _, detail := range response.Data.Items {
			details[detail.ID] = detail
		}
	}
	return details, nil
}

type SKUIdentityConflictError struct {
	OzonSKU    int64
	ProductIDs []int64
}

func (e *SKUIdentityConflictError) Error() string {
	return fmt.Sprintf("SKU_IDENTITY_CONFLICT: ozon_sku=%d has simultaneous product_ids=%v", e.OzonSKU, e.ProductIDs)
}

func validateSKUProductIdentity(sku int64, observations []listObservation) error {
	seen := map[int64]struct{}{}
	for _, observation := range observations {
		seen[observation.item.ProductID] = struct{}{}
	}
	if len(seen) <= 1 {
		return nil
	}
	productIDs := make([]int64, 0, len(seen))
	for productID := range seen {
		productIDs = append(productIDs, productID)
	}
	sort.Slice(productIDs, func(i, j int) bool { return productIDs[i] < productIDs[j] })
	return &SKUIdentityConflictError{OzonSKU: sku, ProductIDs: productIDs}
}

func mergeSKUObservations(candidates []SKUObservation) SKUObservation {
	// validateSKUProductIdentity has already established one ProductID. This is
	// state reconciliation for duplicate streams, never identity selection.
	merged := candidates[0]
	for _, candidate := range candidates[1:] {
		if candidate.State != merged.State {
			merged.State = "UNKNOWN"
			merged.Archived = false
		}
	}
	return merged
}
