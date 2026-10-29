package catalog

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type TaxonomyAPI interface {
	GetDescriptionCategoryTree(context.Context, int64) (*ozon.TypedResponse[[]ozon.DescriptionCategoryTreeJSONNode], error)
}

type TaxonomyResult struct {
	Hash         string `json:"hash"`
	Categories   int    `json:"categories"`
	ProductTypes int    `json:"product_types"`
}

type SupersededTaxonomyRunError struct {
	DomainRunID  int64
	SupersededBy int64
}

func (e *SupersededTaxonomyRunError) Error() string {
	return fmt.Sprintf("taxonomy domain run %d was superseded by newer global taxonomy run %d", e.DomainRunID, e.SupersededBy)
}

func (r *Repository) PersistTaxonomy(ctx context.Context, sellerID int64, domainRunID int64, nodes []ozon.DescriptionCategoryTreeJSONNode, at time.Time) (TaxonomyResult, error) {
	categories, productTypes, err := NormalizeTaxonomy(nodes)
	if err != nil {
		return TaxonomyResult{}, err
	}
	h := normalizedHash(struct {
		Categories   []CategoryNode    `json:"categories"`
		ProductTypes []ProductTypeNode `json:"product_types"`
	}{categories, productTypes})
	result := TaxonomyResult{Hash: h, Categories: len(categories), ProductTypes: len(productTypes)}
	persist := func(q *dbgen.Queries) error {
		sync, err := q.CreateTaxonomySyncRun(ctx, dbgen.CreateTaxonomySyncRunParams{CollectionDomainRunID: domainRunID, TaxonomyHash: h, Status: "COMPLETED", ObservedAt: ts(at)})
		if err != nil {
			return err
		}
		internalCategories := make(map[int64]int64, len(categories))
		observedCategoryIDs := make(map[int64]struct{}, len(categories))
		for _, category := range categories {
			row, err := q.UpsertProviderCategory(ctx, dbgen.UpsertProviderCategoryParams{ProviderCategoryID: category.ExternalID, FirstObservedAt: ts(at)})
			if err != nil {
				return err
			}
			internalCategories[category.ExternalID] = row.ID
			observedCategoryIDs[row.ID] = struct{}{}
		}
		for _, category := range categories {
			var parent pgtype.Int8
			if id := internalCategories[category.ParentExternalID]; id > 0 {
				parent = pgtype.Int8{Int64: id, Valid: true}
			}
			state := "ACTIVE"
			if category.Disabled {
				state = "DISABLED"
			}
			stateHash := categoryStateHash(parent, category.Name, category.Disabled, category.IsLeaf, state)
			latest, latestErr := q.GetLatestProviderCategoryVersion(ctx, internalCategories[category.ExternalID])
			if latestErr == nil && latest.NormalizedHash == stateHash {
				continue
			}
			if latestErr != nil && !errors.Is(latestErr, pgx.ErrNoRows) {
				return latestErr
			}
			if _, err := q.CreateProviderCategoryVersion(ctx, dbgen.CreateProviderCategoryVersionParams{ProviderCategoryID: internalCategories[category.ExternalID], ParentProviderCategoryID: parent, Name: category.Name, Disabled: category.Disabled, IsLeaf: category.IsLeaf, TaxonomyState: state, TaxonomySyncRunID: sync.ID, ObservedAt: ts(at), NormalizedHash: stateHash}); err != nil {
				return err
			}
		}
		allCategories, err := q.ListProviderCategories(ctx)
		if err != nil {
			return err
		}
		for _, category := range allCategories {
			if _, observed := observedCategoryIDs[category.ID]; observed {
				continue
			}
			latest, err := q.GetLatestProviderCategoryVersion(ctx, category.ID)
			if err != nil {
				return err
			}
			if latest.TaxonomyState == "UNOBSERVED" {
				continue
			}
			stateHash := categoryStateHash(latest.ParentProviderCategoryID, latest.Name, latest.Disabled, latest.IsLeaf, "UNOBSERVED")
			if _, err := q.CreateProviderCategoryVersion(ctx, dbgen.CreateProviderCategoryVersionParams{ProviderCategoryID: category.ID, ParentProviderCategoryID: latest.ParentProviderCategoryID, Name: latest.Name, Disabled: latest.Disabled, IsLeaf: latest.IsLeaf, TaxonomyState: "UNOBSERVED", TaxonomySyncRunID: sync.ID, ObservedAt: ts(at), NormalizedHash: stateHash}); err != nil {
				return err
			}
		}
		observedProductTypeIDs := make(map[int64]struct{}, len(productTypes))
		for _, productType := range productTypes {
			categoryID := internalCategories[productType.CategoryExternalID]
			if categoryID == 0 {
				return fmt.Errorf("product type %d references missing category %d", productType.ExternalID, productType.CategoryExternalID)
			}
			typeRow, err := q.UpsertProviderProductType(ctx, dbgen.UpsertProviderProductTypeParams{ProviderCategoryID: categoryID, ProviderTypeID: productType.ExternalID, FirstObservedAt: ts(at)})
			if err != nil {
				return err
			}
			observedProductTypeIDs[typeRow.ID] = struct{}{}
			state := "ACTIVE"
			if productType.Disabled {
				state = "DISABLED"
			}
			stateHash := productTypeStateHash(categoryID, productType.Name, productType.Disabled, state)
			latest, latestErr := q.GetLatestProviderProductTypeVersion(ctx, typeRow.ID)
			if latestErr == nil && latest.NormalizedHash == stateHash {
				continue
			}
			if latestErr != nil && !errors.Is(latestErr, pgx.ErrNoRows) {
				return latestErr
			}
			if _, err := q.CreateProviderProductTypeVersion(ctx, dbgen.CreateProviderProductTypeVersionParams{ProviderProductTypeID: typeRow.ID, ProviderCategoryID: categoryID, Name: productType.Name, Disabled: productType.Disabled, TaxonomyState: state, TaxonomySyncRunID: sync.ID, ObservedAt: ts(at), NormalizedHash: stateHash}); err != nil {
				return err
			}
		}
		allProductTypes, err := q.ListProviderProductTypes(ctx)
		if err != nil {
			return err
		}
		for _, productType := range allProductTypes {
			if _, observed := observedProductTypeIDs[productType.ID]; observed {
				continue
			}
			latest, err := q.GetLatestProviderProductTypeVersion(ctx, productType.ID)
			if err != nil {
				return err
			}
			if latest.TaxonomyState == "UNOBSERVED" {
				continue
			}
			stateHash := productTypeStateHash(productType.ProviderCategoryID, latest.Name, latest.Disabled, "UNOBSERVED")
			if _, err := q.CreateProviderProductTypeVersion(ctx, dbgen.CreateProviderProductTypeVersionParams{ProviderProductTypeID: productType.ID, ProviderCategoryID: productType.ProviderCategoryID, Name: latest.Name, Disabled: latest.Disabled, TaxonomyState: "UNOBSERVED", TaxonomySyncRunID: sync.ID, ObservedAt: ts(at), NormalizedHash: stateHash}); err != nil {
				return err
			}
		}
		return nil
	}
	if r.pool == nil {
		return TaxonomyResult{}, errors.New("taxonomy persistence requires transactional repository")
	}
	var supersededBy int64
	err = pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if _, lockErr := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "taxonomy:OZON:RU"); lockErr != nil {
			return lockErr
		}
		q := r.q.WithTx(tx)
		domainRun, domainErr := q.GetCollectionDomainRunForSeller(ctx, dbgen.GetCollectionDomainRunForSellerParams{ID: domainRunID, SellerAccountID: sellerID})
		if domainErr != nil {
			return domainErr
		}
		if domainRun.Domain != string(collection.DomainTaxonomy) {
			return fmt.Errorf("domain run %d is %s, expected TAXONOMY", domainRunID, domainRun.Domain)
		}
		head, headErr := q.GetProviderTaxonomyHead(ctx, dbgen.GetProviderTaxonomyHeadParams{Provider: "OZON", Market: "RU"})
		if headErr != nil && !errors.Is(headErr, pgx.ErrNoRows) {
			return headErr
		}
		if headErr == nil && head.CollectionDomainRunID > domainRunID {
			supersededBy = head.CollectionDomainRunID
			message := fmt.Sprintf("newer global taxonomy domain run %d already finalized", supersededBy)
			_, cancelErr := q.CancelSupersededCollectionDomainRun(ctx, dbgen.CancelSupersededCollectionDomainRunParams{ID: domainRunID, SellerAccountID: sellerID, ErrorMessage: pgtype.Text{String: message, Valid: true}})
			return cancelErr
		}
		if headErr == nil && head.CollectionDomainRunID == domainRunID {
			return nil
		}
		if domainRun.Status != string(collection.DomainRunStatusRunning) {
			return collection.ValidateDomainTransition(collection.DomainRunStatus(domainRun.Status), collection.DomainRunStatusCompleted)
		}
		// A writer that acquires the global lock later must also sort later in
		// temporal version reads, regardless of its pre-lock worker timestamp.
		at = time.Now().UTC()
		if persistErr := persist(q); persistErr != nil {
			return persistErr
		}
		_, headUpdateErr := q.UpsertProviderTaxonomyHead(ctx, dbgen.UpsertProviderTaxonomyHeadParams{Provider: "OZON", Market: "RU", CollectionDomainRunID: domainRunID, FinalizedAt: ts(at)})
		return headUpdateErr
	})
	if err == nil && supersededBy > 0 {
		err = &SupersededTaxonomyRunError{DomainRunID: domainRunID, SupersededBy: supersededBy}
	}
	return result, err
}

func categoryStateHash(parent pgtype.Int8, name string, disabled, isLeaf bool, state string) string {
	var parentID *int64
	if parent.Valid {
		parentID = &parent.Int64
	}
	return normalizedHash(struct {
		ParentID *int64 `json:"parent_id"`
		Name     string `json:"name"`
		Disabled bool   `json:"disabled"`
		IsLeaf   bool   `json:"is_leaf"`
		State    string `json:"state"`
	}{parentID, name, disabled, isLeaf, state})
}

func productTypeStateHash(categoryID int64, name string, disabled bool, state string) string {
	return normalizedHash(struct {
		CategoryID int64  `json:"category_id"`
		Name       string `json:"name"`
		Disabled   bool   `json:"disabled"`
		State      string `json:"state"`
	}{categoryID, name, disabled, state})
}

func ts(at time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: at, Valid: true} }
func CollectTaxonomy(ctx context.Context, api TaxonomyAPI, service *collection.Service, store *collection.SourcePayloadStore, repo *Repository, sellerID, domainRunID int64) (TaxonomyResult, error) {
	resp, err := api.GetDescriptionCategoryTree(ctx, sellerID)
	if err != nil {
		return TaxonomyResult{}, err
	}
	page := int64(1)
	status := int32(resp.Meta.StatusCode)
	if _, err = service.StoreAndRegisterPayload(ctx, store, collection.RegisterPayloadInput{SellerAccountID: sellerID, DomainRunID: domainRunID, Provider: collection.ProviderOzon, EndpointKey: "ozon.description_category.tree", RequestIdentity: map[string]any{}, Body: resp.Raw, ContentType: "application/json", PageSeq: &page, HTTPStatus: &status}); err != nil {
		return TaxonomyResult{}, err
	}
	return repo.PersistTaxonomy(ctx, sellerID, domainRunID, resp.Data, time.Now().UTC())
}
