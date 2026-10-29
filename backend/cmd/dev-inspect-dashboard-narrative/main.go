package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type narrativeRow struct {
	ID               string
	SellerAccountID  int64
	MetricDate       time.Time
	Status           string
	PromptVersion    string
	ContextHash      string
	RiskDriversHash  string
	RawAIResponse    string
	ValidatedPayload map[string]any
	ValidationErrors any
	CreatedAt        time.Time
	UpdatedAt        time.Time
	FinishedAt       *time.Time
}

func main() {
	var sellerAccountID int64
	var mutatePromptVersion string
	var mutateContextHash string

	flag.Int64Var(&sellerAccountID, "seller-account-id", 0, "target seller_account_id (required)")
	flag.StringVar(&mutatePromptVersion, "set-prompt-version", "", "set prompt_version for latest narrative row")
	flag.StringVar(&mutateContextHash, "set-context-hash", "", "set context_hash for latest narrative row")
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

	latestMetricDate, err := latestMetricDateBySeller(ctx, postgres.Pool, sellerAccountID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "latest metric date: %v\n", err)
		os.Exit(1)
	}

	if mutatePromptVersion != "" || mutateContextHash != "" {
		if err := mutateLatestNarrative(ctx, postgres.Pool, sellerAccountID, latestMetricDate, mutatePromptVersion, mutateContextHash); err != nil {
			fmt.Fprintf(os.Stderr, "mutate latest narrative: %v\n", err)
			os.Exit(1)
		}
	}

	row, err := latestNarrativeBySeller(ctx, postgres.Pool, sellerAccountID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "latest narrative: %v\n", err)
		os.Exit(1)
	}

	nsvc, err := dashboardhome.BuildNarrativeService(cfg, postgres.Pool)
	if err != nil {
		fmt.Fprintf(os.Stderr, "build narrative service: %v\n", err)
		os.Exit(1)
	}
	stockCtx, err := nsvc.BuildStockCoverageContextForDebug(ctx, sellerAccountID, latestMetricDate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "build stock coverage context: %v\n", err)
		os.Exit(1)
	}
	rebuiltContextHash, rebuiltRiskHash, err := nsvc.BuildNarrativeHashesForDebug(ctx, sellerAccountID, latestMetricDate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "build narrative hashes: %v\n", err)
		os.Exit(1)
	}

	rawStock := map[string]any{}
	if row.RawAIResponse != "" {
		var raw map[string]any
		_ = json.Unmarshal([]byte(row.RawAIResponse), &raw)
		if v, ok := raw["stock_coverage_narrative"]; ok {
			if m, ok := v.(map[string]any); ok {
				rawStock = m
			}
		}
	}

	validatedStock := map[string]any{}
	if row.ValidatedPayload != nil {
		if v, ok := row.ValidatedPayload["stock_coverage_narrative"]; ok {
			if m, ok := v.(map[string]any); ok {
				validatedStock = m
			}
		}
	}

	out := map[string]any{
		"latest_narrative_row": map[string]any{
			"id":                row.ID,
			"seller_account_id": row.SellerAccountID,
			"metric_date":       row.MetricDate.Format("2006-01-02"),
			"status":            row.Status,
			"prompt_version":    row.PromptVersion,
			"context_hash":      row.ContextHash,
			"risk_drivers_hash": row.RiskDriversHash,
			"validation_errors": row.ValidationErrors,
			"created_at":        row.CreatedAt.Format(time.RFC3339),
			"updated_at":        row.UpdatedAt.Format(time.RFC3339),
			"finished_at":       formatTimePtr(row.FinishedAt),
		},
		"stock_coverage_context": stockCtx,
		"rebuilt_hashes": map[string]any{
			"context_hash":      rebuiltContextHash,
			"risk_drivers_hash": rebuiltRiskHash,
		},
		"raw_ai_output_stock_coverage_narrative":     rawStock,
		"validated_stock_coverage_narrative":         validatedStock,
		"validated_payload_stock_coverage_narrative": validatedStock,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}

func latestMetricDateBySeller(ctx context.Context, pool *pgxpool.Pool, sellerID int64) (time.Time, error) {
	var metricDate pgtype.Date
	err := pool.QueryRow(ctx, `
		SELECT metric_date
		FROM ai_dashboard_narratives
		WHERE seller_account_id = $1
		ORDER BY metric_date DESC
		LIMIT 1
	`, sellerID).Scan(&metricDate)
	if err != nil {
		return time.Time{}, err
	}
	return metricDate.Time.UTC(), nil
}

func mutateLatestNarrative(ctx context.Context, pool *pgxpool.Pool, sellerID int64, metricDate time.Time, promptVersion string, contextHash string) error {
	if promptVersion == "" {
		promptVersion = "__KEEP__"
	}
	if contextHash == "" {
		contextHash = "__KEEP__"
	}
	_, err := pool.Exec(ctx, `
		UPDATE ai_dashboard_narratives
		SET
			prompt_version = CASE WHEN $3 = '__KEEP__' THEN prompt_version ELSE $3 END,
			context_hash = CASE WHEN $4 = '__KEEP__' THEN context_hash ELSE $4 END,
			updated_at = NOW()
		WHERE seller_account_id = $1 AND metric_date = $2
	`, sellerID, pgtype.Date{Time: metricDate.UTC(), Valid: true}, promptVersion, contextHash)
	return err
}

func latestNarrativeBySeller(ctx context.Context, pool *pgxpool.Pool, sellerID int64) (narrativeRow, error) {
	var (
		row            narrativeRow
		metricDate     pgtype.Date
		rawResponse    pgtype.Text
		validatedBytes []byte
		validationJSON []byte
		finishedAt     pgtype.Timestamptz
	)
	err := pool.QueryRow(ctx, `
		SELECT id::text, seller_account_id, metric_date, status, prompt_version, context_hash, risk_drivers_hash,
		       raw_ai_response, validated_payload, validation_errors, created_at, updated_at, finished_at
		FROM ai_dashboard_narratives
		WHERE seller_account_id = $1
		ORDER BY metric_date DESC
		LIMIT 1
	`, sellerID).Scan(
		&row.ID, &row.SellerAccountID, &metricDate, &row.Status, &row.PromptVersion, &row.ContextHash, &row.RiskDriversHash,
		&rawResponse, &validatedBytes, &validationJSON, &row.CreatedAt, &row.UpdatedAt, &finishedAt,
	)
	if err != nil {
		return narrativeRow{}, err
	}
	row.MetricDate = metricDate.Time.UTC()
	if rawResponse.Valid {
		row.RawAIResponse = rawResponse.String
	}
	if len(validatedBytes) > 0 {
		_ = json.Unmarshal(validatedBytes, &row.ValidatedPayload)
	}
	if len(validationJSON) > 0 {
		_ = json.Unmarshal(validationJSON, &row.ValidationErrors)
	}
	if finishedAt.Valid {
		ft := finishedAt.Time.UTC()
		row.FinishedAt = &ft
	}
	return row, nil
}

func formatTimePtr(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.Format(time.RFC3339)
}
