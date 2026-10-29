package chat

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/chatexport"
	"github.com/google/uuid"
)

func (s *Service) completeExportRequest(
	ctx context.Context,
	session *ChatSession,
	input AskInput,
	userMsg *ChatMessage,
	trace *ChatTrace,
	parsed chatexport.ParsedRequest,
) (*AskResult, error) {
	days := exportDaysInclusive(parsed.DateFrom, parsed.DateTo)
	answerText := chatexport.SuccessMessage(parsed.ExportType, parsed.Format, 0, days)
	var attachments []ChatAttachment

	if s.exporter == nil || s.attachments == nil {
		answerText = "Выгрузка файлов временно недоступна. Обратитесь в поддержку."
	} else {
		outcome, exportErr := s.exporter.Export(ctx, chatexport.ExportRequest{
			SellerAccountID: input.SellerAccountID,
			UserID:          input.UserID,
			SessionID:       session.ID,
			ExportType:      parsed.ExportType,
			Format:          parsed.Format,
			DateFrom:        parsed.DateFrom,
			DateTo:          parsed.DateTo,
			Filters:         parsed.Filters,
		})
		if exportErr != nil {
			slog.Error("chat export failed",
				"trace_id", trace.ID,
				"seller_account_id", input.SellerAccountID,
				"export_type", parsed.ExportType,
				"err", exportErr,
			)
			if errors.Is(exportErr, chatexport.ErrExportStorageUnavailable) {
				answerText = "Хранилище выгрузок временно недоступно. Попробуйте позже или обратитесь в поддержку."
			} else {
				answerText = "Не удалось подготовить файл. Попробуйте позже или уточните запрос."
			}
		} else {
			answerText = chatexport.SuccessMessage(parsed.ExportType, parsed.Format, outcome.RowCount, days)
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
			sessionID := session.ID
			messageID := assistantMsg.ID
			att, attErr := s.attachments.CreateAttachment(ctx, CreateAttachmentInput{
				ID:              outcome.AttachmentID,
				SellerAccountID: input.SellerAccountID,
				SessionID:       &sessionID,
				MessageID:       &messageID,
				CreatedByUserID: input.UserID,
				AttachmentKind:  AttachmentKindExport,
				ExportType:      parsed.ExportType,
				Format:          parsed.Format,
				FileName:        outcome.FileName,
				MimeType:        outcome.MimeType,
				FileSize:        outcome.FileSize,
				StorageKey:      outcome.StorageKey,
				Status:          AttachmentStatusReady,
			})
			if attErr != nil {
				slog.Error("chat attachment persist failed", "err", attErr)
			} else if att != nil {
				attachments = []ChatAttachment{*att}
			}
			return s.finishExportAsk(ctx, session, input, userMsg, trace, assistantMsg, parsed, answerText, attachments)
		}
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
	return s.finishExportAsk(ctx, session, input, userMsg, trace, assistantMsg, parsed, answerText, attachments)
}

func (s *Service) finishExportAsk(
	ctx context.Context,
	session *ChatSession,
	input AskInput,
	userMsg *ChatMessage,
	trace *ChatTrace,
	assistantMsg *ChatMessage,
	parsed chatexport.ParsedRequest,
	answerText string,
	attachments []ChatAttachment,
) (*AskResult, error) {
	_, _ = s.repo.TouchSession(ctx, input.SellerAccountID, session.ID)
	exportPayload := map[string]any{
		"export_type":   parsed.ExportType,
		"format":        parsed.Format,
		"date_from":     parsed.DateFrom.Format("2006-01-02"),
		"date_to":       parsed.DateTo.Format("2006-01-02"),
		"openai_called": false,
	}
	_, err := s.repo.CompleteTrace(ctx, CompleteTraceInput{
		SellerAccountID:          input.SellerAccountID,
		TraceID:                  trace.ID,
		AssistantMessageID:       &assistantMsg.ID,
		DetectedIntent:           ChatIntentUnknown,
		ToolPlanPayload:          map[string]any{"export": exportPayload},
		ValidatedToolPlanPayload: map[string]any{"export": exportPayload},
		AnswerValidationPayload:  map[string]any{"export_controlled": true},
	})
	if err != nil {
		return nil, err
	}
	return &AskResult{
		SessionID:          session.ID,
		UserMessageID:      userMsg.ID,
		AssistantMessageID: &assistantMsg.ID,
		TraceID:            trace.ID,
		Answer: &ChatAnswer{
			Answer:          answerText,
			Summary:         answerText,
			Intent:          ChatIntentUnknown,
			ConfidenceLevel: ConfidenceLevelHigh,
			Limitations:     []string{"Ответ сформирован без вызова OpenAI (export)."},
			Attachments:     mapAttachmentsToDTO(attachments, s.attachmentDownloadPath),
		},
	}, nil
}

func (s *Service) DownloadAttachment(ctx context.Context, sellerAccountID int64, attachmentID uuid.UUID) (*ChatAttachment, []byte, error) {
	if s.attachments == nil {
		return nil, nil, errors.New("attachment download is not configured")
	}
	att, err := s.attachments.GetAttachmentForSeller(ctx, sellerAccountID, attachmentID)
	if err != nil {
		return nil, nil, err
	}
	if att == nil {
		return nil, nil, nil
	}

	// Draft / message input uploads live in the input bucket (not export storage).
	if att.AttachmentKind == AttachmentKindInput {
		if s.inputStorage == nil || strings.TrimSpace(s.inputBucket) == "" {
			return nil, nil, errors.New("input attachment storage is not configured")
		}
		if strings.TrimSpace(att.StorageKey) == "" {
			return att, nil, errors.New("attachment is not ready")
		}
		switch att.Status {
		case InputAttachmentStatusUploaded, InputAttachmentStatusAttached, InputAttachmentStatusProcessed, InputAttachmentStatusProcessing:
			// ok
		default:
			return att, nil, errors.New("attachment is not ready")
		}
		data, err := s.inputStorage.DownloadInputAttachment(ctx, s.inputBucket, att.StorageKey)
		if err != nil {
			return att, nil, err
		}
		return att, data, nil
	}

	if s.exportStorage == nil {
		return nil, nil, errors.New("export download is not configured")
	}
	if att.SessionID == nil {
		return nil, nil, nil
	}
	session, err := s.repo.GetSession(ctx, sellerAccountID, *att.SessionID)
	if err != nil {
		return nil, nil, err
	}
	if session == nil {
		return nil, nil, nil
	}
	if att.MessageID != nil && *att.MessageID > 0 {
		msg, msgErr := s.repo.GetMessage(ctx, sellerAccountID, *att.MessageID)
		if msgErr != nil {
			return nil, nil, msgErr
		}
		if msg == nil || msg.SessionID != *att.SessionID {
			return nil, nil, nil
		}
	}
	// Access model: seller_account_id from auth middleware; any user of the seller account may download.
	// created_by_user_id is stored for audit only and is not enforced here.
	if att.Status != AttachmentStatusReady || att.StorageKey == "" {
		return att, nil, errors.New("attachment is not ready")
	}
	data, err := s.exportStorage.DownloadExport(ctx, att.StorageKey)
	if err != nil {
		return att, nil, err
	}
	return att, data, nil
}

func exportDaysInclusive(from, to time.Time) int {
	from = from.UTC().Truncate(24 * time.Hour)
	to = to.UTC().Truncate(24 * time.Hour)
	if to.Before(from) {
		return DefaultChatDateRangeDays
	}
	d := int(to.Sub(from).Hours()/24) + 1
	if d <= 0 {
		return DefaultChatDateRangeDays
	}
	return d
}

func mapAttachmentsToDTO(items []ChatAttachment, downloadPathFn func(string) string) []ChatAnswerAttachment {
	if len(items) == 0 || downloadPathFn == nil {
		return nil
	}
	out := make([]ChatAnswerAttachment, 0, len(items))
	for _, item := range items {
		out = append(out, ChatAnswerAttachment{
			ID:          item.ID.String(),
			FileName:    item.FileName,
			MimeType:    item.MimeType,
			FileSize:    item.FileSize,
			ExportType:  item.ExportType,
			Format:      item.Format,
			DownloadURL: downloadPathFn(item.ID.String()),
			CreatedAt:   item.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	return out
}
