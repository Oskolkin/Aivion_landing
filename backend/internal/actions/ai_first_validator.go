package actions

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
)

type AIFirstValidationResult struct {
	Status          string                 `json:"status"`
	AcceptedActions []AIFirstDraftAction   `json:"accepted_actions"`
	Issues          []ValidationIssue      `json:"issues"`
	FocusIssues     []ValidationIssue      `json:"focus_issues,omitempty"`
	DroppedActions  []AIFirstDroppedAction `json:"dropped_actions,omitempty"`
}

type AIFirstDroppedAction struct {
	Action  AIFirstDraftAction `json:"action"`
	Reasons []string           `json:"reasons"`
}

// aiFirstDisallowedCTAKeys are CTA keys that must never appear on AI-first
// generated actions (the UI renders the three standard buttons itself).
var aiFirstDisallowedCTAKeys = []string{
	"open_sku_list",
	"open_campaign_list",
	"open_list",
	"download_xlsx",
	"export",
	"copy",
	"snooze",
}

var aiFirstAllowedCTAKeys = map[string]bool{
	"ask_ai":          true,
	"mark_completed":  true,
	"mark_checked":    true,
	"mark_irrelevant": true,
}

// aiFirstGenericPhrases flag vague actions. A phrase alone is not fatal:
// text with a concrete object and a number nearby stays valid.
var aiFirstGenericPhrases = []string{
	"проверьте рекламу",
	"проверить рекламу",
	"проверьте остатки",
	"посмотрите остатки",
	"посмотреть остатки",
	"проверьте товары",
	"проанализируйте",
	"проанализировать",
	"обратите внимание",
	"обратить внимание",
	"посмотрите воронку",
	"улучшите карточку",
	"улучшить карточку",
	"разберите ситуацию",
	"разобрать ситуацию",
	"проверьте sku",
}

// aiFirstRequiresObjectActions reports whether an action type is object-based
// and must carry a concrete object list inside the action.
func aiFirstRequiresObjectActions(t ActionType) bool {
	switch t {
	case ActionTypeFunnelWeakTransition:
		// Funnel actions are grounded through the funnel transition in
		// affected_entities; related SKU/category object_actions are optional.
		return false
	default:
		return true
	}
}

func ValidateActionsAIFirstOutput(input ActionsAIFirstInput, output ActionsAIFirstOutput) AIFirstValidationResult {
	result := AIFirstValidationResult{
		Status:          ValidationStatusValid,
		AcceptedActions: []AIFirstDraftAction{},
		Issues:          []ValidationIssue{},
	}
	allowedTypes := input.AllowedActionTypes
	if len(allowedTypes) == 0 {
		allowedTypes = ActionsAIFirstActiveActionTypes
	}
	allowedCategories := input.AllowedCategories
	if len(allowedCategories) == 0 {
		allowedCategories = ActionsAIFirstActiveCategories
	}
	allowedPriorities := input.AllowedPriorities
	if len(allowedPriorities) == 0 {
		allowedPriorities = []ActionPriority{ActionPriorityCritical, ActionPriorityImportant, ActionPriorityWatch}
	}
	if output.SchemaVersion != "" && output.SchemaVersion != input.SchemaVersion {
		result.Issues = append(result.Issues, ValidationIssue{Severity: IssueSeverityWarning, Code: "schema_version_mismatch", Message: "schema_version does not match input"})
	}
	allowedNumbers := collectAIFirstAllowedNumbers(input)
	for i, action := range output.Actions {
		issues := validateAIFirstAction(input, action, i, allowedTypes, allowedCategories, allowedPriorities, allowedNumbers)
		if len(issues) == 0 {
			result.AcceptedActions = append(result.AcceptedActions, action)
			continue
		}
		reasons := make([]string, 0, len(issues))
		fatalOrRepairable := false
		for _, issue := range issues {
			result.Issues = append(result.Issues, issue)
			reasons = append(reasons, issue.Code)
			if issue.Severity == IssueSeverityFatal || issue.Severity == IssueSeverityRepairable {
				fatalOrRepairable = true
			}
		}
		if fatalOrRepairable {
			result.DroppedActions = append(result.DroppedActions, AIFirstDroppedAction{Action: action, Reasons: reasons})
		} else {
			result.AcceptedActions = append(result.AcceptedActions, action)
		}
	}
	result.FocusIssues = validateAIFirstFocus(input, output.Focus, result.AcceptedActions, allowedNumbers)
	result.Issues = append(result.Issues, validateAIFirstGlobalContextConsistency(input, result.AcceptedActions)...)
	result.Issues = append(result.Issues, validateAIFirstV2Output(input, output)...)
	result = finalizeAIFirstValidationResult(result)
	if input.IsV2CandidateInput() && hasFatalValidationIssues(result.Issues) {
		for _, action := range result.AcceptedActions {
			result.DroppedActions = append(result.DroppedActions, AIFirstDroppedAction{
				Action:  action,
				Reasons: []string{ActionsErrorValidationFailed},
			})
		}
		result.AcceptedActions = []AIFirstDraftAction{}
		result.Status = ValidationStatusInvalid
	}
	return result
}

func hasFatalValidationIssues(issues []ValidationIssue) bool {
	for _, issue := range issues {
		if issue.Severity == IssueSeverityFatal {
			return true
		}
	}
	return false
}

func validateAIFirstGlobalContextConsistency(input ActionsAIFirstInput, accepted []AIFirstDraftAction) []ValidationIssue {
	gc := input.GlobalContext
	if gc == nil {
		return nil
	}
	var issues []ValidationIssue
	prioritySet := make(map[string]bool, len(gc.PriorityOrder))
	for _, cat := range gc.PriorityOrder {
		prioritySet[string(cat)] = true
	}
	globalCategories := make(map[string]bool)
	for _, r := range gc.PrimaryRisks {
		globalCategories[string(r.Category)] = true
	}
	for _, r := range gc.SecondaryRisks {
		globalCategories[string(r.Category)] = true
	}
	for _, s := range gc.Opportunities {
		globalCategories[string(s.Category)] = true
	}
	limitationsText := strings.ToLower(strings.Join(gc.DataLimitations, " "))

	for i, action := range accepted {
		idx := i
		cat := string(action.Category)
		if len(gc.PriorityOrder) > 0 && !prioritySet[cat] {
			issues = append(issues, ValidationIssue{
				Severity: IssueSeverityWarning, Code: "category_outside_priority_order",
				Message:     fmt.Sprintf("action category %q is outside global priority_order", cat),
				ActionIndex: &idx, Field: "category",
			})
		}
		if len(globalCategories) > 0 && !globalCategories[cat] {
			issues = append(issues, ValidationIssue{
				Severity: IssueSeverityWarning, Code: "category_absent_in_global_context",
				Message:     fmt.Sprintf("action category %q is absent from global risks/signals", cat),
				ActionIndex: &idx, Field: "category",
			})
		}
		actionText := strings.ToLower(strings.Join([]string{action.Title, action.Reason, action.Summary}, " "))
		for _, lim := range gc.DataLimitations {
			lim = strings.TrimSpace(strings.ToLower(lim))
			if lim == "" {
				continue
			}
			if strings.Contains(actionText, "sku-level") && strings.Contains(limitationsText, "sku-level") {
				issues = append(issues, ValidationIssue{
					Severity: IssueSeverityRepairable, Code: "contradicts_data_limitation",
					Message:     "action may contradict global data_limitations about SKU-level attribution",
					ActionIndex: &idx, Field: "visible_limitations",
				})
				break
			}
		}
	}
	return issues
}

func validateAIFirstAction(
	input ActionsAIFirstInput,
	action AIFirstDraftAction,
	idx int,
	allowedTypes []ActionType,
	allowedCategories []ActionCategory,
	allowedPriorities []ActionPriority,
	allowedNumbers map[string]bool,
) []ValidationIssue {
	var issues []ValidationIssue
	add := func(severity, code, message, field string) {
		i := idx
		issues = append(issues, ValidationIssue{Severity: severity, Code: code, Message: message, ActionIndex: &i, Field: field})
	}

	// Taxonomy: unknown or disallowed (data_quality_issue, key_sku_risk,
	// ad_high_drr, category data_quality) types are fatal.
	if !containsActionType(allowedTypes, action.Type) {
		add(IssueSeverityFatal, "invalid_action_type", fmt.Sprintf("disallowed action type %q", action.Type), "type")
	}
	if !containsActionCategory(allowedCategories, action.Category) {
		add(IssueSeverityFatal, "invalid_category", fmt.Sprintf("disallowed category %q", action.Category), "category")
	}
	if !containsActionPriority(allowedPriorities, action.Priority) {
		add(IssueSeverityFatal, "invalid_priority", fmt.Sprintf("unknown priority %q", action.Priority), "priority")
	}
	switch action.UrgencyMarker {
	case "", AIFirstUrgencyUrgent, AIFirstUrgencyWatch:
	default:
		add(IssueSeverityRepairable, "invalid_urgency_marker", fmt.Sprintf("unknown urgency_marker %q", action.UrgencyMarker), "urgency_marker")
	}

	// Mandatory fields for self-sufficient actions.
	if strings.TrimSpace(action.Title) == "" {
		add(IssueSeverityRepairable, "missing_title", "title is required", "title")
	}
	if strings.TrimSpace(action.Summary) == "" {
		add(IssueSeverityRepairable, "missing_summary", "summary is required", "summary")
	}
	if strings.TrimSpace(action.Reason) == "" {
		add(IssueSeverityRepairable, "missing_reason", "reason is required", "reason")
	}
	if len(action.ActionPlan) == 0 && strings.TrimSpace(action.NextStep) == "" {
		add(IssueSeverityRepairable, "missing_action_plan", "action_plan or next_step is required", "action_plan")
	}
	if strings.TrimSpace(action.ExpectedEffect) == "" {
		add(IssueSeverityRepairable, "missing_expected_effect", "expected_effect is required", "expected_effect")
	}
	if strings.TrimSpace(action.CompletionCriteria) == "" {
		add(IssueSeverityRepairable, "missing_completion_criteria", "completion_criteria is required", "completion_criteria")
	}
	if len(action.EvidenceFactIDs) == 0 {
		add(IssueSeverityFatal, "missing_evidence_fact_ids", "evidence_fact_ids is required", "evidence_fact_ids")
	}
	if len(action.AffectedEntities) == 0 {
		add(IssueSeverityFatal, "missing_affected_entities", "affected_entities is required", "affected_entities")
	}

	// Object-based actions must carry concrete objects inside the action.
	if aiFirstActionRequiresObjectActionsDetail(action) && len(action.ObjectActions) == 0 {
		add(IssueSeverityRepairable, "missing_object_actions", "object_actions is required when affected_entities contain concrete objects", "object_actions")
	}
	if action.Type == ActionTypeFunnelWeakTransition && !hasAIFirstFunnelRef(action.AffectedEntities) {
		add(IssueSeverityRepairable, "missing_funnel_transition", "funnel_weak_transition requires a funnel affected entity", "affected_entities")
	}
	for oi, obj := range action.ObjectActions {
		field := fmt.Sprintf("object_actions[%d]", oi)
		if strings.TrimSpace(obj.EntityType) == "" || strings.TrimSpace(obj.EntityID) == "" {
			add(IssueSeverityRepairable, "invalid_object_action", "object action requires entity_type and entity_id", field)
			continue
		}
		if strings.TrimSpace(obj.RecommendedAction) == "" {
			add(IssueSeverityRepairable, "missing_object_recommended_action", "object action requires recommended_action", field)
		}
		if strings.TrimSpace(obj.Reason) == "" {
			add(IssueSeverityRepairable, "missing_object_reason", "object action requires reason", field)
		}
		if countNonEmptyObjectMetrics(obj.Metrics) < aiFirstMinObjectActionMetrics {
			add(IssueSeverityRepairable, "insufficient_object_metrics", fmt.Sprintf("object action requires at least %d metrics", aiFirstMinObjectActionMetrics), field)
		}
		if !isKnownAIFirstEntityRef(input, AIFirstAffectedEntityRef{EntityType: obj.EntityType, EntityID: obj.EntityID}) {
			add(IssueSeverityFatal, "unknown_object_entity", fmt.Sprintf("unknown object entity %s:%s", obj.EntityType, obj.EntityID), field)
		}
		for _, id := range obj.EvidenceFactIDs {
			if !input.factExists(id) {
				add(IssueSeverityFatal, "unknown_object_fact_id", fmt.Sprintf("unknown object evidence_fact_id %q", id), field)
			}
		}
	}

	// Generic action guard: vague phrasing without a concrete object/number.
	if isGenericAIFirstText(action.Title) {
		add(IssueSeverityRepairable, "generic_title", "title is too generic", "title")
	}
	if isGenericAIFirstText(action.Summary) {
		add(IssueSeverityRepairable, "generic_summary", "summary is too generic", "summary")
	}
	if isGenericNextStep(action.NextStep) {
		add(IssueSeverityRepairable, "generic_next_step", "next_step is too generic", "next_step")
	}
	if len(action.ActionPlan) > 0 && len(action.ActionPlan) < aiFirstMinActionPlanSteps {
		add(IssueSeverityRepairable, "insufficient_action_plan", fmt.Sprintf("action_plan must contain at least %d steps", aiFirstMinActionPlanSteps), "action_plan")
	} else if len(action.ActionPlan) > aiFirstMaxActionPlanSteps {
		add(IssueSeverityRepairable, "excessive_action_plan", fmt.Sprintf("action_plan must contain at most %d steps", aiFirstMaxActionPlanSteps), "action_plan")
	}
	if aiFirstActionRequiresObjectActionsDetail(action) && len([]rune(strings.TrimSpace(action.Reason))) < aiFirstMinObjectBasedReasonLen {
		add(IssueSeverityRepairable, "reason_too_short", fmt.Sprintf("reason must be at least %d characters for object-based actions", aiFirstMinObjectBasedReasonLen), "reason")
	}
	for si, step := range action.ActionPlan {
		if strings.TrimSpace(step.Instruction) == "" {
			add(IssueSeverityRepairable, "missing_step_instruction", "action plan step requires instruction", fmt.Sprintf("action_plan[%d]", si))
			continue
		}
		if isGenericActionStep(step.Instruction) {
			add(IssueSeverityRepairable, "generic_action_step", "action plan step is too generic", fmt.Sprintf("action_plan[%d]", si))
		}
	}

	// Forbidden claims and advertising SKU-level claims.
	for _, field := range aiFirstTextFields(action) {
		if pattern, ok := containsForbiddenClaim(field.v, input.ForbiddenClaims); ok {
			add(IssueSeverityRepairable, "forbidden_claim", fmt.Sprintf("forbidden claim pattern %q in %s", pattern, field.n), field.n)
		}
		if advertisingSKULevelClaim(field.v) {
			add(IssueSeverityFatal, "advertising_sku_level_claim", "advertising actions must remain campaign-level", field.n)
		}
	}

	// Fact / entity grounding.
	for _, factID := range action.EvidenceFactIDs {
		if !input.factExists(factID) {
			add(IssueSeverityFatal, "unknown_fact_id", fmt.Sprintf("unknown evidence_fact_id %q", factID), "evidence_fact_ids")
		}
	}
	for _, ent := range action.AffectedEntities {
		if !isKnownAIFirstEntityRef(input, ent) {
			add(IssueSeverityFatal, "unknown_affected_entity", fmt.Sprintf("unknown affected entity %s:%s", ent.EntityType, ent.EntityID), "affected_entities")
		}
	}

	// CTA guard: AI-first actions must not carry list/export/copy/snooze CTAs.
	for _, cta := range action.CTAKeys {
		key := strings.ToLower(strings.TrimSpace(cta))
		if key == "" {
			continue
		}
		if isDisallowedAIFirstCTA(key) || !aiFirstAllowedCTAKeys[key] {
			add(IssueSeverityRepairable, "disallowed_cta", fmt.Sprintf("cta_key %q is not allowed for AI-first actions", cta), "cta_keys")
		}
	}

	// Required visible limitations.
	if action.Category == ActionCategoryAdvertising && !hasAdvertisingCampaignLimitation(action) {
		add(IssueSeverityRepairable, "missing_advertising_limitation", "advertising action requires campaign-level limitation in visible_limitations", "visible_limitations")
	}
	if (action.Type == ActionTypeStockReplenishment || action.Type == ActionTypeStockRedistribution) && !hasStockSupplyLimitation(action) {
		add(IssueSeverityRepairable, "missing_stock_limitation", "stock action requires in-transit supplies limitation in visible_limitations", "visible_limitations")
	}

	// Every number in seller-facing text must be backed by referenced facts.
	actionAllowedNumbers := allowedNumbersForAIFirstAction(input, allowedNumbers, action)
	for _, field := range aiFirstTextFields(action) {
		for _, num := range extractNumbers(field.v) {
			if !numberAllowed(num, actionAllowedNumbers) {
				add(IssueSeverityRepairable, "unsupported_number", fmt.Sprintf("unsupported number %q in %s", num, field.n), field.n)
				break
			}
		}
	}
	for oi, obj := range action.ObjectActions {
		for _, m := range obj.Metrics {
			for _, num := range extractNumbers(m.Value) {
				if !numberAllowed(num, actionAllowedNumbers) {
					add(IssueSeverityRepairable, "unsupported_object_metric_number", fmt.Sprintf("unsupported number %q in object_actions[%d] metric %q", num, oi, m.Label), fmt.Sprintf("object_actions[%d]", oi))
					break
				}
			}
		}
	}

	if action.OperationalDetail != nil {
		for _, f := range action.OperationalDetail.Why.Facts {
			if strings.TrimSpace(f.FactID) != "" {
				if !input.factExists(f.FactID) {
					add(IssueSeverityFatal, "unknown_operational_fact_id", fmt.Sprintf("unknown operational fact_id %q", f.FactID), "operational_detail")
				}
			}
		}
		for _, step := range action.OperationalDetail.ActionPlan.Steps {
			for _, id := range step.EvidenceFactIDs {
				if !input.factExists(id) {
					add(IssueSeverityFatal, "unknown_operational_evidence_fact_id", fmt.Sprintf("unknown operational evidence_fact_id %q", id), "operational_detail")
				}
			}
		}
	}
	return issues
}

// validateAIFirstFocus checks the focus block against accepted actions.
// Issues here never invalidate actions; the runtime drops an inconsistent
// focus and builds a deterministic one instead.
func validateAIFirstFocus(input ActionsAIFirstInput, focus *ActionsAIFirstFocus, accepted []AIFirstDraftAction, allowedNumbers map[string]bool) []ValidationIssue {
	if focus == nil {
		return nil
	}
	var issues []ValidationIssue
	add := func(code, message, field string) {
		issues = append(issues, ValidationIssue{Severity: IssueSeverityWarning, Code: code, Message: message, Field: field})
	}
	acceptedCategories := make(map[ActionCategory]bool, len(accepted))
	for _, a := range accepted {
		acceptedCategories[a.Category] = true
	}
	if len(accepted) > 0 {
		for _, cat := range focus.PriorityOrder {
			if !acceptedCategories[cat] {
				add("focus_category_without_action", fmt.Sprintf("focus priority_order category %q has no accepted action", cat), "focus.priority_order")
			}
		}
	}
	for _, id := range focus.EvidenceFactIDs {
		if !input.factExists(id) {
			add("focus_unknown_fact_id", fmt.Sprintf("unknown focus evidence_fact_id %q", id), "focus.evidence_fact_ids")
		}
	}
	for _, field := range []struct{ n, v string }{
		{"focus.headline", focus.Headline},
		{"focus.summary", focus.Summary},
		{"focus.start_with", focus.StartWith},
	} {
		if pattern, ok := containsForbiddenClaim(field.v, input.ForbiddenClaims); ok {
			add("focus_forbidden_claim", fmt.Sprintf("forbidden claim pattern %q in %s", pattern, field.n), field.n)
		}
		for _, num := range extractNumbers(field.v) {
			if !numberAllowed(num, allowedNumbers) {
				add("focus_unsupported_number", fmt.Sprintf("unsupported number %q in %s", num, field.n), field.n)
				break
			}
		}
	}
	return issues
}

type aiFirstTextField struct{ n, v string }

func aiFirstTextFields(action AIFirstDraftAction) []aiFirstTextField {
	fields := []aiFirstTextField{
		{"title", action.Title},
		{"summary", action.Summary},
		{"reason", action.Reason},
		{"next_step", action.NextStep},
		{"expected_effect", action.ExpectedEffect},
		{"completion_criteria", action.CompletionCriteria},
		{"affected_objects_summary", action.AffectedObjectsSummary},
	}
	for i, step := range action.ActionPlan {
		fields = append(fields, aiFirstTextField{fmt.Sprintf("action_plan[%d]", i), strings.Join([]string{step.Title, step.Instruction, step.ExpectedResult}, " ")})
	}
	for i, obj := range action.ObjectActions {
		fields = append(fields, aiFirstTextField{fmt.Sprintf("object_actions[%d]", i), strings.Join([]string{obj.RecommendedAction, obj.Reason}, " ")})
	}
	return fields
}

// isGenericAIFirstText flags vague phrasing. A generic phrase together with a
// concrete number (object count, metric) is acceptable.
func isGenericAIFirstText(text string) bool {
	t := strings.ToLower(strings.TrimSpace(text))
	if t == "" {
		return false
	}
	hasGeneric := false
	for _, g := range aiFirstGenericPhrases {
		if strings.Contains(t, g) {
			hasGeneric = true
			break
		}
	}
	if !hasGeneric {
		// Also reuse legacy generic title list for exact/prefix matches.
		if !isGenericTitle(t) {
			return false
		}
		hasGeneric = true
	}
	return !strings.ContainsAny(t, "0123456789")
}

func isDisallowedAIFirstCTA(key string) bool {
	for _, bad := range aiFirstDisallowedCTAKeys {
		if key == bad || strings.HasPrefix(key, "export_") || strings.HasPrefix(key, "open_") {
			return true
		}
	}
	return false
}

func hasStockSupplyLimitation(action AIFirstDraftAction) bool {
	for _, l := range append(append([]string(nil), action.VisibleLimitations...), action.Limitations...) {
		lower := strings.ToLower(l)
		switch {
		case strings.Contains(lower, "постав"),
			strings.Contains(lower, "in-transit"),
			strings.Contains(lower, "in transit"),
			strings.Contains(lower, "supplies"),
			strings.Contains(lower, "stock analytics"):
			return true
		case strings.Contains(lower, "товар") && (strings.Contains(lower, "пути") || strings.Contains(lower, "transit")):
			return true
		case strings.Contains(lower, "snapshot") && strings.Contains(lower, "stock"):
			return true
		}
	}
	return false
}

func hasLimitationMarker(action AIFirstDraftAction, marker string) bool {
	for _, l := range append(append([]string(nil), action.VisibleLimitations...), action.Limitations...) {
		if strings.Contains(strings.ToLower(l), marker) {
			return true
		}
	}
	return false
}

func hasAdvertisingCampaignLimitation(action AIFirstDraftAction) bool {
	for _, l := range append(append([]string(nil), action.VisibleLimitations...), action.Limitations...) {
		lower := strings.ToLower(l)
		if strings.Contains(lower, "кампан") || strings.Contains(lower, "campaign") {
			return true
		}
	}
	return false
}

func allowedNumbersForAIFirstAction(input ActionsAIFirstInput, base map[string]bool, action AIFirstDraftAction) map[string]bool {
	out := make(map[string]bool, len(base)+24)
	for k, v := range base {
		out[k] = v
	}
	addFromText := func(v string) {
		for _, n := range extractNumbers(v) {
			out[n] = true
		}
	}
	addFromFactID := func(id string) {
		fact, ok := input.factByID(id)
		if !ok {
			return
		}
		for _, n := range numbersFromAny(fact.Value) {
			out[n] = true
			addNumberSignVariants(out, n)
		}
		addFromText(fact.FormattedValue)
	}
	for _, id := range action.EvidenceFactIDs {
		addFromFactID(id)
	}
	for _, obj := range action.ObjectActions {
		for _, id := range obj.EvidenceFactIDs {
			addFromFactID(id)
		}
		addFromText(obj.EntityID)
		for _, m := range obj.Metrics {
			addFromText(m.Value)
		}
	}
	for _, ent := range action.AffectedEntities {
		addFromText(ent.EntityID)
	}
	return out
}

func addNumberSignVariants(out map[string]bool, n string) {
	f, err := strconv.ParseFloat(normalizeNumber(n), 64)
	if err != nil {
		return
	}
	out[normalizeNumber(strconv.FormatFloat(-f, 'f', -1, 64))] = true
	out[normalizeNumber(strconv.FormatFloat(math.Abs(f), 'f', -1, 64))] = true
}

func hasAIFirstFunnelRef(entities []AIFirstAffectedEntityRef) bool {
	for _, ent := range entities {
		if strings.EqualFold(strings.TrimSpace(ent.EntityType), "funnel") {
			return true
		}
	}
	return false
}

func finalizeAIFirstValidationResult(result AIFirstValidationResult) AIFirstValidationResult {
	hasFatal := false
	hasRepairable := false
	hasWarning := len(result.FocusIssues) > 0
	for _, issue := range result.Issues {
		switch issue.Severity {
		case IssueSeverityFatal:
			hasFatal = true
		case IssueSeverityRepairable:
			hasRepairable = true
		case IssueSeverityWarning:
			hasWarning = true
		}
	}
	switch {
	case hasFatal && len(result.AcceptedActions) == 0:
		result.Status = ValidationStatusInvalid
	case hasFatal || hasRepairable:
		result.Status = ValidationStatusRepairable
	case hasWarning:
		result.Status = ValidationStatusValidWithWarnings
	default:
		result.Status = ValidationStatusValid
	}
	return result
}

func isKnownAIFirstEntityRef(input ActionsAIFirstInput, ent AIFirstAffectedEntityRef) bool {
	t := strings.TrimSpace(ent.EntityType)
	id := strings.TrimSpace(ent.EntityID)
	if t == "" || id == "" {
		return false
	}
	switch t {
	case "account":
		return id == "account:seller"
	case "funnel":
		return id == "funnel:account"
	case "data_quality":
		return strings.HasPrefix(id, "data_quality:")
	case "sku":
		if input.IsV2CandidateInput() {
			_, ok := input.EntityCatalog[id]
			return ok
		}
		pack, err := input.requireDataPack()
		if err != nil {
			return false
		}
		_, ok := pack.EntityRegistry.SKUs[id]
		return ok
	case "sku_group":
		if input.IsV2CandidateInput() {
			_, ok := input.EntityCatalog[id]
			return ok
		}
		pack, err := input.requireDataPack()
		if err != nil {
			return false
		}
		_, ok := pack.EntityRegistry.SKUGroups[id]
		return ok
	case "campaign":
		if input.IsV2CandidateInput() {
			_, ok := input.EntityCatalog[id]
			return ok
		}
		pack, err := input.requireDataPack()
		if err != nil {
			return false
		}
		_, ok := pack.EntityRegistry.Campaigns[id]
		return ok
	case "category":
		if input.IsV2CandidateInput() {
			_, ok := input.EntityCatalog[id]
			return ok
		}
		pack, err := input.requireDataPack()
		if err != nil {
			return false
		}
		_, ok := pack.EntityRegistry.Categories[id]
		return ok
	case "warehouse":
		if input.IsV2CandidateInput() {
			_, ok := input.EntityCatalog[id]
			return ok
		}
		pack, err := input.requireDataPack()
		if err != nil {
			return false
		}
		_, ok := pack.EntityRegistry.Warehouses[id]
		return ok
	default:
		return false
	}
}

func containsActionType(list []ActionType, val ActionType) bool {
	for _, item := range list {
		if item == val {
			return true
		}
	}
	return false
}

func containsActionCategory(list []ActionCategory, val ActionCategory) bool {
	for _, item := range list {
		if item == val {
			return true
		}
	}
	return false
}

func containsActionPriority(list []ActionPriority, val ActionPriority) bool {
	for _, item := range list {
		if item == val {
			return true
		}
	}
	return false
}

func stringInList(list []string, val string) bool {
	for _, item := range list {
		if item == val {
			return true
		}
	}
	return false
}

// collectAIFirstAllowedNumbers builds the allowlist of numbers that may appear
// in seller-facing text: fact values, entity identifiers, data window dates
// and standard analysis-window day counts (1-2 days recheck, 7d/14d windows).
func collectAIFirstAllowedNumbers(input ActionsAIFirstInput) map[string]bool {
	var facts map[string]aidatapack.Fact
	if input.IsV2CandidateInput() {
		facts = make(map[string]aidatapack.Fact, len(input.FactCatalog))
		for id, cf := range input.FactCatalog {
			facts[id] = compactFactToFact(cf)
		}
	} else {
		pack, err := input.requireDataPack()
		if err != nil {
			return map[string]bool{}
		}
		facts = pack.FactRegistry
	}
	out := collectAllowedNumbersFromFactRegistry(facts)
	addText := func(v string) {
		for _, n := range extractNumbers(v) {
			out[n] = true
		}
	}
	pack, _ := input.requireDataPack()
	if pack != nil {
		reg := pack.EntityRegistry
		for id, e := range reg.SKUs {
			addText(id)
			addText(e.SKU)
			addText(e.OfferID)
			addText(e.OzonProductID)
		}
		for id := range reg.SKUGroups {
			addText(id)
		}
		for id, e := range reg.Campaigns {
			addText(id)
			addText(e.EntityID)
		}
		for id := range reg.Categories {
			addText(id)
		}
		for id := range reg.Warehouses {
			addText(id)
		}
		w := pack.DataWindows
		for _, v := range []string{w.CurrentDay, w.Yesterday, w.Last7d.From, w.Last7d.To, w.Previous7d.From, w.Previous7d.To, w.Last14d.From, w.Last14d.To} {
			addText(v)
		}
	}
	addText(input.MetricDate)
	// Standard analysis windows and recheck horizons ("через 1-2 дня", "за 7 дней").
	for _, n := range []string{"1", "2", "7", "14"} {
		out[n] = true
	}
	return out
}

func collectAllowedNumbersFromFactRegistry(facts map[string]aidatapack.Fact) map[string]bool {
	out := make(map[string]bool)
	for _, fact := range facts {
		for _, n := range numbersFromAny(fact.Value) {
			out[n] = true
		}
		for _, n := range extractNumbers(fact.FormattedValue) {
			out[n] = true
		}
	}
	return out
}
