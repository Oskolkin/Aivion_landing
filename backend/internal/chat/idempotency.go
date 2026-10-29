package chat

import (
	"context"
	"errors"
)

func (s *Service) rebuildAskResultFromUserMessage(
	ctx context.Context,
	sellerAccountID int64,
	userMsg *ChatMessage,
	session *ChatSession,
	sessionCreated bool,
	sessionReused bool,
) (*AskResult, error) {
	if userMsg == nil || session == nil {
		return nil, errors.New("idempotent message not found")
	}
	trace, err := s.repo.GetLatestTraceBySession(ctx, sellerAccountID, session.ID)
	if err != nil {
		return nil, err
	}
	result := &AskResult{
		SessionID:      session.ID,
		SessionCreated: sessionCreated,
		SessionReused:  sessionReused,
		UserMessageID:  userMsg.ID,
		TraceID:        0,
	}
	if trace != nil {
		result.TraceID = trace.ID
		if trace.AssistantMessageID != nil {
			result.AssistantMessageID = trace.AssistantMessageID
		}
	}
	if result.AssistantMessageID != nil {
		assistant, err := s.repo.GetMessage(ctx, sellerAccountID, *result.AssistantMessageID)
		if err != nil {
			return nil, err
		}
		if assistant != nil {
			// Best-effort idempotent replay: return minimal answer payload.
			result.Answer = &ChatAnswer{
				Answer: assistant.Content,
			}
		}
	}
	return result, nil
}
