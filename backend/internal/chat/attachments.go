package chat

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	AttachmentStatusReady  = "ready"
	AttachmentStatusFailed = "failed"
)

type ChatAttachment struct {
	ID               uuid.UUID
	SellerAccountID  int64
	SessionID        *int64
	MessageID        *int64
	CreatedByUserID  *int64
	AttachmentKind   string
	ExportType       string
	Format           string
	FileName         string
	MimeType         string
	FileSize         int64
	StorageKey       string
	Status           string
	ErrorMessage     *string
	OpenAIFileID     *string
	Sha256           *string
	ExtractionStatus string
	ExtractedText    *string
	ExtractedMeta    []byte
	PageCount        *int32
	SheetCount       *int32
	SlideCount       *int32
	CharacterCount   *int32
	ExtractionError  *string
	CreatedAt        time.Time
	ExpiresAt        *time.Time
}

type CreateAttachmentInput struct {
	ID              uuid.UUID
	SellerAccountID int64
	SessionID       *int64
	MessageID       *int64
	CreatedByUserID *int64
	AttachmentKind  string
	ExportType      string
	Format          string
	FileName        string
	MimeType        string
	FileSize        int64
	StorageKey      string
	Status          string
	ErrorMessage    *string
	Sha256          *string
	ExpiresAt       *time.Time
}

type AttachmentRepository interface {
	CreateAttachment(ctx context.Context, input CreateAttachmentInput) (*ChatAttachment, error)
	GetAttachmentForSeller(ctx context.Context, sellerAccountID int64, attachmentID uuid.UUID) (*ChatAttachment, error)
	ListAttachmentsByMessageIDs(ctx context.Context, sellerAccountID int64, messageIDs []int64) ([]ChatAttachment, error)
	ListDraftAttachmentsByIDs(ctx context.Context, sellerAccountID int64, attachmentIDs []uuid.UUID) ([]ChatAttachment, error)
	CountDraftInputAttachments(ctx context.Context, sellerAccountID int64) (int64, error)
	AttachAttachmentToMessage(ctx context.Context, sellerAccountID int64, attachmentID uuid.UUID, sessionID, messageID int64, openAIFileID string) (*ChatAttachment, error)
	DeleteDraftAttachment(ctx context.Context, sellerAccountID int64, attachmentID uuid.UUID) error
}

type SQLCAttachmentRepository struct {
	q *dbgen.Queries
}

func NewSQLCAttachmentRepository(q *dbgen.Queries) *SQLCAttachmentRepository {
	return &SQLCAttachmentRepository{q: q}
}

func (r *SQLCAttachmentRepository) CreateAttachment(ctx context.Context, input CreateAttachmentInput) (*ChatAttachment, error) {
	var id pgtype.UUID
	if err := id.Scan(input.ID.String()); err != nil {
		return nil, fmt.Errorf("attachment id: %w", err)
	}
	row, err := r.q.CreateChatAttachment(ctx, dbgen.CreateChatAttachmentParams{
		ID:              id,
		SellerAccountID: input.SellerAccountID,
		ChatSessionID:   nullableInt64(input.SessionID),
		ChatMessageID:   nullableInt64(input.MessageID),
		CreatedByUserID: nullableInt64(input.CreatedByUserID),
		ExportType:      nullableText(input.ExportType),
		Format:          input.Format,
		FileName:        input.FileName,
		MimeType:        input.MimeType,
		FileSize:        input.FileSize,
		StorageKey:      input.StorageKey,
		Status:          input.Status,
		ErrorMessage:    nullableAttachmentText(input.ErrorMessage),
		ExpiresAt:       timestamptzPtrPg(input.ExpiresAt),
		AttachmentKind:  input.AttachmentKind,
		Sha256:          nullableTextPtr(input.Sha256),
	})
	if err != nil {
		return nil, fmt.Errorf("create chat attachment: %w", err)
	}
	return mapChatAttachment(row), nil
}

func (r *SQLCAttachmentRepository) GetAttachmentForSeller(
	ctx context.Context,
	sellerAccountID int64,
	attachmentID uuid.UUID,
) (*ChatAttachment, error) {
	var pgID pgtype.UUID
	if err := pgID.Scan(attachmentID.String()); err != nil {
		return nil, fmt.Errorf("attachment id: %w", err)
	}
	row, err := r.q.GetChatAttachmentByIDForSeller(ctx, dbgen.GetChatAttachmentByIDForSellerParams{
		ID:              pgID,
		SellerAccountID: sellerAccountID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get chat attachment: %w", err)
	}
	return mapChatAttachment(row), nil
}

func (r *SQLCAttachmentRepository) ListAttachmentsByMessageIDs(
	ctx context.Context,
	sellerAccountID int64,
	messageIDs []int64,
) ([]ChatAttachment, error) {
	if len(messageIDs) == 0 {
		return nil, nil
	}
	rows, err := r.q.ListChatAttachmentsByMessageIDs(ctx, dbgen.ListChatAttachmentsByMessageIDsParams{
		SellerAccountID: sellerAccountID,
		Column2:         messageIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("list chat attachments: %w", err)
	}
	out := make([]ChatAttachment, 0, len(rows))
	for _, row := range rows {
		out = append(out, *mapChatAttachment(row))
	}
	return out, nil
}

func (r *SQLCAttachmentRepository) ListDraftAttachmentsByIDs(
	ctx context.Context,
	sellerAccountID int64,
	attachmentIDs []uuid.UUID,
) ([]ChatAttachment, error) {
	if len(attachmentIDs) == 0 {
		return nil, nil
	}
	pgIDs := make([]pgtype.UUID, 0, len(attachmentIDs))
	for _, id := range attachmentIDs {
		var pgID pgtype.UUID
		if err := pgID.Scan(id.String()); err != nil {
			return nil, fmt.Errorf("attachment id: %w", err)
		}
		pgIDs = append(pgIDs, pgID)
	}
	rows, err := r.q.ListDraftChatAttachmentsByIDs(ctx, dbgen.ListDraftChatAttachmentsByIDsParams{
		SellerAccountID: sellerAccountID,
		Column2:         pgIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("list draft chat attachments: %w", err)
	}
	out := make([]ChatAttachment, 0, len(rows))
	for _, row := range rows {
		out = append(out, *mapChatAttachment(row))
	}
	return out, nil
}

func (r *SQLCAttachmentRepository) CountDraftInputAttachments(
	ctx context.Context,
	sellerAccountID int64,
) (int64, error) {
	count, err := r.q.CountDraftInputChatAttachments(ctx, sellerAccountID)
	if err != nil {
		return 0, fmt.Errorf("count draft input chat attachments: %w", err)
	}
	return count, nil
}

func (r *SQLCAttachmentRepository) AttachAttachmentToMessage(
	ctx context.Context,
	sellerAccountID int64,
	attachmentID uuid.UUID,
	sessionID, messageID int64,
	openAIFileID string,
) (*ChatAttachment, error) {
	var pgID pgtype.UUID
	if err := pgID.Scan(attachmentID.String()); err != nil {
		return nil, fmt.Errorf("attachment id: %w", err)
	}
	now := time.Now().UTC()
	row, err := r.q.AttachChatAttachmentToMessage(ctx, dbgen.AttachChatAttachmentToMessageParams{
		SellerAccountID:  sellerAccountID,
		ID:               pgID,
		ChatSessionID:    pgtype.Int8{Int64: sessionID, Valid: true},
		ChatMessageID:    pgtype.Int8{Int64: messageID, Valid: true},
		Status:           InputAttachmentStatusAttached,
		OpenaiFileID:     pgtype.Text{String: openAIFileID, Valid: openAIFileID != ""},
		OpenaiUploadedAt: pgtype.Timestamptz{Time: now, Valid: openAIFileID != ""},
	})
	if err != nil {
		return nil, fmt.Errorf("attach chat attachment: %w", err)
	}
	return mapChatAttachment(row), nil
}

func (r *SQLCAttachmentRepository) DeleteDraftAttachment(
	ctx context.Context,
	sellerAccountID int64,
	attachmentID uuid.UUID,
) error {
	var pgID pgtype.UUID
	if err := pgID.Scan(attachmentID.String()); err != nil {
		return fmt.Errorf("attachment id: %w", err)
	}
	if err := r.q.DeleteDraftChatAttachment(ctx, dbgen.DeleteDraftChatAttachmentParams{
		SellerAccountID: sellerAccountID,
		ID:              pgID,
	}); err != nil {
		return fmt.Errorf("delete draft chat attachment: %w", err)
	}
	return nil
}

func mapChatAttachment(row dbgen.ChatAttachment) *ChatAttachment {
	var errMsg *string
	if row.ErrorMessage.Valid {
		s := row.ErrorMessage.String
		errMsg = &s
	}
	var expires *time.Time
	if row.ExpiresAt.Valid {
		t := row.ExpiresAt.Time
		expires = &t
	}
	var userID *int64
	if row.CreatedByUserID.Valid {
		v := row.CreatedByUserID.Int64
		userID = &v
	}
	attID, err := uuidFromPg(row.ID)
	if err != nil {
		return nil
	}
	return &ChatAttachment{
		ID:               attID,
		SellerAccountID:  row.SellerAccountID,
		SessionID:        int8Ptr(row.ChatSessionID),
		MessageID:        int8Ptr(row.ChatMessageID),
		CreatedByUserID:  userID,
		AttachmentKind:   row.AttachmentKind,
		ExportType:       textValue(row.ExportType),
		Format:           row.Format,
		FileName:         row.FileName,
		MimeType:         row.MimeType,
		FileSize:         row.FileSize,
		StorageKey:       row.StorageKey,
		Status:           row.Status,
		ErrorMessage:     errMsg,
		OpenAIFileID:     attachmentTextPtr(row.OpenaiFileID),
		Sha256:           attachmentTextPtr(row.Sha256),
		ExtractionStatus: row.ExtractionStatus,
		ExtractedText:    attachmentTextPtr(row.ExtractedText),
		ExtractedMeta:    row.ExtractedMeta,
		PageCount:        int4Ptr(row.PageCount),
		SheetCount:       int4Ptr(row.SheetCount),
		SlideCount:       int4Ptr(row.SlideCount),
		CharacterCount:   int4Ptr(row.CharacterCount),
		ExtractionError:  attachmentTextPtr(row.ExtractionError),
		CreatedAt:        row.CreatedAt.Time,
		ExpiresAt:        expires,
	}
}

func int4Ptr(v pgtype.Int4) *int32 {
	if !v.Valid {
		return nil
	}
	value := v.Int32
	return &value
}

func textValue(v pgtype.Text) string {
	if !v.Valid {
		return ""
	}
	return v.String
}

func attachmentTextPtr(v pgtype.Text) *string {
	if !v.Valid || strings.TrimSpace(v.String) == "" {
		return nil
	}
	s := v.String
	return &s
}

func timestamptzPtrPg(v *time.Time) pgtype.Timestamptz {
	if v == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *v, Valid: true}
}

func uuidFromPg(v pgtype.UUID) (uuid.UUID, error) {
	if !v.Valid {
		return uuid.Nil, fmt.Errorf("invalid uuid")
	}
	return uuid.FromBytes(v.Bytes[:])
}

func nullableAttachmentText(v *string) pgtype.Text {
	if v == nil || *v == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *v, Valid: true}
}
