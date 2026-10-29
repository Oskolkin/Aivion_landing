package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/devseed"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/storage"
)

func runResetDemoSellerData(args []string) int {
	fs := flag.NewFlagSet("reset-demo-seller-data", flag.ContinueOnError)
	var sellerID int64
	var confirm bool
	var dryRun bool
	var output string
	fs.Int64Var(&sellerID, "seller-id", 0, "seller_account_id (required)")
	fs.BoolVar(&confirm, "confirm", false, "confirm destructive reset (required when --dry-run=false)")
	fs.BoolVar(&dryRun, "dry-run", true, "plan deletion without mutating DB/object store")
	fs.StringVar(&output, "output", "", "optional path to write JSON report")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if sellerID <= 0 {
		fmt.Fprintln(os.Stderr, "seller-id is required")
		return 2
	}
	if !dryRun && !confirm {
		fmt.Fprintln(os.Stderr, "--confirm is required when --dry-run=false")
		return 2
	}
	if err := devseed.AssertDevSeedAllowed(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 2
	}

	_ = config.LoadEnvFiles()
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		return 1
	}
	ctx := context.Background()
	postgres, err := db.New(ctx, cfg.DB.URL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect postgres: %v\n", err)
		return 1
	}
	defer postgres.Close()

	var objectStore devseed.SellerObjectStore
	s3Client, s3Err := storage.New(ctx, storage.S3Config{
		Endpoint:        cfg.S3.Endpoint,
		AccessKey:       cfg.S3.AccessKey,
		SecretKey:       cfg.S3.SecretKey,
		UseSSL:          cfg.S3.UseSSL,
		BucketRaw:       cfg.S3.BucketRaw,
		BucketExports:   cfg.S3.BucketExports,
		BucketArtifacts: cfg.S3.BucketArtifacts,
	})
	if s3Err != nil {
		fmt.Fprintf(os.Stderr, "warning: object store unavailable (%v); DB reset will continue without object listing/deletion\n", s3Err)
	} else {
		objectStore = devseed.StorageSellerObjectStore{Client: s3Client}
	}

	var asynqChecker devseed.SellerAsynqChecker
	if checker := devseed.NewAsynqInspectorFromRedisConfig(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB); checker != nil {
		asynqChecker = checker
	} else {
		fmt.Fprintln(os.Stderr, "warning: asynq inspector unavailable (empty redis addr)")
	}

	report, err := devseed.ResetSellerTestData(ctx, postgres.Pool, objectStore, asynqChecker, devseed.SellerResetOptions{
		SellerAccountID:  sellerID,
		DryRun:           dryRun,
		PreserveIdentity: true,
		PreserveSettings: true,
	})
	raw, _ := devseed.MarshalSellerResetReport(report)
	if output != "" {
		if mkErr := os.MkdirAll(filepath.Dir(output), 0o755); mkErr != nil {
			fmt.Fprintf(os.Stderr, "create output dir: %v\n", mkErr)
			return 1
		}
		if writeErr := os.WriteFile(output, raw, 0o644); writeErr != nil {
			fmt.Fprintf(os.Stderr, "write output: %v\n", writeErr)
			return 1
		}
	}
	fmt.Println(string(raw))
	if err != nil {
		fmt.Fprintf(os.Stderr, "reset failed: %v\n", err)
		return 1
	}
	return 0
}
