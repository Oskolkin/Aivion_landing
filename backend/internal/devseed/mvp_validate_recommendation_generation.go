package devseed

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicost"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/httpserver/handlers"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/recommendations"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrRecommendationValidateOpenAIMissing is returned when OPENAI_API_KEY is unset (caller may exit 2).
var ErrRecommendationValidateOpenAIMissing = errors.New("OPENAI_API_KEY missing")

// MVPRecommendationGenerationValidateOptions configures --validate-recommendation-generation.
type MVPRecommendationGenerationValidateOptions struct {
	SellerAccountID int64
	AsOfDate        *time.Time
}

type mvpRecGenRow struct {
	Check    string
	Expected string
	Actual   string
	OK       bool
}

// NewRecommendationServiceForMVPDev wires recommendations.Service the same way as cmd/api (OpenAI + context limits).
func NewRecommendationServiceForMVPDev(pool *pgxpool.Pool, cfg *config.Config) (*recommendations.Service, error) {
	repo := recommendations.NewSQLCRepository(dbgen.New(pool))
	openAIClient, err := recommendations.NewOpenAIClient(recommendations.OpenAIClientConfig{
		APIKey:               cfg.OpenAI.APIKey,
		Model:                cfg.OpenAI.Model,
		TimeoutSeconds:       cfg.OpenAI.TimeoutSeconds,
		MaxRetries:           cfg.OpenAI.MaxRetries,
		ProxyURL:             cfg.OpenAI.AIHTTPSProxyURL(),
		MaxInputTokensApprox: cfg.AI.OpenAIMaxInputTokensApprox(),
		MaxOutputTokens:      cfg.AI.OpenAIMaxOutputTokens(),
	})
	if err != nil {
		return nil, err
	}
	return recommendations.NewService(
		repo,
		recommendations.NewContextBuilderWithLimits(repo, recommendations.ContextBuildLimits{
			MaxItemsPerList: cfg.AI.RecommendationMaxContextItems,
			MaxContextBytes: cfg.AI.RecommendationMaxContextBytes,
		}),
		openAIClient,
		recommendations.NewOutputValidator(),
		recommendations.ServiceConfig{
			RunType:       "manual",
			Source:        "chatgpt",
			Model:         cfg.OpenAI.Model,
			PromptVersion: "stage8.prompt.v1",
		},
		aicost.NewDailyUsageGuard(dbgen.New(pool), aicost.DailyLimitsConfig{
			ChatMaxQuestionsPerDay:      cfg.AI.ChatMaxQuestionsPerDay,
			RecommendationMaxRunsPerDay: cfg.AI.RecommendationMaxRunsPerDay,
			SummaryMaxRunsPerDay:        cfg.AI.SummaryMaxRunsPerDay,
		}),
	), nil
}

// ValidateMVPRecommendationGeneration runs the production recommendation generator (GenerateForAccount → OpenAI)
// and checks DB invariants. It does not insert recommendation rows except via the real service path.
func ValidateMVPRecommendationGeneration(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, svc *recommendations.Service, opts MVPRecommendationGenerationValidateOptions) (bool, error) {
	if opts.SellerAccountID <= 0 {
		return false, fmt.Errorf("seller_account_id must be > 0")
	}

	q := dbgen.New(pool)
	var rows []mvpRecGenRow
	allOK := true
	add := func(check, expected, actual string, ok bool) {
		if !ok {
			allOK = false
		}
		rows = append(rows, mvpRecGenRow{Check: check, Expected: expected, Actual: actual, OK: ok})
	}

	openAlerts, err := q.CountOpenAlertsBySellerAccountID(ctx, opts.SellerAccountID)
	if err != nil {
		return false, fmt.Errorf("count open alerts: %w", err)
	}
	if openAlerts == 0 {
		fmt.Fprintf(os.Stderr, "validate-recommendation-generation: no open alerts for seller_account_id=%d.\n"+
			"  Run first: go run ./cmd/dev-seed-mvp --seller-account-id %d --validate-alert-generation\n"+
			"  or use /app/alerts → Run alerts.\n", opts.SellerAccountID, opts.SellerAccountID)
		add("prerequisite: open alerts", "> 0", fmt.Sprintf("%d", openAlerts), false)
		printMVPRecommendationGenTable(rows)
		return false, fmt.Errorf("missing prerequisite alerts")
	}
	add("prerequisite: open alerts", "> 0", fmt.Sprintf("%d", openAlerts), true)

	if strings.TrimSpace(cfg.OpenAI.APIKey) == "" {
		fmt.Fprintln(os.Stderr, "validate-recommendation-generation: OPENAI_API_KEY is not set or empty. Set the same variable as for the API server (e.g. backend/.env), then re-run.")
		add("OpenAI API key", "non-empty", "(empty)", false)
		printMVPRecommendationGenTable(rows)
		return false, ErrRecommendationValidateOpenAIMissing
	}
	add("OpenAI API key", "non-empty", "set", true)

	asOf, err := resolveMVPAlertAsOfDate(ctx, pool, opts.SellerAccountID, opts.AsOfDate)
	if err != nil {
		return false, err
	}

	fmt.Printf("Running recommendation generator (GenerateForAccount, as_of_date=%s)…\n", asOf.Format("2006-01-02"))
	genSummary, genErr := svc.GenerateForAccount(ctx, opts.SellerAccountID, asOf)

	runRow, err := q.GetLatestRecommendationRunBySellerAccountID(ctx, opts.SellerAccountID)
	if err != nil {
		return false, fmt.Errorf("get latest recommendation run: %w", err)
	}
	add("recommendation_run created", "row exists", fmt.Sprintf("run_id=%d", runRow.ID), runRow.ID > 0)

	termOK := runRow.Status == "completed" || runRow.Status == "failed"
	var termDetail string
	if runRow.ErrorMessage.Valid && strings.TrimSpace(runRow.ErrorMessage.String) != "" {
		termDetail = fmt.Sprintf("status=%s err=%s", runRow.Status, strings.TrimSpace(runRow.ErrorMessage.String))
	} else {
		termDetail = fmt.Sprintf("status=%s", runRow.Status)
	}
	add("run terminal state", "completed or failed", termDetail, termOK)

	if genErr != nil {
		add("GenerateForAccount", "no error", genErr.Error(), false)
		printRecommendationRunDiagnostics(ctx, q, opts.SellerAccountID, runRow.ID)
		printMVPRecommendationGenTable(rows)
		fmt.Println()
		fmt.Println("Hints: fix the error above (OpenAI quota, context size, validator). If the run status is failed with a clear error_message, the engine path is working.")
		return false, fmt.Errorf("recommendation generation: %w", genErr)
	}
	add("GenerateForAccount", "no error", "ok", true)

	if genSummary == nil {
		return false, fmt.Errorf("nil summary after successful generate")
	}
	if runRow.ID != genSummary.RunID {
		add("run_id matches summary", fmt.Sprintf("%d", genSummary.RunID), fmt.Sprintf("%d", runRow.ID), false)
		printMVPRecommendationGenTable(rows)
		return false, fmt.Errorf("latest run id %d != summary run id %d", runRow.ID, genSummary.RunID)
	}
	add("run_id matches summary", fmt.Sprintf("%d", genSummary.RunID), fmt.Sprintf("%d", runRow.ID), true)

	diagCount, err := countRecommendationRunDiagnostics(ctx, q, opts.SellerAccountID, runRow.ID)
	if err != nil {
		return false, err
	}
	add("recommendation_run_diagnostics", ">= 1 row", fmt.Sprintf("%d", diagCount), diagCount >= 1)

	if runRow.Status == "completed" && runRow.GeneratedRecommendationsCount == 0 && openAlerts > 0 {
		add("run completed with open alerts", "failed or saved > 0", fmt.Sprintf("status=%s generated=%d", runRow.Status, runRow.GeneratedRecommendationsCount), false)
		printRecommendationRunDiagnostics(ctx, q, opts.SellerAccountID, runRow.ID)
		printMVPRecommendationGenTable(rows)
		return false, fmt.Errorf("run completed with zero recommendations despite %d open alerts", openAlerts)
	}

	if runRow.Status != "completed" {
		add("run status after success", "completed", runRow.Status, false)
		printRecommendationRunDiagnostics(ctx, q, opts.SellerAccountID, runRow.ID)
		printMVPRecommendationGenTable(rows)
		return false, fmt.Errorf("expected completed run, got %s", runRow.Status)
	}
	add("run status after success", "completed", runRow.Status, true)
	add("generated_recommendations_count", "> 0", fmt.Sprintf("%d", runRow.GeneratedRecommendationsCount), runRow.GeneratedRecommendationsCount > 0)

	totalRec, err := q.CountRecommendationsBySellerAccountID(ctx, opts.SellerAccountID)
	if err != nil {
		return false, err
	}
	add("recommendations_total", "> 0", fmt.Sprintf("%d", totalRec), totalRec > 0)

	openRec, err := q.CountOpenRecommendationsBySellerAccountID(ctx, opts.SellerAccountID)
	if err != nil {
		return false, err
	}
	add("open recommendations", "> 0", fmt.Sprintf("%d", openRec), openRec > 0)

	linkN, err := q.CountRecommendationAlertLinksBySellerAccountID(ctx, opts.SellerAccountID)
	if err != nil {
		return false, err
	}
	add("recommendations linked to alerts", "> 0", fmt.Sprintf("%d", linkN), linkN > 0)

	payloadN, err := q.CountRecommendationsWithNonEmptyPayloadsBySellerAccountID(ctx, opts.SellerAccountID)
	if err != nil {
		return false, err
	}
	add("supporting_metrics + constraints non-empty", "> 0 rows", fmt.Sprintf("%d", payloadN), payloadN > 0)

	rawN, err := q.CountRecommendationsWithRawAIBySellerAccountID(ctx, opts.SellerAccountID)
	if err != nil {
		return false, err
	}
	add("raw_ai_response stored (admin)", "> 0 rows", fmt.Sprintf("%d", rawN), rawN > 0)

	recs, err := svc.ListRecommendations(ctx, opts.SellerAccountID, recommendations.ListFilter{Limit: 10})
	if err != nil {
		return false, err
	}
	pubOK := false
	pubDetail := "no recommendations to sample"
	if len(recs) > 0 {
		pubOK = true
		for _, rec := range recs {
			m, err := handlers.MapPublicRecommendationJSON(rec)
			if err != nil {
				pubOK = false
				pubDetail = fmt.Sprintf("marshal id=%d: %v", rec.ID, err)
				break
			}
			if _, has := m["raw_ai_response"]; has {
				pubOK = false
				pubDetail = fmt.Sprintf("raw_ai_response leaked on id=%d", rec.ID)
				break
			}
		}
		if pubOK {
			pubDetail = fmt.Sprintf("checked n=%d", len(recs))
		}
	}
	add("public JSON omits raw_ai_response", "no key on sampled rows", pubDetail, pubOK)

	printMVPRecommendationGenTable(rows)
	if !allOK {
		printRecommendationRunDiagnostics(ctx, q, opts.SellerAccountID, runRow.ID)
		fmt.Println()
		fmt.Println("Hints: ensure alerts cover all groups; widen MVP seed or relax validator in internal/recommendations.")
	}
	return allOK, nil
}

func countRecommendationRunDiagnostics(ctx context.Context, q *dbgen.Queries, sellerAccountID, runID int64) (int64, error) {
	rows, err := q.ListRecommendationRunDiagnosticsByRun(ctx, dbgen.ListRecommendationRunDiagnosticsByRunParams{
		SellerAccountID:     sellerAccountID,
		RecommendationRunID: pgtype.Int8{Int64: runID, Valid: true},
		Limit:               10,
		Offset:              0,
	})
	if err != nil {
		return 0, err
	}
	return int64(len(rows)), nil
}

func printRecommendationRunDiagnostics(ctx context.Context, q *dbgen.Queries, sellerAccountID, runID int64) {
	rows, err := q.ListRecommendationRunDiagnosticsByRun(ctx, dbgen.ListRecommendationRunDiagnosticsByRunParams{
		SellerAccountID:     sellerAccountID,
		RecommendationRunID: pgtype.Int8{Int64: runID, Valid: true},
		Limit:               3,
		Offset:              0,
	})
	if err != nil || len(rows) == 0 {
		fmt.Printf("Diagnostics: (none for run_id=%d: %v)\n", runID, err)
		return
	}
	d := rows[0]
	fmt.Println("Latest recommendation_run diagnostics:")
	fmt.Printf("  diagnostics_rows=%d\n", len(rows))
	fmt.Printf("  error_stage=%s error_message=%s\n", textOrDash(d.ErrorStage), textOrDash(d.ErrorMessage))
	if len(d.ValidationResultPayload) > 0 {
		fmt.Printf("  validation_result_payload=%s\n", truncateJSON(string(d.ValidationResultPayload), 800))
	}
	if len(d.RejectedItemsPayload) > 0 {
		fmt.Printf("  rejected_items_payload=%s\n", truncateJSON(string(d.RejectedItemsPayload), 500))
		printRecommendationTypeDiagnosticsHints(string(d.RejectedItemsPayload), string(d.ValidationResultPayload))
	}
	if len(d.ContextPayloadSummary) > 0 {
		fmt.Printf("  context_payload_summary=%s\n", truncateJSON(string(d.ContextPayloadSummary), 500))
	}
	if len(d.RawOpenaiResponse) > 0 {
		fmt.Printf("  raw_openai_response_preview=%s\n", truncateJSON(string(d.RawOpenaiResponse), 600))
	}
}

func textOrDash(t pgtype.Text) string {
	if !t.Valid || strings.TrimSpace(t.String) == "" {
		return "-"
	}
	return strings.TrimSpace(t.String)
}

func printRecommendationTypeDiagnosticsHints(rejectedJSON, validationJSON string) {
	combined := rejectedJSON + validationJSON
	if strings.Contains(combined, "recommendation_type is not allowed") ||
		strings.Contains(combined, "normalized_types") {
		fmt.Println("  hint: check recommendation_type aliases / schema / prompt (canonical list in internal/recommendations/recommendation_types.go)")
	}
	if strings.Contains(validationJSON, "normalized_types_count") && !strings.Contains(validationJSON, `"normalized_types_count":0`) {
		fmt.Println("  note: some recommendation_type values were normalized from AI aliases to canonical types")
	}
}

func truncateJSON(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func printMVPRecommendationGenTable(rows []mvpRecGenRow) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "Recommendation generation\tExpected\tActual\tStatus")
	for _, r := range rows {
		st := "OK"
		if !r.OK {
			st = "FAIL"
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.Check, r.Expected, r.Actual, st)
	}
	_ = w.Flush()
}
