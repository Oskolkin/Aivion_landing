package collection

import (
	"errors"
	"testing"
)

func TestRunLifecycleTransitions(t *testing.T) {
	if err := ValidateRunTransition(CollectionRunStatusPending, CollectionRunStatusRunning); err != nil {
		t.Fatal(err)
	}
	if err := ValidateRunTransition(CollectionRunStatusRunning, CollectionRunStatusCompleted); err != nil {
		t.Fatal(err)
	}
	err := ValidateRunTransition(CollectionRunStatusCompleted, CollectionRunStatusRunning)
	var transition *InvalidTransitionError
	if !errors.As(err, &transition) {
		t.Fatalf("err=%v, want typed invalid transition", err)
	}
}
func TestDomainLifecycleTransitions(t *testing.T) {
	if err := ValidateDomainTransition(DomainRunStatusPending, DomainRunStatusUnavailable); err != nil {
		t.Fatal(err)
	}
	if err := ValidateDomainTransition(DomainRunStatusRunning, DomainRunStatusPartial); err != nil {
		t.Fatal(err)
	}
	if err := ValidateDomainTransition(DomainRunStatusFailed, DomainRunStatusRunning); err == nil {
		t.Fatal("expected terminal transition rejection")
	}
}
