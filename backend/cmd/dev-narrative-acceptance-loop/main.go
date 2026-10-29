package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
)

type homeResponse struct {
	AIDiagnostics struct {
		AccountSummary struct {
			NarrativeSource  string `json:"narrative_source"`
			Freshness        string `json:"freshness"`
			ValidationStatus string `json:"validation_status"`
			NarrativeRunID   string `json:"narrative_run_id"`
		} `json:"account_summary"`
	} `json:"ai_diagnostics"`
}

func main() {
	runs := 10
	if len(os.Args) > 1 {
		fmt.Sscanf(os.Args[1], "%d", &runs)
	}
	_ = config.LoadEnvFiles()
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	ctx := context.Background()
	postgres, err := db.New(ctx, cfg.DB.URL)
	if err != nil {
		panic(err)
	}
	defer postgres.Close()

	cookiePath := filepath.Join("..", "cookies.txt")
	if _, err := os.Stat(cookiePath); err != nil {
		cookiePath = "cookies.txt"
	}
	cookieBytes, _ := os.ReadFile(cookiePath)

	failed := 0
	for i := 1; i <= runs; i++ {
		genCmd := exec.Command("go", "run", "./cmd/dev-generate-dashboard-narrative", "-seller-account-id", "1")
		genCmd.Dir = "."
		genOut, genErr := genCmd.CombinedOutput()
		genExit := 0
		if genCmd.ProcessState != nil {
			genExit = genCmd.ProcessState.ExitCode()
		}
		_ = genErr

		req, _ := http.NewRequest(http.MethodGet, "http://localhost:18081/api/v1/dashboard/home", nil)
		if len(cookieBytes) > 0 {
			req.Header.Set("Cookie", string(cookieBytes))
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Printf("run %d: home request failed: %v\n", i, err)
			failed++
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var home homeResponse
		_ = json.Unmarshal(body, &home)
		summary := home.AIDiagnostics.AccountSummary
		ok := summary.NarrativeSource == "ai" &&
			(summary.Freshness == "current" || summary.Freshness == "current_with_minor_drift") &&
			(summary.ValidationStatus == "completed" || summary.ValidationStatus == "completed_repaired" || summary.ValidationStatus == "completed_partial")
		fmt.Printf("run %d: gen_exit=%d summary={source:%s freshness:%s status:%s run_id:%s} ok=%v\n",
			i, genExit, summary.NarrativeSource, summary.Freshness, summary.ValidationStatus, summary.NarrativeRunID, ok)
		if !ok {
			failed++
			fmt.Printf("  gen_output: %s\n", string(genOut))
		}
		time.Sleep(500 * time.Millisecond)
	}
	if failed > 0 {
		os.Exit(1)
	}
}
