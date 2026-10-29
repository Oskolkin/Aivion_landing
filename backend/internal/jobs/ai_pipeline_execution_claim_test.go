package jobs

import (
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestClassifyDispatchClaim_AlreadyCompleted(t *testing.T) {
	claim := classifyDispatchClaim(dbgen.AiPipelineDispatch{Status: DispatchStatusCompleted}, time.Now(), ClaimSourceHandler)
	if !claim.AlreadyCompleted {
		t.Fatal("expected already completed")
	}
}

func TestClassifyDispatchClaim_FreshLeaseAlreadyRunning(t *testing.T) {
	now := time.Now().UTC()
	claim := classifyDispatchClaim(dbgen.AiPipelineDispatch{
		Status:              DispatchStatusPipelineStarted,
		ExecutionLeaseToken: pgtype.Text{String: "lease", Valid: true},
		LeaseExpiresAt:      pgTimestamptz(now.Add(time.Hour)),
	}, now, ClaimSourceHandler)
	if !claim.AlreadyRunning {
		t.Fatal("expected already running")
	}
}

func TestClassifyDispatchClaim_StaleLeaseAwaitingRecovery(t *testing.T) {
	now := time.Now().UTC()
	claim := classifyDispatchClaim(dbgen.AiPipelineDispatch{
		Status:              DispatchStatusPipelineStarted,
		ExecutionLeaseToken: pgtype.Text{String: "lease", Valid: true},
		LeaseExpiresAt:      pgTimestamptz(now.Add(-time.Minute)),
	}, now, ClaimSourceHandler)
	if !claim.AwaitingRecovery {
		t.Fatal("expected awaiting recovery")
	}
}

func TestClassifyDispatchClaim_RetryNotDue(t *testing.T) {
	now := time.Now().UTC()
	claim := classifyDispatchClaim(dbgen.AiPipelineDispatch{
		Status:        DispatchStatusRetryPending,
		NextAttemptAt: pgTimestamptz(now.Add(time.Hour)),
	}, now, ClaimSourceHandler)
	if !claim.RetryNotDue {
		t.Fatal("expected retry not due")
	}
}

func TestPipelineExecutionClaim_IdempotentSuccess(t *testing.T) {
	cases := []PipelineExecutionClaim{
		{AlreadyRunning: true},
		{AlreadyCompleted: true},
		{AlreadyPublished: true},
		{AwaitingRecovery: true},
	}
	for _, c := range cases {
		if !c.IdempotentSuccess() {
			t.Fatalf("expected idempotent success for %+v", c)
		}
	}
	if (PipelineExecutionClaim{Claimed: true}).IdempotentSuccess() {
		t.Fatal("claimed should not be idempotent success")
	}
}

func TestLeaseHash_DoesNotExposeToken(t *testing.T) {
	a := LeaseHash("secret-token-value")
	b := LeaseHash("secret-token-value")
	c := LeaseHash("other-token")
	if a == "" || a != b || a == c {
		t.Fatalf("hash mismatch a=%q b=%q c=%q", a, b, c)
	}
}
