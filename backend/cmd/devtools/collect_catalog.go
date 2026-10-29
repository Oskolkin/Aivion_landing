package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection/catalog"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/storage"
	"github.com/jackc/pgx/v5/pgtype"
)

func runCollectCatalog(args []string) int {
	fs := flag.NewFlagSet("collect-catalog", flag.ContinueOnError)
	seller := fs.Int64("seller-id", 0, "seller_account_id (required)")
	if fs.Parse(args) != nil || *seller <= 0 {
		fmt.Fprintln(os.Stderr, "seller-id is required and must be > 0")
		return 2
	}
	_ = config.LoadEnvFiles()
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	ctx := context.Background()
	pg, err := db.New(ctx, cfg.DB.URL)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer pg.Close()
	s3, err := storage.New(ctx, storage.S3Config{Endpoint: cfg.S3.Endpoint, AccessKey: cfg.S3.AccessKey, SecretKey: cfg.S3.SecretKey, UseSSL: cfg.S3.UseSSL, BucketRaw: cfg.S3.BucketRaw, BucketExports: cfg.S3.BucketExports, BucketArtifacts: cfg.S3.BucketArtifacts})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	oz, err := ozon.NewService(pg.Pool, cfg.Auth.EncryptionKey, cfg.Ozon.PerformanceAuthBaseURL, cfg.Ozon.DisconnectRetentionDays)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	canonicalTransport := ozon.NewSellerBoundTransport(oz, nil, nil)
	q := dbgen.New(pg.Pool)
	foundation := collection.NewRepository(q)
	life := collection.NewService(foundation)
	run, err := foundation.CreateRun(ctx, *seller, collection.CollectionRunKindManual, "devtools.collect-catalog", nil, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	_, err = life.TransitionRun(ctx, *seller, run.ID, collection.CollectionRunStatusRunning)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	taxonomyDomain, err := foundation.CreateDomainRunWithContract(ctx, run.ID, *seller, collection.DomainTaxonomy, &catalog.TaxonomySourceContract, nil, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	catalogDomain, err := foundation.CreateDomainRunWithContract(ctx, run.ID, *seller, collection.DomainCatalog, &catalog.CatalogSourceContract, nil, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	_, err = life.TransitionDomainRun(ctx, *seller, taxonomyDomain.ID, collection.DomainRunStatusRunning)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	payloads := collection.NewSourcePayloadStore(s3, cfg.S3.BucketRaw)
	catalogRepo := catalog.NewRepositoryWithPool(pg.Pool)
	taxonomyResult, taxonomyErr := catalog.CollectTaxonomy(ctx, canonicalTransport, life, payloads, catalogRepo, *seller, taxonomyDomain.ID)
	if taxonomyErr != nil {
		_, _ = q.UpdateCollectionDomainRunProgress(ctx, dbgen.UpdateCollectionDomainRunProgressParams{ID: taxonomyDomain.ID, SellerAccountID: *seller, CompletenessStatus: pgtype.Text{String: string(collection.CompletenessUnavailable), Valid: true}})
		_, _ = life.TransitionDomainRun(ctx, *seller, taxonomyDomain.ID, collection.DomainRunStatusFailed)
		fmt.Fprintf(os.Stderr, "taxonomy collection failed; catalog will preserve SKUs as UNMAPPED: %v\n", taxonomyErr)
	} else {
		_, err = q.UpdateCollectionDomainRunProgress(ctx, dbgen.UpdateCollectionDomainRunProgressParams{ID: taxonomyDomain.ID, SellerAccountID: *seller, RecordsReceived: int64(taxonomyResult.Categories + taxonomyResult.ProductTypes), RecordsWritten: int64(taxonomyResult.Categories + taxonomyResult.ProductTypes), PagesReceived: 1, CompletenessStatus: pgtype.Text{String: string(collection.CompletenessComplete), Valid: true}})
		if err == nil {
			_, err = life.TransitionDomainRun(ctx, *seller, taxonomyDomain.ID, collection.DomainRunStatusCompleted)
		}
		if err != nil {
			_, _ = life.TransitionRun(ctx, *seller, run.ID, collection.CollectionRunStatusFailed)
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	_, err = life.TransitionDomainRun(ctx, *seller, catalogDomain.ID, collection.DomainRunStatusRunning)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	res, err := (&catalog.Runner{API: canonicalTransport, Collection: life, Payloads: payloads, TaxonomyReady: taxonomyErr == nil}).RunCatalog(ctx, *seller, catalogDomain.ID)
	if err != nil {
		_, _ = q.UpdateCollectionDomainRunProgress(ctx, dbgen.UpdateCollectionDomainRunProgressParams{ID: catalogDomain.ID, SellerAccountID: *seller, RecordsReceived: int64(res.RecordsReceived), RecordsWritten: int64(res.RecordsWritten), PagesReceived: int64(res.Pages), CompletenessStatus: pgtype.Text{String: string(collection.CompletenessPartial), Valid: true}})
		_, _ = life.TransitionDomainRun(ctx, *seller, catalogDomain.ID, collection.DomainRunStatusFailed)
		_, _ = life.TransitionRun(ctx, *seller, run.ID, collection.CollectionRunStatusFailed)
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	err = catalogRepo.FinalizeCompleteRun(ctx, *seller, catalogDomain.ID, &res, time.Now().UTC())
	if err != nil {
		_, _ = life.TransitionDomainRun(ctx, *seller, catalogDomain.ID, collection.DomainRunStatusFailed)
		_, _ = life.TransitionRun(ctx, *seller, run.ID, collection.CollectionRunStatusFailed)
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	runStatus := collection.CollectionRunStatusCompleted
	if taxonomyErr != nil {
		runStatus = collection.CollectionRunStatusCompletedWithLimitations
	}
	if _, err = life.TransitionRun(ctx, *seller, run.ID, runStatus); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	summary, err := q.CountCanonicalCatalogSummary(ctx, *seller)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	b, _ := json.MarshalIndent(map[string]any{"collection_run_id": run.ID, "collection_run_status": runStatus, "taxonomy_domain_run_id": taxonomyDomain.ID, "catalog_domain_run_id": catalogDomain.ID, "taxonomy": taxonomyResult, "taxonomy_error": errorString(taxonomyErr), "catalog": res, "canonical_summary": summary}, "", "  ")
	fmt.Println(string(b))
	return 0
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
