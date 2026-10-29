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
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/ozonenrichmentsync"
)

func runSyncOzonEnrichment(args []string) int {
	fs := flag.NewFlagSet("sync-ozon-enrichment", flag.ContinueOnError)
	var (
		sellerID      int64
		metricDateRaw string
		scope         string
		force         bool
	)
	fs.Int64Var(&sellerID, "seller-id", 0, "seller_account_id (required)")
	fs.StringVar(&metricDateRaw, "metric-date", "", "anchor date YYYY-MM-DD")
	fs.StringVar(&scope, "scope", "p0", "enrichment scope")
	fs.BoolVar(&force, "force", false, "reserved for compatibility")
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

	ozonService, err := ozon.NewService(postgres.Pool, cfg.Auth.EncryptionKey, cfg.Ozon.PerformanceAuthBaseURL, cfg.Ozon.DisconnectRetentionDays)
	if err != nil {
		fmt.Fprintf(os.Stderr, "init ozon service: %v\n", err)
		return 1
	}
	service := ozonenrichmentsync.NewService(postgres.Pool, ozonService)
	result, err := service.SyncOzonEnrichmentP0(ctx, ozonenrichmentsync.SyncInput{
		SellerAccountID: sellerID,
		MetricDate:      metricDate,
		Scope:           scope,
		Force:           force,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "sync ozon enrichment: %v\n", err)
		return 1
	}
	raw, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(raw))
	return 0
}

func runOzonEnrichmentStatus(args []string) int {
	fs := flag.NewFlagSet("ozon-enrichment-status", flag.ContinueOnError)
	var (
		sellerID      int64
		metricDateRaw string
		scope         string
	)
	fs.Int64Var(&sellerID, "seller-id", 0, "seller_account_id (required)")
	fs.StringVar(&metricDateRaw, "metric-date", "", "anchor date YYYY-MM-DD")
	fs.StringVar(&scope, "scope", "p0", "enrichment scope")
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

	service := ozonenrichmentsync.NewService(postgres.Pool, nil)
	status, err := service.GetEnrichmentStatus(ctx, sellerID, metricDate, scope)
	if err != nil {
		fmt.Fprintf(os.Stderr, "enrichment status: %v\n", err)
		return 1
	}
	raw, _ := json.MarshalIndent(status, "", "  ")
	fmt.Println(string(raw))
	return 0
}

func parseMetricDateCLI(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		now := time.Now().UTC()
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC), nil
	}
	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("expected YYYY-MM-DD: %w", err)
	}
	return time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, time.UTC), nil
}
