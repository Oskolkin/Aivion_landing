package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/devseed"
)

func runSeedOzonEnrichment(args []string) int {
	fs := flag.NewFlagSet("seed-ozon-enrichment", flag.ContinueOnError)
	var (
		sellerID                 int64
		metricDateRaw            string
		scenarioRaw              string
		reuseExistingAdvertising bool
	)
	fs.Int64Var(&sellerID, "seller-id", 0, "seller_account_id (required)")
	fs.StringVar(&metricDateRaw, "metric-date", "", "anchor date YYYY-MM-DD")
	fs.StringVar(&scenarioRaw, "scenario", string(devseed.OzonEnrichmentScenarioRich), "rich|minimal|edge")
	fs.BoolVar(&reuseExistingAdvertising, "reuse-existing-advertising", false, "reuse existing ad_campaigns/ad_metrics_daily instead of seeding synthetic advertising fallback (errors if the seller has no existing ad metrics)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if sellerID <= 0 {
		fmt.Fprintln(os.Stderr, "seller-id is required and must be > 0")
		return 2
	}
	metricDate, err := parseMetricDateCLI(metricDateRaw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "metric-date: %v\n", err)
		return 2
	}
	scenario := devseed.OzonEnrichmentSeedScenario(strings.TrimSpace(scenarioRaw))

	_ = config.LoadEnvFiles()
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		return 1
	}
	ctx := context.Background()
	postgres, err := db.New(ctx, cfg.DB.URL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect postgres: %v\n", err)
		return 1
	}
	defer postgres.Close()

	var metricPtr *time.Time
	if strings.TrimSpace(metricDateRaw) != "" {
		metricPtr = &metricDate
	}
	result, err := devseed.SeedOzonEnrichment(ctx, postgres.Pool, devseed.OzonEnrichmentSeedOptions{
		SellerAccountID:          sellerID,
		MetricDate:               metricPtr,
		Scenario:                 scenario,
		EncryptionKey:            cfg.Auth.EncryptionKey,
		ReuseExistingAdvertising: reuseExistingAdvertising,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "seed ozon enrichment: %v\n", err)
		return 1
	}
	raw, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(raw))
	return 0
}
