package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

type report struct {
	GeneratedAt            string                `json:"generated_at"`
	DatabaseConnected      bool                  `json:"database_connected"`
	MinIOChecked           bool                  `json:"minio_checked"`
	StorageBucket          string                `json:"storage_bucket,omitempty"`
	ExportsBucket          string                `json:"exports_bucket,omitempty"`
	TotalAttachments       int64                 `json:"total_attachments"`
	EmptyStorageKey        int64                 `json:"empty_storage_key"`
	DraftExpired           int64                 `json:"draft_expired"`
	OrphanDBRecords        int64                 `json:"orphan_db_records"`
	OrphanStorageObjects   *int64                `json:"orphan_storage_objects"`
	MissingStorageObjects  int64                 `json:"missing_storage_objects"`
	ChecksumMismatches     *int64                `json:"checksum_mismatches"`
	BrokenMessageLinks     int64                 `json:"broken_message_links"`
	StaleGenerations       int64                 `json:"stale_generations"`
	StorageObjectsListed   int64                 `json:"storage_objects_listed,omitempty"`
	DBStorageKeysCounted   int64                 `json:"db_storage_keys_counted,omitempty"`
	ChecksumSamplesChecked int64                 `json:"checksum_samples_checked,omitempty"`
	CleanupDryRun          *cleanupDryRunSummary `json:"cleanup_dry_run,omitempty"`
	Status                 string                `json:"status"`
	Error                  string                `json:"error,omitempty"`
	Notes                  []string              `json:"notes,omitempty"`
}

type cleanupDryRunSummary struct {
	Ran                bool   `json:"ran"`
	MissingTotal       int    `json:"missing_total,omitempty"`
	OrphanTotal        int    `json:"orphan_total,omitempty"`
	WouldReconcile     int    `json:"would_reconcile,omitempty"`
	WouldDeleteOrphans int    `json:"would_delete_orphans,omitempty"`
	SkippedActive      int    `json:"skipped_active,omitempty"`
	Note               string `json:"note,omitempty"`
}

const (
	chatAttachmentPrefix = "chat-attachments/"
	chatExportPrefix     = "chat-exports/"
)

func listChatStorageObjectKeys(ctx context.Context, client *storage.Client, artifactsBucket, exportsBucket string) (map[string]struct{}, int64, error) {
	out := make(map[string]struct{})
	var total int64
	inputKeys, err := storage.ListObjectKeysByPrefix(ctx, client, artifactsBucket, chatAttachmentPrefix)
	if err != nil {
		return nil, 0, fmt.Errorf("list %s/%s: %w", artifactsBucket, chatAttachmentPrefix, err)
	}
	for _, key := range inputKeys {
		out[key] = struct{}{}
	}
	total += int64(len(inputKeys))
	exportKeys, err := storage.ListObjectKeysByPrefix(ctx, client, exportsBucket, chatExportPrefix)
	if err != nil {
		return nil, 0, fmt.Errorf("list %s/%s: %w", exportsBucket, chatExportPrefix, err)
	}
	for _, key := range exportKeys {
		out[key] = struct{}{}
	}
	total += int64(len(exportKeys))
	return out, total, nil
}

func storageBucketForKey(key, artifactsBucket, exportsBucket string) string {
	if strings.HasPrefix(key, chatExportPrefix) {
		return exportsBucket
	}
	return artifactsBucket
}

func main() {
	loadDotEnvLocal()

	out := report{
		GeneratedAt:          time.Now().UTC().Format(time.RFC3339),
		OrphanStorageObjects: nil,
		ChecksumMismatches:   nil,
		Status:               "FAIL",
		Notes:                []string{},
	}

	dsn := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dsn == "" {
		out.Error = "DATABASE_URL is not set; skipping storage integrity audit"
		writeReport(out)
		fmt.Println("SKIP:", out.Error)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		out.Error = "connect failed: " + err.Error()
		writeReport(out)
		fmt.Println("FAIL:", out.Error)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		out.Error = "ping failed: " + err.Error()
		writeReport(out)
		fmt.Println("FAIL:", out.Error)
		os.Exit(1)
	}
	out.DatabaseConnected = true

	queries := []struct {
		dest *int64
		sql  string
	}{
		{&out.TotalAttachments, `SELECT COUNT(*) FROM chat_attachments`},
		{&out.EmptyStorageKey, `
			SELECT COUNT(*) FROM chat_attachments
			WHERE (storage_key IS NULL OR TRIM(storage_key) = '')
			  AND cleaned_at IS NULL
			  AND status NOT IN ('deleted')`},
		{&out.DraftExpired, `
			SELECT COUNT(*) FROM chat_attachments
			WHERE chat_message_id IS NULL
			  AND attachment_kind = 'input'
			  AND status IN ('uploaded', 'failed')
			  AND expires_at IS NOT NULL
			  AND expires_at < NOW()
			  AND cleaned_at IS NULL`},
		{&out.BrokenMessageLinks, `
			SELECT COUNT(*) FROM chat_attachments a
			WHERE a.chat_message_id IS NOT NULL
			  AND NOT EXISTS (
			    SELECT 1 FROM chat_messages m WHERE m.id = a.chat_message_id
			  )`},
		{&out.StaleGenerations, `
			SELECT COUNT(*) FROM chat_request_idempotency
			WHERE status IN (
			    'queued', 'creating_or_loading_conversation', 'attaching_context',
			    'message_created', 'running', 'streaming'
			  )
			  AND (
			    (lease_until IS NOT NULL AND lease_until < NOW())
			    OR (lease_until IS NULL AND updated_at < NOW() - INTERVAL '30 minutes')
			  )`},
	}

	for _, q := range queries {
		if err := pool.QueryRow(ctx, q.sql).Scan(q.dest); err != nil {
			out.Error = "query failed: " + err.Error()
			writeReport(out)
			fmt.Println("FAIL:", out.Error)
			os.Exit(1)
		}
	}

	out.OrphanDBRecords = out.EmptyStorageKey

	if out.StaleGenerations > 0 {
		if samples, err := sampleStaleGenerations(ctx, pool, 5); err == nil && len(samples) > 0 {
			out.Notes = append(out.Notes, "stale_generation_samples: "+strings.Join(samples, "; "))
		}
	}

	dbKeys, keyToSHA, err := loadDBStorageKeys(ctx, pool)
	if err != nil {
		out.Error = "storage_key query failed: " + err.Error()
		writeReport(out)
		fmt.Println("FAIL:", out.Error)
		os.Exit(1)
	}
	out.DBStorageKeysCounted = int64(len(dbKeys))

	allDBKeys, err := loadAllDBStorageKeys(ctx, pool)
	if err != nil {
		out.Error = "all storage_key query failed: " + err.Error()
		writeReport(out)
		fmt.Println("FAIL:", out.Error)
		os.Exit(1)
	}

	s3Cfg, artifactsBucket, exportsBucket, ok := resolveS3Config()
	if !ok {
		out.MissingStorageObjects = out.EmptyStorageKey
		out.Notes = append(out.Notes,
			"MinIO/S3 env not configured (need S3_ENDPOINT, S3_ACCESS_KEY, S3_SECRET_KEY, S3_BUCKET_ARTIFACTS, S3_BUCKET_EXPORTS); orphan_storage_objects and checksum_mismatches remain null",
			"empty storage_key rows counted toward missing_storage_objects",
		)
		out.Status = deriveStatus(out)
		writeReport(out)
		printSummary(out)
		return
	}

	out.StorageBucket = artifactsBucket
	out.ExportsBucket = exportsBucket
	client, err := storage.New(ctx, s3Cfg)
	if err != nil {
		out.Notes = append(out.Notes, "MinIO client init failed: "+err.Error())
		out.MissingStorageObjects = out.EmptyStorageKey + int64(len(dbKeys))
		out.Status = deriveStatus(out)
		writeReport(out)
		printSummary(out)
		os.Exit(1)
	}

	objectSet, listed, err := listChatStorageObjectKeys(ctx, client, artifactsBucket, exportsBucket)
	if err != nil {
		out.Notes = append(out.Notes, "MinIO list failed: "+err.Error())
		out.MissingStorageObjects = out.EmptyStorageKey + int64(len(dbKeys))
		out.Status = deriveStatus(out)
		writeReport(out)
		printSummary(out)
		os.Exit(1)
	}

	out.MinIOChecked = true
	out.StorageObjectsListed = listed

	dbSet := make(map[string]struct{}, len(dbKeys))
	for _, key := range dbKeys {
		dbSet[key] = struct{}{}
	}

	var orphanObjects int64
	for key := range objectSet {
		if _, ok := allDBKeys[key]; !ok {
			orphanObjects++
		}
	}
	out.OrphanStorageObjects = &orphanObjects

	var missingObjects int64
	for key := range dbSet {
		if _, ok := objectSet[key]; !ok {
			missingObjects++
		}
	}
	out.MissingStorageObjects = missingObjects

	checksumMismatches, samplesChecked, checksumNote := verifyChecksums(ctx, client, artifactsBucket, exportsBucket, dbKeys, keyToSHA)
	out.ChecksumMismatches = &checksumMismatches
	out.ChecksumSamplesChecked = samplesChecked
	if checksumNote != "" {
		out.Notes = append(out.Notes, checksumNote)
	}
	out.Notes = append(out.Notes,
		"input attachments live in S3_BUCKET_ARTIFACTS under chat-attachments/*; exports live in S3_BUCKET_EXPORTS under chat-exports/*",
		"orphan_storage_objects counts object-store keys in those prefixes without any chat_attachments.storage_key reference",
		"missing_storage_objects counts active (uncleaned) DB keys absent from the correct bucket",
		"stale_generations counts chat_request_idempotency rows stuck in non-terminal status with expired lease (or updated_at > 30m when lease unset)",
	)

	if (out.OrphanStorageObjects != nil && *out.OrphanStorageObjects > 0) || out.MissingStorageObjects > 0 || out.EmptyStorageKey > 0 {
		out.CleanupDryRun = runCleanupDryRunHint(out)
	}

	out.Status = deriveStatus(out)
	writeReport(out)
	printSummary(out)

	if out.Status == "FAIL" {
		os.Exit(1)
	}
}

func runCleanupDryRunHint(out report) *cleanupDryRunSummary {
	summary := &cleanupDryRunSummary{
		Note: "dry-run only; deletions require explicit: go run ./cmd/chatstoragefix --apply after review",
	}
	if out.OrphanStorageObjects != nil {
		summary.OrphanTotal = int(*out.OrphanStorageObjects)
	}
	summary.MissingTotal = int(out.MissingStorageObjects)

	// Prefer live dry-run via chatstoragefix when orphans/missing exist.
	cmd := exec.Command("go", "run", "./cmd/chatstoragefix", "--dry-run")
	cmd.Dir = "."
	raw, err := cmd.CombinedOutput()
	if err != nil {
		summary.Ran = false
		summary.WouldDeleteOrphans = summary.OrphanTotal
		summary.WouldReconcile = summary.MissingTotal
		summary.Note = fmt.Sprintf("chatstoragefix --dry-run failed (%v); counts estimated from audit. Output: %s", err, trimOut(string(raw), 160))
		return summary
	}
	summary.Ran = true
	path := filepath.Join("debug", "ai-chat-release-gate", "json", "storage-fix-report.json")
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		summary.WouldDeleteOrphans = summary.OrphanTotal
		summary.WouldReconcile = summary.MissingTotal
		summary.Note = "dry-run completed but storage-fix-report.json unreadable; counts estimated from audit"
		return summary
	}
	var fixRep struct {
		MissingTotal   int `json:"missing_total"`
		OrphanTotal    int `json:"orphan_total"`
		Reconciled     int `json:"reconciled"`
		OrphansDeleted int `json:"orphans_deleted"`
		SkippedActive  int `json:"skipped_active"`
	}
	if json.Unmarshal(data, &fixRep) != nil {
		summary.WouldDeleteOrphans = summary.OrphanTotal
		summary.WouldReconcile = summary.MissingTotal
		return summary
	}
	summary.MissingTotal = fixRep.MissingTotal
	summary.OrphanTotal = fixRep.OrphanTotal
	summary.WouldReconcile = fixRep.Reconciled
	summary.WouldDeleteOrphans = fixRep.OrphansDeleted
	summary.SkippedActive = fixRep.SkippedActive
	summary.Note = "chatstoragefix --dry-run completed; no objects deleted (apply not run)"
	return summary
}

func trimOut(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func sampleStaleGenerations(ctx context.Context, pool *pgxpool.Pool, limit int) ([]string, error) {
	rows, err := pool.Query(ctx, `
		SELECT id::text, status, COALESCE(client_request_id, ''),
		       COALESCE(lease_until::text, 'null'), updated_at::text
		FROM chat_request_idempotency
		WHERE status IN (
		    'queued', 'creating_or_loading_conversation', 'attaching_context',
		    'message_created', 'running', 'streaming'
		  )
		  AND (
		    (lease_until IS NOT NULL AND lease_until < NOW())
		    OR (lease_until IS NULL AND updated_at < NOW() - INTERVAL '30 minutes')
		  )
		ORDER BY updated_at ASC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id, status, reqID, lease, updated string
		if err := rows.Scan(&id, &status, &reqID, &lease, &updated); err != nil {
			return nil, err
		}
		out = append(out, fmt.Sprintf("id=%s status=%s req=%s lease=%s updated=%s", id, status, reqID, lease, updated))
	}
	return out, rows.Err()
}

func loadDBStorageKeys(ctx context.Context, pool *pgxpool.Pool) ([]string, map[string]string, error) {
	rows, err := pool.Query(ctx, `
		SELECT trim(storage_key), COALESCE(trim(sha256), '')
		FROM chat_attachments
		WHERE storage_key IS NOT NULL
		  AND trim(storage_key) <> ''
		  AND cleaned_at IS NULL
		  AND status NOT IN ('deleted')`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var keys []string
	keyToSHA := make(map[string]string)
	for rows.Next() {
		var key, sha string
		if err := rows.Scan(&key, &sha); err != nil {
			return nil, nil, err
		}
		keys = append(keys, key)
		if sha != "" {
			keyToSHA[key] = strings.ToLower(sha)
		}
	}
	return keys, keyToSHA, rows.Err()
}

func loadAllDBStorageKeys(ctx context.Context, pool *pgxpool.Pool) (map[string]struct{}, error) {
	rows, err := pool.Query(ctx, `
		SELECT trim(storage_key)
		FROM chat_attachments
		WHERE storage_key IS NOT NULL AND trim(storage_key) <> ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]struct{})
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		out[key] = struct{}{}
	}
	return out, rows.Err()
}

func resolveS3Config() (storage.S3Config, string, string, bool) {
	endpoint := firstEnv("S3_ENDPOINT", "MINIO_ENDPOINT")
	accessKey := firstEnv("S3_ACCESS_KEY", "MINIO_ACCESS_KEY")
	secretKey := firstEnv("S3_SECRET_KEY", "MINIO_SECRET_KEY")
	artifactsBucket := firstEnv("S3_BUCKET_ARTIFACTS", "CHAT_STORAGE_BUCKET", "MINIO_BUCKET_ARTIFACTS")
	exportsBucket := firstEnv("S3_BUCKET_EXPORTS", "MINIO_BUCKET_EXPORTS")
	rawBucket := firstEnv("S3_BUCKET_RAW", artifactsBucket)
	if exportsBucket == "" {
		exportsBucket = artifactsBucket
	}
	useSSL := strings.EqualFold(strings.TrimSpace(os.Getenv("S3_USE_SSL")), "true") ||
		strings.EqualFold(strings.TrimSpace(os.Getenv("MINIO_USE_SSL")), "true")

	if endpoint == "" || accessKey == "" || secretKey == "" || artifactsBucket == "" {
		return storage.S3Config{}, "", "", false
	}
	return storage.S3Config{
		Endpoint:        endpoint,
		AccessKey:       accessKey,
		SecretKey:       secretKey,
		UseSSL:          useSSL,
		BucketRaw:       rawBucket,
		BucketExports:   exportsBucket,
		BucketArtifacts: artifactsBucket,
	}, artifactsBucket, exportsBucket, true
}

func verifyChecksums(ctx context.Context, client *storage.Client, artifactsBucket, exportsBucket string, keys []string, keyToSHA map[string]string) (mismatches, checked int64, note string) {
	const maxChecks = 100
	for _, key := range keys {
		expected, ok := keyToSHA[key]
		if !ok || expected == "" {
			continue
		}
		if checked >= maxChecks {
			note = fmt.Sprintf("checksum verification capped at %d objects with sha256 set", maxChecks)
			break
		}
		bucket := storageBucketForKey(key, artifactsBucket, exportsBucket)
		data, err := storage.DownloadBytes(ctx, client, bucket, key)
		if err != nil {
			continue
		}
		sum := sha256.Sum256(data)
		actual := hex.EncodeToString(sum[:])
		checked++
		if actual != expected {
			mismatches++
		}
	}
	if checked == 0 && len(keyToSHA) > 0 {
		note = "checksum verification skipped: no downloadable objects with sha256 metadata"
	}
	return mismatches, checked, note
}

func deriveStatus(out report) string {
	if out.Error != "" {
		return "FAIL"
	}
	if out.BrokenMessageLinks > 0 {
		return "FAIL"
	}
	if out.ChecksumMismatches != nil && *out.ChecksumMismatches > 0 {
		return "FAIL"
	}
	if out.StaleGenerations > 0 {
		return "WARN"
	}
	if out.EmptyStorageKey > 0 {
		return "WARN"
	}
	if out.OrphanStorageObjects != nil && *out.OrphanStorageObjects > 0 {
		return "WARN"
	}
	if out.MissingStorageObjects > 0 {
		return "WARN"
	}
	return "PASS"
}

func writeReport(out report) {
	data, _ := json.MarshalIndent(out, "", "  ")
	paths := []string{
		filepath.Join("debug", "ai-chat-release-gate", "json", "storage-integrity.json"),
		filepath.Join("debug", "ai-chat-release-gate", "storage-integrity", "storage-integrity.json"),
	}
	for _, path := range paths {
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		_ = os.WriteFile(path, data, 0o644)
		fmt.Println("wrote", path)
	}
}

func printSummary(out report) {
	orphan := "null"
	if out.OrphanStorageObjects != nil {
		orphan = fmt.Sprint(*out.OrphanStorageObjects)
	}
	checksum := "null"
	if out.ChecksumMismatches != nil {
		checksum = fmt.Sprint(*out.ChecksumMismatches)
	}
	fmt.Printf("status=%s total=%d empty_storage_key=%d missing_storage=%d orphan_storage=%s checksum_mismatches=%s stale_generations=%d broken_links=%d minio_checked=%v\n",
		out.Status, out.TotalAttachments, out.EmptyStorageKey, out.MissingStorageObjects, orphan, checksum, out.StaleGenerations, out.BrokenMessageLinks, out.MinIOChecked)
}

func loadDotEnvLocal() {
	for _, path := range []string{
		filepath.Join("..", ".env.local"),
		filepath.Join(".env.local"),
		filepath.Join("..", "..", ".env.local"),
	} {
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
			value = strings.TrimSpace(value)
			value = strings.Trim(value, `"'`)
			if key != "" && os.Getenv(key) == "" {
				_ = os.Setenv(key, value)
			}
		}
		return
	}
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}
