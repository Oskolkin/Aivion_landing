package admin

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

func (s *Service) RerunSync(ctx context.Context, actor AdminActor, input RerunSyncInput) (*AdminActionLog, error) {
	return s.runAuditedAction(
		ctx,
		actor,
		input.SellerAccountID,
		AdminActionRerunSync,
		nil,
		nil,
		map[string]any{"seller_account_id": input.SellerAccountID},
		func(actionCtx context.Context) (map[string]any, error) {
			syncType := strings.TrimSpace(input.SyncType)
			if syncType == "" {
				syncType = "initial_sync"
			}
			if syncType != "initial_sync" {
				return nil, errors.New("unsupported sync_type")
			}
			if s.ingestionService == nil {
				return map[string]any{}, ErrAdminActionNotConfigured
			}
			jobID, jobStatus, err := s.ingestionService.StartInitialSync(actionCtx, input.SellerAccountID)
			if err != nil {
				return nil, err
			}
			return map[string]any{"sync_type": syncType, "sync_job_id": jobID, "status": jobStatus}, nil
		},
	)
}

func (s *Service) ResetCursor(ctx context.Context, actor AdminActor, input ResetCursorInput) (*AdminActionLog, error) {
	return s.runAuditedAction(
		ctx,
		actor,
		input.SellerAccountID,
		AdminActionResetCursor,
		strPtr("sync_cursor"),
		nil,
		map[string]any{
			"seller_account_id": input.SellerAccountID,
			"domain":            input.Domain,
			"cursor_type":       input.CursorType,
			"cursor_value":      input.CursorValue,
		},
		func(actionCtx context.Context) (map[string]any, error) {
			if strings.TrimSpace(input.Domain) == "" || strings.TrimSpace(input.CursorType) == "" {
				return nil, errors.New("domain and cursor_type are required")
			}
			if s.cursorService == nil {
				return map[string]any{}, ErrAdminActionNotConfigured
			}
			if err := s.cursorService.ResetCursor(actionCtx, input.SellerAccountID, strings.TrimSpace(input.Domain), strings.TrimSpace(input.CursorType), input.CursorValue); err != nil {
				return nil, err
			}
			return map[string]any{"domain": input.Domain, "cursor_type": input.CursorType, "cursor_value": input.CursorValue, "reset": true}, nil
		},
	)
}

func (s *Service) RerunMetrics(ctx context.Context, actor AdminActor, input RerunMetricsInput) (*AdminActionLog, error) {
	dateFrom, dateTo, err := resolveRerunMetricsDateRange(input)
	if err != nil {
		return nil, err
	}

	return s.runAuditedAction(
		ctx,
		actor,
		input.SellerAccountID,
		AdminActionRerunMetrics,
		nil,
		nil,
		map[string]any{
			"seller_account_id": input.SellerAccountID,
			"date_from":         dateFrom.Format("2006-01-02"),
			"date_to":           dateTo.Format("2006-01-02"),
		},
		func(actionCtx context.Context) (map[string]any, error) {
			if s.metricsService == nil {
				return map[string]any{}, ErrAdminActionNotConfigured
			}
			extra, err := s.metricsService.Rerun(actionCtx, input.SellerAccountID, dateFrom, dateTo)
			if err != nil {
				return nil, err
			}
			out := map[string]any{
				"seller_account_id": input.SellerAccountID,
				"date_from":         dateFrom.Format("2006-01-02"),
				"date_to":           dateTo.Format("2006-01-02"),
			}
			for k, v := range extra {
				out[k] = v
			}
			return out, nil
		},
	)
}

func (s *Service) RerunAlerts(ctx context.Context, actor AdminActor, input RerunAlertsInput) (*AdminActionLog, error) {
	return s.runAuditedAction(
		ctx,
		actor,
		input.SellerAccountID,
		AdminActionRerunAlerts,
		nil,
		nil,
		map[string]any{"seller_account_id": input.SellerAccountID},
		func(actionCtx context.Context) (map[string]any, error) {
			if input.AsOfDate.IsZero() {
				return nil, errors.New("as_of_date is required")
			}
			if s.alertsService == nil {
				return map[string]any{}, ErrAdminActionNotConfigured
			}
			result, err := s.alertsService.Rerun(actionCtx, input.SellerAccountID, input.AsOfDate)
			if err != nil {
				return nil, err
			}
			if result == nil {
				result = map[string]any{}
			}
			result["as_of_date"] = input.AsOfDate.UTC().Format("2006-01-02")
			return result, nil
		},
	)
}

func (s *Service) RerunRecommendations(ctx context.Context, actor AdminActor, input RerunRecommendationsInput) (*AdminActionLog, error) {
	return s.runAuditedAction(
		ctx,
		actor,
		input.SellerAccountID,
		AdminActionRerunRecommendations,
		nil,
		nil,
		map[string]any{"seller_account_id": input.SellerAccountID},
		func(actionCtx context.Context) (map[string]any, error) {
			if input.AsOfDate.IsZero() {
				return nil, errors.New("as_of_date is required")
			}
			if s.recommendationsService == nil {
				return map[string]any{}, ErrAdminActionNotConfigured
			}
			result, err := s.recommendationsService.Rerun(actionCtx, input.SellerAccountID, input.AsOfDate)
			if err != nil {
				return nil, err
			}
			if result == nil {
				result = map[string]any{}
			}
			result["as_of_date"] = input.AsOfDate.UTC().Format("2006-01-02")
			return result, nil
		},
	)
}

func (s *Service) RetryAIPipelineDispatch(ctx context.Context, actor AdminActor, input RetryAIPipelineDispatchInput) (*AdminActionLog, error) {
	targetType := "ai_pipeline_dispatch"
	targetID := input.DispatchID
	return s.runAuditedAction(
		ctx,
		actor,
		input.SellerAccountID,
		AdminActionRetryAIPipelineDispatch,
		&targetType,
		&targetID,
		map[string]any{
			"dispatch_id": input.DispatchID,
			"sync_job_id": input.SyncJobID,
		},
		func(ctx context.Context) (map[string]any, error) {
			return map[string]any{
				"dispatch_id": input.DispatchID,
				"sync_job_id": input.SyncJobID,
				"status":      "retry_pending",
			}, nil
		},
	)
}

func (s *Service) runAuditedAction(
	ctx context.Context,
	actor AdminActor,
	sellerAccountID int64,
	actionType AdminActionType,
	targetType *string,
	targetID *int64,
	requestPayload map[string]any,
	fn func(ctx context.Context) (map[string]any, error),
) (*AdminActionLog, error) {
	if err := validateAdminActor(actor); err != nil {
		return nil, err
	}
	if err := validateSellerAccountID(sellerAccountID); err != nil {
		return nil, err
	}

	logEntry, err := s.repo.CreateAdminActionLog(ctx, CreateAdminActionLogInput{
		AdminUserID:     actor.UserID,
		AdminEmail:      actor.Email,
		SellerAccountID: sellerAccountID,
		ActionType:      actionType,
		TargetType:      targetType,
		TargetID:        targetID,
		RequestPayload:  requestPayload,
		Status:          AdminActionStatusRunning,
	})
	if err != nil {
		return nil, fmt.Errorf("create admin action log: %w", err)
	}

	resultPayload, runErr := fn(ctx)
	if runErr != nil {
		failed, failErr := s.repo.FailAdminActionLog(ctx, FailAdminActionLogInput{
			ID:            logEntry.ID,
			ResultPayload: fallbackPayload(resultPayload),
			ErrorMessage:  runErr.Error(),
		})
		if failErr != nil {
			return logEntry, fmt.Errorf("action failed: %v (also failed to mark action log as failed: %v)", runErr, failErr)
		}
		return failed, runErr
	}

	completed, completeErr := s.repo.CompleteAdminActionLog(ctx, CompleteAdminActionLogInput{
		ID:            logEntry.ID,
		ResultPayload: fallbackPayload(resultPayload),
	})
	if completeErr != nil {
		return logEntry, fmt.Errorf("complete admin action log: %w", completeErr)
	}
	return completed, nil
}

func fallbackPayload(payload map[string]any) map[string]any {
	if payload == nil {
		return map[string]any{}
	}
	return payload
}

func strPtr(v string) *string {
	s := v
	return &s
}
