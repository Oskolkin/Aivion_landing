package aisummaries

import (
	"encoding/json"
	"strings"
)

// DB row status (ai_daily_summaries.status) is operational: pending | running | completed | failed | not_enough_data.
// Acceptance status (completed | completed_repaired | completed_partial | failed) is stored inside
// validation_errors JSON as the first namespaced entry acceptance_status:<value> and recomputed on read
// via ReacceptSummaryPayload when needed.

const acceptanceStatusPrefix = "acceptance_status:"

func encodeStoredValidationNotes(acceptanceStatus string, repair SummaryRepairResult) []byte {
	if acceptanceStatus == "" {
		acceptanceStatus = "failed"
	}
	notes := []string{acceptanceStatusPrefix + acceptanceStatus}
	notes = append(notes, repair.RepairableErrors...)
	notes = append(notes, repair.RepairActions...)
	if len(notes) == 1 {
		return nil
	}
	b, _ := json.Marshal(notes)
	return b
}

func decodeStoredValidationNotes(raw []byte) (acceptanceStatus string, repairable []string) {
	if len(raw) == 0 {
		return "", nil
	}
	var notes []string
	if err := json.Unmarshal(raw, &notes); err != nil {
		return "", nil
	}
	for _, note := range notes {
		if strings.HasPrefix(note, acceptanceStatusPrefix) {
			acceptanceStatus = strings.TrimPrefix(note, acceptanceStatusPrefix)
			continue
		}
		repairable = append(repairable, note)
	}
	return acceptanceStatus, repairable
}

// DeriveAcceptanceStatus returns stored acceptance_status or recomputes from payload.
func DeriveAcceptanceStatus(rowSummaryType, scopeID, categoryName string, status string, validationErrors []byte, payload *SummaryPayload) string {
	if status == "failed" {
		return "failed"
	}
	if stored, _ := decodeStoredValidationNotes(validationErrors); stored != "" {
		return stored
	}
	if payload == nil || status != "completed" {
		return status
	}
	ctx := SummaryRepairContext{SummaryType: rowSummaryType, ScopeID: scopeID, CategoryName: categoryName}
	outcome := ReacceptSummaryPayload(payload, ctx)
	if outcome.Accepted {
		return outcome.ValidationStatus
	}
	return "failed"
}
