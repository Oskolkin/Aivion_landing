package runtime

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/policy"
	analysispg "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/postgres"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/stages"
)

func TestPrompt16FailureInjectionClassification(t *testing.T) {
	if classify(context.DeadlineExceeded) != policy.Timeout {
		t.Fatal("timeout")
	}
	if classify(analysispg.ErrLeaseLost) != policy.LeaseExpired {
		t.Fatal("lease")
	}
	if classify(errors.New("boom")) != policy.TransientDependency {
		t.Fatal("transient")
	}
}

func TestPrompt16NoOpDetectorType(t *testing.T) {
	_ = NoOpDetector{}
	_ = time.Second
	_ = stages.DiscoveryAI
}
