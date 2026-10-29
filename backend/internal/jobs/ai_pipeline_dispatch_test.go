package jobs

import (
	"testing"
	"time"
)

func TestDispatchBackoff(t *testing.T) {
	cfg := DispatchConfig{
		BaseBackoff: time.Minute,
		MaxBackoff:  time.Hour,
	}
	if got := DispatchBackoff(1, cfg); got != time.Minute {
		t.Fatalf("attempt 1 = %v", got)
	}
	if got := DispatchBackoff(2, cfg); got != 2*time.Minute {
		t.Fatalf("attempt 2 = %v", got)
	}
	if got := DispatchBackoff(5, cfg); got != 16*time.Minute {
		t.Fatalf("attempt 5 = %v", got)
	}
	if got := DispatchBackoff(20, cfg); got != time.Hour {
		t.Fatalf("cap = %v", got)
	}
	if got := DispatchBackoff(0, cfg); got != time.Minute {
		t.Fatalf("zero attempt = %v", got)
	}
}

func TestSellerAIPipelineTaskID_Deterministic(t *testing.T) {
	a := SellerAIPipelineTaskID(7, 55)
	b := SellerAIPipelineTaskID(7, 55)
	if a != b || a != "seller-ai-pipeline:7:55" {
		t.Fatalf("task id = %q", a)
	}
}

func TestDispatchRecalculationDone(t *testing.T) {
	done := []string{
		DispatchStatusRecalculationCompleted,
		DispatchStatusEnqueuePending,
		DispatchStatusEnqueued,
		DispatchStatusPipelineStarted,
		DispatchStatusRetryPending,
		DispatchStatusCompleted,
	}
	for _, s := range done {
		if !dispatchRecalculationDone(s) {
			t.Fatalf("expected done: %s", s)
		}
	}
	if dispatchRecalculationDone(DispatchStatusPending) {
		t.Fatal("pending should not be done")
	}
}

func TestDispatchTerminal(t *testing.T) {
	if !dispatchTerminal(DispatchStatusCompleted) || !dispatchTerminal(DispatchStatusFailed) {
		t.Fatal("expected terminal")
	}
	if dispatchTerminal(DispatchStatusEnqueued) {
		t.Fatal("enqueued is not terminal")
	}
}

func TestIsTerminalEnqueueError(t *testing.T) {
	if !isTerminalEnqueueError(errString("invalid seller_account_id")) {
		t.Fatal("expected terminal")
	}
	if isTerminalEnqueueError(errString("redis timeout")) {
		t.Fatal("expected retryable")
	}
}

type errString string

func (e errString) Error() string { return string(e) }
