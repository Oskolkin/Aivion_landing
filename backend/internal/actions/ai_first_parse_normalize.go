package actions

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func parseActionsAIFirstOutput(raw string) (*ActionsAIFirstOutput, error) {
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
		if salvaged, ok := salvageTruncatedActionsAIFirstOutput(trimmed); ok {
			return salvaged, nil
		}
		return nil, fmt.Errorf("invalid_json: %w", err)
	}
	out := &ActionsAIFirstOutput{
		SchemaVersion: strings.TrimSpace(wire.SchemaVersion),
		Actions:       parseFlexibleAIFirstActions(wire.Actions),
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

var errEmptyActionsJSON = fmt.Errorf("empty response")

func parseFlexibleAIFirstActions(raw json.RawMessage) []AIFirstDraftAction {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		if action, ok := parseFlexibleAIFirstDraftAction(raw); ok {
			return []AIFirstDraftAction{action}
		}
		return nil
	}
	out := make([]AIFirstDraftAction, 0, len(items))
	for _, item := range items {
		if action, ok := parseFlexibleAIFirstDraftAction(item); ok {
			out = append(out, action)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parseFlexibleAIFirstDraftAction(raw json.RawMessage) (AIFirstDraftAction, bool) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return AIFirstDraftAction{}, false
	}

	var action AIFirstDraftAction
	unmarshalActionsField(fields, "type", &action.Type)
	unmarshalActionsField(fields, "category", &action.Category)
	unmarshalActionsField(fields, "priority", &action.Priority)
	action.OperationalGoal = parseFlexibleTextField(fields, "operational_goal")
	action.Title = parseFlexibleTextField(fields, "title")
	action.TitleTemplate = parseFlexibleTextField(fields, "title_template")
	action.Summary = parseFlexibleTextField(fields, "summary")
	action.SummaryTemplate = parseFlexibleTextField(fields, "summary_template")
	action.Reason = parseFlexibleTextField(fields, "reason")
	action.NextStep = parseFlexibleTextField(fields, "next_step")
	action.ExpectedEffect = parseFlexibleTextField(fields, "expected_effect")
	action.CompletionCriteria = parseFlexibleTextField(fields, "completion_criteria")
	if rawCompletion, ok := fields["completion_conditions"]; ok {
		action.CompletionConditions = parseFlexibleCompletionConditions(rawCompletion)
	}
	action.UrgencyMarker = parseFlexibleTextField(fields, "urgency_marker")
	action.AffectedObjectsSummary = parseFlexibleTextField(fields, "affected_objects_summary")

	if rawPlan, ok := fields["action_plan"]; ok {
		action.ActionPlan = parseFlexibleActionPlan(rawPlan)
	}
	if rawEntities, ok := fields["affected_entities"]; ok {
		action.AffectedEntities = parseFlexibleAffectedEntities(rawEntities)
	}
	if rawObjects, ok := fields["object_actions"]; ok {
		action.ObjectActions = parseFlexibleObjectActions(rawObjects)
	}
	if rawFacts, ok := fields["evidence_fact_ids"]; ok {
		action.EvidenceFactIDs = parseFlexibleStringList(rawFacts)
	}
	if rawCTA, ok := fields["cta_keys"]; ok {
		action.CTAKeys = parseFlexibleStringList(rawCTA)
	}
	if rawSource, ok := fields["source_candidate_ids"]; ok {
		action.SourceCandidateIDs = parseFlexibleStringList(rawSource)
	}
	if rawComputed, ok := fields["computed_values"]; ok {
		action.ComputedValues = parseFlexibleComputedValues(rawComputed)
	}
	if rawTotal, ok := fields["affected_objects_total"]; ok {
		action.AffectedObjectsTotal = parseFlexibleIntField(rawTotal)
	}
	if rawDisplayed, ok := fields["affected_objects_displayed"]; ok {
		action.AffectedObjectsDisplayed = parseFlexibleIntField(rawDisplayed)
	}
	if rawTruncated, ok := fields["affected_objects_truncated"]; ok {
		action.AffectedObjectsTruncated = parseFlexibleBoolField(rawTruncated)
	}
	if rawLimits, ok := fields["limitations"]; ok {
		action.Limitations = parseFlexibleStringList(rawLimits)
	}
	if rawVisible, ok := fields["visible_limitations"]; ok {
		action.VisibleLimitations = parseFlexibleStringList(rawVisible)
	}
	if rawNotes, ok := fields["notes_for_legacy_check"]; ok {
		action.NotesForLegacyCheck = parseFlexibleStringList(rawNotes)
	}
	if rawDetail, ok := fields["operational_detail"]; ok {
		var detail OperationalDetail
		if err := json.Unmarshal(rawDetail, &detail); err == nil {
			action.OperationalDetail = &detail
		}
	}

	if strings.TrimSpace(action.NextStep) == "" && len(action.ActionPlan) > 0 {
		for _, step := range action.ActionPlan {
			if instr := strings.TrimSpace(step.Instruction); instr != "" {
				action.NextStep = instr
				break
			}
		}
	}

	if strings.TrimSpace(action.Title) == "" &&
		strings.TrimSpace(action.TitleTemplate) == "" &&
		strings.TrimSpace(action.Reason) == "" &&
		strings.TrimSpace(action.NextStep) == "" &&
		len(action.ActionPlan) == 0 {
		return AIFirstDraftAction{}, false
	}
	return action, true
}

func parseFlexibleActionPlan(raw json.RawMessage) []AIFirstActionStep {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		text = strings.TrimSpace(text)
		if text == "" {
			return nil
		}
		return []AIFirstActionStep{actionStepFromText(text)}
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err == nil {
		out := make([]AIFirstActionStep, 0, len(items))
		for _, item := range items {
			if step, ok := parseOneFlexibleActionPlanStep(item); ok {
				out = append(out, step)
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	}
	if step, ok := parseOneFlexibleActionPlanStep(raw); ok {
		return []AIFirstActionStep{step}
	}
	return nil
}

func parseOneFlexibleActionPlanStep(raw json.RawMessage) (AIFirstActionStep, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return AIFirstActionStep{}, false
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		text = strings.TrimSpace(text)
		if text == "" {
			return AIFirstActionStep{}, false
		}
		return actionStepFromText(text), true
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return AIFirstActionStep{}, false
	}
	step := AIFirstActionStep{
		Title: parseFlexibleTextField(fields, "title"),
		Instruction: firstNonEmptyText(
			parseFlexibleTextField(fields, "instruction"),
			parseFlexibleTextField(fields, "description"),
			parseFlexibleTextField(fields, "text"),
		),
		ExpectedResult: firstNonEmptyText(
			parseFlexibleTextField(fields, "expected_result"),
			parseFlexibleTextField(fields, "result"),
			parseFlexibleTextField(fields, "outcome"),
		),
	}
	if strings.TrimSpace(step.Title) == "" {
		step.Title = step.Instruction
	}
	if strings.TrimSpace(step.Instruction) == "" {
		step.Instruction = step.Title
	}
	if strings.TrimSpace(step.ExpectedResult) == "" {
		step.ExpectedResult = step.Instruction
	}
	if strings.TrimSpace(step.Instruction) == "" {
		return AIFirstActionStep{}, false
	}
	return step, true
}

func actionStepFromText(text string) AIFirstActionStep {
	text = strings.TrimSpace(text)
	return AIFirstActionStep{
		Title:          text,
		Instruction:    text,
		ExpectedResult: text,
	}
}

func parseFlexibleAffectedEntities(raw json.RawMessage) []AIFirstAffectedEntityRef {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var refs []AIFirstAffectedEntityRef
	if err := json.Unmarshal(raw, &refs); err == nil {
		return normalizeAffectedEntityRefs(refs)
	}
	var one AIFirstAffectedEntityRef
	if err := json.Unmarshal(raw, &one); err == nil {
		if ref, ok := normalizeAffectedEntityRef(one); ok {
			return []AIFirstAffectedEntityRef{ref}
		}
	}
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		return entityRefsFromStringIDs([]string{single})
	}
	var strs []string
	if err := json.Unmarshal(raw, &strs); err == nil {
		return entityRefsFromStringIDs(strs)
	}
	return nil
}

func normalizeAffectedEntityRefs(refs []AIFirstAffectedEntityRef) []AIFirstAffectedEntityRef {
	out := make([]AIFirstAffectedEntityRef, 0, len(refs))
	for _, ref := range refs {
		if normalized, ok := normalizeAffectedEntityRef(ref); ok {
			out = append(out, normalized)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeAffectedEntityRef(ref AIFirstAffectedEntityRef) (AIFirstAffectedEntityRef, bool) {
	entityID := strings.TrimSpace(ref.EntityID)
	if entityID == "" {
		return AIFirstAffectedEntityRef{}, false
	}
	entityType := strings.TrimSpace(ref.EntityType)
	if entityType == "" {
		entityType = inferEntityTypeFromEntityID(entityID)
	}
	if entityType == "" {
		return AIFirstAffectedEntityRef{}, false
	}
	ref.EntityID = entityID
	ref.EntityType = entityType
	ref.FactIDs = compactActionStringIDs(ref.FactIDs)
	return ref, true
}

func entityRefsFromStringIDs(ids []string) []AIFirstAffectedEntityRef {
	out := make([]AIFirstAffectedEntityRef, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		entityType := inferEntityTypeFromEntityID(id)
		if entityType == "" {
			continue
		}
		out = append(out, AIFirstAffectedEntityRef{EntityType: entityType, EntityID: id})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func inferEntityTypeFromEntityID(entityID string) string {
	entityID = strings.TrimSpace(strings.ToLower(entityID))
	switch {
	case strings.HasPrefix(entityID, "sku:"), strings.HasPrefix(entityID, "sku_"):
		return "sku"
	case strings.HasPrefix(entityID, "campaign:"), strings.HasPrefix(entityID, "campaign_"):
		return "campaign"
	case strings.HasPrefix(entityID, "category:"), strings.HasPrefix(entityID, "category_"):
		return "category"
	case strings.HasPrefix(entityID, "warehouse:"), strings.HasPrefix(entityID, "warehouse_"):
		return "warehouse"
	case strings.HasPrefix(entityID, "account:"), strings.HasPrefix(entityID, "account_"):
		return "account"
	case strings.HasPrefix(entityID, "product:"), strings.HasPrefix(entityID, "product_"):
		return "product"
	case strings.HasPrefix(entityID, "funnel:"):
		return "funnel"
	default:
		return ""
	}
}

func parseFlexibleObjectActions(raw json.RawMessage) []AIFirstObjectAction {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		if action, ok := parseOneFlexibleObjectAction(raw); ok {
			return []AIFirstObjectAction{action}
		}
		return nil
	}
	out := make([]AIFirstObjectAction, 0, len(items))
	for _, item := range items {
		if action, ok := parseOneFlexibleObjectAction(item); ok {
			out = append(out, action)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parseOneFlexibleObjectAction(raw json.RawMessage) (AIFirstObjectAction, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return AIFirstObjectAction{}, false
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		var action AIFirstObjectAction
		if err := json.Unmarshal(raw, &action); err == nil {
			action.Metrics = normalizeObjectMetrics(action.Metrics)
			return action, strings.TrimSpace(action.EntityID) != ""
		}
		return AIFirstObjectAction{}, false
	}
	action := AIFirstObjectAction{
		EntityType: parseFlexibleTextField(fields, "entity_type"),
		EntityID:   parseFlexibleTextField(fields, "entity_id"),
		Name:       parseFlexibleTextField(fields, "name"),
		RecommendedAction: firstNonEmptyText(
			parseFlexibleTextField(fields, "recommended_action"),
			parseFlexibleTextField(fields, "action"),
		),
		Reason: parseFlexibleTextField(fields, "reason"),
	}
	if rawMetrics, ok := fields["metrics"]; ok {
		action.Metrics = parseFlexibleObjectMetrics(rawMetrics)
	}
	if rawFacts, ok := fields["evidence_fact_ids"]; ok {
		action.EvidenceFactIDs = parseFlexibleStringList(rawFacts)
	}
	if strings.TrimSpace(action.EntityID) == "" {
		return AIFirstObjectAction{}, false
	}
	if strings.TrimSpace(action.EntityType) == "" {
		action.EntityType = inferEntityTypeFromEntityID(action.EntityID)
	}
	return action, true
}

func parseFlexibleObjectMetrics(raw json.RawMessage) []AIFirstObjectMetric {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var metrics []AIFirstObjectMetric
	if err := json.Unmarshal(raw, &metrics); err == nil {
		return normalizeObjectMetrics(metrics)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err == nil {
		out := make([]AIFirstObjectMetric, 0, len(items))
		for _, item := range items {
			if metric, ok := parseOneFlexibleObjectMetric(item); ok {
				out = append(out, metric)
			}
		}
		return normalizeObjectMetrics(out)
	}
	if metric, ok := parseOneFlexibleObjectMetric(raw); ok {
		return []AIFirstObjectMetric{metric}
	}
	var objectMap map[string]json.RawMessage
	if err := json.Unmarshal(raw, &objectMap); err == nil {
		out := make([]AIFirstObjectMetric, 0, len(objectMap))
		for label, valueRaw := range objectMap {
			value := parseFlexibleTextField(map[string]json.RawMessage{"value": valueRaw}, "value")
			if strings.TrimSpace(label) != "" && strings.TrimSpace(value) != "" {
				out = append(out, AIFirstObjectMetric{Label: label, Value: value})
			}
		}
		return normalizeObjectMetrics(out)
	}
	return nil
}

func parseOneFlexibleObjectMetric(raw json.RawMessage) (AIFirstObjectMetric, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return AIFirstObjectMetric{}, false
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		text = strings.TrimSpace(text)
		if text == "" {
			return AIFirstObjectMetric{}, false
		}
		return AIFirstObjectMetric{Label: "metric", Value: text}, true
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return AIFirstObjectMetric{}, false
	}
	metric := AIFirstObjectMetric{
		FactID: parseFlexibleTextField(fields, "fact_id"),
		MetricCode: firstNonEmptyText(
			parseFlexibleTextField(fields, "metric_code"),
			parseFlexibleTextField(fields, "metric"),
		),
		Label: firstNonEmptyText(
			parseFlexibleTextField(fields, "display_label"),
			parseFlexibleTextField(fields, "label"),
			parseFlexibleTextField(fields, "name"),
		),
		Value: firstNonEmptyText(
			parseFlexibleTextField(fields, "formatted_value"),
			parseFlexibleTextField(fields, "value"),
		),
		Unit:         parseFlexibleTextField(fields, "unit"),
		DisplayLabel: parseFlexibleTextField(fields, "display_label"),
		FormattedValue: firstNonEmptyText(
			parseFlexibleTextField(fields, "formatted_value"),
			parseFlexibleTextField(fields, "value"),
		),
	}
	if strings.TrimSpace(metric.FactID) != "" {
		return metric, true
	}
	if strings.TrimSpace(metric.Label) == "" || strings.TrimSpace(metric.Value) == "" {
		return AIFirstObjectMetric{}, false
	}
	return metric, true
}

func parseFlexibleComputedValues(raw json.RawMessage) []AIFirstComputedValue {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var items []AIFirstComputedValue
	if err := json.Unmarshal(raw, &items); err == nil {
		return items
	}
	return nil
}

func parseFlexibleIntField(raw json.RawMessage) int {
	if len(raw) == 0 || string(raw) == "null" {
		return 0
	}
	var n int
	if err := json.Unmarshal(raw, &n); err == nil {
		return n
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return int(f)
	}
	return 0
}

func parseFlexibleCompletionConditions(raw json.RawMessage) []AIFirstCompletionCondition {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var items []AIFirstCompletionCondition
	if err := json.Unmarshal(raw, &items); err == nil {
		return normalizeCompletionConditions(items)
	}
	return nil
}

func parseFlexibleBoolField(raw json.RawMessage) bool {
	if len(raw) == 0 || string(raw) == "null" {
		return false
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		return b
	}
	return false
}

func normalizeObjectMetrics(metrics []AIFirstObjectMetric) []AIFirstObjectMetric {
	if len(metrics) == 0 {
		return nil
	}
	out := make([]AIFirstObjectMetric, 0, len(metrics))
	for _, m := range metrics {
		if strings.TrimSpace(m.FactID) != "" {
			out = append(out, m)
			continue
		}
		label := strings.TrimSpace(m.Label)
		value := strings.TrimSpace(m.Value)
		if label == "" || value == "" {
			continue
		}
		out = append(out, AIFirstObjectMetric{
			Label: label,
			Value: value,
			Unit:  strings.TrimSpace(m.Unit),
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parseFlexibleAIFirstFocus(raw json.RawMessage) (*ActionsAIFirstFocus, error) {
	var focus ActionsAIFirstFocus
	if err := json.Unmarshal(raw, &focus); err != nil {
		return nil, err
	}
	return &focus, nil
}

func parseFlexibleStringList(raw json.RawMessage) []string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var items []string
	if err := json.Unmarshal(raw, &items); err == nil {
		return compactActionStringIDs(items)
	}
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		single = strings.TrimSpace(single)
		if single == "" {
			return nil
		}
		return []string{single}
	}
	return nil
}

func parseFlexibleTextField(fields map[string]json.RawMessage, key string) string {
	raw, ok := fields[key]
	if !ok || len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return strings.TrimSpace(text)
	}
	var num float64
	if err := json.Unmarshal(raw, &num); err == nil {
		return strconv.FormatFloat(num, 'f', -1, 64)
	}
	return ""
}

func unmarshalActionsField(fields map[string]json.RawMessage, key string, dest any) {
	raw, ok := fields[key]
	if !ok {
		return
	}
	_ = json.Unmarshal(raw, dest)
}

func firstNonEmptyText(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func compactActionStringIDs(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func salvageTruncatedActionsAIFirstOutput(raw string) (*ActionsAIFirstOutput, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, false
	}
	var partial map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &partial); err == nil {
		return nil, false
	}
	actionsIdx := strings.Index(raw, `"actions"`)
	if actionsIdx < 0 {
		return nil, false
	}
	arrayStart := strings.Index(raw[actionsIdx:], "[")
	if arrayStart < 0 {
		return nil, false
	}
	arrayStart += actionsIdx
	decoder := json.NewDecoder(bytes.NewReader([]byte(raw[arrayStart:])))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('[') {
		return nil, false
	}
	actions := make([]AIFirstDraftAction, 0, 8)
	for decoder.More() {
		var item json.RawMessage
		if err := decoder.Decode(&item); err != nil {
			break
		}
		if action, ok := parseFlexibleAIFirstDraftAction(item); ok {
			actions = append(actions, action)
		}
	}
	if len(actions) == 0 {
		return nil, false
	}
	out := &ActionsAIFirstOutput{
		SchemaVersion: ActionsAIFirstSchemaVersionV2,
		Actions:       actions,
	}
	if schemaIdx := strings.Index(raw, `"schema_version"`); schemaIdx >= 0 {
		var wire struct {
			SchemaVersion string `json:"schema_version"`
		}
		_ = json.Unmarshal([]byte(raw), &wire)
		if strings.TrimSpace(wire.SchemaVersion) != "" {
			out.SchemaVersion = strings.TrimSpace(wire.SchemaVersion)
		}
	}
	return out, true
}
