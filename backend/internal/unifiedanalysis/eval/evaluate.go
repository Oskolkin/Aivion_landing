package eval

import (
	"fmt"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
)

// Evaluate checks output against ExpectedChecks and returns a pass/fail report.
// It performs semantic checks only; it does not validate the technical contract.
func Evaluate(output *contract.UnifiedAnalysisOutputV1, checks ExpectedChecks) EvalReport {
	var report EvalReport
	if output == nil {
		report.Failures = append(report.Failures, EvalFailure{
			Check:   "output_present",
			Message: "output is nil",
		})
		report.Passed = false
		return report
	}

	if checks.Outcome != "" && output.AnalysisOutcome != checks.Outcome {
		report.addFailure("outcome", fmt.Sprintf("got %q want %q", output.AnalysisOutcome, checks.Outcome))
	}

	categories := actionCategories(output.Actions)
	skuRefs, campaignRefs := entityRefs(output.Actions)

	for _, cat := range checks.RequiredCategories {
		if !categories[cat] {
			report.addFailure("required_category", fmt.Sprintf("missing category %q in actions", cat))
		}
	}
	for _, sku := range checks.RequiredSKUIDs {
		norm := normalizeSKURef(sku)
		if !skuRefs[norm] {
			report.addFailure("required_sku", fmt.Sprintf("missing sku %q in action entity_ids", norm))
		}
	}
	for _, camp := range checks.RequiredCampaignIDs {
		norm := normalizeCampaignRef(camp)
		if !campaignRefs[norm] {
			report.addFailure("required_campaign", fmt.Sprintf("missing campaign %q in action entity_ids", norm))
		}
	}

	if checks.RequireTopAction && len(output.Dashboard.TopActionIDs) == 0 {
		report.addFailure("require_top_action", "dashboard.top_action_ids is empty")
	}

	for _, cat := range checks.ForbiddenCategories {
		if categories[cat] {
			report.addFailure("forbidden_category", fmt.Sprintf("forbidden category %q present in actions", cat))
		}
	}
	for _, sku := range checks.ForbiddenSKUIDs {
		norm := normalizeSKURef(sku)
		if skuRefs[norm] {
			report.addFailure("forbidden_sku", fmt.Sprintf("forbidden sku %q present in action entity_ids", norm))
		}
	}
	for _, camp := range checks.ForbiddenCampaignIDs {
		norm := normalizeCampaignRef(camp)
		if campaignRefs[norm] {
			report.addFailure("forbidden_campaign", fmt.Sprintf("forbidden campaign %q present in action entity_ids", norm))
		}
	}

	if checks.MaxActions != nil && len(output.Actions) > *checks.MaxActions {
		report.addFailure("max_actions", fmt.Sprintf("actions=%d max=%d", len(output.Actions), *checks.MaxActions))
	}

	if checks.NoDuplicateSKU {
		owner := map[string]string{}
		for _, action := range output.Actions {
			id := strings.TrimSpace(action.AnalysisActionID)
			for _, entityID := range action.Scope.EntityIDs {
				entityID = strings.TrimSpace(entityID)
				if !strings.HasPrefix(entityID, "sku:") {
					continue
				}
				if prev, ok := owner[entityID]; ok && prev != id {
					report.addFailure("no_duplicate_sku", fmt.Sprintf("sku %q used in actions %q and %q", entityID, prev, id))
				} else if id != "" {
					owner[entityID] = id
				}
			}
			for _, step := range action.Steps {
				for _, entityID := range step.EntityIDs {
					entityID = strings.TrimSpace(entityID)
					if !strings.HasPrefix(entityID, "sku:") {
						continue
					}
					if prev, ok := owner[entityID]; ok && prev != id {
						report.addFailure("no_duplicate_sku", fmt.Sprintf("sku %q used in actions %q and %q", entityID, prev, id))
					} else if id != "" {
						owner[entityID] = id
					}
				}
			}
		}
	}

	report.Passed = len(report.Failures) == 0
	return report
}

func (r *EvalReport) addFailure(check, message string) {
	r.Failures = append(r.Failures, EvalFailure{Check: check, Message: message})
}

func actionCategories(actions []contract.UnifiedActionV1) map[string]bool {
	out := make(map[string]bool, len(actions))
	for _, a := range actions {
		cat := strings.TrimSpace(a.Category)
		if cat != "" {
			out[cat] = true
		}
	}
	return out
}

func entityRefs(actions []contract.UnifiedActionV1) (skus, campaigns map[string]bool) {
	skus = make(map[string]bool)
	campaigns = make(map[string]bool)
	for _, a := range actions {
		for _, id := range a.Scope.EntityIDs {
			id = strings.TrimSpace(id)
			if strings.HasPrefix(id, "sku:") {
				skus[id] = true
			}
			if strings.HasPrefix(id, "campaign:") {
				campaigns[id] = true
			}
		}
		for _, step := range a.Steps {
			for _, id := range step.EntityIDs {
				id = strings.TrimSpace(id)
				if strings.HasPrefix(id, "sku:") {
					skus[id] = true
				}
				if strings.HasPrefix(id, "campaign:") {
					campaigns[id] = true
				}
			}
		}
	}
	return skus, campaigns
}

func normalizeSKURef(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ref
	}
	if strings.HasPrefix(ref, "sku:") {
		return ref
	}
	return "sku:" + ref
}

func normalizeCampaignRef(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ref
	}
	if strings.HasPrefix(ref, "campaign:") {
		return ref
	}
	return "campaign:" + ref
}
