package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aisummaries"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

func main() {
	var sellerAccountID int64
	var dryRun bool
	var force bool

	flag.Int64Var(&sellerAccountID, "seller-account-id", 0, "target seller_account_id (required)")
	flag.BoolVar(&dryRun, "dry-run", false, "precheck only: no OpenAI calls, no DB writes")
	flag.BoolVar(&force, "force", false, "delete completed/running run for latest metric_date and regenerate")
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

	queries := dbgen.New(postgres.Pool)

	precheck, err := aisummaries.PrecheckForDevGenerate(ctx, queries, aisummaries.DevPrecheckInput{
		SellerAccountID: sellerAccountID,
		OpenAIAPIKey:    cfg.OpenAI.APIKey,
		OpenAIModel:     cfg.OpenAI.Model,
	})
	if err != nil {
		if errors.Is(err, aisummaries.ErrOpenAIKeyRequired) {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	mode := "run"
	if dryRun {
		mode = "dry-run"
	}

	fmt.Println("Generating AI daily summaries")
	fmt.Printf("seller_account_id: %d\n", precheck.SellerAccountID)
	fmt.Printf("seller_name: %s\n", precheck.SellerName)
	fmt.Printf("metric_date: %s\n", precheck.MetricDateStr)
	fmt.Printf("model: %s\n", precheck.Model)
	fmt.Printf("mode: %s\n", mode)

	if dryRun {
		n, err := aisummaries.PreviewCategoryCountForAISummary(ctx, queries, sellerAccountID, precheck.MetricDate, cfg.AI.SummaryMaxCategoriesPerRun)
		if err != nil {
			fmt.Fprintf(os.Stderr, "preview categories: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("categories_to_process: %d (max %d)\n", n, cfg.AI.SummaryMaxCategoriesPerRun)
		dash := analytics.NewDashboardService(postgres.Pool)
		preview, err := aisummaries.PreviewAccountContextForAISummary(ctx, queries, dash, sellerAccountID, precheck.MetricDate, cfg.AI.OpenAIMaxInputTokensApprox())
		if err == nil {
			fmt.Printf("account_context_estimated_input_tokens: %d\n", preview.EstimatedInputTokens)
			fmt.Printf("sku_coverage_total: %d\n", preview.TotalSKUCount)
			fmt.Printf("sku_coverage_backend_scanned: %d\n", preview.BackendScannedSKUCount)
			fmt.Printf("sku_candidates_in_prompt: %d\n", preview.SKUCandidatesInPrompt)
			fmt.Printf("category_rollups_in_prompt: %d\n", preview.CategoryRollups)
			fmt.Printf("raw_top_skus_in_prompt: %t\n", preview.RawTopSKUsIncluded)
		}
		if precheck.ExistingRun != nil {
			fmt.Printf("existing_run_status: %s\n", precheck.ExistingRun.Status)
		}
		fmt.Println("dry-run finished (no OpenAI, no DB writes)")
		return
	}

	if force && precheck.ExistingRun != nil {
		if err := aisummaries.ForceDeleteAIDailyRunForMetricDate(ctx, queries, sellerAccountID, precheck.MetricDate); err != nil {
			fmt.Fprintf(os.Stderr, "force delete: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("force: deleted existing ai_daily_summary run and rows for metric_date")
		precheck.ExistingRun = nil
	} else if precheck.ExistingRun != nil {
		if msg := aisummaries.MessageForExistingRunStatus(precheck.ExistingRun.Status); msg != "" {
			fmt.Println(msg)
			if aisummaries.ShouldSkipDevGenerate(precheck.ExistingRun.Status) {
				printSummaryResult(ctx, queries, precheck)
				return
			}
		}
	}

	var aiService *aisummaries.Service
	if force {
		aiService, err = aisummaries.BuildServiceForAcceptanceLoop(cfg, postgres.Pool)
	} else {
		aiService, err = aisummaries.BuildService(cfg, postgres.Pool)
	}
	if err != nil {
		if errors.Is(err, aisummaries.ErrOpenAIKeyRequired) {
			fmt.Fprintln(os.Stderr, err.Error())
		} else {
			fmt.Fprintf(os.Stderr, "build ai summaries service: %v\n", err)
		}
		os.Exit(1)
	}

	if err := aiService.RunForSellerAfterSync(ctx, sellerAccountID); err != nil {
		fmt.Fprintf(os.Stderr, "RunForSellerAfterSync failed: %v\n", err)
		os.Exit(1)
	}

	printSummaryResult(ctx, queries, precheck)
}

func printSummaryResult(ctx context.Context, q *dbgen.Queries, precheck aisummaries.DevPrecheckResult) {
	report, err := aisummaries.LoadDevRunReport(ctx, q, precheck.SellerAccountID, precheck.MetricDate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load run report: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("AI daily summaries generation finished")
	if report.RunID != "" {
		fmt.Printf("run_id: %s\n", report.RunID)
	}
	fmt.Printf("status: %s\n", report.Status)
	fmt.Printf("metric_date: %s\n", precheck.MetricDateStr)
	if report.AccountSummaryID != "" {
		fmt.Printf("account_summary_id: %s\n", report.AccountSummaryID)
	}
	fmt.Printf("account_has_funnel_ai_analysis: %t\n", report.AccountHasFunnelAIAnalysis)
	fmt.Printf("account_summaries: %d\n", report.AccountSummaries)
	fmt.Printf("category_summaries: %d\n", report.CategorySummaries)
	fmt.Printf("failed_categories: %d\n", report.FailedCategories)
	if report.RunErrorMessage != "" {
		fmt.Printf("run_error_message: %s\n", report.RunErrorMessage)
	}
	if report.FirstCategoryError != "" {
		fmt.Printf("first_category_error: %s\n", report.FirstCategoryError)
	}
	audits, err := aisummaries.LoadSummaryRowAudits(ctx, q, precheck.SellerAccountID, precheck.MetricDate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load row audits: %v\n", err)
	} else {
		fmt.Println()
		fmt.Println("row_audits:")
		for _, a := range audits {
			b, _ := json.Marshal(a)
			fmt.Println(string(b))
		}
	}
	fmt.Println()
	fmt.Println("Check results:")
	fmt.Println("  GET /api/v1/ai-summaries/latest")
	fmt.Println("  http://localhost:3000/app/dashboard")
}
