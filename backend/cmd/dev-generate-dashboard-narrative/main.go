package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
)

func main() {
	var sellerAccountID int64
	var debugOutputDir string
	var aiOnly bool
	flag.Int64Var(&sellerAccountID, "seller-account-id", 0, "target seller_account_id (required)")
	flag.StringVar(&debugOutputDir, "debug-output-dir", "", "optional directory for chunk/synthesis debug artifacts")
	flag.BoolVar(&aiOnly, "ai-only", false, "enable strict AI-only dashboard generation checks")
	flag.Parse()
	if sellerAccountID <= 0 {
		fmt.Fprintln(os.Stderr, "seller-account-id is required and must be > 0")
		os.Exit(2)
	}

	_ = config.LoadEnvFiles()
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}
	ctx := context.Background()
	postgres, err := db.New(ctx, cfg.DB.URL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect postgres: %v\n", err)
		os.Exit(1)
	}
	defer postgres.Close()

	svc, err := dashboardhome.BuildNarrativeService(cfg, postgres.Pool)
	if err != nil {
		fmt.Fprintf(os.Stderr, "build dashboard narrative service: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Generating dashboard narrative for seller_account_id=%d\n", sellerAccountID)
	if aiOnly {
		_ = os.Setenv("DASHBOARD_AI_ONLY", "1")
	}
	result, err := svc.RunForSellerAfterSyncDetailedWithOptions(ctx, sellerAccountID, dashboardhome.NarrativeRunOptions{
		DebugOutputDir: debugOutputDir,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "RunForSellerAfterSyncDetailed failed: %v\n", err)
		os.Exit(1)
	}
	printGenerationResult(result)
	if err := result.DevExitError(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	fmt.Println("Dashboard narrative generation finished")
}

func printGenerationResult(result *dashboardhome.NarrativeGenerationResult) {
	if result == nil {
		fmt.Println("generation_result: null")
		return
	}
	encoded, _ := json.MarshalIndent(result, "", "  ")
	fmt.Printf("generation_result: %s\n", encoded)
}
