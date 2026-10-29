package actions

// ObjectDetailBatchInput is the AI input for one Object Detail batch.
type ObjectDetailBatchInput struct {
	Action   ObjectDetailActionContext `json:"action"`
	Batch    ObjectDetailBatchMeta     `json:"batch"`
	Entities []ObjectDetailEntityInput `json:"entities"`
}

type ObjectDetailActionContext struct {
	ActionKey         string                 `json:"action_key"`
	Type              ActionType             `json:"type"`
	ScopeType         AIFirstActionScopeType `json:"scope_type"`
	OperationalGoal   string                 `json:"operational_goal"`
	Priority          ActionPriority         `json:"priority"`
	Reason            string                 `json:"reason"`
	Steps             []string               `json:"steps,omitempty"`
	ObjectDetailBrief string                 `json:"object_detail_brief,omitempty"`
}

type ObjectDetailBatchMeta struct {
	BatchIndex  int `json:"batch_index"`
	BatchCount  int `json:"batch_count"`
	EntityCount int `json:"entity_count"`
}

type ObjectDetailEntityInput struct {
	EntityRef        string                     `json:"entity_ref"`
	EntityType       string                     `json:"entity_type"`
	Name             string                     `json:"name,omitempty"`
	OfferID          string                     `json:"offer_id,omitempty"`
	CandidateContext []ObjectDetailCandidateCtx `json:"candidate_context,omitempty"`
	AvailableFacts   []ObjectDetailFactInput    `json:"available_facts"`
}

type ObjectDetailCandidateCtx struct {
	CandidateID string `json:"candidate_id"`
	Observation string `json:"observation,omitempty"`
}

type ObjectDetailFactInput struct {
	FactRef      string `json:"fact_ref"`
	MetricCode   string `json:"metric_code,omitempty"`
	Value        any    `json:"value,omitempty"`
	Unit         string `json:"unit,omitempty"`
	DisplayLabel string `json:"display_label,omitempty"`
}

// ObjectDetailBatchOutput is the AI output for one Object Detail batch.
type ObjectDetailBatchOutput struct {
	ActionKey     string                     `json:"action_key"`
	BatchIndex    int                        `json:"batch_index"`
	ObjectActions []ObjectDetailObjectAction `json:"object_actions"`
}

type ObjectDetailObjectAction struct {
	EntityRef       string   `json:"entity_ref"`
	Recommendation  string   `json:"recommendation"`
	Reason          string   `json:"reason,omitempty"`
	Steps           []string `json:"steps,omitempty"`
	MetricRefs      []string `json:"metric_refs,omitempty"`
	DataSufficiency string   `json:"data_sufficiency,omitempty"`
}

// ObjectDetailRefMapping maps local refs to real IDs for one batch.
type ObjectDetailRefMapping struct {
	EntityRefs map[string]ObjectDetailEntityMapping `json:"entity_refs"`
	FactRefs   map[string]string                    `json:"fact_refs"`
}

type ObjectDetailEntityMapping struct {
	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id"`
}

// ObjectDetailBatchResult captures one batch execution outcome.
type ObjectDetailBatchResult struct {
	ActionIndex int
	ActionKey   string
	BatchIndex  int
	BatchCount  int
	EntityCount int

	Input       ObjectDetailBatchInput
	Output      *ObjectDetailBatchOutput
	Mapping     ObjectDetailRefMapping
	RawResponse *AIRawResponse

	Status   string // completed, failed, partial, truncated
	Warnings []ValidationIssue

	ValidObjectActions []AIFirstObjectAction
	MissingEntityRefs  []string

	StartedAt       string `json:"started_at,omitempty"`
	FinishedAt      string `json:"finished_at,omitempty"`
	ElapsedMs       int64  `json:"elapsed_ms,omitempty"`
	TimeoutSeconds  int    `json:"timeout_seconds,omitempty"`
	ProviderError   string `json:"provider_error,omitempty"`
	OutputTruncated bool   `json:"output_truncated,omitempty"`
}

// ObjectDetailMergeResult aggregates all batch results for one action.
type ObjectDetailMergeResult struct {
	ActionKey             string
	AuthoritativeTotal    int
	DisplayedCount        int
	Truncated             bool
	ObjectActions         []AIFirstObjectAction
	MissingEntityIDs      []string
	BatchResults          []ObjectDetailBatchResult
	DetailCoveragePartial bool
}
