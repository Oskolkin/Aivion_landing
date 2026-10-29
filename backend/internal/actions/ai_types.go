package actions

import (
	"context"
	"encoding/json"
	"time"
)

const (
	GenerationRunStatusRunning   = "running"
	GenerationRunStatusCompleted = "completed"
	GenerationRunStatusFailed    = "failed"

	GenerationFinalStatusCompleted         = "completed"
	GenerationFinalStatusCompletedRepaired = "completed_repaired"
	GenerationFinalStatusCompletedPartial  = "completed_partial"
	// GenerationFinalStatusCompletedAfterRetry marks an AI-first run whose
	// validation issues were fixed by an explicit AI retry (never by silent
	// backend repair).
	GenerationFinalStatusCompletedAfterRetry = "completed_after_retry"
	GenerationFinalStatusFallback            = "fallback"
	GenerationFinalStatusEmpty               = "empty"
	GenerationFinalStatusFailed              = "failed"

	CategoryRunStatusCompleted         = "completed"
	CategoryRunStatusCompletedPartial  = "completed_partial"
	CategoryRunStatusCompletedRepaired = "completed_repaired"
	CategoryRunStatusFailed            = "failed"
	CategoryRunStatusFallback          = "fallback"
	CategoryRunStatusSkippedEmpty      = "skipped_empty"
	CategoryRunStatusRepair            = "repaired"

	DomainLegacyGlobalCheck = "global_review"
	DomainAIFirst           = "ai_first"

	ValidationStatusValid             = "valid"
	ValidationStatusValidWithWarnings = "valid_with_warnings"
	ValidationStatusRepairable        = "repairable"
	ValidationStatusInvalid           = "invalid"

	IssueSeverityWarning    = "warning"
	IssueSeverityRepairable = "repairable"
	IssueSeverityFatal      = "fatal"
)

// GenerateActionsOptions controls the AI generation pipeline.
type GenerateActionsOptions struct {
	ForceRegenerate bool
	// ForceActionsRegenerate creates a new Actions run even when input hashes match.
	ForceActionsRegenerate bool
	DryRun                 bool

	AIEnabled bool
	AIFirst   bool

	EnableAIRepair          bool
	AlwaysRunAIRepair       bool
	EnableLegacyGlobalCheck bool

	MaxActionsOutput int
	MaxTopActions    int

	PipelineRunID     *int64
	GlobalContextHash string
	// ExistingActionsRunID reuses a failed pipeline-linked generation run on resume.
	ExistingActionsRunID *int64
}

// DefaultGenerateActionsOptions returns pipeline defaults.
func DefaultGenerateActionsOptions() GenerateActionsOptions {
	return GenerateActionsOptions{
		AIEnabled:               true,
		EnableAIRepair:          true,
		AlwaysRunAIRepair:       false,
		EnableLegacyGlobalCheck: true,
		MaxActionsOutput:        20,
		MaxTopActions:           3,
	}
}

// GenerateActionsResult is the outcome of a generation run.
type GenerateActionsResult struct {
	RunID         int64
	MetricDate    string
	FinalStatus   string
	Actions       []ActionItem
	TopActionKeys []string
	AIFocus       *ActionsAIFocus
	AcceptedCount int
	RepairedCount int
	DroppedCount  int
	FallbackCount int
	DryRun        bool

	ReturnedCount         int
	ValidBeforeLimitCount int
	TruncatedCount        int
	ComposeCalls          int
	ObjectDetailCalls     int
	ValidatorMode         string
	FailureSubreason      string
	DropIssueCounts       map[string]int `json:"-"`

	InputTokens  int `json:"-"`
	OutputTokens int `json:"-"`
	LatencyMs    int64
	RequestID    string

	// AIFirstValidation is populated for unified pipeline debug artifacts (not API-facing).
	AIFirstValidation *AIFirstValidationResult `json:"-"`

	// PipelineDiagnostics captures stage-level action loss and quality counters for observability.
	PipelineDiagnostics *AIFirstPipelineDiagnostics `json:"-"`

	// Two-phase debug payloads for pipeline artifacts (not API-facing).
	ComposeOutput       *ActionsAIFirstComposeOutput   `json:"-"`
	ComposeWireInput    json.RawMessage                `json:"-"`
	ComposeRawResponse  *AIRawResponse                 `json:"-"`
	BaseActions         []AIFirstBaseAction            `json:"-"`
	DetailMerges        []ObjectDetailMergeResult      `json:"-"`
	NormalizationEvents []AIPipelineNormalizationEvent `json:"-"`

	// Lifecycle diagnostics for debug artifacts.
	LifecycleSummary ActionsLifecycleSummary     `json:"-"`
	ActionLifecycles []ActionLifecycleDiagnostic `json:"-"`
}

// ActionAIClient abstracts domain compose, repair and legacy global check AI calls.
type ActionAIClient interface {
	ComposeDomainActions(ctx context.Context, input DomainComposerInput) (*DomainComposerOutput, *AIRawResponse, error)
	RepairDomainActions(ctx context.Context, input DomainRepairInput) (*DomainComposerOutput, *AIRawResponse, error)
	RunLegacyGlobalCheck(ctx context.Context, input LegacyGlobalCheckInput) (*LegacyGlobalCheckOutput, *AIRawResponse, error)
}

// AIRawResponse captures model metadata for persistence and debugging.
type AIRawResponse struct {
	Model        string
	PromptJSON   json.RawMessage
	ResponseJSON json.RawMessage
	RawText      string
	RequestID    string
	InputTokens  int
	OutputTokens int
	LatencyMs    int64

	ProviderStatus           string
	FinishReason             string
	IncompleteReason         string
	OutputTruncated          bool
	HTTPStatusCode           int
	ProviderError            string
	PromptVersion            string
	InputSchemaVersion       string
	OutputSchemaVersion      string
	RequestedMaxOutputTokens int
	TimeoutSeconds           int
	ParseStatus              string
	ParseError               string
	ParseErrorType           string
	ParseErrorOffset         *int
}

// DomainComposerInput is sent to a domain-specific AI composer.
type DomainComposerInput struct {
	Domain          string
	DomainContext   ActionDomainContext
	AccountSnapshot AccountSnapshot
	ForbiddenClaims []string
	AllowedTypes    []ActionType
}

// DomainComposerOutput is the strict JSON schema returned by domain composers.
type DomainComposerOutput struct {
	Domain  string           `json:"domain"`
	Actions []DomainAIAction `json:"actions"`
}

// DomainAIAction is a draft action produced by AI before backend enrichment.
type DomainAIAction struct {
	SourceCandidateIDs []string `json:"source_candidate_ids"`

	Type     ActionType     `json:"type"`
	Category ActionCategory `json:"category"`
	Priority ActionPriority `json:"priority"`

	Title          string `json:"title"`
	Summary        string `json:"summary"`
	Reason         string `json:"reason"`
	NextStep       string `json:"next_step"`
	ExpectedEffect string `json:"expected_effect,omitempty"`

	EvidenceFactIDs []string `json:"evidence_fact_ids"`
	CTAKeys         []string `json:"cta_keys"`
	Limitations     []string `json:"limitations,omitempty"`

	OperationalDetail *OperationalDetail `json:"operational_detail,omitempty"`

	NotesForLegacyCheck []string `json:"notes_for_legacy_check,omitempty"`
}

// DomainRepairInput is sent to the AI repair step.
type DomainRepairInput struct {
	DomainContext    ActionDomainContext
	DraftOutput      DomainComposerOutput
	ValidationIssues []ValidationIssue
}

// DomainValidationResult is the outcome of backend validation for a domain output.
type DomainValidationResult struct {
	Status          string            `json:"status"`
	AcceptedActions []DomainAIAction  `json:"accepted_actions"`
	Issues          []ValidationIssue `json:"issues"`
	DroppedActions  []DroppedAction   `json:"dropped_actions,omitempty"`
}

// ValidationIssue describes a validation problem on a draft action.
type ValidationIssue struct {
	Severity    string `json:"severity"`
	Code        string `json:"code"`
	Message     string `json:"message"`
	ActionIndex *int   `json:"action_index,omitempty"`
	Field       string `json:"field,omitempty"`
}

// DroppedAction records an action removed during validation.
type DroppedAction struct {
	Action  DomainAIAction `json:"action"`
	Reasons []string       `json:"reasons"`
}

// ValidationRules configures backend validation for a domain.
type ValidationRules struct {
	Domain              string
	AllowedTypes        []ActionType
	ForbiddenClaims     []string
	RequiredLimitations map[ActionType][]string // substring markers that must appear
}

// ScoredDraftAction is a validated draft with backend score and temp ID for legacy global check.
type ScoredDraftAction struct {
	TempActionID     string
	Action           DomainAIAction
	Score            int
	Priority         ActionPriority
	Domain           string
	EntityGroupRef   string
	SourceCandidates []ActionCandidate
}

// DraftActionForLegacyGlobalCheck is a compact action sent to the legacy global check.
type DraftActionForLegacyGlobalCheck struct {
	TempActionID string         `json:"temp_action_id"`
	Domain       string         `json:"domain"`
	Type         ActionType     `json:"type"`
	Category     ActionCategory `json:"category"`
	Priority     ActionPriority `json:"priority"`
	Score        int            `json:"score"`
	Title        string         `json:"title"`
	Summary      string         `json:"summary"`
	Reason       string         `json:"reason"`
	NextStep     string         `json:"next_step"`
}

// LegacyGlobalCheckInput is sent to the legacy global check AI step.
type LegacyGlobalCheckInput struct {
	MetricDate       string
	AccountSnapshot  AccountSnapshot
	DraftActions     []DraftActionForLegacyGlobalCheck
	MaxTopActions    int
	MaxActionsOutput int
	ForbiddenClaims  []string
}

// LegacyGlobalCheckOutput is the strict JSON schema from the legacy global check.
type LegacyGlobalCheckOutput struct {
	AIFocus           *ActionsAIFocus              `json:"ai_focus,omitempty"`
	OrderedActionRefs []LegacyGlobalCheckActionRef `json:"ordered_action_refs"`
	TopActionRefs     []string                     `json:"top_action_refs"`
	Notes             []string                     `json:"notes,omitempty"`
}

// LegacyGlobalCheckActionRef references a draft action for ordering and priority adjustment.
type LegacyGlobalCheckActionRef struct {
	TempActionID string         `json:"temp_action_id"`
	Priority     ActionPriority `json:"priority,omitempty"`
	Rationale    string         `json:"rationale,omitempty"`
}

// LegacyGlobalCheckValidationResult is the outcome of legacy global check validation.
//
// Validity is tracked per-part so that a valid ai_focus is never discarded
// because of unrelated problems in ordered_action_refs/top_action_refs.
type LegacyGlobalCheckValidationResult struct {
	// Valid is true only when every part is valid (no issues at all).
	Valid bool
	// OrderingValid is true when ordered_action_refs resolved cleanly.
	OrderingValid bool
	// TopValid is true when top_action_refs resolved cleanly and within limits.
	TopValid bool
	// FocusValid is true when ai_focus is present and contains no forbidden claims.
	FocusValid bool

	Issues            []ValidationIssue
	FocusIssues       []ValidationIssue
	OrderedTempIDs    []string
	TopTempIDs        []string
	AIFocus           *ActionsAIFocus
	PriorityOverrides map[string]ActionPriority

	UnknownOrderedRefs []string
	UnknownTopRefs     []string
	AvailableRefs      []string
}

// LegacyGlobalCheckDiagnostic is a structured record persisted to
// action_generation_category_runs.validation_json / final_validation_json so
// that any fallback in the legacy global check is observable.
type LegacyGlobalCheckDiagnostic struct {
	Stage            string            `json:"stage"`
	Status           string            `json:"status"`
	Reason           string            `json:"reason,omitempty"`
	OrderingValid    bool              `json:"ordering_valid"`
	TopValid         bool              `json:"top_valid"`
	FocusValid       bool              `json:"focus_valid"`
	FocusApplied     bool              `json:"focus_applied"`
	FocusSource      string            `json:"focus_source,omitempty"`
	FallbackScope    string            `json:"fallback_scope,omitempty"`
	UnknownRefs      []string          `json:"unknown_refs,omitempty"`
	AvailableRefs    []string          `json:"available_refs,omitempty"`
	ParseError       string            `json:"parse_error,omitempty"`
	SanitizedNumbers []string          `json:"sanitized_numbers,omitempty"`
	Issues           []ValidationIssue `json:"issues,omitempty"`
}

// DomainPipelineResult captures per-domain generation outcome.
type DomainPipelineResult struct {
	Domain          string
	Status          string
	AcceptedActions []DomainAIAction
	UsedFallback    bool
	Repaired        bool
	DroppedCount    int
	RawResponse     *AIRawResponse
	Validation      DomainValidationResult
	RepairResponse  *AIRawResponse
	FinalValidation DomainValidationResult
	InputContext    json.RawMessage
}

// GenerationRepository extends read repository with generation persistence.
type GenerationRepository interface {
	Repository
	CreateActionGenerationRun(ctx context.Context, sellerAccountID int64, metricDate time.Time, status string) (int64, error)
	UpdateActionGenerationRun(ctx context.Context, params UpdateGenerationRunParams) error
	CreateActionGenerationCategoryRun(ctx context.Context, params CreateCategoryRunParams) (int64, error)
	UpdateActionGenerationCategoryRun(ctx context.Context, params UpdateCategoryRunParams) error
	UpsertActionItem(ctx context.Context, params UpsertActionItemParams) error
	DeleteStaleActionItemsForDate(ctx context.Context, sellerAccountID int64, metricDate time.Time, keepKeys []string) error
	LinkActionGenerationRunToPipeline(ctx context.Context, pipelineRunID, actionsRunID int64, dataPackHash, globalContextHash string) error
	PersistGenerationAtomic(ctx context.Context, params PersistGenerationParams) error
}

// UpdateGenerationRunParams updates a generation run after pipeline completion.
type UpdateGenerationRunParams struct {
	RunID             int64
	Status            string
	ContextHash       string
	FinalStatus       string
	AcceptedCount     int
	RepairedCount     int
	DroppedCount      int
	FallbackCount     int
	AIFocusJSON       []byte
	TopActionKeysJSON []byte
	DataQualityNotes  []byte

	PipelineRunID     *int64
	DataPackHash      string
	GlobalContextHash string
}

// CreateCategoryRunParams creates a category run record.
type CreateCategoryRunParams struct {
	RunID               int64
	SellerAccountID     int64
	Domain              string
	Status              string
	Model               string
	InputContextJSON    []byte
	RawResponseJSON     []byte
	ValidationJSON      []byte
	RepairResponseJSON  []byte
	FinalValidationJSON []byte
	AcceptedCount       int
	RepairedCount       int
	DroppedCount        int
}

// UpdateCategoryRunParams updates a category run record.
type UpdateCategoryRunParams struct {
	CategoryRunID       int64
	Status              string
	Model               string
	InputContextJSON    []byte
	RawResponseJSON     []byte
	ValidationJSON      []byte
	RepairResponseJSON  []byte
	FinalValidationJSON []byte
	AcceptedCount       int
	RepairedCount       int
	DroppedCount        int
}

// UpsertActionItemParams holds fields for upserting a generated action item.
type UpsertActionItemParams struct {
	SellerAccountID      int64
	RunID                int64
	ActionKey            string
	Type                 ActionType
	Category             ActionCategory
	Priority             ActionPriority
	Title                string
	Summary              string
	Reason               string
	NextStep             string
	ExpectedEffect       string
	EvidenceJSON         []byte
	LimitationsJSON      []byte
	CTAJSON              []byte
	AffectedEntitiesJSON []byte
	EntityGroupRef       string
	SourceCandidateIDs   []byte
	EvidenceFactIDs      []byte
	PayloadJSON          []byte
	MetricDate           time.Time
}

// PersistGenerationParams bundles atomic final persistence.
type PersistGenerationParams struct {
	RunUpdate    UpdateGenerationRunParams
	ActionItems  []UpsertActionItemParams
	StaleDelete  *StaleDeleteParams
	CategoryRuns []UpdateCategoryRunParams // optional final updates

	PipelineRunID     *int64
	DataPackHash      string
	GlobalContextHash string
}

// StaleDeleteParams identifies stale actions to remove within one generation run after successful persist.
type StaleDeleteParams struct {
	RunID           int64
	SellerAccountID int64
	MetricDate      time.Time
	KeepKeys        []string
}
