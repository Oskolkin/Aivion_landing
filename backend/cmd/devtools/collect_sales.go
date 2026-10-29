package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection/sales"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/storage"
	"github.com/jackc/pgx/v5/pgtype"
)

func runCollectSales(args []string) int {
	fs := flag.NewFlagSet("collect-sales", flag.ContinueOnError)
	seller := fs.Int64("seller-id", 0, "seller_account_id (required)")
	fromRaw := fs.String("date-from", "", "inclusive provider day YYYY-MM-DD")
	toRaw := fs.String("date-to", "", "inclusive provider day YYYY-MM-DD")
	initial := fs.Bool("initial-90d", false, "last 90 completed Europe/Moscow days")
	resumeDomain := fs.Int64("resume-domain-run-id", 0, "resume an existing RUNNING Sales domain run")
	if fs.Parse(args) != nil || *seller <= 0 {
		fmt.Fprintln(os.Stderr, "seller-id is required and must be > 0")
		return 2
	}
	var from, to time.Time
	var err error
	if *resumeDomain > 0 {
		if *initial || *fromRaw != "" || *toRaw != "" {
			fmt.Fprintln(os.Stderr, "resume-domain-run-id cannot be combined with range flags")
			return 2
		}
	} else if *initial {
		if *fromRaw != "" || *toRaw != "" {
			fmt.Fprintln(os.Stderr, "initial-90d cannot be combined with date-from/date-to")
			return 2
		}
		from, to = sales.Initial90DayRange(time.Now())
	} else {
		if *fromRaw == "" || *toRaw == "" {
			fmt.Fprintln(os.Stderr, "use --initial-90d or provide both --date-from and --date-to")
			return 2
		}
		from, err = time.Parse("2006-01-02", *fromRaw)
		if err != nil {
			fmt.Fprintln(os.Stderr, "invalid date-from")
			return 2
		}
		to, err = time.Parse("2006-01-02", *toRaw)
		if err != nil || to.Before(from) {
			fmt.Fprintln(os.Stderr, "invalid date-to/range")
			return 2
		}
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
	transport := ozon.NewSellerBoundTransport(oz, nil, nil)
	q := dbgen.New(pg.Pool)
	foundation := collection.NewRepository(q)
	life := collection.NewService(foundation)
	var runID, domainID int64
	if *resumeDomain > 0 {
		var domainStatus, parentStatus, domainName string
		err = pg.Pool.QueryRow(ctx, `SELECT dr.collection_run_id,dr.id,dr.requested_from,dr.requested_to,dr.status,dr.domain,cr.status FROM collection_domain_runs dr JOIN collection_runs cr ON cr.id=dr.collection_run_id WHERE dr.id=$1 AND dr.seller_account_id=$2`, *resumeDomain, *seller).Scan(&runID, &domainID, &from, &to, &domainStatus, &domainName, &parentStatus)
		if err != nil || domainName != string(collection.DomainSalesDemand) || domainStatus != string(collection.DomainRunStatusRunning) || parentStatus != string(collection.CollectionRunStatusRunning) {
			fmt.Fprintf(os.Stderr, "resume requires an existing RUNNING Sales domain and RUNNING parent: %v\n", err)
			return 1
		}
	} else {
		kind := collection.CollectionRunKindManual
		if *initial {
			kind = collection.CollectionRunKindInitial
		}
		run, createErr := foundation.CreateRun(ctx, *seller, kind, "devtools.collect-sales", nil, nil)
		if createErr != nil {
			fmt.Fprintln(os.Stderr, createErr)
			return 1
		}
		runID = run.ID
		if _, err = life.TransitionRun(ctx, *seller, runID, collection.CollectionRunStatusRunning); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		domain, createErr := foundation.CreateDomainRunWithContract(ctx, runID, *seller, collection.DomainSalesDemand, &sales.DomainSourceContract, &from, &to)
		if createErr != nil {
			fmt.Fprintln(os.Stderr, createErr)
			return 1
		}
		domainID = domain.ID
		if _, err = life.TransitionDomainRun(ctx, *seller, domainID, collection.DomainRunStatusRunning); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	payloads := collection.NewSourcePayloadStore(s3, cfg.S3.BucketRaw)
	result, err := (&sales.Runner{API: transport, Collection: life, Payloads: payloads, Cursors: life, Recovery: foundation}).Collect(ctx, *seller, domainID, from, to)
	if err == nil {
		err = sales.NewRepository(pg.Pool).Finalize(ctx, *seller, domainID, &result)
	}
	if err != nil {
		disposition := classifySalesFailure(err)
		if disposition.superseded {
			_, _ = life.TransitionRun(ctx, *seller, runID, collection.CollectionRunStatusCancelled)
			printSalesResult(runID, domainID, collection.CollectionRunStatusCancelled, collection.DomainRunStatusCancelled, from, to, result, "SUPERSEDED_BY_NEWER_AUTHORITATIVE_RUN")
			return 0
		}
		message := err.Error()
		if len(message) > 1000 {
			message = message[:1000]
		}
		_, _ = pg.Pool.Exec(ctx, `UPDATE collection_domain_runs SET error_code=$3,error_message=$4,updated_at=NOW() WHERE id=$1 AND seller_account_id=$2`, domainID, *seller, sales.ErrorCode(err), message)
		_, _ = q.UpdateCollectionDomainRunProgress(ctx, dbgen.UpdateCollectionDomainRunProgressParams{ID: domainID, SellerAccountID: *seller, RecordsReceived: int64(result.RecordsReceived), RecordsWritten: int64(result.RecordsWritten), PagesReceived: int64(result.Pages), CompletenessStatus: pgtype.Text{String: string(collection.CompletenessPartial), Valid: true}})
		if disposition.terminalFailure {
			_, _ = life.TransitionDomainRun(ctx, *seller, domainID, collection.DomainRunStatusFailed)
			_, _ = life.TransitionRun(ctx, *seller, runID, collection.CollectionRunStatusFailed)
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	dr, err := foundation.GetDomainRun(ctx, *seller, domainID)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	runStatus := collection.CollectionRunStatusCompleted
	if dr.Status == collection.DomainRunStatusPartial {
		runStatus = collection.CollectionRunStatusCompletedWithLimitations
	}
	if _, err = life.TransitionRun(ctx, *seller, runID, runStatus); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	printSalesResult(runID, domainID, runStatus, dr.Status, from, to, result, "")
	return 0
}

type salesFailureDisposition struct {
	superseded, terminalFailure bool
}

func classifySalesFailure(err error) salesFailureDisposition {
	var superseded *sales.SupersededRunError
	if errors.As(err, &superseded) {
		return salesFailureDisposition{superseded: true}
	}
	return salesFailureDisposition{terminalFailure: !sales.IsResumableInterruption(err)}
}

func printSalesResult(runID, domainID int64, runStatus collection.CollectionRunStatus, domainStatus collection.DomainRunStatus, from, to time.Time, result sales.Result, reason string) {
	out := map[string]any{"collection_run_id": runID, "collection_run_status": runStatus, "sales_domain_run_id": domainID, "sales_domain_status": domainStatus, "date_from": from.Format("2006-01-02"), "date_to": to.Format("2006-01-02"), "result": result, "currency": result.Currency, "auth_mode": "PRIVATE_OAUTH"}
	if reason != "" {
		out["reason"] = reason
	}
	raw, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(raw))
}
