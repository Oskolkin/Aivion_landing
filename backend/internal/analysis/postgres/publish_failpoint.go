package postgres

import "errors"

// PublishFailpoint is a test-only injection hook for rollback verification.
type PublishFailpoint string

const (
	FailpointAfterPublicationInsert PublishFailpoint = "AFTER_PUBLICATION_INSERT"
	FailpointAfterActionMutations   PublishFailpoint = "AFTER_ACTION_MUTATIONS"
	FailpointAfterProjections       PublishFailpoint = "AFTER_PROJECTIONS"
	FailpointBeforeCurrentPointer   PublishFailpoint = "BEFORE_CURRENT_POINTER"
)

var ErrPublishFailpoint = errors.New("PUBLISH_FAILPOINT")

// TestPublishFailpoint is set by integration tests; never set in production.
var TestPublishFailpoint PublishFailpoint

// TestPublishUnknownCommitAfterSuccess simulates transport loss after COMMIT.
// When true, PublishAtomic commits successfully then returns ErrPublishUnknownCommit.
var TestPublishUnknownCommitAfterSuccess bool

var ErrPublishUnknownCommit = errors.New("PUBLISH_UNKNOWN_COMMIT")

// TestOutboxFailNext forces the next ProcessOutbox delivery attempt to fail once.
var TestOutboxFailNext bool

// TestOutboxFailSellerID when non-zero scopes TestOutboxFailNext to that seller's events.
var TestOutboxFailSellerID int64

var ErrOutboxConsumer = errors.New("OUTBOX_CONSUMER_FAIL")

// TestOutboxSideEffectCount counts idempotent business side-effects (test-only).
var TestOutboxSideEffectCount int

// TestOutboxDeliveryAttemptCount counts delivery attempts including retries (test-only).
var TestOutboxDeliveryAttemptCount int

// TestOutboxProcessedAggregates tracks business-effect idempotency keys (test-only).
var TestOutboxProcessedAggregates map[string]bool

// ResetOutboxTestCounters clears test-only outbox counters before a scenario.
// TestOutboxSellerFilter when non-zero limits ProcessOutbox to events for that seller.
var TestOutboxSellerFilter int64

func ResetOutboxTestCounters() {
	TestOutboxSideEffectCount = 0
	TestOutboxDeliveryAttemptCount = 0
	TestOutboxProcessedAggregates = make(map[string]bool)
}

func maybeFailpoint(fp PublishFailpoint) error {
	if TestPublishFailpoint == fp {
		return ErrPublishFailpoint
	}
	return nil
}
