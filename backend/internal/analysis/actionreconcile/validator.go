package actionreconcile

func ValidatePlan(plan PlanV1, proposalCount, previousCount int) ReportV1 {
	r := ReportV1{SchemaVersion: ReportSchemaV1, Errors: []string{}}
	if len(plan.NewProposalIntents) != proposalCount {
		r.Errors = append(r.Errors, "DISPOSITION_COUNT")
	}
	if len(plan.PreviousOutcomes) != previousCount {
		r.Errors = append(r.Errors, "PREVIOUS_OUTCOME_COUNT")
	}
	r.Valid = len(r.Errors) == 0
	return r
}
