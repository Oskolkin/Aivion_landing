package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/devseed"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/sourcerecon"
)

// runValidateSellerAnalysisData loads a seller's analysis-data snapshot and runs the
// full shared reconciliation (sourcerecon + DB source-of-truth checks), then writes
// a JSON + CSV report under --output. Exit code is non-zero when the report is not
// valid so it can gate CI/manual acceptance.
func runValidateSellerAnalysisData(args []string) int {
	fs := flag.NewFlagSet("validate-seller-analysis-data", flag.ContinueOnError)
	var (
		sellerID      int64
		metricDateRaw string
		historyDays   int
		output        string
	)
	fs.Int64Var(&sellerID, "seller-id", 0, "seller_account_id (required)")
	fs.Int64Var(&sellerID, "seller-account-id", 0, "alias for --seller-id")
	fs.StringVar(&metricDateRaw, "metric-date", "", "anchor date YYYY-MM-DD (default: MAX(daily_account_metrics.metric_date))")
	fs.IntVar(&historyDays, "history-days", 30, "inclusive history window ending at the anchor")
	fs.StringVar(&output, "output", "./validate-seller-analysis-data", "output directory for report.json + report.csv")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if sellerID <= 0 {
		fmt.Fprintln(os.Stderr, "seller-id is required and must be > 0")
		return 2
	}

	var metricDate time.Time
	if strings.TrimSpace(metricDateRaw) != "" {
		parsed, err := time.Parse("2006-01-02", strings.TrimSpace(metricDateRaw))
		if err != nil {
			fmt.Fprintf(os.Stderr, "metric-date: expected YYYY-MM-DD: %v\n", err)
			return 2
		}
		metricDate = time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, time.UTC)
	}
	if historyDays <= 0 {
		historyDays = 30
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

	report, err := devseed.ReconcileSellerAnalysisData(ctx, postgres.Pool, devseed.ReconciliationOptions{
		SellerAccountID: sellerID,
		MetricDate:      metricDate,
		HistoryDays:     historyDays,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "validate-seller-analysis-data: %v\n", err)
		return 1
	}

	if err := writeReconciliationReport(output, report); err != nil {
		fmt.Fprintf(os.Stderr, "write report: %v\n", err)
		return 1
	}

	fmt.Printf("validate-seller-analysis-data: seller=%d valid=%t errors=%d warnings=%d hash=%s\n",
		sellerID, report.Valid, len(report.Errors), len(report.Warnings), report.Hash)
	fmt.Printf("report written to %s\n", output)
	if !report.Valid {
		return 1
	}
	return 0
}

func writeReconciliationReport(dir string, report sourcerecon.Report) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}

	jsonPath := filepath.Join(dir, "report.json")
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	if err := os.WriteFile(jsonPath, raw, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", jsonPath, err)
	}

	csvPath := filepath.Join(dir, "report.csv")
	f, err := os.Create(csvPath)
	if err != nil {
		return fmt.Errorf("create %s: %w", csvPath, err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()
	if err := w.Write([]string{"severity", "code", "path", "date", "entity_id", "expected", "actual", "message"}); err != nil {
		return err
	}
	// Only real findings are written — never a NOT_CHECKED placeholder row.
	writeIssues := func(severity string, issues []sourcerecon.ReconciliationIssue) error {
		for _, issue := range issues {
			if err := w.Write([]string{
				severity,
				issue.Code,
				issue.Path,
				issue.Date,
				issue.EntityID,
				formatAny(issue.Expected),
				formatAny(issue.Actual),
				issue.Message,
			}); err != nil {
				return err
			}
		}
		return nil
	}
	if err := writeIssues("error", report.Errors); err != nil {
		return err
	}
	if err := writeIssues("warning", report.Warnings); err != nil {
		return err
	}
	return nil
}

func formatAny(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}
