package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/chat"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/storage"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

const TaskTypeChatAttachmentsCleanup = "chat.attachments.cleanup"

type chatAttachmentsCleanupPayload struct{}

func NewChatAttachmentsCleanupTask() (*asynq.Task, error) {
	raw, err := json.Marshal(chatAttachmentsCleanupPayload{})
	if err != nil {
		return nil, fmt.Errorf("marshal chat attachments cleanup payload: %w", err)
	}
	return asynq.NewTask(TaskTypeChatAttachmentsCleanup, raw), nil
}

type ChatAttachmentsCleanupHandler struct {
	svc *chat.Service
	log *zap.Logger
}

func NewChatAttachmentsCleanupHandler(pool *pgxpool.Pool, cfg *config.Config, s3Client *storage.Client, log *zap.Logger) (*ChatAttachmentsCleanupHandler, error) {
	if !chatAttachmentCleanupEnabled() {
		return &ChatAttachmentsCleanupHandler{log: log}, nil
	}
	queries := dbgen.New(pool)
	chatAIClient, err := chat.NewOpenAIClient(chat.OpenAIClientConfig{
		APIKey:         cfg.OpenAI.APIKey,
		Model:          cfg.OpenAI.Model,
		TimeoutSeconds: cfg.OpenAI.TimeoutSeconds,
		MaxRetries:     cfg.OpenAI.MaxRetries,
		ProxyURL:       cfg.OpenAI.AIHTTPSProxyURL(),
	})
	if err != nil {
		return nil, err
	}
	chatToolRegistry := chat.NewDefaultToolRegistry()
	chatToolSet := chat.NewToolSet(chatToolRegistry, chat.NewSQLCToolDataRepository(queries))
	svc, err := chat.NewServiceWithDeps(chat.ServiceDeps{
		Repo:             chat.NewSQLCRepository(queries),
		AIClient:         chatAIClient,
		ToolRegistry:     chatToolRegistry,
		ToolExecutor:     chatToolSet,
		ContextAssembler: chat.NewContextAssembler(),
		AnswerValidator:  chat.NewAnswerValidator(),
		Attachments:      chat.NewSQLCAttachmentRepository(queries),
		InputStorage:     chat.NewS3InputAttachmentStorage(s3Client),
		InputBucket:      cfg.S3.BucketArtifacts,
	})
	if err != nil {
		return nil, err
	}
	return &ChatAttachmentsCleanupHandler{svc: svc, log: log}, nil
}

func (h *ChatAttachmentsCleanupHandler) Handle(ctx context.Context, _ []byte) error {
	if h.svc == nil {
		return nil
	}
	result, err := h.svc.CleanupAttachments(ctx, false)
	if err != nil {
		return err
	}
	if h.log != nil {
		h.log.Info("chat attachments cleanup completed",
			zap.Int("scanned", result.Scanned),
			zap.Int("expired_candidates", result.ExpiredCandidates),
			zap.Int("minio_deleted", result.MinioDeleted),
			zap.Int("openai_deleted", result.OpenAIDeleted),
			zap.Int("failed", result.Failed),
		)
	}
	return nil
}

func chatAttachmentCleanupEnabled() bool {
	return chat.ChatAttachmentCleanupEnabled()
}
