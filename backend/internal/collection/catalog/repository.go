package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	q    *dbgen.Queries
	pool *pgxpool.Pool
}

func NewRepository(q *dbgen.Queries) *Repository { return &Repository{q: q} }
func NewRepositoryWithPool(pool *pgxpool.Pool) *Repository {
	return &Repository{q: dbgen.New(pool), pool: pool}
}

func (r *Repository) PersistSKU(ctx context.Context, sellerID, domainRunID int64, observation SKUObservation, at time.Time) (int64, bool, error) {
	if r.pool == nil {
		return persistSKU(ctx, r.q, sellerID, domainRunID, observation, at)
	}
	var id int64
	var changed bool
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		var err error
		id, changed, err = persistSKU(ctx, r.q.WithTx(tx), sellerID, domainRunID, observation, at)
		return err
	})
	return id, changed, err
}

func persistSKU(ctx context.Context, q *dbgen.Queries, sellerID, domainRunID int64, observation SKUObservation, at time.Time) (int64, bool, error) {
	row, err := q.UpsertCanonicalSKU(ctx, dbgen.UpsertCanonicalSKUParams{SellerAccountID: sellerID, OzonSku: observation.OzonSKU, FirstObservedAt: ts(at), CatalogState: observation.State})
	if err != nil {
		return 0, false, err
	}
	metadataHash := MetadataHash(observation)
	latest, err := q.GetLatestCanonicalSKUVersion(ctx, row.ID)
	metadataChanged := !(err == nil && latest.NormalizedHash == metadataHash)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return 0, false, err
	}
	if metadataChanged {
		if _, err = q.InsertCanonicalSKUVersion(ctx, dbgen.InsertCanonicalSKUVersionParams{CanonicalSkuID: row.ID, ProviderProductID: nullInt(observation.ProductID), OfferID: nullText(observation.OfferID), Name: nullText(observation.Name), ProviderStatus: nullText(observation.ProviderStatus), CatalogState: observation.State, NormalizedHash: metadataHash, ObservedAt: ts(at), CollectionDomainRunID: domainRunID}); err != nil {
			return 0, false, err
		}
	}
	contentHash := ContentHash(observation)
	contentChanged := true
	content, contentErr := q.GetLatestCanonicalSKUContentSnapshot(ctx, row.ID)
	if contentErr == nil && content.NormalizedHash == contentHash {
		contentChanged = false
	} else if contentErr != nil && !errors.Is(contentErr, pgx.ErrNoRows) {
		return 0, false, contentErr
	}
	if contentChanged {
		images, marshalErr := json.Marshal(observation.ImageURLs)
		if marshalErr != nil {
			return 0, false, marshalErr
		}
		if _, err = q.InsertCanonicalSKUContentSnapshot(ctx, dbgen.InsertCanonicalSKUContentSnapshotParams{CanonicalSkuID: row.ID, PrimaryImageUrl: nullText(observation.PrimaryImage), Column3: images, NormalizedHash: contentHash, ObservedAt: ts(at), CollectionDomainRunID: domainRunID}); err != nil {
			return 0, false, err
		}
	}
	return row.ID, metadataChanged || contentChanged, nil
}

func (r *Repository) PersistAssignment(ctx context.Context, sellerID, canonicalSKUID, domainRunID int64, observation SKUObservation, at time.Time, taxonomyReady bool) (bool, error) {
	if r.pool == nil {
		return persistAssignment(ctx, r.q, sellerID, canonicalSKUID, domainRunID, observation, at, taxonomyReady)
	}
	var changed bool
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		var err error
		changed, err = persistAssignment(ctx, r.q.WithTx(tx), sellerID, canonicalSKUID, domainRunID, observation, at, taxonomyReady)
		return err
	})
	return changed, err
}

func persistAssignment(ctx context.Context, q *dbgen.Queries, sellerID, canonicalSKUID, domainRunID int64, observation SKUObservation, at time.Time, taxonomyReady bool) (bool, error) {
	var sellerCategoryID pgtype.Int8
	var providerProductTypeID pgtype.Int8
	mapping := "UNMAPPED"
	categoryResolution := "UNRESOLVED"
	productTypeResolution := "NOT_APPLICABLE"
	if observation.CategoryID == 0 {
		mapping = "UNKNOWN"
		categoryResolution = "NOT_APPLICABLE"
	} else if !taxonomyReady {
		mapping = "UNMAPPED"
	} else if providerCategory, err := q.GetProviderCategoryByExternalID(ctx, observation.CategoryID); err == nil {
		version, versionErr := q.GetLatestProviderCategoryVersion(ctx, providerCategory.ID)
		if versionErr == nil && version.IsLeaf && version.TaxonomyState == "ACTIVE" {
			sellerCategory, upsertErr := q.UpsertSellerCategory(ctx, dbgen.UpsertSellerCategoryParams{SellerAccountID: sellerID, ProviderCategoryID: providerCategory.ID, FirstObservedAt: ts(at)})
			if upsertErr != nil {
				return false, upsertErr
			}
			sellerCategoryID = pgtype.Int8{Int64: sellerCategory.ID, Valid: true}
			mapping = "MAPPED"
			categoryResolution = "RESOLVED"
			if observation.ProductTypeID > 0 {
				productType, typeErr := q.GetProviderProductTypeByExternalID(ctx, dbgen.GetProviderProductTypeByExternalIDParams{ProviderCategoryID: providerCategory.ID, ProviderTypeID: observation.ProductTypeID})
				if typeErr == nil {
					latestType, latestTypeErr := q.GetLatestProviderProductTypeVersion(ctx, productType.ID)
					if latestTypeErr == nil && latestType.TaxonomyState == "ACTIVE" && !latestType.Disabled {
						providerProductTypeID = pgtype.Int8{Int64: productType.ID, Valid: true}
						productTypeResolution = "RESOLVED"
					} else if latestTypeErr != nil && !errors.Is(latestTypeErr, pgx.ErrNoRows) {
						return false, latestTypeErr
					} else {
						productTypeResolution = "UNRESOLVED"
					}
				} else if !errors.Is(typeErr, pgx.ErrNoRows) {
					return false, typeErr
				} else {
					productTypeResolution = "UNRESOLVED"
				}
			}
		} else if versionErr != nil && !errors.Is(versionErr, pgx.ErrNoRows) {
			return false, versionErr
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}
	rawCategory, rawType := nullInt(observation.CategoryID), nullInt(observation.ProductTypeID)
	current, err := q.GetCurrentSKUCategoryAssignment(ctx, canonicalSKUID)
	if err == nil && sameAssignment(current, sellerCategoryID, rawCategory, providerProductTypeID, rawType, mapping, categoryResolution, productTypeResolution) {
		return false, nil
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}
	if err == nil {
		if err := q.CloseCurrentSKUCategoryAssignment(ctx, dbgen.CloseCurrentSKUCategoryAssignmentParams{CanonicalSkuID: canonicalSKUID, ObservedTo: ts(at)}); err != nil {
			return false, err
		}
	}
	_, err = q.CreateSKUCategoryAssignment(ctx, dbgen.CreateSKUCategoryAssignmentParams{CanonicalSkuID: canonicalSKUID, SellerCategoryID: sellerCategoryID, RawProviderCategoryID: rawCategory, ProviderProductTypeID: providerProductTypeID, RawProviderProductTypeID: rawType, MappingStatus: mapping, CategoryResolutionStatus: categoryResolution, ProductTypeResolutionStatus: productTypeResolution, ObservedFrom: ts(at), CollectionDomainRunID: domainRunID})
	return err == nil, err
}

// FinalizeCompleteRun is the sole canonical write path for a collected
// universe. All observed state, assignments, UNOBSERVED reconciliation,
// counters and the CAS completion transition commit in one transaction.
type SupersededRunError struct {
	DomainRunID  int64
	SupersededBy int64
}

func (e *SupersededRunError) Error() string {
	return fmt.Sprintf("catalog domain run %d was superseded by newer authoritative run %d", e.DomainRunID, e.SupersededBy)
}

func (r *Repository) FinalizeCompleteRun(ctx context.Context, sellerID, domainRunID int64, result *Result, at time.Time) error {
	if r.pool == nil {
		return errors.New("catalog finalization requires transactional repository")
	}
	if result == nil {
		return errors.New("catalog finalization result is nil")
	}
	written, assignments, markedUnobserved := 0, 0, 0
	var supersededBy int64
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if _, lockErr := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, fmt.Sprintf("catalog:%d", sellerID)); lockErr != nil {
			return lockErr
		}
		q := r.q.WithTx(tx)
		domain, domainErr := q.GetCollectionDomainRunForSeller(ctx, dbgen.GetCollectionDomainRunForSellerParams{ID: domainRunID, SellerAccountID: sellerID})
		if domainErr != nil {
			return domainErr
		}
		head, headErr := q.GetCollectionAuthoritativeHead(ctx, dbgen.GetCollectionAuthoritativeHeadParams{SellerAccountID: sellerID, Domain: string(collection.DomainCatalog)})
		if headErr != nil && !errors.Is(headErr, pgx.ErrNoRows) {
			return headErr
		}
		if domain.Status == string(collection.DomainRunStatusCompleted) && headErr == nil && head.CollectionDomainRunID >= domainRunID {
			return nil
		}
		if domain.Status != string(collection.DomainRunStatusRunning) {
			return collection.ValidateDomainTransition(collection.DomainRunStatus(domain.Status), collection.DomainRunStatusCompleted)
		}
		if headErr == nil && head.CollectionDomainRunID > domainRunID {
			supersededBy = head.CollectionDomainRunID
			message := fmt.Sprintf("newer authoritative catalog domain run %d already finalized", supersededBy)
			_, cancelErr := q.CancelSupersededCollectionDomainRun(ctx, dbgen.CancelSupersededCollectionDomainRunParams{ID: domainRunID, SellerAccountID: sellerID, ErrorMessage: pgtype.Text{String: message, Valid: true}})
			return cancelErr
		}
		// Canonical version ordering follows serialized finalization order, not
		// an untrusted worker-supplied timestamp captured before this lock.
		at = time.Now().UTC()
		for _, observation := range result.observations {
			canonicalID, changed, persistErr := persistSKU(ctx, q, sellerID, domainRunID, observation, at)
			if persistErr != nil {
				return persistErr
			}
			if changed {
				written++
			}
			assignmentChanged, assignmentErr := persistAssignment(ctx, q, sellerID, canonicalID, domainRunID, observation, at, rTaxonomyReady(result))
			if assignmentErr != nil {
				return assignmentErr
			}
			if assignmentChanged {
				assignments++
				written++
			}
		}
		rows, markErr := q.MarkUnobservedCanonicalSKUs(ctx, dbgen.MarkUnobservedCanonicalSKUsParams{SellerAccountID: sellerID, Column2: result.ObservedSKUs})
		if markErr != nil {
			return markErr
		}
		for _, row := range rows {
			latest, latestErr := q.GetLatestCanonicalSKUVersion(ctx, row.ID)
			if latestErr != nil {
				return latestErr
			}
			observation := SKUObservation{OzonSKU: row.OzonSku, ProductID: latest.ProviderProductID.Int64, OfferID: latest.OfferID.String, Name: latest.Name.String, ProviderStatus: latest.ProviderStatus.String, State: "UNOBSERVED"}
			if _, insertErr := q.InsertCanonicalSKUVersion(ctx, dbgen.InsertCanonicalSKUVersionParams{CanonicalSkuID: row.ID, ProviderProductID: latest.ProviderProductID, OfferID: latest.OfferID, Name: latest.Name, ProviderStatus: latest.ProviderStatus, CatalogState: "UNOBSERVED", NormalizedHash: MetadataHash(observation), ObservedAt: ts(at), CollectionDomainRunID: domainRunID}); insertErr != nil {
				return insertErr
			}
		}
		markedUnobserved = len(rows)
		written += markedUnobserved
		if _, progressErr := q.UpdateCollectionDomainRunProgress(ctx, dbgen.UpdateCollectionDomainRunProgressParams{ID: domainRunID, SellerAccountID: sellerID, RecordsReceived: int64(result.RecordsReceived), RecordsWritten: int64(written), PagesReceived: int64(result.Pages), CompletenessStatus: pgtype.Text{String: string(collection.CompletenessComplete), Valid: true}}); progressErr != nil {
			return progressErr
		}
		_, transitionErr := q.TransitionCollectionDomainRunForSeller(ctx, dbgen.TransitionCollectionDomainRunForSellerParams{ID: domainRunID, SellerAccountID: sellerID, ExpectedStatus: string(collection.DomainRunStatusRunning), Status: string(collection.DomainRunStatusCompleted)})
		if errors.Is(transitionErr, pgx.ErrNoRows) {
			return &collection.StaleTransitionError{EntityKind: "domain run", EntityID: domainRunID, Expected: string(collection.DomainRunStatusRunning), Requested: string(collection.DomainRunStatusCompleted)}
		}
		if transitionErr != nil {
			return transitionErr
		}
		_, headUpdateErr := q.UpsertCollectionAuthoritativeHead(ctx, dbgen.UpsertCollectionAuthoritativeHeadParams{SellerAccountID: sellerID, Domain: string(collection.DomainCatalog), CollectionDomainRunID: domainRunID, FinalizedAt: ts(at)})
		return headUpdateErr
	})
	if err != nil {
		return err
	}
	if supersededBy > 0 {
		return &SupersededRunError{DomainRunID: domainRunID, SupersededBy: supersededBy}
	}
	result.RecordsWritten = written
	result.AssignmentsWritten = assignments
	result.MarkedUnobserved = markedUnobserved
	return nil
}

func rTaxonomyReady(result *Result) bool {
	return result != nil && result.taxonomyReady
}

func sameAssignment(a dbgen.SkuCategoryAssignment, category, rawCategory, productType, rawType pgtype.Int8, status, categoryResolution, productTypeResolution string) bool {
	return a.MappingStatus == status && a.CategoryResolutionStatus == categoryResolution && a.ProductTypeResolutionStatus == productTypeResolution && a.SellerCategoryID == category && a.RawProviderCategoryID == rawCategory && a.ProviderProductTypeID == productType && a.RawProviderProductTypeID == rawType
}

func nullInt(value int64) pgtype.Int8 {
	if value == 0 {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: value, Valid: true}
}

func nullText(value string) pgtype.Text {
	if value == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: value, Valid: true}
}
