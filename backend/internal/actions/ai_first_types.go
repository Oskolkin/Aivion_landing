package actions

import "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"

type ActionsAIFirstInput struct {
	SchemaVersion string `json:"schema_version"`

	SellerAccountID int64  `json:"seller_account_id"`
	MetricDate      string `json:"metric_date"`

	DataPackSchemaVersion string `json:"data_pack_schema_version,omitempty"`
	DataPackHash          string `json:"data_pack_hash,omitempty"`

	// DataPack is backend-only and never serialized in provider wire payloads.
	DataPack *aidatapack.SellerAIDataPack `json:"-"`

	GlobalContext     *GlobalSemanticContext `json:"global_context,omitempty"`
	GlobalContextHash string                 `json:"global_context_hash,omitempty"`

	ActionCandidates []AIFirstActionCandidate            `json:"action_candidates,omitempty"`
	FactCatalog      map[string]aidatapack.CompactFact   `json:"fact_catalog,omitempty"`
	EntityCatalog    map[string]aidatapack.CompactEntity `json:"entity_catalog,omitempty"`
	DataLimitations  []string                            `json:"data_limitations,omitempty"`
	FailedChunkIDs   []string                            `json:"failed_chunk_ids,omitempty"`

	AllowedActionTypes []ActionType     `json:"allowed_action_types"`
	AllowedCategories  []ActionCategory `json:"allowed_categories"`
	AllowedPriorities  []ActionPriority `json:"allowed_priorities"`
	AllowedCTAKeys     []string         `json:"allowed_cta_keys,omitempty"`

	ForbiddenClaims []string `json:"forbidden_claims,omitempty"`

	MaxActions int `json:"max_actions,omitempty"`
}

type ActionsAIFirstOutput struct {
	SchemaVersion string               `json:"schema_version"`
	Focus         *ActionsAIFirstFocus `json:"focus,omitempty"`
	Actions       []AIFirstDraftAction `json:"actions"`
	Notes         []string             `json:"notes,omitempty"`
}

// ActionsAIFirstFocus is the AI-generated "focus of the day" built from the
// generated actions and the Data Pack (not an independent generic summary).
type ActionsAIFirstFocus struct {
	Headline        string           `json:"headline"`
	Summary         string           `json:"summary"`
	PriorityOrder   []ActionCategory `json:"priority_order,omitempty"`
	StartWith       string           `json:"start_with,omitempty"`
	Limitations     []string         `json:"limitations,omitempty"`
	EvidenceFactIDs []string         `json:"evidence_fact_ids,omitempty"`
}

// Urgency markers allowed on AI-first draft actions.
const (
	AIFirstUrgencyUrgent = "urgent"
	AIFirstUrgencyWatch  = "watch"
)

type AIFirstDraftAction struct {
	Type     ActionType     `json:"type"`
	Category ActionCategory `json:"category"`
	Priority ActionPriority `json:"priority"`

	// OperationalGoal is an AI-defined scope key for technical deduplication, e.g.
	// replenish_stock, reduce_excess_stock, stop_ad_waste.
	OperationalGoal string `json:"operational_goal,omitempty"`

	// ScopeType declares whether the action applies to the whole account, specific
	// entities, or both. Required for new AI-first v2_3+ output.
	ScopeType AIFirstActionScopeType `json:"scope_type,omitempty"`

	Title           string `json:"title"`
	TitleTemplate   string `json:"title_template,omitempty"`
	Summary         string `json:"summary,omitempty"`
	SummaryTemplate string `json:"summary_template,omitempty"`
	Reason          string `json:"reason"`
	NextStep        string `json:"next_step"`
	ExpectedEffect  string `json:"expected_effect,omitempty"`

	ComputedValues []AIFirstComputedValue `json:"computed_values,omitempty"`

	EvidenceFactIDs []string `json:"evidence_fact_ids"`
	CTAKeys         []string `json:"cta_keys,omitempty"`

	AffectedEntities []AIFirstAffectedEntityRef `json:"affected_entities"`

	AffectedObjectsTotal     int  `json:"affected_objects_total,omitempty"`
	AffectedObjectsDisplayed int  `json:"affected_objects_displayed,omitempty"`
	AffectedObjectsTruncated bool `json:"affected_objects_truncated,omitempty"`

	// UrgencyMarker is an optional in-action marker: "urgent", "watch" or empty.
	// It is not a page filter.
	UrgencyMarker string `json:"urgency_marker,omitempty"`

	// AffectedObjectsSummary is an optional AI note about object selection (not a count source).
	AffectedObjectsSummary string `json:"affected_objects_summary,omitempty"`

	// ObjectActions is the in-action object list: what exactly is affected and
	// what to do with each object. Mandatory for object-based action types.
	ObjectActions []AIFirstObjectAction `json:"object_actions,omitempty"`

	// ObjectActionsCoverage is optional AI metadata describing object_actions completeness.
	ObjectActionsCoverage *AIFirstObjectActionsCoverage `json:"object_actions_coverage,omitempty"`

	// ActionPlan is the ordered "что сделать" plan.
	ActionPlan []AIFirstActionStep `json:"action_plan,omitempty"`

	// CompletionCriteria is the human-readable display text for completion checks.
	CompletionCriteria string `json:"completion_criteria"`

	// CompletionConditions are machine-readable checks chosen by AI.
	CompletionConditions []AIFirstCompletionCondition `json:"completion_conditions,omitempty"`

	// VisibleLimitations are seller-facing analysis limitations.
	VisibleLimitations []string `json:"visible_limitations,omitempty"`

	Limitations []string `json:"limitations,omitempty"`

	OperationalDetail *OperationalDetail `json:"operational_detail,omitempty"`

	NotesForLegacyCheck []string `json:"notes_for_legacy_check,omitempty"`

	SourceCandidateIDs []string `json:"source_candidate_ids,omitempty"`
}

type AIFirstAffectedEntityRef struct {
	EntityType string   `json:"entity_type"`
	EntityID   string   `json:"entity_id"`
	EntityRef  string   `json:"entity_ref,omitempty"`
	Name       string   `json:"name,omitempty"`
	FactIDs    []string `json:"fact_ids,omitempty"`
}

// AIFirstObjectAction is a per-object task inside a self-sufficient action.
type AIFirstObjectAction struct {
	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id"`
	Name       string `json:"name,omitempty"`

	Metrics []AIFirstObjectMetric `json:"metrics,omitempty"`

	RecommendedAction string `json:"recommended_action"`
	Reason            string `json:"reason,omitempty"`

	EvidenceFactIDs []string `json:"evidence_fact_ids,omitempty"`
}

type AIFirstComputedValue struct {
	Key     string   `json:"key"`
	Op      string   `json:"op"`
	FactIDs []string `json:"fact_ids"`
}

type AIFirstObjectMetric struct {
	FactID     string `json:"fact_id,omitempty"`
	MetricCode string `json:"metric_code,omitempty"`
	// Legacy advisory fields — not source of truth for v2_2+ output.
	Label string `json:"label,omitempty"`
	Value string `json:"value,omitempty"`
	Unit  string `json:"unit,omitempty"`
	// Hydrated by backend normalization for persistence/UI.
	DisplayLabel   string `json:"display_label,omitempty"`
	FormattedValue string `json:"formatted_value,omitempty"`
}

// AIFirstNormalizationResult captures technical normalization diagnostics.
type AIFirstNormalizationResult struct {
	Issues  []ValidationIssue    `json:"issues,omitempty"`
	Actions []AIFirstDraftAction `json:"actions"`

	ReturnedActionCount int `json:"returned_action_count,omitempty"`
	DroppedActionCount  int `json:"dropped_action_count,omitempty"`

	CompletionConditionsReturnedCount      int `json:"completion_conditions_returned_count,omitempty"`
	CompletionConditionsValidCount         int `json:"completion_conditions_valid_count,omitempty"`
	CompletionConditionsDroppedCount       int `json:"completion_conditions_dropped_count,omitempty"`
	CompletionConditionsFactIDRemovedCount int `json:"completion_conditions_fact_id_removed_count,omitempty"`
	CompletionConditionsEmptyActionsCount  int `json:"completion_conditions_empty_actions_count,omitempty"`
}

type AIFirstActionStep struct {
	Title          string `json:"title,omitempty"`
	Instruction    string `json:"instruction"`
	ExpectedResult string `json:"expected_result,omitempty"`
}

// AIFirstActionCandidate is a compact action proposal produced during shared
// chunk analysis. Backend assigns candidate_id and chunk_id on later stages.
type AIFirstActionCandidate struct {
	CandidateID string `json:"candidate_id,omitempty"`
	ChunkID     string `json:"chunk_id,omitempty"`

	Type          ActionType     `json:"type"`
	Category      ActionCategory `json:"category"`
	LocalPriority ActionPriority `json:"local_priority"`

	Problem           string `json:"problem"`
	RecommendedAction string `json:"recommended_action"`
	WhyActionable     string `json:"why_actionable"`

	AffectedEntities []AIFirstAffectedEntityRef `json:"affected_entities"`

	EvidenceFactIDs  []string `json:"evidence_fact_ids"`
	EvidenceFactRefs []string `json:"evidence_fact_refs,omitempty"`

	SourceSignalIDs []string `json:"source_signal_ids"`

	Limitations []string `json:"limitations,omitempty"`
}
