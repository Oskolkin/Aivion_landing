package aipipeline

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestAdminDiagnosticsIncludesPipelineTrace(t *testing.T) {
	run := dbgen.AiPipelineRun{
		ID:                24,
		SellerAccountID:   1,
		DataPackHash:      "sha256:abc",
		GlobalContextHash: pgtype.Text{String: "sha256:ctx", Valid: true},
		DashboardStatus:   pgtype.Text{String: "completed", Valid: true},
		ActionsStatus:     pgtype.Text{String: "completed", Valid: true},
		ActionsRunID:      pgtype.Int8{Int64: 456, Valid: true},
	}
	artifacts := []dbgen.AiPipelineArtifact{
		{Kind: "data_pack"},
		{Kind: "actions_output"},
	}
	diag := BuildAdminPipelineDiagnostics(run, artifacts, DashboardTopActionsDiag{
		Source:       "actions_final",
		ActionIDs:    []int64{101, 102},
		ActionsRunID: 456,
	})
	if diag.PipelineRunID != 24 || diag.ActionsRunID != 456 {
		t.Fatalf("pipeline trace missing: %+v", diag)
	}
	if diag.DataPackHash == "" || diag.GlobalContextHash == "" {
		t.Fatalf("hashes missing: %+v", diag)
	}
	if diag.DashboardTopActions.Source != "actions_final" {
		t.Fatalf("top actions source=%q", diag.DashboardTopActions.Source)
	}
}

func TestAdminDiagnosticsNoReviewerFields(t *testing.T) {
	run := dbgen.AiPipelineRun{ID: 1, SellerAccountID: 1, DataPackHash: "h"}
	diag := BuildAdminPipelineDiagnostics(run, nil, DashboardTopActionsDiag{})
	rawBytes, err := json.Marshal(diag)
	if err != nil {
		t.Fatal(err)
	}
	raw := string(rawBytes)
	for _, forbidden := range []string{"reviewer", "consistency_review", "patch"} {
		if strings.Contains(strings.ToLower(raw), forbidden) {
			t.Fatalf("forbidden %q in diagnostics", forbidden)
		}
	}
}

func TestAdminDiagnosticsMasksProxyCredentials(t *testing.T) {
	run := dbgen.AiPipelineRun{ID: 1, SellerAccountID: 1, DataPackHash: "h"}
	diag := BuildAdminPipelineDiagnostics(run, nil, DashboardTopActionsDiag{})
	rawBytes, err := json.Marshal(diag)
	if err != nil {
		t.Fatal(err)
	}
	raw := string(rawBytes)
	if strings.Contains(raw, "password") || strings.Contains(raw, "api_key") {
		t.Fatalf("proxy diagnostics must not leak credentials: %s", raw)
	}
}
