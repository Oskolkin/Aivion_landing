package aisummaries

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrOpenAIKeyRequired = errors.New("OPENAI_API_KEY is required to generate AI summaries")
	ErrSellerNotFound    = errors.New("seller account not found")
	ErrNoDailyMetrics    = errors.New("no daily metrics found for seller account; run dev seed or metrics rebuild first")
)

// DevPrecheckInput is used by the dev-generate-ai-summaries CLI.
type DevPrecheckInput struct {
	SellerAccountID int64
	OpenAIAPIKey    string
	OpenAIModel     string
}

type AccountContextPreview struct {
	EstimatedInputTokens   int
	TotalSKUCount          int
	BackendScannedSKUCount int
	SKUCandidatesInPrompt  int
	RawTopSKUsIncluded     bool
	CategoryRollups        int
}

func PreviewAccountContextForAISummary(ctx context.Context, q *dbgen.Queries, dash *analytics.DashboardService, sellerID int64, metricDate pgtype.Date, maxInputTokens int) (AccountContextPreview, error) {
	if maxInputTokens <= 0 {
		maxInputTokens = 20000
	}
	pkg, err := BuildAccountContext(ctx, q, dash, sellerID, metricDate.Time.UTC(), nil)
	if err != nil {
		return AccountContextPreview{}, err
	}
	ctxMap := structToMap(pkg)
	trim := trimAccountContextForTokenBudget(systemPromptDailySummary(), userPromptAccountSummary(), ctxMap, maxInputTokens)
	out := AccountContextPreview{
		EstimatedInputTokens: trim.FinalTokens,
		CategoryRollups:      len(pkg.CategoryRollupTop),
	}
	if cov, ok := ctxMap["sku_coverage"].(map[string]any); ok {
		if v, ok := cov["total_sku_count"].(float64); ok {
			out.TotalSKUCount = int(v)
		}
		if v, ok := cov["backend_scanned_sku_count"].(float64); ok {
			out.BackendScannedSKUCount = int(v)
		}
		if v, ok := cov["sku_candidates_in_prompt"].(float64); ok {
			out.SKUCandidatesInPrompt = int(v)
		}
	}
	if dashMap, ok := ctxMap["dashboard"].(map[string]any); ok {
		_, out.RawTopSKUsIncluded = dashMap["top_skus"]
	}
	return out, nil
}

// DevPrecheckResult holds context printed before generation.
type DevPrecheckResult struct {
	SellerAccountID int64
	SellerName      string
	MetricDate      pgtype.Date
	MetricDateStr   string
	Model           string
	ExistingRun     *dbgen.AiDailySummaryRun
}

// ValidateOpenAIKeyForDev returns ErrOpenAIKeyRequired when the key is empty.
func ValidateOpenAIKeyForDev(apiKey string) error {
	if strings.TrimSpace(apiKey) == "" {
		return ErrOpenAIKeyRequired
	}
	return nil
}

// PrecheckForDevGenerate verifies seller, metrics, and OpenAI config for local dev runs.
func PrecheckForDevGenerate(ctx context.Context, q *dbgen.Queries, in DevPrecheckInput) (DevPrecheckResult, error) {
	if in.SellerAccountID <= 0 {
		return DevPrecheckResult{}, fmt.Errorf("seller-account-id is required and must be > 0")
	}
	if err := ValidateOpenAIKeyForDev(in.OpenAIAPIKey); err != nil {
		return DevPrecheckResult{}, err
	}

	sa, err := q.GetSellerAccountByID(ctx, in.SellerAccountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return DevPrecheckResult{}, ErrSellerNotFound
		}
		return DevPrecheckResult{}, fmt.Errorf("get seller account: %w", err)
	}

	latest, err := q.GetLatestAvailableDashboardMetricDateBySellerAccountID(ctx, in.SellerAccountID)
	if err != nil {
		return DevPrecheckResult{}, fmt.Errorf("latest metric date: %w", err)
	}
	if !latest.Valid || latest.Time.Year() < 2000 {
		return DevPrecheckResult{}, ErrNoDailyMetrics
	}

	model := strings.TrimSpace(in.OpenAIModel)
	if model == "" {
		model = "gpt-4.1-mini"
	}

	out := DevPrecheckResult{
		SellerAccountID: in.SellerAccountID,
		SellerName:      sa.Name,
		MetricDate:      latest,
		MetricDateStr:   latest.Time.UTC().Format("2006-01-02"),
		Model:           model,
	}

	run, err := q.GetAIDailySummaryRunBySellerAndMetricDate(ctx, dbgen.GetAIDailySummaryRunBySellerAndMetricDateParams{
		SellerAccountID: in.SellerAccountID,
		MetricDate:      latest,
	})
	if err == nil {
		out.ExistingRun = &run
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return DevPrecheckResult{}, fmt.Errorf("get ai summary run: %w", err)
	}

	return out, nil
}

// ForceDeleteAIDailyRunForMetricDate removes summaries and run rows so generation can rerun.
func ForceDeleteAIDailyRunForMetricDate(ctx context.Context, q *dbgen.Queries, sellerID int64, metricDate pgtype.Date) error {
	_ = q.ClearAIDailySummaryRunAccountFK(ctx, dbgen.ClearAIDailySummaryRunAccountFKParams{
		SellerAccountID: sellerID,
		MetricDate:      metricDate,
	})
	if err := q.DeleteAIDailySummariesBySellerMetricDate(ctx, dbgen.DeleteAIDailySummariesBySellerMetricDateParams{
		SellerAccountID: sellerID,
		MetricDate:      metricDate,
	}); err != nil {
		return fmt.Errorf("delete ai summaries: %w", err)
	}
	if err := q.DeleteAIDailySummaryRunsBySellerMetricDate(ctx, dbgen.DeleteAIDailySummaryRunsBySellerMetricDateParams{
		SellerAccountID: sellerID,
		MetricDate:      metricDate,
	}); err != nil {
		return fmt.Errorf("delete ai summary runs: %w", err)
	}
	return nil
}

// MessageForExistingRunStatus returns a user-facing line when generation should be skipped.
func MessageForExistingRunStatus(status string) string {
	switch status {
	case "completed":
		return "AI daily summary run already completed for this metric_date."
	case "running":
		return "AI daily summary run is already running."
	case "pending":
		return "AI daily summary run is pending; wait for the worker or retry after it finishes."
	default:
		return ""
	}
}

// ShouldSkipDevGenerate is true when RunForSellerAfterSync would no-op (completed/running/pending).
func ShouldSkipDevGenerate(status string) bool {
	switch status {
	case "completed", "running", "pending":
		return true
	default:
		return false
	}
}

// SummaryCounts holds completed summary rows for a seller + metric_date.
type SummaryCounts struct {
	AccountSummaries  int
	CategorySummaries int
}

// DevRunReport is printed by dev-generate-ai-summaries after a run.
type DevRunReport struct {
	SummaryCounts
	RunID                      string
	Status                     string
	AccountSummaryID           string
	AccountHasFunnelAIAnalysis bool
	RunErrorMessage            string
	FailedCategories           int
	FirstCategoryError         string
}

// LoadDevRunReport loads run row + completed/failed summary counts for CLI output.
func LoadDevRunReport(ctx context.Context, q *dbgen.Queries, sellerID int64, metricDate pgtype.Date) (DevRunReport, error) {
	counts, err := CountCompletedSummariesForMetricDate(ctx, q, sellerID, metricDate)
	if err != nil {
		return DevRunReport{}, err
	}
	out := DevRunReport{SummaryCounts: counts, Status: "unknown"}

	run, err := q.GetAIDailySummaryRunBySellerAndMetricDate(ctx, dbgen.GetAIDailySummaryRunBySellerAndMetricDateParams{
		SellerAccountID: sellerID,
		MetricDate:      metricDate,
	})
	if err == nil {
		if run.ID.Valid {
			out.RunID = uuid.UUID(run.ID.Bytes).String()
		}
		out.Status = run.Status
		out.FailedCategories = int(run.CategoryFailedCount)
		if run.AccountSummaryID.Valid {
			out.AccountSummaryID = uuid.UUID(run.AccountSummaryID.Bytes).String()
		}
		if run.ErrorMessage.Valid {
			out.RunErrorMessage = run.ErrorMessage.String
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return DevRunReport{}, fmt.Errorf("get ai summary run: %w", err)
	}

	rows, err := q.ListAIDailySummariesForSellerMetricDate(ctx, dbgen.ListAIDailySummariesForSellerMetricDateParams{
		SellerAccountID: sellerID,
		MetricDate:      metricDate,
	})
	if err != nil {
		return DevRunReport{}, fmt.Errorf("list ai summaries: %w", err)
	}
	for _, row := range rows {
		if row.ID.Valid && out.AccountSummaryID == "" && row.SummaryType == "account_daily_summary" {
			out.AccountSummaryID = uuid.UUID(row.ID.Bytes).String()
		}
		if row.SummaryType == "account_daily_summary" && len(row.SummaryJson) > 0 {
			var raw map[string]any
			if err := json.Unmarshal(row.SummaryJson, &raw); err == nil {
				_, out.AccountHasFunnelAIAnalysis = raw["funnel_ai_analysis"]
			}
		}
		if row.SummaryType != "category_daily_summary" || row.Status != "failed" {
			continue
		}
		name := ""
		if row.ScopeName.Valid {
			name = row.ScopeName.String
		}
		reason := ""
		if row.ErrorMessage.Valid {
			reason = row.ErrorMessage.String
		}
		out.FirstCategoryError = fmt.Sprintf(
			"category_id=%s category_name=%q: %s",
			row.ScopeID,
			name,
			reason,
		)
		break
	}

	return out, nil
}

// CountCompletedSummariesForMetricDate counts completed account/category daily summaries.
func CountCompletedSummariesForMetricDate(ctx context.Context, q *dbgen.Queries, sellerID int64, metricDate pgtype.Date) (SummaryCounts, error) {
	rows, err := q.ListAIDailySummariesForSellerMetricDate(ctx, dbgen.ListAIDailySummariesForSellerMetricDateParams{
		SellerAccountID: sellerID,
		MetricDate:      metricDate,
	})
	if err != nil {
		return SummaryCounts{}, fmt.Errorf("list ai summaries: %w", err)
	}
	var out SummaryCounts
	for _, row := range rows {
		if row.Status != "completed" {
			continue
		}
		switch row.SummaryType {
		case "account_daily_summary":
			out.AccountSummaries++
		case "category_daily_summary":
			out.CategorySummaries++
		}
	}
	return out, nil
}

// PreviewCategoryCountForAISummary estimates how many category summaries would be generated (read-only).
func PreviewCategoryCountForAISummary(ctx context.Context, q *dbgen.Queries, sellerID int64, metricDate pgtype.Date, maxCategories int) (int, error) {
	rollup, err := q.ListDescriptionCategoryRollupForAISummary(ctx, dbgen.ListDescriptionCategoryRollupForAISummaryParams{
		SellerAccountID: sellerID,
		MetricDate:      metricDate,
	})
	if err != nil {
		return 0, fmt.Errorf("category rollup: %w", err)
	}
	if maxCategories <= 0 {
		maxCategories = 10
	}
	catIDs, err := selectCategoryIDsForAISummary(ctx, q, sellerID, metricDate, rollup, maxCategories, nil)
	if err != nil {
		return 0, fmt.Errorf("select categories: %w", err)
	}
	return len(catIDs), nil
}
