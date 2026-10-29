package unifiedanalysis

import (
	"context"
	"fmt"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/provider"
)

// CancelRun requests cancellation and finalizes the run when possible.
func (s *Service) CancelRun(ctx context.Context, runID int64) error {
	if s == nil || s.repo == nil {
		return fmt.Errorf("unified analysis service is not configured")
	}

	run, err := s.repo.GetRun(ctx, runID)
	if err != nil {
		return err
	}
	if run.Status == StatusCancelled || run.Status == StatusFailed {
		return nil
	}
	if run.Status == StatusProviderCompleted {
		return nil
	}

	run, err = s.repo.RequestCancellation(ctx, runID)
	if err != nil {
		return err
	}

	responseID := strings.TrimSpace(run.ProviderResponseID)
	if responseID == "" {
		_, err = s.repo.MarkCancelled(ctx, runID, dbgen.MarkUnifiedAnalysisCancelledParams{
			CleanupStatus: nullableText(CleanupStatusPending),
		})
		if err != nil {
			return err
		}
		if s.cfg.CleanupEnabled {
			return s.CleanupRun(ctx, runID)
		}
		return nil
	}

	if s.provider == nil {
		return errorf(CodeProviderDisabled, "provider is not configured")
	}

	switch run.Status {
	case StatusProviderRunning, StatusProviderResponseStarting:
		if _, err := s.provider.CancelResponse(ctx, responseID); err != nil {
			return wrapError(CodeCancellationFailed, "cancel provider response", err)
		}
	}

	_, err = s.repo.MarkCancelled(ctx, runID, dbgen.MarkUnifiedAnalysisCancelledParams{
		ProviderStatus: nullableText(string(provider.ProviderResponseStatusCancelled)),
		CleanupStatus:  nullableText(CleanupStatusPending),
	})
	if err != nil {
		return err
	}
	if s.cfg.CleanupEnabled {
		return s.CleanupRun(ctx, runID)
	}
	return nil
}
