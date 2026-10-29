package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type SQLCRepository struct {
	q *dbgen.Queries
}

func NewSQLCRepository(q *dbgen.Queries) *SQLCRepository {
	return &SQLCRepository{q: q}
}

var _ Repository = (*SQLCRepository)(nil)

func (r *SQLCRepository) ListClients(ctx context.Context, filter ClientListFilter) (*ClientListResult, error) {
	page := NormalizePage(filter.Limit, filter.Offset, 50, 200)
	rows, err := r.q.AdminListClients(ctx, dbgen.AdminListClientsParams{
		Search:           optionalText(filter.Search),
		SellerStatus:     optionalText(filter.SellerStatus),
		ConnectionStatus: optionalText(filter.ConnectionStatus),
		BillingStatus:    optionalText(filter.BillingStatus),
		PageOffset:       page.Offset,
		PageLimit:        page.Limit,
	})
	if err != nil {
		return nil, err
	}

	items := make([]ClientListItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, ClientListItem{
			SellerAccountID:               row.SellerAccountID,
			SellerName:                    row.SellerName,
			UserEmail:                     row.OwnerEmail,
			SellerStatus:                  row.SellerStatus,
			ConnectionStatus:              textPtr(row.ConnectionStatus),
			LastConnectionCheckAt:         timePtr(row.ConnectionLastCheckAt),
			LastConnectionError:           textPtr(row.ConnectionLastError),
			LatestSyncStatus:              textPtrFromString(row.LatestSyncStatus),
			LatestSyncStartedAt:           timePtr(row.LatestSyncStartedAt),
			LatestSyncFinishedAt:          timePtr(row.LatestSyncFinishedAt),
			OpenAlertsCount:               row.OpenAlertsCount,
			OpenRecommendationsCount:      row.OpenRecommendationsCount,
			LatestRecommendationRunStatus: textPtrFromString(row.LatestRecommendationRunStatus),
			LatestChatTraceStatus:         textPtrFromString(row.LatestChatTraceStatus),
			BillingStatus:                 textPtr(row.BillingStatus),
			CreatedAt:                     timeValue(row.SellerCreatedAt),
			UpdatedAt:                     timeValue(row.SellerUpdatedAt),
		})
	}

	return &ClientListResult{
		Items:  items,
		Limit:  page.Limit,
		Offset: page.Offset,
	}, nil
}

func (r *SQLCRepository) GetClientOverview(ctx context.Context, sellerAccountID int64) (*ClientOverview, error) {
	row, err := r.q.AdminGetClientOverview(ctx, sellerAccountID)
	if err != nil {
		return nil, err
	}
	ownerID := row.OwnerUserID
	ownerEmail := row.OwnerEmail
	return &ClientOverview{
		SellerAccountID: row.SellerAccountID,
		SellerName:      row.SellerName,
		SellerStatus:    row.SellerStatus,
		OwnerUserID:     &ownerID,
		OwnerEmail:      &ownerEmail,
		CreatedAt:       timeValue(row.CreatedAt),
		UpdatedAt:       timeValue(row.UpdatedAt),
	}, nil
}

func (r *SQLCRepository) GetClientConnections(ctx context.Context, sellerAccountID int64) ([]ClientConnection, error) {
	rows, err := r.q.AdminListClientConnections(ctx, sellerAccountID)
	if err != nil {
		return nil, err
	}
	items := make([]ClientConnection, 0, len(rows))
	for _, row := range rows {
		items = append(items, ClientConnection{
			Provider:                    row.Provider,
			ConnectionStatus:            row.ConnectionStatus,
			LastCheckAt:                 timePtr(row.LastCheckAt),
			LastCheckResult:             textPtr(row.LastCheckResult),
			LastConnectionErr:           textPtr(row.LastError),
			UpdatedAt:                   timePtr(row.UpdatedAt),
			PerformanceConnectionStatus: row.PerformanceConnectionStatus,
			PerformanceTokenSet:         row.PerformanceTokenSet,
			PerformanceLastCheckAt:      timePtr(row.PerformanceLastCheckAt),
			PerformanceLastCheckResult:  textPtr(row.PerformanceLastCheckResult),
			PerformanceLastError:        textPtr(row.PerformanceLastError),
		})
	}
	return items, nil
}

func (r *SQLCRepository) ListSyncJobs(ctx context.Context, sellerAccountID int64, filter SyncJobFilter, page Page) ([]SyncJobSummary, error) {
	rows, err := r.q.AdminListSyncJobs(ctx, dbgen.AdminListSyncJobsParams{
		SellerAccountID: sellerAccountID,
		Status:          optionalText(filter.Status),
		PageLimit:       page.Limit,
		PageOffset:      page.Offset,
	})
	if err != nil {
		return nil, err
	}
	items := make([]SyncJobSummary, 0, len(rows))
	for _, row := range rows {
		items = append(items, SyncJobSummary{
			ID:           row.ID,
			Type:         row.Type,
			Status:       row.Status,
			StartedAt:    timePtr(row.StartedAt),
			FinishedAt:   timePtr(row.FinishedAt),
			ErrorMessage: textPtr(row.ErrorMessage),
			CreatedAt:    timeValue(row.CreatedAt),
		})
	}
	return items, nil
}

func (r *SQLCRepository) ListImportJobs(ctx context.Context, sellerAccountID int64, filter ImportJobFilter, page Page) ([]ImportJobSummary, error) {
	rows, err := r.q.AdminListImportJobs(ctx, dbgen.AdminListImportJobsParams{
		SellerAccountID: sellerAccountID,
		Status:          optionalText(filter.Status),
		Domain:          optionalText(filter.Domain),
		PageLimit:       page.Limit,
		PageOffset:      page.Offset,
	})
	if err != nil {
		return nil, err
	}
	items := make([]ImportJobSummary, 0, len(rows))
	for _, row := range rows {
		items = append(items, ImportJobSummary{
			ID:              row.ID,
			SyncJobID:       row.SyncJobID,
			Domain:          row.Domain,
			Status:          row.Status,
			SourceCursor:    textPtr(row.SourceCursor),
			RecordsReceived: row.RecordsReceived,
			RecordsImported: row.RecordsImported,
			RecordsFailed:   row.RecordsFailed,
			StartedAt:       timePtr(row.StartedAt),
			FinishedAt:      timePtr(row.FinishedAt),
			ErrorMessage:    textPtr(row.ErrorMessage),
			CreatedAt:       timeValue(row.CreatedAt),
		})
	}
	return items, nil
}

func (r *SQLCRepository) ListImportErrors(ctx context.Context, sellerAccountID int64, filter ImportErrorFilter, page Page) ([]ImportErrorItem, error) {
	rows, err := r.q.AdminListImportErrors(ctx, dbgen.AdminListImportErrorsParams{
		SellerAccountID: sellerAccountID,
		Domain:          optionalText(filter.Domain),
		Status:          optionalText(filter.Status),
		PageLimit:       page.Limit,
		PageOffset:      page.Offset,
	})
	if err != nil {
		return nil, err
	}
	items := make([]ImportErrorItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, ImportErrorItem{
			ImportJobID:  row.ID,
			SyncJobID:    row.SyncJobID,
			Domain:       row.Domain,
			Status:       row.Status,
			ErrorMessage: row.ErrorMessage.String,
			CreatedAt:    timeValue(row.CreatedAt),
		})
	}
	return items, nil
}

func (r *SQLCRepository) ListSyncCursors(ctx context.Context, sellerAccountID int64, filter SyncCursorFilter, page Page) ([]SyncCursorItem, error) {
	rows, err := r.q.AdminListSyncCursors(ctx, dbgen.AdminListSyncCursorsParams{
		SellerAccountID: sellerAccountID,
		Domain:          optionalText(filter.Domain),
		PageLimit:       page.Limit,
		PageOffset:      page.Offset,
	})
	if err != nil {
		return nil, err
	}
	items := make([]SyncCursorItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, SyncCursorItem{
			ID:          row.ID,
			Domain:      row.Domain,
			CursorType:  row.CursorType,
			CursorValue: textPtr(row.CursorValue),
			UpdatedAt:   timeValue(row.UpdatedAt),
		})
	}
	return items, nil
}

func (r *SQLCRepository) ListAlertRuns(ctx context.Context, sellerAccountID int64, page Page) ([]AlertRunSummary, error) {
	rows, err := r.q.ListAlertRunsBySellerAccountID(ctx, dbgen.ListAlertRunsBySellerAccountIDParams{
		SellerAccountID: sellerAccountID,
		Limit:           page.Limit,
		Offset:          page.Offset,
	})
	if err != nil {
		return nil, err
	}
	items := make([]AlertRunSummary, 0, len(rows))
	for _, row := range rows {
		items = append(items, AlertRunSummary{
			ID:               row.ID,
			RunType:          row.RunType,
			Status:           row.Status,
			StartedAt:        timePtr(row.StartedAt),
			FinishedAt:       timePtr(row.FinishedAt),
			TotalAlertsCount: row.TotalAlertsCount,
			ErrorMessage:     textPtr(row.ErrorMessage),
		})
	}
	return items, nil
}

func (r *SQLCRepository) ListRecommendationRuns(ctx context.Context, sellerAccountID int64, filter RecommendationRunLogFilter, page Page) ([]RecommendationRunLogItem, error) {
	rows, err := r.q.AdminListRecommendationRuns(ctx, dbgen.AdminListRecommendationRunsParams{
		SellerAccountID: sellerAccountID,
		Status:          optionalText(filter.Status),
		RunType:         optionalText(filter.RunType),
		PageLimit:       page.Limit,
		PageOffset:      page.Offset,
	})
	if err != nil {
		return nil, err
	}
	items := make([]RecommendationRunLogItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, recommendationRunSummaryFromDB(row))
	}
	return items, nil
}

func (r *SQLCRepository) ListChatTraces(ctx context.Context, sellerAccountID int64, filter ChatTraceFilter, page Page) ([]ChatTraceLogItem, error) {
	rows, err := r.q.AdminListChatTracesBySeller(ctx, dbgen.AdminListChatTracesBySellerParams{
		SellerAccountID: sellerAccountID,
		Status:          optionalText(filter.Status),
		DetectedIntent:  optionalText(filter.Intent),
		SessionID:       int8FromPtr(filter.SessionID),
		PageLimit:       page.Limit,
		PageOffset:      page.Offset,
	})
	if err != nil {
		return nil, err
	}
	items := make([]ChatTraceLogItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, ChatTraceLogItem{
			ID:                   row.ID,
			SessionID:            row.SessionID,
			UserMessageID:        int64Ptr(row.UserMessageID),
			AssistantMessageID:   int64Ptr(row.AssistantMessageID),
			Status:               row.Status,
			DetectedIntent:       textPtr(row.DetectedIntent),
			PlannerModel:         row.PlannerModel,
			AnswerModel:          row.AnswerModel,
			PlannerPromptVersion: row.PlannerPromptVersion,
			AnswerPromptVersion:  row.AnswerPromptVersion,
			InputTokens:          row.InputTokens,
			OutputTokens:         row.OutputTokens,
			EstimatedCost:        numericToFloat64(row.EstimatedCost),
			StartedAt:            timePtr(row.StartedAt),
			FinishedAt:           timePtr(row.FinishedAt),
			ErrorMessage:         textPtr(row.ErrorMessage),
		})
	}
	return items, nil
}

func (r *SQLCRepository) ListChatSessions(ctx context.Context, sellerAccountID int64, filter ChatSessionFilter, page Page) ([]ChatSessionItem, error) {
	rows, err := r.q.AdminListChatSessionsBySeller(ctx, dbgen.AdminListChatSessionsBySellerParams{
		SellerAccountID: sellerAccountID,
		Status:          optionalText(filter.Status),
		PageLimit:       page.Limit,
		PageOffset:      page.Offset,
	})
	if err != nil {
		return nil, err
	}
	items := make([]ChatSessionItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, ChatSessionItem{
			ID:            row.ID,
			Title:         row.Title,
			Status:        row.Status,
			CreatedAt:     timeValue(row.CreatedAt),
			UpdatedAt:     timeValue(row.UpdatedAt),
			LastMessageAt: timePtr(row.LastMessageAt),
		})
	}
	return items, nil
}

func (r *SQLCRepository) GetChatSession(ctx context.Context, sellerAccountID, sessionID int64) (*ChatSessionItem, error) {
	row, err := r.q.AdminGetChatSessionByID(ctx, dbgen.AdminGetChatSessionByIDParams{
		SellerAccountID: sellerAccountID,
		ID:              sessionID,
	})
	if err != nil {
		if errorsIsNoRows(err) {
			return nil, nil
		}
		return nil, err
	}
	return &ChatSessionItem{
		ID:            row.ID,
		Title:         row.Title,
		Status:        row.Status,
		CreatedAt:     timeValue(row.CreatedAt),
		UpdatedAt:     timeValue(row.UpdatedAt),
		LastMessageAt: timePtr(row.LastMessageAt),
	}, nil
}

func (r *SQLCRepository) ListChatMessages(ctx context.Context, sellerAccountID, sessionID int64, _ ChatMessageFilter, page Page) ([]ChatMessageItem, error) {
	rows, err := r.q.AdminListChatMessagesBySession(ctx, dbgen.AdminListChatMessagesBySessionParams{
		SellerAccountID: sellerAccountID,
		SessionID:       sessionID,
		PageLimit:       page.Limit,
		PageOffset:      page.Offset,
	})
	if err != nil {
		return nil, err
	}
	items := make([]ChatMessageItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, ChatMessageItem{
			ID:          row.ID,
			SessionID:   row.SessionID,
			Role:        row.Role,
			MessageType: row.MessageType,
			Content:     row.Content,
			CreatedAt:   timeValue(row.CreatedAt),
		})
	}
	return items, nil
}

func (r *SQLCRepository) ListChatFeedback(ctx context.Context, filter ChatFeedbackFilter, page Page) ([]ChatFeedbackItem, error) {
	rows, err := r.q.AdminListChatFeedback(ctx, dbgen.AdminListChatFeedbackParams{
		SellerAccountID: int8FromPtr(filter.SellerAccountID),
		Rating:          optionalText(filter.Rating),
		PageLimit:       page.Limit,
		PageOffset:      page.Offset,
	})
	if err != nil {
		return nil, err
	}
	items := make([]ChatFeedbackItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, ChatFeedbackItem{
			ID:              row.ID,
			SellerAccountID: row.SellerAccountID,
			SellerName:      optionalStringPtr(row.SellerName),
			SessionID:       row.SessionID,
			MessageID:       row.MessageID,
			Rating:          row.Rating,
			Comment:         textPtr(row.Comment),
			MessageRole:     optionalStringPtr(row.MessageRole),
			MessageType:     optionalStringPtr(row.MessageType),
			MessageContent:  optionalStringPtr(row.MessageContent),
			SessionTitle:    optionalStringPtr(row.SessionTitle),
			TraceID:         int64Ptr(row.TraceID),
			CreatedAt:       timeValue(row.CreatedAt),
		})
	}
	return items, nil
}

func (r *SQLCRepository) ListRecommendationFeedback(ctx context.Context, sellerAccountID int64, filter RecommendationFeedbackFilter, page Page) ([]RecommendationFeedbackItem, error) {
	rows, err := r.q.AdminListRecommendationFeedbackBySeller(ctx, dbgen.AdminListRecommendationFeedbackBySellerParams{
		SellerAccountID:      sellerAccountID,
		Rating:               optionalText(filter.Rating),
		RecommendationStatus: optionalText(filter.Status),
		PageLimit:            page.Limit,
		PageOffset:           page.Offset,
	})
	if err != nil {
		return nil, err
	}
	items := make([]RecommendationFeedbackItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, RecommendationFeedbackItem{
			ID:                      row.ID,
			SellerAccountID:         row.SellerAccountID,
			RecommendationID:        row.RecommendationID,
			Rating:                  row.Rating,
			Comment:                 textPtr(row.Comment),
			CreatedAt:               timeValue(row.FeedbackCreatedAt),
			RecommendationType:      row.RecommendationType,
			Title:                   row.Title,
			PriorityLevel:           row.PriorityLevel,
			ConfidenceLevel:         row.ConfidenceLevel,
			RecommendationStatus:    row.RecommendationStatus,
			EntityType:              row.EntityType,
			EntityID:                textPtr(row.EntityID),
			EntitySKU:               int64Ptr(row.EntitySku),
			EntityOfferID:           textPtr(row.EntityOfferID),
			RecommendationCreatedAt: timeValue(row.RecommendationCreatedAt),
		})
	}
	return items, nil
}

func (r *SQLCRepository) GetRecommendationProxyFeedbackCounts(ctx context.Context, sellerAccountID int64) (RecommendationFeedbackProxyStatus, error) {
	row, err := r.q.AdminGetRecommendationProxyFeedbackCounts(ctx, sellerAccountID)
	if err != nil {
		return RecommendationFeedbackProxyStatus{}, err
	}
	return RecommendationFeedbackProxyStatus{
		AcceptedCount:  row.AcceptedCount,
		DismissedCount: row.DismissedCount,
		ResolvedCount:  row.ResolvedCount,
	}, nil
}

func (r *SQLCRepository) PeekRecommendationForAudit(ctx context.Context, sellerAccountID, recommendationID int64) (RecommendationViewAuditMeta, error) {
	row, err := r.q.AdminPeekRecommendationForAudit(ctx, dbgen.AdminPeekRecommendationForAuditParams{
		ID:              recommendationID,
		SellerAccountID: sellerAccountID,
	})
	if err != nil {
		return RecommendationViewAuditMeta{}, err
	}
	return RecommendationViewAuditMeta{
		ID:              row.ID,
		AIModel:         row.AiModel,
		AIPromptVersion: row.AiPromptVersion,
	}, nil
}

func (r *SQLCRepository) PeekChatTraceForAudit(ctx context.Context, sellerAccountID, traceID int64) (ChatTraceViewAuditMeta, error) {
	row, err := r.q.AdminPeekChatTraceForAudit(ctx, dbgen.AdminPeekChatTraceForAuditParams{
		ID:              traceID,
		SellerAccountID: sellerAccountID,
	})
	if err != nil {
		return ChatTraceViewAuditMeta{}, err
	}
	return ChatTraceViewAuditMeta{
		ID:                   row.ID,
		SessionID:            row.SessionID,
		UserMessageID:        int64Ptr(row.UserMessageID),
		AssistantMessageID:   int64Ptr(row.AssistantMessageID),
		PlannerModel:         row.PlannerModel,
		AnswerModel:          row.AnswerModel,
		PlannerPromptVersion: row.PlannerPromptVersion,
		AnswerPromptVersion:  row.AnswerPromptVersion,
		Status:               row.Status,
	}, nil
}

func (r *SQLCRepository) PeekRecommendationRunForAudit(ctx context.Context, sellerAccountID, runID int64) (RecommendationRunViewAuditMeta, error) {
	row, err := r.q.AdminPeekRecommendationRunForAudit(ctx, dbgen.AdminPeekRecommendationRunForAuditParams{
		ID:              runID,
		SellerAccountID: sellerAccountID,
	})
	if err != nil {
		return RecommendationRunViewAuditMeta{}, err
	}
	return RecommendationRunViewAuditMeta{
		ID:              row.ID,
		RunType:         row.RunType,
		Status:          row.Status,
		AIModel:         row.AiModel,
		AIPromptVersion: row.AiPromptVersion,
	}, nil
}

func (r *SQLCRepository) GetChatTraceDetail(ctx context.Context, sellerAccountID, traceID int64) (*ChatTraceDetail, error) {
	row, err := r.q.GetChatTraceByID(ctx, dbgen.GetChatTraceByIDParams{SellerAccountID: sellerAccountID, ID: traceID})
	if err != nil {
		return nil, err
	}
	messageRows, err := r.q.AdminListChatMessagesBySession(ctx, dbgen.AdminListChatMessagesBySessionParams{
		SellerAccountID: sellerAccountID,
		SessionID:       row.SessionID,
		PageLimit:       200,
		PageOffset:      0,
	})
	if err != nil && !errorsIsNoRows(err) {
		return nil, err
	}
	messages := make([]ChatMessageItem, 0, len(messageRows))
	for _, message := range messageRows {
		messages = append(messages, ChatMessageItem{
			ID:          message.ID,
			SessionID:   message.SessionID,
			Role:        message.Role,
			MessageType: message.MessageType,
			Content:     message.Content,
			CreatedAt:   timeValue(message.CreatedAt),
		})
	}
	return &ChatTraceDetail{
		ID:                       row.ID,
		SessionID:                row.SessionID,
		UserMessageID:            int64Ptr(row.UserMessageID),
		AssistantMessageID:       int64Ptr(row.AssistantMessageID),
		SellerAccountID:          row.SellerAccountID,
		Status:                   row.Status,
		PlannerPromptVersion:     row.PlannerPromptVersion,
		AnswerPromptVersion:      row.AnswerPromptVersion,
		PlannerModel:             row.PlannerModel,
		AnswerModel:              row.AnswerModel,
		DetectedIntent:           textPtr(row.DetectedIntent),
		ToolPlanPayload:          jsonMapFromRaw(row.ToolPlanPayload),
		ValidatedToolPlanPayload: jsonMapFromRaw(row.ValidatedToolPlanPayload),
		ToolResultsPayload:       jsonMapFromRaw(row.ToolResultsPayload),
		FactContextPayload:       jsonMapFromRaw(row.FactContextPayload),
		RawPlannerResponse:       jsonMapFromRaw(row.RawPlannerResponse),
		RawAnswerResponse:        jsonMapFromRaw(row.RawAnswerResponse),
		AnswerValidationPayload:  jsonMapFromRaw(row.AnswerValidationPayload),
		InputTokens:              row.InputTokens,
		OutputTokens:             row.OutputTokens,
		EstimatedCost:            numericToFloat64(row.EstimatedCost),
		ErrorMessage:             textPtr(row.ErrorMessage),
		StartedAt:                timePtr(row.StartedAt),
		FinishedAt:               timePtr(row.FinishedAt),
		CreatedAt:                timeValue(row.CreatedAt),
		Messages:                 messages,
		Limitations:              []string{},
	}, nil
}

func (r *SQLCRepository) GetRecommendationRunDetail(ctx context.Context, sellerAccountID, runID int64) (*RecommendationRunDetail, error) {
	run, err := r.q.AdminGetRecommendationRunByID(ctx, dbgen.AdminGetRecommendationRunByIDParams{
		SellerAccountID: sellerAccountID,
		ID:              runID,
	})
	if err != nil {
		return nil, err
	}
	diagnostics, err := r.q.ListRecommendationRunDiagnosticsByRun(ctx, dbgen.ListRecommendationRunDiagnosticsByRunParams{
		SellerAccountID:     sellerAccountID,
		RecommendationRunID: pgtype.Int8{Int64: runID, Valid: true},
		Limit:               200,
		Offset:              0,
	})
	if err != nil && !errorsIsNoRows(err) {
		return nil, err
	}

	items := make([]RecommendationRunDiagnosticItem, 0, len(diagnostics))
	for _, row := range diagnostics {
		items = append(items, RecommendationRunDiagnosticItem{
			ID:                      row.ID,
			RecommendationRunID:     int64Ptr(row.RecommendationRunID),
			OpenAIRequestID:         textPtr(row.OpenaiRequestID),
			AIModel:                 textPtr(row.AiModel),
			PromptVersion:           textPtr(row.PromptVersion),
			ContextPayloadSummary:   jsonMapFromRaw(row.ContextPayloadSummary),
			RawOpenAIResponse:       jsonMapFromRaw(row.RawOpenaiResponse),
			ValidationResultPayload: jsonMapFromRaw(row.ValidationResultPayload),
			RejectedItemsPayload:    jsonMapFromRaw(row.RejectedItemsPayload),
			ErrorStage:              textPtr(row.ErrorStage),
			ErrorMessage:            textPtr(row.ErrorMessage),
			InputTokens:             row.InputTokens,
			OutputTokens:            row.OutputTokens,
			EstimatedCost:           numericToFloat64(row.EstimatedCost),
			CreatedAt:               timeValue(row.CreatedAt),
		})
	}

	return &RecommendationRunDetail{
		Run:             recommendationRunSummaryFromDB(run),
		Recommendations: []RecommendationItem{},
		Diagnostics:     items,
		Limitations:     []string{},
	}, nil
}

func (r *SQLCRepository) GetRecommendationRawAI(ctx context.Context, sellerAccountID, recommendationID int64) (*RecommendationRawAI, error) {
	row, err := r.q.GetRecommendationByID(ctx, dbgen.GetRecommendationByIDParams{
		ID:              recommendationID,
		SellerAccountID: sellerAccountID,
	})
	if err != nil {
		return nil, err
	}
	alertRows, err := r.q.ListAlertsByRecommendationID(ctx, dbgen.ListAlertsByRecommendationIDParams{
		SellerAccountID:  sellerAccountID,
		RecommendationID: recommendationID,
	})
	if err != nil {
		return nil, err
	}
	relatedAlerts := make([]RecommendationAlertItem, 0, len(alertRows))
	for _, alert := range alertRows {
		relatedAlerts = append(relatedAlerts, RecommendationAlertItem{
			ID:         alert.ID,
			AlertType:  alert.AlertType,
			AlertGroup: alert.AlertGroup,
			Severity:   alert.Severity,
			Urgency:    alert.Urgency,
			Title:      alert.Title,
			Status:     alert.Status,
		})
	}
	return &RecommendationRawAI{
		Recommendation: recommendationItemFromDB(row),
		RelatedAlerts:  relatedAlerts,
		Diagnostics:    []RecommendationRunDiagnosticItem{},
		Limitations: []string{
			"Recommendation-level diagnostics linkage is unavailable in current schema.",
		},
	}, nil
}

func (r *SQLCRepository) GetBillingState(ctx context.Context, sellerAccountID int64) (*BillingState, error) {
	row, err := r.q.GetSellerBillingState(ctx, sellerAccountID)
	if err != nil {
		if errorsIsNoRows(err) {
			return nil, nil
		}
		return nil, err
	}
	state := billingStateFromDB(row)
	return &state, nil
}

func (r *SQLCRepository) ListBillingStates(ctx context.Context, filter BillingStateFilter) ([]BillingState, error) {
	page := NormalizePage(filter.Limit, filter.Offset, 50, 200)
	rows, err := r.q.ListSellerBillingStates(ctx, dbgen.ListSellerBillingStatesParams{
		Status:     optionalText(filter.Status),
		PageOffset: page.Offset,
		PageLimit:  page.Limit,
	})
	if err != nil {
		return nil, err
	}
	items := make([]BillingState, 0, len(rows))
	for _, row := range rows {
		items = append(items, billingStateFromDB(row))
	}
	return items, nil
}

func (r *SQLCRepository) CreateAdminActionLog(ctx context.Context, input CreateAdminActionLogInput) (*AdminActionLog, error) {
	row, err := r.q.CreateAdminActionLog(ctx, dbgen.CreateAdminActionLogParams{
		AdminUserID:     int8FromPtr(input.AdminUserID),
		AdminEmail:      input.AdminEmail,
		SellerAccountID: input.SellerAccountID,
		ActionType:      string(input.ActionType),
		TargetType:      textFromPtr(input.TargetType),
		TargetID:        int8FromPtr(input.TargetID),
		RequestPayload:  jsonBytes(input.RequestPayload),
		Status:          textFromString(string(input.Status)),
	})
	if err != nil {
		return nil, err
	}
	action := adminActionFromDB(row)
	return &action, nil
}

func (r *SQLCRepository) CompleteAdminActionLog(ctx context.Context, input CompleteAdminActionLogInput) (*AdminActionLog, error) {
	row, err := r.q.CompleteAdminActionLog(ctx, dbgen.CompleteAdminActionLogParams{
		ID:            input.ID,
		ResultPayload: jsonBytes(input.ResultPayload),
	})
	if err != nil {
		return nil, err
	}
	action := adminActionFromDB(row)
	return &action, nil
}

func (r *SQLCRepository) FailAdminActionLog(ctx context.Context, input FailAdminActionLogInput) (*AdminActionLog, error) {
	row, err := r.q.FailAdminActionLog(ctx, dbgen.FailAdminActionLogParams{
		ID:            input.ID,
		ResultPayload: jsonBytes(input.ResultPayload),
		ErrorMessage:  textFromString(input.ErrorMessage),
	})
	if err != nil {
		return nil, err
	}
	action := adminActionFromDB(row)
	return &action, nil
}

func (r *SQLCRepository) UpsertBillingState(ctx context.Context, input UpsertBillingStateInput) (*BillingState, error) {
	row, err := r.q.UpsertSellerBillingState(ctx, dbgen.UpsertSellerBillingStateParams{
		SellerAccountID:      input.SellerAccountID,
		PlanCode:             input.PlanCode,
		Status:               string(input.Status),
		TrialEndsAt:          timeFromPtr(input.TrialEndsAt),
		CurrentPeriodStart:   timeFromPtr(input.CurrentPeriodStart),
		CurrentPeriodEnd:     timeFromPtr(input.CurrentPeriodEnd),
		AiTokensLimitMonth:   int8FromPtr(input.AITokensLimitMonth),
		AiTokensUsedMonth:    input.AITokensUsedMonth,
		EstimatedAiCostMonth: numericFromFloat64(input.EstimatedAICostMonth),
		Notes:                textFromPtr(input.Notes),
	})
	if err != nil {
		return nil, err
	}
	state := billingStateFromDB(row)
	return &state, nil
}

func adminActionFromDB(row dbgen.AdminActionLog) AdminActionLog {
	return AdminActionLog{
		ID:              row.ID,
		AdminUserID:     int64Ptr(row.AdminUserID),
		AdminEmail:      row.AdminEmail,
		SellerAccountID: row.SellerAccountID,
		ActionType:      AdminActionType(row.ActionType),
		TargetType:      textPtr(row.TargetType),
		TargetID:        int64Ptr(row.TargetID),
		RequestPayload:  jsonMapFromRaw(row.RequestPayload),
		ResultPayload:   jsonMapFromRaw(row.ResultPayload),
		Status:          AdminActionStatus(row.Status),
		ErrorMessage:    textPtr(row.ErrorMessage),
		CreatedAt:       timeValue(row.CreatedAt),
		FinishedAt:      timePtr(row.FinishedAt),
	}
}

func billingStateFromDB(row dbgen.SellerBillingState) BillingState {
	return BillingState{
		SellerAccountID:      row.SellerAccountID,
		PlanCode:             row.PlanCode,
		Status:               BillingStatus(row.Status),
		TrialEndsAt:          timePtr(row.TrialEndsAt),
		CurrentPeriodStart:   timePtr(row.CurrentPeriodStart),
		CurrentPeriodEnd:     timePtr(row.CurrentPeriodEnd),
		AITokensLimitMonth:   int64Ptr(row.AiTokensLimitMonth),
		AITokensUsedMonth:    row.AiTokensUsedMonth,
		EstimatedAICostMonth: numericToFloat64(row.EstimatedAiCostMonth),
		Notes:                textPtr(row.Notes),
		CreatedAt:            timeValue(row.CreatedAt),
		UpdatedAt:            timeValue(row.UpdatedAt),
	}
}

func recommendationRunSummaryFromDB(row dbgen.RecommendationRun) RecommendationRunSummary {
	var rejected *int32
	if row.GeneratedRecommendationsCount >= row.AcceptedRecommendationsCount {
		value := row.GeneratedRecommendationsCount - row.AcceptedRecommendationsCount
		rejected = &value
	}
	return RecommendationRunSummary{
		ID:                            row.ID,
		RunType:                       row.RunType,
		Status:                        row.Status,
		AsOfDate:                      datePtr(row.AsOfDate),
		AIModel:                       textPtr(row.AiModel),
		AIPromptVersion:               textPtr(row.AiPromptVersion),
		StartedAt:                     timePtr(row.StartedAt),
		FinishedAt:                    timePtr(row.FinishedAt),
		InputTokens:                   row.InputTokens,
		OutputTokens:                  row.OutputTokens,
		EstimatedCost:                 numericToFloat64(row.EstimatedCost),
		GeneratedRecommendationsCount: row.GeneratedRecommendationsCount,
		AcceptedRecommendationsCount:  row.AcceptedRecommendationsCount,
		RejectedRecommendationsCount:  rejected,
		ErrorMessage:                  textPtr(row.ErrorMessage),
	}
}

func recommendationItemFromDB(row dbgen.Recommendation) RecommendationItem {
	return RecommendationItem{
		ID:                       row.ID,
		RecommendationType:       row.RecommendationType,
		Title:                    row.Title,
		Status:                   row.Status,
		PriorityLevel:            row.PriorityLevel,
		ConfidenceLevel:          row.ConfidenceLevel,
		Horizon:                  row.Horizon,
		EntityType:               row.EntityType,
		EntityID:                 textPtr(row.EntityID),
		EntitySKU:                int64Ptr(row.EntitySku),
		EntityOfferID:            textPtr(row.EntityOfferID),
		WhatHappened:             optionalStringPtr(row.WhatHappened),
		WhyItMatters:             optionalStringPtr(row.WhyItMatters),
		RecommendedAction:        optionalStringPtr(row.RecommendedAction),
		ExpectedEffect:           textPtr(row.ExpectedEffect),
		SupportingMetricsPayload: jsonMapFromRaw(row.SupportingMetricsPayload),
		ConstraintsPayload:       jsonMapFromRaw(row.ConstraintsPayload),
		RawAIResponse:            jsonMapFromRaw(row.RawAiResponse),
		CreatedAt:                timeValue(row.CreatedAt),
		UpdatedAt:                timeValue(row.UpdatedAt),
	}
}

func jsonMapFromRaw(raw []byte) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err == nil {
		return out
	}
	return map[string]any{"raw": string(raw)}
}

func jsonBytes(v map[string]any) []byte {
	if v == nil {
		return []byte("{}")
	}
	b, err := json.Marshal(v)
	if err != nil {
		return []byte("{}")
	}
	return b
}

func numericToFloat64(v pgtype.Numeric) float64 {
	f, err := v.Float64Value()
	if err != nil || !f.Valid {
		return 0
	}
	return f.Float64
}

func numericFromFloat64(v float64) pgtype.Numeric {
	n := pgtype.Numeric{}
	_ = n.Scan(fmt.Sprintf("%f", v))
	return n
}

func optionalText(v string) pgtype.Text {
	if strings.TrimSpace(v) == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: strings.TrimSpace(v), Valid: true}
}

func textFromString(v string) pgtype.Text {
	if strings.TrimSpace(v) == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: strings.TrimSpace(v), Valid: true}
}

func textFromPtr(v *string) pgtype.Text {
	if v == nil || strings.TrimSpace(*v) == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: strings.TrimSpace(*v), Valid: true}
}

func textPtr(v pgtype.Text) *string {
	if !v.Valid {
		return nil
	}
	s := v.String
	return &s
}

func textPtrFromString(v string) *string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	s := v
	return &s
}

func timeFromPtr(v *time.Time) pgtype.Timestamptz {
	if v == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *v, Valid: true}
}

func timePtr(v pgtype.Timestamptz) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time
	return &t
}

func timeValue(v pgtype.Timestamptz) time.Time {
	if !v.Valid {
		return time.Time{}
	}
	return v.Time
}

func datePtr(v pgtype.Date) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time
	return &t
}

func optionalStringPtr(v string) *string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	out := v
	return &out
}

func int8FromPtr(v *int64) pgtype.Int8 {
	if v == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *v, Valid: true}
}

func int64Ptr(v pgtype.Int8) *int64 {
	if !v.Valid {
		return nil
	}
	n := v.Int64
	return &n
}

func errorsIsNoRows(err error) bool {
	return err == pgx.ErrNoRows || (err != nil && strings.Contains(err.Error(), pgx.ErrNoRows.Error()))
}

func (r *SQLCRepository) String() string {
	return fmt.Sprintf("SQLCRepository")
}
