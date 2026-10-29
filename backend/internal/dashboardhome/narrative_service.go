package dashboardhome

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicost"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/alerts"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/openaix"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/recommendations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type NarrativeService struct {
	pool        *pgxpool.Pool
	q           *dbgen.Queries
	dashboard   *analytics.DashboardService
	advertising *analytics.AdvertisingService
	alerts      *alerts.Service
	ai          *recommendations.OpenAIClient
	dataPack    *aidatapack.Service
	openAIKey   string
	openAIModel string
	dailyUsage  aicost.DailyUsageChecker
}

type NarrativeServiceDeps struct {
	Pool        *pgxpool.Pool
	Dashboard   *analytics.DashboardService
	Advertising *analytics.AdvertisingService
	Alerts      *alerts.Service
	AI          *recommendations.OpenAIClient
	DataPack    *aidatapack.Service
	OpenAIKey   string
	OpenAIModel string
	DailyUsage  aicost.DailyUsageChecker
}

func NewNarrativeServiceFromDeps(d NarrativeServiceDeps) (*NarrativeService, error) {
	if d.Pool == nil {
		return nil, fmt.Errorf("postgres pool is required for dashboard narrative service")
	}
	if d.Dashboard == nil {
		return nil, fmt.Errorf("dashboard service is required for dashboard narrative service")
	}
	if d.Advertising == nil {
		return nil, fmt.Errorf("advertising service is required for dashboard narrative service")
	}
	if d.Alerts == nil {
		return nil, fmt.Errorf("alerts service is required for dashboard narrative service")
	}
	if d.AI == nil {
		return nil, fmt.Errorf("openai client is required for dashboard narrative service")
	}
	if strings.TrimSpace(d.OpenAIKey) == "" {
		return nil, fmt.Errorf("openai api key is required for dashboard narrative service")
	}
	dataPack := d.DataPack
	if dataPack == nil {
		dataPack = aidatapack.NewService(d.Pool)
	}
	return &NarrativeService{
		pool:        d.Pool,
		q:           dbgen.New(d.Pool),
		dashboard:   d.Dashboard,
		advertising: d.Advertising,
		alerts:      d.Alerts,
		ai:          d.AI,
		dataPack:    dataPack,
		openAIKey:   strings.TrimSpace(d.OpenAIKey),
		openAIModel: strings.TrimSpace(d.OpenAIModel),
		dailyUsage:  d.DailyUsage,
	}, nil
}

func BuildNarrativeService(cfg *config.Config, pool *pgxpool.Pool) (*NarrativeService, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}
	if pool == nil {
		return nil, fmt.Errorf("postgres pool is required")
	}
	openAIClient, err := recommendations.NewOpenAIClient(recommendations.OpenAIClientConfig{
		APIKey:               cfg.OpenAI.APIKey,
		Model:                cfg.OpenAI.Model,
		TimeoutSeconds:       cfg.OpenAI.TimeoutSeconds,
		MaxRetries:           cfg.OpenAI.MaxRetries,
		ProxyURL:             cfg.OpenAI.AIHTTPSProxyURL(),
		MaxInputTokensApprox: cfg.AI.OpenAIMaxInputTokensApprox(),
		MaxOutputTokens:      dashboardNarrativeMaxOutputTokens(cfg),
	})
	if err != nil {
		return nil, fmt.Errorf("openai client: %w", err)
	}
	return NewNarrativeServiceFromDeps(NarrativeServiceDeps{
		Pool:        pool,
		Dashboard:   analytics.NewDashboardService(pool),
		Advertising: analytics.NewAdvertisingService(pool),
		Alerts:      alerts.NewService(alerts.NewSQLCRepository(dbgen.New(pool))),
		AI:          openAIClient,
		DataPack:    aidatapack.NewService(pool),
		OpenAIKey:   cfg.OpenAI.APIKey,
		OpenAIModel: cfg.OpenAI.Model,
		DailyUsage: aicost.NewDailyUsageGuard(dbgen.New(pool), aicost.DailyLimitsConfig{
			ChatMaxQuestionsPerDay:      cfg.AI.ChatMaxQuestionsPerDay,
			RecommendationMaxRunsPerDay: cfg.AI.RecommendationMaxRunsPerDay,
			SummaryMaxRunsPerDay:        cfg.AI.SummaryMaxRunsPerDay,
			MaxInputTokensPerRequest:    cfg.AI.MaxInputTokensPerRequest,
			MaxOutputTokensPerRequest:   cfg.AI.MaxOutputTokensPerRequest,
		}),
	})
}

func (s *NarrativeService) validateRuntime() error {
	if s == nil {
		return fmt.Errorf("dashboard narrative service is nil")
	}
	if s.pool == nil || s.q == nil {
		return fmt.Errorf("postgres is not configured")
	}
	if s.dashboard == nil {
		return fmt.Errorf("dashboard service is not configured")
	}
	if s.advertising == nil {
		return fmt.Errorf("advertising service is not configured")
	}
	if s.alerts == nil {
		return fmt.Errorf("alerts service is not configured")
	}
	if s.ai == nil {
		return fmt.Errorf("openai client is not configured")
	}
	if s.openAIKey == "" {
		return fmt.Errorf("openai api key is required")
	}
	return nil
}

// OpenAIClient exposes the configured OpenAI client for devtools.
func (s *NarrativeService) OpenAIClient() *recommendations.OpenAIClient {
	if s == nil {
		return nil
	}
	return s.ai
}

// AnalyzeDataPackChunks runs standalone chunk analysis for a pre-built Data Pack and ChunkSet.
func (s *NarrativeService) AnalyzeDataPackChunks(
	ctx context.Context,
	pack *aidatapack.SellerAIDataPack,
	chunkSet *aidatapack.ChunkSet,
	opts NarrativeRunOptions,
) (*ChunkAnalysisRunResult, error) {
	if pack == nil {
		return nil, errors.New("seller AI data pack is nil")
	}
	if chunkSet == nil {
		return nil, errors.New("chunk set is nil")
	}

	orch := &DataPackChunksOrchestrator{
		AI:    s.ai,
		Model: s.openAIModel,
		Debug: NewNarrativeDebugRecorder(opts.DebugOutputDir),
	}
	result, err := orch.AnalyzeChunksOnly(ctx, pack, chunkSet)
	if err != nil {
		return nil, fmt.Errorf("analyze data pack chunks: %w", err)
	}
	return &result, nil
}

func (s *NarrativeService) RunForSellerAfterSync(ctx context.Context, sellerAccountID int64) error {
	result, err := s.RunForSellerAfterSyncDetailed(ctx, sellerAccountID)
	if err != nil {
		return err
	}
	if result != nil && !result.IsSuccess() {
		slog.Warn("dashboard narrative generation finished with non-success status",
			"seller_account_id", sellerAccountID,
			"run_id", result.RunID,
			"status", result.Status,
			"metric_date", result.MetricDate,
			"error_message", result.ErrorMessage,
			"validation_errors", result.ValidationErrors,
		)
	}
	return nil
}

func (s *NarrativeService) RunForSellerAfterSyncDetailed(ctx context.Context, sellerAccountID int64) (*NarrativeGenerationResult, error) {
	return s.RunForSellerAfterSyncDetailedWithOptions(ctx, sellerAccountID, NarrativeRunOptions{})
}

func (s *NarrativeService) RunForSellerAfterSyncDetailedWithOptions(ctx context.Context, sellerAccountID int64, opts NarrativeRunOptions) (*NarrativeGenerationResult, error) {
	if err := s.validateRuntime(); err != nil {
		return nil, err
	}
	if opts.MetricDate != nil {
		return s.RunForSellerMetricDateWithOptions(ctx, sellerAccountID, *opts.MetricDate, opts)
	}
	if s.dailyUsage != nil {
		if err := s.dailyUsage.CheckSummaryRun(ctx, sellerAccountID); err != nil {
			if errors.Is(err, aicost.ErrDailyLimitExceeded) {
				return &NarrativeGenerationResult{Status: "skipped", Skipped: true, ErrorMessage: "daily_limit_exceeded"}, nil
			}
			return nil, err
		}
	}

	latest, err := s.q.GetLatestAvailableDashboardMetricDateBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		return nil, fmt.Errorf("dashboard narrative latest metric date: %w", err)
	}
	if !latest.Valid || latest.Time.Year() < 2000 {
		return &NarrativeGenerationResult{Status: "skipped", Skipped: true, ErrorMessage: "no_metric_date"}, nil
	}
	metricDate := latest.Time.UTC()
	return s.RunForSellerMetricDateWithOptions(ctx, sellerAccountID, metricDate, opts)
}

func (s *NarrativeService) RunForSellerMetricDateWithOptions(
	ctx context.Context,
	sellerAccountID int64,
	metricDate time.Time,
	opts NarrativeRunOptions,
) (*NarrativeGenerationResult, error) {
	if err := s.validateRuntime(); err != nil {
		return nil, err
	}
	if opts.MetricDate == nil {
		if s.dailyUsage != nil {
			if err := s.dailyUsage.CheckSummaryRun(ctx, sellerAccountID); err != nil {
				if errors.Is(err, aicost.ErrDailyLimitExceeded) {
					return &NarrativeGenerationResult{Status: "skipped", Skipped: true, ErrorMessage: "daily_limit_exceeded"}, nil
				}
				return nil, err
			}
		}
	}
	metricDate = metricDate.UTC()

	build, score, severity, err := s.buildNarrativeContextForSellerWithOptions(ctx, sellerAccountID, metricDate, opts)
	if err != nil {
		return nil, err
	}

	existing, existingFound, err := s.lookupDashboardNarrativeForGeneration(ctx, sellerAccountID, metricDate, opts)
	if err != nil {
		return nil, err
	}
	if existingFound {
		if existing.Status == "running" {
			return narrativeGenerationResultFromRow(existing, metricDate, &build, true), nil
		}
		if shouldReuseExistingDashboardNarrative(existing, opts, build) {
			return narrativeGenerationResultFromRow(existing, metricDate, &build, true), nil
		}
	}

	model := s.openAIModel
	if model == "" {
		model = "gpt-4.1-mini"
	}

	var row dbgen.AiDashboardNarrative
	if existingFound && s.shouldRetryDashboardNarrativeOnSameRow(existing, opts) {
		row, err = s.q.ResetAIDashboardNarrativeForRetry(ctx, dbgen.ResetAIDashboardNarrativeForRetryParams{
			ID:              existing.ID,
			SellerAccountID: sellerAccountID,
		})
		if err != nil {
			return nil, fmt.Errorf("reset dashboard narrative for retry: %w", err)
		}
	} else {
		row, err = s.q.InsertAIDashboardNarrative(ctx, dbgen.InsertAIDashboardNarrativeParams{
			SellerAccountID: sellerAccountID,
			MetricDate:      dateValue(metricDate),
			Status:          "running",
			HealthScore:     pgtype.Int4{Int32: score, Valid: true},
			Severity:        pgtype.Text{String: severity, Valid: severity != ""},
			RiskDriversHash: build.ContextHash,
			ContextHash:     build.FactualContextHash,
			PromptVersion:   DashboardNarrativePromptVersion,
			Model:           pgtype.Text{String: model, Valid: true},
			StartedAt:       pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		})
		if err != nil {
			return nil, fmt.Errorf("insert dashboard narrative: %w", err)
		}
		if err := s.linkDashboardNarrativeToPipeline(ctx, row.ID, sellerAccountID, opts); err != nil {
			return nil, err
		}
	}

	recordNarrativeGenerated()

	outcome, err := s.generateDashboardNarrativePayload(ctx, sellerAccountID, metricDate, build, model, opts)
	if err != nil {
		status := narrativeFailureStatus(err)
		if build.ContextSource == ContextSourceSellerAIDataPackChunks && status == "fallback" {
			status = "failed"
		}
		_, _ = s.q.FailAIDashboardNarrative(ctx, dbgen.FailAIDashboardNarrativeParams{
			ID:               row.ID,
			SellerAccountID:  sellerAccountID,
			Status:           status,
			RawAiResponse:    pgtype.Text{},
			ValidationErrors: []byte("[]"),
			InputTokens:      pgtype.Int4{},
			OutputTokens:     pgtype.Int4{},
			EstimatedCost:    pgtype.Numeric{},
			ErrorMessage:     pgtype.Text{String: err.Error(), Valid: true},
			FinishedAt:       pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		})
		result := narrativeGenerationResultFromInsert(row, metricDate, &build, status, err.Error(), nil, 0, 0)
		if code := NarrativeErrorCode(err); code != "" {
			result.ErrorCode = code
		}
		applyAIOnlyRequiredBlocksMeta(result, nil)
		if outcome != nil {
			result.InputTokens = outcome.meta.InputTokens
			result.OutputTokens = outcome.meta.OutputTokens
			enrichGenerationResultFromOutcome(result, &build, outcome)
		}
		if outcome != nil && outcome.debug != nil && outcome.debug.Enabled() {
			_ = outcome.debug.WriteRunSummary(result, outcome.pipeline)
		}
		recordNarrativeGenerationOutcome(status, nil)
		return result, nil
	}

	aiMeta := outcome.meta
	if aiMeta.IncompleteReason == "max_output_tokens" || strings.EqualFold(aiMeta.ResponseStatus, "incomplete") {
		msg := "openai_output_incomplete_length"
		_, _ = s.q.FailAIDashboardNarrative(ctx, dbgen.FailAIDashboardNarrativeParams{
			ID:               row.ID,
			SellerAccountID:  sellerAccountID,
			Status:           "failed",
			RawAiResponse:    pgtype.Text{String: aiMeta.Content, Valid: aiMeta.Content != ""},
			ValidationErrors: []byte(`["openai_output_incomplete_length"]`),
			InputTokens:      narrativeInt4(aiMeta.InputTokens),
			OutputTokens:     narrativeInt4(aiMeta.OutputTokens),
			EstimatedCost:    narrativeNumeric(aicost.EstimateUSD(aiMeta.Model, aiMeta.InputTokens, aiMeta.OutputTokens).CostUSD),
			ErrorMessage:     pgtype.Text{String: msg, Valid: true},
			FinishedAt:       pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		})
		result := narrativeGenerationResultFromInsert(row, metricDate, &build, "failed", msg, []string{"openai_output_incomplete_length"}, aiMeta.InputTokens, aiMeta.OutputTokens)
		applyAIOnlyRequiredBlocksMeta(result, nil)
		recordNarrativeGenerationOutcome("failed", []string{"openai_output_incomplete_length"})
		return result, nil
	}

	var payload *dashboardNarrativePayload
	var jsonRepairs []string
	var parseErr error
	if outcome.payload != nil {
		payload = outcome.payload
	} else {
		payload, jsonRepairs, parseErr = parseDashboardNarrativePayloadWithRepair(aiMeta.Content)
	}
	if parseErr != nil {
		errMsg := parseErr.Error()
		if aiMeta.IncompleteReason == "max_output_tokens" {
			errMsg = "openai_output_incomplete_length"
		}
		_, _ = s.q.FailAIDashboardNarrative(ctx, dbgen.FailAIDashboardNarrativeParams{
			ID:               row.ID,
			SellerAccountID:  sellerAccountID,
			Status:           "failed",
			RawAiResponse:    pgtype.Text{String: aiMeta.Content, Valid: true},
			ValidationErrors: []byte(`["invalid_json"]`),
			InputTokens:      narrativeInt4(aiMeta.InputTokens),
			OutputTokens:     narrativeInt4(aiMeta.OutputTokens),
			EstimatedCost:    narrativeNumeric(aicost.EstimateUSD(aiMeta.Model, aiMeta.InputTokens, aiMeta.OutputTokens).CostUSD),
			ErrorMessage:     pgtype.Text{String: errMsg, Valid: true},
			FinishedAt:       pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		})
		result := narrativeGenerationResultFromInsert(row, metricDate, &build, "failed", errMsg, []string{"invalid_json"}, aiMeta.InputTokens, aiMeta.OutputTokens)
		applyAIOnlyRequiredBlocksMeta(result, nil)
		recordNarrativeGenerationOutcome("failed", []string{"invalid_json"})
		return result, nil
	}

	accepted, repairedPayload, repair, acceptanceValidation := acceptGeneratedDashboardNarrativeWithValidation(payload, build)
	repair.RepairActions = append(repair.RepairActions, jsonRepairs...)
	if len(jsonRepairs) > 0 {
		repair.Repaired = true
	}
	validationResult := acceptanceValidation
	if outcome != nil && (outcome.dashboardValidation.Mode != "" || len(outcome.dashboardValidation.Warnings) > 0) {
		validationResult = mergeDashboardValidationResults(acceptanceValidation, outcome.dashboardValidation)
	}
	if !accepted {
		validationErrors := append(append([]string{}, validationResult.BlockingCodes()...), repair.RepairableErrors...)
		validationBytes, _ := json.Marshal(validationErrors)
		errMsg := narrativeGenerationFailureMessage(parseErr, &aiMeta, validationResult.BlockingCodes())
		_, _ = s.q.FailAIDashboardNarrative(ctx, dbgen.FailAIDashboardNarrativeParams{
			ID:               row.ID,
			SellerAccountID:  sellerAccountID,
			Status:           "failed",
			RawAiResponse:    pgtype.Text{String: aiMeta.Content, Valid: true},
			ValidationErrors: validationBytes,
			InputTokens:      narrativeInt4(aiMeta.InputTokens),
			OutputTokens:     narrativeInt4(aiMeta.OutputTokens),
			EstimatedCost:    narrativeNumeric(aicost.EstimateUSD(aiMeta.Model, aiMeta.InputTokens, aiMeta.OutputTokens).CostUSD),
			ErrorMessage:     pgtype.Text{String: errMsg, Valid: true},
			FinishedAt:       pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		})
		result := narrativeGenerationResultFromInsert(row, metricDate, &build, "failed", errMsg, validationErrors, aiMeta.InputTokens, aiMeta.OutputTokens)
		applyAIOnlyRequiredBlocksMeta(result, repairedPayload)
		recordNarrativeGenerationOutcome("failed", validationErrors)
		return result, nil
	}

	validatedPayload, _ := json.Marshal(repairedPayload)
	validationBytes, _ := json.Marshal(repair.RepairableErrors)
	finalStatus := resolveFinalStatus(repair.Repaired, validationResult.BlockingCodes(), outcome.pipeline, validationResult.Status())
	dbStatus := narrativeStatusForDB(finalStatus)
	completed, err := s.q.CompleteAIDashboardNarrative(ctx, dbgen.CompleteAIDashboardNarrativeParams{
		ID:               row.ID,
		SellerAccountID:  sellerAccountID,
		Status:           dbStatus,
		RawAiResponse:    pgtype.Text{String: aiMeta.Content, Valid: true},
		ValidatedPayload: validatedPayload,
		ValidationErrors: validationBytes,
		InputTokens:      narrativeInt4(aiMeta.InputTokens),
		OutputTokens:     narrativeInt4(aiMeta.OutputTokens),
		EstimatedCost:    narrativeNumeric(aicost.EstimateUSD(aiMeta.Model, aiMeta.InputTokens, aiMeta.OutputTokens).CostUSD),
		ErrorMessage:     pgtype.Text{},
		FinishedAt:       pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("complete dashboard narrative: %w", err)
	}
	if err := s.linkDashboardNarrativeToPipeline(ctx, completed.ID, sellerAccountID, opts); err != nil {
		return nil, err
	}
	recordNarrativeGenerationOutcome(finalStatus, repair.RepairableErrors)
	logNarrativeMetrics()
	result := narrativeGenerationResultFromRow(completed, metricDate, &build, false)
	result.Status = finalStatus
	result.RepairSummary = buildNarrativeRepairSummary(repair, finalStatus)
	result.ValidatedNarrativePayload = validatedPayload
	applyAIOnlyRequiredBlocksMeta(result, repairedPayload)
	applyDashboardValidationToResult(result, validationResult)
	applyPreOverrideTopActionsDiagnostics(result, repairedPayload)
	enrichGenerationResultFromOutcome(result, &build, outcome)
	if outcome != nil && outcome.debug != nil && outcome.debug.Enabled() {
		_ = outcome.debug.WriteRunSummary(result, outcome.pipeline)
	}
	return result, nil
}

func narrativeGenerationResultFromInsert(
	row dbgen.AiDashboardNarrative,
	metricDate time.Time,
	build *dashboardNarrativeBuildResult,
	status string,
	errMsg string,
	validationErrors []string,
	inputTokens int,
	outputTokens int,
) *NarrativeGenerationResult {
	result := narrativeGenerationResultFromRow(row, metricDate, build, false)
	result.Status = status
	result.ErrorMessage = errMsg
	result.ValidationErrors = append([]string(nil), validationErrors...)
	result.InputTokens = inputTokens
	result.OutputTokens = outputTokens
	return result
}

func narrativeGenerationResultFromRow(row dbgen.AiDashboardNarrative, metricDate time.Time, build *dashboardNarrativeBuildResult, skipped bool) *NarrativeGenerationResult {
	result := &NarrativeGenerationResult{
		Status:        row.Status,
		MetricDate:    metricDate.UTC().Format("2006-01-02"),
		PromptVersion: row.PromptVersion,
		Skipped:       skipped,
		AIOnly:        dashboardAIOnlyEnabled(),
	}
	if row.ID.Valid {
		result.RunID = row.ID.String()
	}
	if row.ContextHash != "" {
		result.ContextHash = row.ContextHash
	}
	if row.RiskDriversHash != "" {
		result.RenderedContextHash = row.RiskDriversHash
	}
	if row.ErrorMessage.Valid {
		result.ErrorMessage = row.ErrorMessage.String
	}
	if len(row.ValidationErrors) > 0 {
		_ = json.Unmarshal(row.ValidationErrors, &result.ValidationErrors)
	}
	if row.InputTokens.Valid {
		result.InputTokens = int(row.InputTokens.Int32)
	}
	if row.OutputTokens.Valid {
		result.OutputTokens = int(row.OutputTokens.Int32)
	}
	if build != nil && result.ContextHash == "" {
		result.ContextHash = build.FactualContextHash
	}
	if build != nil && result.RenderedContextHash == "" {
		result.RenderedContextHash = build.ContextHash
	}
	if build != nil {
		result.ContextSource = build.ContextSource
	}
	return result
}

func applyAIOnlyRequiredBlocksMeta(result *NarrativeGenerationResult, payload *dashboardNarrativePayload) {
	if result == nil || !dashboardAIOnlyEnabled() {
		return
	}
	errs := validateAIOnlyRequiredBlocks(payload)
	result.AllRequiredBlocksPresent = len(errs) == 0
	result.MissingRequiredBlocks = append([]string(nil), errs...)
	result.FallbackBlocks = []string{}
	result.LegacyBlocks = []string{}
	result.PendingBlocks = []string{}
	result.StaleBlocks = []string{}
	if payload == nil {
		return
	}
	for _, block := range classifyAIOnlyRequiredBlocks(payload) {
		switch block.kind {
		case "substitution":
			result.FallbackBlocks = append(result.FallbackBlocks, block.name)
		case "deprecated":
			result.LegacyBlocks = append(result.LegacyBlocks, block.name)
		case "queued":
			result.PendingBlocks = append(result.PendingBlocks, block.name)
		case "outdated":
			result.StaleBlocks = append(result.StaleBlocks, block.name)
		}
	}
}

func dashboardNarrativeMaxOutputTokens(cfg *config.Config) int {
	if cfg == nil {
		return 12000
	}
	if raw := strings.TrimSpace(os.Getenv("DASHBOARD_NARRATIVE_MAX_OUTPUT_TOKENS")); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value > 0 {
			return value
		}
	}
	if cfg.AI.MaxOutputTokensPerRequest >= 8000 {
		return cfg.AI.MaxOutputTokensPerRequest
	}
	if cfg.AI.MaxOutputTokens >= 8000 {
		return cfg.AI.MaxOutputTokens
	}
	return 12000
}

func (s *NarrativeService) buildNarrativeContextForSeller(
	ctx context.Context,
	sellerAccountID int64,
	metricDate time.Time,
) (dashboardNarrativeBuildResult, int32, string, error) {
	return s.buildNarrativeContextForSellerWithOptions(ctx, sellerAccountID, metricDate, NarrativeRunOptions{})
}

func shouldPersistFrozenDashboardSnapshot(opts NarrativeRunOptions) bool {
	return opts.PipelineRunID != nil && *opts.PipelineRunID > 0 &&
		strings.TrimSpace(opts.DataPackHash) != "" && strings.TrimSpace(opts.GlobalContextHash) != ""
}

func (s *NarrativeService) buildNarrativeContextForSellerWithOptions(
	ctx context.Context,
	sellerAccountID int64,
	metricDate time.Time,
	opts NarrativeRunOptions,
) (dashboardNarrativeBuildResult, int32, string, error) {
	dto, err := s.dashboard.BuildDashboardMetrics(ctx, sellerAccountID, &metricDate)
	if err != nil {
		return dashboardNarrativeBuildResult{}, 0, "", fmt.Errorf("dashboard metrics: %w", err)
	}
	helper := &Service{
		pool:          s.pool,
		q:             s.q,
		dashboard:     s.dashboard,
		criticalSKUs:  analytics.NewCriticalSKUService(s.pool),
		replenishment: analytics.NewReplenishmentService(s.pool),
		advertising:   s.advertising,
		alerts:        s.alerts,
	}
	resp := newDashboardResponseShellForMetrics()
	resp.MetricDate = metricDate.UTC().Format("2006-01-02")
	helper.populateResponseForNarrativeHashes(ctx, sellerAccountID, metricDate, dto, &resp)

	if shouldPersistFrozenDashboardSnapshot(opts) {
		if _, err := helper.persistFrozenDashboardSnapshot(ctx, *opts.PipelineRunID, sellerAccountID, metricDate, opts.DataPackHash, opts.GlobalContextHash, dto, resp); err != nil {
			return dashboardNarrativeBuildResult{}, 0, "", fmt.Errorf("persist frozen dashboard snapshot: %w", err)
		}
	}

	var recommendationsRows []dbgen.Recommendation
	var alertRows []dbgen.Alert
	if !UsesUnifiedDataPackContext(opts) {
		var loadErr error
		legacyAlertsLoads.Add(1)
		recommendationsRows, loadErr = s.q.ListOpenRecommendationsBySellerAccountID(ctx, dbgen.ListOpenRecommendationsBySellerAccountIDParams{
			SellerAccountID: sellerAccountID, Limit: 20, Offset: 0,
		})
		if loadErr != nil {
			return dashboardNarrativeBuildResult{}, 0, "", fmt.Errorf("open recommendations: %w", loadErr)
		}
		legacyRecommendationsLoads.Add(1)
		alertRows, loadErr = s.q.ListAlertsFiltered(ctx, dbgen.ListAlertsFilteredParams{
			SellerAccountID: sellerAccountID,
			Status:          pgtype.Text{String: "open", Valid: true},
			Limit:           30, Offset: 0,
		})
		if loadErr != nil {
			return dashboardNarrativeBuildResult{}, 0, "", fmt.Errorf("open alerts: %w", loadErr)
		}
	}
	var previousSnapshot *accountMetricsSnapshot
	from := metricDate.UTC().AddDate(0, 0, -14)
	if accRows, metricsErr := s.q.ListDailyAccountMetricsBySellerAndDateRange(ctx, dbgen.ListDailyAccountMetricsBySellerAndDateRangeParams{
		SellerAccountID: sellerAccountID,
		MetricDate:      dateValue(from),
		MetricDate_2:    dateValue(metricDate),
	}); metricsErr == nil {
		previousSnapshot = findPreviousAccountSnapshot(accRows, metricDate)
	}

	legacyInput := dataPackNarrativeLegacyInput{
		DTO: dto, AlertRows: alertRows, Recs: recommendationsRows, PreviousSnap: previousSnapshot,
	}

	if s.dataPack != nil || opts.DataPackOverride != nil {
		var pack *aidatapack.SellerAIDataPack
		var packErr error
		if opts.DataPackOverride != nil {
			pack = opts.DataPackOverride
		} else {
			pack, packErr = s.dataPack.Build(ctx, sellerAccountID, metricDate)
		}
		if packErr == nil && pack != nil {
			chunkReady := opts.ChunkSetOverride != nil
			if !chunkReady {
				if _, chunkErr := buildChunkSetForNarrative(pack, aidatapack.DefaultChunkOptions()); chunkErr == nil {
					chunkReady = true
				}
			}
			if chunkReady {
				build, score, severity, ctxErr := buildDashboardNarrativeContextFromDataPackFull(pack, &resp, legacyInput)
				if ctxErr == nil {
					audit := aidatapack.AuditDataPack(pack)
					build.DataPackAuditValid = audit.Valid
					build.ContextSource = ContextSourceSellerAIDataPackChunks
					build.GlobalSemanticContext = opts.GlobalContext
					build.GlobalContextHash = opts.GlobalContextHash
					build.DataPackHash = opts.DataPackHash
					return build, score, severity, nil
				}
				slog.Warn("dashboard narrative data pack context build failed, falling back to legacy",
					"seller_account_id", sellerAccountID, "error", ctxErr.Error())
			} else {
				slog.Warn("dashboard narrative chunk build failed, falling back to legacy",
					"seller_account_id", sellerAccountID, "reason", "chunk_set_unavailable")
			}
		} else {
			slog.Warn("dashboard narrative data pack build failed, falling back to legacy",
				"seller_account_id", sellerAccountID, "error", packErr.Error(), "reason", "data_pack_build_failed")
		}
	}
	if dashboardAIOnlyEnabled() {
		return dashboardNarrativeBuildResult{}, 0, "", fmt.Errorf("ai_only_required_block_missing: unable to build datapack chunks context")
	}

	build, score, severity, err := helper.buildNarrativeBuildResult(ctx, sellerAccountID, metricDate, dto, &resp)
	if err != nil {
		return dashboardNarrativeBuildResult{}, 0, "", err
	}
	build.ContextSource = ContextSourceLegacyDashboard
	return build, score, severity, nil
}

func narrativeAIContextForBuild(build dashboardNarrativeBuildResult) any {
	if (build.ContextSource == ContextSourceSellerAIDataPack ||
		build.ContextSource == ContextSourceSellerAIDataPackChunks ||
		build.ContextSource == ContextSourceSellerAIDataPackOneShot) && build.AIContext != nil {
		return build.AIContext
	}
	return build.Context
}

func narrativeSystemPromptForBuild(build dashboardNarrativeBuildResult) string {
	if build.ContextSource == ContextSourceSellerAIDataPack ||
		build.ContextSource == ContextSourceSellerAIDataPackChunks ||
		build.ContextSource == ContextSourceSellerAIDataPackOneShot {
		return systemPromptDashboardNarrativeV8()
	}
	return systemPromptDashboardNarrative()
}

func narrativeUserPromptForBuild(build dashboardNarrativeBuildResult) string {
	if build.ContextSource == ContextSourceSellerAIDataPack ||
		build.ContextSource == ContextSourceSellerAIDataPackChunks ||
		build.ContextSource == ContextSourceSellerAIDataPackOneShot {
		return userPromptDashboardNarrativeV8()
	}
	return userPromptDashboardNarrative()
}

// BuildStockCoverageContextForDebug rebuilds narrative context and returns stock_coverage_context payload.
func (s *NarrativeService) BuildStockCoverageContextForDebug(
	ctx context.Context,
	sellerAccountID int64,
	metricDate time.Time,
) (map[string]any, error) {
	build, _, _, err := s.buildNarrativeContextForSeller(ctx, sellerAccountID, metricDate)
	if err != nil {
		return nil, err
	}
	if build.Context.StockCoverageContext == nil {
		return map[string]any{}, nil
	}
	return build.Context.StockCoverageContext, nil
}

// BuildNarrativeHashesForDebug rebuilds hashes for comparison with stored narrative row.
func (s *NarrativeService) BuildNarrativeHashesForDebug(
	ctx context.Context,
	sellerAccountID int64,
	metricDate time.Time,
) (contextHash string, riskDriversHash string, err error) {
	build, _, _, err := s.buildNarrativeContextForSeller(ctx, sellerAccountID, metricDate)
	if err != nil {
		return "", "", err
	}
	return build.FactualContextHash, build.ContextHash, nil
}

func narrativeFailureStatus(err error) string {
	if dashboardAIOnlyEnabled() {
		return "failed"
	}
	if errors.Is(err, recommendations.ErrOpenAIRequestTooLarge) {
		return "fallback"
	}
	if openaix.IsTemporarilyUnavailable(err) {
		return "fallback"
	}
	var outage *openaix.HTTPOutageError
	if errors.As(err, &outage) {
		return "fallback"
	}
	return "failed"
}

func (s *NarrativeService) lookupDashboardNarrativeForGeneration(
	ctx context.Context,
	sellerAccountID int64,
	metricDate time.Time,
	opts NarrativeRunOptions,
) (dbgen.AiDashboardNarrative, bool, error) {
	if opts.PipelineRunID != nil && *opts.PipelineRunID > 0 {
		row, err := s.q.GetAIDashboardNarrativeByPipelineRunID(ctx, dbgen.GetAIDashboardNarrativeByPipelineRunIDParams{
			AiPipelineRunID: pgtype.Int8{Int64: *opts.PipelineRunID, Valid: true},
			SellerAccountID: sellerAccountID,
		})
		if err == nil {
			return row, true, nil
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return dbgen.AiDashboardNarrative{}, false, nil
		}
		return dbgen.AiDashboardNarrative{}, false, fmt.Errorf("get dashboard narrative by pipeline: %w", err)
	}
	row, err := s.q.GetAIDashboardNarrativeBySellerAndMetricDate(ctx, dbgen.GetAIDashboardNarrativeBySellerAndMetricDateParams{
		SellerAccountID: sellerAccountID,
		MetricDate:      dateValue(metricDate),
	})
	if err == nil {
		return row, true, nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return dbgen.AiDashboardNarrative{}, false, nil
	}
	return dbgen.AiDashboardNarrative{}, false, fmt.Errorf("get dashboard narrative: %w", err)
}

func shouldReuseExistingDashboardNarrative(
	existing dbgen.AiDashboardNarrative,
	opts NarrativeRunOptions,
	build dashboardNarrativeBuildResult,
) bool {
	if opts.ForceRegenerate {
		return false
	}
	if opts.PipelineRunID != nil && *opts.PipelineRunID > 0 {
		if !existing.AiPipelineRunID.Valid || existing.AiPipelineRunID.Int64 != *opts.PipelineRunID {
			return false
		}
	}
	if existing.Status != "completed" || existing.PromptVersion != DashboardNarrativePromptVersion {
		return false
	}
	if existing.ContextHash != build.FactualContextHash &&
		!(existing.ContextHash == build.ContextHash && existing.RiskDriversHash == build.ContextHash) {
		return false
	}
	if opts.GlobalContext != nil {
		return existing.GlobalContextHash.Valid && existing.DataPackHash.Valid &&
			existing.GlobalContextHash.String == opts.GlobalContextHash &&
			existing.DataPackHash.String == opts.DataPackHash
	}
	return !existing.GlobalContextHash.Valid || existing.GlobalContextHash.String == ""
}

func (s *NarrativeService) shouldRetryDashboardNarrativeOnSameRow(
	existing dbgen.AiDashboardNarrative,
	opts NarrativeRunOptions,
) bool {
	if opts.PipelineRunID == nil || *opts.PipelineRunID <= 0 {
		return false
	}
	if !existing.AiPipelineRunID.Valid || existing.AiPipelineRunID.Int64 != *opts.PipelineRunID {
		return false
	}
	if existing.Status == "failed" {
		return true
	}
	return opts.ForceRegenerate && existing.Status != "running"
}

func (s *NarrativeService) linkDashboardNarrativeToPipeline(
	ctx context.Context,
	narrativeID pgtype.UUID,
	sellerAccountID int64,
	opts NarrativeRunOptions,
) error {
	if opts.PipelineRunID == nil || *opts.PipelineRunID <= 0 {
		return nil
	}
	if err := s.q.ClearAIDashboardNarrativePipelineLinkExcept(ctx, dbgen.ClearAIDashboardNarrativePipelineLinkExceptParams{
		AiPipelineRunID: pgtype.Int8{Int64: *opts.PipelineRunID, Valid: true},
		ID:              narrativeID,
	}); err != nil {
		return fmt.Errorf("clear previous dashboard pipeline link: %w", err)
	}
	_, err := s.q.UpdateAIDashboardNarrativePipelineFields(ctx, dbgen.UpdateAIDashboardNarrativePipelineFieldsParams{
		ID:                narrativeID,
		SellerAccountID:   sellerAccountID,
		AiPipelineRunID:   pgtype.Int8{Int64: *opts.PipelineRunID, Valid: true},
		DataPackHash:      pgtype.Text{String: opts.DataPackHash, Valid: opts.DataPackHash != ""},
		GlobalContextHash: pgtype.Text{String: opts.GlobalContextHash, Valid: opts.GlobalContextHash != ""},
	})
	if err != nil {
		return fmt.Errorf("link dashboard narrative to pipeline: %w", err)
	}
	return nil
}

func narrativeInt4(v int) pgtype.Int4 {
	if v == 0 {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: int32(v), Valid: true}
}

func narrativeNumeric(v float64) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(fmt.Sprintf("%.6f", v))
	return n
}

func buildDashboardNarrativeValidationContextFromBuild(build dashboardNarrativeBuildResult) dashboardNarrativeValidationContext {
	drivers := make(map[string]RiskDriver)
	if build.ContextSource == ContextSourceSellerAIDataPack ||
		build.ContextSource == ContextSourceSellerAIDataPackChunks ||
		build.ContextSource == ContextSourceSellerAIDataPackOneShot {
		if v8, ok := build.AIContext.(dashboardNarrativeContextV8); ok {
			for _, driver := range v8.RiskDrivers {
				drivers[driver.ID] = RiskDriver{
					ID: driver.ID, Category: driver.Category, CategoryLabel: driver.CategoryLabel,
					Severity: driver.Severity, ImpactScore: driver.ImpactScore,
					Title: driver.Title, Reason: driver.Reason, Evidence: driver.Evidence,
					LinkedAlertIDs: driver.LinkedAlertIDs, LinkedRecommendationIDs: driver.LinkedRecommendationIDs,
				}
			}
		}
	} else {
		for _, driver := range build.Context.RiskDrivers {
			drivers[driver.ID] = RiskDriver{
				ID:                      driver.ID,
				Category:                driver.Category,
				CategoryLabel:           driver.CategoryLabel,
				Severity:                driver.Severity,
				ImpactScore:             driver.ImpactScore,
				Title:                   driver.Title,
				Reason:                  driver.Reason,
				Evidence:                driver.Evidence,
				LinkedAlertIDs:          driver.LinkedAlertIDs,
				LinkedRecommendationIDs: driver.LinkedRecommendationIDs,
			}
		}
	}
	sellerPulseCtx := build.SellerPulseCtx
	adEfficiencyCtx := build.AdEfficiencyNarrativeCtx
	return dashboardNarrativeValidationContext{
		DriversByID:               drivers,
		CandidatesByID:            build.CandidateByID,
		MetricsByKey:              build.MetricCatalogByKey,
		AllowedQuotedName:         build.AllowedQuotedNames,
		WhatChanged:               build.WhatChanged,
		Stock:                     copyStockNarrativeValidationCtx(build.StockNarrativeCtx),
		StockRequired:             build.StockNarrativeRequired,
		AdEfficiency:              &adEfficiencyCtx,
		AdEfficiencyRequired:      build.AdEfficiencyNarrativeRequired,
		SellerPulse:               &sellerPulseCtx,
		SellerPulseRequired:       build.SellerPulseRequired,
		ContextSource:             build.ContextSource,
		DataPackFactIDs:           buildDataPackFactIDSet(build.DataPackFactIDs),
		FinanceAvailable:          build.FinanceAvailable,
		PriceConstraintsAvailable: build.PriceConstraintsAvailable,
		AllowedFormattedValues:    buildDataPackFormattedValueAllowlist(buildDataPackFactIDSet(build.DataPackFactIDs), build.MetricCatalogByKey),
	}
}
