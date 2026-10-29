package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/minio/minio-go/v7"
)

type rowInfo struct {
	ID              string  `json:"id"`
	SellerAccountID int64   `json:"seller_account_id"`
	AttachmentKind  string  `json:"attachment_kind"`
	Status          string  `json:"status"`
	StorageKey      string  `json:"storage_key"`
	ChatMessageID   *int64  `json:"chat_message_id,omitempty"`
	ChatSessionID   *int64  `json:"chat_session_id,omitempty"`
	CleanedAt       *string `json:"cleaned_at,omitempty"`
	ExpiresAt       *string `json:"expires_at,omitempty"`
	CreatedAt       string  `json:"created_at"`
	AgeHours        float64 `json:"age_hours"`
	Bucket          string  `json:"bucket"`
}

type classification struct {
	Bucket string `json:"bucket"`
	Count  int    `json:"count"`
}

type report struct {
	GeneratedAt       string           `json:"generated_at"`
	DryRun            bool             `json:"dry_run"`
	MissingTotal      int              `json:"missing_total"`
	OrphanTotal       int              `json:"orphan_total"`
	Classification    []classification `json:"classification"`
	Sample            []rowInfo        `json:"sample,omitempty"`
	OrphanSample      []string         `json:"orphan_sample,omitempty"`
	Reconciled        int              `json:"reconciled"`
	OrphansDeleted    int              `json:"orphans_deleted"`
	SkippedActive     int              `json:"skipped_active"`
	SkippedExistsInS3 int              `json:"skipped_exists_in_s3"`
	Errors            []string         `json:"errors,omitempty"`
	Notes             []string         `json:"notes,omitempty"`
}

func main() {
	dryRun := flag.Bool("dry-run", true, "report only (default)")
	apply := flag.Bool("apply", false, "apply reconciliation")
	classifyOnly := flag.Bool("classify", false, "classification report only")
	cleanOrphans := flag.Bool("clean-orphans", true, "remove MinIO objects without DB references")
	flag.Parse()
	if *apply {
		*dryRun = false
	}

	loadDotEnvLocal()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	dsn := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL required")
		os.Exit(1)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer pool.Close()

	s3Cfg, artifactsBucket, exportsBucket, ok := resolveS3Config()
	if !ok {
		fmt.Fprintln(os.Stderr, "S3/MinIO env not configured")
		os.Exit(1)
	}
	client, err := storage.New(ctx, s3Cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	objectSet, _, err := listChatStorageObjectKeys(ctx, client, artifactsBucket, exportsBucket)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	missing, err := loadMissingRows(ctx, pool, objectSet)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	orphans, _, err := loadOrphanObjectKeys(ctx, pool, objectSet)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	rep := report{
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
		DryRun:         *dryRun,
		MissingTotal:   len(missing),
		OrphanTotal:    len(orphans),
		Classification: classifyRows(missing),
	}
	if len(missing) > 0 {
		limit := len(missing)
		if limit > 20 {
			limit = 20
		}
		rep.Sample = missing[:limit]
	}

	if len(orphans) > 0 {
		limit := len(orphans)
		if limit > 20 {
			limit = 20
		}
		rep.OrphanSample = orphans[:limit]
	}

	if *classifyOnly {
		writeReport(rep)
		printSummary(rep)
		return
	}

	for _, row := range missing {
		key := strings.TrimSpace(row.StorageKey)
		if key == "" {
			continue
		}
		if _, exists := objectSet[key]; exists {
			rep.SkippedExistsInS3++
			continue
		}
		if !isSafeToReconcile(row) {
			rep.SkippedActive++
			continue
		}
		if *dryRun {
			rep.Reconciled++
			continue
		}
		if err := reconcileRow(ctx, pool, row); err != nil {
			rep.Errors = append(rep.Errors, fmt.Sprintf("%s: %v", row.ID, err))
			continue
		}
		rep.Reconciled++
	}

	if *cleanOrphans {
		for _, key := range orphans {
			bucket := storageBucketForKey(key, artifactsBucket, exportsBucket)
			if *dryRun {
				rep.OrphansDeleted++
				continue
			}
			if err := client.Raw.RemoveObject(ctx, bucket, key, minioRemoveOptions()); err != nil {
				rep.Errors = append(rep.Errors, fmt.Sprintf("orphan %s: %v", key, err))
				continue
			}
			rep.OrphansDeleted++
		}
	}

	if purged, err := purgeReconciledGhostRows(ctx, pool, *dryRun); err != nil {
		rep.Errors = append(rep.Errors, "purge ghosts: "+err.Error())
	} else if purged > 0 {
		rep.Notes = append(rep.Notes, fmt.Sprintf("purged %d cleaned ghost rows", purged))
	}

	writeReport(rep)
	printSummary(rep)
	if len(rep.Errors) > 0 {
		os.Exit(1)
	}
}

func purgeReconciledGhostRows(ctx context.Context, pool *pgxpool.Pool, dryRun bool) (int64, error) {
	if dryRun {
		var n int64
		err := pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM chat_attachments
			WHERE cleaned_at IS NOT NULL
			  AND status IN ('deleted', 'failed')
			  AND (storage_key IS NULL OR trim(storage_key) = '')`).Scan(&n)
		return n, err
	}
	tag, err := pool.Exec(ctx, `
		DELETE FROM chat_attachments
		WHERE cleaned_at IS NOT NULL
		  AND status IN ('deleted', 'failed')
		  AND (storage_key IS NULL OR trim(storage_key) = '')`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func loadMissingRows(ctx context.Context, pool *pgxpool.Pool, objectSet map[string]struct{}) ([]rowInfo, error) {
	rows, err := pool.Query(ctx, `
		SELECT
			id::text,
			seller_account_id,
			COALESCE(attachment_kind, 'export'),
			status,
			trim(storage_key),
			chat_message_id,
			chat_session_id,
			cleaned_at,
			expires_at,
			created_at
		FROM chat_attachments
		WHERE storage_key IS NOT NULL AND trim(storage_key) <> ''
		ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var missing []rowInfo
	for rows.Next() {
		var r rowInfo
		var cleanedAt, expiresAt *time.Time
		var createdAt time.Time
		if err := rows.Scan(&r.ID, &r.SellerAccountID, &r.AttachmentKind, &r.Status, &r.StorageKey,
			&r.ChatMessageID, &r.ChatSessionID, &cleanedAt, &expiresAt, &createdAt); err != nil {
			return nil, err
		}
		r.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		r.AgeHours = time.Since(createdAt).Hours()
		if cleanedAt != nil {
			s := cleanedAt.UTC().Format(time.RFC3339)
			r.CleanedAt = &s
		}
		if expiresAt != nil {
			s := expiresAt.UTC().Format(time.RFC3339)
			r.ExpiresAt = &s
		}
		if _, ok := objectSet[r.StorageKey]; ok {
			continue
		}
		r.Bucket = reconcileBucket(r)
		missing = append(missing, r)
	}
	return missing, rows.Err()
}

func loadOrphanObjectKeys(ctx context.Context, pool *pgxpool.Pool, objectSet map[string]struct{}) ([]string, map[string]struct{}, error) {
	rows, err := pool.Query(ctx, `
		SELECT trim(storage_key)
		FROM chat_attachments
		WHERE storage_key IS NOT NULL AND trim(storage_key) <> ''`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	dbSet := make(map[string]struct{})
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, nil, err
		}
		dbSet[key] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	var orphans []string
	for key := range objectSet {
		if _, ok := dbSet[key]; !ok {
			orphans = append(orphans, key)
		}
	}
	return orphans, dbSet, nil
}

func storageBucketForKey(key, artifactsBucket, exportsBucket string) string {
	if strings.HasPrefix(key, "chat-exports/") {
		return exportsBucket
	}
	return artifactsBucket
}

func minioRemoveOptions() minio.RemoveObjectOptions {
	return minio.RemoveObjectOptions{}
}

func classifyRows(rows []rowInfo) []classification {
	counts := map[string]int{}
	for _, r := range rows {
		counts[r.Bucket]++
	}
	out := make([]classification, 0, len(counts))
	for k, v := range counts {
		out = append(out, classification{Bucket: k, Count: v})
	}
	return out
}

func reconcileBucket(r rowInfo) string {
	if r.CleanedAt != nil {
		return "already_cleaned_stale_key"
	}
	if r.ChatMessageID == nil {
		if r.AttachmentKind == "input" {
			if r.Status == "deleted" {
				return "draft_input_deleted"
			}
			return "draft_input"
		}
		return "draft_unattached"
	}
	if r.AttachmentKind == "export" {
		return "attached_export"
	}
	if r.AttachmentKind == "input" {
		return "attached_input"
	}
	return "attached_other"
}

func isSafeToReconcile(r rowInfo) bool {
	switch reconcileBucket(r) {
	case "already_cleaned_stale_key", "draft_input", "draft_input_deleted", "draft_unattached":
		return true
	case "attached_export", "attached_input", "attached_other":
		// Only reconcile attached rows when already marked cleaned or object confirmed gone for >1h.
		if r.CleanedAt != nil {
			return true
		}
		return r.AgeHours >= 1
	default:
		return false
	}
}

func reconcileRow(ctx context.Context, pool *pgxpool.Pool, r rowInfo) error {
	id, err := uuid.Parse(r.ID)
	if err != nil {
		return err
	}
	if reconcileBucket(r) == "already_cleaned_stale_key" || (r.Status == "deleted" && r.CleanedAt != nil) {
		_, err = pool.Exec(ctx, `
			DELETE FROM chat_attachments
			WHERE seller_account_id = $1
			  AND id = $2
			  AND cleaned_at IS NOT NULL`,
			r.SellerAccountID, id)
		return err
	}
	status := "deleted"
	if r.ChatMessageID != nil && r.AttachmentKind == "export" {
		status = "failed"
	}
	_, err = pool.Exec(ctx, `
		UPDATE chat_attachments
		SET
			status = $4,
			storage_key = '',
			cleaned_at = COALESCE(cleaned_at, NOW()),
			error_message = COALESCE(NULLIF(trim(error_message), ''), 'storage object missing; reconciled by chatstoragefix'),
			cleanup_error_code = 'storage_missing_reconciled',
			updated_at = NOW()
		WHERE seller_account_id = $1
		  AND id = $2
		  AND trim(storage_key) = $3`,
		r.SellerAccountID, id, r.StorageKey, status)
	return err
}

func listChatStorageObjectKeys(ctx context.Context, client *storage.Client, artifactsBucket, exportsBucket string) (map[string]struct{}, int64, error) {
	out := make(map[string]struct{})
	var total int64
	for _, spec := range []struct {
		bucket string
		prefix string
	}{
		{artifactsBucket, "chat-attachments/"},
		{exportsBucket, "chat-exports/"},
	} {
		keys, err := storage.ListObjectKeysByPrefix(ctx, client, spec.bucket, spec.prefix)
		if err != nil {
			return nil, 0, err
		}
		for _, key := range keys {
			out[key] = struct{}{}
		}
		total += int64(len(keys))
	}
	return out, total, nil
}

func resolveS3Config() (storage.S3Config, string, string, bool) {
	endpoint := firstEnv("S3_ENDPOINT", "MINIO_ENDPOINT")
	accessKey := firstEnv("S3_ACCESS_KEY", "MINIO_ACCESS_KEY")
	secretKey := firstEnv("S3_SECRET_KEY", "MINIO_SECRET_KEY")
	artifactsBucket := firstEnv("S3_BUCKET_ARTIFACTS", "CHAT_STORAGE_BUCKET", "MINIO_BUCKET_ARTIFACTS")
	exportsBucket := firstEnv("S3_BUCKET_EXPORTS", "MINIO_BUCKET_EXPORTS")
	if exportsBucket == "" {
		exportsBucket = artifactsBucket
	}
	useSSL := strings.EqualFold(strings.TrimSpace(os.Getenv("S3_USE_SSL")), "true")
	if endpoint == "" || accessKey == "" || secretKey == "" || artifactsBucket == "" {
		return storage.S3Config{}, "", "", false
	}
	return storage.S3Config{
		Endpoint: endpoint, AccessKey: accessKey, SecretKey: secretKey, UseSSL: useSSL,
		BucketRaw: firstEnv("S3_BUCKET_RAW", artifactsBucket), BucketExports: exportsBucket,
		BucketArtifacts: artifactsBucket,
	}, artifactsBucket, exportsBucket, true
}

func writeReport(rep report) {
	path := filepath.Join("debug", "ai-chat-release-gate", "json", "storage-fix-report.json")
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	data, _ := json.MarshalIndent(rep, "", "  ")
	_ = os.WriteFile(path, data, 0o644)
	fmt.Println("wrote", path)
}

func printSummary(rep report) {
	fmt.Printf("missing=%d orphans=%d reconciled=%d orphans_deleted=%d skipped_active=%d dry_run=%v\n",
		rep.MissingTotal, rep.OrphanTotal, rep.Reconciled, rep.OrphansDeleted, rep.SkippedActive, rep.DryRun)
	for _, c := range rep.Classification {
		fmt.Printf("  %s: %d\n", c.Bucket, c.Count)
	}
}

func loadDotEnvLocal() {
	for _, path := range []string{filepath.Join("..", ".env.local"), ".env.local"} {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			key, value, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			key = strings.TrimSpace(key)
			value = strings.Trim(strings.TrimSpace(value), `"'`)
			if key != "" && os.Getenv(key) == "" {
				_ = os.Setenv(key, value)
			}
		}
		return
	}
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	return ""
}
