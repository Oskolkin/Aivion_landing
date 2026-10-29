package chatv2

import (
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

type TurnStatus string

const (
	StatusQueued           TurnStatus = "QUEUED"
	StatusResolvingContext TurnStatus = "RESOLVING_CONTEXT"
	StatusReadingData      TurnStatus = "READING_DATA"
	StatusAnalyzing        TurnStatus = "ANALYZING"
	StatusValidating       TurnStatus = "VALIDATING"
	StatusStreaming        TurnStatus = "STREAMING"
	StatusCompleted        TurnStatus = "COMPLETED"
	StatusFailed           TurnStatus = "FAILED"
	StatusCancelled        TurnStatus = "CANCELLED"
)

type ContextMode string

const (
	ContextPublicationBound      ContextMode = "PUBLICATION_BOUND"
	ContextPrepublicationFactual ContextMode = "PREPUBLICATION_FACTUAL"
	ContextGeneral               ContextMode = "GENERAL"
)

type AnswerMode string

const (
	ModePublishedExplanation AnswerMode = "PUBLISHED_EXPLANATION"
	ModeFactualQuery         AnswerMode = "FACTUAL_QUERY"
	ModeAdHocAnalysis        AnswerMode = "AD_HOC_ANALYSIS"
	ModeActionAssistance     AnswerMode = "ACTION_ASSISTANCE"
	ModeAttachmentAnalysis   AnswerMode = "ATTACHMENT_ANALYSIS"
	ModeMixedAnalysis        AnswerMode = "MIXED_ANALYSIS"
	ModeExport               AnswerMode = "EXPORT"
	ModeGeneralGuidance      AnswerMode = "GENERAL_GUIDANCE"
)

type GroundingStatus string

const (
	GroundingNone     GroundingStatus = "NONE"
	GroundingGrounded GroundingStatus = "GROUNDED"
	GroundingLimited  GroundingStatus = "LIMITED"
)

type RefType string

const (
	RefFact           RefType = "FACT"
	RefFinding        RefType = "FINDING"
	RefRecommendation RefType = "RECOMMENDATION"
	RefAction         RefType = "ACTION"
	RefActionCase     RefType = "ACTION_CASE"
	RefAttachment     RefType = "ATTACHMENT"
	RefPublication    RefType = "PUBLICATION"
)

type Completeness string

const (
	CompleteComplete       Completeness = "COMPLETE"
	CompletePartial        Completeness = "PARTIAL"
	CompleteUnreadable     Completeness = "UNREADABLE"
	CompleteVisionRequired Completeness = "VISION_REQUIRED"
)

type PublicationRef struct {
	ID           int64
	UniverseID   string
	UniverseHash string
	AnalysisDate time.Time
	CutoffAt     time.Time
	Generation   int64
}

type HistoryMessage struct {
	ID      int64
	Role    string
	Content string
	Created time.Time
}

type AttachmentContext struct {
	ID           string       `json:"id"`
	FileName     string       `json:"file_name"`
	MimeType     string       `json:"mime_type"`
	Completeness Completeness `json:"completeness"`
	Excerpt      string       `json:"excerpt"`
	Warning      string       `json:"warning,omitempty"`
	Handle       string       `json:"handle,omitempty"`
}

type ActionMemberSnapshot struct {
	CaseID                  string         `json:"case_id"`
	Active                  bool           `json:"active"`
	UserState               string         `json:"user_state,omitempty"`
	SystemState             string         `json:"system_state,omitempty"`
	TargetKind              string         `json:"target_kind,omitempty"`
	TargetID                string         `json:"target_id,omitempty"`
	ActionType              string         `json:"action_type,omitempty"`
	Parameters              map[string]any `json:"parameters,omitempty"`
	RecommendationRef       string         `json:"recommendation_ref,omitempty"`
	OriginParameters        map[string]any `json:"origin_parameters,omitempty"`
	OriginRecommendationRef string         `json:"origin_recommendation_ref,omitempty"`
	Limitations             []string       `json:"limitations,omitempty"`
}

type ActionTurnContext struct {
	BindingID           int64
	Title               string
	OriginKind          string
	OriginPublicationID int64
	OriginGroupID       string
	OriginCaseID        string
	OriginMemberCount   int
	CurrentActiveCount  int
	Members             []ActionMemberSnapshot
}

type ContextHashes struct {
	Base         string
	Conversation string
	Attachment   string
	Action       string
}

type FrozenTurnContext struct {
	SellerID              int64
	UserID                *int64
	SessionID             int64
	Question              string
	ContextMode           ContextMode
	Publication           *PublicationRef
	PreviousPublicationID *int64
	Factual               factualapi.FactualReadContext
	HasFactual            bool
	FactualIdentity       string
	BaseContext           map[string]any
	Conversation          []HistoryMessage
	ReferencedEntities    []EntityRef
	Action                *ActionTurnContext
	Attachments           []AttachmentContext
	Hashes                ContextHashes
	Truncated             bool
	TruncationReason      string
	Limitations           []string
}

type TurnResult struct {
	Answer             string            `json:"answer"`
	AnswerMode         AnswerMode        `json:"answer_mode"`
	ReferencedEntities []EntityRef       `json:"referenced_entities"`
	GroundingHandles   []string          `json:"grounding_handles"`
	Limitations        []string          `json:"limitations"`
	ExportDataset      *ExportDatasetRef `json:"export_dataset,omitempty"`
}

type EntityRef struct {
	Kind  string `json:"kind"`
	ID    string `json:"id"`
	Label string `json:"label,omitempty"`
}

type ExportDatasetRef struct {
	Kind    string               `json:"kind"`
	Period  string               `json:"period,omitempty"`
	Filters []ExportFilterClause `json:"filters,omitempty"`
}

type ExportFilterClause struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

type GroundingEntry struct {
	Handle      string
	Type        RefType
	RefID       string
	Fingerprint string
	Summary     string
}

type ToolCallRecord struct {
	CallID     string
	Round      int
	Ordinal    int
	ToolKey    string
	Version    string
	ArgsHash   string
	Status     string
	ResultHash string
	Handles    []string
	ErrorCode  string
}

type AskInput struct {
	SellerAccountID int64
	UserID          *int64
	SessionID       *int64
	Question        string
	AttachmentIDs   []string
	ClientRequestID string
	BindingID       *int64
}

type AskResult struct {
	SessionID          int64           `json:"session_id"`
	SessionCreated     bool            `json:"session_created,omitempty"`
	TurnID             int64           `json:"turn_id"`
	UserMessageID      int64           `json:"user_message_id"`
	AssistantMessageID *int64          `json:"assistant_message_id,omitempty"`
	MessageID          *int64          `json:"message_id,omitempty"`
	Answer             string          `json:"answer"`
	AnswerMode         AnswerMode      `json:"answer_mode"`
	ContextMode        ContextMode     `json:"context_mode"`
	GroundingStatus    GroundingStatus `json:"grounding_status"`
	Publication        *PubDTO         `json:"publication"`
	ReferencedEntities []EntityRef     `json:"referenced_entities"`
	GroundingRefs      []GroundingDTO  `json:"grounding_refs"`
	Limitations        []string        `json:"limitations"`
	Attachments        []AttachmentDTO `json:"attachments,omitempty"`
	ErrorCode          string          `json:"error_code,omitempty"`
	Cancelled          bool            `json:"cancelled,omitempty"`
}

type PubDTO struct {
	ID               string `json:"id"`
	AnalysisDate     string `json:"analysis_date"`
	AnalysisCutoffAt string `json:"analysis_cutoff_at"`
}

type GroundingDTO struct {
	Handle string `json:"handle"`
	Type   string `json:"type"`
	ID     string `json:"id"`
}

type AttachmentDTO struct {
	ID           string `json:"id"`
	FileName     string `json:"file_name"`
	MimeType     string `json:"mime_type"`
	FileSize     int64  `json:"file_size"`
	ExportType   string `json:"export_type"`
	Format       string `json:"format"`
	DownloadURL  string `json:"download_url"`
	CreatedAt    string `json:"created_at"`
	Completeness string `json:"completeness,omitempty"`
	Warning      string `json:"warning,omitempty"`
}

type StreamEvent struct {
	Type   string
	Status string
	Delta  string
	Result *AskResult
	Error  string
}

type HandoffInput struct {
	SellerAccountID int64
	UserID          *int64
	ActionViewID    string
	ClientRequestID string
}

type HandoffResult struct {
	SessionID int64  `json:"session_id"`
	BindingID int64  `json:"binding_id"`
	Title     string `json:"title"`
	Created   bool   `json:"created"`
}

type TurnRow struct {
	ID                  int64
	SessionID           int64
	SellerID            int64
	UserMessageID       int64
	AssistantMessageID  *int64
	AnswerMode          AnswerMode
	ContextMode         ContextMode
	Status              TurnStatus
	PublicationID       *int64
	UniverseID          *string
	ModelPolicyVersion  string
	ToolPolicyVersion   string
	MemoryPolicyVersion string
	PromptVersion       string
	ResultSchemaVersion string
	GroundingStatus     GroundingStatus
	ResultHash          string
	ErrorCode           string
}
