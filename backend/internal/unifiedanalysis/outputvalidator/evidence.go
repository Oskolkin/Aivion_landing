package outputvalidator

import (
	"fmt"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

// HasFact reports whether a fact_id resolves in the Pack facts registry.
// It classifies the fact_id by scope prefix:
//   - account:<metric>:<period> → resolved via the indexed account metrics
//   - sku:<id>[:...]            → resolved via the SKU id index
//   - campaign:<id>[:...]       → resolved via the campaign id index
//
// Fact ids with any other prefix (e.g. funnel:*) or a nil index are treated
// as "not disprovable" and return true, so the validator never rejects a
// fact scope it has no registry for. Use IsClassifiableFactID to decide
// whether an unresolved fact_id is a hard failure.
func (idx *PackReferenceIndex) HasFact(factID string) bool {
	if idx == nil {
		return true
	}
	scope, entity := splitFactID(factID)
	switch scope {
	case "account":
		if len(idx.accountMetrics) == 0 {
			return true
		}
		_, ok := idx.LookupAccountMetric(strings.TrimSpace(factID), "", "")
		return ok
	case "sku":
		return idx.HasSKU(entity)
	case "campaign":
		return idx.HasCampaign(entity)
	default:
		return true
	}
}

// IsClassifiableFactID reports whether the fact_id references a scope the
// registry can authoritatively verify (account/sku/campaign). Only
// classifiable-but-unresolved fact_ids are hard failures.
func IsClassifiableFactID(factID string) bool {
	scope, _ := splitFactID(factID)
	switch scope {
	case "account", "sku", "campaign":
		return true
	default:
		return false
	}
}

// splitFactID returns the scope prefix and the entity/metric token that
// follows it. For "sku:801000100002:orders_7d" it returns ("sku",
// "801000100002"); for "account:drr:current_7d" it returns ("account",
// "drr").
func splitFactID(factID string) (scope, entity string) {
	factID = strings.TrimSpace(factID)
	if factID == "" {
		return "", ""
	}
	parts := strings.SplitN(factID, ":", 3)
	if len(parts) < 2 {
		return strings.ToLower(parts[0]), ""
	}
	return strings.ToLower(strings.TrimSpace(parts[0])), strings.TrimSpace(parts[1])
}

// validateActionEvidence enforces the schema v8 evidence contract: every
// action must carry at least one evidence_fact_id, and every classifiable
// evidence fact_id must resolve in the Pack facts registry. Evidence facts
// that reference an entity of the same type as the action's scope but are
// not part of scope.entity_ids raise a (non-blocking) consistency warning.
func validateActionEvidence(report *Report, path string, action contract.UnifiedActionV1, idx *PackReferenceIndex) {
	nonEmpty := make([]string, 0, len(action.EvidenceFactIDs))
	seen := map[string]struct{}{}
	for _, f := range action.EvidenceFactIDs {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		if _, ok := seen[f]; ok {
			continue
		}
		seen[f] = struct{}{}
		nonEmpty = append(nonEmpty, f)
	}
	if len(nonEmpty) == 0 {
		report.addError(CodeActionEvidenceMissing, path+".evidence_fact_ids",
			"action must cite at least one evidence_fact_id from the Pack facts registry")
		return
	}
	for i, f := range nonEmpty {
		fp := fmt.Sprintf("%s.evidence_fact_ids[%d]", path, i)
		if idx != nil && IsClassifiableFactID(f) && !idx.HasFact(f) {
			report.addError(CodeActionEvidenceFactUnknown, fp,
				fmt.Sprintf("evidence_fact_id %q does not resolve in the Pack facts registry", f))
		}
	}
	validateEvidenceScopeConsistency(report, path, action, nonEmpty)
}

func validateEvidenceScopeConsistency(report *Report, path string, action contract.UnifiedActionV1, evidence []string) {
	scopeType := strings.TrimSpace(action.Scope.ScopeType)
	if scopeType != "sku" && scopeType != "campaign" {
		return
	}
	inScope := map[string]struct{}{}
	for _, id := range action.Scope.EntityIDs {
		id = strings.TrimSpace(id)
		id = strings.TrimPrefix(id, scopeType+":")
		if id != "" {
			inScope[id] = struct{}{}
		}
	}
	for i, f := range evidence {
		scope, entity := splitFactID(f)
		if scope != scopeType || entity == "" {
			continue
		}
		if _, ok := inScope[entity]; !ok {
			report.addWarning(CodeActionEvidenceScopeInconsistent,
				fmt.Sprintf("%s.evidence_fact_ids[%d]", path, i),
				fmt.Sprintf("evidence %s references %s entity %q outside the action scope", f, scopeType, entity))
		}
	}
}

// validateCompletionConditions enforces the schema v8 completion-condition
// contract for machine-measurable conditions: a loadable baseline
// (baseline_fact_id + baseline_value), a fully specified target
// (metric_field + operator + target_value), a target that is NOT already
// satisfied by the baseline, and consistency between the machine spec and
// the text completion_criteria.
func validateCompletionConditions(report *Report, path string, action contract.UnifiedActionV1, idx *PackReferenceIndex) {
	for i, c := range action.CompletionConditions {
		cp := fmt.Sprintf("%s.completion_conditions[%d]", path, i)
		measurable := strings.TrimSpace(c.Operator) != "" || c.TargetValue != nil ||
			strings.TrimSpace(c.MetricField) != "" || c.BaselineValue != nil ||
			strings.TrimSpace(c.BaselineFactID) != ""
		if !measurable {
			continue
		}

		// A measurable condition must fully specify its target and stay
		// consistent with the text criterion.
		if strings.TrimSpace(c.MetricField) == "" || strings.TrimSpace(c.Operator) == "" || c.TargetValue == nil {
			report.addError(CodeActionCompletionTextMachineMismatch, cp,
				"machine completion condition must set metric_field, operator and target_value to match the text criterion")
		}
		if strings.TrimSpace(action.CompletionCriteria) == "" {
			report.addError(CodeActionCompletionTextMachineMismatch, path+".completion_criteria",
				"machine completion conditions require a non-empty completion_criteria text")
		}

		// Baseline must be loadable: an explicit fact_id + value that
		// resolves in the Pack facts registry.
		if strings.TrimSpace(c.BaselineFactID) == "" || c.BaselineValue == nil {
			report.addError(CodeActionCompletionBaselineMissing, cp,
				"completion condition must set baseline_fact_id and baseline_value")
		} else if idx != nil && IsClassifiableFactID(c.BaselineFactID) && !idx.HasFact(c.BaselineFactID) {
			report.addError(CodeActionCompletionBaselineMissing, cp+".baseline_fact_id",
				fmt.Sprintf("baseline_fact_id %q does not resolve in the Pack facts registry", c.BaselineFactID))
		}

		// A condition that is already satisfied at analysis time is not a
		// real completion target.
		if c.BaselineValue != nil && c.TargetValue != nil && strings.TrimSpace(c.Operator) != "" {
			if conditionAlreadyMet(*c.BaselineValue, c.Operator, *c.TargetValue) {
				report.addError(CodeActionCompletionTargetAlreadyMet, cp,
					fmt.Sprintf("baseline %.4g already satisfies operator %q target %.4g", *c.BaselineValue, c.Operator, *c.TargetValue))
			}
		}
	}
}

// conditionAlreadyMet reports whether baseline already satisfies the target
// under the given operator.
func conditionAlreadyMet(baseline float64, operator string, target float64) bool {
	switch normalizeOperator(operator) {
	case ">=":
		return baseline >= target
	case ">":
		return baseline > target
	case "<=":
		return baseline <= target
	case "<":
		return baseline < target
	case "==":
		return baseline == target
	default:
		return false
	}
}

func normalizeOperator(op string) string {
	switch strings.ToLower(strings.TrimSpace(op)) {
	case ">=", "gte", "ge", "at_least", "min":
		return ">="
	case ">", "gt", "greater", "above":
		return ">"
	case "<=", "lte", "le", "at_most", "max":
		return "<="
	case "<", "lt", "less", "below":
		return "<"
	case "==", "=", "eq", "equal", "equals":
		return "=="
	default:
		return ""
	}
}
