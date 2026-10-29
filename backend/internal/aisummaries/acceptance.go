package aisummaries

import (
	"fmt"
	"strings"
)

type SummaryAcceptanceOutcome struct {
	Accepted         bool
	Payload          *SummaryPayload
	Repair           SummaryRepairResult
	ValidationStatus string
	JSONRepairs      []string
}

// ReacceptSummaryPayload re-validates a stored payload at dashboard read time.
func ReacceptSummaryPayload(payload *SummaryPayload, ctx SummaryRepairContext) SummaryAcceptanceOutcome {
	outcome := SummaryAcceptanceOutcome{ValidationStatus: "failed"}
	if payload == nil {
		outcome.Repair.HardErrors = append(outcome.Repair.HardErrors, "nil_payload")
		return outcome
	}
	cloned := *payload
	repair := repairSummaryPayload(&cloned, ctx)
	outcome.Repair = repair
	outcome.Payload = &cloned

	remaining := validateSummaryPayload(&cloned)
	if ctx.SummaryType != "" {
		remaining = append(remaining, validateSummaryScopeSoft(ctx.SummaryType, ctx.ScopeID, &cloned)...)
	}
	hard, repairable := classifySummaryValidationErrors(remaining)
	outcome.Repair.HardErrors = append(outcome.Repair.HardErrors, hard...)
	outcome.Repair.RepairableErrors = append(outcome.Repair.RepairableErrors, repairable...)

	if len(outcome.Repair.HardErrors) > 0 || !hasUsableSummaryText(&cloned) {
		return outcome
	}
	outcome.Accepted = true
	outcome.ValidationStatus = validationStatusFromRepair(outcome.Repair)
	return outcome
}

func AcceptSummaryJSON(raw string, ctx SummaryRepairContext) (*SummaryAcceptanceOutcome, error) {
	recordSummaryGenerated()
	outcome := &SummaryAcceptanceOutcome{
		ValidationStatus: "failed",
	}

	repairedRaw, jsonRepairs, err := repairRawSummaryJSON(raw)
	outcome.JSONRepairs = jsonRepairs
	if err != nil {
		recordSummaryOutcome("failed", SummaryRepairResult{}, true, false)
		return nil, fmt.Errorf("json repair: %w", err)
	}
	if len(jsonRepairs) > 0 {
		outcome.Repair.Repaired = true
		outcome.Repair.RepairActions = append(outcome.Repair.RepairActions, jsonRepairs...)
	}

	categoryName := ctx.CategoryName
	if categoryName == "" && ctx.SummaryType == "category_daily_summary" {
		categoryName = ctx.CategoryName
	}
	payload, err := parseAndNormalizeSummaryJSONWithScope(repairedRaw, ctx.SummaryType, ctx.ScopeID, categoryName)
	if err != nil {
		recordSummaryOutcome("failed", SummaryRepairResult{}, true, false)
		return nil, fmt.Errorf("json: %w", err)
	}

	repair := repairSummaryPayload(payload, ctx)
	outcome.Repair = mergeRepairResult(outcome.Repair, repair)
	outcome.Payload = payload

	remaining := validateSummaryPayload(payload)
	if ctx.SummaryType != "" {
		remaining = append(remaining, validateSummaryScopeSoft(ctx.SummaryType, ctx.ScopeID, payload)...)
	}

	hard, repairable := classifySummaryValidationErrors(remaining)
	outcome.Repair.HardErrors = append(outcome.Repair.HardErrors, hard...)
	outcome.Repair.RepairableErrors = append(outcome.Repair.RepairableErrors, repairable...)

	if len(hard) > 0 {
		recordSummaryOutcome("failed", outcome.Repair, false, false)
		return outcome, nil
	}

	if !hasUsableSummaryText(payload) {
		outcome.Repair.HardErrors = append(outcome.Repair.HardErrors, "no_usable_summary_content")
		recordSummaryOutcome("failed", outcome.Repair, false, false)
		return outcome, nil
	}

	outcome.Accepted = true
	outcome.ValidationStatus = validationStatusFromRepair(outcome.Repair)
	recordSummaryOutcome(outcome.ValidationStatus, outcome.Repair, false, false)
	return outcome, nil
}

func validationStatusFromRepair(repair SummaryRepairResult) string {
	switch {
	case repair.Partial || repair.DroppedActions > 0 || len(repair.FallbackBlocks) > 0:
		return "completed_partial"
	case repair.Repaired || len(repair.RepairActions) > 0:
		return "completed_repaired"
	default:
		return "completed"
	}
}

func hasUsableSummaryText(p *SummaryPayload) bool {
	if p == nil {
		return false
	}
	return strings.TrimSpace(p.Headline) != "" || strings.TrimSpace(p.ExecutiveSummary) != ""
}

func classifySummaryValidationErrors(errs []string) (hard []string, repairable []string) {
	for _, err := range errs {
		if isHardSummaryValidationError(err) {
			hard = append(hard, err)
		} else {
			repairable = append(repairable, err)
		}
	}
	return hard, repairable
}

func isHardSummaryValidationError(err string) bool {
	switch err {
	case "nil payload", "missing_headline", "missing_executive_summary",
		"health_score_out_of_range", "invalid_severity", "invalid_data_richness", "invalid_summary_depth",
		"category_id_scope_mismatch", "account_summary_must_not_include_category_id",
		"account_summary_must_not_include_category_name", "nan_or_inf_in_payload",
		"category_summary_missing_category_id":
		return true
	default:
		return strings.HasPrefix(err, "funnel_ai_analysis_summary_too_long")
	}
}

// validateSummaryScopeSoft keeps only hard scope mismatches after repair.
func validateSummaryScopeSoft(summaryType, scopeID string, p *SummaryPayload) []string {
	var errs []string
	switch summaryType {
	case "account_daily_summary":
		if p.CategoryID != nil {
			errs = append(errs, "account_summary_must_not_include_category_id")
		}
		if p.CategoryName != nil && strings.TrimSpace(*p.CategoryName) != "" {
			errs = append(errs, "account_summary_must_not_include_category_name")
		}
	case "category_daily_summary":
		if p.CategoryID == nil {
			errs = append(errs, "category_summary_missing_category_id")
		} else if scopeID != "" && fmt.Sprint(*p.CategoryID) != scopeID {
			errs = append(errs, "category_id_scope_mismatch")
		}
	}
	return errs
}
