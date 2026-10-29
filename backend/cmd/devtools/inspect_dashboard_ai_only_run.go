package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

func runInspectDashboardAIOnlyRun(args []string) int {
	fs := flag.NewFlagSet("inspect-dashboard-ai-only-run", flag.ContinueOnError)
	var (
		sellerID      int64
		metricDateRaw string
		runID         string
		dashboardJSON string
		debugDir      string
		dataPackPath  string
		output        string
	)
	fs.Int64Var(&sellerID, "seller-id", 0, "seller_account_id (required)")
	fs.StringVar(&metricDateRaw, "metric-date", "", "metric date YYYY-MM-DD (required)")
	fs.StringVar(&runID, "run-id", "", "optional ai_dashboard_narratives run id")
	fs.StringVar(&dashboardJSON, "dashboard-json", "", "path to /dashboard/home JSON export")
	fs.StringVar(&debugDir, "debug-dir", "", "dashboard narrative debug dir")
	fs.StringVar(&dataPackPath, "data-pack", "", "ai data pack json path")
	fs.StringVar(&output, "output", "dashboard-content-forensics.json", "output report path")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if sellerID <= 0 || metricDateRaw == "" {
		fmt.Fprintln(os.Stderr, "seller-id and metric-date are required")
		return 2
	}

	in := dashboardhome.DashboardContentForensicsInput{MetricDate: metricDateRaw, RunID: runID}

	if dataPackPath != "" {
		pack, err := dashboardhome.LoadDataPackFromFile(dataPackPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "load data pack: %v\n", err)
			return 1
		}
		in.DataPack = pack
	}
	if dashboardJSON != "" {
		resp, err := dashboardhome.LoadDashboardHomeResponseFromFile(dashboardJSON)
		if err != nil {
			fmt.Fprintf(os.Stderr, "load dashboard json: %v\n", err)
			return 1
		}
		in.DashboardHome = resp
		if in.RunID == "" && resp.DashboardNarrativeDebug != nil {
			in.RunID = resp.DashboardNarrativeDebug.LatestRunID
		}
	}
	if debugDir != "" {
		if synth, err := dashboardhome.LoadSynthesisInputFromFile(debugDir + "/synthesis/synthesis_input.json"); err == nil {
			in.SynthesisInput = synth
		}
		if raw, err := os.ReadFile(debugDir + "/synthesis/synthesis_raw_response.txt"); err == nil {
			in.RawAIResponse = string(raw)
		}
		if raw, err := os.ReadFile(debugDir + "/synthesis/synthesis_parsed.json"); err == nil {
			if payload, err := dashboardhome.ParseDashboardNarrativePayloadFromBytes(raw); err == nil {
				in.ParsedPayload = payload
			}
		}
	}

	_ = config.LoadEnvFiles()
	if cfg, err := config.Load(); err == nil {
		ctx := context.Background()
		if postgres, err := db.New(ctx, cfg.DB.URL); err == nil {
			defer postgres.Close()
			if in.RunID == "" {
				if id, err := latestCompletedAIOnlyRunID(ctx, postgres.Pool, sellerID, metricDateRaw); err == nil {
					in.RunID = id
				}
			}
			if in.StoredPayload == nil && in.RunID != "" {
				var raw []byte
				err := postgres.Pool.QueryRow(ctx, `
					SELECT validated_payload
					FROM ai_dashboard_narratives
					WHERE seller_account_id = $1 AND metric_date = $2::date AND id = $3::uuid
				`, sellerID, metricDateRaw, in.RunID).Scan(&raw)
				if err == nil {
					if payload, perr := dashboardhome.ParseDashboardNarrativePayloadFromBytes(raw); perr == nil {
						in.StoredPayload = payload
						if in.ParsedPayload == nil {
							in.ParsedPayload = payload
						}
					}
				}
			}
		}
	}

	report := dashboardhome.RunDashboardContentForensics(in)
	raw, _ := json.MarshalIndent(report, "", "  ")
	if err := os.WriteFile(output, raw, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write output: %v\n", err)
		return 1
	}
	fmt.Println(string(raw))
	if report.Summary.CriticalIssues > 0 {
		return 1
	}
	return 0
}

func latestCompletedAIOnlyRunID(ctx context.Context, pool *pgxpool.Pool, sellerID int64, metricDate string) (string, error) {
	var id string
	err := pool.QueryRow(ctx, `
		SELECT id::text
		FROM ai_dashboard_narratives
		WHERE seller_account_id = $1
		  AND metric_date = $2::date
		  AND status = 'completed'
		  AND prompt_version = $3
		ORDER BY updated_at DESC
		LIMIT 1
	`, sellerID, metricDate, dashboardhome.DashboardNarrativePromptVersion).Scan(&id)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(id), nil
}
