package admin

import (
	"context"
	"errors"
	"strings"
	"time"
)

var (
	ErrRepositoryRequired       = errors.New("admin repository is required")
	ErrAdminActorRequired       = errors.New("admin actor is required")
	ErrSellerAccountIDRequired  = errors.New("seller account id is required")
	ErrAdminActionNotConfigured = errors.New("admin action dependency is not configured")
	ErrAdminDataUnavailable     = errors.New("admin data is unavailable")
	ErrAdminAuditLogWriteFailed = errors.New("admin audit log write failed; raw AI payload withheld")
)

type AdminActionType string

const (
	AdminActionRerunSync               AdminActionType = "rerun_sync"
	AdminActionResetCursor             AdminActionType = "reset_cursor"
	AdminActionRerunMetrics            AdminActionType = "rerun_metrics"
	AdminActionRerunAlerts             AdminActionType = "rerun_alerts"
	AdminActionRerunRecommendations    AdminActionType = "rerun_recommendations"
	AdminActionUpdateBillingState      AdminActionType = "update_billing_state"
	AdminActionViewRawAIPayload        AdminActionType = "view_raw_ai_payload"
	AdminActionRetryAIPipelineDispatch AdminActionType = "retry_ai_pipeline_dispatch"
	AdminActionSeedCreated             AdminActionType = "seed_created"
)

// Target types for AdminActionViewRawAIPayload (admin_action_logs.target_type).
const (
	AdminRawViewTargetRecommendation    = "recommendation"
	AdminRawViewTargetChatTrace         = "chat_trace"
	AdminRawViewTargetRecommendationRun = "recommendation_run"
)

type AdminActionStatus string

const (
	AdminActionStatusRunning   AdminActionStatus = "running"
	AdminActionStatusCompleted AdminActionStatus = "completed"
	AdminActionStatusFailed    AdminActionStatus = "failed"
)

type BillingStatus string

const (
	BillingStatusTrial     BillingStatus = "trial"
	BillingStatusActive    BillingStatus = "active"
	BillingStatusPastDue   BillingStatus = "past_due"
	BillingStatusPaused    BillingStatus = "paused"
	BillingStatusCancelled BillingStatus = "cancelled"
	BillingStatusInternal  BillingStatus = "internal"
)

type Page struct {
	Limit  int32
	Offset int32
}

func NormalizePage(limit, offset, defaultLimit, maxLimit int32) Page {
	if defaultLimit <= 0 {
		defaultLimit = 50
	}
	if maxLimit <= 0 {
		maxLimit = 200
	}
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	if offset < 0 {
		offset = 0
	}
	return Page{Limit: limit, Offset: offset}
}

type ClientListFilter struct {
	Search           string
	SellerStatus     string
	ConnectionStatus string
	BillingStatus    string
	Limit            int32
	Offset           int32
}

type ClientListItem struct {
	SellerAccountID int64
	SellerName      string
	UserEmail       string
	SellerStatus    string

	ConnectionStatus      *string
	LastConnectionCheckAt *time.Time
	LastConnectionError   *string

	LatestSyncStatus     *string
	LatestSyncStartedAt  *time.Time
	LatestSyncFinishedAt *time.Time

	OpenAlertsCount          int64
	OpenRecommendationsCount int64

	LatestRecommendationRunStatus *string
	LatestChatTraceStatus         *string
	BillingStatus                 *string

	CreatedAt time.Time
	UpdatedAt time.Time
}

type ClientListResult struct {
	Items  []ClientListItem
	Limit  int32
	Offset int32
}

type ClientConnection struct {
	Provider                    string
	ConnectionStatus            string
	LastCheckAt                 *time.Time
	LastCheckResult             *string
	LastConnectionErr           *string
	UpdatedAt                   *time.Time
	PerformanceConnectionStatus string
	PerformanceTokenSet         bool
	PerformanceLastCheckAt      *time.Time
	PerformanceLastCheckResult  *string
	PerformanceLastError        *string
}

type ClientDetail struct {
	Overview          ClientOverview
	Connections       []ClientConnection
	OperationalStatus OperationalStatus
	Billing           *BillingState
}

type ClientOverview struct {
	SellerAccountID int64
	SellerName      string
	SellerStatus    string
	OwnerUserID     *int64
	OwnerEmail      *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type OperationalStatus struct {
	LatestSyncJob            *SyncJobSummary
	LatestImportJobs         []ImportJobSummary
	LatestAlertRun           *AlertRunSummary
	LatestRecommendationRun  *RecommendationRunSummary
	LatestChatTrace          *ChatTraceSummary
	OpenAlertsCount          int64
	OpenRecommendationsCount int64
	Limitations              []string
}

type SyncJobSummary struct {
	ID           int64
	Type         string
	Status       string
	StartedAt    *time.Time
	FinishedAt   *time.Time
	ErrorMessage *string
	CreatedAt    time.Time
}

type ImportJobSummary struct {
	ID              int64
	SyncJobID       int64
	Domain          string
	Status          string
	SourceCursor    *string
	RecordsReceived int32
	RecordsImported int32
	RecordsFailed   int32
	StartedAt       *time.Time
	FinishedAt      *time.Time
	ErrorMessage    *string
	CreatedAt       time.Time
}

type ImportErrorItem struct {
	ImportJobID   int64
	SyncJobID     int64
	Domain        string
	Status        string
	ErrorMessage  string
	RecordsFailed int32
	StartedAt     *time.Time
	FinishedAt    *time.Time
	CreatedAt     time.Time
}

type SyncCursorItem struct {
	ID          int64
	Domain      string
	CursorType  string
	CursorValue *string
	UpdatedAt   time.Time
}

type SyncJobFilter struct {
	Status string
	Limit  int32
	Offset int32
}

type ImportJobFilter struct {
	Status string
	Domain string
	Limit  int32
	Offset int32
}

type ImportErrorFilter struct {
	Status string
	Domain string
	Limit  int32
	Offset int32
}

type SyncCursorFilter struct {
	Domain string
	Limit  int32
	Offset int32
}

type SyncJobListResult struct {
	Items  []SyncJobSummary
	Limit  int32
	Offset int32
}

type ImportJobListResult struct {
	Items  []ImportJobSummary
	Limit  int32
	Offset int32
}

type ImportErrorListResult struct {
	Items  []ImportErrorItem
	Limit  int32
	Offset int32
}

type SyncCursorListResult struct {
	Items  []SyncCursorItem
	Limit  int32
	Offset int32
}

type AlertRunSummary struct {
	ID               int64
	RunType          string
	Status           string
	StartedAt        *time.Time
	FinishedAt       *time.Time
	TotalAlertsCount int32
	ErrorMessage     *string
}

type RecommendationRunSummary struct {
	ID                            int64
	RunType                       string
	Status                        string
	AsOfDate                      *time.Time
	AIModel                       *string
	AIPromptVersion               *string
	StartedAt                     *time.Time
	FinishedAt                    *time.Time
	InputTokens                   int32
	OutputTokens                  int32
	EstimatedCost                 float64
	GeneratedRecommendationsCount int32
	AcceptedRecommendationsCount  int32
	RejectedRecommendationsCount  *int32
	ErrorMessage                  *string
}

type RecommendationRunLogItem = RecommendationRunSummary

type RecommendationRunLogFilter struct {
	Status  string
	RunType string
	Limit   int32
	Offset  int32
}

type RecommendationRunLogListResult struct {
	Items  []RecommendationRunLogItem
	Limit  int32
	Offset int32
}

type RecommendationRunDetail struct {
	Run             RecommendationRunSummary
	Recommendations []RecommendationItem
	Diagnostics     []RecommendationRunDiagnosticItem
	Limitations     []string
}

type RecommendationRunDiagnosticItem struct {
	ID                      int64
	RecommendationRunID     *int64
	OpenAIRequestID         *string
	AIModel                 *string
	PromptVersion           *string
	ContextPayloadSummary   map[string]any
	RawOpenAIResponse       map[string]any
	ValidationResultPayload map[string]any
	RejectedItemsPayload    map[string]any
	ErrorStage              *string
	ErrorMessage            *string
	InputTokens             int64
	OutputTokens            int64
	EstimatedCost           float64
	CreatedAt               time.Time
}

type RecommendationRawAI struct {
	Recommendation RecommendationItem
	RelatedAlerts  []RecommendationAlertItem
	Diagnostics    []RecommendationRunDiagnosticItem
	Limitations    []string
}

type RecommendationItem struct {
	ID                       int64
	RecommendationType       string
	Title                    string
	Status                   string
	PriorityLevel            string
	ConfidenceLevel          string
	Horizon                  string
	EntityType               string
	EntityID                 *string
	EntitySKU                *int64
	EntityOfferID            *string
	WhatHappened             *string
	WhyItMatters             *string
	RecommendedAction        *string
	ExpectedEffect           *string
	SupportingMetricsPayload map[string]any
	ConstraintsPayload       map[string]any
	RawAIResponse            map[string]any
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

type RecommendationAlertItem struct {
	ID         int64
	AlertType  string
	AlertGroup string
	Severity   string
	Urgency    string
	Title      string
	Status     string
}

type ChatTraceSummary struct {
	ID                   int64
	SessionID            int64
	UserMessageID        *int64
	AssistantMessageID   *int64
	Status               string
	DetectedIntent       *string
	PlannerModel         string
	AnswerModel          string
	PlannerPromptVersion string
	AnswerPromptVersion  string
	InputTokens          int32
	OutputTokens         int32
	EstimatedCost        float64
	StartedAt            *time.Time
	FinishedAt           *time.Time
	ErrorMessage         *string
}

type ChatTraceLogItem = ChatTraceSummary

type ChatTraceFilter struct {
	Status    string
	Intent    string
	SessionID *int64
	Limit     int32
	Offset    int32
}

type ChatTraceListResult struct {
	Items  []ChatTraceLogItem
	Limit  int32
	Offset int32
}

type ChatTraceDetail struct {
	ID                       int64
	SessionID                int64
	UserMessageID            *int64
	AssistantMessageID       *int64
	SellerAccountID          int64
	Status                   string
	PlannerPromptVersion     string
	AnswerPromptVersion      string
	PlannerModel             string
	AnswerModel              string
	DetectedIntent           *string
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
	ErrorMessage             *string
	StartedAt                *time.Time
	FinishedAt               *time.Time
	CreatedAt                time.Time
	Messages                 []ChatMessageItem
	Limitations              []string
}

type ChatSessionFilter struct {
	Status string
	Limit  int32
	Offset int32
}

type ChatSessionItem struct {
	ID            int64
	Title         string
	Status        string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	LastMessageAt *time.Time
}

type ChatSessionListResult struct {
	Items  []ChatSessionItem
	Limit  int32
	Offset int32
}

type ChatMessageFilter struct {
	Limit  int32
	Offset int32
}

type ChatMessageItem struct {
	ID          int64
	SessionID   int64
	Role        string
	MessageType string
	Content     string
	CreatedAt   time.Time
}

type ChatMessageListResult struct {
	Items  []ChatMessageItem
	Limit  int32
	Offset int32
}

type ChatFeedbackItem struct {
	ID              int64
	SellerAccountID int64
	SellerName      *string
	SessionID       int64
	MessageID       int64
	Rating          string
	Comment         *string
	MessageRole     *string
	MessageType     *string
	MessageContent  *string
	SessionTitle    *string
	TraceID         *int64
	CreatedAt       time.Time
}

type ChatFeedbackFilter struct {
	SellerAccountID *int64
	Rating          string
	Limit           int32
	Offset          int32
}

type ChatFeedbackListResult struct {
	Items  []ChatFeedbackItem
	Limit  int32
	Offset int32
}

type RecommendationFeedbackProxyStatus struct {
	AcceptedCount  int64
	DismissedCount int64
	ResolvedCount  int64
}

type RecommendationFeedbackFilter struct {
	Rating string
	Status string
	Limit  int32
	Offset int32
}

type RecommendationFeedbackItem struct {
	ID                      int64
	SellerAccountID         int64
	RecommendationID        int64
	Rating                  string
	Comment                 *string
	CreatedAt               time.Time
	RecommendationType      string
	Title                   string
	PriorityLevel           string
	ConfidenceLevel         string
	RecommendationStatus    string
	EntityType              string
	EntityID                *string
	EntitySKU               *int64
	EntityOfferID           *string
	RecommendationCreatedAt time.Time
}

type RecommendationFeedbackListResult struct {
	Items               []RecommendationFeedbackItem
	ProxyStatusFeedback RecommendationFeedbackProxyStatus
	Limitations         []string
	Limit               int32
	Offset              int32
}

type FeedbackSummary struct {
	ChatPositive            int64
	ChatNegative            int64
	ChatNeutral             int64
	RecommendationAccepted  int64
	RecommendationDismissed int64
	RecommendationResolved  int64
}

type BillingState struct {
	SellerAccountID      int64
	PlanCode             string
	Status               BillingStatus
	TrialEndsAt          *time.Time
	CurrentPeriodStart   *time.Time
	CurrentPeriodEnd     *time.Time
	AITokensLimitMonth   *int64
	AITokensUsedMonth    int64
	EstimatedAICostMonth float64
	Notes                *string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type BillingStateFilter struct {
	Status string
	Limit  int32
	Offset int32
}

type AdminActionLog struct {
	ID              int64
	AdminUserID     *int64
	AdminEmail      string
	SellerAccountID int64
	ActionType      AdminActionType
	TargetType      *string
	TargetID        *int64
	RequestPayload  map[string]any
	ResultPayload   map[string]any
	Status          AdminActionStatus
	ErrorMessage    *string
	CreatedAt       time.Time
	FinishedAt      *time.Time
}

type AdminActor struct {
	UserID *int64
	Email  string
}

type RecommendationViewAuditMeta struct {
	ID              int64
	AIModel         string
	AIPromptVersion string
}

type ChatTraceViewAuditMeta struct {
	ID                   int64
	SessionID            int64
	UserMessageID        *int64
	AssistantMessageID   *int64
	PlannerModel         string
	AnswerModel          string
	PlannerPromptVersion string
	AnswerPromptVersion  string
	Status               string
}

type RecommendationRunViewAuditMeta struct {
	ID              int64
	RunType         string
	Status          string
	AIModel         string
	AIPromptVersion string
}

type CreateAdminActionLogInput struct {
	AdminUserID     *int64
	AdminEmail      string
	SellerAccountID int64
	ActionType      AdminActionType
	TargetType      *string
	TargetID        *int64
	RequestPayload  map[string]any
	Status          AdminActionStatus
}

type CompleteAdminActionLogInput struct {
	ID            int64
	ResultPayload map[string]any
}

type FailAdminActionLogInput struct {
	ID            int64
	ResultPayload map[string]any
	ErrorMessage  string
}

type UpsertBillingStateInput struct {
	SellerAccountID      int64
	PlanCode             string
	Status               BillingStatus
	TrialEndsAt          *time.Time
	CurrentPeriodStart   *time.Time
	CurrentPeriodEnd     *time.Time
	AITokensLimitMonth   *int64
	AITokensUsedMonth    int64
	EstimatedAICostMonth float64
	Notes                *string
}

type RerunSyncInput struct {
	SellerAccountID int64
	SyncType        string
}

type RetryAIPipelineDispatchInput struct {
	DispatchID      int64
	SellerAccountID int64
	SyncJobID       int64
}

type ResetCursorInput struct {
	SellerAccountID int64
	Domain          string
	CursorType      string
	CursorValue     *string
}

type RerunMetricsInput struct {
	SellerAccountID int64
	DateFrom        time.Time
	DateTo          time.Time
}

type RerunAlertsInput struct {
	SellerAccountID int64
	AsOfDate        time.Time
}

type RerunRecommendationsInput struct {
	SellerAccountID int64
	AsOfDate        time.Time
}

type UpdateBillingStateInput = UpsertBillingStateInput

type IngestionRerunner interface {
	StartInitialSync(ctx context.Context, sellerAccountID int64) (syncJobID int64, syncStatus string, err error)
}

type MetricsRerunner interface {
	Rerun(ctx context.Context, sellerAccountID int64, dateFrom, dateTo time.Time) (map[string]any, error)
}

type AlertsRerunner interface {
	Rerun(ctx context.Context, sellerAccountID int64, asOfDate time.Time) (map[string]any, error)
}

type RecommendationsRerunner interface {
	Rerun(ctx context.Context, sellerAccountID int64, asOfDate time.Time) (map[string]any, error)
}

type CursorResetter interface {
	ResetCursor(ctx context.Context, sellerAccountID int64, domain, cursorType string, cursorValue *string) error
}

type Repository interface {
	ListClients(ctx context.Context, filter ClientListFilter) (*ClientListResult, error)
	GetClientOverview(ctx context.Context, sellerAccountID int64) (*ClientOverview, error)
	GetClientConnections(ctx context.Context, sellerAccountID int64) ([]ClientConnection, error)

	ListSyncJobs(ctx context.Context, sellerAccountID int64, filter SyncJobFilter, page Page) ([]SyncJobSummary, error)
	ListImportJobs(ctx context.Context, sellerAccountID int64, filter ImportJobFilter, page Page) ([]ImportJobSummary, error)
	ListImportErrors(ctx context.Context, sellerAccountID int64, filter ImportErrorFilter, page Page) ([]ImportErrorItem, error)
	ListSyncCursors(ctx context.Context, sellerAccountID int64, filter SyncCursorFilter, page Page) ([]SyncCursorItem, error)

	ListAlertRuns(ctx context.Context, sellerAccountID int64, page Page) ([]AlertRunSummary, error)
	ListRecommendationRuns(ctx context.Context, sellerAccountID int64, filter RecommendationRunLogFilter, page Page) ([]RecommendationRunLogItem, error)

	ListChatTraces(ctx context.Context, sellerAccountID int64, filter ChatTraceFilter, page Page) ([]ChatTraceLogItem, error)
	ListChatSessions(ctx context.Context, sellerAccountID int64, filter ChatSessionFilter, page Page) ([]ChatSessionItem, error)
	GetChatSession(ctx context.Context, sellerAccountID, sessionID int64) (*ChatSessionItem, error)
	ListChatMessages(ctx context.Context, sellerAccountID, sessionID int64, filter ChatMessageFilter, page Page) ([]ChatMessageItem, error)
	ListChatFeedback(ctx context.Context, filter ChatFeedbackFilter, page Page) ([]ChatFeedbackItem, error)
	ListRecommendationFeedback(ctx context.Context, sellerAccountID int64, filter RecommendationFeedbackFilter, page Page) ([]RecommendationFeedbackItem, error)
	GetRecommendationProxyFeedbackCounts(ctx context.Context, sellerAccountID int64) (RecommendationFeedbackProxyStatus, error)

	GetChatTraceDetail(ctx context.Context, sellerAccountID, traceID int64) (*ChatTraceDetail, error)
	GetRecommendationRunDetail(ctx context.Context, sellerAccountID, runID int64) (*RecommendationRunDetail, error)
	GetRecommendationRawAI(ctx context.Context, sellerAccountID, recommendationID int64) (*RecommendationRawAI, error)

	PeekRecommendationForAudit(ctx context.Context, sellerAccountID, recommendationID int64) (RecommendationViewAuditMeta, error)
	PeekChatTraceForAudit(ctx context.Context, sellerAccountID, traceID int64) (ChatTraceViewAuditMeta, error)
	PeekRecommendationRunForAudit(ctx context.Context, sellerAccountID, runID int64) (RecommendationRunViewAuditMeta, error)

	GetBillingState(ctx context.Context, sellerAccountID int64) (*BillingState, error)
	ListBillingStates(ctx context.Context, filter BillingStateFilter) ([]BillingState, error)

	CreateAdminActionLog(ctx context.Context, input CreateAdminActionLogInput) (*AdminActionLog, error)
	CompleteAdminActionLog(ctx context.Context, input CompleteAdminActionLogInput) (*AdminActionLog, error)
	FailAdminActionLog(ctx context.Context, input FailAdminActionLogInput) (*AdminActionLog, error)
	UpsertBillingState(ctx context.Context, input UpsertBillingStateInput) (*BillingState, error)
}

func validateSellerAccountID(id int64) error {
	if id <= 0 {
		return ErrSellerAccountIDRequired
	}
	return nil
}

func validateAdminActor(actor AdminActor) error {
	if strings.TrimSpace(actor.Email) == "" {
		return ErrAdminActorRequired
	}
	return nil
}
