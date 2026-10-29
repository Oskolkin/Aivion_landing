package actions

// ActionsAIFirstComposeOutput is the transport type for Phase A (Compose).
// It contains only top-level action definitions without per-object details.
type ActionsAIFirstComposeOutput struct {
	SchemaVersion string                 `json:"schema_version"`
	Focus         *ActionsAIFirstFocus   `json:"focus,omitempty"`
	Actions       []AIFirstComposeAction `json:"actions"`
	Notes         []string               `json:"notes,omitempty"`
}

// AIFirstComposeAction is a base action from the Compose phase.
type AIFirstComposeAction struct {
	ActionKey string `json:"action_key"`

	Type            ActionType             `json:"type"`
	Category        ActionCategory         `json:"category"`
	Priority        ActionPriority         `json:"priority"`
	ScopeType       AIFirstActionScopeType `json:"scope_type"`
	OperationalGoal string                 `json:"operational_goal"`

	SourceCandidateIDs []string `json:"source_candidate_ids"`
	ExcludedEntityIDs  []string `json:"excluded_entity_ids,omitempty"`

	TitleTemplate   string `json:"title_template,omitempty"`
	SummaryTemplate string `json:"summary_template,omitempty"`
	Title           string `json:"title,omitempty"`
	Summary         string `json:"summary,omitempty"`
	Reason          string `json:"reason"`

	Steps          []string               `json:"steps,omitempty"`
	ActionPlan     []AIFirstActionStep    `json:"action_plan,omitempty"`
	ExpectedEffect string                 `json:"expected_effect,omitempty"`
	ComputedValues []AIFirstComputedValue `json:"computed_values,omitempty"`

	CompletionCriteria   string                       `json:"completion_criteria"`
	CompletionConditions []AIFirstCompletionCondition `json:"completion_conditions,omitempty"`

	ObjectDetailBrief string `json:"object_detail_brief,omitempty"`

	EvidenceFactIDs []string `json:"evidence_fact_ids,omitempty"`
	CTAKeys         []string `json:"cta_keys,omitempty"`
	NextStep        string   `json:"next_step,omitempty"`

	VisibleLimitations []string `json:"visible_limitations,omitempty"`
	Limitations        []string `json:"limitations,omitempty"`
	UrgencyMarker      string   `json:"urgency_marker,omitempty"`
}

// AIFirstBaseAction is a normalized compose action with authoritative entity scope.
type AIFirstBaseAction struct {
	AIFirstComposeAction

	AIIndex int `json:"ai_index"`

	AuthoritativeEntities []AIFirstAffectedEntityRef `json:"authoritative_entities"`
	ValidSourceCandidates []string                   `json:"valid_source_candidate_ids"`
	RemovedCandidateIDs   []string                   `json:"removed_candidate_ids,omitempty"`
	AppliedExclusions     []string                   `json:"applied_exclusions,omitempty"`
	RejectedExclusions    []string                   `json:"rejected_exclusions,omitempty"`

	Dropped    bool   `json:"dropped,omitempty"`
	DropReason string `json:"drop_reason,omitempty"`

	EvidenceDerivation ActionEvidenceDerivation `json:"evidence_derivation,omitempty"`
}
