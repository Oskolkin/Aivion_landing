package main

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/publication"
)

// runRebuildDashboardPublication rebuilds only deterministic Dashboard Facts
// and projection from the current seller source rows. It never starts AI.
func runRebuildDashboardPublication(args []string) int {
	fs := flag.NewFlagSet("rebuild-dashboard-publication", flag.ContinueOnError)
	var sellerID int64
	var metricDateRaw string
	fs.Int64Var(&sellerID, "seller-id", 0, "seller_account_id (required)")
	fs.StringVar(&metricDateRaw, "metric-date", "", "metric date YYYY-MM-DD (required)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if sellerID <= 0 || strings.TrimSpace(metricDateRaw) == "" {
		fmt.Fprintln(fs.Output(), "seller-id and metric-date are required")
		return 2
	}
	metricDate, err := time.Parse("2006-01-02", strings.TrimSpace(metricDateRaw))
	if err != nil {
		fmt.Fprintf(fs.Output(), "metric-date: %v\n", err)
		return 2
	}

	ctx := context.Background()
	env, err := loadUnifiedDevtoolEnv(ctx, unifiedDevtoolOverrides{})
	if err != nil {
		fmt.Fprintf(fs.Output(), "load environment: %v\n", err)
		return 1
	}
	defer env.Close()

	result, err := buildPublicationService(env).RebuildDashboardPublication(ctx, publication.RebuildDashboardPublicationRequest{
		SellerAccountID: sellerID,
		MetricDate:      metricDate,
		Actor:           "devtool:rebuild-dashboard-publication",
	})
	if err != nil {
		fmt.Fprintf(fs.Output(), "rebuild dashboard publication: %v\n", err)
		return 1
	}
	printJSON(map[string]any{
		"ok":                      true,
		"seller_account_id":       result.SellerAccountID,
		"metric_date":             result.MetricDate,
		"previous_publication_id": result.PreviousPublicationID,
		"publication_id":          result.PublicationID,
		"source_run_id":           result.SourceRunID,
		"deterministic_run_id":    result.DeterministicRunID,
		"provider_calls_delta":    result.ProviderCallsDelta,
		"reused_existing":         result.ReusedExisting,
	})
	return 0
}
