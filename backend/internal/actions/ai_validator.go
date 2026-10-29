package actions

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var numberPattern = regexp.MustCompile(`-?\d+(?:[.,]\d+)?`)
var spacedThousandsPattern = regexp.MustCompile(`\d{1,3}(?:\s\d{3})+`)

// ValidateDomainComposerOutput validates AI draft actions against domain context and rules.
func ValidateDomainComposerOutput(domainCtx ActionDomainContext, output DomainComposerOutput, rules ValidationRules) DomainValidationResult {
	result := DomainValidationResult{
		Status:          ValidationStatusValid,
		AcceptedActions: []DomainAIAction{},
		Issues:          []ValidationIssue{},
	}

	if output.Domain != "" && output.Domain != domainCtx.Domain {
		result.Issues = append(result.Issues, ValidationIssue{
			Severity: IssueSeverityFatal,
			Code:     "domain_mismatch",
			Message:  fmt.Sprintf("expected domain %q, got %q", domainCtx.Domain, output.Domain),
		})
		result.Status = ValidationStatusInvalid
		return finalizeValidationResult(result)
	}

	candidateByID := make(map[string]ActionCandidate, len(domainCtx.Candidates))
	for _, c := range domainCtx.Candidates {
		candidateByID[c.CandidateID] = c
	}

	for i, action := range output.Actions {
		idx := i
		actionIssues := validateSingleAction(action, idx, domainCtx.Domain, candidateByID, rules)
		if len(actionIssues) == 0 {
			result.AcceptedActions = append(result.AcceptedActions, action)
			continue
		}

		fatal := false
		repairable := false
		var reasons []string
		for _, issue := range actionIssues {
			result.Issues = append(result.Issues, issue)
			reasons = append(reasons, issue.Code)
			switch issue.Severity {
			case IssueSeverityFatal:
				fatal = true
			case IssueSeverityRepairable:
				repairable = true
			}
		}
		if fatal {
			result.DroppedActions = append(result.DroppedActions, DroppedAction{
				Action:  action,
				Reasons: reasons,
			})
		} else if repairable {
			// repairable actions are not accepted until repaired
			result.DroppedActions = append(result.DroppedActions, DroppedAction{
				Action:  action,
				Reasons: reasons,
			})
		} else {
			result.AcceptedActions = append(result.AcceptedActions, action)
		}
	}

	return finalizeValidationResult(result)
}

func validateSingleAction(
	action DomainAIAction,
	idx int,
	domain string,
	candidateByID map[string]ActionCandidate,
	rules ValidationRules,
) []ValidationIssue {
	var issues []ValidationIssue

	add := func(severity, code, message, field string) {
		i := idx
		issues = append(issues, ValidationIssue{
			Severity:    severity,
			Code:        code,
			Message:     message,
			ActionIndex: &i,
			Field:       field,
		})
	}

	if len(action.SourceCandidateIDs) == 0 {
		add(IssueSeverityFatal, "missing_source_candidates", "source_candidate_ids is required", "source_candidate_ids")
	}
	if strings.TrimSpace(action.Title) == "" {
		add(IssueSeverityRepairable, "missing_title", "title is required", "title")
	}
	if strings.TrimSpace(action.Reason) == "" {
		add(IssueSeverityRepairable, "missing_reason", "reason is required", "reason")
	}
	if strings.TrimSpace(action.NextStep) == "" {
		add(IssueSeverityRepairable, "missing_next_step", "next_step is required", "next_step")
	}
	if !isValidActionType(action.Type) {
		add(IssueSeverityFatal, "invalid_action_type", fmt.Sprintf("unknown action type %q", action.Type), "type")
	} else if !typeAllowedInDomain(domain, action.Type) {
		add(IssueSeverityFatal, "action_type_domain_mismatch", fmt.Sprintf("type %q not allowed in domain %q", action.Type, domain), "type")
	}
	if !isValidCategory(action.Category) {
		add(IssueSeverityRepairable, "invalid_category", fmt.Sprintf("unknown category %q", action.Category), "category")
	}
	if !isValidPriority(action.Priority) {
		add(IssueSeverityRepairable, "invalid_priority", fmt.Sprintf("unknown priority %q", action.Priority), "priority")
	}

	if isGenericTitle(action.Title) {
		add(IssueSeverityRepairable, "generic_title", "title is too generic", "title")
	}
	if isGenericNextStep(action.NextStep) {
		add(IssueSeverityRepairable, "generic_next_step", "next_step is too generic", "next_step")
	}

	for _, field := range []struct {
		name string
		val  string
	}{
		{"title", action.Title},
		{"summary", action.Summary},
		{"reason", action.Reason},
		{"next_step", action.NextStep},
		{"expected_effect", action.ExpectedEffect},
	} {
		if pattern, ok := containsForbiddenClaim(field.val, rules.ForbiddenClaims); ok {
			add(IssueSeverityRepairable, "forbidden_claim", fmt.Sprintf("forbidden claim pattern %q in %s", pattern, field.name), field.name)
		}
	}

	if domain == ActionDomainAdvertising {
		for _, field := range []struct {
			name string
			val  string
		}{
			{"title", action.Title},
			{"summary", action.Summary},
			{"reason", action.Reason},
			{"next_step", action.NextStep},
		} {
			if advertisingSKULevelClaim(field.val) {
				add(IssueSeverityFatal, "advertising_sku_level_claim", "advertising actions must remain campaign-level", field.name)
			}
		}
	}

	// Candidate refs
	var sourceCandidates []ActionCandidate
	for _, cid := range action.SourceCandidateIDs {
		cand, ok := candidateByID[cid]
		if !ok {
			add(IssueSeverityFatal, "unknown_candidate_id", fmt.Sprintf("unknown candidate_id %q", cid), "source_candidate_ids")
			continue
		}
		if cand.Domain != domain {
			add(IssueSeverityFatal, "candidate_wrong_domain", fmt.Sprintf("candidate %q belongs to domain %q", cid, cand.Domain), "source_candidate_ids")
		}
		if cand.Type != action.Type {
			add(IssueSeverityFatal, "candidate_type_mismatch", fmt.Sprintf("candidate %q type %q does not match action type %q", cid, cand.Type, action.Type), "type")
		}
		sourceCandidates = append(sourceCandidates, cand)
	}

	allowedCTAs := unionAllowedCTAs(sourceCandidates)
	allowedFacts := collectFactsFromCandidates(sourceCandidates)
	allowedNumbers := collectAllowedNumbers(sourceCandidates)

	for _, factID := range action.EvidenceFactIDs {
		if _, ok := allowedFacts[factID]; !ok {
			add(IssueSeverityFatal, "unknown_fact_id", fmt.Sprintf("unknown evidence_fact_id %q", factID), "evidence_fact_ids")
		}
	}

	if len(action.EvidenceFactIDs) == 0 {
		if len(allowedFacts) > 0 {
			add(IssueSeverityRepairable, "missing_evidence_fact_ids", "evidence_fact_ids is required", "evidence_fact_ids")
		} else {
			add(IssueSeverityFatal, "missing_evidence_fact_ids", "evidence_fact_ids is required", "evidence_fact_ids")
		}
	}

	if len(action.CTAKeys) == 0 {
		if len(allowedCTAs) > 0 {
			add(IssueSeverityRepairable, "missing_cta_keys", "cta_keys is required", "cta_keys")
		} else {
			add(IssueSeverityFatal, "missing_cta_keys", "cta_keys is required", "cta_keys")
		}
	}

	for _, cta := range action.CTAKeys {
		if !allowedCTAs[cta] {
			add(IssueSeverityRepairable, "unknown_cta", fmt.Sprintf("unknown or disallowed cta_key %q", cta), "cta_keys")
		}
	}

	if isObjectBasedActionType(action.Type) {
		hasEntityGroup := false
		hasSKUGroup := false
		hasCampaignGroup := false
		hasAffectedPreview := false
		for _, c := range sourceCandidates {
			if c.EntityGroupRef != nil && strings.TrimSpace(c.EntityGroupRef.GroupKey) != "" {
				hasEntityGroup = true
				switch c.EntityGroupRef.EntityType {
				case "sku":
					hasSKUGroup = true
				case "campaign":
					hasCampaignGroup = true
				}
			}
			if len(c.TopExamples) > 0 {
				hasAffectedPreview = true
			}
		}
		if !hasEntityGroup && !hasAffectedPreview {
			add(IssueSeverityFatal, "missing_entity_group", "object-based action requires entity_group_ref or affected entities", "entity_group_ref")
		}
		if isSKUEntityGroupType(action.Type) && !hasSKUGroup && !hasAffectedPreview {
			add(IssueSeverityFatal, "missing_sku_entity_group", "SKU action requires sku entity group", "entity_group_ref")
		}
		if isCampaignEntityGroupType(action.Type) && !hasCampaignGroup && !hasAffectedPreview {
			add(IssueSeverityFatal, "missing_campaign_entity_group", "advertising action requires campaign entity group", "entity_group_ref")
		}
	}

	if action.Type == ActionTypeFunnelWeakTransition {
		hasTransition := false
		for _, c := range sourceCandidates {
			if c.Type != ActionTypeFunnelWeakTransition {
				continue
			}
			key := stringFromAgg(c.Aggregates, "transition_key")
			from := stringFromAgg(c.Aggregates, "from_stage")
			to := stringFromAgg(c.Aggregates, "to_stage")
			if key != "" || from != "" || to != "" {
				hasTransition = true
				break
			}
			if len(c.Facts) > 0 {
				hasTransition = true
				break
			}
		}
		if !hasTransition {
			add(IssueSeverityFatal, "missing_funnel_transition", "funnel action requires transition evidence", "affected_entities")
		}
	}

	for _, field := range []struct {
		name string
		val  string
	}{
		{"title", action.Title},
		{"summary", action.Summary},
		{"reason", action.Reason},
		{"next_step", action.NextStep},
		{"expected_effect", action.ExpectedEffect},
	} {
		for _, num := range extractNumbers(field.val) {
			if !numberAllowed(num, allowedNumbers) {
				add(IssueSeverityRepairable, "unsupported_number", fmt.Sprintf("unsupported number %q in %s", num, field.name), field.name)
				break
			}
		}
	}

	// Limitations
	for _, cand := range sourceCandidates {
		markers, ok := rules.RequiredLimitations[cand.Type]
		if !ok || len(cand.Limitations) == 0 {
			continue
		}
		combined := strings.ToLower(strings.Join(append(action.Limitations, action.Summary, action.Reason), " "))
		for _, marker := range markers {
			found := false
			for _, lim := range cand.Limitations {
				if strings.Contains(strings.ToLower(lim), strings.ToLower(marker)) {
					found = true
					break
				}
			}
			if found && !strings.Contains(combined, strings.ToLower(marker)) {
				add(IssueSeverityRepairable, "missing_limitation", fmt.Sprintf("missing required limitation marker %q", marker), "limitations")
			}
		}
		// Ensure at least one candidate limitation is preserved when present
		if len(cand.Limitations) > 0 && len(action.Limitations) == 0 {
			add(IssueSeverityRepairable, "missing_limitation", "action must preserve source candidate limitations", "limitations")
		}
	}

	return issues
}

func finalizeValidationResult(result DomainValidationResult) DomainValidationResult {
	hasFatal := false
	hasRepairable := false
	hasWarning := false
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
	case len(result.AcceptedActions) == 0 && hasFatal:
		result.Status = ValidationStatusInvalid
	case hasRepairable && len(result.DroppedActions) > 0:
		result.Status = ValidationStatusRepairable
	case hasWarning:
		result.Status = ValidationStatusValidWithWarnings
	case len(result.AcceptedActions) > 0:
		result.Status = ValidationStatusValid
	default:
		result.Status = ValidationStatusInvalid
	}
	return result
}

func unionAllowedCTAs(candidates []ActionCandidate) map[string]bool {
	out := make(map[string]bool)
	for _, c := range candidates {
		for _, cta := range c.AllowedCTAs {
			out[cta] = true
		}
	}
	return out
}

func collectFactsFromCandidates(candidates []ActionCandidate) map[string]ActionCandidateFact {
	out := make(map[string]ActionCandidateFact)
	for _, c := range candidates {
		for _, f := range c.Facts {
			out[f.FactID] = f
		}
	}
	return out
}

func collectAllowedNumbers(candidates []ActionCandidate) map[string]bool {
	out := make(map[string]bool)
	addNum := func(v string) {
		for _, n := range extractNumbers(v) {
			out[n] = true
			out[normalizeNumber(n)] = true
		}
	}
	for _, c := range candidates {
		addNum(c.Summary)
		for _, f := range c.Facts {
			addNum(f.Value)
			addNum(f.Label)
		}
		for k, v := range c.Aggregates {
			addNum(fmt.Sprint(v))
			addNum(k)
		}
	}
	return out
}

func extractNumbers(text string) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	text = collapseSpacedThousands(text)
	matches := numberPattern.FindAllString(text, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		out = append(out, normalizeNumber(m))
	}
	return out
}

func collapseSpacedThousands(text string) string {
	return spacedThousandsPattern.ReplaceAllStringFunc(text, func(s string) string {
		return strings.ReplaceAll(s, " ", "")
	})
}

func normalizeNumber(s string) string {
	return strings.ReplaceAll(strings.TrimSpace(s), ",", ".")
}

func numberAllowed(num string, allowed map[string]bool) bool {
	n := normalizeNumber(num)
	if allowed[n] {
		return true
	}
	// Allow small integers like 0-100 that might be percentages without explicit fact
	if f, err := strconv.ParseFloat(n, 64); err == nil {
		if f == float64(int64(f)) && f >= 0 && f <= 100 {
			// conservative: only allow if any allowed number matches as int
			for a := range allowed {
				if af, err := strconv.ParseFloat(a, 64); err == nil && af == f {
					return true
				}
			}
		}
	}
	return false
}

func shouldAttemptRepair(result DomainValidationResult, alwaysRun bool) bool {
	if alwaysRun && len(result.Issues) > 0 {
		return true
	}
	return result.Status == ValidationStatusRepairable
}
