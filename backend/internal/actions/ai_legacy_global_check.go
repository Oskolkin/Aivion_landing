package actions

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ValidateLegacyGlobalCheckOutput validates legacy global check AI output.
//
// Validation is granular: ordered_action_refs, top_action_refs and ai_focus are
// validated independently so that a valid ai_focus is preserved even when the
// reference lists contain problems. Unsupported numbers in ai_focus are recorded
// as warnings (so the focus can still be applied) while forbidden claims keep the
// focus from being used.
func ValidateLegacyGlobalCheckOutput(
	output *LegacyGlobalCheckOutput,
	drafts map[string]ScoredDraftAction,
	opts GenerateActionsOptions,
	forbiddenClaims []string,
	snapshot AccountSnapshot,
) LegacyGlobalCheckValidationResult {
	result := LegacyGlobalCheckValidationResult{
		PriorityOverrides: make(map[string]ActionPriority),
	}
	if output == nil {
		result.Issues = append(result.Issues, ValidationIssue{
			Severity: IssueSeverityFatal,
			Code:     "missing_output",
			Message:  "legacy global check output is nil",
		})
		return result
	}

	available := make([]string, 0, len(drafts))
	for id := range drafts {
		available = append(available, id)
	}
	sort.Strings(available)
	result.AvailableRefs = available

	// ----- ordered_action_refs -----
	seen := make(map[string]bool)
	orderingHasUnknown := false
	for _, ref := range output.OrderedActionRefs {
		if ref.TempActionID == "" {
			orderingHasUnknown = true
			result.Issues = append(result.Issues, ValidationIssue{
				Severity: IssueSeverityRepairable,
				Code:     "missing_temp_action_id",
				Message:  "ordered_action_refs entry missing temp_action_id",
			})
			continue
		}
		if _, ok := drafts[ref.TempActionID]; !ok {
			orderingHasUnknown = true
			result.UnknownOrderedRefs = append(result.UnknownOrderedRefs, ref.TempActionID)
			result.Issues = append(result.Issues, ValidationIssue{
				Severity: IssueSeverityRepairable,
				Code:     "unknown_temp_action_id",
				Message:  fmt.Sprintf("unknown temp_action_id %q", ref.TempActionID),
			})
			continue
		}
		if ref.Priority != "" && isValidPriority(ref.Priority) {
			result.PriorityOverrides[ref.TempActionID] = ref.Priority
		}
		if !seen[ref.TempActionID] {
			result.OrderedTempIDs = append(result.OrderedTempIDs, ref.TempActionID)
			seen[ref.TempActionID] = true
		}
	}
	orderingOverflow := len(result.OrderedTempIDs) > opts.MaxActionsOutput
	if orderingOverflow {
		result.Issues = append(result.Issues, ValidationIssue{
			Severity: IssueSeverityRepairable,
			Code:     "too_many_ordered_actions",
			Message:  fmt.Sprintf("ordered_action_refs count %d exceeds max %d", len(result.OrderedTempIDs), opts.MaxActionsOutput),
		})
	}
	result.OrderingValid = len(result.OrderedTempIDs) > 0 && !orderingHasUnknown && !orderingOverflow

	// ----- top_action_refs -----
	topHasUnknown := false
	for _, id := range output.TopActionRefs {
		if _, ok := drafts[id]; !ok {
			topHasUnknown = true
			result.UnknownTopRefs = append(result.UnknownTopRefs, id)
			result.Issues = append(result.Issues, ValidationIssue{
				Severity: IssueSeverityRepairable,
				Code:     "unknown_top_action_ref",
				Message:  fmt.Sprintf("unknown top_action_ref %q", id),
			})
			continue
		}
		result.TopTempIDs = append(result.TopTempIDs, id)
	}
	topOverflow := len(result.TopTempIDs) > opts.MaxTopActions
	if topOverflow {
		result.Issues = append(result.Issues, ValidationIssue{
			Severity: IssueSeverityRepairable,
			Code:     "too_many_top_actions",
			Message:  fmt.Sprintf("top_action_refs count %d exceeds max %d", len(result.TopTempIDs), opts.MaxTopActions),
		})
	}
	result.TopValid = len(result.TopTempIDs) > 0 && !topHasUnknown && !topOverflow

	// ----- ai_focus -----
	if output.AIFocus != nil {
		allowed := aiFocusAllowedNumbers(drafts, snapshot)
		focusValid := true
		for _, field := range []struct {
			name string
			val  string
		}{
			{"headline", output.AIFocus.Headline},
			{"summary", output.AIFocus.Summary},
		} {
			if pattern, ok := containsForbiddenClaim(field.val, forbiddenClaims); ok {
				// Forbidden claims are unsafe: the focus must not be used as-is.
				focusValid = false
				issue := ValidationIssue{
					Severity: IssueSeverityFatal,
					Code:     "forbidden_ai_focus_claim",
					Message:  fmt.Sprintf("forbidden claim %q in ai_focus.%s", pattern, field.name),
					Field:    "ai_focus." + field.name,
				}
				result.Issues = append(result.Issues, issue)
				result.FocusIssues = append(result.FocusIssues, issue)
			}
			for _, num := range extractNumbers(field.val) {
				if !numberAllowed(num, allowed) {
					// Unsupported numbers are recorded but do NOT void the focus:
					// the narrative stays useful and the reason is observable.
					issue := ValidationIssue{
						Severity: IssueSeverityWarning,
						Code:     "unsupported_ai_focus_number",
						Message:  fmt.Sprintf("unsupported number %q in ai_focus.%s", num, field.name),
						Field:    "ai_focus." + field.name,
					}
					result.Issues = append(result.Issues, issue)
					result.FocusIssues = append(result.FocusIssues, issue)
				}
			}
		}
		result.FocusValid = focusValid
		result.AIFocus = output.AIFocus
	}

	result.Valid = result.OrderingValid && result.TopValid && result.FocusValid && len(result.Issues) == 0
	return result
}

// aiFocusAllowedNumbers collects every number that backend context can support,
// so legitimate figures in ai_focus (including account-level deltas and grouped
// shares from candidate facts/aggregates/examples) are not flagged as unsupported.
func aiFocusAllowedNumbers(drafts map[string]ScoredDraftAction, snapshot AccountSnapshot) map[string]bool {
	allowed := make(map[string]bool)
	addText := func(text string) {
		for _, n := range extractNumbers(text) {
			allowed[n] = true
		}
	}
	addAny := func(v any) {
		for _, n := range numbersFromAny(v) {
			allowed[n] = true
		}
	}

	for _, d := range drafts {
		addText(d.Action.Title + " " + d.Action.Reason + " " + d.Action.Summary + " " + d.Action.NextStep + " " + d.Action.ExpectedEffect)
		for _, c := range d.SourceCandidates {
			addText(c.Summary)
			for _, f := range c.Facts {
				addText(f.Value)
				addText(f.Label)
			}
			for k, v := range c.Aggregates {
				_ = k
				addAny(v)
			}
			for _, ex := range c.TopExamples {
				for _, mv := range ex.Metrics {
					addAny(mv)
				}
			}
			if c.EntityGroupRef != nil {
				for _, sv := range c.EntityGroupRef.Summary {
					addAny(sv)
				}
				allowed[strconv.Itoa(c.EntityGroupRef.Count)] = true
			}
		}
	}

	if snapshot.Revenue != nil {
		addAny(*snapshot.Revenue)
	}
	if snapshot.Orders != nil {
		addAny(*snapshot.Orders)
	}
	if snapshot.AOV != nil {
		addAny(*snapshot.AOV)
	}
	if snapshot.DRR != nil {
		addAny(*snapshot.DRR)
	}
	if snapshot.RevenueDeltaPct != nil {
		addAny(*snapshot.RevenueDeltaPct)
	}
	if snapshot.OrdersDeltaPct != nil {
		addAny(*snapshot.OrdersDeltaPct)
	}
	if snapshot.HealthScore != nil {
		addAny(*snapshot.HealthScore)
	}

	// Numbers appear in ai_focus text without their sign (e.g. "снизилась на 0.11%"
	// for a stored delta of -0.11), so allow the absolute value of every number too.
	for n := range allowed {
		if f, err := strconv.ParseFloat(n, 64); err == nil && f < 0 {
			allowed[normalizeNumber(strconv.FormatFloat(-f, 'f', -1, 64))] = true
		}
	}
	return allowed
}

// numbersFromAny normalizes scalar values (and the numbers inside strings) so they
// can be matched against numbers extracted from ai_focus text.
func numbersFromAny(v any) []string {
	switch t := v.(type) {
	case nil:
		return nil
	case string:
		return extractNumbers(t)
	case float64:
		return []string{normalizeNumber(strconv.FormatFloat(t, 'f', -1, 64))}
	case float32:
		return []string{normalizeNumber(strconv.FormatFloat(float64(t), 'f', -1, 64))}
	case int:
		return []string{strconv.Itoa(t)}
	case int32:
		return []string{strconv.FormatInt(int64(t), 10)}
	case int64:
		return []string{strconv.FormatInt(t, 10)}
	case json.Number:
		return []string{normalizeNumber(t.String())}
	default:
		return extractNumbers(fmt.Sprintf("%v", t))
	}
}

// aiFocusClaimPattern captures an optional leading preposition, a number, and an
// optional trailing unit so an unsupported numeric claim can be removed cleanly.
var aiFocusClaimPattern = regexp.MustCompile(`(?i)(?:\b(?:на|до|около|с|свыше|более|менее)\s+|~\s*)?-?\d+(?:[.,]\d+)?\s*(?:%|процент\w*|₽|руб\.?|шт\.?|дн\.?|дней|раз\w*)?`)

// cloneAIFocus returns a deep copy so sanitization never mutates the raw AI output.
func cloneAIFocus(f *ActionsAIFocus) *ActionsAIFocus {
	if f == nil {
		return nil
	}
	cp := *f
	cp.TopCategories = append([]string(nil), f.TopCategories...)
	cp.Limitations = append([]string(nil), f.Limitations...)
	return &cp
}

// SanitizeAIFocusNumbers removes numeric claims from ai_focus text that are not
// backed by backend context. It returns the list of unsupported numbers that were
// removed so the action remains observable. A valid focus is never discarded — only
// the unsupported figures are stripped, leaving safe, qualitative wording.
func SanitizeAIFocusNumbers(focus *ActionsAIFocus, allowed map[string]bool) []string {
	if focus == nil {
		return nil
	}
	removed := make(map[string]bool)
	focus.Headline = sanitizeNumbersInText(focus.Headline, allowed, removed)
	focus.Summary = sanitizeNumbersInText(focus.Summary, allowed, removed)
	for i := range focus.Limitations {
		focus.Limitations[i] = sanitizeNumbersInText(focus.Limitations[i], allowed, removed)
	}
	out := make([]string, 0, len(removed))
	for n := range removed {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

func sanitizeNumbersInText(text string, allowed map[string]bool, removed map[string]bool) string {
	if strings.TrimSpace(text) == "" {
		return text
	}
	cleaned := aiFocusClaimPattern.ReplaceAllStringFunc(text, func(match string) string {
		nums := extractNumbers(match)
		if len(nums) == 0 {
			return match
		}
		for _, n := range nums {
			if !numberAllowed(n, allowed) {
				removed[n] = true
				return ""
			}
		}
		return match
	})
	return cleanupSanitizedText(cleaned)
}

// cleanupSanitizedText fixes whitespace and dangling punctuation left after removal.
func cleanupSanitizedText(s string) string {
	s = strings.ReplaceAll(s, " ,", ",")
	s = strings.ReplaceAll(s, " .", ".")
	s = strings.ReplaceAll(s, "( ", "(")
	s = strings.ReplaceAll(s, " )", ")")
	s = spaceCollapsePattern.ReplaceAllString(s, " ")
	// Collapse artifacts like ", ," and " ," produced by removed clauses.
	s = strings.ReplaceAll(s, ",,", ",")
	s = strings.ReplaceAll(s, " ,", ",")
	s = strings.ReplaceAll(s, ", и ", " и ")
	s = strings.TrimSpace(s)
	s = strings.Trim(s, ",;: ")
	return strings.TrimSpace(s)
}

var spaceCollapsePattern = regexp.MustCompile(`\s{2,}`)

// ApplyLegacyGlobalCheckOrdering applies the legacy global check result part-by-part with
// deterministic fallback for any part that is not usable. A valid ai_focus is kept
// even when ordering/top refs require deterministic repair.
func ApplyLegacyGlobalCheckOrdering(
	drafts []ScoredDraftAction,
	validation LegacyGlobalCheckValidationResult,
	opts GenerateActionsOptions,
) ([]ScoredDraftAction, []ScoredDraftAction, *ActionsAIFocus) {
	byID := make(map[string]ScoredDraftAction, len(drafts))
	for _, d := range drafts {
		byID[d.TempActionID] = d
	}

	deterministicOrder := func() []ScoredDraftAction {
		ord := OrderDraftsByScore(drafts)
		if len(ord) > opts.MaxActionsOutput {
			ord = ord[:opts.MaxActionsOutput]
		}
		return ord
	}

	// ----- ordering -----
	var ordered []ScoredDraftAction
	if len(validation.OrderedTempIDs) > 0 {
		ordered = make([]ScoredDraftAction, 0, len(validation.OrderedTempIDs))
		for _, id := range validation.OrderedTempIDs {
			d, ok := byID[id]
			if !ok {
				continue
			}
			if p, ok := validation.PriorityOverrides[id]; ok {
				d.Priority = p
			}
			ordered = append(ordered, d)
		}
		// append any drafts missing from the AI ordering, sorted by score
		seen := make(map[string]bool, len(ordered))
		for _, d := range ordered {
			seen[d.TempActionID] = true
		}
		for _, d := range OrderDraftsByScore(drafts) {
			if !seen[d.TempActionID] {
				ordered = append(ordered, d)
			}
		}
		if len(ordered) > opts.MaxActionsOutput {
			ordered = ordered[:opts.MaxActionsOutput]
		}
	} else {
		ordered = deterministicOrder()
	}

	// ----- top -----
	top := make([]ScoredDraftAction, 0, opts.MaxTopActions)
	topSeen := make(map[string]bool)
	for _, id := range validation.TopTempIDs {
		if len(top) >= opts.MaxTopActions {
			break
		}
		d, ok := byID[id]
		if !ok {
			continue
		}
		if p, ok := validation.PriorityOverrides[id]; ok {
			d.Priority = p
		}
		top = append(top, d)
		topSeen[d.TempActionID] = true
	}
	// Fill any missing top slots deterministically from the ordered list.
	if len(top) < opts.MaxTopActions {
		for _, d := range ordered {
			if len(top) >= opts.MaxTopActions {
				break
			}
			if topSeen[d.TempActionID] {
				continue
			}
			top = append(top, d)
			topSeen[d.TempActionID] = true
		}
	}

	// ----- ai_focus -----
	var focus *ActionsAIFocus
	if validation.FocusValid && validation.AIFocus != nil {
		focus = validation.AIFocus
		if strings.TrimSpace(focus.Source) == "" {
			focus.Source = "ai"
		}
	} else {
		focus = BuildDeterministicAIFocus(top)
	}

	return ordered, top, focus
}

// legacyGlobalCheckCategoryStatus derives the category-run status from how much of the
// AI result was usable. A valid (applied) ai_focus keeps the run out of full
// fallback; full fallback is reserved for the case where nothing AI-produced is used.
func legacyGlobalCheckCategoryStatus(v LegacyGlobalCheckValidationResult) string {
	focusApplied := v.FocusValid && v.AIFocus != nil
	switch {
	case v.Valid:
		return CategoryRunStatusCompleted
	case focusApplied || v.OrderingValid || v.TopValid:
		return CategoryRunStatusCompletedPartial
	default:
		return CategoryRunStatusFallback
	}
}

// buildLegacyGlobalCheckDiagnostic builds the structured record persisted to
// validation_json/final_validation_json for observability.
func buildLegacyGlobalCheckDiagnostic(stage, status string, v LegacyGlobalCheckValidationResult) LegacyGlobalCheckDiagnostic {
	focusApplied := v.FocusValid && v.AIFocus != nil
	diag := LegacyGlobalCheckDiagnostic{
		Stage:         stage,
		Status:        status,
		OrderingValid: v.OrderingValid,
		TopValid:      v.TopValid,
		FocusValid:    v.FocusValid,
		FocusApplied:  focusApplied,
		AvailableRefs: v.AvailableRefs,
		Issues:        v.Issues,
	}
	if focusApplied && v.AIFocus != nil {
		diag.FocusSource = v.AIFocus.Source
		if strings.TrimSpace(diag.FocusSource) == "" {
			diag.FocusSource = "ai"
		}
	} else {
		diag.FocusSource = "deterministic"
	}

	unknown := append([]string(nil), v.UnknownOrderedRefs...)
	unknown = append(unknown, v.UnknownTopRefs...)
	diag.UnknownRefs = unknown

	reasons := make([]string, 0, 3)
	scopes := make([]string, 0, 3)
	if !v.OrderingValid {
		reasons = append(reasons, "ordered_action_refs_not_resolved")
		scopes = append(scopes, "ordering")
	}
	if !v.TopValid {
		reasons = append(reasons, "top_action_refs_not_resolved")
		scopes = append(scopes, "top_actions")
	}
	if !v.FocusValid {
		if v.AIFocus == nil {
			reasons = append(reasons, "ai_focus_missing")
		} else {
			reasons = append(reasons, "ai_focus_invalid")
		}
		scopes = append(scopes, "ai_focus")
	}
	diag.Reason = strings.Join(reasons, ",")
	diag.FallbackScope = strings.Join(scopes, ",")
	return diag
}

// buildLegacyGlobalCheckInputContext captures exactly what the legacy global check saw,
// including the temp_action_id → action_key mapping, so the run is debuggable.
func buildLegacyGlobalCheckInputContext(input LegacyGlobalCheckInput, merged []ScoredDraftAction, metricDate string) map[string]any {
	mapping := make([]map[string]any, 0, len(merged))
	for _, d := range merged {
		mapping = append(mapping, map[string]any{
			"temp_action_id": d.TempActionID,
			"action_key":     BuildActionKey(metricDate, d.Action.Type, d.Action.SourceCandidateIDs, d.EntityGroupRef),
			"domain":         d.Domain,
			"type":           d.Action.Type,
			"category":       d.Action.Category,
			"priority":       d.Priority,
			"score":          d.Score,
			"title":          d.Action.Title,
		})
	}
	return map[string]any{
		"metric_date":         input.MetricDate,
		"account_snapshot":    input.AccountSnapshot,
		"max_top_actions":     input.MaxTopActions,
		"max_actions_output":  input.MaxActionsOutput,
		"forbidden_claims":    input.ForbiddenClaims,
		"draft_actions":       input.DraftActions,
		"temp_action_mapping": mapping,
	}
}

func buildDraftsForLegacyGlobalCheck(scored []ScoredDraftAction) []DraftActionForLegacyGlobalCheck {
	out := make([]DraftActionForLegacyGlobalCheck, 0, len(scored))
	for _, d := range scored {
		out = append(out, DraftActionForLegacyGlobalCheck{
			TempActionID: d.TempActionID,
			Domain:       d.Domain,
			Type:         d.Action.Type,
			Category:     d.Action.Category,
			Priority:     d.Priority,
			Score:        d.Score,
			Title:        d.Action.Title,
			Summary:      d.Action.Summary,
			Reason:       d.Action.Reason,
			NextStep:     d.Action.NextStep,
		})
	}
	return out
}

func draftsByTempID(drafts []ScoredDraftAction) map[string]ScoredDraftAction {
	out := make(map[string]ScoredDraftAction, len(drafts))
	for _, d := range drafts {
		out[d.TempActionID] = d
	}
	return out
}
