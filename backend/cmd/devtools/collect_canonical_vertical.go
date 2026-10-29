package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection/accounthealth"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection/advertising"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection/finance"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection/funnel"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection/price"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection/promotions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection/search"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection/stock"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection/supply"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/storage"
	"github.com/jackc/pgx/v5/pgtype"
)

func runCollectCanonicalVertical(args []string) int {
	fs := flag.NewFlagSet("collect-canonical-vertical", flag.ContinueOnError)
	seller := fs.Int64("seller-id", 0, "seller_account_id (required)")
	domainRaw := fs.String("domain", "", "stock, price, advertising, funnel, finance, search, promotions, health, supply, remaining, or all")
	fromRaw := fs.String("date-from", "", "advertising/funnel/finance inclusive day YYYY-MM-DD")
	toRaw := fs.String("date-to", "", "dated vertical inclusive day YYYY-MM-DD; search through-day")
	resume := fs.Int64("resume-domain-run-id", 0, "resume one existing RUNNING domain run")
	if fs.Parse(args) != nil || *seller <= 0 {
		fmt.Fprintln(os.Stderr, "seller-id is required and must be > 0")
		return 2
	}
	domainName := strings.ToLower(strings.TrimSpace(*domainRaw))
	if domainName != "stock" && domainName != "price" && domainName != "advertising" && domainName != "funnel" && domainName != "finance" && domainName != "search" && domainName != "promotions" && domainName != "health" && domainName != "supply" && domainName != "remaining" && domainName != "all" {
		fmt.Fprintln(os.Stderr, "domain must be stock, price, advertising, funnel, finance, search, promotions, health, supply, remaining, or all")
		return 2
	}
	domains := []string{domainName}
	if domainName == "remaining" {
		domains = []string{"funnel", "finance", "search", "promotions", "health", "supply"}
	}
	if domainName == "all" {
		domains = []string{"stock", "price", "advertising", "funnel", "finance", "search", "promotions", "health", "supply"}
	}
	var from, to time.Time
	var err error
	needsRange := false
	for _, d := range domains {
		if d == "advertising" || d == "funnel" || d == "finance" {
			needsRange = true
		}
	}
	if *resume > 0 {
		if domainName == "all" || domainName == "remaining" {
			fmt.Fprintln(os.Stderr, "resume-domain-run-id cannot be combined with domain=all or remaining")
			return 2
		}
		if *fromRaw != "" || *toRaw != "" {
			fmt.Fprintln(os.Stderr, "resume-domain-run-id cannot be combined with date flags")
			return 2
		}
	} else if needsRange {
		from, err = time.Parse("2006-01-02", *fromRaw)
		if err != nil {
			fmt.Fprintln(os.Stderr, "dated vertical requires valid date-from")
			return 2
		}
		to, err = time.Parse("2006-01-02", *toRaw)
		if err != nil || to.Before(from) {
			fmt.Fprintln(os.Stderr, "dated vertical requires valid date-to/range")
			return 2
		}
	} else if domainName == "search" {
		if *fromRaw != "" {
			fmt.Fprintln(os.Stderr, "search accepts date-to only")
			return 2
		}
		if *toRaw == "" {
			to = time.Now().UTC().AddDate(0, 0, -1)
		} else if to, err = time.Parse("2006-01-02", *toRaw); err != nil {
			fmt.Fprintln(os.Stderr, "search requires valid date-to")
			return 2
		}
		from = to.AddDate(0, 0, -13)
	} else if *fromRaw != "" || *toRaw != "" {
		fmt.Fprintln(os.Stderr, "date flags apply only to advertising, funnel, finance, or search")
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
	oauth, err := ozon.NewService(pg.Pool, cfg.Auth.EncryptionKey, cfg.Ozon.PerformanceAuthBaseURL, cfg.Ozon.DisconnectRetentionDays)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	transport := ozon.NewSellerBoundTransport(oauth, nil, nil)
	foundation := collection.NewRepository(dbgen.New(pg.Pool))
	lifecycle := collection.NewService(foundation)
	payloads := collection.NewSourcePayloadStore(s3, cfg.S3.BucketRaw)

	var run collection.CollectionRun
	if *resume == 0 {
		run, err = foundation.CreateRun(ctx, *seller, collection.CollectionRunKindManual, "devtools.collect-canonical-vertical", nil, nil)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if _, err = lifecycle.TransitionRun(ctx, *seller, run.ID, collection.CollectionRunStatusRunning); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}

	results := make([]map[string]any, 0, len(domains))
	parentStatus := collection.CollectionRunStatusCompleted
	for _, one := range domains {
		var canonicalDomain collection.CollectionDomain
		var contract collection.SourceContract
		var requestedFrom, requestedTo *time.Time
		switch one {
		case "stock":
			canonicalDomain, contract = collection.DomainStock, stock.PrimaryContract
		case "price":
			canonicalDomain, contract = collection.DomainPrice, price.CoreContract
		case "advertising":
			canonicalDomain, contract, requestedFrom, requestedTo = collection.DomainAdvertising, advertising.CampaignContract, &from, &to
		case "funnel":
			canonicalDomain, contract, requestedFrom, requestedTo = collection.DomainFunnel, funnel.DomainSourceContract, &from, &to
		case "finance":
			canonicalDomain, contract, requestedFrom, requestedTo = collection.DomainFinance, finance.ByDayContract, &from, &to
		case "search":
			if to.IsZero() {
				to = time.Now().UTC().AddDate(0, 0, -1)
				from = to.AddDate(0, 0, -13)
			}
			canonicalDomain, contract, requestedFrom, requestedTo = collection.DomainSearch, search.SummaryContract, &from, &to
		case "promotions":
			canonicalDomain, contract = collection.DomainPromotions, promotions.ActionsContract
		case "health":
			canonicalDomain, contract = collection.DomainAccountHealth, accounthealth.SummaryContract
		case "supply":
			canonicalDomain, contract = collection.DomainSupply, supply.StateContract
		}
		var dr collection.DomainRun
		if *resume > 0 {
			var persistedDomain, domainStatus, parent string
			var persistedFrom, persistedTo pgtype.Date
			err = pg.Pool.QueryRow(ctx, `SELECT cr.id,dr.domain,dr.status,cr.status,dr.requested_from,dr.requested_to FROM collection_domain_runs dr JOIN collection_runs cr ON cr.id=dr.collection_run_id WHERE dr.id=$1 AND dr.seller_account_id=$2`, *resume, *seller).Scan(&run.ID, &persistedDomain, &domainStatus, &parent, &persistedFrom, &persistedTo)
			if err != nil || persistedDomain != string(canonicalDomain) || domainStatus != string(collection.DomainRunStatusRunning) || parent != string(collection.CollectionRunStatusRunning) {
				fmt.Fprintf(os.Stderr, "resume requires matching RUNNING domain and parent: %v\n", err)
				return 1
			}
			dr.ID = *resume
			if canonicalDomain == collection.DomainAdvertising || canonicalDomain == collection.DomainFunnel || canonicalDomain == collection.DomainFinance || canonicalDomain == collection.DomainSearch {
				if !persistedFrom.Valid || !persistedTo.Valid {
					fmt.Fprintln(os.Stderr, "dated vertical resume range is missing")
					return 1
				}
				from, to = persistedFrom.Time, persistedTo.Time
			}
		} else {
			dr, err = foundation.CreateDomainRunWithContract(ctx, run.ID, *seller, canonicalDomain, &contract, requestedFrom, requestedTo)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			if _, err = lifecycle.TransitionDomainRun(ctx, *seller, dr.ID, collection.DomainRunStatusRunning); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
		}
		out := map[string]any{"collection_run_id": run.ID, "domain_run_id": dr.ID, "domain": canonicalDomain, "auth_mode": "PRIVATE_OAUTH"}
		switch one {
		case "stock":
			result, e := (&stock.Runner{API: transport, Universe: stock.NewRepository(pg.Pool), Collection: lifecycle, Payloads: payloads, Recovery: foundation}).Collect(ctx, *seller, dr.ID)
			if e == nil {
				e = stock.NewRepository(pg.Pool).Finalize(ctx, *seller, dr.ID, &result)
			}
			out["result"], err = result, e
		case "price":
			result, e := (&price.Runner{API: transport, Universe: price.NewRepository(pg.Pool), Collection: lifecycle, Payloads: payloads, Recovery: foundation}).Collect(ctx, *seller, dr.ID)
			if e == nil {
				e = price.NewRepository(pg.Pool).Finalize(ctx, *seller, dr.ID, &result)
			}
			out["result"], err = result, e
		case "advertising":
			repo := advertising.NewRepository(pg.Pool)
			result, e := (&advertising.Runner{API: transport, Collection: lifecycle, Payloads: payloads, Recovery: foundation, Reports: repo, SKUs: repo, Orders: repo}).Collect(ctx, *seller, dr.ID, from, to)
			if e == nil {
				e = repo.Finalize(ctx, *seller, dr.ID, &result)
			}
			out["result"], err = result, e
		case "funnel":
			repo := funnel.NewRepository(pg.Pool)
			result, e := (&funnel.Runner{API: transport, Collection: lifecycle, Payloads: payloads, Cursors: foundation, Recovery: foundation}).Collect(ctx, *seller, dr.ID, from, to)
			if e == nil {
				e = repo.Finalize(ctx, *seller, dr.ID, &result)
			}
			out["result"], err = result, e
		case "finance":
			repo := finance.NewRepository(pg.Pool)
			result, e := (&finance.Runner{API: transport, Collection: lifecycle, Payloads: payloads}).Collect(ctx, *seller, dr.ID, from, to)
			if e == nil {
				e = repo.Finalize(ctx, *seller, dr.ID, &result)
			}
			out["result"], err = result, e
		case "search":
			repo := search.NewRepository(pg.Pool)
			result, e := (&search.Runner{API: transport, Universe: repo, Collection: lifecycle, Payloads: payloads}).Collect(ctx, *seller, dr.ID, to)
			if e == nil {
				e = repo.Finalize(ctx, *seller, dr.ID, &result)
			}
			out["result"], err = result, e
		case "promotions":
			repo := promotions.NewRepository(pg.Pool)
			result, e := (&promotions.Runner{API: transport, Collection: lifecycle, Payloads: payloads, SKUs: repo}).Collect(ctx, *seller, dr.ID)
			if e == nil {
				e = repo.Finalize(ctx, *seller, dr.ID, &result)
			}
			out["result"], err = result, e
		case "health":
			repo := accounthealth.NewRepository(pg.Pool)
			result, e := (&accounthealth.Runner{API: transport, Collection: lifecycle, Payloads: payloads, Postings: repo}).Collect(ctx, *seller, dr.ID)
			if e == nil {
				e = repo.Finalize(ctx, *seller, dr.ID, &result)
			}
			out["result"], err = result, e
		case "supply":
			repo := supply.NewRepository(pg.Pool)
			result, e := (&supply.Runner{API: transport, Collection: lifecycle, Payloads: payloads, SKUs: repo}).Collect(ctx, *seller, dr.ID)
			if e == nil {
				e = repo.Finalize(ctx, *seller, dr.ID, &result)
			}
			out["result"], err = result, e
		}
		if err != nil {
			_, _ = pg.Pool.Exec(ctx, `UPDATE collection_domain_runs SET error_code='CANONICAL_VERTICAL_COLLECTION_FAILED',error_message=$3,updated_at=NOW() WHERE id=$1 AND seller_account_id=$2`, dr.ID, *seller, err.Error())
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		finalDomain, e := foundation.GetDomainRun(ctx, *seller, dr.ID)
		if e != nil {
			fmt.Fprintln(os.Stderr, e)
			return 1
		}
		if finalDomain.Status == collection.DomainRunStatusPartial {
			parentStatus = collection.CollectionRunStatusCompletedWithLimitations
		}
		out["domain_status"] = finalDomain.Status
		results = append(results, out)
	}
	if *resume == 0 {
		if _, err = lifecycle.TransitionRun(ctx, *seller, run.ID, parentStatus); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	payload := map[string]any{"collection_run_id": run.ID, "collection_run_status": parentStatus, "auth_mode": "PRIVATE_OAUTH", "domains": results}
	if len(results) == 1 {
		payload = results[0]
		payload["collection_run_status"] = parentStatus
	}
	raw, _ := json.MarshalIndent(payload, "", "  ")
	fmt.Println(string(raw))
	return 0
}
