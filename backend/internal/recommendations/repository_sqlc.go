package recommendations

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type SQLCRepository struct {
	queries *dbgen.Queries
}

func NewSQLCRepository(queries *dbgen.Queries) *SQLCRepository {
	return &SQLCRepository{queries: queries}
}

func (r *SQLCRepository) GetDailyAccountMetricByDate(ctx context.Context, sellerAccountID int64, metricDate time.Time) (*AccountDailyMetric, error) {
	pgDate := toDate(metricDate)
	rows, err := r.queries.ListDailyAccountMetricsBySellerAndDateRange(ctx, dbgen.ListDailyAccountMetricsBySellerAndDateRangeParams{
		SellerAccountID: sellerAccountID,
		MetricDate:      pgDate,
		MetricDate_2:    pgDate,
	})
	if err != nil {
		return nil, fmt.Errorf("list daily account metrics: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	row := rows[0]
	revenue := numericFloat64(row.Revenue)
	return &AccountDailyMetric{
		MetricDate:        dateString(row.MetricDate),
		Revenue:           revenue,
		OrdersCount:       row.OrdersCount,
		ReturnsCount:      row.ReturnsCount,
		CancelCount:       row.CancelCount,
		AverageOrderValue: analytics.RatioOrNil(revenue, float64(row.OrdersCount)),
		CancellationRate:  analytics.CancellationRate(row.CancelCount, row.OrdersCount),
	}, nil
}

func (r *SQLCRepository) ListDailySKUMetricsByDate(ctx context.Context, sellerAccountID int64, metricDate time.Time) ([]SKUDailyMetric, error) {
	pgDate := toDate(metricDate)
	rows, err := r.queries.ListDailySKUMetricsBySellerAndDateRange(ctx, dbgen.ListDailySKUMetricsBySellerAndDateRangeParams{
		SellerAccountID: sellerAccountID,
		MetricDate:      pgDate,
		MetricDate_2:    pgDate,
	})
	if err != nil {
		return nil, fmt.Errorf("list daily sku metrics: %w", err)
	}
	out := make([]SKUDailyMetric, 0, len(rows))
	for _, row := range rows {
		out = append(out, SKUDailyMetric{
			OzonProductID:  row.OzonProductID,
			SKU:            int8Ptr(row.Sku),
			OfferID:        textPtr(row.OfferID),
			ProductName:    textPtr(row.ProductName),
			Revenue:        numericFloat64(row.Revenue),
			OrdersCount:    row.OrdersCount,
			StockAvailable: row.StockAvailable,
			DaysOfCover:    numericFloat64Ptr(row.DaysOfCover),
		})
	}
	return out, nil
}

func (r *SQLCRepository) ListOpenAlerts(ctx context.Context, sellerAccountID int64, limit int, offset int) ([]AlertSignal, error) {
	rows, err := r.queries.ListOpenAlertsBySellerAccountID(ctx, dbgen.ListOpenAlertsBySellerAccountIDParams{
		SellerAccountID: sellerAccountID,
		Limit:           normalizeLimit(limit),
		Offset:          normalizeOffset(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("list open alerts: %w", err)
	}
	out := make([]AlertSignal, 0, len(rows))
	for _, row := range rows {
		evidence := map[string]any{}
		if len(row.EvidencePayload) > 0 {
			if err := json.Unmarshal(row.EvidencePayload, &evidence); err != nil {
				return nil, fmt.Errorf("unmarshal alert evidence payload: %w", err)
			}
		}
		out = append(out, AlertSignal{
			ID:            row.ID,
			AlertType:     row.AlertType,
			AlertGroup:    row.AlertGroup,
			EntityType:    row.EntityType,
			EntityID:      textPtr(row.EntityID),
			EntitySKU:     int8Ptr(row.EntitySku),
			EntityOfferID: textPtr(row.EntityOfferID),
			Title:         row.Title,
			Message:       row.Message,
			Severity:      row.Severity,
			Urgency:       row.Urgency,
			LastSeenAt:    timestamptz(row.LastSeenAt),
			Evidence:      evidence,
		})
	}
	return out, nil
}

func (r *SQLCRepository) CountOpenAlertsBySeverity(ctx context.Context, sellerAccountID int64) ([]NamedCount, error) {
	rows, err := r.queries.CountOpenAlertsBySeverity(ctx, sellerAccountID)
	if err != nil {
		return nil, fmt.Errorf("count open alerts by severity: %w", err)
	}
	out := make([]NamedCount, 0, len(rows))
	for _, row := range rows {
		out = append(out, NamedCount{Name: row.Severity, Count: row.AlertsCount})
	}
	return out, nil
}

func (r *SQLCRepository) CountOpenAlertsByGroup(ctx context.Context, sellerAccountID int64) ([]NamedCount, error) {
	rows, err := r.queries.CountOpenAlertsByGroup(ctx, sellerAccountID)
	if err != nil {
		return nil, fmt.Errorf("count open alerts by group: %w", err)
	}
	out := make([]NamedCount, 0, len(rows))
	for _, row := range rows {
		out = append(out, NamedCount{Name: row.AlertGroup, Count: row.AlertsCount})
	}
	return out, nil
}

func (r *SQLCRepository) GetLatestAlertRun(ctx context.Context, sellerAccountID int64) (*RunInfo, error) {
	row, err := r.queries.GetLatestAlertRunBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get latest alert run: %w", err)
	}
	return &RunInfo{
		ID:         row.ID,
		RunType:    row.RunType,
		Status:     row.Status,
		StartedAt:  timestamptz(row.StartedAt),
		FinishedAt: timestamptzPtr(row.FinishedAt),
	}, nil
}

func (r *SQLCRepository) ListAdCampaignSummariesByDateRange(ctx context.Context, sellerAccountID int64, dateFrom time.Time, dateTo time.Time) ([]AdCampaignSummary, error) {
	rows, err := r.queries.ListAdCampaignSummariesBySellerAndDateRange(ctx, dbgen.ListAdCampaignSummariesBySellerAndDateRangeParams{
		SellerAccountID: sellerAccountID,
		DateFrom:        toDate(dateFrom),
		DateTo:          toDate(dateTo),
	})
	if err != nil {
		return nil, fmt.Errorf("list ad campaign summaries: %w", err)
	}
	out := make([]AdCampaignSummary, 0, len(rows))
	for _, row := range rows {
		spend := numericFloat64(row.SpendTotal)
		revenue := numericFloat64(row.RevenueTotal)
		out = append(out, AdCampaignSummary{
			CampaignExternalID: row.CampaignExternalID,
			CampaignName:       row.CampaignName,
			CampaignType:       textPtr(row.CampaignType),
			Status:             textPtr(row.Status),
			SpendTotal:         spend,
			OrdersTotal:        row.OrdersTotal,
			RevenueTotal:       revenue,
			CampaignDRR:        analytics.CampaignDRR(spend, revenue),
			CampaignROAS:       analytics.CampaignROAS(revenue, spend),
		})
	}
	return out, nil
}

func (r *SQLCRepository) ListSKUEffectiveConstraintsBySellerAccountID(ctx context.Context, sellerAccountID int64) ([]EffectiveConstraint, error) {
	rows, err := r.queries.ListSKUEffectiveConstraintsBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		return nil, fmt.Errorf("list effective constraints: %w", err)
	}
	out := make([]EffectiveConstraint, 0, len(rows))
	for _, row := range rows {
		out = append(out, EffectiveConstraint{
			OzonProductID:     row.OzonProductID,
			SKU:               int8Ptr(row.Sku),
			OfferID:           textPtr(row.OfferID),
			RuleID:            row.RuleID,
			ResolvedFrom:      row.ResolvedFromScopeType,
			EffectiveMinPrice: numericFloat64Ptr(row.EffectiveMinPrice),
			EffectiveMaxPrice: numericFloat64Ptr(row.EffectiveMaxPrice),
			ReferencePrice:    numericFloat64Ptr(row.ReferencePrice),
			ImpliedCost:       numericFloat64Ptr(row.ImpliedCost),
		})
	}
	return out, nil
}

func (r *SQLCRepository) ListOpenRecommendations(ctx context.Context, sellerAccountID int64, limit int, offset int) ([]RecommendationDigest, error) {
	rows, err := r.queries.ListOpenRecommendationsBySellerAccountID(ctx, dbgen.ListOpenRecommendationsBySellerAccountIDParams{
		SellerAccountID: sellerAccountID,
		Limit:           normalizeLimit(limit),
		Offset:          normalizeOffset(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("list open recommendations: %w", err)
	}
	out := make([]RecommendationDigest, 0, len(rows))
	for _, row := range rows {
		out = append(out, RecommendationDigest{
			ID:                 row.ID,
			RecommendationType: row.RecommendationType,
			Horizon:            row.Horizon,
			EntityType:         row.EntityType,
			EntityID:           textPtr(row.EntityID),
			EntitySKU:          int8Ptr(row.EntitySku),
			EntityOfferID:      textPtr(row.EntityOfferID),
			Title:              row.Title,
			PriorityScore:      numericFloat64(row.PriorityScore),
			PriorityLevel:      row.PriorityLevel,
			Urgency:            row.Urgency,
			ConfidenceLevel:    row.ConfidenceLevel,
			LastSeenAt:         timestamptz(row.LastSeenAt),
		})
	}
	return out, nil
}

func (r *SQLCRepository) CountOpenRecommendations(ctx context.Context, sellerAccountID int64) (int64, error) {
	count, err := r.queries.CountOpenRecommendationsBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		return 0, fmt.Errorf("count open recommendations: %w", err)
	}
	return count, nil
}

func (r *SQLCRepository) CountOpenRecommendationsByPriority(ctx context.Context, sellerAccountID int64) ([]NamedCount, error) {
	rows, err := r.queries.CountOpenRecommendationsByPriority(ctx, sellerAccountID)
	if err != nil {
		return nil, fmt.Errorf("count open recommendations by priority: %w", err)
	}
	out := make([]NamedCount, 0, len(rows))
	for _, row := range rows {
		out = append(out, NamedCount{Name: row.PriorityLevel, Count: row.RecommendationsCount})
	}
	return out, nil
}

func (r *SQLCRepository) CountOpenRecommendationsByConfidence(ctx context.Context, sellerAccountID int64) ([]NamedCount, error) {
	rows, err := r.queries.CountOpenRecommendationsByConfidence(ctx, sellerAccountID)
	if err != nil {
		return nil, fmt.Errorf("count open recommendations by confidence: %w", err)
	}
	out := make([]NamedCount, 0, len(rows))
	for _, row := range rows {
		out = append(out, NamedCount{Name: row.ConfidenceLevel, Count: row.RecommendationsCount})
	}
	return out, nil
}

func (r *SQLCRepository) GetLatestRecommendationRun(ctx context.Context, sellerAccountID int64) (*RunInfo, error) {
	row, err := r.queries.GetLatestRecommendationRunBySellerAccountID(ctx, sellerAccountID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get latest recommendation run: %w", err)
	}
	var asOf *string
	if row.AsOfDate.Valid {
		s := dateString(row.AsOfDate)
		asOf = &s
	}
	est := numericFloat64(row.EstimatedCost)
	return &RunInfo{
		ID:                            row.ID,
		RunType:                       row.RunType,
		Status:                        row.Status,
		StartedAt:                     timestamptz(row.StartedAt),
		FinishedAt:                    timestamptzPtr(row.FinishedAt),
		AsOfDate:                      asOf,
		AIModel:                       textPtr(row.AiModel),
		AIPromptVersion:               textPtr(row.AiPromptVersion),
		InputTokens:                   int(row.InputTokens),
		OutputTokens:                  int(row.OutputTokens),
		EstimatedCost:                 est,
		GeneratedRecommendationsCount: int(row.GeneratedRecommendationsCount),
		ErrorMessage:                  textPtr(row.ErrorMessage),
	}, nil
}

func toDate(t time.Time) pgtype.Date {
	return pgtype.Date{Time: t.UTC(), Valid: true}
}

func dateString(v pgtype.Date) string {
	if !v.Valid {
		return ""
	}
	return v.Time.UTC().Format("2006-01-02")
}

func timestamptz(v pgtype.Timestamptz) time.Time {
	if !v.Valid {
		return time.Time{}
	}
	return v.Time.UTC()
}

func timestamptzPtr(v pgtype.Timestamptz) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time.UTC()
	return &t
}

func textPtr(v pgtype.Text) *string {
	if !v.Valid {
		return nil
	}
	s := v.String
	return &s
}

func int8Ptr(v pgtype.Int8) *int64 {
	if !v.Valid {
		return nil
	}
	i := v.Int64
	return &i
}

func numericFloat64(v pgtype.Numeric) float64 {
	if !v.Valid {
		return 0
	}
	f, err := v.Float64Value()
	if err != nil || !f.Valid {
		return 0
	}
	return f.Float64
}

func numericFloat64Ptr(v pgtype.Numeric) *float64 {
	if !v.Valid {
		return nil
	}
	f, err := v.Float64Value()
	if err != nil || !f.Valid {
		return nil
	}
	out := f.Float64
	return &out
}

func normalizeLimit(limit int) int32 {
	if limit <= 0 {
		return 20
	}
	if limit > 200 {
		return 200
	}
	return int32(limit)
}

func normalizeOffset(offset int) int32 {
	if offset < 0 {
		return 0
	}
	return int32(offset)
}

func (r *SQLCRepository) CreateRun(ctx context.Context, input CreateRecommendationRunInput) (int64, error) {
	row, err := r.queries.CreateRecommendationRun(ctx, dbgen.CreateRecommendationRunParams{
		SellerAccountID: input.SellerAccountID,
		RunType:         input.RunType,
		AsOfDate:        toDate(input.AsOfDate),
		AiModel:         nullableText(input.AIModel),
		AiPromptVersion: nullableText(input.AIPromptVersion),
	})
	if err != nil {
		return 0, fmt.Errorf("create recommendation run: %w", err)
	}
	return row.ID, nil
}

func (r *SQLCRepository) CompleteRun(ctx context.Context, input CompleteRecommendationRunInput) error {
	estimatedCost, err := numericFromFloat(input.EstimatedCost, 6)
	if err != nil {
		return fmt.Errorf("convert estimated cost: %w", err)
	}
	_, err = r.queries.CompleteRecommendationRun(ctx, dbgen.CompleteRecommendationRunParams{
		ID:                            input.RunID,
		SellerAccountID:               input.SellerAccountID,
		InputTokens:                   int32(input.InputTokens),
		OutputTokens:                  int32(input.OutputTokens),
		EstimatedCost:                 estimatedCost,
		GeneratedRecommendationsCount: int32(input.GeneratedRecommendationsCount),
		AcceptedRecommendationsCount:  int32(input.AcceptedRecommendationsCount),
	})
	if err != nil {
		return fmt.Errorf("complete recommendation run: %w", err)
	}
	return nil
}

func (r *SQLCRepository) CreateRunDiagnostic(ctx context.Context, input CreateRunDiagnosticInput) error {
	contextSummary, err := json.Marshal(input.ContextPayloadSummary)
	if err != nil {
		return fmt.Errorf("marshal context summary: %w", err)
	}
	rawResponse := input.RawOpenAIResponse
	if len(rawResponse) == 0 {
		rawResponse = []byte("{}")
	}
	validationPayload, err := json.Marshal(input.ValidationResultPayload)
	if err != nil {
		return fmt.Errorf("marshal validation payload: %w", err)
	}
	rejectedPayload, err := json.Marshal(input.RejectedItemsPayload)
	if err != nil {
		return fmt.Errorf("marshal rejected payload: %w", err)
	}
	estCost, err := numericFromFloat(input.EstimatedCost, 6)
	if err != nil {
		return fmt.Errorf("convert estimated cost: %w", err)
	}
	_, err = r.queries.CreateRecommendationRunDiagnostic(ctx, dbgen.CreateRecommendationRunDiagnosticParams{
		RecommendationRunID:     pgtype.Int8{Int64: input.RunID, Valid: true},
		SellerAccountID:         input.SellerAccountID,
		OpenaiRequestID:         nullableText(input.OpenAIRequestID),
		AiModel:                 nullableText(input.AIModel),
		PromptVersion:           nullableText(input.PromptVersion),
		ContextPayloadSummary:   contextSummary,
		RawOpenaiResponse:       rawResponse,
		ValidationResultPayload: validationPayload,
		RejectedItemsPayload:    rejectedPayload,
		ErrorStage:              nullableText(input.ErrorStage),
		ErrorMessage:            nullableText(input.ErrorMessage),
		InputTokens:             int64(input.InputTokens),
		OutputTokens:            int64(input.OutputTokens),
		EstimatedCost:           estCost,
	})
	if err != nil {
		return fmt.Errorf("create recommendation run diagnostic: %w", err)
	}
	return nil
}

func (r *SQLCRepository) FailRun(ctx context.Context, sellerAccountID int64, runID int64, errorMessage string) error {
	_, err := r.queries.FailRecommendationRun(ctx, dbgen.FailRecommendationRunParams{
		ID:              runID,
		SellerAccountID: sellerAccountID,
		ErrorMessage:    nullableText(errorMessage),
	})
	if err != nil {
		return fmt.Errorf("fail recommendation run: %w", err)
	}
	return nil
}

func (r *SQLCRepository) UpsertRecommendation(ctx context.Context, input UpsertRecommendationInput) (int64, error) {
	supportingMetricsPayload, err := json.Marshal(input.SupportingMetrics)
	if err != nil {
		return 0, fmt.Errorf("marshal supporting metrics payload: %w", err)
	}
	constraintsPayload, err := json.Marshal(input.Constraints)
	if err != nil {
		return 0, fmt.Errorf("marshal constraints payload: %w", err)
	}
	priorityScore, err := numericFromFloat(input.PriorityScore, 2)
	if err != nil {
		return 0, fmt.Errorf("convert priority score: %w", err)
	}

	row, err := r.queries.UpsertRecommendation(ctx, dbgen.UpsertRecommendationParams{
		SellerAccountID:          input.SellerAccountID,
		Source:                   input.Source,
		RecommendationType:       input.RecommendationType,
		Horizon:                  input.Horizon,
		EntityType:               input.EntityType,
		EntityID:                 nullableTextPtr(input.EntityID),
		EntitySku:                nullableInt64(input.EntitySKU),
		EntityOfferID:            nullableTextPtr(input.EntityOfferID),
		Title:                    input.Title,
		WhatHappened:             input.WhatHappened,
		WhyItMatters:             input.WhyItMatters,
		RecommendedAction:        input.RecommendedAction,
		ExpectedEffect:           nullableTextPtr(input.ExpectedEffect),
		PriorityScore:            priorityScore,
		PriorityLevel:            input.PriorityLevel,
		Urgency:                  input.Urgency,
		ConfidenceLevel:          input.ConfidenceLevel,
		SupportingMetricsPayload: supportingMetricsPayload,
		ConstraintsPayload:       constraintsPayload,
		AiModel:                  nullableText(input.AIModel),
		AiPromptVersion:          nullableText(input.AIPromptVersion),
		RawAiResponse:            input.RawAIResponse,
		Fingerprint:              input.Fingerprint,
	})
	if err != nil {
		return 0, fmt.Errorf("upsert recommendation: %w", err)
	}
	return row.ID, nil
}

func (r *SQLCRepository) LinkRecommendationAlert(ctx context.Context, sellerAccountID int64, recommendationID int64, alertID int64) error {
	if err := r.queries.LinkRecommendationAlert(ctx, dbgen.LinkRecommendationAlertParams{
		RecommendationID: recommendationID,
		AlertID:          alertID,
		SellerAccountID:  sellerAccountID,
		LinkType:         "supporting_signal",
	}); err != nil {
		return fmt.Errorf("link recommendation alert: %w", err)
	}
	return nil
}

func (r *SQLCRepository) DeleteRecommendationAlertLinks(ctx context.Context, sellerAccountID int64, recommendationID int64) error {
	if err := r.queries.DeleteRecommendationAlertLinks(ctx, dbgen.DeleteRecommendationAlertLinksParams{
		RecommendationID: recommendationID,
		SellerAccountID:  sellerAccountID,
	}); err != nil {
		return fmt.Errorf("delete recommendation alert links: %w", err)
	}
	return nil
}

func (r *SQLCRepository) ListRecommendationsFiltered(ctx context.Context, sellerAccountID int64, filter ListFilter) ([]Recommendation, error) {
	rows, err := r.queries.ListRecommendationsFiltered(ctx, dbgen.ListRecommendationsFilteredParams{
		SellerAccountID:    sellerAccountID,
		Limit:              normalizeLimit(filter.Limit),
		Offset:             normalizeOffset(filter.Offset),
		Status:             nullableTextPtr(filter.Status),
		RecommendationType: nullableTextPtr(filter.RecommendationType),
		PriorityLevel:      nullableTextPtr(filter.PriorityLevel),
		ConfidenceLevel:    nullableTextPtr(filter.ConfidenceLevel),
		Horizon:            nullableTextPtr(filter.Horizon),
		EntityType:         nullableTextPtr(filter.EntityType),
	})
	if err != nil {
		return nil, fmt.Errorf("list recommendations filtered: %w", err)
	}
	out := make([]Recommendation, 0, len(rows))
	for _, row := range rows {
		item, mapErr := mapRecommendation(row)
		if mapErr != nil {
			return nil, mapErr
		}
		out = append(out, item)
	}
	return out, nil
}

func (r *SQLCRepository) GetRecommendationByID(ctx context.Context, sellerAccountID int64, recommendationID int64) (Recommendation, error) {
	row, err := r.queries.GetRecommendationByID(ctx, dbgen.GetRecommendationByIDParams{
		ID:              recommendationID,
		SellerAccountID: sellerAccountID,
	})
	if err != nil {
		return Recommendation{}, fmt.Errorf("get recommendation by id=%d: %w", recommendationID, err)
	}
	return mapRecommendation(row)
}

func (r *SQLCRepository) ListAlertsByRecommendationID(ctx context.Context, sellerAccountID int64, recommendationID int64) ([]RelatedAlert, error) {
	rows, err := r.queries.ListAlertsByRecommendationID(ctx, dbgen.ListAlertsByRecommendationIDParams{
		SellerAccountID:  sellerAccountID,
		RecommendationID: recommendationID,
	})
	if err != nil {
		return nil, fmt.Errorf("list alerts by recommendation id=%d: %w", recommendationID, err)
	}
	out := make([]RelatedAlert, 0, len(rows))
	for _, row := range rows {
		evidence := map[string]any{}
		if len(row.EvidencePayload) > 0 {
			if err := json.Unmarshal(row.EvidencePayload, &evidence); err != nil {
				return nil, fmt.Errorf("unmarshal alert evidence payload: %w", err)
			}
		}
		out = append(out, RelatedAlert{
			ID:              row.ID,
			AlertType:       row.AlertType,
			AlertGroup:      row.AlertGroup,
			EntityType:      row.EntityType,
			EntityID:        textPtr(row.EntityID),
			EntitySKU:       int8Ptr(row.EntitySku),
			EntityOfferID:   textPtr(row.EntityOfferID),
			Title:           row.Title,
			Message:         row.Message,
			Severity:        row.Severity,
			Urgency:         row.Urgency,
			Status:          row.Status,
			EvidencePayload: evidence,
			FirstSeenAt:     timestamptz(row.FirstSeenAt),
			LastSeenAt:      timestamptz(row.LastSeenAt),
		})
	}
	return out, nil
}

func (r *SQLCRepository) AcceptRecommendation(ctx context.Context, sellerAccountID int64, recommendationID int64) (Recommendation, error) {
	row, err := r.queries.AcceptRecommendation(ctx, dbgen.AcceptRecommendationParams{
		ID:              recommendationID,
		SellerAccountID: sellerAccountID,
	})
	if err != nil {
		return Recommendation{}, fmt.Errorf("accept recommendation id=%d: %w", recommendationID, err)
	}
	return mapRecommendation(row)
}

func (r *SQLCRepository) DismissRecommendation(ctx context.Context, sellerAccountID int64, recommendationID int64) (Recommendation, error) {
	row, err := r.queries.DismissRecommendation(ctx, dbgen.DismissRecommendationParams{
		ID:              recommendationID,
		SellerAccountID: sellerAccountID,
	})
	if err != nil {
		return Recommendation{}, fmt.Errorf("dismiss recommendation id=%d: %w", recommendationID, err)
	}
	return mapRecommendation(row)
}

func (r *SQLCRepository) ResolveRecommendation(ctx context.Context, sellerAccountID int64, recommendationID int64) (Recommendation, error) {
	row, err := r.queries.ResolveRecommendation(ctx, dbgen.ResolveRecommendationParams{
		ID:              recommendationID,
		SellerAccountID: sellerAccountID,
	})
	if err != nil {
		return Recommendation{}, fmt.Errorf("resolve recommendation id=%d: %w", recommendationID, err)
	}
	return mapRecommendation(row)
}

func (r *SQLCRepository) CreateFeedback(ctx context.Context, input AddRecommendationFeedbackInput) (*RecommendationFeedback, error) {
	row, err := r.queries.CreateRecommendationFeedback(ctx, dbgen.CreateRecommendationFeedbackParams{
		RecommendationID: input.RecommendationID,
		SellerAccountID:  input.SellerAccountID,
		Rating:           string(input.Rating),
		Comment:          nullableTextPtr(input.Comment),
	})
	if err != nil {
		return nil, fmt.Errorf("create recommendation feedback: %w", err)
	}
	item := RecommendationFeedback{
		ID:               row.ID,
		RecommendationID: row.RecommendationID,
		SellerAccountID:  row.SellerAccountID,
		Rating:           RecommendationFeedbackRating(row.Rating),
		Comment:          textPtr(row.Comment),
		CreatedAt:        timestamptz(row.CreatedAt),
	}
	return &item, nil
}

func nullableText(v string) pgtype.Text {
	if strings.TrimSpace(v) == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: v, Valid: true}
}

func nullableTextPtr(v *string) pgtype.Text {
	if v == nil || strings.TrimSpace(*v) == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: *v, Valid: true}
}

func nullableInt64(v *int64) pgtype.Int8 {
	if v == nil {
		return pgtype.Int8{Valid: false}
	}
	return pgtype.Int8{Int64: *v, Valid: true}
}

func numericFromFloat(v float64, scale int) (pgtype.Numeric, error) {
	var n pgtype.Numeric
	precision := strconv.Itoa(scale)
	if err := n.Scan(fmt.Sprintf("%."+precision+"f", v)); err != nil {
		return pgtype.Numeric{}, err
	}
	return n, nil
}

func mapRecommendation(row dbgen.Recommendation) (Recommendation, error) {
	supporting := map[string]any{}
	if len(row.SupportingMetricsPayload) > 0 {
		if err := json.Unmarshal(row.SupportingMetricsPayload, &supporting); err != nil {
			return Recommendation{}, fmt.Errorf("unmarshal supporting metrics payload: %w", err)
		}
	}
	constraints := map[string]any{}
	if len(row.ConstraintsPayload) > 0 {
		if err := json.Unmarshal(row.ConstraintsPayload, &constraints); err != nil {
			return Recommendation{}, fmt.Errorf("unmarshal constraints payload: %w", err)
		}
	}
	raw := map[string]any{}
	if len(row.RawAiResponse) > 0 {
		if err := json.Unmarshal(row.RawAiResponse, &raw); err != nil {
			raw = map[string]any{}
		}
	}
	return Recommendation{
		ID:                 row.ID,
		Source:             row.Source,
		RecommendationType: row.RecommendationType,
		Horizon:            row.Horizon,
		EntityType:         row.EntityType,
		EntityID:           textPtr(row.EntityID),
		EntitySKU:          int8Ptr(row.EntitySku),
		EntityOfferID:      textPtr(row.EntityOfferID),
		Title:              row.Title,
		WhatHappened:       row.WhatHappened,
		WhyItMatters:       row.WhyItMatters,
		RecommendedAction:  row.RecommendedAction,
		ExpectedEffect:     textPtr(row.ExpectedEffect),
		PriorityScore:      numericFloat64(row.PriorityScore),
		PriorityLevel:      row.PriorityLevel,
		Urgency:            row.Urgency,
		ConfidenceLevel:    row.ConfidenceLevel,
		Status:             row.Status,
		SupportingMetrics:  supporting,
		Constraints:        constraints,
		AIModel:            textPtr(row.AiModel),
		AIPromptVersion:    textPtr(row.AiPromptVersion),
		RawAIResponse:      raw,
		FirstSeenAt:        timestamptz(row.FirstSeenAt),
		LastSeenAt:         timestamptz(row.LastSeenAt),
		AcceptedAt:         timestamptzPtr(row.AcceptedAt),
		DismissedAt:        timestamptzPtr(row.DismissedAt),
		ResolvedAt:         timestamptzPtr(row.ResolvedAt),
		CreatedAt:          timestamptz(row.CreatedAt),
		UpdatedAt:          timestamptz(row.UpdatedAt),
	}, nil
}
