package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/adsync"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/rawpayloads"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/storage"
)

func main() {
	var sellerAccountID int64
	var importJobID int64
	var sourceCursor string
	flag.Int64Var(&sellerAccountID, "seller-account-id", 0, "target seller_account_id (required)")
	flag.Int64Var(&importJobID, "import-job-id", 0, "optional existing import_job_id for raw payload linkage")
	flag.StringVar(&sourceCursor, "source-cursor", "", "optional RFC3339 cursor for incremental sync")
	flag.Parse()

	if sellerAccountID <= 0 {
		fmt.Fprintln(os.Stderr, "seller-account-id is required and must be > 0")
		os.Exit(2)
	}

	_ = config.LoadEnvFiles()
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	postgres, err := db.New(ctx, cfg.DB.URL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect postgres: %v\n", err)
		os.Exit(1)
	}
	defer postgres.Close()

	ozonService, err := ozon.NewService(
		postgres.Pool,
		cfg.Auth.EncryptionKey,
		cfg.Ozon.PerformanceAuthBaseURL,
		cfg.Ozon.DisconnectRetentionDays,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "init ozon service: %v\n", err)
		os.Exit(1)
	}

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
		fmt.Fprintf(os.Stderr, "connect s3: %v\n", err)
		os.Exit(1)
	}

	rawPayloadsService := rawpayloads.NewService(postgres.Pool, s3Client, cfg.S3.BucketRaw)
	service := adsync.NewService(postgres.Pool, ozonService, rawPayloadsService)

	result, err := service.Run(ctx, adsync.RunInput{
		SellerAccountID: sellerAccountID,
		ImportJobID:     importJobID,
		SourceCursor:    sourceCursor,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "advertising ingest failed: %v\n", err)
		os.Exit(1)
	}

	payload, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal output: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(payload))
}
