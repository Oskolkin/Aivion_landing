package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

const staleRunManualCloseCode = "manually_closed_stale_local_dev_run"

type runningPipelineReport struct {
	Found         bool   `json:"found"`
	RunID         int64  `json:"run_id,omitempty"`
	SellerID      int64  `json:"seller_id,omitempty"`
	MetricDate    string `json:"metric_date,omitempty"`
	StartedAt     string `json:"started_at,omitempty"`
	MarkedFailed  bool   `json:"marked_failed,omitempty"`
	FailureReason string `json:"failure_reason,omitempty"`
}

func runCheckAIPipelineRunning(args []string) int {
	fs := flag.NewFlagSet("check-ai-pipeline-running", flag.ContinueOnError)
	sellerID := fs.Int64("seller-id", 0, "seller account id")
	metricDate := fs.String("metric-date", "", "metric date YYYY-MM-DD")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *sellerID <= 0 || strings.TrimSpace(*metricDate) == "" {
		fmt.Fprintln(os.Stderr, "seller-id and metric-date are required")
		return 2
	}

	_ = config.LoadEnvFiles()
	report, err := lookupRunningPipelineRun(context.Background(), *sellerID, strings.TrimSpace(*metricDate))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if *jsonOut {
		raw, _ := json.MarshalIndent(report, "", "  ")
		fmt.Println(string(raw))
	} else if report.Found {
		fmt.Printf("run_id: %d\nseller_id: %d\nmetric_date: %s\nstarted_at: %s\n",
			report.RunID, report.SellerID, report.MetricDate, report.StartedAt)
	}
	if report.Found {
		return 2
	}
	return 0
}

func runMarkStaleAIPipelineRun(args []string) int {
	fs := flag.NewFlagSet("mark-stale-ai-pipeline-run", flag.ContinueOnError)
	sellerID := fs.Int64("seller-id", 0, "seller account id")
	metricDate := fs.String("metric-date", "", "metric date YYYY-MM-DD")
	confirm := fs.Bool("confirm", false, "required to apply database change")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *sellerID <= 0 || strings.TrimSpace(*metricDate) == "" {
		fmt.Fprintln(os.Stderr, "seller-id and metric-date are required")
		return 2
	}
	if !*confirm {
		fmt.Fprintln(os.Stderr, "refusing to update database without --confirm=true")
		return 2
	}

	_ = config.LoadEnvFiles()
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	ctx := context.Background()
	postgres, err := db.New(ctx, cfg.DB.URL)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer postgres.Close()

	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(*metricDate))
	if err != nil {
		fmt.Fprintf(os.Stderr, "metric-date: expected YYYY-MM-DD: %v\n", err)
		return 2
	}
	metricDateValue := pgtype.Date{Time: time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, time.UTC), Valid: true}

	queries := dbgen.New(postgres.Pool)
	run, err := queries.GetRunningAIPipelineRunBySellerAndDate(ctx, dbgen.GetRunningAIPipelineRunBySellerAndDateParams{
		SellerAccountID: *sellerID,
		MetricDate:      metricDateValue,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "no running pipeline run found for seller/date")
		return 1
	}

	now := time.Now().UTC()
	_, err = queries.UpdateAIPipelineRun(ctx, dbgen.UpdateAIPipelineRunParams{
		ID:           run.ID,
		Status:       pgtype.Text{String: aipipeline.PipelineStatusFailed, Valid: true},
		ErrorCode:    pgtype.Text{String: staleRunManualCloseCode, Valid: true},
		ErrorMessage: pgtype.Text{String: staleRunManualCloseCode, Valid: true},
		CompletedAt:  pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	fmt.Printf("Marked pipeline run %d as failed (%s)\n", run.ID, staleRunManualCloseCode)
	return 0
}

func lookupRunningPipelineRun(ctx context.Context, sellerID int64, metricDateRaw string) (runningPipelineReport, error) {
	cfg, err := config.Load()
	if err != nil {
		return runningPipelineReport{}, err
	}
	postgres, err := db.New(ctx, cfg.DB.URL)
	if err != nil {
		return runningPipelineReport{}, err
	}
	defer postgres.Close()

	parsed, err := time.Parse("2006-01-02", metricDateRaw)
	if err != nil {
		return runningPipelineReport{}, fmt.Errorf("metric-date: expected YYYY-MM-DD: %w", err)
	}
	metricDate := pgtype.Date{Time: time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, time.UTC), Valid: true}

	run, err := dbgen.New(postgres.Pool).GetRunningAIPipelineRunBySellerAndDate(ctx, dbgen.GetRunningAIPipelineRunBySellerAndDateParams{
		SellerAccountID: sellerID,
		MetricDate:      metricDate,
	})
	if err != nil {
		return runningPipelineReport{Found: false}, nil
	}

	startedAt := ""
	if run.StartedAt.Valid {
		startedAt = run.StartedAt.Time.UTC().Format(time.RFC3339)
	}
	metricDateStr := metricDateRaw
	if run.MetricDate.Valid {
		metricDateStr = run.MetricDate.Time.Format("2006-01-02")
	}

	return runningPipelineReport{
		Found:      true,
		RunID:      run.ID,
		SellerID:   run.SellerAccountID,
		MetricDate: metricDateStr,
		StartedAt:  startedAt,
	}, nil
}
