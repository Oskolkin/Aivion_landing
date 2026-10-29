package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aisummaries"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
)

type dashboardHome struct {
	AIDiagnostics struct {
		AccountSummary struct {
			NarrativeSource  string `json:"narrative_source"`
			Freshness        string `json:"freshness"`
			ValidationStatus string `json:"validation_status"`
		} `json:"account_summary"`
		CategorySummaries []any `json:"category_summaries"`
		FunnelAnalysis    struct {
			AIAnalysis struct {
				Status string `json:"status"`
			} `json:"ai_analysis"`
		} `json:"funnel_analysis"`
		AISummariesDebug json.RawMessage `json:"ai_summaries_debug"`
	} `json:"ai_diagnostics"`
	RecommendedActions []any `json:"recommended_actions"`
}

func main() {
	var sellerID int64
	var runs int
	var force bool
	var apiURL string
	var cookieFile string

	flag.Int64Var(&sellerID, "seller-account-id", 0, "target seller_account_id (required)")
	flag.IntVar(&runs, "runs", 5, "number of force generations")
	flag.BoolVar(&force, "force", true, "delete existing run before each generation")
	flag.StringVar(&apiURL, "api-url", "http://localhost:18081", "dashboard API base URL")
	flag.StringVar(&cookieFile, "cookie-file", "cookies.txt", "Netscape cookie file for dashboard/home")
	flag.Parse()

	if sellerID <= 0 {
		fmt.Fprintln(os.Stderr, "seller-account-id is required")
		os.Exit(2)
	}
	if runs <= 0 {
		runs = 5
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
		SellerAccountID: sellerID,
		OpenAIAPIKey:    cfg.OpenAI.APIKey,
		OpenAIModel:     cfg.OpenAI.Model,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "precheck: %v\n", err)
		os.Exit(1)
	}

	aiService, err := aisummaries.BuildServiceForAcceptanceLoop(cfg, postgres.Pool)
	if err != nil {
		fmt.Fprintf(os.Stderr, "build service: %v\n", err)
		os.Exit(1)
	}

	cookiePath := resolveCookiePath(cookieFile)
	failedLoops := 0

	for i := 1; i <= runs; i++ {
		fmt.Printf("\n========== AI summaries acceptance run %d/%d ==========\n", i, runs)
		if force {
			if err := aisummaries.ForceDeleteAIDailyRunForMetricDate(ctx, queries, sellerID, precheck.MetricDate); err != nil {
				fmt.Fprintf(os.Stderr, "force delete: %v\n", err)
				failedLoops++
				continue
			}
			fmt.Println("force: deleted existing ai_daily_summary run and rows")
		}

		if err := aiService.RunForSellerAfterSync(ctx, sellerID); err != nil {
			fmt.Fprintf(os.Stderr, "generation failed: %v\n", err)
			failedLoops++
			continue
		}

		audits, err := aisummaries.LoadSummaryRowAudits(ctx, queries, sellerID, precheck.MetricDate)
		if err != nil {
			fmt.Fprintf(os.Stderr, "load audits: %v\n", err)
			failedLoops++
			continue
		}

		runFailed := false
		accountOK := false
		for _, a := range audits {
			b, _ := json.Marshal(a)
			fmt.Println(string(b))
			if a.SummaryType == "account_daily_summary" {
				accountOK = a.DBStatus == "completed" && a.AcceptanceStatus != "failed"
				if !accountOK {
					runFailed = true
				}
				if strings.Contains(a.ErrorMessage, "invalid_json") ||
					strings.Contains(a.ErrorMessage, "validation_failed") ||
					a.Truncated {
					runFailed = true
				}
			}
		}
		if !accountOK {
			runFailed = true
		}

		completedCats := 0
		failedCats := 0
		for _, a := range audits {
			if a.SummaryType != "category_daily_summary" {
				continue
			}
			switch a.DBStatus {
			case "completed":
				completedCats++
			case "failed":
				failedCats++
			}
		}
		fmt.Printf("category_rows: completed=%d failed=%d\n", completedCats, failedCats)
		if completedCats == 0 {
			runFailed = true
		}

		home, homeErr := fetchDashboardHomeCurl(apiURL, cookiePath)
		if homeErr != nil {
			fmt.Fprintf(os.Stderr, "dashboard/home: %v\n", homeErr)
			runFailed = true
		} else {
			ok := checkDashboardHome(home)
			fmt.Printf("dashboard/home: categories=%d funnel_status=%s narrative_source=%s narrative_validation=%s recommended_actions=%d ok=%v\n",
				len(home.AIDiagnostics.CategorySummaries),
				home.AIDiagnostics.FunnelAnalysis.AIAnalysis.Status,
				home.AIDiagnostics.AccountSummary.NarrativeSource,
				home.AIDiagnostics.AccountSummary.ValidationStatus,
				len(home.RecommendedActions),
				ok,
			)
			if !ok {
				runFailed = true
			}
			if len(home.AIDiagnostics.AISummariesDebug) > 0 && i == runs {
				fmt.Println("ai_summaries_debug sample:")
				fmt.Println(prettyJSON(home.AIDiagnostics.AISummariesDebug))
			}
		}

		if runFailed {
			failedLoops++
			fmt.Println("run result: FAIL")
		} else {
			fmt.Println("run result: OK")
		}
		time.Sleep(2 * time.Second)
	}

	if failedLoops > 0 {
		fmt.Printf("\n%d/%d runs failed\n", failedLoops, runs)
		os.Exit(1)
	}
	fmt.Printf("\nall %d runs passed\n", runs)
}

func resolveCookiePath(cookieFile string) string {
	if filepath.IsAbs(cookieFile) {
		return cookieFile
	}
	for _, base := range []string{".", ".."} {
		p := filepath.Join(base, cookieFile)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return cookieFile
}

func fetchDashboardHomeCurl(apiURL, cookiePath string) (dashboardHome, error) {
	var home dashboardHome
	url := strings.TrimRight(apiURL, "/") + "/api/v1/dashboard/home"
	args := []string{"-s", "--noproxy", "*"}
	if cookiePath != "" {
		if _, err := os.Stat(cookiePath); err == nil {
			args = append(args, "-b", cookiePath)
		}
	}
	args = append(args, url)
	out, err := exec.Command("curl.exe", args...).Output()
	if err != nil {
		return home, fmt.Errorf("curl: %w", err)
	}
	if err := json.Unmarshal(out, &home); err != nil {
		return home, fmt.Errorf("parse home json: %w", err)
	}
	return home, nil
}

func checkDashboardHome(home dashboardHome) bool {
	if home.AIDiagnostics.AccountSummary.NarrativeSource == "fallback" {
		return false
	}
	if home.AIDiagnostics.AccountSummary.Freshness == "fallback" {
		return false
	}
	if home.AIDiagnostics.AccountSummary.ValidationStatus == "fallback_stale" {
		return false
	}
	if len(home.AIDiagnostics.CategorySummaries) == 0 {
		return false
	}
	if len(home.RecommendedActions) == 0 {
		return false
	}
	funnelStatus := home.AIDiagnostics.FunnelAnalysis.AIAnalysis.Status
	if funnelStatus == "" || funnelStatus == "error" {
		return false
	}
	return true
}

func prettyJSON(raw json.RawMessage) string {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return string(raw)
	}
	return string(b)
}
