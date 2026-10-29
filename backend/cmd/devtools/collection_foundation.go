package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

// runCollectionFoundationScenario is an operator-only DB inspection scenario.
// It makes no network request and deliberately does not start a collector.
func runCollectionFoundationScenario(args []string) int {
	fs := flag.NewFlagSet("collection-foundation-scenario", flag.ContinueOnError)
	sellerID := fs.Int64("seller-id", 0, "seller_account_id (required)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *sellerID <= 0 {
		fmt.Fprintln(os.Stderr, "seller-id is required and must be > 0")
		return 2
	}
	_ = config.LoadEnvFiles()
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		return 1
	}
	postgres, err := db.New(context.Background(), cfg.DB.URL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect postgres: %v\n", err)
		return 1
	}
	defer postgres.Close()
	repo := collection.NewRepository(dbgen.New(postgres.Pool))
	service := collection.NewService(repo)
	run, err := repo.CreateRun(context.Background(), *sellerID, collection.CollectionRunKindManual, "devtools", []byte(`{"scenario":"collection-foundation"}`), nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create run: %v\n", err)
		return 1
	}
	domain, err := repo.CreateDomainRun(context.Background(), run.ID, *sellerID, collection.DomainCatalog, nil, nil, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create domain run: %v\n", err)
		return 1
	}
	if _, err = service.TransitionRun(context.Background(), *sellerID, run.ID, collection.CollectionRunStatusRunning); err == nil {
		_, err = service.TransitionDomainRun(context.Background(), *sellerID, domain.ID, collection.DomainRunStatusRunning)
	}
	if err == nil {
		domain, err = service.TransitionDomainRun(context.Background(), *sellerID, domain.ID, collection.DomainRunStatusCompleted)
	}
	if err == nil {
		_, err = service.TransitionRun(context.Background(), *sellerID, run.ID, collection.CollectionRunStatusCompleted)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "advance lifecycle: %v\n", err)
		return 1
	}
	state, err := repo.GetRun(context.Background(), *sellerID, run.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read run: %v\n", err)
		return 1
	}
	output, _ := json.MarshalIndent(map[string]any{"collection_run": state, "domain_run": domain}, "", "  ")
	fmt.Println(string(output))
	return 0
}
