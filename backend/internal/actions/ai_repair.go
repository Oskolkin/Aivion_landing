package actions

// Repair flow is orchestrated in processDomain (ai_composer.go) via ActionAIClient.RepairDomainActions.
// This file documents repair eligibility helpers used by the pipeline.

func countRepairableIssues(issues []ValidationIssue) int {
	n := 0
	for _, i := range issues {
		if i.Severity == IssueSeverityRepairable {
			n++
		}
	}
	return n
}
