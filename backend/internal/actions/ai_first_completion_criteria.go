package actions

import (
	"fmt"
	"math"
	"strings"
)

const (
	completionOpEQ               = "eq"
	completionOpNE               = "ne"
	completionOpLT               = "lt"
	completionOpLTE              = "lte"
	completionOpGT               = "gt"
	completionOpGTE              = "gte"
	completionOpBetween          = "between"
	completionOpDecisionRecorded = "decision_recorded"
)

var allowedCompletionOperators = map[string]bool{
	completionOpEQ: true, completionOpNE: true, completionOpLT: true, completionOpLTE: true,
	completionOpGT: true, completionOpGTE: true, completionOpBetween: true,
	completionOpDecisionRecorded: true,
}

var allowedCompletionScopes = map[string]bool{
	"per_entity": true, "all_entities": true, "per_sku": true, "per_campaign": true,
	"per_category": true, "account": true, "action": true,
}

var vagueCompletionCriteriaPhrases = []string{
	"до приемлемого уровня",
	"до приемлемого",
	"до оптимального значения",
	"до оптимального",
	"оптимизирована",
	"оптимизирован",
	"кампания оптимизирована",
	"улучшены показатели",
	"показатели улучшены",
	"приемлемый уровень",
	"оптимальное значение",
}

// AIFirstCompletionCondition is a machine-readable completion check defined by AI.
// Backend validates structure and grounding only — it never chooses metrics or targets.
type AIFirstCompletionCondition struct {
	MetricCode     string   `json:"metric_code"`
	Operator       string   `json:"operator"`
	TargetValue    *float64 `json:"target_value,omitempty"`
	TargetValueMax *float64 `json:"target_value_max,omitempty"`
	FactID         string   `json:"fact_id,omitempty"`
	Scope          string   `json:"scope"`
	PeriodDays     int      `json:"period_days,omitempty"`
	DisplayText    string   `json:"display_text,omitempty"`
}

// ActionCompletionCondition is the persisted/API completion condition contract.
type ActionCompletionCondition struct {
	MetricCode     string   `json:"metric_code"`
	Operator       string   `json:"operator"`
	TargetValue    *float64 `json:"target_value,omitempty"`
	TargetValueMax *float64 `json:"target_value_max,omitempty"`
	FactID         string   `json:"fact_id,omitempty"`
	Scope          string   `json:"scope"`
	PeriodDays     int      `json:"period_days,omitempty"`
	DisplayText    string   `json:"display_text,omitempty"`
}

// CompletionConditionsNormalizeStats tracks condition-level normalization counters.
type CompletionConditionsNormalizeStats struct {
	ReturnedCount      int
	ValidCount         int
	DroppedCount       int
	FactIDRemovedCount int
}

func normalizeCompletionConditions(conditions []AIFirstCompletionCondition) []AIFirstCompletionCondition {
	normalized, _ := normalizeActionCompletionConditions(ActionsAIFirstInput{}, conditions, 0)
	return normalized
}

func normalizeActionCompletionConditions(
	input ActionsAIFirstInput,
	conditions []AIFirstCompletionCondition,
	actionIdx int,
) ([]AIFirstCompletionCondition, []ValidationIssue) {
	stats := CompletionConditionsNormalizeStats{ReturnedCount: len(conditions)}
	normalized, issues, _ := normalizeActionCompletionConditionsWithStats(input, conditions, actionIdx)
	_ = stats
	return normalized, issues
}

func normalizeActionCompletionConditionsWithStats(
	input ActionsAIFirstInput,
	conditions []AIFirstCompletionCondition,
	actionIdx int,
) ([]AIFirstCompletionCondition, []ValidationIssue, CompletionConditionsNormalizeStats) {
	stats := CompletionConditionsNormalizeStats{ReturnedCount: len(conditions)}
	if len(conditions) == 0 {
		return nil, nil, stats
	}
	var issues []ValidationIssue
	addWarning := func(code, message, field string) {
		i := actionIdx
		issues = append(issues, ValidationIssue{
			Severity: IssueSeverityWarning, Code: code, Message: message,
			ActionIndex: &i, Field: field,
		})
	}

	out := make([]AIFirstCompletionCondition, 0, len(conditions))
	for ci, c := range conditions {
		field := fmt.Sprintf("completion_conditions[%d]", ci)
		normalized := AIFirstCompletionCondition{
			MetricCode:  strings.TrimSpace(c.MetricCode),
			Operator:    strings.ToLower(strings.TrimSpace(c.Operator)),
			FactID:      strings.TrimSpace(c.FactID),
			Scope:       strings.ToLower(strings.TrimSpace(c.Scope)),
			PeriodDays:  c.PeriodDays,
			DisplayText: strings.TrimSpace(c.DisplayText),
		}
		if c.TargetValue != nil {
			v := *c.TargetValue
			normalized.TargetValue = &v
		}
		if c.TargetValueMax != nil {
			v := *c.TargetValueMax
			normalized.TargetValueMax = &v
		}

		if !isCompletionConditionStructurallyValid(input, normalized) {
			addWarning("invalid_completion_condition_dropped", "completion condition removed after normalization", field)
			stats.DroppedCount++
			continue
		}

		if factID := strings.TrimSpace(normalized.FactID); factID != "" && !input.factExists(factID) {
			normalized.FactID = ""
			addWarning("unknown_completion_fact_id_removed", fmt.Sprintf("unknown fact_id %q removed from completion condition", factID), field+".fact_id")
			stats.FactIDRemovedCount++
			if !isCompletionConditionStructurallyValid(input, normalized) {
				addWarning("invalid_completion_condition_dropped", "completion condition removed after fact_id removal", field)
				stats.DroppedCount++
				continue
			}
		}

		out = append(out, normalized)
		stats.ValidCount++
	}

	if len(out) == 0 && stats.ReturnedCount > 0 {
		addWarning("completion_conditions_empty_after_normalization", "all completion_conditions removed during normalization", "completion_conditions")
		addWarning("legacy_or_text_only_completion_criteria", "use completion_criteria display text when structured conditions are unavailable", "completion_criteria")
	}
	if len(out) == 0 {
		return nil, issues, stats
	}
	return out, issues, stats
}

func isCompletionConditionStructurallyValid(input ActionsAIFirstInput, cond AIFirstCompletionCondition) bool {
	op := strings.ToLower(strings.TrimSpace(cond.Operator))
	scope := strings.ToLower(strings.TrimSpace(cond.Scope))
	metricCode := strings.TrimSpace(cond.MetricCode)
	if op == "" || !allowedCompletionOperators[op] {
		return false
	}
	if scope == "" || !allowedCompletionScopes[scope] {
		return false
	}
	if cond.PeriodDays < 0 {
		return false
	}
	switch op {
	case completionOpDecisionRecorded:
		return metricCode != "" || strings.TrimSpace(cond.DisplayText) != ""
	case completionOpBetween:
		if metricCode == "" || cond.TargetValue == nil || cond.TargetValueMax == nil {
			return false
		}
		return *cond.TargetValue <= *cond.TargetValueMax
	default:
		return metricCode != "" && cond.TargetValue != nil
	}
}

func validateCompletionConditionsStructure(
	input ActionsAIFirstInput,
	action AIFirstDraftAction,
	idx int,
) []ValidationIssue {
	var issues []ValidationIssue
	addWarning := func(code, message, field string) {
		i := idx
		issues = append(issues, ValidationIssue{
			Severity: IssueSeverityWarning, Code: code, Message: message,
			ActionIndex: &i, Field: field,
		})
	}

	structured := isStructuredAIFirstAction(action)
	if structured && len(action.CompletionConditions) == 0 {
		addWarning("missing_completion_conditions", "structured actions should include completion_conditions with verifiable checks", "completion_conditions")
	}

	for ci, cond := range action.CompletionConditions {
		field := fmt.Sprintf("completion_conditions[%d]", ci)
		metricCode := strings.TrimSpace(cond.MetricCode)
		if cond.PeriodDays == 0 && needsPeriodDaysHint(metricCode) {
			addWarning("missing_completion_period_days", fmt.Sprintf("period_days recommended for metric_code %q", metricCode), field+".period_days")
		}
	}

	display := strings.TrimSpace(action.CompletionCriteria)
	if display != "" {
		for _, phrase := range findVagueCompletionPhrases(display) {
			addWarning("vague_completion_criteria_phrase", fmt.Sprintf("completion_criteria contains unverifiable phrase %q", phrase), "completion_criteria")
		}
	}

	if structured && len(action.CompletionConditions) > 0 && display != "" {
		if !completionDisplayMatchesConditions(display, action.CompletionConditions) {
			addWarning("completion_display_conditions_mismatch", "completion_criteria text may not reflect structured completion_conditions", "completion_criteria")
		}
	}

	if !structured && len(action.CompletionConditions) == 0 && display != "" {
		addWarning("legacy_completion_criteria_only", "completion_criteria is text-only without structured completion_conditions", "completion_criteria")
	}

	return issues
}

func isStructuredAIFirstAction(action AIFirstDraftAction) bool {
	return strings.TrimSpace(action.TitleTemplate) != "" || strings.TrimSpace(action.OperationalGoal) != ""
}

func needsPeriodDaysHint(metricCode string) bool {
	lower := strings.ToLower(strings.TrimSpace(metricCode))
	return strings.Contains(lower, "orders") ||
		strings.Contains(lower, "revenue") ||
		strings.Contains(lower, "spend") ||
		strings.Contains(lower, "last_7d") ||
		strings.Contains(lower, "7d")
}

func findVagueCompletionPhrases(text string) []string {
	lower := strings.ToLower(text)
	var found []string
	for _, phrase := range vagueCompletionCriteriaPhrases {
		if strings.Contains(lower, phrase) {
			found = append(found, phrase)
		}
	}
	return found
}

func completionDisplayMatchesConditions(display string, conditions []AIFirstCompletionCondition) bool {
	lower := strings.ToLower(display)
	for _, cond := range conditions {
		if cond.TargetValue != nil {
			targetStr := formatCompletionTarget(*cond.TargetValue)
			if strings.Contains(display, targetStr) || strings.Contains(lower, strings.ToLower(targetStr)) {
				return true
			}
		}
		if code := strings.ToLower(strings.TrimSpace(cond.MetricCode)); code != "" && strings.Contains(lower, code) {
			return true
		}
		if strings.TrimSpace(cond.DisplayText) != "" && strings.Contains(lower, strings.ToLower(cond.DisplayText)) {
			return true
		}
	}
	// decision_recorded without numbers is still valid if no vague-only phrases flagged separately
	for _, cond := range conditions {
		if strings.ToLower(strings.TrimSpace(cond.Operator)) == completionOpDecisionRecorded {
			return true
		}
	}
	return false
}

func formatCompletionTarget(v float64) string {
	if math.Mod(v, 1) == 0 {
		return fmt.Sprintf("%.0f", v)
	}
	return fmt.Sprintf("%g", v)
}

func mapCompletionConditionsToAPI(conditions []AIFirstCompletionCondition) []ActionCompletionCondition {
	if len(conditions) == 0 {
		return nil
	}
	out := make([]ActionCompletionCondition, 0, len(conditions))
	for _, c := range conditions {
		item := ActionCompletionCondition{
			MetricCode:  c.MetricCode,
			Operator:    c.Operator,
			FactID:      c.FactID,
			Scope:       c.Scope,
			PeriodDays:  c.PeriodDays,
			DisplayText: c.DisplayText,
		}
		if c.TargetValue != nil {
			v := *c.TargetValue
			item.TargetValue = &v
		}
		if c.TargetValueMax != nil {
			v := *c.TargetValueMax
			item.TargetValueMax = &v
		}
		out = append(out, item)
	}
	return out
}

func completionOperatorLabel(op string) string {
	switch strings.ToLower(strings.TrimSpace(op)) {
	case completionOpEQ:
		return "="
	case completionOpNE:
		return "≠"
	case completionOpLT:
		return "<"
	case completionOpLTE:
		return "≤"
	case completionOpGT:
		return ">"
	case completionOpGTE:
		return "≥"
	case completionOpBetween:
		return "между"
	case completionOpDecisionRecorded:
		return "решение зафиксировано"
	default:
		return op
	}
}

func formatCompletionConditionSummary(c ActionCompletionCondition) string {
	if strings.TrimSpace(c.DisplayText) != "" {
		return c.DisplayText
	}
	metric := strings.TrimSpace(c.MetricCode)
	op := completionOperatorLabel(c.Operator)
	switch strings.ToLower(strings.TrimSpace(c.Operator)) {
	case completionOpBetween:
		if c.TargetValue != nil && c.TargetValueMax != nil {
			return fmt.Sprintf("%s %s %s–%s (%s)", metric, op, formatCompletionTarget(*c.TargetValue), formatCompletionTarget(*c.TargetValueMax), c.Scope)
		}
	case completionOpDecisionRecorded:
		if metric != "" {
			return fmt.Sprintf("%s: %s (%s)", metric, op, c.Scope)
		}
		return fmt.Sprintf("%s (%s)", op, c.Scope)
	default:
		if c.TargetValue != nil {
			summary := fmt.Sprintf("%s %s %s (%s)", metric, op, formatCompletionTarget(*c.TargetValue), c.Scope)
			if c.PeriodDays > 0 {
				summary += fmt.Sprintf(", %d дн.", c.PeriodDays)
			}
			return summary
		}
	}
	return metric
}
