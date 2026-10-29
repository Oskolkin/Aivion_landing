package admin

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

func (s *Service) GetBillingState(ctx context.Context, sellerAccountID int64) (*BillingState, error) {
	if err := validateSellerAccountID(sellerAccountID); err != nil {
		return nil, err
	}
	return s.repo.GetBillingState(ctx, sellerAccountID)
}

func (s *Service) ListBillingStates(ctx context.Context, filter BillingStateFilter) ([]BillingState, error) {
	filterPage := NormalizePage(filter.Limit, filter.Offset, 50, 200)
	filter.Limit = filterPage.Limit
	filter.Offset = filterPage.Offset
	filter.Status = strings.TrimSpace(filter.Status)
	return s.repo.ListBillingStates(ctx, filter)
}

func (s *Service) UpdateBillingState(ctx context.Context, actor AdminActor, input UpdateBillingStateInput) (*BillingState, *AdminActionLog, error) {
	if err := validateAdminActor(actor); err != nil {
		return nil, nil, err
	}
	if err := validateSellerAccountID(input.SellerAccountID); err != nil {
		return nil, nil, err
	}
	input.PlanCode = strings.TrimSpace(input.PlanCode)
	input.Status = BillingStatus(strings.TrimSpace(string(input.Status)))
	if input.Notes != nil {
		n := strings.TrimSpace(*input.Notes)
		if n == "" {
			input.Notes = nil
		} else {
			input.Notes = &n
		}
	}
	if err := validateBillingUpdateInput(input); err != nil {
		return nil, nil, err
	}

	requestPayload := map[string]any{
		"seller_account_id": input.SellerAccountID,
		"plan_code":         input.PlanCode,
		"status":            input.Status,
		"ai_tokens_limit":   input.AITokensLimitMonth,
		"ai_tokens_used":    input.AITokensUsedMonth,
	}

	actionLog, err := s.runAuditedAction(
		ctx,
		actor,
		input.SellerAccountID,
		AdminActionUpdateBillingState,
		nil,
		nil,
		requestPayload,
		func(actionCtx context.Context) (map[string]any, error) {
			state, upsertErr := s.repo.UpsertBillingState(actionCtx, UpsertBillingStateInput(input))
			if upsertErr != nil {
				return nil, upsertErr
			}
			requestPayload["updated_status"] = state.Status
			return map[string]any{
				"seller_account_id": state.SellerAccountID,
				"status":            state.Status,
			}, nil
		},
	)
	if err != nil {
		return nil, actionLog, err
	}

	state, stateErr := s.repo.GetBillingState(ctx, input.SellerAccountID)
	if stateErr != nil {
		return nil, actionLog, fmt.Errorf("get updated billing state: %w", stateErr)
	}
	return state, actionLog, nil
}

func validateBillingUpdateInput(input UpdateBillingStateInput) error {
	if input.PlanCode == "" {
		return errors.New("plan_code is required")
	}
	switch input.Status {
	case BillingStatusTrial, BillingStatusActive, BillingStatusPastDue, BillingStatusPaused, BillingStatusCancelled, BillingStatusInternal:
	default:
		return errors.New("invalid billing status")
	}
	if input.AITokensLimitMonth != nil && *input.AITokensLimitMonth < 0 {
		return errors.New("ai_tokens_limit_month must be non-negative")
	}
	if input.AITokensUsedMonth < 0 {
		return errors.New("ai_tokens_used_month must be non-negative")
	}
	if input.EstimatedAICostMonth < 0 {
		return errors.New("estimated_ai_cost_month must be non-negative")
	}
	if input.CurrentPeriodStart != nil && input.CurrentPeriodEnd != nil && input.CurrentPeriodStart.After(*input.CurrentPeriodEnd) {
		return errors.New("current_period_start must be before or equal to current_period_end")
	}
	return nil
}
