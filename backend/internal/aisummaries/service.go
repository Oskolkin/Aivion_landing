package aisummaries

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicost"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/descriptioncategories"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/openaix"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/recommendations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool                 *pgxpool.Pool
	q                    *dbgen.Queries
	dashboard            *analytics.DashboardService
	ai                   *recommendations.OpenAIClient
	ozonSvc              *ozon.Service
	ozonHTTP             *ozon.Client
	openAIKey            string
	openAIModel          string
	maxInputTokensApprox int
	maxCategoriesPerRun  int
	dailyUsage           aicost.DailyUsageChecker
}

// NewService constructs Service with dependency validation (same deps as worker).
func NewService(
	pool *pgxpool.Pool,
	dashboard *analytics.DashboardService,
	ai *recommendations.OpenAIClient,
	ozonSvc *ozon.Service,
	openAIKey string,
	openAIModel string,
) (*Service, error) {
	return NewServiceFromDeps(ServiceDeps{
		Pool:        pool,
		Dashboard:   dashboard,
		AI:          ai,
		OzonSvc:     ozonSvc,
		OpenAIKey:   openAIKey,
		OpenAIModel: openAIModel,
	})
}

func (s *Service) validateRuntime() error {
	if s == nil {
		return fmt.Errorf("ai summaries service is nil")
	}
	if s.pool == nil {
		return fmt.Errorf("postgres pool is not configured")
	}
	if s.q == nil {
		return fmt.Errorf("db queries are not configured")
	}
	if s.dashboard == nil {
		return fmt.Errorf("dashboard service is not configured")
	}
	if s.ai == nil {
		return fmt.Errorf("openai client is not configured")
	}
	if s.ozonSvc == nil {
		return fmt.Errorf("ozon service is not configured")
	}
	if s.ozonHTTP == nil {
		return fmt.Errorf("ozon http client is not configured")
	}
	if s.openAIKey == "" {
		return ErrOpenAIKeyRequired
	}
	return nil
}

// RunForSellerAfterSync generates account + category summaries for the latest dashboard metric date.
func (s *Service) RunForSellerAfterSync(ctx context.Context, sellerID int64) error {
	if err := s.validateRuntime(); err != nil {
		return err
	}
	if s.dailyUsage != nil {
		if err := s.dailyUsage.CheckSummaryRun(ctx, sellerID); err != nil {
			if errors.Is(err, aicost.ErrDailyLimitExceeded) {
				return nil
			}
			return err
		}
	}

	latest, err := s.q.GetLatestAvailableDashboardMetricDateBySellerAccountID(ctx, sellerID)
	if err != nil {
		return fmt.Errorf("latest metric date: %w", err)
	}
	if !latest.Valid || latest.Time.Year() < 2000 {
		return nil
	}
	metricDate := latest.Time.UTC()

	existingRun, err := s.q.GetAIDailySummaryRunBySellerAndMetricDate(ctx, dbgen.GetAIDailySummaryRunBySellerAndMetricDateParams{
		SellerAccountID: sellerID,
		MetricDate:      latest,
	})
	if err == nil {
		switch existingRun.Status {
		case "completed", "running":
			// Avoid duplicate OpenAI spend for the same seller_account_id + metric_date.
			return nil
		case "failed", "partial":
			_ = s.q.ClearAIDailySummaryRunAccountFK(ctx, dbgen.ClearAIDailySummaryRunAccountFKParams{
				SellerAccountID: sellerID,
				MetricDate:      latest,
			})
			_ = s.q.DeleteAIDailySummariesBySellerMetricDate(ctx, dbgen.DeleteAIDailySummariesBySellerMetricDateParams{
				SellerAccountID: sellerID,
				MetricDate:      latest,
			})
			_ = s.q.DeleteAIDailySummaryRunsBySellerMetricDate(ctx, dbgen.DeleteAIDailySummaryRunsBySellerMetricDateParams{
				SellerAccountID: sellerID,
				MetricDate:      latest,
			})
		default:
			// Unknown / pending: do not delete an in-flight run.
			return nil
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("get ai summary run: %w", err)
	}

	creds, credErr := s.ozonSvc.GetDecryptedCredentials(ctx, sellerID)
	if credErr == nil {
		_, _ = descriptioncategories.SyncGlobalIfStale(ctx, s.pool, s.ozonHTTP, creds.ClientID, creds.APIKey, 24*time.Hour)
	}

	runNotes := []string{}
	noteBytes, _ := json.Marshal(runNotes)

	model := s.openAIModel
	if model == "" {
		model = "gpt-4.1-mini"
	}

	run, err := s.q.CreateAIDailySummaryRun(ctx, dbgen.CreateAIDailySummaryRunParams{
		SellerAccountID:        sellerID,
		MetricDate:             latest,
		Status:                 "running",
		CategoryTotalCount:     0,
		CategoryCompletedCount: 0,
		CategoryFailedCount:    0,
		PromptVersion:          PromptVersion,
		Model:                  pgtype.Text{String: model, Valid: true},
		DataQualityNotes:       noteBytes,
	})
	if err != nil {
		return fmt.Errorf("create ai daily summary run: %w", err)
	}
	runID := run.ID

	var totalIn, totalOut int
	var totalCost float64

	rollup, err := s.q.ListDescriptionCategoryRollupForAISummary(ctx, dbgen.ListDescriptionCategoryRollupForAISummaryParams{
		SellerAccountID: sellerID,
		MetricDate:      latest,
	})
	if err != nil {
		_ = s.failRun(ctx, runID, sellerID, latest, fmt.Errorf("category rollup: %w", err))
		return nil
	}
	catIDs, err := selectCategoryIDsForAISummary(ctx, s.q, sellerID, latest, rollup, s.maxCategoriesPerRun, &runNotes)
	if err != nil {
		_ = s.failRun(ctx, runID, sellerID, latest, fmt.Errorf("select categories: %w", err))
		return nil
	}
	if len(runNotes) > 0 {
		noteBytes, _ = json.Marshal(runNotes)
	}

	var prevAccPtr *dbgen.AiDailySummary
	if prevAcc, err := s.q.GetPreviousAccountAIDailySummary(ctx, dbgen.GetPreviousAccountAIDailySummaryParams{
		SellerAccountID: sellerID,
		MetricDate:      latest,
	}); err == nil && prevAcc.ID.Valid {
		prevAccPtr = &prevAcc
	} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		_ = s.failRun(ctx, runID, sellerID, latest, err)
		return nil
	}

	accCtx, err := BuildAccountContext(ctx, s.q, s.dashboard, sellerID, metricDate, prevAccPtr)
	if err != nil {
		_ = s.failRun(ctx, runID, sellerID, latest, err)
		return nil
	}
	dto, _ := s.dashboard.BuildDashboardMetrics(ctx, sellerID, &metricDate)
	rich := classifyAccountDataRichness(dto)
	accCtxMap := structToMap(accCtx)
	trimRes := trimAccountContextForTokenBudget(systemPromptDailySummary(), userPromptAccountSummary(), accCtxMap, s.maxInputTokensApprox)
	accCtxMap = trimRes.Context
	attachTrimDebugToContext(accCtxMap, trimRes)
	if trimRes.Trimmed {
		runNotes = append(runNotes, fmt.Sprintf("account_context_trim_tier_%d", trimRes.TrimTier))
		noteBytes, _ = json.Marshal(runNotes)
	}
	accCtxMap["inferred_data_richness_hint"] = rich

	prevUUID := pgtype.UUID{}
	if prevAccPtr != nil && prevAccPtr.ID.Valid {
		prevUUID = prevAccPtr.ID
	}

	accRow, err := s.q.InsertAIDailySummary(ctx, dbgen.InsertAIDailySummaryParams{
		RunID:             runID,
		SellerAccountID:   sellerID,
		MetricDate:        latest,
		SummaryType:       "account_daily_summary",
		ScopeType:         "account",
		ScopeID:           "",
		ScopeName:         pgtype.Text{},
		Status:            "running",
		PreviousSummaryID: prevUUID,
		PromptVersion:     PromptVersion,
		Model:             pgtype.Text{String: model, Valid: true},
	})
	if err != nil {
		_ = s.failRun(ctx, runID, sellerID, latest, err)
		return nil
	}

	out, genErr := s.callOpenAI(ctx, systemPromptDailySummary(), userPromptAccountSummary(), accCtxMap)
	if genErr != nil && errors.Is(genErr, recommendations.ErrOpenAIRequestTooLarge) {
		recoveryLimit := s.maxInputTokensApprox - 4000
		if recoveryLimit < 12000 {
			recoveryLimit = 12000
		}
		minimalRes := trimAccountContextForTokenBudget(systemPromptDailySummary(), userPromptAccountSummary(), accCtxMap, recoveryLimit)
		accCtxMap = minimalRes.Context
		attachTrimDebugToContext(accCtxMap, minimalRes)
		runNotes = append(runNotes, "account_context_retry_after_token_budget")
		noteBytes, _ = json.Marshal(runNotes)
		out, genErr = s.callOpenAI(ctx, systemPromptDailySummary(), userPromptAccountSummary(), accCtxMap)
	}
	if genErr != nil {
		_ = s.failSummary(ctx, accRow.ID, sellerID, genErr)
		_ = s.failRun(ctx, runID, sellerID, latest, genErr)
		return nil
	}
	totalIn += out.InputTokens
	totalOut += out.OutputTokens
	totalCost += aicost.EstimateUSD(out.Model, out.InputTokens, out.OutputTokens).CostUSD

	if truncated, truncMsg := openAIOutputTruncated(out); truncated {
		recordSummaryOutcome("failed", SummaryRepairResult{}, false, true)
		_ = s.failSummary(ctx, accRow.ID, sellerID, fmt.Errorf("%s", truncMsg))
		_ = s.failRun(ctx, runID, sellerID, latest, fmt.Errorf("%s", truncMsg))
		return nil
	}
	acceptOut, perr := AcceptSummaryJSON(out.Content, SummaryRepairContext{SummaryType: "account_daily_summary"})
	MaybeDumpFunnelAIDevFiles(out.Content, &ValidationOutcome{Payload: acceptOut.Payload, ValidationErrors: append(acceptOut.Repair.HardErrors, acceptOut.Repair.RepairableErrors...)})
	if perr != nil {
		_ = s.failSummary(ctx, accRow.ID, sellerID, perr)
		_ = s.failRun(ctx, runID, sellerID, latest, perr)
		return nil
	}
	inCtxBytes, _ := json.Marshal(accCtx)
	if acceptOut == nil || !acceptOut.Accepted {
		ve, _ := json.Marshal(append(acceptOut.Repair.HardErrors, acceptOut.Repair.RepairableErrors...))
		_, _ = s.q.FailAIDailySummary(ctx, dbgen.FailAIDailySummaryParams{
			ID:               accRow.ID,
			SellerAccountID:  sellerID,
			Status:           "failed",
			ErrorMessage:     pgtype.Text{String: firstSummaryErrorMessage(acceptOut), Valid: true},
			ValidationErrors: ve,
		})
		_ = s.q.UpdateAIDailySummaryRun(ctx, dbgen.UpdateAIDailySummaryRunParams{
			ID:                     runID,
			SellerAccountID:        sellerID,
			Status:                 "failed",
			AccountSummaryID:       pgtype.UUID{},
			CategoryTotalCount:     int32(len(catIDs)),
			CategoryCompletedCount: 0,
			CategoryFailedCount:    int32(len(catIDs)),
			EstimatedInputTokens:   i32p(totalIn),
			EstimatedOutputTokens:  i32p(totalOut),
			EstimatedCost:          mustNumeric(totalCost),
			ErrorMessage:           pgtype.Text{String: "account_summary_validation_failed", Valid: true},
			DataQualityNotes:       noteBytes,
			CompletedAt:            pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		})
		return nil
	}
	p := *acceptOut.Payload
	validationNotes := encodeStoredValidationNotes(acceptOut.ValidationStatus, acceptOut.Repair)
	sumBytes, _ := json.Marshal(p)
	alB, _ := json.Marshal(filterIntIDs(p.LinkedAlertIDs, 40))
	rB, _ := json.Marshal(filterIntIDs(p.LinkedRecommendationIDs, 40))
	costA := aicost.EstimateUSD(out.Model, out.InputTokens, out.OutputTokens).CostUSD

	_, _ = s.q.CompleteAIDailySummary(ctx, dbgen.CompleteAIDailySummaryParams{
		ID:                      accRow.ID,
		SellerAccountID:         sellerID,
		Status:                  "completed",
		HealthScore:             pgtype.Int4{Int32: int32(p.HealthScore), Valid: true},
		Severity:                pgtype.Text{String: p.Severity, Valid: true},
		DataRichness:            pgtype.Text{String: p.DataRichness, Valid: true},
		SummaryDepth:            pgtype.Text{String: p.SummaryDepth, Valid: true},
		Headline:                pgtype.Text{String: p.Headline, Valid: true},
		ExecutiveSummary:        pgtype.Text{String: p.ExecutiveSummary, Valid: true},
		SummaryJson:             sumBytes,
		InputContextJson:        truncateJSONForStore(inCtxBytes, 256*1024),
		LinkedAlertIds:          alB,
		LinkedRecommendationIds: rB,
		EstimatedInputTokens:    i32p(out.InputTokens),
		EstimatedOutputTokens:   i32p(out.OutputTokens),
		EstimatedCost:           mustNumeric(costA),
		ValidationErrors:        validationNotes,
		ErrorMessage:            pgtype.Text{},
	})

	_ = s.q.UpdateAIDailySummaryRun(ctx, dbgen.UpdateAIDailySummaryRunParams{
		ID:                     runID,
		SellerAccountID:        sellerID,
		Status:                 "running",
		AccountSummaryID:       accRow.ID,
		CategoryTotalCount:     int32(len(catIDs)),
		CategoryCompletedCount: 0,
		CategoryFailedCount:    0,
		EstimatedInputTokens:   i32p(totalIn),
		EstimatedOutputTokens:  i32p(totalOut),
		EstimatedCost:          mustNumeric(totalCost),
		ErrorMessage:           pgtype.Text{},
		DataQualityNotes:       noteBytes,
		CompletedAt:            pgtype.Timestamptz{},
	})

	catOK := 0
	catFail := 0
	catDiag := newCategoryRunDiagnostics(sellerID)
	for _, cid := range catIDs {
		name := categoryName(ctx, s.q, cid)
		if strings.TrimSpace(name) == "" {
			name = "Категория без названия"
		}
		skus, err := s.q.ListProductSKUMetricsForCategoryAndDate(ctx, dbgen.ListProductSKUMetricsForCategoryAndDateParams{
			SellerAccountID:       sellerID,
			DescriptionCategoryID: pgtype.Int8{Int64: cid, Valid: true},
			MetricDate:            latest,
		})
		if err != nil {
			catFail++
			catDiag.recordErr(cid, name, categoryFailLoadSKUs, err)
			continue
		}
		var prevCatPtr *dbgen.AiDailySummary
		if prevCat, err := s.q.GetPreviousCategoryAIDailySummary(ctx, dbgen.GetPreviousCategoryAIDailySummaryParams{
			SellerAccountID: sellerID,
			ScopeID:         strconv.FormatInt(cid, 10),
			MetricDate:      latest,
		}); err == nil && prevCat.ID.Valid {
			prevCatPtr = &prevCat
		}

		catPkg := BuildCategoryContext(ctx, s.q, sellerID, cid, name, metricDate, prevCatPtr, skus)
		alertsN := len(catPkg.AlertsSample)
		recsN := len(catPkg.RecommendationsSample)
		revCat := 0.0
		var totalStock int32
		for _, r := range skus {
			revCat += numericToFloat(r.Revenue)
			totalStock += r.StockAvailable
		}
		cRich := classifyCategoryDataRichness(len(skus), revCat, alertsN, recsN, totalStock)
		m := structToMap(catPkg)
		m["inferred_data_richness_hint"] = cRich
		m["category_id"] = cid
		m["category_name"] = name

		prevCatUUID := pgtype.UUID{}
		if prevCatPtr != nil && prevCatPtr.ID.Valid {
			prevCatUUID = prevCatPtr.ID
		}

		row, err := s.q.InsertAIDailySummary(ctx, dbgen.InsertAIDailySummaryParams{
			RunID:             runID,
			SellerAccountID:   sellerID,
			MetricDate:        latest,
			SummaryType:       "category_daily_summary",
			ScopeType:         "category",
			ScopeID:           strconv.FormatInt(cid, 10),
			ScopeName:         pgtype.Text{String: name, Valid: true},
			Status:            "running",
			PreviousSummaryID: prevCatUUID,
			PromptVersion:     PromptVersion,
			Model:             pgtype.Text{String: model, Valid: true},
		})
		if err != nil {
			catFail++
			catDiag.recordErr(cid, name, categoryFailSave, err)
			continue
		}

		catUserPrompt := categorySummaryUserPrompt(cid, name, cRich)
		outC, errC := s.callOpenAI(ctx, systemPromptDailySummary(), catUserPrompt, m)
		if errC != nil {
			catDiag.recordErr(cid, name, categoryFailOpenAI, errC)
			_, _ = s.q.FailAIDailySummary(ctx, dbgen.FailAIDailySummaryParams{
				ID:               row.ID,
				SellerAccountID:  sellerID,
				Status:           "failed",
				ErrorMessage:     pgtype.Text{String: errC.Error(), Valid: true},
				ValidationErrors: nil,
			})
			catFail++
			continue
		}
		totalIn += outC.InputTokens
		totalOut += outC.OutputTokens
		costC := aicost.EstimateUSD(outC.Model, outC.InputTokens, outC.OutputTokens).CostUSD
		totalCost += costC
		scopeID := strconv.FormatInt(cid, 10)
		acceptCat, perr := AcceptSummaryJSON(outC.Content, SummaryRepairContext{
			SummaryType:      "category_daily_summary",
			ScopeID:          scopeID,
			CategoryName:     name,
			InferredCategory: cRich,
		})
		if perr != nil {
			catDiag.recordErr(cid, name, categoryFailValidationJSON, perr)
			_, _ = s.q.FailAIDailySummary(ctx, dbgen.FailAIDailySummaryParams{
				ID:               row.ID,
				SellerAccountID:  sellerID,
				Status:           "failed",
				ErrorMessage:     pgtype.Text{String: perr.Error(), Valid: true},
				ValidationErrors: nil,
			})
			catFail++
			continue
		}
		inCatBytes, _ := json.Marshal(catPkg)
		if acceptCat == nil || !acceptCat.Accepted {
			ve, _ := json.Marshal(append(acceptCat.Repair.HardErrors, acceptCat.Repair.RepairableErrors...))
			catDiag.record(cid, name, categoryFailValidation, truncateDiagText(string(ve), 400))
			_, _ = s.q.FailAIDailySummary(ctx, dbgen.FailAIDailySummaryParams{
				ID:               row.ID,
				SellerAccountID:  sellerID,
				Status:           "failed",
				ErrorMessage:     pgtype.Text{String: firstSummaryErrorMessage(acceptCat), Valid: true},
				ValidationErrors: ve,
			})
			catFail++
			continue
		}
		pc := *acceptCat.Payload
		catValidationNotes := encodeStoredValidationNotes(acceptCat.ValidationStatus, acceptCat.Repair)
		sumB, _ := json.Marshal(pc)
		alBC, _ := json.Marshal(filterIntIDs(pc.LinkedAlertIDs, 40))
		rBC, _ := json.Marshal(filterIntIDs(pc.LinkedRecommendationIDs, 40))
		_, _ = s.q.CompleteAIDailySummary(ctx, dbgen.CompleteAIDailySummaryParams{
			ID:                      row.ID,
			SellerAccountID:         sellerID,
			Status:                  "completed",
			HealthScore:             pgtype.Int4{Int32: int32(pc.HealthScore), Valid: true},
			Severity:                pgtype.Text{String: pc.Severity, Valid: true},
			DataRichness:            pgtype.Text{String: pc.DataRichness, Valid: true},
			SummaryDepth:            pgtype.Text{String: pc.SummaryDepth, Valid: true},
			Headline:                pgtype.Text{String: pc.Headline, Valid: true},
			ExecutiveSummary:        pgtype.Text{String: pc.ExecutiveSummary, Valid: true},
			SummaryJson:             sumB,
			InputContextJson:        truncateJSONForStore(inCatBytes, 200*1024),
			LinkedAlertIds:          alBC,
			LinkedRecommendationIds: rBC,
			EstimatedInputTokens:    i32p(outC.InputTokens),
			EstimatedOutputTokens:   i32p(outC.OutputTokens),
			EstimatedCost:           mustNumeric(costC),
			ValidationErrors:        catValidationNotes,
			ErrorMessage:            pgtype.Text{},
		})
		catOK++
	}

	logSummaryMetrics()
	runStatus := "completed"
	runErrMsg := pgtype.Text{}
	if catFail > 0 {
		runStatus = "partial"
		if agg := catDiag.aggregatePartialRunError(catFail, len(catIDs)); agg != "" {
			runErrMsg = pgtype.Text{String: agg, Valid: true}
		}
	}
	_ = s.q.UpdateAIDailySummaryRun(ctx, dbgen.UpdateAIDailySummaryRunParams{
		ID:                     runID,
		SellerAccountID:        sellerID,
		Status:                 runStatus,
		AccountSummaryID:       accRow.ID,
		CategoryTotalCount:     int32(len(catIDs)),
		CategoryCompletedCount: int32(catOK),
		CategoryFailedCount:    int32(catFail),
		EstimatedInputTokens:   i32p(totalIn),
		EstimatedOutputTokens:  i32p(totalOut),
		EstimatedCost:          mustNumeric(totalCost),
		ErrorMessage:           runErrMsg,
		DataQualityNotes:       noteBytes,
		CompletedAt:            pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})
	return nil
}

func (s *Service) failRun(ctx context.Context, runID pgtype.UUID, seller int64, _ pgtype.Date, cause error) error {
	msg := cause.Error()
	var ho *openaix.HTTPOutageError
	if errors.As(cause, &ho) {
		msg = fmt.Sprintf("openai_http_%d: %s", ho.StatusCode, ho.Body)
	}
	_ = s.q.UpdateAIDailySummaryRun(ctx, dbgen.UpdateAIDailySummaryRunParams{
		ID:                     runID,
		SellerAccountID:        seller,
		Status:                 "failed",
		AccountSummaryID:       pgtype.UUID{},
		CategoryTotalCount:     0,
		CategoryCompletedCount: 0,
		CategoryFailedCount:    0,
		EstimatedInputTokens:   pgtype.Int4{},
		EstimatedOutputTokens:  pgtype.Int4{},
		EstimatedCost:          pgtype.Numeric{},
		ErrorMessage:           pgtype.Text{String: msg, Valid: true},
		DataQualityNotes:       []byte("[]"),
		CompletedAt:            pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})
	return nil
}

func (s *Service) failSummary(ctx context.Context, id pgtype.UUID, seller int64, cause error) error {
	_, _ = s.q.FailAIDailySummary(ctx, dbgen.FailAIDailySummaryParams{
		ID:               id,
		SellerAccountID:  seller,
		Status:           "failed",
		ErrorMessage:     pgtype.Text{String: cause.Error(), Valid: true},
		ValidationErrors: nil,
	})
	return nil
}

func (s *Service) callOpenAI(ctx context.Context, sys, user string, payload any) (*recommendations.GenerateRecommendationsOutput, error) {
	if s.ai == nil {
		return nil, fmt.Errorf("openai client is not configured")
	}
	model := s.openAIModel
	if model == "" {
		model = "gpt-4.1-mini"
	}
	out, err := s.ai.GenerateStructuredJSON(ctx, recommendations.GenerateStructuredJSONInput{
		SystemPrompt: sys,
		UserPrompt:   user,
		Context:      payload,
	})
	if err != nil {
		return nil, fmt.Errorf("openai request failed (model=%s): %w", model, err)
	}
	if out == nil {
		return nil, fmt.Errorf("openai returned nil result without error (model=%s)", model)
	}
	return out, nil
}

func categoryName(ctx context.Context, q *dbgen.Queries, id int64) string {
	row, err := q.GetDescriptionCategoryByID(ctx, id)
	if err != nil {
		return ""
	}
	return row.CategoryName
}

func structToMap(v any) map[string]any {
	b, err := json.Marshal(v)
	if err != nil {
		return map[string]any{"marshal_error": true}
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return map[string]any{"unmarshal_error": true}
	}
	return m
}

func i32p(v int) pgtype.Int4 {
	if v == 0 {
		return pgtype.Int4{}
	}
	if v > math.MaxInt32 {
		v = math.MaxInt32
	}
	if v < math.MinInt32 {
		v = math.MinInt32
	}
	return pgtype.Int4{Int32: int32(v), Valid: true}
}

func mustNumeric(f float64) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(fmt.Sprintf("%.6f", f))
	return n
}

func filterIntIDs(ids []int64, max int) []int64 {
	if len(ids) == 0 {
		return ids
	}
	if len(ids) > max {
		return ids[:max]
	}
	return ids
}

func openAIOutputTruncated(out *recommendations.GenerateRecommendationsOutput) (bool, string) {
	if out == nil {
		return false, ""
	}
	if strings.EqualFold(out.IncompleteReason, "length") || strings.EqualFold(out.ResponseStatus, "incomplete") {
		return true, "openai_output_incomplete_length"
	}
	return false, ""
}

func attachTrimDebugToContext(ctx map[string]any, trim accountContextTrimResult) {
	if ctx == nil {
		return
	}
	ctx["token_budget_debug"] = map[string]any{
		"context_trimmed":           trim.Trimmed,
		"trim_tier":                 trim.TrimTier,
		"original_estimated_tokens": trim.OriginalTokens,
		"final_estimated_tokens":    trim.FinalTokens,
		"dropped_sections":          trim.DroppedSections,
		"sku_candidates_before":     trim.SKUCandidatesBefore,
		"sku_candidates_after":      trim.SKUCandidatesAfter,
	}
	if coverage, ok := ctx["sku_coverage"].(map[string]any); ok {
		coverage["trim_tier"] = trim.TrimTier
		coverage["context_trimmed"] = trim.Trimmed
	}
}

func firstSummaryErrorMessage(outcome *SummaryAcceptanceOutcome) string {
	if outcome == nil {
		return "validation_failed"
	}
	if len(outcome.Repair.HardErrors) > 0 {
		return outcome.Repair.HardErrors[0]
	}
	if len(outcome.Repair.RepairableErrors) > 0 {
		return outcome.Repair.RepairableErrors[0]
	}
	return "validation_failed"
}
