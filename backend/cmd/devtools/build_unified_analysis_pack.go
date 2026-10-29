package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/storage"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	uapack "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/pack"
)

func runBuildUnifiedAnalysisPack(args []string) int {
	fs := flag.NewFlagSet("build-unified-analysis-pack", flag.ContinueOnError)
	var (
		sellerID         int64
		metricDateRaw    string
		outputDir        string
		upload           bool
		validate         bool
		printManifest    bool
		printLimitations bool
		overwrite        bool
	)
	fs.Int64Var(&sellerID, "seller-id", 0, "seller_account_id (required)")
	fs.StringVar(&metricDateRaw, "metric-date", "", "metric date YYYY-MM-DD (required)")
	fs.StringVar(&outputDir, "output", "./debug/unified-analysis-pack", "output directory for zip and summary artifacts")
	fs.BoolVar(&upload, "upload", false, "upload ZIP to MinIO artifacts bucket")
	fs.BoolVar(&validate, "validate", true, "run technical pack validator")
	fs.BoolVar(&printManifest, "print-manifest", false, "print manifest.json to stdout")
	fs.BoolVar(&printLimitations, "print-limitations", false, "print limitations.json to stdout")
	fs.BoolVar(&overwrite, "overwrite", true, "overwrite existing output files")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if sellerID <= 0 {
		fmt.Fprintln(os.Stderr, "seller-id is required and must be > 0")
		return 2
	}
	if strings.TrimSpace(metricDateRaw) == "" {
		fmt.Fprintln(os.Stderr, "metric-date is required (YYYY-MM-DD)")
		return 2
	}
	metricDate, err := parseMetricDateCLI(metricDateRaw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "metric-date: %v\n", err)
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

	loader := analysisdata.NewSQLLoader(postgres.Pool)
	builder := uapack.NewPackBuilder(uapack.DefaultBuildOptions())
	validator := uapack.NewValidator(uapack.DefaultBuildOptions())

	var packStorage uapack.PackStorage
	if upload {
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
			fmt.Fprintf(os.Stderr, "connect minio: %v\n", err)
			return 1
		}
		packStorage = uapack.NewS3PackStorage(s3Client)
	}

	svc := uapack.NewService(loader, builder, validator, packStorage)

	started := time.Now()
	var (
		result    *uapack.BuildResult
		objectKey string
	)
	if upload {
		result, objectKey, err = svc.BuildAndStore(ctx, sellerID, metricDate)
	} else {
		result, err = svc.Build(ctx, sellerID, metricDate)
	}
	elapsed := time.Since(started)

	if result == nil {
		fmt.Fprintf(os.Stderr, "build pack: %v\n", err)
		return 1
	}
	if err != nil && !errors.Is(err, uapack.ErrValidationFailed) && !errors.Is(err, uapack.ErrBusinessConsistencyFailed) {
		fmt.Fprintf(os.Stderr, "build pack: %v\n", err)
		return 1
	}
	if validate && !result.Validation.Valid {
		fmt.Fprintf(os.Stderr, "validation failed (%d issues)\n", len(result.Validation.Issues))
		for _, issue := range result.Validation.Issues {
			fmt.Fprintf(os.Stderr, "  [%s] %s: %s\n", issue.Code, issue.File, issue.Message)
		}
		if errors.Is(err, uapack.ErrValidationFailed) {
			// still write artifacts for diagnosis
		} else {
			return 1
		}
	}
	if result.BusinessConsistency.HasErrors() {
		fmt.Fprintf(os.Stderr, "business consistency failed (%d errors)\n", len(result.BusinessConsistency.Errors))
		for _, issue := range result.BusinessConsistency.Errors {
			fmt.Fprintf(os.Stderr, "  [%s] %s: %s\n", issue.Code, issue.SKU, issue.Message)
		}
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir output: %v\n", err)
		return 1
	}
	zipPath := filepath.Join(outputDir, "analysis_pack.zip")
	if !overwrite {
		if _, err := os.Stat(zipPath); err == nil {
			fmt.Fprintf(os.Stderr, "output exists (use --overwrite): %s\n", zipPath)
			return 2
		}
	}
	if err := os.WriteFile(zipPath, result.ZipBytes, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write zip: %v\n", err)
		return 1
	}

	manifestRaw, _ := json.MarshalIndent(result.Manifest, "", "  ")
	_ = os.WriteFile(filepath.Join(outputDir, "manifest.json"), append(manifestRaw, '\n'), 0o644)
	limDoc := contract.LimitationsDocumentV1{
		SchemaVersion: result.SchemaVersion,
		Limitations:   result.Limitations,
	}
	limRaw, _ := json.MarshalIndent(limDoc, "", "  ")
	_ = os.WriteFile(filepath.Join(outputDir, "limitations.json"), append(limRaw, '\n'), 0o644)
	valRaw, _ := json.MarshalIndent(result.Validation, "", "  ")
	_ = os.WriteFile(filepath.Join(outputDir, "validation.json"), append(valRaw, '\n'), 0o644)
	bizRaw, _ := json.MarshalIndent(result.BusinessConsistency, "", "  ")
	_ = os.WriteFile(filepath.Join(outputDir, "business_consistency.json"), append(bizRaw, '\n'), 0o644)
	fmt.Println(result.BusinessConsistency.SummaryLine())

	archiveSHA := sha256.Sum256(result.ZipBytes)
	rowCounts := map[string]int{}
	for _, f := range result.Files {
		if strings.HasSuffix(f.Name, ".csv") {
			rowCounts[f.Name] = f.RowCount
		}
	}
	summary := map[string]any{
		"seller_id":               sellerID,
		"metric_date":             result.MetricDate,
		"pack_id":                 result.PackID,
		"pack_hash":               result.PackHash,
		"archive_sha256":          hex.EncodeToString(archiveSHA[:]),
		"compressed_bytes":        len(result.ZipBytes),
		"uncompressed_bytes":      result.Validation.UncompressedBytes,
		"storage_key":             objectKey,
		"validation_status":       result.Validation.Valid,
		"business_consistency_ok": !result.BusinessConsistency.HasErrors(),
		"business_errors":         len(result.BusinessConsistency.Errors),
		"business_warnings":       len(result.BusinessConsistency.Warnings),
		"eligibility":             result.BusinessConsistency.Eligibility,
		"limitations_count":       len(result.Limitations),
		"row_counts":              rowCounts,
		"build_duration_ms":       elapsed.Milliseconds(),
		"output_dir":              outputDir,
	}
	summaryRaw, _ := json.MarshalIndent(summary, "", "  ")
	_ = os.WriteFile(filepath.Join(outputDir, "summary.json"), append(summaryRaw, '\n'), 0o644)

	fmt.Printf("seller_id=%d\n", sellerID)
	fmt.Printf("metric_date=%s\n", result.MetricDate)
	fmt.Printf("pack_id=%s\n", result.PackID)
	fmt.Printf("pack_hash=%s\n", result.PackHash)
	fmt.Printf("archive_sha256=%s\n", hex.EncodeToString(archiveSHA[:]))
	fmt.Printf("compressed_bytes=%d\n", len(result.ZipBytes))
	fmt.Printf("uncompressed_bytes=%d\n", result.Validation.UncompressedBytes)
	fmt.Printf("storage_key=%s\n", objectKey)
	fmt.Printf("validation_status=%v\n", result.Validation.Valid)
	fmt.Printf("limitations_count=%d\n", len(result.Limitations))
	fmt.Printf("build_duration_ms=%d\n", elapsed.Milliseconds())
	fmt.Println("row_counts:")
	for _, name := range []string{
		"account_summary.csv", "account_daily.csv", "sku_summary.csv", "sku_daily.csv",
		"campaign_summary.csv", "campaign_daily.csv", "campaign_sku.csv", "product_enrichment.csv",
	} {
		fmt.Printf("  %s=%d\n", name, rowCounts[name])
	}
	fmt.Printf("output=%s\n", outputDir)

	if printManifest {
		fmt.Println(string(manifestRaw))
	}
	if printLimitations {
		fmt.Println(string(limRaw))
	}
	if errors.Is(err, uapack.ErrValidationFailed) || errors.Is(err, uapack.ErrBusinessConsistencyFailed) {
		return 1
	}
	return 0
}
