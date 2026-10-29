package publishedsnapshot

import (
	"testing"
)

func TestProtectedSnapshotIDs_ContainsCurrentRows(t *testing.T) {
	protected := ProtectedSnapshotIDs{
		PipelineRunID:  10,
		DashboardRunID: "dash-1",
		ActionsRunID:   20,
	}
	if !protected.ContainsPipelineRun(10) {
		t.Fatal("pipeline must be protected")
	}
	if !protected.ContainsDashboardRun("dash-1") {
		t.Fatal("dashboard must be protected")
	}
	if !protected.ContainsActionsRun(20) {
		t.Fatal("actions run must be protected")
	}
	if protected.ContainsPipelineRun(11) || protected.ContainsActionsRun(21) {
		t.Fatal("unrelated ids must not be protected")
	}
}
