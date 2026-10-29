package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/devseed"
)

// runPrepareDemoSellerCabinet rebuilds seller demo data, builds+validates the
// Analysis Pack, optionally runs Unified AI + publish, and exports a debug bundle.
//
// Typical PowerShell:
//
//	$env:ALLOW_DEV_SEED=1
//	$env:UNIFIED_ANALYSIS_PROVIDER_ENABLED=1
//	$env:UNIFIED_ANALYSIS_PROMPT_PROFILE="analysis"
//	go run ./cmd/devtools prepare-demo-seller-cabinet --seller-id=1 --metric-date=2026-07-08 --run-ai=true --publish=true
func runPrepareDemoSellerCabinet(args []string) int {
	fs := flag.NewFlagSet("prepare-demo-seller-cabinet", flag.ContinueOnError)
	var (
		sellerID      int64
		metricDateRaw string
		outputDir     string
		runAI         bool
		publish       bool
		skipE2E       bool
		products      int
		seed          int64
	)
	fs.Int64Var(&sellerID, "seller-id", 1, "seller_account_id")
	fs.StringVar(&metricDateRaw, "metric-date", "2026-07-08", "metric date YYYY-MM-DD")
	fs.StringVar(&outputDir, "output", "./debug/demo-seller-cabinet", "debug export directory")
	fs.BoolVar(&runAI, "run-ai", true, "run Unified AI analysis after pack validation")
	fs.BoolVar(&publish, "publish", true, "publish Dashboard+Actions as current when AI succeeds")
	fs.BoolVar(&skipE2E, "skip-e2e", true, "skip Playwright smoke (set false to run)")
	fs.IntVar(&products, "products", 80, "MVP product count")
	fs.Int64Var(&seed, "seed", int64(devseed.MVPDefaultSeed), "deterministic seed")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if sellerID <= 0 {
		fmt.Fprintln(os.Stderr, "seller-id must be > 0")
		return 2
	}
	metricDate, err := parseMetricDateCLI(metricDateRaw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "metric-date: %v\n", err)
		return 2
	}
	if err := devseed.AssertDevSeedAllowed(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 2
	}

	_ = os.Setenv("ALLOW_DEV_SEED", "1")
	started := time.Now()
	report := map[string]any{
		"seller_id":   sellerID,
		"metric_date": metricDate.Format("2006-01-02"),
		"started_at":  started.UTC().Format(time.RFC3339),
		"steps":       []map[string]any{},
	}
	addStep := func(name string, ok bool, detail any) {
		steps, _ := report["steps"].([]map[string]any)
		report["steps"] = append(steps, map[string]any{
			"name":   name,
			"ok":     ok,
			"detail": detail,
		})
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir: %v\n", err)
		return 1
	}

	// 1) Reset seller test data (preserve identity/settings).
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

	resetReport, resetErr := devseed.ResetSellerTestData(ctx, postgres.Pool, nil, nil, devseed.SellerResetOptions{
		SellerAccountID:  sellerID,
		DryRun:           false,
		PreserveIdentity: true,
		PreserveSettings: true,
	})
	addStep("reset_seller_data", resetErr == nil, resetReport)
	if resetErr != nil {
		fmt.Fprintf(os.Stderr, "reset: %v\n", resetErr)
		_ = writeDemoReport(outputDir, report)
		return 1
	}

	backendDir := findBackendDir()
	run := func(name string, cmdArgs ...string) (string, error) {
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		cmd.Dir = backendDir
		cmd.Env = append(os.Environ(), "ALLOW_DEV_SEED=1")
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	// 2) Seed MVP commerce + metrics.
	mvpOut, mvpErr := run("dev-seed-mvp", "go", "run", "./cmd/dev-seed-mvp",
		fmt.Sprintf("--seller-account-id=%d", sellerID),
		fmt.Sprintf("--anchor-date=%s", metricDate.Format("2006-01-02")),
		fmt.Sprintf("--products=%d", products),
		fmt.Sprintf("--seed=%d", seed),
		"--reset=false",
		"--days=90",
	)
	addStep("seed_mvp", mvpErr == nil, truncateOut(mvpOut, 4000))
	if mvpErr != nil {
		fmt.Fprintf(os.Stderr, "seed mvp: %v\n%s\n", mvpErr, mvpOut)
		_ = writeDemoReport(outputDir, report)
		return 1
	}

	// 3) Ozon enrichment snapshots (scenario-derived stock grades).
	enrOut, enrErr := run("seed-ozon-enrichment", "go", "run", "./cmd/devtools", "seed-ozon-enrichment",
		fmt.Sprintf("--seller-id=%d", sellerID),
		fmt.Sprintf("--metric-date=%s", metricDate.Format("2006-01-02")),
		"--scenario=rich",
		"--reuse-existing-advertising=true",
	)
	addStep("seed_ozon_enrichment", enrErr == nil, truncateOut(enrOut, 4000))
	if enrErr != nil {
		fmt.Fprintf(os.Stderr, "enrichment: %v\n%s\n", enrErr, enrOut)
		_ = writeDemoReport(outputDir, report)
		return 1
	}

	// 4) Build + validate Analysis Pack (technical + business).
	packDir := filepath.Join(outputDir, "analysis_pack")
	packOut, packErr := run("build-pack", "go", "run", "./cmd/devtools", "build-unified-analysis-pack",
		fmt.Sprintf("--seller-id=%d", sellerID),
		fmt.Sprintf("--metric-date=%s", metricDate.Format("2006-01-02")),
		fmt.Sprintf("--output=%s", packDir),
		"--upload=false",
		"--validate=true",
		"--overwrite=true",
	)
	addStep("build_analysis_pack", packErr == nil, truncateOut(packOut, 6000))
	if packErr != nil {
		fmt.Fprintf(os.Stderr, "pack: %v\n%s\n", packErr, packOut)
		_ = writeDemoReport(outputDir, report)
		return 1
	}

	if runAI {
		aiArgs := []string{"go", "run", "./cmd/devtools", "run-unified-analysis",
			fmt.Sprintf("--seller-id=%d", sellerID),
			"--date-mode=fixed",
			fmt.Sprintf("--metric-date=%s", metricDate.Format("2006-01-02")),
			// direct: local process uses .env.local OpenAI key; docker worker may have a stub key.
			"--execution=direct",
			"--wait=true",
			"--force=true",
			fmt.Sprintf("--publish-as-current=%v", publish),
		}
		aiOut, aiErr := run("run-unified-analysis", aiArgs...)
		addStep("run_unified_analysis", aiErr == nil, truncateOut(aiOut, 8000))
		if aiErr != nil {
			fmt.Fprintf(os.Stderr, "ai run: %v\n%s\n", aiErr, aiOut)
			_ = writeDemoReport(outputDir, report)
			return 1
		}
	}

	if !skipE2E {
		frontendDir := filepath.Join(filepath.Dir(backendDir), "frontend")
		e2e := exec.Command("npx", "playwright", "test", "e2e/unified-publication-smoke.spec.ts", "--reporter=line")
		e2e.Dir = frontendDir
		e2e.Env = os.Environ()
		e2eOut, e2eErr := e2e.CombinedOutput()
		addStep("playwright_smoke", e2eErr == nil, truncateOut(string(e2eOut), 6000))
		if e2eErr != nil {
			fmt.Fprintf(os.Stderr, "e2e: %v\n%s\n", e2eErr, e2eOut)
			_ = writeDemoReport(outputDir, report)
			return 1
		}
	}

	report["finished_at"] = time.Now().UTC().Format(time.RFC3339)
	report["duration_ms"] = time.Since(started).Milliseconds()
	report["ok"] = true
	if err := writeDemoReport(outputDir, report); err != nil {
		fmt.Fprintf(os.Stderr, "write report: %v\n", err)
		return 1
	}
	fmt.Printf("demo cabinet ready: seller=%d metric_date=%s output=%s\n", sellerID, metricDate.Format("2006-01-02"), outputDir)
	return 0
}

func writeDemoReport(dir string, report map[string]any) error {
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "demo_cabinet_report.json"), append(raw, '\n'), 0o644)
}

func truncateOut(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func findBackendDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}
	// .../backend/cmd/devtools/prepare_demo_seller_cabinet.go → backend
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
