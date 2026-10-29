package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection/lifecycle"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

func runCollectLifecycle(args []string) int {
	fs := flag.NewFlagSet("collect-lifecycle", flag.ContinueOnError)
	seller := fs.Int64("seller-id", 0, "seller_account_id (required)")
	fromRaw := fs.String("date-from", "", "inclusive provider day YYYY-MM-DD")
	toRaw := fs.String("date-to", "", "inclusive provider day YYYY-MM-DD")
	initial := fs.Bool("initial-90d", false, "last 90 completed Europe/Moscow days")
	streams := fs.String("streams", "all", "postings, returns, or all")
	resume := fs.Int64("resume-domain-run-id", 0, "resume one existing RUNNING lifecycle domain run")
	if fs.Parse(args) != nil || *seller <= 0 {
		fmt.Fprintln(os.Stderr, "seller-id is required and must be > 0")
		return 2
	}
	selected := strings.ToLower(strings.TrimSpace(*streams))
	if selected != "all" && selected != "postings" && selected != "returns" {
		fmt.Fprintln(os.Stderr, "streams must be postings, returns, or all")
		return 2
	}
	var from, to time.Time
	var err error
	if *resume > 0 {
		if *initial || *fromRaw != "" || *toRaw != "" || selected == "all" {
			fmt.Fprintln(os.Stderr, "resume requires exactly one stream and no range flags")
			return 2
		}
	} else if *initial {
		from, to = lifecycle.Initial90DayRange(time.Now())
	} else {
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
	oauth, err := ozon.NewService(pg.Pool, cfg.Auth.EncryptionKey, cfg.Ozon.PerformanceAuthBaseURL, cfg.Ozon.DisconnectRetentionDays)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	transport := ozon.NewSellerBoundTransport(oauth, nil, nil)
	foundation := collection.NewRepository(dbgen.New(pg.Pool))
	life := collection.NewService(foundation)
	payloads := collection.NewSourcePayloadStore(s3, cfg.S3.BucketRaw)
	collector := &lifecycle.Runner{API: transport, Collection: life, Payloads: payloads, Cursors: life, Recovery: foundation}
	repo := lifecycle.NewRepository(pg.Pool)
	var parentID int64
	domains := map[string]int64{}
	if *resume > 0 {
		var domainName, status, parentStatus string
		err = pg.Pool.QueryRow(ctx, `SELECT dr.collection_run_id,dr.domain,dr.status,cr.status,dr.requested_from,dr.requested_to FROM collection_domain_runs dr JOIN collection_runs cr ON cr.id=dr.collection_run_id WHERE dr.id=$1 AND dr.seller_account_id=$2`, *resume, *seller).Scan(&parentID, &domainName, &status, &parentStatus, &from, &to)
		if err != nil || status != "RUNNING" || parentStatus != "RUNNING" || domainName != strings.ToUpper(selected) {
			fmt.Fprintf(os.Stderr, "resume requires matching RUNNING domain and parent: %v\n", err)
			return 1
		}
		domains[selected] = *resume
	} else {
		kind := collection.CollectionRunKindManual
		if *initial {
			kind = collection.CollectionRunKindInitial
		}
		run, e := foundation.CreateRun(ctx, *seller, kind, "devtools.collect-lifecycle", nil, nil)
		if e != nil {
			fmt.Fprintln(os.Stderr, e)
			return 1
		}
		parentID = run.ID
		if _, e = life.TransitionRun(ctx, *seller, parentID, collection.CollectionRunStatusRunning); e != nil {
			fmt.Fprintln(os.Stderr, e)
			return 1
		}
		if selected == "all" || selected == "postings" {
			d, e := foundation.CreateDomainRunWithContract(ctx, parentID, *seller, collection.DomainPostings, &lifecycle.PostingsDomainContract, &from, &to)
			if e != nil {
				fmt.Fprintln(os.Stderr, e)
				return 1
			}
			if _, e = life.TransitionDomainRun(ctx, *seller, d.ID, collection.DomainRunStatusRunning); e != nil {
				fmt.Fprintln(os.Stderr, e)
				return 1
			}
			domains["postings"] = d.ID
		}
		if selected == "all" || selected == "returns" {
			d, e := foundation.CreateDomainRunWithContract(ctx, parentID, *seller, collection.DomainReturns, &lifecycle.ReturnsDomainContract, &from, &to)
			if e != nil {
				fmt.Fprintln(os.Stderr, e)
				return 1
			}
			if _, e = life.TransitionDomainRun(ctx, *seller, d.ID, collection.DomainRunStatusRunning); e != nil {
				fmt.Fprintln(os.Stderr, e)
				return 1
			}
			domains["returns"] = d.ID
		}
	}
	out := map[string]any{"collection_run_id": parentID, "requested_from": from.Format("2006-01-02"), "requested_to": to.Format("2006-01-02"), "auth_mode": "PRIVATE_OAUTH", "contracts": map[string]string{"fbo": lifecycle.FBOListContract.SemanticMappingHash, "fbs": lifecycle.FBSListContract.SemanticMappingHash, "returns": lifecycle.ReturnsListContract.SemanticMappingHash}, "redaction_profiles": []string{"OZON_POSTINGS_FBO_REDACTION_V1", "OZON_POSTINGS_FBS_REDACTION_V1", "OZON_RETURNS_REDACTION_V1"}}
	domainStatuses := map[string]string{}
	eventSummaries := map[string]any{}
	out["domain_statuses"] = domainStatuses
	out["lifecycle_events"] = eventSummaries
	limited := false
	for _, name := range []string{"postings", "returns"} {
		id, ok := domains[name]
		if !ok {
			continue
		}
		var collectErr error
		if name == "postings" {
			result, e := collector.CollectPostings(ctx, *seller, id, from, to)
			if e == nil {
				e = repo.FinalizePostings(ctx, *seller, id, &result)
			}
			out["postings_domain_run_id"], out["postings"] = id, result
			collectErr = e
		} else {
			result, e := collector.CollectReturns(ctx, *seller, id, from, to)
			if e == nil {
				e = repo.FinalizeReturns(ctx, *seller, id, &result)
			}
			out["returns_domain_run_id"], out["returns"] = id, result
			collectErr = e
		}
		if collectErr != nil {
			var stale *lifecycle.SupersededRunError
			if errors.As(collectErr, &stale) {
				_, _ = life.TransitionRun(ctx, *seller, parentID, collection.CollectionRunStatusCancelled)
				out["status"] = "CANCELLED"
				out["reason"] = "SUPERSEDED_BY_NEWER_AUTHORITATIVE_RUN"
				printLifecycle(out)
				return 0
			}
			message := collectErr.Error()
			if len(message) > 1000 {
				message = message[:1000]
			}
			_, _ = pg.Pool.Exec(ctx, `UPDATE collection_domain_runs SET error_code=$3,error_message=$4,updated_at=NOW() WHERE id=$1 AND seller_account_id=$2`, id, *seller, lifecycle.ErrorCode(collectErr), message)
			if !lifecycle.IsResumableInterruption(collectErr) {
				_, _ = life.TransitionDomainRun(ctx, *seller, id, collection.DomainRunStatusFailed)
				_, _ = life.TransitionRun(ctx, *seller, parentID, collection.CollectionRunStatusFailed)
			}
			fmt.Fprintln(os.Stderr, collectErr)
			return 1
		}
		dr, e := foundation.GetDomainRun(ctx, *seller, id)
		if e != nil {
			fmt.Fprintln(os.Stderr, e)
			return 1
		}
		if dr.Status == collection.DomainRunStatusPartial {
			limited = true
		}
		domainStatuses[name] = string(dr.Status)
		eventSummary, e := lifecycleEventCounts(ctx, pg.Pool, *seller, id)
		if e != nil {
			fmt.Fprintln(os.Stderr, e)
			return 1
		}
		eventSummaries[name] = eventSummary
	}
	parentStatus := collection.CollectionRunStatusCompleted
	if limited {
		parentStatus = collection.CollectionRunStatusCompletedWithLimitations
	}
	if _, err = life.TransitionRun(ctx, *seller, parentID, parentStatus); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	out["status"] = parentStatus
	printLifecycle(out)
	return 0
}
func printLifecycle(out map[string]any) {
	raw, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(raw))
}

func lifecycleEventCounts(ctx context.Context, pool *pgxpool.Pool, seller, domainRun int64) (map[string]map[string]int64, error) {
	rows, err := pool.Query(ctx, `SELECT event_type,time_quality,count(*) FROM sales_lifecycle_events WHERE seller_account_id=$1 AND collection_domain_run_id=$2 GROUP BY event_type,time_quality ORDER BY event_type,time_quality`, seller, domainRun)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]map[string]int64{}
	for rows.Next() {
		var eventType, quality string
		var count int64
		if err = rows.Scan(&eventType, &quality, &count); err != nil {
			return nil, err
		}
		if out[eventType] == nil {
			out[eventType] = map[string]int64{}
		}
		out[eventType][quality] = count
	}
	return out, rows.Err()
}
