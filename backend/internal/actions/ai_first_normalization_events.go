package actions

// AIPipelineNormalizationEvent records a structured normalization step for Inspector.
type AIPipelineNormalizationEvent struct {
	Stage        string         `json:"stage"`
	ActionIndex  int            `json:"action_index"`
	ActionKey    string         `json:"action_key,omitempty"`
	EventCode    string         `json:"event_code"`
	Severity     string         `json:"severity"`
	BeforeCount  *int           `json:"before_count,omitempty"`
	AfterCount   *int           `json:"after_count,omitempty"`
	EntityIDs    []string       `json:"entity_ids,omitempty"`
	CandidateIDs []string       `json:"candidate_ids,omitempty"`
	FactIDs      []string       `json:"fact_ids,omitempty"`
	Details      map[string]any `json:"details,omitempty"`
}

func intPtr(v int) *int { return &v }

func appendNormEvent(events *[]AIPipelineNormalizationEvent, ev AIPipelineNormalizationEvent) {
	*events = append(*events, ev)
}
