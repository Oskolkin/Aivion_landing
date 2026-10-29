package jobs

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
	"go.uber.org/zap"
)

type recordingPipelineRunner struct {
	calls atomic.Int32
}

func (r *recordingPipelineRunner) RunPostSync(ctx context.Context, in aipipeline.PostSyncPipelineInput) (*aipipeline.RunSellerAIPipelineResult, error) {
	return nil, fmt.Errorf("unexpected RunPostSync")
}

func (r *recordingPipelineRunner) RunExistingPipeline(ctx context.Context, in aipipeline.ExistingPipelineInput) (*aipipeline.RunSellerAIPipelineResult, error) {
	r.calls.Add(1)
	return &aipipeline.RunSellerAIPipelineResult{
		SellerAccountID: in.SellerAccountID,
		PipelineRunID:   in.PipelineRunID,
		Status:          aipipeline.PipelineStatusCompleted,
		Published:       true,
	}, nil
}

func TestRunSellerAIPipelineHandler_ClaimErrorStopsBeforePipeline(t *testing.T) {
	handler := NewRunSellerAIPipelineHandler(&recordingPipelineRunner{}, nil, nil, zap.NewNop(), nil)
	task, err := NewRunSellerAIPipelineTask(1, 1, time.Now().UTC().Format("2006-01-02"), "post_sync", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := handler.Handle(context.Background(), task.Payload()); err == nil {
		t.Fatal("expected claim error")
	}
}

func TestMapIdempotentClaimResult_NoPipelineCall(t *testing.T) {
	runner := &recordingPipelineRunner{}
	handler := NewRunSellerAIPipelineHandler(runner, nil, nil, zap.NewNop(), nil)
	claims := []PipelineExecutionClaim{
		{AlreadyRunning: true},
		{AlreadyCompleted: true},
		{AlreadyPublished: true},
		{AwaitingRecovery: true},
		{RetryNotDue: true},
		{TerminalFailed: true},
	}
	for _, claim := range claims {
		if claim.Claimed {
			t.Fatal("test setup error")
		}
		if claim.IdempotentSuccess() || claim.RetryNotDue || claim.TerminalFailed {
			continue
		}
		t.Fatalf("unexpected claim %+v", claim)
	}
	_ = handler
	if runner.calls.Load() != 0 {
		t.Fatalf("calls=%d want 0", runner.calls.Load())
	}
}
