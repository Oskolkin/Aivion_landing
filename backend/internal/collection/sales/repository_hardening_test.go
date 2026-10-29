package sales

import (
	"context"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
)

func TestAccountCoverageUsesObservedOuterBoundsAndCountsZero(t *testing.T) {
	d := func(day int) time.Time { return time.Date(2026, 8, day, 0, 0, 0, 0, time.UTC) }
	zero := accountCandidate{Date: d(6), Amount: Zero(), Units: 0}
	from, to := accountCoverage([]accountCandidate{{Date: d(8)}, zero, {Date: d(3)}})
	if from == nil || to == nil || !from.Equal(d(3)) || !to.Equal(d(8)) {
		t.Fatalf("coverage=%v..%v", from, to)
	}
	from, to = accountCoverage(nil)
	if from != nil || to != nil {
		t.Fatalf("empty coverage=%v..%v", from, to)
	}
}

func TestInterruptionHasStableMachineReadableCode(t *testing.T) {
	for _, err := range []error{context.Canceled, context.DeadlineExceeded, ozon.ErrNetworkError, &ozon.ProviderError{StatusCode: 503}} {
		if got := ErrorCode(err); got != "SALES_COLLECTION_INTERRUPTED" {
			t.Fatalf("error %v code=%s", err, got)
		}
	}
}
