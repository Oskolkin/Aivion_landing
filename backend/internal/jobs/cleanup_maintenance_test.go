package jobs

import "testing"

func TestNewCleanupMaintenanceTask(t *testing.T) {
	if _, err := NewCleanupMaintenanceTask(0); err == nil {
		t.Fatal("expected error for zero retention")
	}
	task, err := NewCleanupMaintenanceTask(30)
	if err != nil {
		t.Fatal(err)
	}
	if task.Type() != TaskTypeCleanupMaintenance {
		t.Fatalf("unexpected task type %q", task.Type())
	}
}
