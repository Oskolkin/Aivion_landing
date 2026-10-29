package jobs

import (
	"context"
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
)

type dispatchLeaseGuard struct {
	svc        *AIPipelineDispatchService
	dispatchID int64
	leaseToken string
}

func (g dispatchLeaseGuard) ValidateLease(ctx context.Context, dispatchID, pipelineRunID int64) error {
	if g.svc == nil {
		return fmt.Errorf("dispatch lease guard is not configured")
	}
	if dispatchID != g.dispatchID {
		return fmt.Errorf("dispatch_id mismatch")
	}
	return g.svc.ValidateExecutionLease(ctx, dispatchID, pipelineRunID, g.leaseToken)
}

func (g dispatchLeaseGuard) RenewLease(ctx context.Context, dispatchID int64) error {
	if g.svc == nil {
		return fmt.Errorf("dispatch lease guard is not configured")
	}
	if dispatchID != g.dispatchID {
		return fmt.Errorf("dispatch_id mismatch")
	}
	return g.svc.RenewExecutionLease(ctx, dispatchID, g.leaseToken)
}

var _ aipipeline.ExecutionLeaseGuard = dispatchLeaseGuard{}
