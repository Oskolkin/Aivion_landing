package chat

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

type countingAttachmentRepo struct {
	fakeAttachmentRepo
	draftCount  int64
	created     int
	deleted     int
	rows        map[uuid.UUID]*ChatAttachment
	extractions []attachmentExtractionUpdate
}

func (r *countingAttachmentRepo) CountDraftInputAttachments(ctx context.Context, sellerAccountID int64) (int64, error) {
	return r.draftCount, nil
}

func (r *countingAttachmentRepo) CreateAttachment(ctx context.Context, input CreateAttachmentInput) (*ChatAttachment, error) {
	r.created++
	r.draftCount++
	row := &ChatAttachment{
		ID:              input.ID,
		SellerAccountID: input.SellerAccountID,
		FileName:        input.FileName,
		Status:          input.Status,
		AttachmentKind:  input.AttachmentKind,
		StorageKey:      input.StorageKey,
	}
	if r.rows == nil {
		r.rows = map[uuid.UUID]*ChatAttachment{}
	}
	r.rows[input.ID] = row
	return row, nil
}

func (r *countingAttachmentRepo) GetAttachmentForSeller(ctx context.Context, sellerAccountID int64, attachmentID uuid.UUID) (*ChatAttachment, error) {
	if row, ok := r.rows[attachmentID]; ok {
		return row, nil
	}
	return nil, nil
}

func (r *countingAttachmentRepo) DeleteDraftAttachment(ctx context.Context, sellerAccountID int64, attachmentID uuid.UUID) error {
	r.deleted++
	delete(r.rows, attachmentID)
	if r.draftCount > 0 {
		r.draftCount--
	}
	return nil
}

func (r *countingAttachmentRepo) setAttachmentExtraction(ctx context.Context, sellerAccountID int64, attachmentID uuid.UUID, update attachmentExtractionUpdate) (*ChatAttachment, error) {
	r.extractions = append(r.extractions, update)
	row := r.rows[attachmentID]
	text := update.Text
	row.ExtractionStatus = update.Status
	row.ExtractedText = &text
	row.CharacterCount = &update.CharacterCount
	return row, nil
}

type noopInputStorage struct{}

func (noopInputStorage) UploadInputAttachment(ctx context.Context, bucket, objectKey string, data []byte, contentType string) error {
	return nil
}
func (noopInputStorage) DownloadInputAttachment(ctx context.Context, bucket, objectKey string) ([]byte, error) {
	return nil, nil
}
func (noopInputStorage) DeleteInputAttachment(ctx context.Context, bucket, objectKey string) error {
	return nil
}

func TestDeleteInputAttachment_IdempotentWhenMissing(t *testing.T) {
	repo := &countingAttachmentRepo{}
	svc := &Service{
		attachments:  repo,
		inputStorage: noopInputStorage{},
		inputBucket:  "bucket",
	}
	err := svc.DeleteInputAttachment(context.Background(), 1, uuid.New())
	if err != nil {
		t.Fatalf("expected nil for missing attachment, got %v", err)
	}
}

func TestUploadInputAttachment_RejectsWhenDraftLimitReached(t *testing.T) {
	repo := &countingAttachmentRepo{draftCount: int64(chatAttachmentMaxFiles())}
	svc := &Service{
		attachments:  repo,
		inputStorage: noopInputStorage{},
		inputBucket:  "bucket",
	}
	_, err := svc.UploadInputAttachment(context.Background(), UploadInputAttachmentInput{
		SellerAccountID: 1,
		FileName:        "note.txt",
		MimeType:        "text/plain",
		Data:            []byte("hello"),
	})
	if !errors.Is(err, ErrAttachmentTooMany) {
		t.Fatalf("expected ErrAttachmentTooMany, got %v", err)
	}
	if repo.created != 0 {
		t.Fatalf("expected no attachment created, got %d", repo.created)
	}
}

func TestUploadInputAttachment_ExtractsText(t *testing.T) {
	repo := &countingAttachmentRepo{}
	svc := &Service{attachments: repo, inputStorage: noopInputStorage{}, inputBucket: "bucket"}
	dto, err := svc.UploadInputAttachment(context.Background(), UploadInputAttachmentInput{
		SellerAccountID: 1, FileName: "note.txt", MimeType: "text/plain", Data: []byte("hello extracted context"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(repo.extractions) != 1 || repo.extractions[0].Status != "ready" || repo.extractions[0].Text != "hello extracted context" {
		t.Fatalf("unexpected extraction: %+v", repo.extractions)
	}
	if dto.ExtractionStatus != "ready" || dto.CharacterCount == nil || *dto.CharacterCount == 0 {
		t.Fatalf("unexpected dto: %+v", dto)
	}
}

func TestValidateInputAttachmentContent_AllowsImageMagicBytes(t *testing.T) {
	tests := []struct {
		name string
		mime string
		data []byte
	}{
		{"chart.png", "image/png", []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}},
		{"chart.jpg", "image/jpeg", []byte{0xff, 0xd8, 0xff, 0xdb}},
		{"chart.webp", "image/webp", append(append([]byte("RIFF"), []byte{0, 0, 0, 0}...), []byte("WEBP")...)},
		{"archive.zip", "application/x-zip-compressed", []byte{0x50, 0x4b, 0x03, 0x04, 0x00, 0x00}},
	}
	for _, tt := range tests {
		if err := validateInputAttachmentContent(tt.name, tt.mime, tt.data); err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
	}
}

func TestDeleteInputAttachment_RemovesExistingDraft(t *testing.T) {
	id := uuid.New()
	repo := &countingAttachmentRepo{
		draftCount: 1,
		rows: map[uuid.UUID]*ChatAttachment{
			id: {
				ID:              id,
				SellerAccountID: 1,
				AttachmentKind:  AttachmentKindInput,
				StorageKey:      "chat-attachments/1/" + id.String(),
			},
		},
	}
	svc := &Service{
		attachments:  repo,
		inputStorage: noopInputStorage{},
		inputBucket:  "bucket",
	}
	if err := svc.DeleteInputAttachment(context.Background(), 1, id); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if repo.deleted != 1 {
		t.Fatalf("expected one delete, got %d", repo.deleted)
	}
}

func TestUploadInputAttachment_AllowsUploadWhenDeletedDraftsExist(t *testing.T) {
	// CountDraftInputAttachments must exclude soft-deleted drafts; the repo
	// under test models an active-only count (starts at 0 while max is 5).
	repo := &countingAttachmentRepo{draftCount: 0}
	svc := &Service{
		attachments:  repo,
		inputStorage: noopInputStorage{},
		inputBucket:  "test-bucket",
	}
	for i := 0; i < chatAttachmentMaxFiles(); i++ {
		_, err := svc.UploadInputAttachment(context.Background(), UploadInputAttachmentInput{
			SellerAccountID: 1,
			FileName:        "control.txt",
			MimeType:        "text/plain",
			Data:            []byte("TXT_CONTROL_16735"),
		})
		if err != nil {
			t.Fatalf("upload %d: %v", i+1, err)
		}
	}
	_, err := svc.UploadInputAttachment(context.Background(), UploadInputAttachmentInput{
		SellerAccountID: 1,
		FileName:        "control.txt",
		MimeType:        "text/plain",
		Data:            []byte("TXT_CONTROL_16735"),
	})
	if !errors.Is(err, ErrAttachmentTooMany) {
		t.Fatalf("expected ErrAttachmentTooMany after max active drafts, got %v", err)
	}
}
