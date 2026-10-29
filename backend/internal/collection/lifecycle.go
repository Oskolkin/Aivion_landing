package collection

import "fmt"

type InvalidTransitionError struct{ From, To string }

func (e *InvalidTransitionError) Error() string {
	return fmt.Sprintf("invalid collection lifecycle transition: %s -> %s", e.From, e.To)
}

type StaleTransitionError struct {
	EntityKind string
	EntityID   int64
	Expected   string
	Requested  string
}

func (e *StaleTransitionError) Error() string {
	return fmt.Sprintf("stale %s lifecycle transition for id=%d: expected %s, requested %s", e.EntityKind, e.EntityID, e.Expected, e.Requested)
}

func ValidateRunTransition(from, to CollectionRunStatus) error {
	if from == CollectionRunStatusPending && (to == CollectionRunStatusRunning || to == CollectionRunStatusCancelled) {
		return nil
	}
	if from == CollectionRunStatusRunning && (to == CollectionRunStatusCompleted || to == CollectionRunStatusCompletedWithLimitations || to == CollectionRunStatusFailed || to == CollectionRunStatusCancelled) {
		return nil
	}
	return &InvalidTransitionError{From: string(from), To: string(to)}
}
func ValidateDomainTransition(from, to DomainRunStatus) error {
	if from == DomainRunStatusPending && (to == DomainRunStatusRunning || to == DomainRunStatusCancelled || to == DomainRunStatusUnavailable || to == DomainRunStatusNotApplicable) {
		return nil
	}
	if from == DomainRunStatusRunning && (to == DomainRunStatusCompleted || to == DomainRunStatusPartial || to == DomainRunStatusFailed || to == DomainRunStatusCancelled) {
		return nil
	}
	return &InvalidTransitionError{From: string(from), To: string(to)}
}
