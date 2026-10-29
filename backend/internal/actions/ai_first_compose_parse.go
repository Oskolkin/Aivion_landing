package actions

import (
	"encoding/json"
	"fmt"
	"strings"
)

func parseActionsAIFirstComposeOutput(raw string) (*ActionsAIFirstComposeOutput, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("invalid_json: %w", errEmptyActionsJSON)
	}
	var wire struct {
		SchemaVersion string          `json:"schema_version"`
		Focus         json.RawMessage `json:"focus"`
		Actions       json.RawMessage `json:"actions"`
		Notes         json.RawMessage `json:"notes"`
	}
	if err := json.Unmarshal([]byte(trimmed), &wire); err != nil {
		if salvaged, ok := salvageTruncatedComposeOutput(trimmed); ok {
			return salvaged, nil
		}
		return nil, fmt.Errorf("invalid_json: %w", err)
	}
	out := &ActionsAIFirstComposeOutput{
		SchemaVersion: strings.TrimSpace(wire.SchemaVersion),
		Actions:       parseFlexibleComposeActions(wire.Actions),
		Notes:         parseFlexibleStringList(wire.Notes),
	}
	if len(wire.Focus) > 0 && string(wire.Focus) != "null" {
		focus, err := parseFlexibleAIFirstFocus(wire.Focus)
		if err == nil {
			out.Focus = focus
		}
	}
	if out.SchemaVersion == "" {
		out.SchemaVersion = ActionsAIFirstSchemaVersionV2
	}
	return out, nil
}

func parseFlexibleComposeActions(raw json.RawMessage) []AIFirstComposeAction {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		if action, ok := parseFlexibleComposeAction(raw); ok {
			return []AIFirstComposeAction{action}
		}
		return nil
	}
	out := make([]AIFirstComposeAction, 0, len(items))
	for _, item := range items {
		if action, ok := parseFlexibleComposeAction(item); ok {
			out = append(out, action)
		}
	}
	return out
}

func parseFlexibleComposeAction(raw json.RawMessage) (AIFirstComposeAction, bool) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return AIFirstComposeAction{}, false
	}
	var action AIFirstComposeAction
	action.ActionKey = parseFlexibleTextField(fields, "action_key")
	unmarshalActionsField(fields, "type", &action.Type)
	unmarshalActionsField(fields, "category", &action.Category)
	unmarshalActionsField(fields, "priority", &action.Priority)
	action.OperationalGoal = parseFlexibleTextField(fields, "operational_goal")
	action.ScopeType = AIFirstActionScopeType(parseFlexibleTextField(fields, "scope_type"))
	action.Title = parseFlexibleTextField(fields, "title")
	action.TitleTemplate = parseFlexibleTextField(fields, "title_template")
	action.Summary = parseFlexibleTextField(fields, "summary")
	action.SummaryTemplate = parseFlexibleTextField(fields, "summary_template")
	action.Reason = parseFlexibleTextField(fields, "reason")
	action.NextStep = parseFlexibleTextField(fields, "next_step")
	action.ExpectedEffect = parseFlexibleTextField(fields, "expected_effect")
	action.CompletionCriteria = parseFlexibleTextField(fields, "completion_criteria")
	action.ObjectDetailBrief = parseFlexibleTextField(fields, "object_detail_brief")
	action.UrgencyMarker = parseFlexibleTextField(fields, "urgency_marker")
	if rawPlan, ok := fields["action_plan"]; ok {
		action.ActionPlan = parseFlexibleActionPlan(rawPlan)
	}
	if rawSteps, ok := fields["steps"]; ok {
		action.Steps = parseFlexibleStringList(rawSteps)
	}
	if rawCompletion, ok := fields["completion_conditions"]; ok {
		action.CompletionConditions = parseFlexibleCompletionConditions(rawCompletion)
	}
	if rawSource, ok := fields["source_candidate_ids"]; ok {
		action.SourceCandidateIDs = parseFlexibleStringList(rawSource)
	}
	if rawExcluded, ok := fields["excluded_entity_ids"]; ok {
		action.ExcludedEntityIDs = parseFlexibleStringList(rawExcluded)
	}
	if rawFacts, ok := fields["evidence_fact_ids"]; ok {
		action.EvidenceFactIDs = parseFlexibleStringList(rawFacts)
	}
	if rawCTA, ok := fields["cta_keys"]; ok {
		action.CTAKeys = parseFlexibleStringList(rawCTA)
	}
	if rawComputed, ok := fields["computed_values"]; ok {
		action.ComputedValues = parseFlexibleComputedValues(rawComputed)
	}
	if rawLim, ok := fields["visible_limitations"]; ok {
		action.VisibleLimitations = parseFlexibleStringList(rawLim)
	}
	if rawLim2, ok := fields["limitations"]; ok {
		action.Limitations = parseFlexibleStringList(rawLim2)
	}
	if action.ActionKey == "" && action.OperationalGoal != "" {
		action.ActionKey = action.OperationalGoal
	}
	return action, action.Type != "" || action.OperationalGoal != "" || len(action.SourceCandidateIDs) > 0
}

func parseObjectDetailBatchOutput(raw string) (*ObjectDetailBatchOutput, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("invalid_json: empty object detail response")
	}
	var out ObjectDetailBatchOutput
	if err := json.Unmarshal([]byte(trimmed), &out); err != nil {
		return nil, fmt.Errorf("invalid_json: %w", err)
	}
	return &out, nil
}

func salvageTruncatedComposeOutput(raw string) (*ActionsAIFirstComposeOutput, bool) {
	salvaged, ok := salvageTruncatedActionsAIFirstOutput(raw)
	if !ok || salvaged == nil {
		return nil, false
	}
	out := &ActionsAIFirstComposeOutput{
		SchemaVersion: salvaged.SchemaVersion,
		Focus:         salvaged.Focus,
		Notes:         salvaged.Notes,
	}
	for _, a := range salvaged.Actions {
		out.Actions = append(out.Actions, composeActionFromDraft(a))
	}
	return out, len(out.Actions) > 0
}

func composeActionFromDraft(d AIFirstDraftAction) AIFirstComposeAction {
	steps := make([]string, 0, len(d.ActionPlan))
	for _, s := range d.ActionPlan {
		if instr := strings.TrimSpace(s.Instruction); instr != "" {
			steps = append(steps, instr)
		}
	}
	return AIFirstComposeAction{
		ActionKey:            d.OperationalGoal,
		Type:                 d.Type,
		Category:             d.Category,
		Priority:             d.Priority,
		ScopeType:            d.ScopeType,
		OperationalGoal:      d.OperationalGoal,
		SourceCandidateIDs:   append([]string(nil), d.SourceCandidateIDs...),
		Title:                d.Title,
		TitleTemplate:        d.TitleTemplate,
		Summary:              d.Summary,
		SummaryTemplate:      d.SummaryTemplate,
		Reason:               d.Reason,
		Steps:                steps,
		ActionPlan:           append([]AIFirstActionStep(nil), d.ActionPlan...),
		ExpectedEffect:       d.ExpectedEffect,
		ComputedValues:       append([]AIFirstComputedValue(nil), d.ComputedValues...),
		CompletionCriteria:   d.CompletionCriteria,
		CompletionConditions: append([]AIFirstCompletionCondition(nil), d.CompletionConditions...),
		EvidenceFactIDs:      append([]string(nil), d.EvidenceFactIDs...),
		CTAKeys:              append([]string(nil), d.CTAKeys...),
		NextStep:             d.NextStep,
		VisibleLimitations:   append([]string(nil), d.VisibleLimitations...),
		Limitations:          append([]string(nil), d.Limitations...),
		UrgencyMarker:        d.UrgencyMarker,
	}
}

func composeOutputToLegacyOutput(compose *ActionsAIFirstComposeOutput) *ActionsAIFirstOutput {
	if compose == nil {
		return &ActionsAIFirstOutput{}
	}
	out := &ActionsAIFirstOutput{
		SchemaVersion: compose.SchemaVersion,
		Focus:         compose.Focus,
		Notes:         compose.Notes,
	}
	return out
}

func IsResponseOutputTruncated(resp *AIRawResponse, requestedMax int) bool {
	if resp == nil || requestedMax <= 0 {
		return false
	}
	if resp.OutputTokens >= requestedMax {
		return true
	}
	if len(resp.ResponseJSON) > 0 {
		var meta struct {
			Status            string `json:"status"`
			IncompleteDetails *struct {
				Reason string `json:"reason"`
			} `json:"incomplete_details"`
		}
		if err := json.Unmarshal(resp.ResponseJSON, &meta); err == nil {
			if meta.Status == "incomplete" {
				return true
			}
			if meta.IncompleteDetails != nil && strings.TrimSpace(meta.IncompleteDetails.Reason) != "" {
				return true
			}
		}
	}
	return false
}
