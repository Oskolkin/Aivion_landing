package devseed

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/storage"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SellerResetOptions configures ResetSellerTestData.
type SellerResetOptions struct {
	SellerAccountID  int64
	DryRun           bool
	PreserveIdentity bool // kept for API clarity; identity is always preserved
	PreserveSettings bool // when true (default), seller_analysis_settings is retained
}

// SellerResetTableResult records one table delete/count step.
type SellerResetTableResult struct {
	Table       string `json:"table"`
	Deleted     int64  `json:"deleted"`
	WouldDelete int64  `json:"would_delete,omitempty"`
	Via         string `json:"via,omitempty"`
}

// SellerResetReport is the full reset plan or execution result.
type SellerResetReport struct {
	SellerAccountID   int64                    `json:"seller_account_id"`
	DryRun            bool                     `json:"dry_run"`
	Tables            []SellerResetTableResult `json:"tables"`
	ObjectKeys        []string                 `json:"object_keys"`
	ObjectKeysDeleted []string                 `json:"object_keys_deleted,omitempty"`
	ActiveRuns        []int64                  `json:"active_runs"`
	ActiveDispatches  []int64                  `json:"active_dispatches"`
	ActiveSyncJobs    []int64                  `json:"active_sync_jobs"`
	ActiveImportJobs  []int64                  `json:"active_import_jobs"`
	ActiveAsynqTasks  []string                 `json:"active_asynq_tasks,omitempty"`
	Preserved         []string                 `json:"preserved"`
	PublicationIDs    []int64                  `json:"publication_ids"`
	PipelineRunIDs    []int64                  `json:"pipeline_run_ids"`
	BlockedReason     string                   `json:"blocked_reason,omitempty"`
}

// SellerObjectStore abstracts MinIO/S3 operations needed for seller-scoped cleanup.
type SellerObjectStore interface {
	ListObjectKeysByPrefix(ctx context.Context, bucket, prefix string) ([]string, error)
	DeleteObject(ctx context.Context, bucket, objectKey string) error
	ArtifactsBucket() string
	RawBucket() string
}

// StorageSellerObjectStore adapts storage.Client to SellerObjectStore.
type StorageSellerObjectStore struct {
	Client *storage.Client
}

func (s StorageSellerObjectStore) ListObjectKeysByPrefix(ctx context.Context, bucket, prefix string) ([]string, error) {
	return storage.ListObjectKeysByPrefix(ctx, s.Client, bucket, prefix)
}

func (s StorageSellerObjectStore) DeleteObject(ctx context.Context, bucket, objectKey string) error {
	return storage.DeleteObject(ctx, s.Client, bucket, objectKey)
}

func (s StorageSellerObjectStore) ArtifactsBucket() string {
	if s.Client == nil {
		return ""
	}
	return s.Client.Config.BucketArtifacts
}

func (s StorageSellerObjectStore) RawBucket() string {
	if s.Client == nil {
		return ""
	}
	return s.Client.Config.BucketRaw
}

// SellerAsynqChecker optionally detects in-flight Asynq tasks for a seller.
type SellerAsynqChecker interface {
	ListActiveSellerTasks(ctx context.Context, sellerAccountID int64) ([]string, error)
}

// AsynqInspectorSellerChecker scans Redis-backed Asynq queues for seller-scoped tasks.
type AsynqInspectorSellerChecker struct {
	Inspector *asynq.Inspector
}

func NewAsynqInspectorSellerChecker(redisOpt asynq.RedisConnOpt) *AsynqInspectorSellerChecker {
	if redisOpt == nil {
		return nil
	}
	return &AsynqInspectorSellerChecker{Inspector: asynq.NewInspector(redisOpt)}
}

func NewAsynqInspectorFromRedisConfig(addr, password string, db int) *AsynqInspectorSellerChecker {
	if strings.TrimSpace(addr) == "" {
		return nil
	}
	return NewAsynqInspectorSellerChecker(asynq.RedisClientOpt{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
}

func (c *AsynqInspectorSellerChecker) ListActiveSellerTasks(ctx context.Context, sellerAccountID int64) ([]string, error) {
	if c == nil || c.Inspector == nil || sellerAccountID <= 0 {
		return nil, nil
	}
	_ = ctx
	queues, err := c.Inspector.Queues()
	if err != nil {
		return nil, fmt.Errorf("list asynq queues: %w", err)
	}
	needleJSON := fmt.Sprintf(`"seller_account_id":%d`, sellerAccountID)
	needleSpacedJSON := fmt.Sprintf(`"seller_account_id": %d`, sellerAccountID)
	out := make([]string, 0)
	seen := map[string]struct{}{}
	add := func(id string) {
		if id == "" {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	payloadBelongsToSeller := func(payload string) bool {
		// Exact JSON number match: avoid seller_id=1 matching seller_id=10/11/...
		for _, needle := range []string{needleJSON, needleSpacedJSON} {
			idx := strings.Index(payload, needle)
			for idx >= 0 {
				end := idx + len(needle)
				if end >= len(payload) {
					return true
				}
				next := payload[end]
				if next < '0' || next > '9' {
					return true
				}
				idx = strings.Index(payload[end:], needle)
				if idx >= 0 {
					idx += end
				}
			}
		}
		return false
	}
	taskIDPrefix := fmt.Sprintf("seller-ai-pipeline:%d:", sellerAccountID)
	dailyPrefix := fmt.Sprintf("ai-daily-summaries:%d:", sellerAccountID)
	for _, q := range queues {
		for _, listFn := range []func(string, ...asynq.ListOption) ([]*asynq.TaskInfo, error){
			c.Inspector.ListActiveTasks,
			c.Inspector.ListPendingTasks,
			c.Inspector.ListScheduledTasks,
			c.Inspector.ListRetryTasks,
		} {
			tasks, listErr := listFn(q)
			if listErr != nil {
				return nil, fmt.Errorf("list asynq tasks queue=%s: %w", q, listErr)
			}
			for _, t := range tasks {
				if t == nil {
					continue
				}
				payload := string(t.Payload)
				if payloadBelongsToSeller(payload) ||
					strings.HasPrefix(t.ID, taskIDPrefix) || strings.HasPrefix(t.ID, dailyPrefix) {
					add(fmt.Sprintf("%s/%s", q, t.ID))
				}
			}
		}
	}
	sort.Strings(out)
	return out, nil
}

// SellerAnalysisPackPrefix returns the exact MinIO prefix for unified packs of one seller.
// Trailing slash prevents matching seller-10 when sellerID=1.
func SellerAnalysisPackPrefix(sellerAccountID int64) string {
	return fmt.Sprintf("ai-analysis-packs/seller-%d/", sellerAccountID)
}

func SellerChatAttachmentPrefix(sellerAccountID int64) string {
	return fmt.Sprintf("chat-attachments/%d/", sellerAccountID)
}

func SellerRawPayloadPrefix(sellerAccountID int64) string {
	return fmt.Sprintf("raw/ozon/%d/", sellerAccountID)
}

// ResetSellerTestData is the single seller cleanup implementation for MVP seed and
// reset-demo-seller-data. Any SQL/object error aborts and rolls back the DB transaction.
func ResetSellerTestData(
	ctx context.Context,
	pool *pgxpool.Pool,
	objectStore SellerObjectStore,
	asynqChecker SellerAsynqChecker,
	opts SellerResetOptions,
) (SellerResetReport, error) {
	report := SellerResetReport{
		SellerAccountID: opts.SellerAccountID,
		DryRun:          opts.DryRun,
		Preserved: []string{
			"users",
			"seller_accounts",
			"user_seller_links",
			"sessions",
			"auth_credentials",
			"seller_analysis_settings",
		},
	}
	opts.PreserveIdentity = true
	opts.PreserveSettings = true
	if opts.SellerAccountID <= 0 {
		return report, fmt.Errorf("seller_account_id must be > 0")
	}

	var exists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM seller_accounts WHERE id = $1)`, opts.SellerAccountID).Scan(&exists); err != nil {
		return report, fmt.Errorf("check seller exists: %w", err)
	}
	if !exists {
		return report, fmt.Errorf("seller_account_id %d does not exist", opts.SellerAccountID)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return report, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, opts.SellerAccountID); err != nil {
		return report, fmt.Errorf("advisory lock: %w", err)
	}

	if err := collectSellerResetIDs(ctx, tx, opts.SellerAccountID, &report); err != nil {
		return report, err
	}
	if err := detectBlockingSellerActivity(ctx, tx, opts.SellerAccountID, &report); err != nil {
		return report, err
	}
	if asynqChecker != nil {
		tasks, err := asynqChecker.ListActiveSellerTasks(ctx, opts.SellerAccountID)
		if err != nil {
			return report, fmt.Errorf("asynq check: %w", err)
		}
		report.ActiveAsynqTasks = tasks
		if len(tasks) > 0 {
			report.BlockedReason = fmt.Sprintf("active asynq tasks: %s", strings.Join(tasks, ", "))
			return report, fmt.Errorf("reset blocked: %s", report.BlockedReason)
		}
	}
	if report.BlockedReason != "" {
		return report, fmt.Errorf("reset blocked: %s", report.BlockedReason)
	}

	objectKeys, err := collectSellerObjectKeys(ctx, tx, objectStore, opts.SellerAccountID, &report)
	if err != nil {
		return report, err
	}
	report.ObjectKeys = objectKeys

	if opts.DryRun {
		if err := planSellerTableDeletes(ctx, tx, opts, &report); err != nil {
			return report, err
		}
		// Dry-run must not mutate; rollback intentionally.
		return report, nil
	}

	if err := executeSellerTableDeletes(ctx, tx, opts, &report); err != nil {
		return report, err
	}
	if err := tx.Commit(ctx); err != nil {
		return report, fmt.Errorf("commit: %w", err)
	}

	deletedKeys, err := deleteSellerObjects(ctx, objectStore, objectKeys)
	report.ObjectKeysDeleted = deletedKeys
	if err != nil {
		return report, fmt.Errorf("object cleanup incomplete (db reset committed; re-run to finish): %w", err)
	}
	return report, nil
}

func collectSellerResetIDs(ctx context.Context, tx pgx.Tx, sellerID int64, report *SellerResetReport) error {
	rows, err := tx.Query(ctx, `SELECT id FROM ai_pipeline_runs WHERE seller_account_id = $1 ORDER BY id`, sellerID)
	if err != nil {
		return fmt.Errorf("list pipeline runs: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return err
		}
		report.PipelineRunIDs = append(report.PipelineRunIDs, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	pubRows, err := tx.Query(ctx, `SELECT id FROM unified_analysis_publications WHERE seller_account_id = $1 ORDER BY id`, sellerID)
	if err != nil {
		return fmt.Errorf("list publications: %w", err)
	}
	defer pubRows.Close()
	for pubRows.Next() {
		var id int64
		if err := pubRows.Scan(&id); err != nil {
			return err
		}
		report.PublicationIDs = append(report.PublicationIDs, id)
	}
	return pubRows.Err()
}

func detectBlockingSellerActivity(ctx context.Context, tx pgx.Tx, sellerID int64, report *SellerResetReport) error {
	// Non-terminal unified / pipeline runs.
	runRows, err := tx.Query(ctx, `
SELECT id FROM ai_pipeline_runs
WHERE seller_account_id = $1
  AND status NOT IN (
    'failed','cancelled','publication_blocked','published','publication_failed','cleanup_completed',
    'completed','completed_partial','context_failed','dashboard_failed','actions_failed','empty'
  )
ORDER BY id`, sellerID)
	if err != nil {
		return fmt.Errorf("scan active pipeline runs: %w", err)
	}
	defer runRows.Close()
	for runRows.Next() {
		var id int64
		if err := runRows.Scan(&id); err != nil {
			return err
		}
		report.ActiveRuns = append(report.ActiveRuns, id)
	}
	if err := runRows.Err(); err != nil {
		return err
	}

	dispRows, err := tx.Query(ctx, `
SELECT id FROM ai_pipeline_dispatches
WHERE seller_account_id = $1
  AND status NOT IN ('completed','failed')
ORDER BY id`, sellerID)
	if err != nil {
		return fmt.Errorf("scan active dispatches: %w", err)
	}
	defer dispRows.Close()
	for dispRows.Next() {
		var id int64
		if err := dispRows.Scan(&id); err != nil {
			return err
		}
		report.ActiveDispatches = append(report.ActiveDispatches, id)
	}
	if err := dispRows.Err(); err != nil {
		return err
	}

	syncRows, err := tx.Query(ctx, `
SELECT id FROM sync_jobs
WHERE seller_account_id = $1 AND status IN ('pending','running')
ORDER BY id`, sellerID)
	if err != nil {
		return fmt.Errorf("scan active sync jobs: %w", err)
	}
	defer syncRows.Close()
	for syncRows.Next() {
		var id int64
		if err := syncRows.Scan(&id); err != nil {
			return err
		}
		report.ActiveSyncJobs = append(report.ActiveSyncJobs, id)
	}
	if err := syncRows.Err(); err != nil {
		return err
	}

	importRows, err := tx.Query(ctx, `
SELECT id FROM import_jobs
WHERE seller_account_id = $1 AND status IN ('pending','fetching','importing','retrying')
ORDER BY id`, sellerID)
	if err != nil {
		return fmt.Errorf("scan active import jobs: %w", err)
	}
	defer importRows.Close()
	for importRows.Next() {
		var id int64
		if err := importRows.Scan(&id); err != nil {
			return err
		}
		report.ActiveImportJobs = append(report.ActiveImportJobs, id)
	}
	if err := importRows.Err(); err != nil {
		return err
	}

	var blockers []string
	if len(report.ActiveRuns) > 0 {
		blockers = append(blockers, fmt.Sprintf("non-terminal ai_pipeline_runs=%v", report.ActiveRuns))
	}
	if len(report.ActiveDispatches) > 0 {
		blockers = append(blockers, fmt.Sprintf("active ai_pipeline_dispatches=%v", report.ActiveDispatches))
	}
	if len(report.ActiveSyncJobs) > 0 {
		blockers = append(blockers, fmt.Sprintf("active sync_jobs=%v", report.ActiveSyncJobs))
	}
	if len(report.ActiveImportJobs) > 0 {
		blockers = append(blockers, fmt.Sprintf("unfinished import_jobs=%v", report.ActiveImportJobs))
	}
	if len(blockers) > 0 {
		report.BlockedReason = strings.Join(blockers, "; ")
	}
	return nil
}

func collectSellerObjectKeys(
	ctx context.Context,
	tx pgx.Tx,
	objectStore SellerObjectStore,
	sellerID int64,
	report *SellerResetReport,
) ([]string, error) {
	keySet := map[string]struct{}{}
	add := func(key string) {
		key = strings.TrimSpace(key)
		if key == "" {
			return
		}
		keySet[key] = struct{}{}
	}

	rows, err := tx.Query(ctx, `
SELECT COALESCE(pack_storage_key, '') FROM ai_pipeline_runs WHERE seller_account_id = $1`, sellerID)
	if err != nil {
		return nil, fmt.Errorf("collect pack_storage_key: %w", err)
	}
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			rows.Close()
			return nil, err
		}
		add(k)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(report.PipelineRunIDs) > 0 {
		artRows, err := tx.Query(ctx, `
SELECT COALESCE(object_key, '') FROM ai_pipeline_artifacts
WHERE pipeline_run_id = ANY($1)`, report.PipelineRunIDs)
		if err != nil {
			return nil, fmt.Errorf("collect artifact object_key: %w", err)
		}
		for artRows.Next() {
			var k string
			if err := artRows.Scan(&k); err != nil {
				artRows.Close()
				return nil, err
			}
			add(k)
		}
		artRows.Close()
		if err := artRows.Err(); err != nil {
			return nil, err
		}
	}

	attRows, err := tx.Query(ctx, `
SELECT COALESCE(storage_key, '') FROM chat_attachments WHERE seller_account_id = $1`, sellerID)
	if err != nil {
		return nil, fmt.Errorf("collect chat attachment keys: %w", err)
	}
	for attRows.Next() {
		var k string
		if err := attRows.Scan(&k); err != nil {
			attRows.Close()
			return nil, err
		}
		add(k)
	}
	attRows.Close()
	if err := attRows.Err(); err != nil {
		return nil, err
	}

	rawRows, err := tx.Query(ctx, `
SELECT COALESCE(storage_object_key, '') FROM raw_payloads WHERE seller_account_id = $1`, sellerID)
	if err != nil {
		return nil, fmt.Errorf("collect raw payload keys: %w", err)
	}
	for rawRows.Next() {
		var k string
		if err := rawRows.Scan(&k); err != nil {
			rawRows.Close()
			return nil, err
		}
		add(k)
	}
	rawRows.Close()
	if err := rawRows.Err(); err != nil {
		return nil, err
	}

	if objectStore != nil {
		prefixes := []struct {
			bucket string
			prefix string
		}{
			{objectStore.ArtifactsBucket(), SellerAnalysisPackPrefix(sellerID)},
			{objectStore.ArtifactsBucket(), SellerChatAttachmentPrefix(sellerID)},
			{objectStore.RawBucket(), SellerRawPayloadPrefix(sellerID)},
		}
		for _, p := range prefixes {
			if strings.TrimSpace(p.bucket) == "" || strings.TrimSpace(p.prefix) == "" {
				continue
			}
			listed, listErr := objectStore.ListObjectKeysByPrefix(ctx, p.bucket, p.prefix)
			if listErr != nil {
				return nil, fmt.Errorf("list objects %s/%s: %w", p.bucket, p.prefix, listErr)
			}
			for _, k := range listed {
				// Exact seller prefix safety: reject accidental seller-10 match.
				if !strings.HasPrefix(k, p.prefix) {
					continue
				}
				add(k)
			}
		}
	}

	out := make([]string, 0, len(keySet))
	for k := range keySet {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}

func deleteSellerObjects(ctx context.Context, objectStore SellerObjectStore, keys []string) ([]string, error) {
	if objectStore == nil || len(keys) == 0 {
		return nil, nil
	}
	deleted := make([]string, 0, len(keys))
	var firstErr error
	for _, key := range keys {
		bucket := bucketForSellerObjectKey(objectStore, key)
		if bucket == "" {
			if firstErr == nil {
				firstErr = fmt.Errorf("cannot resolve bucket for object key %q", key)
			}
			continue
		}
		if err := objectStore.DeleteObject(ctx, bucket, key); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("delete %s/%s: %w", bucket, key, err)
			}
			continue
		}
		deleted = append(deleted, key)
	}
	return deleted, firstErr
}

func bucketForSellerObjectKey(objectStore SellerObjectStore, key string) string {
	switch {
	case strings.HasPrefix(key, "ai-analysis-packs/"), strings.HasPrefix(key, "chat-attachments/"):
		return objectStore.ArtifactsBucket()
	case strings.HasPrefix(key, "raw/"):
		return objectStore.RawBucket()
	default:
		// Prefer artifacts for unknown AI keys; callers may store full keys from artifacts.
		if objectStore.ArtifactsBucket() != "" {
			return objectStore.ArtifactsBucket()
		}
		return objectStore.RawBucket()
	}
}

type sellerDeleteStep struct {
	Table string
	SQL   string
	Via   string
}

func sellerResetDeleteSteps(sellerID int64, publicationIDs, runIDs []int64, preserveSettings bool) []sellerDeleteStep {
	_ = sellerID
	steps := []sellerDeleteStep{
		// Break cyclic / published pointers in one update so check constraints stay satisfied.
		{
			Table: "ai_pipeline_runs.published_pointers",
			SQL: `UPDATE ai_pipeline_runs
SET unified_publication_id = NULL,
    published_at = NULL,
    dashboard_data_snapshot_id = NULL,
    dashboard_run_id = NULL,
    actions_run_id = NULL
WHERE seller_account_id = $1`,
			Via: "null_pointer",
		},
		{Table: "seller_current_unified_publications", SQL: `DELETE FROM seller_current_unified_publications WHERE seller_account_id = $1`},
		{Table: "seller_current_ai_snapshots", SQL: `DELETE FROM seller_current_ai_snapshots WHERE seller_account_id = $1`},
		{Table: "unified_analysis_publication_audit", SQL: `DELETE FROM unified_analysis_publication_audit WHERE seller_account_id = $1`},
	}
	if len(publicationIDs) > 0 {
		steps = append(steps, sellerDeleteStep{
			Table: "unified_publication_action_map",
			SQL:   `DELETE FROM unified_publication_action_map WHERE publication_id = ANY($1)`,
			Via:   "publication_ids",
		})
	}
	steps = append(steps,
		sellerDeleteStep{Table: "unified_analysis_publications", SQL: `DELETE FROM unified_analysis_publications WHERE seller_account_id = $1`},
	)
	if len(runIDs) > 0 {
		steps = append(steps, sellerDeleteStep{
			Table: "ai_pipeline_artifacts",
			SQL:   `DELETE FROM ai_pipeline_artifacts WHERE pipeline_run_id = ANY($1)`,
			Via:   "pipeline_run_ids",
		})
	}
	steps = append(steps,
		sellerDeleteStep{Table: "ai_pipeline_dispatches", SQL: `DELETE FROM ai_pipeline_dispatches WHERE seller_account_id = $1`},
		sellerDeleteStep{Table: "ai_usage_events", SQL: `DELETE FROM ai_usage_events WHERE seller_account_id = $1`},
		// Snapshots/narratives/summaries may FK to ai_pipeline_runs — delete before runs.
		sellerDeleteStep{Table: "ai_dashboard_data_snapshots", SQL: `DELETE FROM ai_dashboard_data_snapshots WHERE seller_account_id = $1`},
		sellerDeleteStep{Table: "ai_dashboard_narratives", SQL: `DELETE FROM ai_dashboard_narratives WHERE seller_account_id = $1`},
		sellerDeleteStep{Table: "ai_daily_summaries", SQL: `DELETE FROM ai_daily_summaries WHERE seller_account_id = $1`},
		sellerDeleteStep{Table: "ai_daily_summary_runs", SQL: `DELETE FROM ai_daily_summary_runs WHERE seller_account_id = $1`},
		sellerDeleteStep{Table: "ai_pipeline_runs", SQL: `DELETE FROM ai_pipeline_runs WHERE seller_account_id = $1`},

		sellerDeleteStep{Table: "action_user_states", SQL: `DELETE FROM action_user_states WHERE seller_account_id = $1`},
		sellerDeleteStep{Table: "action_entity_groups", SQL: `DELETE FROM action_entity_groups WHERE seller_account_id = $1`},
		sellerDeleteStep{Table: "action_generation_category_runs", SQL: `DELETE FROM action_generation_category_runs WHERE seller_account_id = $1`},
		sellerDeleteStep{Table: "action_items", SQL: `DELETE FROM action_items WHERE seller_account_id = $1`},
		sellerDeleteStep{Table: "action_generation_runs", SQL: `DELETE FROM action_generation_runs WHERE seller_account_id = $1`},

		sellerDeleteStep{Table: "chat_attachments", SQL: `DELETE FROM chat_attachments WHERE seller_account_id = $1`},
		sellerDeleteStep{Table: "chat_feedback", SQL: `DELETE FROM chat_feedback WHERE seller_account_id = $1`},
		sellerDeleteStep{Table: "chat_traces", SQL: `DELETE FROM chat_traces WHERE seller_account_id = $1`},
		sellerDeleteStep{Table: "chat_messages", SQL: `DELETE FROM chat_messages WHERE seller_account_id = $1`},
		sellerDeleteStep{Table: "chat_sessions", SQL: `DELETE FROM chat_sessions WHERE seller_account_id = $1`},

		sellerDeleteStep{Table: "recommendation_run_diagnostics", SQL: `DELETE FROM recommendation_run_diagnostics WHERE seller_account_id = $1`},
		sellerDeleteStep{Table: "recommendation_feedback", SQL: `DELETE FROM recommendation_feedback WHERE seller_account_id = $1`},
		sellerDeleteStep{Table: "recommendation_alert_links", SQL: `DELETE FROM recommendation_alert_links WHERE seller_account_id = $1`},
		sellerDeleteStep{Table: "recommendation_runs", SQL: `DELETE FROM recommendation_runs WHERE seller_account_id = $1`},
		sellerDeleteStep{Table: "recommendations", SQL: `DELETE FROM recommendations WHERE seller_account_id = $1`},
		sellerDeleteStep{Table: "alert_runs", SQL: `DELETE FROM alert_runs WHERE seller_account_id = $1`},
		sellerDeleteStep{Table: "alerts", SQL: `DELETE FROM alerts WHERE seller_account_id = $1`},

		sellerDeleteStep{Table: "ozon_api_capabilities", SQL: `DELETE FROM ozon_api_capabilities WHERE seller_account_id = $1`},
		sellerDeleteStep{Table: "ozon_enrichment_sync_runs", SQL: `DELETE FROM ozon_enrichment_sync_runs WHERE seller_account_id = $1`},
		sellerDeleteStep{Table: "ozon_product_info_snapshots", SQL: `DELETE FROM ozon_product_info_snapshots WHERE seller_account_id = $1`},
		sellerDeleteStep{Table: "ozon_related_sku_errors", SQL: `DELETE FROM ozon_related_sku_errors WHERE seller_account_id = $1`},
		sellerDeleteStep{Table: "ozon_related_skus", SQL: `DELETE FROM ozon_related_skus WHERE seller_account_id = $1`},
		sellerDeleteStep{Table: "stock_analytics_snapshots", SQL: `DELETE FROM stock_analytics_snapshots WHERE seller_account_id = $1`},
		sellerDeleteStep{Table: "ad_campaign_objects", SQL: `DELETE FROM ad_campaign_objects WHERE seller_account_id = $1`},

		sellerDeleteStep{Table: "daily_sku_metrics", SQL: `DELETE FROM daily_sku_metrics WHERE seller_account_id = $1`},
		sellerDeleteStep{Table: "daily_account_metrics", SQL: `DELETE FROM daily_account_metrics WHERE seller_account_id = $1`},
		sellerDeleteStep{Table: "product_analytics_daily", SQL: `DELETE FROM product_analytics_daily WHERE seller_account_id = $1`},
		sellerDeleteStep{Table: "ad_metrics_daily", SQL: `DELETE FROM ad_metrics_daily WHERE seller_account_id = $1`},
		sellerDeleteStep{Table: "ad_campaign_skus", SQL: `DELETE FROM ad_campaign_skus WHERE seller_account_id = $1`},
		sellerDeleteStep{Table: "ad_campaigns", SQL: `DELETE FROM ad_campaigns WHERE seller_account_id = $1`},
		sellerDeleteStep{Table: "sku_effective_constraints", SQL: `DELETE FROM sku_effective_constraints WHERE seller_account_id = $1`},
		sellerDeleteStep{Table: "pricing_constraint_rules", SQL: `DELETE FROM pricing_constraint_rules WHERE seller_account_id = $1`},
		sellerDeleteStep{Table: "sales", SQL: `DELETE FROM sales WHERE seller_account_id = $1`},
		sellerDeleteStep{Table: "orders", SQL: `DELETE FROM orders WHERE seller_account_id = $1`},
		sellerDeleteStep{Table: "stocks", SQL: `DELETE FROM stocks WHERE seller_account_id = $1`},
		sellerDeleteStep{Table: "products", SQL: `DELETE FROM products WHERE seller_account_id = $1`},
		sellerDeleteStep{Table: "import_jobs", SQL: `DELETE FROM import_jobs WHERE seller_account_id = $1`},
		sellerDeleteStep{Table: "sync_jobs", SQL: `DELETE FROM sync_jobs WHERE seller_account_id = $1`},
		sellerDeleteStep{Table: "sync_cursors", SQL: `DELETE FROM sync_cursors WHERE seller_account_id = $1`},
		sellerDeleteStep{Table: "raw_payloads", SQL: `DELETE FROM raw_payloads WHERE seller_account_id = $1`},
		sellerDeleteStep{Table: "ozon_connections", SQL: `DELETE FROM ozon_connections WHERE seller_account_id = $1`},
		sellerDeleteStep{Table: "admin_action_logs", SQL: `DELETE FROM admin_action_logs WHERE seller_account_id = $1`},
		sellerDeleteStep{Table: "seller_billing_state", SQL: `DELETE FROM seller_billing_state WHERE seller_account_id = $1`},
	)
	_ = preserveSettings
	return steps
}

func planSellerTableDeletes(ctx context.Context, tx pgx.Tx, opts SellerResetOptions, report *SellerResetReport) error {
	steps := sellerResetDeleteSteps(opts.SellerAccountID, report.PublicationIDs, report.PipelineRunIDs, true)
	for _, step := range steps {
		count, err := countSellerDeleteStep(ctx, tx, step, opts.SellerAccountID, report.PublicationIDs, report.PipelineRunIDs)
		if err != nil {
			return fmt.Errorf("dry-run count %s: %w", step.Table, err)
		}
		report.Tables = append(report.Tables, SellerResetTableResult{
			Table:       step.Table,
			WouldDelete: count,
			Via:         step.Via,
		})
	}
	return nil
}

func countSellerDeleteStep(ctx context.Context, tx pgx.Tx, step sellerDeleteStep, sellerID int64, publicationIDs, runIDs []int64) (int64, error) {
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(step.SQL)), "UPDATE") {
		var n int64
		err := tx.QueryRow(ctx, `
SELECT COUNT(*) FROM ai_pipeline_runs
WHERE seller_account_id = $1
  AND (
    unified_publication_id IS NOT NULL
    OR published_at IS NOT NULL
    OR dashboard_data_snapshot_id IS NOT NULL
    OR dashboard_run_id IS NOT NULL
    OR actions_run_id IS NOT NULL
  )`, sellerID).Scan(&n)
		return n, err
	}
	switch step.Via {
	case "publication_ids":
		if len(publicationIDs) == 0 {
			return 0, nil
		}
		var n int64
		err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM unified_publication_action_map WHERE publication_id = ANY($1)`, publicationIDs).Scan(&n)
		return n, err
	case "pipeline_run_ids":
		if len(runIDs) == 0 {
			return 0, nil
		}
		var n int64
		err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM ai_pipeline_artifacts WHERE pipeline_run_id = ANY($1)`, runIDs).Scan(&n)
		return n, err
	default:
		// Derive count SQL from DELETE ... WHERE seller_account_id = $1
		table := step.Table
		if strings.Contains(table, ".") {
			return 0, nil
		}
		var n int64
		q := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE seller_account_id = $1`, table)
		err := tx.QueryRow(ctx, q, sellerID).Scan(&n)
		return n, err
	}
}

func executeSellerTableDeletes(ctx context.Context, tx pgx.Tx, opts SellerResetOptions, report *SellerResetReport) error {
	steps := sellerResetDeleteSteps(opts.SellerAccountID, report.PublicationIDs, report.PipelineRunIDs, true)
	for _, step := range steps {
		var tag pgconn.CommandTag
		var err error
		switch step.Via {
		case "publication_ids":
			if len(report.PublicationIDs) == 0 {
				report.Tables = append(report.Tables, SellerResetTableResult{Table: step.Table, Deleted: 0, Via: step.Via})
				continue
			}
			tag, err = tx.Exec(ctx, step.SQL, report.PublicationIDs)
		case "pipeline_run_ids":
			if len(report.PipelineRunIDs) == 0 {
				report.Tables = append(report.Tables, SellerResetTableResult{Table: step.Table, Deleted: 0, Via: step.Via})
				continue
			}
			tag, err = tx.Exec(ctx, step.SQL, report.PipelineRunIDs)
		default:
			tag, err = tx.Exec(ctx, step.SQL, opts.SellerAccountID)
		}
		if err != nil {
			return fmt.Errorf("delete %s: %w", step.Table, err)
		}
		report.Tables = append(report.Tables, SellerResetTableResult{
			Table:   step.Table,
			Deleted: tag.RowsAffected(),
			Via:     step.Via,
		})
	}
	return nil
}

// cleanupMVPSellerData deletes seller-scoped test data inside an existing transaction
// using the unified delete step list from ResetSellerTestData.
func cleanupMVPSellerData(ctx context.Context, exec dbgen.DBTX, sellerAccountID int64) error {
	if sellerAccountID <= 0 {
		return fmt.Errorf("cleanup: seller_account_id must be > 0")
	}

	var publicationIDs, runIDs []int64
	rows, err := exec.Query(ctx, `SELECT id FROM ai_pipeline_runs WHERE seller_account_id = $1`, sellerAccountID)
	if err != nil {
		return fmt.Errorf("cleanup list runs: %w", err)
	}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		runIDs = append(runIDs, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	pubRows, err := exec.Query(ctx, `SELECT id FROM unified_analysis_publications WHERE seller_account_id = $1`, sellerAccountID)
	if err != nil {
		return fmt.Errorf("cleanup list publications: %w", err)
	}
	for pubRows.Next() {
		var id int64
		if err := pubRows.Scan(&id); err != nil {
			pubRows.Close()
			return err
		}
		publicationIDs = append(publicationIDs, id)
	}
	pubRows.Close()
	if err := pubRows.Err(); err != nil {
		return err
	}

	steps := sellerResetDeleteSteps(sellerAccountID, publicationIDs, runIDs, true)
	for _, step := range steps {
		var err error
		switch step.Via {
		case "publication_ids":
			if len(publicationIDs) == 0 {
				continue
			}
			_, err = exec.Exec(ctx, step.SQL, publicationIDs)
		case "pipeline_run_ids":
			if len(runIDs) == 0 {
				continue
			}
			_, err = exec.Exec(ctx, step.SQL, runIDs)
		default:
			_, err = exec.Exec(ctx, step.SQL, sellerAccountID)
		}
		if err != nil {
			return fmt.Errorf("cleanup %s: %w", step.Table, err)
		}
	}
	return nil
}

// MarshalSellerResetReport returns indented JSON for CLI output files.
func MarshalSellerResetReport(report SellerResetReport) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}
