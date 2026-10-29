package chat

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

const maxAttachmentCleanupRetries = 5

type AttachmentCleanupResult struct {
	Scanned           int  `json:"scanned"`
	ExpiredCandidates int  `json:"expired_candidates"`
	MinioDeleted      int  `json:"minio_deleted"`
	OpenAIDeleted     int  `json:"openai_deleted"`
	Failed            int  `json:"failed"`
	DryRun            bool `json:"dry_run"`
}

func (s *Service) CleanupAttachments(ctx context.Context, dryRun bool) (AttachmentCleanupResult, error) {
	out := AttachmentCleanupResult{DryRun: dryRun}
	if s.attachments == nil {
		return out, fmt.Errorf("attachment repository is required")
	}
	repo, ok := s.attachments.(*SQLCAttachmentRepository)
	if !ok {
		return out, fmt.Errorf("cleanup requires SQLC attachment repository")
	}
	batch := int32(chatAttachmentCleanupBatchSize())

	expired, err := repo.q.ListExpiredDraftChatAttachments(ctx, batch)
	if err != nil {
		return out, err
	}
	out.ExpiredCandidates = len(expired)
	out.Scanned = len(expired)

	for _, row := range expired {
		if dryRun {
			continue
		}
		att := mapChatAttachment(row)
		if att == nil {
			continue
		}
		if err := s.cleanupDraftAttachment(ctx, repo, *att); err != nil {
			out.Failed++
			slog.Warn("chat attachment cleanup failed", "attachment_id", att.ID, "err", err)
		} else {
			out.MinioDeleted++
		}
	}

	pending, err := repo.q.ListOpenAICleanupPendingAttachments(ctx, dbgen.ListOpenAICleanupPendingAttachmentsParams{
		CleanupRetryCount: int32(maxAttachmentCleanupRetries),
		Limit:             batch,
	})
	if err != nil {
		return out, err
	}
	out.Scanned += len(pending)
	for _, row := range pending {
		if dryRun {
			continue
		}
		att := mapChatAttachment(row)
		if att == nil || att.OpenAIFileID == nil {
			continue
		}
		if err := s.deleteOpenAIAttachmentFile(ctx, repo, *att); err != nil {
			out.Failed++
		} else {
			out.OpenAIDeleted++
		}
	}
	return out, nil
}

func (s *Service) cleanupDraftAttachment(ctx context.Context, repo *SQLCAttachmentRepository, att ChatAttachment) error {
	if s.inputStorage != nil && strings.TrimSpace(s.inputBucket) != "" && strings.TrimSpace(att.StorageKey) != "" {
		_ = s.inputStorage.DeleteInputAttachment(ctx, s.inputBucket, att.StorageKey)
	}
	if att.OpenAIFileID != nil && strings.TrimSpace(*att.OpenAIFileID) != "" {
		if err := s.deleteOpenAIFileBestEffort(ctx, *att.OpenAIFileID); err != nil {
			_ = repo.q.IncrementChatAttachmentCleanupRetry(ctx, dbgen.IncrementChatAttachmentCleanupRetryParams{
				SellerAccountID:  att.SellerAccountID,
				ID:               pgUUID(att.ID),
				CleanupErrorCode: pgtype.Text{String: err.Error(), Valid: true},
			})
		}
	}
	_, err := repo.q.MarkChatAttachmentCleaned(ctx, dbgen.MarkChatAttachmentCleanedParams{
		SellerAccountID:  att.SellerAccountID,
		ID:               pgUUID(att.ID),
		Status:           InputAttachmentStatusDeleted,
		CleanupErrorCode: pgtype.Text{},
	})
	return err
}

func (s *Service) deleteOpenAIAttachmentFile(ctx context.Context, repo *SQLCAttachmentRepository, att ChatAttachment) error {
	if att.OpenAIFileID == nil {
		return nil
	}
	if err := s.deleteOpenAIFileBestEffort(ctx, *att.OpenAIFileID); err != nil {
		return repo.q.IncrementChatAttachmentCleanupRetry(ctx, dbgen.IncrementChatAttachmentCleanupRetryParams{
			SellerAccountID:  att.SellerAccountID,
			ID:               pgUUID(att.ID),
			CleanupErrorCode: pgtype.Text{String: err.Error(), Valid: true},
		})
	}
	_, err := repo.q.MarkChatAttachmentOpenAIDeleted(ctx, dbgen.MarkChatAttachmentOpenAIDeletedParams{
		SellerAccountID: att.SellerAccountID,
		ID:              pgUUID(att.ID),
	})
	return err
}

func (s *Service) deleteOpenAIFileBestEffort(ctx context.Context, fileID string) error {
	client, ok := s.aiClient.(fileLifecycleClient)
	if !ok || client == nil {
		return fmt.Errorf("openai file delete not configured")
	}
	return client.DeleteUserFile(ctx, fileID)
}

func (s *Service) scheduleOpenAIFileCleanup(ctx context.Context, sellerAccountID int64, attachmentID uuid.UUID) {
	if s.attachments == nil {
		return
	}
	repo, ok := s.attachments.(*SQLCAttachmentRepository)
	if !ok {
		return
	}
	_ = repo.q.MarkChatAttachmentOpenAICleanupPending(ctx, dbgen.MarkChatAttachmentOpenAICleanupPendingParams{
		SellerAccountID: sellerAccountID,
		ID:              pgUUID(attachmentID),
	})
}

func (s *Service) cleanupOpenAIFilesAfterAsk(ctx context.Context, sellerAccountID int64, items []ChatAttachment) {
	for _, item := range items {
		if item.OpenAIFileID == nil || strings.TrimSpace(*item.OpenAIFileID) == "" {
			continue
		}
		if err := s.deleteOpenAIFileBestEffort(ctx, *item.OpenAIFileID); err != nil {
			s.scheduleOpenAIFileCleanup(ctx, sellerAccountID, item.ID)
			continue
		}
		if repo, ok := s.attachments.(*SQLCAttachmentRepository); ok {
			_, _ = repo.q.MarkChatAttachmentOpenAIDeleted(ctx, dbgen.MarkChatAttachmentOpenAIDeletedParams{
				SellerAccountID: sellerAccountID,
				ID:              pgUUID(item.ID),
			})
		}
	}
}

func pgUUID(id uuid.UUID) pgtype.UUID {
	var out pgtype.UUID
	_ = out.Scan(id.String())
	return out
}
