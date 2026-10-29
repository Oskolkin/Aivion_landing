package actions

import "encoding/json"

const OperationalDetailFormatVersion = "actions_detail_v2"

const (
	DecisionModeDirect      = "direct"
	DecisionModeConditional = "conditional"
	DecisionModeDiagnostic  = "diagnostic"

	ConfidenceHigh   = "high"
	ConfidenceMedium = "medium"
	ConfidenceLow    = "low"
)

// OperationalDetail is the v2 task-oriented action format for the detail UI.
type OperationalDetail struct {
	Status            string                       `json:"status"`
	FormatVersion     string                       `json:"format_version"`
	Why               OperationalWhy               `json:"why"`
	ActionPlan        OperationalActionPlan        `json:"action_plan"`
	ObjectActions     []OperationalObjectAction    `json:"object_actions,omitempty"`
	SecondaryActions  []OperationalSecondaryAction `json:"secondary_actions,omitempty"`
	HiddenLimitations []string                     `json:"hidden_limitations,omitempty"`
}

type OperationalWhy struct {
	Headline string            `json:"headline"`
	Text     string            `json:"text"`
	Facts    []OperationalFact `json:"facts,omitempty"`
}

type OperationalFact struct {
	FactID         string `json:"fact_id"`
	Label          string `json:"label"`
	Value          string `json:"value"`
	Unit           string `json:"unit,omitempty"`
	FormattedValue string `json:"formatted_value,omitempty"`
}

type OperationalActionPlan struct {
	Headline string            `json:"headline"`
	Steps    []OperationalStep `json:"steps"`
}

type OperationalStep struct {
	StepID          string                    `json:"step_id"`
	ActionKind      string                    `json:"action_kind,omitempty"`
	Title           string                    `json:"title"`
	Instruction     string                    `json:"instruction"`
	WhyThisStep     string                    `json:"why_this_step"`
	ExpectedResult  string                    `json:"expected_result"`
	Confidence      string                    `json:"confidence,omitempty"`
	DecisionMode    string                    `json:"decision_mode,omitempty"`
	EvidenceFactIDs []string                  `json:"evidence_fact_ids,omitempty"`
	TargetEntities  []OperationalTargetEntity `json:"target_entities,omitempty"`
}

type OperationalTargetEntity struct {
	EntityType string `json:"entity_type"`
	ID         string `json:"id"`
	Name       string `json:"name,omitempty"`
}

type OperationalObjectAction struct {
	EntityType        string                 `json:"entity_type"`
	EntityID          string                 `json:"entity_id"`
	Name              string                 `json:"name,omitempty"`
	Metrics           []OperationalMetricRef `json:"metrics,omitempty"`
	RecommendedAction string                 `json:"recommended_action"`
	Reason            string                 `json:"reason,omitempty"`
	Confidence        string                 `json:"confidence,omitempty"`
}

type OperationalMetricRef struct {
	Label          string `json:"label"`
	Value          string `json:"value"`
	MetricCode     string `json:"metric_code,omitempty"`
	DisplayLabel   string `json:"display_label,omitempty"`
	FormattedValue string `json:"formatted_value,omitempty"`
}

type OperationalSecondaryAction struct {
	Type  string `json:"type"`
	Label string `json:"label"`
}

// actionPayload wraps operational_detail stored in action_items.payload_json.
type actionPayload struct {
	OperationalDetail *OperationalDetail    `json:"operational_detail,omitempty"`
	AIFirst           *aiFirstActionPayload `json:"ai_first,omitempty"`
}

// aiFirstActionPayload persists AI-first Stage 2 fields inside payload_json
// without changing the action_items schema.
type aiFirstActionPayload struct {
	UrgencyMarker            string                      `json:"urgency_marker,omitempty"`
	AffectedObjectsSummary   string                      `json:"affected_objects_summary,omitempty"`
	AffectedObjectsTotal     int                         `json:"affected_objects_total,omitempty"`
	AffectedObjectsDisplayed int                         `json:"affected_objects_displayed,omitempty"`
	AffectedObjectsTruncated bool                        `json:"affected_objects_truncated,omitempty"`
	OperationalGoal          string                      `json:"operational_goal,omitempty"`
	ExecutionTier            string                      `json:"execution_tier,omitempty"`
	CompletionCriteria       string                      `json:"completion_criteria,omitempty"`
	CompletionConditions     []ActionCompletionCondition `json:"completion_conditions,omitempty"`
	VisibleLimitations       []string                    `json:"visible_limitations,omitempty"`
}

func cloneOperationalDetail(d *OperationalDetail) *OperationalDetail {
	if d == nil {
		return nil
	}
	cloned := *d
	return &cloned
}

func marshalActionPayload(detail *OperationalDetail) []byte {
	return marshalActionPayloadFull(detail, nil)
}

// MarshalActionItemPayloadJSON persists operational_detail and AI-first fields.
func MarshalActionItemPayloadJSON(item ActionItem) []byte {
	return marshalActionPayloadFull(item.OperationalDetail, aiFirstPayloadFromItem(item))
}

func marshalActionPayloadFull(detail *OperationalDetail, aiFirst *aiFirstActionPayload) []byte {
	if detail == nil && aiFirst == nil {
		return []byte("{}")
	}
	b, err := json.Marshal(actionPayload{OperationalDetail: detail, AIFirst: aiFirst})
	if err != nil {
		return []byte("{}")
	}
	return b
}

func aiFirstPayloadFromItem(item ActionItem) *aiFirstActionPayload {
	if item.UrgencyMarker == "" && item.AffectedObjectsSummary == "" &&
		item.CompletionCriteria == "" && len(item.CompletionConditions) == 0 && len(item.VisibleLimitations) == 0 &&
		item.AffectedObjectsTotal == 0 && item.OperationalGoal == "" && item.ExecutionTier == "" {
		return nil
	}
	return &aiFirstActionPayload{
		UrgencyMarker:            item.UrgencyMarker,
		AffectedObjectsSummary:   item.AffectedObjectsSummary,
		AffectedObjectsTotal:     item.AffectedObjectsTotal,
		AffectedObjectsDisplayed: item.AffectedObjectsDisplayed,
		AffectedObjectsTruncated: item.AffectedObjectsTruncated,
		OperationalGoal:          item.OperationalGoal,
		ExecutionTier:            item.ExecutionTier,
		CompletionCriteria:       item.CompletionCriteria,
		CompletionConditions:     item.CompletionConditions,
		VisibleLimitations:       item.VisibleLimitations,
	}
}

func parseOperationalDetailFromPayload(raw []byte) *OperationalDetail {
	detail, _ := parseActionPayload(raw)
	return detail
}

// AIFirstPayloadMeta holds AI-first fields stored inside payload_json.
type AIFirstPayloadMeta struct {
	UrgencyMarker            string                      `json:"urgency_marker,omitempty"`
	AffectedObjectsSummary   string                      `json:"affected_objects_summary,omitempty"`
	AffectedObjectsTotal     int                         `json:"affected_objects_total,omitempty"`
	AffectedObjectsDisplayed int                         `json:"affected_objects_displayed,omitempty"`
	AffectedObjectsTruncated bool                        `json:"affected_objects_truncated,omitempty"`
	OperationalGoal          string                      `json:"operational_goal,omitempty"`
	ExecutionTier            string                      `json:"execution_tier,omitempty"`
	CompletionCriteria       string                      `json:"completion_criteria,omitempty"`
	CompletionConditions     []ActionCompletionCondition `json:"completion_conditions,omitempty"`
	VisibleLimitations       []string                    `json:"visible_limitations,omitempty"`
}

// ParseActionPayload decodes operational_detail and AI-first metadata from payload_json.
func ParseActionPayload(raw []byte) (*OperationalDetail, *AIFirstPayloadMeta) {
	op, ai := parseActionPayload(raw)
	if ai == nil {
		return op, nil
	}
	return op, &AIFirstPayloadMeta{
		UrgencyMarker:            ai.UrgencyMarker,
		AffectedObjectsSummary:   ai.AffectedObjectsSummary,
		AffectedObjectsTotal:     ai.AffectedObjectsTotal,
		AffectedObjectsDisplayed: ai.AffectedObjectsDisplayed,
		AffectedObjectsTruncated: ai.AffectedObjectsTruncated,
		OperationalGoal:          ai.OperationalGoal,
		ExecutionTier:            ai.ExecutionTier,
		CompletionCriteria:       ai.CompletionCriteria,
		CompletionConditions:     ai.CompletionConditions,
		VisibleLimitations:       ai.VisibleLimitations,
	}
}

func parseActionPayload(raw []byte) (*OperationalDetail, *aiFirstActionPayload) {
	if len(raw) == 0 {
		return nil, nil
	}
	var payload actionPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, nil
	}
	return payload.OperationalDetail, payload.AIFirst
}
