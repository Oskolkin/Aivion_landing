package admin

import (
	"context"
	"fmt"
)

// runViewRawAIAuditAndFetch writes an admin_action_logs row before returning sensitive AI payloads.
// If audit logging fails after the log row exists, the sensitive payload is not returned (fail closed).
func runViewRawAIAuditAndFetch[T any](
	s *Service,
	ctx context.Context,
	actor AdminActor,
	sellerAccountID int64,
	targetType string,
	targetID int64,
	requestPayload map[string]any,
	fetch func() (T, error),
	successSummary func(T) map[string]any,
) (T, error) {
	var zero T
	if err := validateAdminActor(actor); err != nil {
		return zero, err
	}
	if err := validateSellerAccountID(sellerAccountID); err != nil {
		return zero, err
	}

	tt := targetType
	tid := targetID
	logEntry, err := s.repo.CreateAdminActionLog(ctx, CreateAdminActionLogInput{
		AdminUserID:     actor.UserID,
		AdminEmail:      actor.Email,
		SellerAccountID: sellerAccountID,
		ActionType:      AdminActionViewRawAIPayload,
		TargetType:      &tt,
		TargetID:        &tid,
		RequestPayload:  requestPayload,
		Status:          AdminActionStatusRunning,
	})
	if err != nil {
		return zero, fmt.Errorf("%w: create log: %w", ErrAdminAuditLogWriteFailed, err)
	}

	payload, err := fetch()
	if err != nil {
		_, _ = s.repo.FailAdminActionLog(ctx, FailAdminActionLogInput{
			ID:            logEntry.ID,
			ResultPayload: map[string]any{"phase": "fetch"},
			ErrorMessage:  err.Error(),
		})
		return zero, err
	}

	result := successSummary(payload)
	if result == nil {
		result = map[string]any{"ok": true}
	}
	_, completeErr := s.repo.CompleteAdminActionLog(ctx, CompleteAdminActionLogInput{
		ID:            logEntry.ID,
		ResultPayload: result,
	})
	if completeErr != nil {
		_, _ = s.repo.FailAdminActionLog(ctx, FailAdminActionLogInput{
			ID:            logEntry.ID,
			ResultPayload: map[string]any{"phase": "complete_audit"},
			ErrorMessage:  completeErr.Error(),
		})
		return zero, fmt.Errorf("%w: complete log: %w", ErrAdminAuditLogWriteFailed, completeErr)
	}
	return payload, nil
}
