package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type Repository interface {
	CreateSession(ctx context.Context, input CreateSessionInput) (*ChatSession, error)
	GetSession(ctx context.Context, sellerAccountID, sessionID int64) (*ChatSession, error)
	GetSessionByActionKey(ctx context.Context, sellerAccountID int64, actionKey string) (*ChatSession, error)
	ReactivateSession(ctx context.Context, sellerAccountID, sessionID int64) (*ChatSession, error)
	ListSessions(ctx context.Context, sellerAccountID int64, limit, offset int32) ([]ChatSession, error)
	ListActiveSessions(ctx context.Context, sellerAccountID int64, limit, offset int32) ([]ChatSession, error)
	ArchiveSession(ctx context.Context, sellerAccountID, sessionID int64) (*ChatSession, error)
	SoftDeleteSession(ctx context.Context, sellerAccountID, sessionID int64) (*ChatSession, error)
	TouchSession(ctx context.Context, sellerAccountID, sessionID int64) (*ChatSession, error)
	UpdateSessionTitle(ctx context.Context, sellerAccountID, sessionID int64, title string) (*ChatSession, error)

	CreateMessage(ctx context.Context, input CreateMessageInput) (*ChatMessage, error)
	GetMessage(ctx context.Context, sellerAccountID, messageID int64) (*ChatMessage, error)
	GetMessageByClientRequestID(ctx context.Context, sellerAccountID int64, clientRequestID string) (*ChatMessage, error)
	ListMessages(ctx context.Context, sellerAccountID, sessionID int64, limit, offset int32) ([]ChatMessage, error)
	ListRecentMessages(ctx context.Context, sellerAccountID, sessionID int64, limit int32) ([]ChatMessage, error)

	CreateTrace(ctx context.Context, input CreateTraceInput) (*ChatTrace, error)
	CompleteTrace(ctx context.Context, input CompleteTraceInput) (*ChatTrace, error)
	FailTrace(ctx context.Context, input FailTraceInput) (*ChatTrace, error)
	GetTrace(ctx context.Context, sellerAccountID, traceID int64) (*ChatTrace, error)
	GetLatestTraceBySession(ctx context.Context, sellerAccountID, sessionID int64) (*ChatTrace, error)
	ListTracesBySession(ctx context.Context, sellerAccountID, sessionID int64, limit, offset int32) ([]ChatTrace, error)

	CreateFeedback(ctx context.Context, input AddFeedbackInput) (*ChatFeedback, error)
	GetFeedbackByMessage(ctx context.Context, sellerAccountID, messageID int64) (*ChatFeedback, error)
	ListFeedback(ctx context.Context, sellerAccountID int64, limit, offset int32) ([]ChatFeedback, error)
}

type CreateTraceInput struct {
	SessionID            int64
	UserMessageID        *int64
	SellerAccountID      int64
	PlannerPromptVersion string
	AnswerPromptVersion  string
	PlannerModel         string
	AnswerModel          string
}

type CompleteTraceInput struct {
	SellerAccountID          int64
	TraceID                  int64
	AssistantMessageID       *int64
	DetectedIntent           ChatIntent
	ToolPlanPayload          map[string]any
	ValidatedToolPlanPayload map[string]any
	ToolResultsPayload       map[string]any
	FactContextPayload       map[string]any
	RawPlannerResponse       map[string]any
	RawAnswerResponse        map[string]any
	AnswerValidationPayload  map[string]any
	InputTokens              int32
	OutputTokens             int32
	EstimatedCost            float64
}

type FailTraceInput struct {
	SellerAccountID          int64
	TraceID                  int64
	DetectedIntent           ChatIntent
	ToolPlanPayload          map[string]any
	ValidatedToolPlanPayload map[string]any
	ToolResultsPayload       map[string]any
	FactContextPayload       map[string]any
	RawPlannerResponse       map[string]any
	RawAnswerResponse        map[string]any
	AnswerValidationPayload  map[string]any
	InputTokens              int32
	OutputTokens             int32
	EstimatedCost            float64
	ErrorMessage             string
}

type SQLCRepository struct {
	q *dbgen.Queries
}

func NewSQLCRepository(q *dbgen.Queries) *SQLCRepository {
	return &SQLCRepository{q: q}
}

func (r *SQLCRepository) CreateSession(ctx context.Context, input CreateSessionInput) (*ChatSession, error) {
	row, err := r.q.CreateChatSession(ctx, dbgen.CreateChatSessionParams{
		SellerAccountID: input.SellerAccountID,
		UserID:          nullableInt64(input.UserID),
		Title:           normalizeSessionTitle(input.Title),
		ActionKey:       nullableTextPtr(input.ActionKey),
	})
	if err != nil {
		return nil, fmt.Errorf("create chat session: %w", err)
	}
	item := mapChatSession(row)
	return &item, nil
}

func (r *SQLCRepository) GetSessionByActionKey(ctx context.Context, sellerAccountID int64, actionKey string) (*ChatSession, error) {
	row, err := r.q.GetChatSessionByActionKey(ctx, dbgen.GetChatSessionByActionKeyParams{
		SellerAccountID: sellerAccountID,
		ActionKey:       nullableText(actionKey),
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get chat session by action_key: %w", err)
	}
	item := mapChatSession(row)
	return &item, nil
}

func (r *SQLCRepository) ReactivateSession(ctx context.Context, sellerAccountID, sessionID int64) (*ChatSession, error) {
	row, err := r.q.ReactivateChatSession(ctx, dbgen.ReactivateChatSessionParams{
		SellerAccountID: sellerAccountID,
		ID:              sessionID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("reactivate chat session: %w", err)
	}
	item := mapChatSession(row)
	return &item, nil
}

func (r *SQLCRepository) GetSession(ctx context.Context, sellerAccountID, sessionID int64) (*ChatSession, error) {
	row, err := r.q.GetChatSessionByID(ctx, dbgen.GetChatSessionByIDParams{
		SellerAccountID: sellerAccountID,
		ID:              sessionID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get chat session id=%d: %w", sessionID, err)
	}
	item := mapChatSession(row)
	return &item, nil
}

func (r *SQLCRepository) ListSessions(ctx context.Context, sellerAccountID int64, limit, offset int32) ([]ChatSession, error) {
	rows, err := r.q.ListChatSessionsBySellerAccountID(ctx, dbgen.ListChatSessionsBySellerAccountIDParams{
		SellerAccountID: sellerAccountID,
		Limit:           normalizeLimit(limit),
		Offset:          normalizeOffset(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("list chat sessions: %w", err)
	}
	out := make([]ChatSession, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapChatSession(row))
	}
	return out, nil
}

func (r *SQLCRepository) ListActiveSessions(ctx context.Context, sellerAccountID int64, limit, offset int32) ([]ChatSession, error) {
	rows, err := r.q.ListActiveChatSessionsBySellerAccountID(ctx, dbgen.ListActiveChatSessionsBySellerAccountIDParams{
		SellerAccountID: sellerAccountID,
		Limit:           normalizeLimit(limit),
		Offset:          normalizeOffset(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("list active chat sessions: %w", err)
	}
	out := make([]ChatSession, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapChatSession(row))
	}
	return out, nil
}

func (r *SQLCRepository) ArchiveSession(ctx context.Context, sellerAccountID, sessionID int64) (*ChatSession, error) {
	row, err := r.q.ArchiveChatSession(ctx, dbgen.ArchiveChatSessionParams{
		SellerAccountID: sellerAccountID,
		ID:              sessionID,
	})
	if err != nil {
		return nil, fmt.Errorf("archive chat session id=%d: %w", sessionID, err)
	}
	item := mapChatSession(row)
	return &item, nil
}

func (r *SQLCRepository) TouchSession(ctx context.Context, sellerAccountID, sessionID int64) (*ChatSession, error) {
	row, err := r.q.TouchChatSession(ctx, dbgen.TouchChatSessionParams{
		SellerAccountID: sellerAccountID,
		ID:              sessionID,
	})
	if err != nil {
		return nil, fmt.Errorf("touch chat session id=%d: %w", sessionID, err)
	}
	item := mapChatSession(row)
	return &item, nil
}

func (r *SQLCRepository) SoftDeleteSession(ctx context.Context, sellerAccountID, sessionID int64) (*ChatSession, error) {
	row, err := r.q.SoftDeleteChatSession(ctx, dbgen.SoftDeleteChatSessionParams{
		SellerAccountID: sellerAccountID,
		ID:              sessionID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("soft delete chat session id=%d: %w", sessionID, err)
	}
	item := mapChatSession(row)
	return &item, nil
}

func (r *SQLCRepository) UpdateSessionTitle(ctx context.Context, sellerAccountID, sessionID int64, title string) (*ChatSession, error) {
	row, err := r.q.UpdateChatSessionTitle(ctx, dbgen.UpdateChatSessionTitleParams{
		SellerAccountID: sellerAccountID,
		ID:              sessionID,
		Title:           normalizeSessionTitle(title),
	})
	if err != nil {
		return nil, fmt.Errorf("update chat session title id=%d: %w", sessionID, err)
	}
	item := mapChatSession(row)
	return &item, nil
}

func (r *SQLCRepository) CreateMessage(ctx context.Context, input CreateMessageInput) (*ChatMessage, error) {
	row, err := r.q.CreateChatMessage(ctx, dbgen.CreateChatMessageParams{
		SessionID:       input.SessionID,
		SellerAccountID: input.SellerAccountID,
		Role:            string(input.Role),
		Content:         input.Content,
		MessageType:     string(input.MessageType),
		ClientRequestID: nullableText(input.ClientRequestID),
	})
	if err != nil {
		return nil, fmt.Errorf("create chat message: %w", err)
	}
	item := mapChatMessage(row)
	return &item, nil
}

func (r *SQLCRepository) GetMessageByClientRequestID(
	ctx context.Context,
	sellerAccountID int64,
	clientRequestID string,
) (*ChatMessage, error) {
	row, err := r.q.GetChatMessageByClientRequestID(ctx, dbgen.GetChatMessageByClientRequestIDParams{
		SellerAccountID: sellerAccountID,
		ClientRequestID: nullableText(clientRequestID),
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get chat message by client_request_id: %w", err)
	}
	item := mapChatMessage(row)
	return &item, nil
}

func (r *SQLCRepository) GetMessage(ctx context.Context, sellerAccountID, messageID int64) (*ChatMessage, error) {
	row, err := r.q.GetChatMessageByID(ctx, dbgen.GetChatMessageByIDParams{
		SellerAccountID: sellerAccountID,
		ID:              messageID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get chat message id=%d: %w", messageID, err)
	}
	item := mapChatMessage(row)
	return &item, nil
}

func (r *SQLCRepository) ListMessages(ctx context.Context, sellerAccountID, sessionID int64, limit, offset int32) ([]ChatMessage, error) {
	rows, err := r.q.ListChatMessagesBySessionID(ctx, dbgen.ListChatMessagesBySessionIDParams{
		SellerAccountID: sellerAccountID,
		SessionID:       sessionID,
		Limit:           normalizeLimit(limit),
		Offset:          normalizeOffset(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("list chat messages: %w", err)
	}
	out := make([]ChatMessage, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapChatMessage(row))
	}
	return out, nil
}

func (r *SQLCRepository) ListRecentMessages(ctx context.Context, sellerAccountID, sessionID int64, limit int32) ([]ChatMessage, error) {
	rows, err := r.q.ListRecentChatMessagesBySessionID(ctx, dbgen.ListRecentChatMessagesBySessionIDParams{
		SellerAccountID: sellerAccountID,
		SessionID:       sessionID,
		Limit:           normalizeLimit(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list recent chat messages: %w", err)
	}
	out := make([]ChatMessage, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapChatMessage(row))
	}
	return out, nil
}

func (r *SQLCRepository) CreateTrace(ctx context.Context, input CreateTraceInput) (*ChatTrace, error) {
	row, err := r.q.CreateChatTrace(ctx, dbgen.CreateChatTraceParams{
		SessionID:            input.SessionID,
		UserMessageID:        nullableInt64(input.UserMessageID),
		SellerAccountID:      input.SellerAccountID,
		PlannerPromptVersion: input.PlannerPromptVersion,
		AnswerPromptVersion:  input.AnswerPromptVersion,
		PlannerModel:         input.PlannerModel,
		AnswerModel:          input.AnswerModel,
	})
	if err != nil {
		return nil, fmt.Errorf("create chat trace: %w", err)
	}
	item, mapErr := mapChatTrace(row)
	if mapErr != nil {
		return nil, mapErr
	}
	return &item, nil
}

func (r *SQLCRepository) CompleteTrace(ctx context.Context, input CompleteTraceInput) (*ChatTrace, error) {
	estimated, err := numericFromFloat(input.EstimatedCost, 6)
	if err != nil {
		return nil, fmt.Errorf("convert estimated cost: %w", err)
	}
	row, err := r.q.CompleteChatTrace(ctx, dbgen.CompleteChatTraceParams{
		SellerAccountID:          input.SellerAccountID,
		ID:                       input.TraceID,
		AssistantMessageID:       nullableInt64(input.AssistantMessageID),
		DetectedIntent:           nullableText(string(input.DetectedIntent)),
		ToolPlanPayload:          toJSONBytes(input.ToolPlanPayload),
		ValidatedToolPlanPayload: toJSONBytes(input.ValidatedToolPlanPayload),
		ToolResultsPayload:       toJSONBytes(input.ToolResultsPayload),
		FactContextPayload:       toJSONBytes(input.FactContextPayload),
		RawPlannerResponse:       toJSONBytes(input.RawPlannerResponse),
		RawAnswerResponse:        toJSONBytes(input.RawAnswerResponse),
		AnswerValidationPayload:  toJSONBytes(input.AnswerValidationPayload),
		InputTokens:              input.InputTokens,
		OutputTokens:             input.OutputTokens,
		EstimatedCost:            estimated,
	})
	if err != nil {
		return nil, fmt.Errorf("complete chat trace id=%d: %w", input.TraceID, err)
	}
	item, mapErr := mapChatTrace(row)
	if mapErr != nil {
		return nil, mapErr
	}
	return &item, nil
}

func (r *SQLCRepository) FailTrace(ctx context.Context, input FailTraceInput) (*ChatTrace, error) {
	estimated, err := numericFromFloat(input.EstimatedCost, 6)
	if err != nil {
		return nil, fmt.Errorf("convert estimated cost: %w", err)
	}
	row, err := r.q.FailChatTrace(ctx, dbgen.FailChatTraceParams{
		SellerAccountID:          input.SellerAccountID,
		ID:                       input.TraceID,
		DetectedIntent:           nullableText(string(input.DetectedIntent)),
		ToolPlanPayload:          toJSONBytes(input.ToolPlanPayload),
		ValidatedToolPlanPayload: toJSONBytes(input.ValidatedToolPlanPayload),
		ToolResultsPayload:       toJSONBytes(input.ToolResultsPayload),
		FactContextPayload:       toJSONBytes(input.FactContextPayload),
		RawPlannerResponse:       toJSONBytes(input.RawPlannerResponse),
		RawAnswerResponse:        toJSONBytes(input.RawAnswerResponse),
		AnswerValidationPayload:  toJSONBytes(input.AnswerValidationPayload),
		InputTokens:              input.InputTokens,
		OutputTokens:             input.OutputTokens,
		EstimatedCost:            estimated,
		ErrorMessage:             nullableText(input.ErrorMessage),
	})
	if err != nil {
		return nil, fmt.Errorf("fail chat trace id=%d: %w", input.TraceID, err)
	}
	item, mapErr := mapChatTrace(row)
	if mapErr != nil {
		return nil, mapErr
	}
	return &item, nil
}

func (r *SQLCRepository) GetTrace(ctx context.Context, sellerAccountID, traceID int64) (*ChatTrace, error) {
	row, err := r.q.GetChatTraceByID(ctx, dbgen.GetChatTraceByIDParams{
		SellerAccountID: sellerAccountID,
		ID:              traceID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get chat trace id=%d: %w", traceID, err)
	}
	item, mapErr := mapChatTrace(row)
	if mapErr != nil {
		return nil, mapErr
	}
	return &item, nil
}

func (r *SQLCRepository) GetLatestTraceBySession(ctx context.Context, sellerAccountID, sessionID int64) (*ChatTrace, error) {
	row, err := r.q.GetLatestChatTraceBySessionID(ctx, dbgen.GetLatestChatTraceBySessionIDParams{
		SellerAccountID: sellerAccountID,
		SessionID:       sessionID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get latest chat trace by session id=%d: %w", sessionID, err)
	}
	item, mapErr := mapChatTrace(row)
	if mapErr != nil {
		return nil, mapErr
	}
	return &item, nil
}

func (r *SQLCRepository) ListTracesBySession(ctx context.Context, sellerAccountID, sessionID int64, limit, offset int32) ([]ChatTrace, error) {
	rows, err := r.q.ListChatTracesBySessionID(ctx, dbgen.ListChatTracesBySessionIDParams{
		SellerAccountID: sellerAccountID,
		SessionID:       sessionID,
		Limit:           normalizeLimit(limit),
		Offset:          normalizeOffset(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("list chat traces by session: %w", err)
	}
	out := make([]ChatTrace, 0, len(rows))
	for _, row := range rows {
		item, mapErr := mapChatTrace(row)
		if mapErr != nil {
			return nil, mapErr
		}
		out = append(out, item)
	}
	return out, nil
}

func (r *SQLCRepository) CreateFeedback(ctx context.Context, input AddFeedbackInput) (*ChatFeedback, error) {
	row, err := r.q.CreateChatFeedback(ctx, dbgen.CreateChatFeedbackParams{
		SessionID:       input.SessionID,
		MessageID:       input.MessageID,
		SellerAccountID: input.SellerAccountID,
		Rating:          string(input.Rating),
		Comment:         nullableTextPtr(input.Comment),
	})
	if err != nil {
		return nil, fmt.Errorf("create chat feedback: %w", err)
	}
	item := mapChatFeedback(row)
	return &item, nil
}

func (r *SQLCRepository) GetFeedbackByMessage(ctx context.Context, sellerAccountID, messageID int64) (*ChatFeedback, error) {
	row, err := r.q.GetChatFeedbackByMessageID(ctx, dbgen.GetChatFeedbackByMessageIDParams{
		SellerAccountID: sellerAccountID,
		MessageID:       messageID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get chat feedback by message id=%d: %w", messageID, err)
	}
	item := mapChatFeedback(row)
	return &item, nil
}

func (r *SQLCRepository) ListFeedback(ctx context.Context, sellerAccountID int64, limit, offset int32) ([]ChatFeedback, error) {
	rows, err := r.q.ListChatFeedbackBySellerAccountID(ctx, dbgen.ListChatFeedbackBySellerAccountIDParams{
		SellerAccountID: sellerAccountID,
		Limit:           normalizeLimit(limit),
		Offset:          normalizeOffset(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("list chat feedback: %w", err)
	}
	out := make([]ChatFeedback, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapChatFeedback(row))
	}
	return out, nil
}

func mapChatSession(row dbgen.ChatSession) ChatSession {
	var actionKey *string
	if row.ActionKey.Valid && strings.TrimSpace(row.ActionKey.String) != "" {
		v := row.ActionKey.String
		actionKey = &v
	}
	return ChatSession{
		ID:              row.ID,
		SellerAccountID: row.SellerAccountID,
		UserID:          int8Ptr(row.UserID),
		Title:           row.Title,
		ActionKey:       actionKey,
		Status:          ChatSessionStatus(row.Status),
		CreatedAt:       timestamptz(row.CreatedAt),
		UpdatedAt:       timestamptz(row.UpdatedAt),
		LastMessageAt:   timestamptzPtr(row.LastMessageAt),
	}
}

func mapChatMessage(row dbgen.ChatMessage) ChatMessage {
	return ChatMessage{
		ID:              row.ID,
		SessionID:       row.SessionID,
		SellerAccountID: row.SellerAccountID,
		Role:            ChatMessageRole(row.Role),
		Content:         row.Content,
		MessageType:     ChatMessageType(row.MessageType),
		CreatedAt:       timestamptz(row.CreatedAt),
	}
}

func mapChatTrace(row dbgen.ChatTrace) (ChatTrace, error) {
	return ChatTrace{
		ID:                       row.ID,
		SessionID:                row.SessionID,
		UserMessageID:            int8Ptr(row.UserMessageID),
		AssistantMessageID:       int8Ptr(row.AssistantMessageID),
		SellerAccountID:          row.SellerAccountID,
		PlannerPromptVersion:     row.PlannerPromptVersion,
		AnswerPromptVersion:      row.AnswerPromptVersion,
		PlannerModel:             row.PlannerModel,
		AnswerModel:              row.AnswerModel,
		DetectedIntent:           chatIntent(row.DetectedIntent),
		ToolPlanPayload:          fromJSONBytes(row.ToolPlanPayload),
		ValidatedToolPlanPayload: fromJSONBytes(row.ValidatedToolPlanPayload),
		ToolResultsPayload:       fromJSONBytes(row.ToolResultsPayload),
		FactContextPayload:       fromJSONBytes(row.FactContextPayload),
		RawPlannerResponse:       fromJSONBytes(row.RawPlannerResponse),
		RawAnswerResponse:        fromJSONBytes(row.RawAnswerResponse),
		AnswerValidationPayload:  fromJSONBytes(row.AnswerValidationPayload),
		InputTokens:              row.InputTokens,
		OutputTokens:             row.OutputTokens,
		EstimatedCost:            numericFloat64(row.EstimatedCost),
		Status:                   ChatTraceStatus(row.Status),
		ErrorMessage:             textPtr(row.ErrorMessage),
		StartedAt:                timestamptz(row.StartedAt),
		FinishedAt:               timestamptzPtr(row.FinishedAt),
		CreatedAt:                timestamptz(row.CreatedAt),
	}, nil
}

func mapChatFeedback(row dbgen.ChatFeedback) ChatFeedback {
	return ChatFeedback{
		ID:              row.ID,
		SessionID:       row.SessionID,
		MessageID:       row.MessageID,
		SellerAccountID: row.SellerAccountID,
		Rating:          ChatFeedbackRating(row.Rating),
		Comment:         textPtr(row.Comment),
		CreatedAt:       timestamptz(row.CreatedAt),
	}
}

func toJSONBytes(payload map[string]any) []byte {
	if payload == nil {
		return []byte("{}")
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return []byte("{}")
	}
	return b
}

func fromJSONBytes(raw []byte) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{}
	}
	return out
}

func chatIntent(v pgtype.Text) ChatIntent {
	if !v.Valid || strings.TrimSpace(v.String) == "" {
		return ChatIntentUnknown
	}
	return ChatIntent(v.String)
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

func int8Ptr(v pgtype.Int8) *int64 {
	if !v.Valid {
		return nil
	}
	i := v.Int64
	return &i
}

func textPtr(v pgtype.Text) *string {
	if !v.Valid {
		return nil
	}
	s := v.String
	return &s
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

func numericFromFloat(v float64, scale int) (pgtype.Numeric, error) {
	var n pgtype.Numeric
	precision := strconv.Itoa(scale)
	if err := n.Scan(fmt.Sprintf("%."+precision+"f", v)); err != nil {
		return pgtype.Numeric{}, err
	}
	return n, nil
}

func normalizeLimit(limit int32) int32 {
	if limit <= 0 {
		return 20
	}
	if limit > 200 {
		return 200
	}
	return limit
}

func normalizeOffset(offset int32) int32 {
	if offset < 0 {
		return 0
	}
	return offset
}
