package aisummaries

import "testing"

func TestEncodeDecodeStoredValidationNotes(t *testing.T) {
	raw := encodeStoredValidationNotes("completed_repaired", SummaryRepairResult{
		RepairableErrors: []string{"key_risk_filled_from_context"},
		RepairActions:    []string{"headline_filled_from_executive_summary"},
	})
	status, notes := decodeStoredValidationNotes(raw)
	if status != "completed_repaired" {
		t.Fatalf("status=%q", status)
	}
	if len(notes) != 2 {
		t.Fatalf("notes=%v", notes)
	}
}

func TestDeriveAcceptanceStatus_fromStoredMarker(t *testing.T) {
	raw := encodeStoredValidationNotes("completed_partial", SummaryRepairResult{
		RepairableErrors: []string{"action_0_forbidden_recommendation"},
	})
	status := DeriveAcceptanceStatus("account_daily_summary", "", "", "completed", raw, nil)
	if status != "completed_partial" {
		t.Fatalf("status=%q", status)
	}
}
