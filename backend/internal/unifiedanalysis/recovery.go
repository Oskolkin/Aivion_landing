package unifiedanalysis

import (
	"context"
	"fmt"
)

// RecoveryActionType is the next Asynq task to schedule for a recoverable run.
type RecoveryActionType string

const (
	RecoveryActionStart    RecoveryActionType = "start"
	RecoveryActionPoll     RecoveryActionType = "poll"
	RecoveryActionCleanup  RecoveryActionType = "cleanup"
	RecoveryActionValidate RecoveryActionType = "validate"
	// RecoveryActionPublish means Stage 5 publication.Service.PublishRun should be
	// (re-)invoked for this run. Unlike the other recovery actions, RecoverRun cannot
	// execute this synchronously (unifiedanalysis has no dependency on the publication
	// package to avoid an import cycle); only worker's PlanRecovery -> EnqueuePublish
	// path handles it.
	RecoveryActionPublish RecoveryActionType = "publish"
)

// RecoveryAction describes one recoverable run and the task to enqueue.
type RecoveryAction struct {
	RunID  int64
	Action RecoveryActionType
	Status string
}

// PlanRecovery lists recoverable runs and the safe next task without executing provider work.
// Worker startup must enqueue these actions rather than calling StartRun/PollRun inline.
// A run may need both validate and cleanup; PlanRecovery returns all required actions.
func (s *Service) PlanRecovery(ctx context.Context, limit int) ([]RecoveryAction, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("unified analysis service is not configured")
	}
	if limit <= 0 {
		limit = 10
	}
	runs, err := s.repo.ListRecoverableRuns(ctx, int32(limit))
	if err != nil {
		return nil, err
	}
	out := make([]RecoveryAction, 0, len(runs))
	for _, run := range runs {
		if run == nil {
			continue
		}
		for _, action := range classifyRecoveryActions(run) {
			out = append(out, RecoveryAction{
				RunID:  run.ID,
				Action: action,
				Status: run.Status,
			})
		}
	}
	return out, nil
}

func classifyRecovery(run *Run) (RecoveryActionType, bool) {
	actions := classifyRecoveryActions(run)
	if len(actions) == 0 {
		return "", false
	}
	return actions[0], true
}

func classifyRecoveryActions(run *Run) []RecoveryActionType {
	if run == nil {
		return nil
	}
	var out []RecoveryActionType
	switch run.Status {
	case StatusCreated, StatusPackBuilding, StatusPackValidating, StatusPackReady,
		StatusProviderContainerCreating, StatusProviderContainerReady,
		StatusProviderFileUploading, StatusProviderFileReady,
		StatusProviderResponseStarting:
		out = append(out, RecoveryActionStart)
	case StatusProviderRunning:
		if run.ProviderResponseID == "" {
			out = append(out, RecoveryActionStart)
		} else {
			out = append(out, RecoveryActionPoll)
		}
	case StatusProviderCompleted:
		if isAnalysisPromptVersion(run.PromptVersion) {
			out = append(out, RecoveryActionValidate)
		}
		if needsCleanup(run) {
			out = append(out, RecoveryActionCleanup)
		}
	case StatusOutputValidating:
		out = append(out, RecoveryActionValidate)
		if needsCleanup(run) {
			out = append(out, RecoveryActionCleanup)
		}
	case StatusCleanupPending, StatusCleanupRunning:
		out = append(out, RecoveryActionCleanup)
	case StatusPublicationReady:
		out = append(out, RecoveryActionPublish)
		if needsCleanup(run) {
			out = append(out, RecoveryActionCleanup)
		}
	case StatusPublishing:
		out = append(out, RecoveryActionPublish)
	case StatusPublicationBlocked:
		if needsCleanup(run) {
			out = append(out, RecoveryActionCleanup)
		}
	}
	return out
}

func needsCleanup(run *Run) bool {
	if run == nil {
		return false
	}
	switch run.CleanupStatus {
	case CleanupStatusPending, CleanupStatusFailed, CleanupStatusRunning, "":
		return true
	default:
		return false
	}
}

// RecoverRun resumes a single run synchronously (direct/devtools). Prefer PlanRecovery + enqueue in Worker.
func (s *Service) RecoverRun(ctx context.Context, runID int64) error {
	if s == nil || s.repo == nil {
		return fmt.Errorf("unified analysis service is not configured")
	}
	run, err := s.repo.GetRun(ctx, runID)
	if err != nil {
		return err
	}
	actions := classifyRecoveryActions(run)
	for _, action := range actions {
		switch action {
		case RecoveryActionStart:
			if err := s.StartRun(ctx, runID); err != nil {
				return err
			}
		case RecoveryActionPoll:
			if _, err := s.PollRun(ctx, runID); err != nil {
				return err
			}
		case RecoveryActionCleanup:
			if err := s.CleanupRun(ctx, runID); err != nil {
				return err
			}
		case RecoveryActionValidate:
			if _, err := s.ValidateOutput(ctx, runID); err != nil {
				return err
			}
		}
	}
	return nil
}

// RecoverAll is retained for compatibility: it plans recovery actions then executes them synchronously.
// Worker startup should call PlanRecovery and enqueue Asynq tasks instead.
func (s *Service) RecoverAll(ctx context.Context, limit int) ([]int64, error) {
	actions, err := s.PlanRecovery(ctx, limit)
	if err != nil {
		return nil, err
	}
	seen := map[int64]struct{}{}
	ids := make([]int64, 0, len(actions))
	for _, action := range actions {
		if _, ok := seen[action.RunID]; ok {
			continue
		}
		seen[action.RunID] = struct{}{}
		if err := s.RecoverRun(ctx, action.RunID); err != nil {
			return ids, err
		}
		ids = append(ids, action.RunID)
	}
	return ids, nil
}
