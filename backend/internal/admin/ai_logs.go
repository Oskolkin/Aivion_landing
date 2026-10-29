package admin

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
)

func (s *Service) GetAIRecommendationLogs(ctx context.Context, sellerAccountID int64, filter RecommendationRunLogFilter) (*RecommendationRunLogListResult, error) {
	if err := validateSellerAccountID(sellerAccountID); err != nil {
		return &RecommendationRunLogListResult{}, err
	}
	page := NormalizePage(filter.Limit, filter.Offset, 50, 200)
	filter.Status = strings.TrimSpace(filter.Status)
	filter.RunType = strings.TrimSpace(filter.RunType)
	items, err := s.repo.ListRecommendationRuns(ctx, sellerAccountID, filter, page)
	if err != nil {
		return nil, err
	}
	return &RecommendationRunLogListResult{
		Items:  items,
		Limit:  page.Limit,
		Offset: page.Offset,
	}, nil
}

func (s *Service) GetRecommendationRunDetail(ctx context.Context, actor AdminActor, sellerAccountID, runID int64) (*RecommendationRunDetail, error) {
	if err := validateSellerAccountID(sellerAccountID); err != nil {
		return nil, err
	}
	if runID <= 0 {
		return nil, ErrAdminDataUnavailable
	}
	meta, err := s.repo.PeekRecommendationRunForAudit(ctx, sellerAccountID, runID)
	if err != nil {
		return nil, err
	}
	req := map[string]any{
		"target_type":       AdminRawViewTargetRecommendationRun,
		"target_id":         runID,
		"seller_account_id": sellerAccountID,
		"run_type":          meta.RunType,
		"run_status":        meta.Status,
		"ai_model":          meta.AIModel,
		"ai_prompt_version": meta.AIPromptVersion,
	}
	return runViewRawAIAuditAndFetch(s, ctx, actor, sellerAccountID, AdminRawViewTargetRecommendationRun, runID, req,
		func() (*RecommendationRunDetail, error) {
			detail, derr := s.repo.GetRecommendationRunDetail(ctx, sellerAccountID, runID)
			if derr != nil {
				return nil, derr
			}
			if len(detail.Diagnostics) == 0 {
				detail.Limitations = append(detail.Limitations, "Rejected item payloads are unavailable for historical runs.")
			}
			if len(detail.Recommendations) == 0 {
				detail.Limitations = append(detail.Limitations, "Recommendations cannot be reliably linked to a specific run with current schema.")
			}
			return detail, nil
		},
		func(d *RecommendationRunDetail) map[string]any {
			return map[string]any{
				"ok":                    true,
				"target_type":           AdminRawViewTargetRecommendationRun,
				"target_id":             runID,
				"diagnostics_count":     len(d.Diagnostics),
				"recommendations_count": len(d.Recommendations),
			}
		},
	)
}

func (s *Service) GetRecommendationRawAI(ctx context.Context, actor AdminActor, sellerAccountID, recommendationID int64) (*RecommendationRawAI, error) {
	if err := validateSellerAccountID(sellerAccountID); err != nil {
		return nil, err
	}
	if recommendationID <= 0 {
		return nil, ErrAdminDataUnavailable
	}
	meta, err := s.repo.PeekRecommendationForAudit(ctx, sellerAccountID, recommendationID)
	if err != nil {
		return nil, err
	}
	req := map[string]any{
		"target_type":       AdminRawViewTargetRecommendation,
		"target_id":         recommendationID,
		"seller_account_id": sellerAccountID,
		"ai_model":          meta.AIModel,
		"ai_prompt_version": meta.AIPromptVersion,
	}
	return runViewRawAIAuditAndFetch(s, ctx, actor, sellerAccountID, AdminRawViewTargetRecommendation, recommendationID, req,
		func() (*RecommendationRawAI, error) {
			return s.repo.GetRecommendationRawAI(ctx, sellerAccountID, recommendationID)
		},
		func(r *RecommendationRawAI) map[string]any {
			return map[string]any{
				"ok":                   true,
				"target_type":          AdminRawViewTargetRecommendation,
				"target_id":            recommendationID,
				"related_alerts_count": len(r.RelatedAlerts),
				"diagnostics_count":    len(r.Diagnostics),
			}
		},
	)
}

func (s *Service) GetAIChatLogs(ctx context.Context, sellerAccountID int64, filter ChatTraceFilter) (*ChatTraceListResult, error) {
	if err := validateSellerAccountID(sellerAccountID); err != nil {
		return nil, err
	}
	filter.Status = strings.TrimSpace(filter.Status)
	filter.Intent = strings.TrimSpace(filter.Intent)
	page := NormalizePage(filter.Limit, filter.Offset, 50, 200)
	items, err := s.repo.ListChatTraces(ctx, sellerAccountID, filter, page)
	if err != nil {
		return nil, err
	}
	return &ChatTraceListResult{
		Items:  items,
		Limit:  page.Limit,
		Offset: page.Offset,
	}, nil
}

func (s *Service) GetChatTraceDetail(ctx context.Context, actor AdminActor, sellerAccountID, traceID int64) (*ChatTraceDetail, error) {
	if err := validateSellerAccountID(sellerAccountID); err != nil {
		return nil, err
	}
	if traceID <= 0 {
		return nil, ErrAdminDataUnavailable
	}
	meta, err := s.repo.PeekChatTraceForAudit(ctx, sellerAccountID, traceID)
	if err != nil {
		return nil, err
	}
	req := map[string]any{
		"target_type":            AdminRawViewTargetChatTrace,
		"target_id":              traceID,
		"seller_account_id":      sellerAccountID,
		"session_id":             meta.SessionID,
		"user_message_id":        meta.UserMessageID,
		"assistant_message_id":   meta.AssistantMessageID,
		"planner_model":          meta.PlannerModel,
		"answer_model":           meta.AnswerModel,
		"planner_prompt_version": meta.PlannerPromptVersion,
		"answer_prompt_version":  meta.AnswerPromptVersion,
		"trace_status":           meta.Status,
	}
	return runViewRawAIAuditAndFetch(s, ctx, actor, sellerAccountID, AdminRawViewTargetChatTrace, traceID, req,
		func() (*ChatTraceDetail, error) {
			return s.repo.GetChatTraceDetail(ctx, sellerAccountID, traceID)
		},
		func(d *ChatTraceDetail) map[string]any {
			out := map[string]any{
				"ok":             true,
				"target_type":    AdminRawViewTargetChatTrace,
				"target_id":      traceID,
				"session_id":     d.SessionID,
				"messages_count": len(d.Messages),
			}
			if d.UserMessageID != nil {
				out["user_message_id"] = *d.UserMessageID
			}
			if d.AssistantMessageID != nil {
				out["assistant_message_id"] = *d.AssistantMessageID
			}
			return out
		},
	)
}

func (s *Service) ListChatSessions(ctx context.Context, sellerAccountID int64, filter ChatSessionFilter) (*ChatSessionListResult, error) {
	if err := validateSellerAccountID(sellerAccountID); err != nil {
		return nil, err
	}
	filter.Status = strings.TrimSpace(filter.Status)
	page := NormalizePage(filter.Limit, filter.Offset, 50, 200)
	items, err := s.repo.ListChatSessions(ctx, sellerAccountID, filter, page)
	if err != nil {
		return nil, err
	}
	return &ChatSessionListResult{
		Items:  items,
		Limit:  page.Limit,
		Offset: page.Offset,
	}, nil
}

func (s *Service) ListChatMessages(ctx context.Context, sellerAccountID, sessionID int64, filter ChatMessageFilter) (*ChatMessageListResult, error) {
	if err := validateSellerAccountID(sellerAccountID); err != nil {
		return nil, err
	}
	if sessionID <= 0 {
		return nil, ErrAdminDataUnavailable
	}
	session, err := s.repo.GetChatSession(ctx, sellerAccountID, sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, pgx.ErrNoRows
	}
	page := NormalizePage(filter.Limit, filter.Offset, 50, 200)
	items, err := s.repo.ListChatMessages(ctx, sellerAccountID, sessionID, filter, page)
	if err != nil {
		return nil, err
	}
	return &ChatMessageListResult{
		Items:  items,
		Limit:  page.Limit,
		Offset: page.Offset,
	}, nil
}

func (s *Service) ListChatFeedback(ctx context.Context, filter ChatFeedbackFilter) (*ChatFeedbackListResult, error) {
	if filter.SellerAccountID != nil {
		if err := validateSellerAccountID(*filter.SellerAccountID); err != nil {
			return nil, err
		}
	}
	filter.Rating = strings.TrimSpace(filter.Rating)
	page := NormalizePage(filter.Limit, filter.Offset, 50, 200)
	items, err := s.repo.ListChatFeedback(ctx, filter, page)
	if err != nil {
		return nil, err
	}
	return &ChatFeedbackListResult{Items: items, Limit: page.Limit, Offset: page.Offset}, nil
}

func (s *Service) ListRecommendationFeedback(ctx context.Context, sellerAccountID int64, filter RecommendationFeedbackFilter) (*RecommendationFeedbackListResult, error) {
	if err := validateSellerAccountID(sellerAccountID); err != nil {
		return nil, err
	}
	filter.Rating = strings.TrimSpace(filter.Rating)
	filter.Status = strings.TrimSpace(filter.Status)
	page := NormalizePage(filter.Limit, filter.Offset, 50, 200)
	items, err := s.repo.ListRecommendationFeedback(ctx, sellerAccountID, filter, page)
	if err != nil {
		return nil, err
	}
	counts, err := s.repo.GetRecommendationProxyFeedbackCounts(ctx, sellerAccountID)
	if err != nil {
		return nil, err
	}
	return &RecommendationFeedbackListResult{
		Items:               items,
		ProxyStatusFeedback: counts,
		Limitations:         []string{"Historical recommendation feedback is represented by recommendation statuses accepted/dismissed/resolved."},
		Limit:               page.Limit,
		Offset:              page.Offset,
	}, nil
}
