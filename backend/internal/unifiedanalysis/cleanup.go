package unifiedanalysis

import (
	"context"
	"fmt"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/provider"
)

// CleanupRun deletes provider container resources when enabled.
func (s *Service) CleanupRun(ctx context.Context, runID int64) error {
	if s == nil || s.repo == nil {
		return fmt.Errorf("unified analysis service is not configured")
	}
	if !s.cfg.CleanupEnabled {
		return nil
	}
	if s.provider == nil {
		return errorf(CodeProviderDisabled, "provider is not configured")
	}

	run, err := s.repo.GetRun(ctx, runID)
	if err != nil {
		return err
	}
	if run.CleanupStatus == CleanupStatusCompleted {
		return nil
	}
	if strings.TrimSpace(run.ProviderContainerID) == "" {
		return nil
	}

	now := s.now()
	if run.CleanupStatus == "" || run.CleanupStatus == CleanupStatusPending {
		_, _ = s.repo.SaveCleanupResult(ctx, run.ID, dbgen.SaveUnifiedAnalysisCleanupResultParams{
			CleanupStatus:    nullableText(CleanupStatusRunning),
			CleanupStartedAt: timestamptzNow(now),
		})
	}

	cleanupErr := provider.CleanupContainer(ctx, s.provider, run.ProviderContainerID, run.ProviderContainerFileID)
	status := CleanupStatusCompleted
	errMsg := ""
	if cleanupErr != nil {
		status = CleanupStatusFailed
		errMsg = cleanupErr.Error()
	}

	completed := s.now()
	params := dbgen.SaveUnifiedAnalysisCleanupResultParams{
		CleanupStatus:      nullableText(status),
		CleanupCompletedAt: timestamptzNow(completed),
		CleanupError:       nullableText(errMsg),
	}
	// Keep terminal provider status; track cleanup via cleanup_status only.
	updated, err := s.repo.SaveCleanupResult(ctx, run.ID, params)
	if err != nil {
		return err
	}

	if s.artifacts != nil {
		_ = s.artifacts.SaveCleanup(ctx, run.ID, cleanupArtifact{
			ContainerID: run.ProviderContainerID,
			FileID:      run.ProviderContainerFileID,
			Status:      status,
			Error:       errMsg,
		})
	}

	if cleanupErr != nil {
		return wrapError(CodeProviderFailed, "cleanup provider resources", cleanupErr)
	}
	_ = updated
	return nil
}
