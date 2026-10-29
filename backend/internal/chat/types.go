package chat

import "time"

type ChatSessionStatus string

const (
	ChatSessionStatusActive   ChatSessionStatus = "active"
	ChatSessionStatusArchived ChatSessionStatus = "archived"
)

type ChatMessageRole string

const (
	ChatMessageRoleUser      ChatMessageRole = "user"
	ChatMessageRoleAssistant ChatMessageRole = "assistant"
	ChatMessageRoleSystem    ChatMessageRole = "system"
)

type ChatMessageType string

const (
	ChatMessageTypeQuestion ChatMessageType = "question"
	ChatMessageTypeAnswer   ChatMessageType = "answer"
	ChatMessageTypeError    ChatMessageType = "error"
	ChatMessageTypeMeta     ChatMessageType = "meta"
)

type ChatTraceStatus string

const (
	ChatTraceStatusRunning   ChatTraceStatus = "running"
	ChatTraceStatusCompleted ChatTraceStatus = "completed"
	ChatTraceStatusFailed    ChatTraceStatus = "failed"
)

type ChatFeedbackRating string

const (
	ChatFeedbackRatingPositive ChatFeedbackRating = "positive"
	ChatFeedbackRatingNegative ChatFeedbackRating = "negative"
	ChatFeedbackRatingNeutral  ChatFeedbackRating = "neutral"
)

type ChatIntent string

const (
	ChatIntentPriorities            ChatIntent = "priorities"
	ChatIntentExplainRecommendation ChatIntent = "explain_recommendation"
	ChatIntentUnsafeAds             ChatIntent = "unsafe_ads"
	ChatIntentAdLoss                ChatIntent = "ad_loss"
	ChatIntentSales                 ChatIntent = "sales"
	ChatIntentStock                 ChatIntent = "stock"
	ChatIntentAdvertising           ChatIntent = "advertising"
	ChatIntentPricing               ChatIntent = "pricing"
	ChatIntentAlerts                ChatIntent = "alerts"
	ChatIntentRecommendations       ChatIntent = "recommendations"
	ChatIntentABCAnalysis           ChatIntent = "abc_analysis"
	ChatIntentGeneralOverview       ChatIntent = "general_overview"
	ChatIntentUnknown               ChatIntent = "unknown"
	ChatIntentUnsupported           ChatIntent = "unsupported"
)

type ConfidenceLevel string

const (
	ConfidenceLevelLow    ConfidenceLevel = "low"
	ConfidenceLevelMedium ConfidenceLevel = "medium"
	ConfidenceLevelHigh   ConfidenceLevel = "high"
)

type ChatSession struct {
	ID              int64
	SellerAccountID int64
	UserID          *int64
	Title           string
	ActionKey       *string
	Status          ChatSessionStatus
	CreatedAt       time.Time
	UpdatedAt       time.Time
	LastMessageAt   *time.Time
}

type ChatMessage struct {
	ID              int64
	SessionID       int64
	SellerAccountID int64
	Role            ChatMessageRole
	Content         string
	MessageType     ChatMessageType
	CreatedAt       time.Time
}

type ChatTrace struct {
	ID                       int64
	SessionID                int64
	UserMessageID            *int64
	AssistantMessageID       *int64
	SellerAccountID          int64
	PlannerPromptVersion     string
	AnswerPromptVersion      string
	PlannerModel             string
	AnswerModel              string
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
	Status                   ChatTraceStatus
	ErrorMessage             *string
	StartedAt                time.Time
	FinishedAt               *time.Time
	CreatedAt                time.Time
}

type ChatFeedback struct {
	ID              int64
	SessionID       int64
	MessageID       int64
	SellerAccountID int64
	Rating          ChatFeedbackRating
	Comment         *string
	CreatedAt       time.Time
}

type ChatQuestion struct {
	SessionID       *int64
	SellerAccountID int64
	UserID          *int64
	Question        string
	AsOfDate        *time.Time
}

type ChatAnswer struct {
	Answer                   string                 `json:"answer"`
	Summary                  string                 `json:"summary"`
	Intent                   ChatIntent             `json:"intent"`
	ConfidenceLevel          ConfidenceLevel        `json:"confidence_level"`
	RelatedAlertIDs          []int64                `json:"related_alert_ids"`
	RelatedRecommendationIDs []int64                `json:"related_recommendation_ids"`
	SupportingFacts          []SupportingFact       `json:"supporting_facts"`
	Limitations              []string               `json:"limitations"`
	Attachments              []ChatAnswerAttachment `json:"attachments,omitempty"`
}

type ChatAnswerAttachment struct {
	ID          string `json:"id"`
	FileName    string `json:"file_name"`
	MimeType    string `json:"mime_type"`
	FileSize    int64  `json:"file_size"`
	ExportType  string `json:"export_type"`
	Format      string `json:"format"`
	DownloadURL string `json:"download_url"`
	CreatedAt   string `json:"created_at"`
}

type SupportingFact struct {
	Source string `json:"source"`
	ID     *int64 `json:"id,omitempty"`
	Fact   string `json:"fact"`
}

type DetectedIntent struct {
	Intent      ChatIntent
	Confidence  float64
	Language    string
	Topics      []string
	EntityHints EntityHints
}

type EntityHints struct {
	SKU              *int64
	OfferID          *string
	ProductID        *int64
	CampaignID       *int64
	RecommendationID *int64
	AlertID          *int64
	CategoryHint     *string
}

type ToolPlan struct {
	Intent            ChatIntent `json:"intent"`
	Confidence        float64    `json:"confidence"`
	Language          string     `json:"language"`
	ToolCalls         []ToolCall `json:"tool_calls"`
	Assumptions       []string   `json:"assumptions"`
	UnsupportedReason *string    `json:"unsupported_reason,omitempty"`
}

type ToolCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

type ValidatedToolPlan struct {
	Intent            ChatIntent
	ToolCalls         []ToolCall
	Assumptions       []string
	Warnings          []string
	UnsupportedReason *string
}

type ToolResult struct {
	Name        string
	Args        map[string]any
	Data        any
	Error       *string
	Limitations []string
}

type FactContext struct {
	ContextVersion string    `json:"context_version"`
	GeneratedAt    time.Time `json:"generated_at"`

	Question string               `json:"question"`
	Intent   ChatIntent           `json:"intent"`
	Language string               `json:"language,omitempty"`
	AsOfDate *time.Time           `json:"as_of_date,omitempty"`
	Action   *ActionPromptContext `json:"action_context,omitempty"`

	Seller SellerContext `json:"seller"`

	ToolPlan    ValidatedToolPlan `json:"tool_plan"`
	ToolResults []ToolResult      `json:"tool_results"`
	Facts       FactContextFacts  `json:"facts"`

	RelatedAlerts          []FactAlertReference          `json:"related_alerts"`
	RelatedRecommendations []FactRecommendationReference `json:"related_recommendations"`

	Assumptions []string       `json:"assumptions"`
	Limitations []string       `json:"limitations"`
	Freshness   map[string]any `json:"freshness"`

	// ContextTruncated mirrors context_stats.truncated for trace consumers (no extra payload).
	ContextTruncated        bool   `json:"context_truncated,omitempty"`
	ContextTruncationReason string `json:"context_truncation_reason,omitempty"`

	ContextStats FactContextStats `json:"context_stats"`
}

type SellerContext struct {
	SellerAccountID int64   `json:"seller_account_id"`
	AccountName     *string `json:"account_name,omitempty"`
	SecretsIncluded bool    `json:"secrets_included"`
}

type FactContextFacts struct {
	Dashboard             map[string]any   `json:"dashboard,omitempty"`
	Recommendations       []map[string]any `json:"recommendations,omitempty"`
	RecommendationDetails []map[string]any `json:"recommendation_details,omitempty"`
	Alerts                []map[string]any `json:"alerts,omitempty"`
	PriceEconomicsRisks   []map[string]any `json:"price_economics_risks,omitempty"`
	CriticalSKUs          []map[string]any `json:"critical_skus,omitempty"`
	StockRisks            []map[string]any `json:"stock_risks,omitempty"`
	Advertising           map[string]any   `json:"advertising,omitempty"`
	SKUMetrics            []map[string]any `json:"sku_metrics,omitempty"`
	SKUContexts           []map[string]any `json:"sku_contexts,omitempty"`
	CampaignContexts      []map[string]any `json:"campaign_contexts,omitempty"`
	ABCAnalysis           map[string]any   `json:"abc_analysis,omitempty"`
	Other                 []map[string]any `json:"other,omitempty"`
}

type FactAlertReference struct {
	ID            int64   `json:"id"`
	AlertType     string  `json:"alert_type,omitempty"`
	Group         string  `json:"group,omitempty"`
	Severity      string  `json:"severity,omitempty"`
	Urgency       string  `json:"urgency,omitempty"`
	EntityType    string  `json:"entity_type,omitempty"`
	EntitySKU     *int64  `json:"entity_sku,omitempty"`
	EntityOfferID *string `json:"entity_offer_id,omitempty"`
	Title         string  `json:"title,omitempty"`
}

type FactRecommendationReference struct {
	ID                 int64   `json:"id"`
	RecommendationType string  `json:"recommendation_type,omitempty"`
	PriorityLevel      string  `json:"priority_level,omitempty"`
	Urgency            string  `json:"urgency,omitempty"`
	ConfidenceLevel    string  `json:"confidence_level,omitempty"`
	EntityType         string  `json:"entity_type,omitempty"`
	EntitySKU          *int64  `json:"entity_sku,omitempty"`
	EntityOfferID      *string `json:"entity_offer_id,omitempty"`
	Title              string  `json:"title,omitempty"`
	RecommendedAction  string  `json:"recommended_action,omitempty"`
}

type FactContextStats struct {
	ToolResultsCount      int    `json:"tool_results_count"`
	FailedToolsCount      int    `json:"failed_tools_count"`
	TotalItemsIncluded    int    `json:"total_items_included"`
	EstimatedContextBytes int    `json:"estimated_context_bytes"`
	Truncated             bool   `json:"truncated"`
	TruncationReason      string `json:"truncation_reason,omitempty"`
}

type AssembleContextInput struct {
	Question        string
	SellerAccountID int64
	SellerName      *string
	Plan            ValidatedToolPlan
	ToolResults     []ToolResult
	AsOfDate        *time.Time
	Language        string
}

type AnswerValidationResult struct {
	IsValid              bool
	Errors               []string
	Warnings             []string
	FinalConfidenceLevel ConfidenceLevel
}

type AskInput struct {
	SellerAccountID    int64
	UserID             *int64
	SessionID          *int64
	Question           string
	ActionKey          string
	ChatCreationSource string
	AsOfDate           *time.Time
	AttachmentIDs      []string
	ClientRequestID    string
}

type AskResult struct {
	SessionID          int64       `json:"session_id"`
	SessionCreated     bool        `json:"session_created"`
	SessionReused      bool        `json:"session_reused"`
	UserMessageID      int64       `json:"user_message_id"`
	AssistantMessageID *int64      `json:"assistant_message_id"`
	TraceID            int64       `json:"trace_id"`
	Answer             *ChatAnswer `json:"answer,omitempty"`
	ErrorMessage       *string     `json:"error_message,omitempty"`
}

// AskStreamEvent is one server-sent event emitted while processing an ask.
type AskStreamEvent struct {
	Type      string     `json:"type"`             // status|delta|done|error
	Status    string     `json:"status,omitempty"` // queued|running|streaming|completed|failed|cancelled
	Delta     string     `json:"delta,omitempty"`
	Result    *AskResult `json:"result,omitempty"`
	Error     string     `json:"error,omitempty"`
	RequestID string     `json:"request_id,omitempty"`
}

type ActionPromptContext struct {
	ActionKey          string   `json:"action_key,omitempty"`
	Title              string   `json:"title,omitempty"`
	Type               string   `json:"type,omitempty"`
	Category           string   `json:"category,omitempty"`
	Priority           string   `json:"priority,omitempty"`
	Status             string   `json:"status,omitempty"`
	MetricDate         string   `json:"metric_date,omitempty"`
	Summary            string   `json:"summary,omitempty"`
	Reason             string   `json:"reason,omitempty"`
	NextStep           string   `json:"next_step,omitempty"`
	ExpectedEffect     string   `json:"expected_effect,omitempty"`
	Evidence           []string `json:"evidence,omitempty"`
	EvidenceFactIDs    []string `json:"evidence_fact_ids,omitempty"`
	ObjectActions      []string `json:"object_actions,omitempty"`
	CompletionCriteria string   `json:"completion_criteria,omitempty"`
	Limitations        []string `json:"limitations,omitempty"`
	PipelineRunID      int64    `json:"pipeline_run_id,omitempty"`
	ActionsRunID       int64    `json:"actions_run_id,omitempty"`
	DataPackHash       string   `json:"data_pack_hash,omitempty"`
	GlobalContextHash  string   `json:"global_context_hash,omitempty"`
}
