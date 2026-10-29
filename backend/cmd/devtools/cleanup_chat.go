package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/chat"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/storage"
)

func runCleanupDuplicateActionChats(args []string) int {
	fs := flag.NewFlagSet("cleanup-duplicate-action-chats", flag.ExitOnError)
	dryRun := fs.Bool("dry-run", false, "report only")
	apply := fs.Bool("apply", false, "apply cleanup")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *dryRun == *apply {
		fmt.Fprintln(os.Stderr, "specify exactly one of --dry-run or --apply")
		return 2
	}

	_ = config.LoadEnvFiles()
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	ctx := context.Background()
	postgres, err := db.New(ctx, cfg.DB.URL)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer postgres.Close()

	result, err := chat.CleanupDuplicateActionChats(ctx, postgres.Pool, *dryRun)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(result)
	return 0
}

func runCleanupChatAttachments(args []string) int {
	fs := flag.NewFlagSet("cleanup-chat-attachments", flag.ExitOnError)
	dryRun := fs.Bool("dry-run", false, "report only")
	apply := fs.Bool("apply", false, "apply cleanup")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *dryRun == *apply {
		fmt.Fprintln(os.Stderr, "specify exactly one of --dry-run or --apply")
		return 2
	}

	_ = config.LoadEnvFiles()
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	ctx := context.Background()
	postgres, err := db.New(ctx, cfg.DB.URL)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer postgres.Close()

	s3Client, err := storage.New(ctx, storage.S3Config{
		Endpoint:        cfg.S3.Endpoint,
		AccessKey:       cfg.S3.AccessKey,
		SecretKey:       cfg.S3.SecretKey,
		UseSSL:          cfg.S3.UseSSL,
		BucketRaw:       cfg.S3.BucketRaw,
		BucketExports:   cfg.S3.BucketExports,
		BucketArtifacts: cfg.S3.BucketArtifacts,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	queries := dbgen.New(postgres.Pool)
	chatAIClient, err := chat.NewOpenAIClient(chat.OpenAIClientConfig{
		APIKey:         cfg.OpenAI.APIKey,
		Model:          cfg.OpenAI.Model,
		TimeoutSeconds: cfg.OpenAI.TimeoutSeconds,
		MaxRetries:     cfg.OpenAI.MaxRetries,
		ProxyURL:       cfg.OpenAI.AIHTTPSProxyURL(),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
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
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	result, err := svc.CleanupAttachments(ctx, *dryRun)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(result)
	return 0
}
