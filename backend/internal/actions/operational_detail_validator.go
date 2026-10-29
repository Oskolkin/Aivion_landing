package actions

import (
	"fmt"
	"strings"
)

var genericOperationalStepPatterns = []string{
	"откройте список",
	"проверьте настройки",
	"проанализируйте данные",
	"проанализируйте ситуацию",
	"изучите данные",
	"обратите внимание",
	"посмотрите подробнее",
	"проверьте данные",
}

var directPriceActionKinds = []string{
	"decrease_price",
	"increase_price",
	"reduce_price",
	"change_price",
}

var directReplenishQtyKinds = []string{
	"replenish_stock_qty",
	"replenish_exact_qty",
}

// ValidateOperationalDetail validates operational_detail against candidate facts and safety rules.
func ValidateOperationalDetail(
	detail OperationalDetail,
	actionType ActionType,
	candidates []ActionCandidate,
) []ValidationIssue {
	var issues []ValidationIssue

	if strings.TrimSpace(detail.Status) == "" {
		issues = append(issues, ValidationIssue{
			Severity: IssueSeverityRepairable,
			Code:     "missing_operational_status",
			Message:  "operational_detail.status is required",
			Field:    "operational_detail.status",
		})
	}
	if strings.TrimSpace(detail.Why.Text) == "" {
		issues = append(issues, ValidationIssue{
			Severity: IssueSeverityRepairable,
			Code:     "missing_operational_why",
			Message:  "operational_detail.why.text is required",
			Field:    "operational_detail.why.text",
		})
	}
	if len(detail.ActionPlan.Steps) == 0 {
		issues = append(issues, ValidationIssue{
			Severity: IssueSeverityRepairable,
			Code:     "missing_operational_steps",
			Message:  "operational_detail.action_plan.steps is required",
			Field:    "operational_detail.action_plan.steps",
		})
	}

	allowedFacts := collectFactsFromCandidates(candidates)
	allowedNumbers := collectAllowedNumbers(candidates)
	allowedEntityIDs := collectAllowedEntityIDs(candidates)

	if strings.TrimSpace(detail.Why.Text) != "" && len(allowedFacts) > 0 {
		if !whyContainsConfirmedFact(detail.Why.Text, allowedFacts) {
			issues = append(issues, ValidationIssue{
				Severity: IssueSeverityRepairable,
				Code:     "operational_why_missing_fact",
				Message:  "operational_detail.why.text must reference at least one confirmed fact",
				Field:    "operational_detail.why.text",
			})
		}
	}

	for i, step := range detail.ActionPlan.Steps {
		stepIssues := validateOperationalStep(step, i, actionType, allowedFacts, allowedNumbers, allowedEntityIDs, candidates)
		issues = append(issues, stepIssues...)
	}

	for i, obj := range detail.ObjectActions {
		if strings.TrimSpace(obj.EntityID) == "" {
			continue
		}
		key := obj.EntityType + ":" + obj.EntityID
		if len(allowedEntityIDs) > 0 && !allowedEntityIDs[key] {
			issues = append(issues, ValidationIssue{
				Severity: IssueSeverityRepairable,
				Code:     "operational_unknown_entity",
				Message:  fmt.Sprintf("object_actions[%d] entity %q not in candidate examples/entity group", i, obj.EntityID),
				Field:    "operational_detail.object_actions",
			})
		}
	}

	return issues
}

func validateOperationalStep(
	step OperationalStep,
	idx int,
	actionType ActionType,
	allowedFacts map[string]ActionCandidateFact,
	allowedNumbers map[string]bool,
	allowedEntityIDs map[string]bool,
	candidates []ActionCandidate,
) []ValidationIssue {
	var issues []ValidationIssue
	add := func(severity, code, message, field string) {
		issues = append(issues, ValidationIssue{
			Severity:    severity,
			Code:        code,
			Message:     message,
			ActionIndex: &idx,
			Field:       field,
		})
	}

	combined := strings.ToLower(step.Title + " " + step.Instruction)
	if isGenericOperationalStep(combined) {
		add(IssueSeverityRepairable, "generic_operational_step", "operational step is too generic", fmt.Sprintf("operational_detail.action_plan.steps[%d]", idx))
	}

	for _, field := range []struct {
		name string
		val  string
	}{
		{"title", step.Title},
		{"instruction", step.Instruction},
		{"why_this_step", step.WhyThisStep},
		{"expected_result", step.ExpectedResult},
	} {
		if strings.TrimSpace(field.val) == "" {
			add(IssueSeverityRepairable, "missing_operational_step_field", fmt.Sprintf("step %s is required", field.name), fmt.Sprintf("operational_detail.action_plan.steps[%d].%s", idx, field.name))
		}
	}

	mode := step.DecisionMode
	if mode == "" {
		mode = mapConfidenceToDecisionMode(step.Confidence)
	}
	if mode == "" {
		add(IssueSeverityRepairable, "missing_decision_mode", "step must have decision_mode or confidence", fmt.Sprintf("operational_detail.action_plan.steps[%d].decision_mode", idx))
	}

	if mode == DecisionModeDiagnostic || mode == DecisionModeConditional {
		if !diagnosticStepIsConcrete(step) {
			add(IssueSeverityRepairable, "vague_diagnostic_step", "diagnostic/conditional step must specify what to check and what decision to take", fmt.Sprintf("operational_detail.action_plan.steps[%d]", idx))
		}
	}

	if isDirectPriceAction(step, combined) && !hasPriceMarginContext(candidates) {
		add(IssueSeverityRepairable, "unsupported_direct_price", "direct price change requires price/margin context", fmt.Sprintf("operational_detail.action_plan.steps[%d]", idx))
	}

	if isDirectReplenishQtyAction(step) && !hasReplenishQtyFacts(candidates) {
		add(IssueSeverityRepairable, "unsupported_replenish_qty", "exact replenishment quantity requires supply quantity facts", fmt.Sprintf("operational_detail.action_plan.steps[%d]", idx))
	}

	for _, factID := range step.EvidenceFactIDs {
		if _, ok := allowedFacts[factID]; !ok {
			add(IssueSeverityRepairable, "operational_unknown_fact_id", fmt.Sprintf("unknown evidence_fact_id %q in step", factID), fmt.Sprintf("operational_detail.action_plan.steps[%d].evidence_fact_ids", idx))
		}
	}

	for _, field := range []string{step.Title, step.Instruction, step.WhyThisStep, step.ExpectedResult} {
		for _, num := range extractNumbers(field) {
			if !numberAllowed(num, allowedNumbers) {
				add(IssueSeverityRepairable, "operational_unsupported_number", fmt.Sprintf("unsupported number %q in step", num), fmt.Sprintf("operational_detail.action_plan.steps[%d]", idx))
				break
			}
		}
	}

	for _, ent := range step.TargetEntities {
		key := ent.EntityType + ":" + ent.ID
		if ent.ID != "" && len(allowedEntityIDs) > 0 && !allowedEntityIDs[key] {
			add(IssueSeverityRepairable, "operational_unknown_target_entity", fmt.Sprintf("unknown target entity %q", ent.ID), fmt.Sprintf("operational_detail.action_plan.steps[%d].target_entities", idx))
		}
	}

	if issue := validateActionSafety(step, actionType, combined); issue != nil {
		issues = append(issues, *issue)
	}

	return issues
}

func isGenericOperationalStep(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	for _, p := range genericOperationalStepPatterns {
		if strings.Contains(lower, p) {
			// Allow if there's also a concrete condition/decision cue.
			if strings.Contains(lower, "если") || strings.Contains(lower, "→") || strings.Contains(lower, "—") {
				continue
			}
			return true
		}
	}
	return false
}

func diagnosticStepIsConcrete(step OperationalStep) bool {
	combined := strings.ToLower(step.Instruction + " " + step.WhyThisStep + " " + step.Title)
	hasCondition := strings.Contains(combined, "если") ||
		strings.Contains(combined, "когда") ||
		strings.Contains(step.Title, "→")
	hasDecision := strings.Contains(combined, "провер") ||
		strings.Contains(combined, "сниз") ||
		strings.Contains(combined, "пополн") ||
		strings.Contains(combined, "сформир") ||
		strings.Contains(combined, "отключ") ||
		strings.Contains(combined, "пауз") ||
		strings.Contains(combined, "перейд") ||
		strings.Contains(combined, "исключ") ||
		strings.Contains(combined, "огранич")
	return hasCondition && hasDecision
}

func mapConfidenceToDecisionMode(confidence string) string {
	switch strings.ToLower(confidence) {
	case ConfidenceHigh:
		return DecisionModeDirect
	case ConfidenceMedium:
		return DecisionModeConditional
	case ConfidenceLow:
		return DecisionModeDiagnostic
	default:
		return confidence
	}
}

func whyContainsConfirmedFact(text string, facts map[string]ActionCandidateFact) bool {
	lower := strings.ToLower(text)
	for _, f := range facts {
		if strings.TrimSpace(f.Value) != "" && strings.Contains(lower, strings.ToLower(strings.TrimSpace(f.Value))) {
			return true
		}
		if strings.TrimSpace(f.Label) != "" && strings.Contains(lower, strings.ToLower(strings.TrimSpace(f.Label))) {
			return true
		}
	}
	for _, n := range extractNumbers(text) {
		if len(n) > 0 {
			return true
		}
	}
	return false
}

func collectAllowedEntityIDs(candidates []ActionCandidate) map[string]bool {
	out := make(map[string]bool)
	for _, c := range candidates {
		for _, ex := range c.TopExamples {
			out[ex.EntityType+":"+ex.EntityID] = true
		}
	}
	return out
}

func hasPriceMarginContext(candidates []ActionCandidate) bool {
	for _, c := range candidates {
		for k := range c.Aggregates {
			kl := strings.ToLower(k)
			if strings.Contains(kl, "margin") || strings.Contains(kl, "price") || strings.Contains(kl, "себестоим") {
				return true
			}
		}
	}
	return false
}

func hasReplenishQtyFacts(candidates []ActionCandidate) bool {
	for _, c := range candidates {
		for _, f := range c.Facts {
			id := strings.ToLower(f.FactID)
			if strings.Contains(id, "recommended_supply_qty") || strings.Contains(id, "supply_qty") {
				return true
			}
		}
		for k := range c.Aggregates {
			if strings.Contains(strings.ToLower(k), "recommended_supply_qty") {
				return true
			}
		}
	}
	return false
}

func isDirectPriceAction(step OperationalStep, combined string) bool {
	kind := strings.ToLower(step.ActionKind)
	for _, k := range directPriceActionKinds {
		if kind == k {
			return true
		}
	}
	if step.DecisionMode == DecisionModeDirect || mapConfidenceToDecisionMode(step.Confidence) == DecisionModeDirect {
		for _, p := range []string{"снизить цену", "снизьте цену", "поднять цену", "изменить цену"} {
			if strings.Contains(combined, p) {
				return true
			}
		}
	}
	return false
}

func isDirectReplenishQtyAction(step OperationalStep) bool {
	kind := strings.ToLower(step.ActionKind)
	for _, k := range directReplenishQtyKinds {
		if kind == k {
			return true
		}
	}
	if step.DecisionMode != DecisionModeDirect && mapConfidenceToDecisionMode(step.Confidence) != DecisionModeDirect {
		return false
	}
	combined := strings.ToLower(step.Instruction)
	return strings.Contains(combined, "пополнить на") ||
		(strings.Contains(combined, "пополн") && strings.Contains(combined, " шт"))
}

func validateActionSafety(step OperationalStep, actionType ActionType, combined string) *ValidationIssue {
	mode := step.DecisionMode
	if mode == "" {
		mode = mapConfidenceToDecisionMode(step.Confidence)
	}
	if mode != DecisionModeDirect {
		return nil
	}

	kind := strings.ToLower(step.ActionKind)
	switch {
	case strings.Contains(kind, "pause") || strings.Contains(combined, "на паузу"):
		if actionType != ActionTypeAdSpendNoOrders && actionType != ActionTypeAdHighDRR {
			return &ValidationIssue{
				Severity: IssueSeverityRepairable,
				Code:     "unsupported_pause_action",
				Message:  "pause_campaign direct action is only allowed for ad_spend_no_orders and ad_high_drr",
				Field:    "operational_detail.action_plan.steps",
			}
		}
	case strings.Contains(kind, "reduce_budget") || strings.Contains(kind, "reduce_bids"):
		if actionType != ActionTypeAdHighDRR && actionType != ActionTypeAdSpendNoOrders {
			return &ValidationIssue{
				Severity: IssueSeverityRepairable,
				Code:     "unsupported_budget_action",
				Message:  "reduce_budget/reduce_bids only allowed for advertising actions",
				Field:    "operational_detail.action_plan.steps",
			}
		}
	}
	return nil
}
