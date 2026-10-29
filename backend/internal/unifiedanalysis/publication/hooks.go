package publication

// FailurePoint identifies a point inside PublishRun's transaction where a
// test may inject a synthetic failure via Service.WithTestFailurePoint, to
// exercise the atomicity/rollback and retry-recovery guarantees Stage 5
// publication depends on (see publication_integration_test.go
// TestFailureInjection_*).
//
// Every point below is checked via maybeFail at the corresponding place in
// service.go/rollback.go. Production Services are built with testFailAt =="
// " (the zero value), so maybeFail is always a no-op there.
type FailurePoint string

const (
	// FailAfterPublicationCreated fires right after the
	// unified_analysis_publications row is created/reused (getOrCreatePublicationRow),
	// before any dashboard/actions projection is persisted.
	FailAfterPublicationCreated FailurePoint = "after_publication_created"
	// FailAfterDashboardInserted fires after the dashboard narrative + frozen
	// snapshot are persisted (persistDashboard), before actions are persisted.
	FailAfterDashboardInserted FailurePoint = "after_dashboard_inserted"
	// FailAfterFirstActionInserted fires after the first action_items row is
	// upserted inside persistActions, while later actions are still pending.
	FailAfterFirstActionInserted FailurePoint = "after_first_action_inserted"
	// FailAfterAllActionsInserted fires after every projected action has been
	// upserted into action_items, before unified_publication_action_map rows
	// are (re-)written.
	FailAfterAllActionsInserted FailurePoint = "after_all_actions_inserted"
	// FailAfterActionMapInserted fires after all unified_publication_action_map
	// rows have been (re-)written, before the publication row is marked published.
	FailAfterActionMapInserted FailurePoint = "after_action_map_inserted"
	// FailAfterRunPublishing fires after the unified_analysis run's own status
	// has been established as "publishing" (whether transitioned this call or
	// already publishing from a previous attempt).
	FailAfterRunPublishing FailurePoint = "after_run_publishing"
	// FailAfterUnifiedPointer fires after seller_current_unified_publications
	// has been upserted to point at the new publication (visible publish only).
	FailAfterUnifiedPointer FailurePoint = "after_unified_pointer"
	// FailAfterCompatPointer fires after seller_current_ai_snapshots has been
	// upserted to point at the new run (visible publish only). PublishRun
	// writes the compat pointer BEFORE the unified pointer (see service.go),
	// so this point fires before FailAfterUnifiedPointer within the same call.
	FailAfterCompatPointer FailurePoint = "after_compat_pointer"
	// FailBeforeAudit fires immediately before the best-effort audit row is
	// written, after every authoritative state change for this publish.
	FailBeforeAudit FailurePoint = "before_audit"
	// FailBeforeCommit fires in withSellerTx after fn has returned
	// successfully but before the transaction is committed, so every write
	// fn performed is rolled back exactly as if fn itself had failed.
	FailBeforeCommit FailurePoint = "before_commit"
)

// WithTestFailurePoint returns a shallow clone of s configured so that
// maybeFail(p) returns a synthetic error the next time PublishRun (or
// withSellerTx) checks failure point p. It exists for tests only: pass ""
// to clear a previously-configured failure point, e.g. to retry a publish
// after asserting that the injected failure rolled back cleanly.
//
// Cloning (rather than mutating s in place) keeps this safe to use from
// table-driven/parallel tests that share a base Service built once per
// TestMain-style setup: each WithTestFailurePoint call produces an
// independent *Service pointing at the same pool/queries/artifacts.
func (s *Service) WithTestFailurePoint(p FailurePoint) *Service {
	if s == nil {
		return nil
	}
	clone := *s
	clone.testFailAt = p
	return &clone
}

// maybeFail returns a typed CodeTestInjectedFailure error when point matches
// the service's configured testFailAt, and nil otherwise. In production,
// testFailAt is always empty (NewService never sets it), so maybeFail is
// always nil there regardless of point.
func (s *Service) maybeFail(point FailurePoint) error {
	if s == nil || point == "" || s.testFailAt == "" || s.testFailAt != point {
		return nil
	}
	return Errorf(CodeTestInjectedFailure, "test-injected failure at %s", point)
}
