package aipipeline

import (
	"context"
	"time"
)

// ExecutionLeaseGuard validates and renews dispatch execution leases during pipeline stages.
type ExecutionLeaseGuard interface {
	ValidateLease(ctx context.Context, dispatchID, pipelineRunID int64) error
	RenewLease(ctx context.Context, dispatchID int64) error
}

// ExistingPipelineInput executes a pre-claimed logical pipeline run.
type ExistingPipelineInput struct {
	SellerAccountID int64
	SyncJobID       int64
	MetricDate      time.Time
	Trigger         string
	Force           bool
	DispatchID      int64
	PipelineRunID   int64
	LeaseGuard      ExecutionLeaseGuard
}
