package chat

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/chatextract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type InputAttachmentDTO struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	MimeType         string   `json:"mime_type"`
	SizeBytes        int64    `json:"size_bytes"`
	Status           string   `json:"status"`
	WarningCode      *string  `json:"warning_code,omitempty"`
	WarningText      *string  `json:"warning_text,omitempty"`
	ExtractionStatus string   `json:"extraction_status,omitempty"`
	CharacterCount   *int32   `json:"character_count,omitempty"`
	Warnings         []string `json:"warnings,omitempty"`
}

type UploadInputAttachmentInput struct {
	SellerAccountID int64
	UserID          *int64
	FileName        string
	MimeType        string
	Data            []byte
}

type attachmentExtractionUpdate struct {
	Status         string
	Text           string
	Meta           []byte
	PageCount      int32
	SheetCount     int32
	SlideCount     int32
	CharacterCount int32
	Error          string
}

type attachmentExtractionSetter interface {
	setAttachmentExtraction(ctx context.Context, sellerAccountID int64, attachmentID uuid.UUID, update attachmentExtractionUpdate) (*ChatAttachment, error)
}

func (s *Service) UploadInputAttachment(ctx context.Context, input UploadInputAttachmentInput) (*InputAttachmentDTO, error) {
	if s.attachments == nil {
		return nil, errors.New("attachment repository is required")
	}
	if s.inputStorage == nil || strings.TrimSpace(s.inputBucket) == "" {
		return nil, errors.New("input attachment storage is not configured")
	}
	fileName := sanitizeDisplayFileName(input.FileName)
	if err := validateInputAttachmentContent(fileName, input.MimeType, input.Data); err != nil {
		return nil, err
	}
	format := formatFromFileName(fileName)
	mimeType := strings.TrimSpace(input.MimeType)
	switch strings.ToLower(format) {
	case "png", "jpg", "jpeg", "webp", "gif":
		mimeType = resolveImageMIMEType(format, mimeType, input.Data)
	}
	count, err := s.attachments.CountDraftInputAttachments(ctx, input.SellerAccountID)
	if err != nil {
		return nil, err
	}
	if count >= int64(chatAttachmentMaxFiles()) {
		return nil, fmt.Errorf("%w: max %d", ErrAttachmentTooMany, chatAttachmentMaxFiles())
	}
	attachmentID := uuid.New()
	storageKey := fmt.Sprintf("chat-attachments/%d/%s", input.SellerAccountID, attachmentID.String())
	if err := s.inputStorage.UploadInputAttachment(ctx, s.inputBucket, storageKey, input.Data, mimeType); err != nil {
		return nil, err
	}
	sum := sha256.Sum256(input.Data)
	hash := hex.EncodeToString(sum[:])
	expires := time.Now().UTC().Add(time.Duration(chatDraftAttachmentTTLHours()) * time.Hour)
	row, err := s.attachments.CreateAttachment(ctx, CreateAttachmentInput{
		ID:              attachmentID,
		SellerAccountID: input.SellerAccountID,
		CreatedByUserID: input.UserID,
		AttachmentKind:  AttachmentKindInput,
		Format:          format,
		FileName:        fileName,
		MimeType:        mimeType,
		FileSize:        int64(len(input.Data)),
		StorageKey:      storageKey,
		Status:          InputAttachmentStatusUploaded,
		Sha256:          &hash,
		ExpiresAt:       &expires,
	})
	if err != nil {
		_ = s.inputStorage.DeleteInputAttachment(ctx, s.inputBucket, storageKey)
		return nil, err
	}
	dto := mapInputAttachmentDTO(row)
	if updated, extractionErr := s.extractInputAttachment(ctx, input.SellerAccountID, attachmentID, fileName, input.MimeType, input.Data); extractionErr == nil && updated != nil {
		dto = mapInputAttachmentDTO(updated)
	}
	if warningMsg := spreadsheetRowLimitWarning(fileName, input.Data); warningMsg != "" {
		_ = s.setAttachmentWarningCode(ctx, input.SellerAccountID, attachmentID, warningCodeSpreadsheetTruncated)
		code := warningCodeSpreadsheetTruncated
		dto.WarningCode = &code
		dto.WarningText = &warningMsg
		dto.Warnings = append(dto.Warnings, warningMsg)
	}
	return dto, nil
}

func (s *Service) DeleteInputAttachment(ctx context.Context, sellerAccountID int64, attachmentID uuid.UUID) error {
	if s.attachments == nil {
		return errors.New("attachment repository is required")
	}
	row, err := s.attachments.GetAttachmentForSeller(ctx, sellerAccountID, attachmentID)
	if err != nil {
		return err
	}
	if row == nil {
		return nil
	}
	if row.AttachmentKind != AttachmentKindInput || row.MessageID != nil {
		return ErrAttachmentInvalidState
	}
	if err := s.attachments.DeleteDraftAttachment(ctx, sellerAccountID, attachmentID); err != nil {
		return err
	}
	if s.inputStorage != nil && strings.TrimSpace(s.inputBucket) != "" && strings.TrimSpace(row.StorageKey) != "" {
		_ = s.inputStorage.DeleteInputAttachment(ctx, s.inputBucket, row.StorageKey)
	}
	return nil
}

func (s *Service) resolveInputAttachments(ctx context.Context, sellerAccountID int64, rawIDs []string) ([]ChatAttachment, error) {
	if len(rawIDs) == 0 {
		return nil, nil
	}
	if s.attachments == nil {
		return nil, errors.New("attachment repository is required")
	}
	ids := make([]uuid.UUID, 0, len(rawIDs))
	for _, raw := range rawIDs {
		id, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			return nil, fmt.Errorf("invalid attachment id: %s", raw)
		}
		ids = append(ids, id)
	}
	out := make([]ChatAttachment, 0, len(ids))
	for _, id := range ids {
		drafts, err := s.attachments.ListDraftAttachmentsByIDs(ctx, sellerAccountID, []uuid.UUID{id})
		if err != nil {
			return nil, err
		}
		if len(drafts) == 1 {
			if drafts[0].Status != InputAttachmentStatusUploaded {
				return nil, ErrAttachmentInvalidState
			}
			out = append(out, drafts[0])
			continue
		}
		// Follow-up asks may resubmit IDs already linked to a prior message.
		row, err := s.attachments.GetAttachmentForSeller(ctx, sellerAccountID, id)
		if err != nil {
			return nil, err
		}
		if row == nil || row.AttachmentKind != AttachmentKindInput {
			return nil, ErrAttachmentNotFound
		}
		out = append(out, *row)
	}
	if err := validateInputAttachmentBatch(out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) attachInputFilesForAsk(
	ctx context.Context,
	sellerAccountID int64,
	sessionID, messageID int64,
	items []ChatAttachment,
) ([]string, []string, error) {
	if len(items) == 0 {
		return nil, nil, nil
	}
	fileIDs := make([]string, len(items))
	warnings := make([]string, 0)
	pending := make([]ChatAttachment, 0, len(items))
	pendingIdx := make([]int, 0, len(items))
	for i, item := range items {
		if item.MessageID != nil {
			if item.OpenAIFileID != nil {
				fileIDs[i] = strings.TrimSpace(*item.OpenAIFileID)
			}
			warnings = append(warnings, "Файл "+item.FileName+" уже прикреплён к сообщению; используем сохранённый контекст.")
			continue
		}
		pending = append(pending, item)
		pendingIdx = append(pendingIdx, i)
	}
	if len(pending) == 0 {
		return fileIDs, warnings, nil
	}
	uploader, ok := s.aiClient.(fileUploadClient)
	if !ok || uploader == nil {
		return nil, nil, errors.New("openai file upload is not configured")
	}
	if s.inputStorage == nil || strings.TrimSpace(s.inputBucket) == "" {
		return nil, nil, errors.New("input attachment storage is not configured")
	}
	uploadedIDs, uploadWarnings, err := uploadAttachmentsToOpenAI(ctx, uploader, s.inputStorage, s.inputBucket, pending)
	warnings = append(warnings, uploadWarnings...)
	if err != nil {
		return fileIDs, warnings, err
	}
	for j, item := range pending {
		openAIID := ""
		if j < len(uploadedIDs) {
			openAIID = uploadedIDs[j]
		}
		idx := pendingIdx[j]
		fileIDs[idx] = openAIID
		if _, err := s.attachments.AttachAttachmentToMessage(ctx, sellerAccountID, item.ID, sessionID, messageID, openAIID); err != nil {
			return fileIDs, warnings, err
		}
	}
	return fileIDs, warnings, nil
}

func mapInputAttachmentDTO(row *ChatAttachment) *InputAttachmentDTO {
	if row == nil {
		return nil
	}
	dto := &InputAttachmentDTO{
		ID:               row.ID.String(),
		Name:             row.FileName,
		MimeType:         row.MimeType,
		SizeBytes:        row.FileSize,
		Status:           row.Status,
		ExtractionStatus: row.ExtractionStatus,
		CharacterCount:   row.CharacterCount,
	}
	return dto
}

// extractInputAttachment persists bounded attachment text for answer-context assembly.
// Extraction failure leaves the uploaded file available for OpenAI file handling.
func (s *Service) extractInputAttachment(ctx context.Context, sellerAccountID int64, attachmentID uuid.UUID, fileName, mimeType string, data []byte) (*ChatAttachment, error) {
	repo, ok := s.attachments.(attachmentExtractionSetter)
	if !ok {
		return nil, nil
	}
	extracted, err := chatextract.Extract(fileName, mimeType, data)
	status, text, extractionError := "ready", "", ""
	meta := map[string]any{}
	if err != nil {
		status, extractionError = "failed", err.Error()
		if strings.Contains(strings.ToLower(extractionError), "unsupported") {
			status = "unsupported"
		}
	} else {
		text, meta = extracted.PlainText, extracted.Metadata
		if meta == nil {
			meta = map[string]any{}
		}
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return nil, err
	}
	var pageCount, sheetCount, slideCount int32
	if extracted != nil {
		pageCount, sheetCount = int32(len(extracted.Sections)), int32(len(extracted.Sheets))
		if extracted.Format == "pptx" {
			slideCount, pageCount = int32(len(extracted.Sections)), 0
		}
	}
	return repo.setAttachmentExtraction(ctx, sellerAccountID, attachmentID, attachmentExtractionUpdate{
		Status:         status,
		Text:           text,
		Meta:           metaJSON,
		PageCount:      pageCount,
		SheetCount:     sheetCount,
		SlideCount:     slideCount,
		CharacterCount: int32(len([]rune(text))),
		Error:          extractionError,
	})
}

func (r *SQLCAttachmentRepository) setAttachmentExtraction(ctx context.Context, sellerAccountID int64, attachmentID uuid.UUID, update attachmentExtractionUpdate) (*ChatAttachment, error) {
	row, err := r.q.SetChatAttachmentExtraction(ctx, dbgen.SetChatAttachmentExtractionParams{
		SellerAccountID:  sellerAccountID,
		ID:               pgUUID(attachmentID),
		ExtractionStatus: update.Status,
		ExtractedText:    pgtype.Text{String: update.Text, Valid: update.Text != ""},
		ExtractedMeta:    update.Meta,
		PageCount:        pgtype.Int4{Int32: update.PageCount, Valid: update.PageCount > 0},
		SheetCount:       pgtype.Int4{Int32: update.SheetCount, Valid: update.SheetCount > 0},
		SlideCount:       pgtype.Int4{Int32: update.SlideCount, Valid: update.SlideCount > 0},
		CharacterCount:   pgtype.Int4{Int32: update.CharacterCount, Valid: true},
		ExtractionError:  pgtype.Text{String: update.Error, Valid: update.Error != ""},
	})
	if err != nil {
		return nil, err
	}
	return mapChatAttachment(row), nil
}

func (s *Service) setAttachmentWarningCode(ctx context.Context, sellerAccountID int64, attachmentID uuid.UUID, code string) error {
	repo, ok := s.attachments.(*SQLCAttachmentRepository)
	if !ok {
		return nil
	}
	return repo.q.SetChatAttachmentWarningCode(ctx, dbgen.SetChatAttachmentWarningCodeParams{
		SellerAccountID: sellerAccountID,
		ID:              pgUUID(attachmentID),
		WarningCode:     pgtype.Text{String: code, Valid: code != ""},
	})
}

func appendExtractedAttachmentContext(prompt string, items []ChatAttachment) string {
	var b strings.Builder
	b.WriteString(prompt)
	for _, item := range items {
		if item.ExtractedText == nil || strings.TrimSpace(*item.ExtractedText) == "" {
			continue
		}
		b.WriteString("\n\nAttached file ")
		b.WriteString(item.FileName)
		b.WriteString(" contents:\n")
		b.WriteString(*item.ExtractedText)
		if len(item.ExtractedMeta) > 0 {
			b.WriteString("\nAttachment metadata: ")
			b.Write(item.ExtractedMeta)
		}
	}
	return b.String()
}

func sanitizeAttachmentFileName(name string) string {
	base := filepath.Base(strings.TrimSpace(name))
	if base == "" || base == "." {
		return "attachment"
	}
	return base
}
