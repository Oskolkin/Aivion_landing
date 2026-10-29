package actions

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
)

const (
	objectCountPlaceholder   = "{object_count}"
	computedPlaceholderRegex = `\{computed:([a-zA-Z0-9_]+)\}`
)

// NormalizeActionsAIFirstV2Output performs technical normalization on AI output.
// It never makes analytical decisions — only ID validation, dedup, counts,
// template substitution, AI-declared arithmetic and fact hydration.
func NormalizeActionsAIFirstV2Output(input ActionsAIFirstInput, output ActionsAIFirstOutput) (ActionsAIFirstOutput, AIFirstNormalizationResult) {
	result := AIFirstNormalizationResult{
		Actions: make([]AIFirstDraftAction, 0, len(output.Actions)),
	}
	if !input.IsV2CandidateInput() {
		result.Actions = append(result.Actions, output.Actions...)
		return output, result
	}

	candidateByID := buildCandidateIndex(input.ActionCandidates)
	normalized := make([]AIFirstDraftAction, 0, len(output.Actions))
	result.ReturnedActionCount = len(output.Actions)

	for i, action := range output.Actions {
		result.CompletionConditionsReturnedCount += len(action.CompletionConditions)
		norm, issues := normalizeOneAIFirstAction(input, action, i, candidateByID)
		result.Issues = append(result.Issues, issues...)
		for _, issue := range issues {
			switch issue.Code {
			case "invalid_completion_condition_dropped":
				result.CompletionConditionsDroppedCount++
			case "unknown_completion_fact_id_removed":
				result.CompletionConditionsFactIDRemovedCount++
			}
		}
		result.CompletionConditionsValidCount += len(norm.CompletionConditions)
		if len(action.CompletionConditions) > 0 && len(norm.CompletionConditions) == 0 {
			result.CompletionConditionsEmptyActionsCount++
		}
		normalized = append(normalized, norm)
	}

	normalized, dedupIssues := deduplicateActionsByOperationalGoal(normalized)
	result.Issues = append(result.Issues, dedupIssues...)

	// Drop actions that became empty after technical dedup.
	filtered := make([]AIFirstDraftAction, 0, len(normalized))
	for i, action := range normalized {
		if len(action.AffectedEntities) == 0 {
			result.DroppedActionCount++
			idx := i
			result.Issues = append(result.Issues, ValidationIssue{
				Severity:    IssueSeverityWarning,
				Code:        "duplicate_scope_after_ai_order_normalization",
				Message:     "action removed because all entities were deduplicated into an earlier action with the same operational_goal",
				ActionIndex: &idx,
			})
			continue
		}
		objectCount := len(action.AffectedEntities)
		computedMap, computeIssues := executeComputedValues(input, action.ComputedValues, i)
		result.Issues = append(result.Issues, computeIssues...)
		action = renderActionTemplates(action, objectCount, computedMap)
		action.AffectedObjectsTotal = objectCount
		action.AffectedObjectsDisplayed = len(action.ObjectActions)
		if objectCount > 0 {
			action.AffectedObjectsTruncated = action.AffectedObjectsDisplayed < objectCount
		}
		filtered = append(filtered, action)
	}

	output.Actions = filtered
	result.Actions = append(result.Actions, filtered...)
	return output, result
}

func normalizeOneAIFirstAction(
	input ActionsAIFirstInput,
	action AIFirstDraftAction,
	idx int,
	candidateByID map[string]AIFirstActionCandidate,
) (AIFirstDraftAction, []ValidationIssue) {
	var issues []ValidationIssue
	addWarning := func(code, message, field string) {
		i := idx
		issues = append(issues, ValidationIssue{
			Severity: IssueSeverityWarning, Code: code, Message: message,
			ActionIndex: &i, Field: field,
		})
	}

	action.SourceCandidateIDs = normalizeSourceCandidateIDs(action.SourceCandidateIDs)
	validCandidates := make([]string, 0, len(action.SourceCandidateIDs))
	for _, sourceID := range action.SourceCandidateIDs {
		if _, ok := candidateByID[sourceID]; ok {
			validCandidates = append(validCandidates, sourceID)
		}
	}
	rawSourceIDs := append([]string(nil), action.SourceCandidateIDs...)
	evidence := deriveAndApplyCanonicalActionEvidence(
		input, action.EvidenceFactIDs, rawSourceIDs, validCandidates, nil,
		idx, "", candidateByID,
	)
	issues = append(issues, evidence.Issues...)
	if evidence.Dropped {
		return action, issues
	}
	action.EvidenceFactIDs = append([]string(nil), evidence.Canonical...)

	action.AffectedEntities = normalizeActionAffectedEntities(input, action, candidateByID, idx, &issues)

	if wantCategory, ok := ActionCategoryForType(action.Type); ok && action.Category != wantCategory {
		addWarning(
			"category_taxonomy_normalized",
			fmt.Sprintf("category normalized from %q to %q for type %q", action.Category, wantCategory, action.Type),
			"category",
		)
		action.Category = wantCategory
	}

	objectCount := len(action.AffectedEntities)

	computedMap, computeIssues := executeComputedValues(input, action.ComputedValues, idx)
	issues = append(issues, computeIssues...)

	action = renderActionTemplates(action, objectCount, computedMap)

	action.ObjectActions = hydrateObjectActionMetrics(input, action.ObjectActions, idx, &issues)

	normalizedConditions, condIssues, _ := normalizeActionCompletionConditionsWithStats(input, action.CompletionConditions, idx)
	issues = append(issues, condIssues...)
	action.CompletionConditions = normalizedConditions

	action.AffectedObjectsTotal = objectCount
	action.AffectedObjectsDisplayed = len(action.ObjectActions)
	if objectCount > 0 {
		action.AffectedObjectsTruncated = action.AffectedObjectsDisplayed < objectCount
	} else {
		action.AffectedObjectsTruncated = false
	}

	if action.AffectedObjectsDisplayed == 0 && len(action.ObjectActions) > 0 {
		action.AffectedObjectsDisplayed = len(action.ObjectActions)
		action.AffectedObjectsTruncated = action.AffectedObjectsDisplayed < objectCount
	}

	return action, issues
}

func normalizeActionAffectedEntities(
	input ActionsAIFirstInput,
	action AIFirstDraftAction,
	candidateByID map[string]AIFirstActionCandidate,
	idx int,
	issues *[]ValidationIssue,
) []AIFirstAffectedEntityRef {
	allowed := unionEntitiesFromSourceCandidates(action.SourceCandidateIDs, candidateByID)
	allowedSet := make(map[string]AIFirstAffectedEntityRef, len(allowed))
	for _, ref := range allowed {
		allowedSet[entityRefKey(ref)] = ref
	}

	out := make([]AIFirstAffectedEntityRef, 0, len(allowed))
	seen := map[string]bool{}

	addRef := func(ref AIFirstAffectedEntityRef, fromCandidate bool) {
		key := entityRefKey(ref)
		if seen[key] {
			return
		}
		if _, ok := allowedSet[key]; !ok {
			if !fromCandidate {
				i := idx
				*issues = append(*issues, ValidationIssue{
					Severity: IssueSeverityWarning, Code: "entity_outside_candidate_scope_removed",
					Message:     fmt.Sprintf("entity %s removed because it is outside AI source candidates", ref.EntityID),
					ActionIndex: &i, Field: "affected_entities",
				})
			}
			return
		}
		seen[key] = true
		if canonical, ok := allowedSet[key]; ok {
			if ref.Name == "" {
				ref.Name = canonical.Name
			}
			if len(ref.FactIDs) == 0 {
				ref.FactIDs = append([]string(nil), canonical.FactIDs...)
			}
		}
		out = append(out, ref)
	}

	for _, ref := range action.AffectedEntities {
		if normalized, ok := normalizeAffectedEntityRef(ref); ok {
			addRef(normalized, false)
		}
	}

	for _, ref := range allowed {
		key := entityRefKey(ref)
		if !seen[key] {
			i := idx
			*issues = append(*issues, ValidationIssue{
				Severity: IssueSeverityWarning, Code: "entity_restored_from_candidate_scope",
				Message:     fmt.Sprintf("entity %s restored from AI source candidate union", ref.EntityID),
				ActionIndex: &i, Field: "affected_entities",
			})
			addRef(ref, true)
		}
	}

	if len(out) == 0 {
		return nil
	}
	return out
}

func unionEntitiesFromSourceCandidates(sourceIDs []string, candidateByID map[string]AIFirstActionCandidate) []AIFirstAffectedEntityRef {
	seen := map[string]bool{}
	out := make([]AIFirstAffectedEntityRef, 0)
	for _, sourceID := range sourceIDs {
		c, ok := candidateByID[strings.TrimSpace(sourceID)]
		if !ok {
			continue
		}
		for _, ref := range c.AffectedEntities {
			if normalized, ok := normalizeAffectedEntityRef(ref); ok {
				key := entityRefKey(normalized)
				if seen[key] {
					continue
				}
				seen[key] = true
				out = append(out, normalized)
			}
		}
	}
	return out
}

func entityRefKey(ref AIFirstAffectedEntityRef) string {
	return strings.TrimSpace(ref.EntityType) + "|" + strings.TrimSpace(ref.EntityID)
}

func renderActionTemplates(action AIFirstDraftAction, objectCount int, computed map[string]string) AIFirstDraftAction {
	countStr := strconv.Itoa(objectCount)
	if tmpl := strings.TrimSpace(action.TitleTemplate); tmpl != "" {
		action.Title = substituteTemplatePlaceholders(tmpl, countStr, computed)
	} else if strings.Contains(action.Title, objectCountPlaceholder) {
		action.Title = strings.ReplaceAll(action.Title, objectCountPlaceholder, countStr)
	}

	if tmpl := strings.TrimSpace(action.SummaryTemplate); tmpl != "" {
		action.Summary = substituteTemplatePlaceholders(tmpl, countStr, computed)
	} else if strings.Contains(action.Summary, objectCountPlaceholder) {
		action.Summary = strings.ReplaceAll(action.Summary, objectCountPlaceholder, countStr)
	}
	if action.Summary != "" {
		action.Summary = substituteComputedPlaceholders(action.Summary, computed)
	}
	if action.CompletionCriteria != "" {
		action.CompletionCriteria = substituteTemplatePlaceholders(action.CompletionCriteria, countStr, computed)
	}
	return action
}

func substituteTemplatePlaceholders(text, countStr string, computed map[string]string) string {
	text = strings.ReplaceAll(text, objectCountPlaceholder, countStr)
	return substituteComputedPlaceholders(text, computed)
}

var computedPlaceholderPattern = regexp.MustCompile(computedPlaceholderRegex)

func substituteComputedPlaceholders(text string, computed map[string]string) string {
	return computedPlaceholderPattern.ReplaceAllStringFunc(text, func(match string) string {
		sub := computedPlaceholderPattern.FindStringSubmatch(match)
		if len(sub) != 2 {
			return match
		}
		if v, ok := computed[sub[1]]; ok {
			return v
		}
		return match
	})
}

func executeComputedValues(input ActionsAIFirstInput, values []AIFirstComputedValue, actionIdx int) (map[string]string, []ValidationIssue) {
	out := map[string]string{}
	var issues []ValidationIssue
	for _, cv := range values {
		key := strings.TrimSpace(cv.Key)
		if key == "" {
			continue
		}
		formatted, unit, err := computeAIFirstValue(input, cv)
		if err != nil {
			i := actionIdx
			issues = append(issues, ValidationIssue{
				Severity: IssueSeverityWarning, Code: "computed_value_failed",
				Message: err.Error(), ActionIndex: &i, Field: "computed_values",
			})
			continue
		}
		_ = unit
		out[key] = formatted
	}
	return out, issues
}

func computeAIFirstValue(input ActionsAIFirstInput, cv AIFirstComputedValue) (string, string, error) {
	op := strings.ToLower(strings.TrimSpace(cv.Op))
	if op == "" {
		return "", "", fmt.Errorf("computed value %q: missing op", cv.Key)
	}
	factIDs := compactActionStringIDs(cv.FactIDs)
	if len(factIDs) == 0 {
		return "", "", fmt.Errorf("computed value %q: no fact_ids", cv.Key)
	}

	facts := make([]aidatapack.Fact, 0, len(factIDs))
	unit := ""
	for _, factID := range factIDs {
		fact, ok := input.factByID(factID)
		if !ok {
			return "", "", fmt.Errorf("computed value %q: unknown fact_id %q", cv.Key, factID)
		}
		if unit == "" {
			unit = strings.TrimSpace(fact.Unit)
		} else if u := strings.TrimSpace(fact.Unit); u != "" && u != unit {
			return "", "", fmt.Errorf("computed value %q: incompatible units for op %s", cv.Key, op)
		}
		facts = append(facts, fact)
	}

	switch op {
	case "count":
		return strconv.Itoa(len(facts)), unit, nil
	case "sum":
		var sum float64
		for _, f := range facts {
			v, ok := factToFloat64(f.Value)
			if !ok {
				return "", "", fmt.Errorf("computed value %q: non-numeric fact %q", cv.Key, f.FactID)
			}
			sum += v
		}
		return formatComputedValue(op, sum, unit), unit, nil
	case "min", "max":
		var bound float64
		set := false
		for _, f := range facts {
			v, ok := factToFloat64(f.Value)
			if !ok {
				return "", "", fmt.Errorf("computed value %q: non-numeric fact %q", cv.Key, f.FactID)
			}
			if !set {
				bound = v
				set = true
				continue
			}
			if op == "min" && v < bound {
				bound = v
			}
			if op == "max" && v > bound {
				bound = v
			}
		}
		return formatComputedValue(op, bound, unit), unit, nil
	case "range":
		var minV, maxV float64
		set := false
		for _, f := range facts {
			v, ok := factToFloat64(f.Value)
			if !ok {
				return "", "", fmt.Errorf("computed value %q: non-numeric fact %q", cv.Key, f.FactID)
			}
			if !set {
				minV, maxV = v, v
				set = true
				continue
			}
			if v < minV {
				minV = v
			}
			if v > maxV {
				maxV = v
			}
		}
		if math.Abs(minV-maxV) < 0.0001 {
			return formatComputedValue("max", maxV, unit), unit, nil
		}
		return formatComputedValue("min", minV, unit) + "–" + formatComputedValue("max", maxV, unit), unit, nil
	default:
		return "", "", fmt.Errorf("computed value %q: unsupported op %q", cv.Key, op)
	}
}

func hydrateObjectActionMetrics(
	input ActionsAIFirstInput,
	objects []AIFirstObjectAction,
	actionIdx int,
	issues *[]ValidationIssue,
) []AIFirstObjectAction {
	if len(objects) == 0 {
		return objects
	}
	out := make([]AIFirstObjectAction, 0, len(objects))
	for oi, obj := range objects {
		obj.Metrics = hydrateObjectMetrics(input, obj.Metrics, actionIdx, oi, issues)
		out = append(out, obj)
	}
	return out
}

func hydrateObjectMetrics(
	input ActionsAIFirstInput,
	metrics []AIFirstObjectMetric,
	actionIdx, objectIdx int,
	issues *[]ValidationIssue,
) []AIFirstObjectMetric {
	if len(metrics) == 0 {
		return metrics
	}
	out := make([]AIFirstObjectMetric, 0, len(metrics))
	for mi, m := range metrics {
		factID := strings.TrimSpace(m.FactID)
		if factID == "" {
			// Legacy path: preserve AI label/value when no fact_id.
			if strings.TrimSpace(m.Label) != "" && strings.TrimSpace(m.Value) != "" {
				hydrated := m
				if hydrated.DisplayLabel == "" {
					hydrated.DisplayLabel = m.Label
				}
				if hydrated.FormattedValue == "" {
					hydrated.FormattedValue = m.Value
					if strings.TrimSpace(m.Unit) != "" {
						hydrated.FormattedValue = strings.TrimSpace(m.Value + " " + m.Unit)
					}
				}
				out = append(out, hydrated)
			}
			continue
		}
		fact, ok := input.factByID(factID)
		if !ok {
			i, oi := actionIdx, objectIdx
			_ = mi
			*issues = append(*issues, ValidationIssue{
				Severity: IssueSeverityWarning, Code: "unknown_object_metric_fact_id",
				Message:     fmt.Sprintf("unknown fact_id %q in object metric", factID),
				ActionIndex: &i, Field: fmt.Sprintf("object_actions[%d].metrics", oi),
			})
			continue
		}
		code := strings.TrimSpace(m.MetricCode)
		if code == "" {
			code = metricCodeFromFact(fact)
		}
		display := metricDisplayLabel(fact, code)
		formatted := formatAIFirstFactValue(fact)
		out = append(out, AIFirstObjectMetric{
			FactID:         factID,
			MetricCode:     code,
			DisplayLabel:   display,
			FormattedValue: formatted,
			Label:          display,
			Value:          formatted,
			Unit:           fact.Unit,
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func deduplicateActionsByOperationalGoal(actions []AIFirstDraftAction) ([]AIFirstDraftAction, []ValidationIssue) {
	type entityGoalKey struct {
		entityID string
		goal     string
	}
	seen := map[entityGoalKey]int{}
	var issues []ValidationIssue

	out := make([]AIFirstDraftAction, len(actions))
	copy(out, actions)

	for actionIdx := range out {
		goal := strings.TrimSpace(out[actionIdx].OperationalGoal)
		if goal == "" {
			continue
		}
		kept := make([]AIFirstAffectedEntityRef, 0, len(out[actionIdx].AffectedEntities))
		for _, ent := range out[actionIdx].AffectedEntities {
			entityID := strings.TrimSpace(ent.EntityID)
			if entityID == "" {
				continue
			}
			key := entityGoalKey{entityID: entityID, goal: goal}
			if firstIdx, ok := seen[key]; ok && firstIdx != actionIdx {
				i := actionIdx
				issues = append(issues, ValidationIssue{
					Severity:    IssueSeverityWarning,
					Code:        "duplicate_entity_operational_goal_removed",
					Message:     fmt.Sprintf("entity %q removed from action %d; kept in earlier action %d for operational_goal %q", entityID, actionIdx, firstIdx, goal),
					ActionIndex: &i, Field: "affected_entities",
				})
				out[actionIdx].ObjectActions = removeObjectActionForEntity(out[actionIdx].ObjectActions, ent.EntityType, entityID)
				continue
			}
			if _, ok := seen[key]; !ok {
				seen[key] = actionIdx
			}
			kept = append(kept, ent)
		}
		if len(kept) != len(out[actionIdx].AffectedEntities) {
			out[actionIdx].AffectedEntities = kept
			out[actionIdx].AffectedObjectsTotal = len(kept)
			out[actionIdx].AffectedObjectsDisplayed = len(out[actionIdx].ObjectActions)
			out[actionIdx].AffectedObjectsTruncated = out[actionIdx].AffectedObjectsDisplayed > 0 &&
				out[actionIdx].AffectedObjectsDisplayed < out[actionIdx].AffectedObjectsTotal
		} else {
			out[actionIdx].AffectedEntities = kept
		}
	}
	return out, issues
}

func removeObjectActionForEntity(objects []AIFirstObjectAction, entityType, entityID string) []AIFirstObjectAction {
	out := make([]AIFirstObjectAction, 0, len(objects))
	for _, obj := range objects {
		if strings.TrimSpace(obj.EntityType) == strings.TrimSpace(entityType) &&
			strings.TrimSpace(obj.EntityID) == strings.TrimSpace(entityID) {
			continue
		}
		out = append(out, obj)
	}
	return out
}

// BuildActionsCandidateCoverageDiagnostics records factual candidate reference tracing.
// Aggregate counts are derived from per-candidate dispositions (single source of truth).
func BuildActionsCandidateCoverageDiagnostics(
	input ActionsAIFirstInput,
	returned []AIFirstDraftAction,
	accepted []AIFirstDraftAction,
	dropped []PilotDroppedAction,
	persistedKeys map[string]bool,
) map[string]any {
	if !input.IsV2CandidateInput() {
		return nil
	}

	referencedReturned := map[string]bool{}
	referencedPersisted := map[string]bool{}
	duplicateReference := 0

	collectIDs := func(action AIFirstDraftAction, into map[string]bool) {
		for _, id := range normalizeSourceCandidateIDs(action.SourceCandidateIDs) {
			into[id] = true
		}
	}
	for _, action := range returned {
		collectIDs(action, referencedReturned)
	}
	for _, action := range accepted {
		for _, id := range normalizeSourceCandidateIDs(action.SourceCandidateIDs) {
			if persistedKeys == nil || persistedKeys[id] {
				referencedPersisted[id] = true
			}
		}
	}

	seenPerAction := map[string]int{}
	for _, action := range returned {
		seen := map[string]bool{}
		for _, id := range normalizeSourceCandidateIDs(action.SourceCandidateIDs) {
			if seen[id] {
				duplicateReference++
			}
			seen[id] = true
			seenPerAction[id]++
		}
	}

	total := len(input.ActionCandidates)
	entries := make([]map[string]any, 0, total)
	var persistedCount, droppedOnlyCount, notReferencedCount int

	for _, c := range input.ActionCandidates {
		id := strings.TrimSpace(c.CandidateID)
		disposition := "not_referenced_by_ai"
		var actionIndices []int
		switch {
		case referencedPersisted[id]:
			disposition = "referenced_in_persisted_action"
			persistedCount++
		case referencedReturned[id]:
			disposition = "referenced_in_dropped_action_only"
			droppedOnlyCount++
		default:
			notReferencedCount++
		}
		for i, action := range returned {
			for _, sourceID := range normalizeSourceCandidateIDs(action.SourceCandidateIDs) {
				if sourceID == id {
					actionIndices = append(actionIndices, i)
				}
			}
		}
		entries = append(entries, map[string]any{
			"candidate_id":              id,
			"disposition":               disposition,
			"referenced_action_indices": actionIndices,
		})
	}

	referencedReturnedCount := len(referencedReturned)
	invariantMismatch := persistedCount+droppedOnlyCount+notReferencedCount != total

	returnedCoverage := 0.0
	persistedCoverage := 0.0
	if total > 0 {
		returnedCoverage = float64(referencedReturnedCount) / float64(total) * 100
		persistedCoverage = float64(persistedCount) / float64(total) * 100
	}

	result := map[string]any{
		"candidates":                                      entries,
		"actions_candidate_total_count":                   total,
		"actions_candidate_referenced_returned_count":     referencedReturnedCount,
		"actions_candidate_referenced_persisted_count":    persistedCount,
		"actions_candidate_referenced_dropped_only_count": droppedOnlyCount,
		"actions_candidate_not_referenced_by_ai_count":    notReferencedCount,
		"actions_candidate_duplicate_reference_count":     duplicateReference,
		"actions_candidate_returned_coverage_pct":         returnedCoverage,
		"actions_candidate_persisted_coverage_pct":        persistedCoverage,
	}
	if invariantMismatch {
		result["candidate_coverage_invariant_mismatch"] = true
		result["candidate_coverage_invariant_expected_total"] = total
		result["candidate_coverage_invariant_actual_total"] = persistedCount + droppedOnlyCount + notReferencedCount
	}
	return result
}
