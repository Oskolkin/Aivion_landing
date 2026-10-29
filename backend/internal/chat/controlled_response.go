package chat

import (
	"context"
	"log/slog"
	"strings"
)

const (
	controlledMsgPlanner       = "Не удалось спланировать запрос к данным кабинета. Попробуйте переформулировать вопрос или повторить позже."
	controlledMsgInvalidPlan   = "Не удалось построить безопасный план чтения данных. Уточните вопрос: укажите SKU, период или тип отчёта."
	controlledMsgToolsFailed   = "Не удалось получить данные для ответа. Попробуйте позже или сузьте запрос (SKU, период, раздел кабинета)."
	controlledMsgContextFailed = "Недостаточно подготовленных данных для ответа. Уточните SKU, период или переформулируйте вопрос."
	controlledMsgAnswerInvalid = "Ответ не прошёл проверку по фактам из кабинета. Задайте более конкретный вопрос по метрикам, остаткам, рекламе или рекомендациям."
	controlledMsgDailyLimit    = "Достигнут дневной лимит AI-запросов для кабинета. Попробуйте завтра или обратитесь в поддержку."
)

func (s *Service) completeControlledFailure(
	ctx context.Context,
	session *ChatSession,
	input AskInput,
	userMsg *ChatMessage,
	trace *ChatTrace,
	reasonCode string,
	userMessage string,
	limitations []string,
	traceExtras map[string]any,
) (*AskResult, error) {
	userMessage = strings.TrimSpace(userMessage)
	if userMessage == "" {
		userMessage = "Не удалось обработать запрос. Попробуйте переформулировать вопрос."
	}
	slog.Warn("chat controlled failure response",
		"trace_id", trace.ID,
		"seller_account_id", input.SellerAccountID,
		"session_id", session.ID,
		"reason", reasonCode,
		"openai_called", false,
	)

	answerText := FormatControlledAnswer(userMessage, nil)
	finalAnswer := ChatAnswer{
		Answer:          answerText,
		Summary:         userMessage,
		Intent:          ChatIntentUnknown,
		ConfidenceLevel: ConfidenceLevelMedium,
		Limitations:     limitations,
	}
	if len(finalAnswer.Limitations) == 0 {
		finalAnswer.Limitations = []string{"Ответ сформирован без вызова OpenAI (controlled fallback)."}
	}

	assistantMsg, err := s.repo.CreateMessage(ctx, CreateMessageInput{
		SessionID:       session.ID,
		SellerAccountID: input.SellerAccountID,
		Role:            ChatMessageRoleAssistant,
		Content:         answerText,
		MessageType:     ChatMessageTypeAnswer,
	})
	if err != nil {
		s.failTraceBestEffort(ctx, FailTraceInput{
			SellerAccountID: input.SellerAccountID,
			TraceID:         trace.ID,
			ErrorMessage:    errorString(err),
		})
		return nil, err
	}
	_, _ = s.repo.TouchSession(ctx, input.SellerAccountID, session.ID)

	payload := map[string]any{
		"controlled_failure": reasonCode,
		"openai_called":      false,
	}
	for k, v := range traceExtras {
		payload[k] = v
	}
	actionContext := s.resolveActionPromptContext(ctx, input.SellerAccountID, input.ActionKey)
	payload = mergeTraceDiagnostics(payload, buildChatTraceDiagnostics(input, actionContext, false, true))
	_, err = s.repo.CompleteTrace(ctx, CompleteTraceInput{
		SellerAccountID:          input.SellerAccountID,
		TraceID:                  trace.ID,
		AssistantMessageID:       &assistantMsg.ID,
		DetectedIntent:           finalAnswer.Intent,
		ValidatedToolPlanPayload: payload,
		AnswerValidationPayload:  map[string]any{"controlled_failure": true, "reason": reasonCode},
	})
	if err != nil {
		return nil, err
	}
	return &AskResult{
		SessionID:          session.ID,
		UserMessageID:      userMsg.ID,
		AssistantMessageID: &assistantMsg.ID,
		TraceID:            trace.ID,
		Answer:             &finalAnswer,
	}, nil
}
