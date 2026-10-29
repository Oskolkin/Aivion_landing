package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/devseed"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	_ = config.LoadEnvFiles()
	cfg, err := config.Load()
	if err != nil {
		fatal(err)
	}
	ctx := context.Background()
	postgres, err := db.New(ctx, cfg.DB.URL)
	if err != nil {
		fatal(err)
	}
	defer postgres.Close()
	pool := postgres.Pool

	outDir := "debug/seller-1-full-reseed/before"
	if len(os.Args) > 1 {
		outDir = os.Args[1]
	}
	_ = os.MkdirAll(outDir, 0o755)

	report := map[string]any{}
	report["captured_at"] = time.Now().UTC().Format(time.RFC3339)
	report["db_url_host"] = redactDB(cfg.DB.URL)

	var dbName string
	_ = pool.QueryRow(ctx, `SELECT current_database()`).Scan(&dbName)
	report["db_name"] = dbName

	var sellerCount int64
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM seller_accounts`).Scan(&sellerCount)
	report["seller_count"] = sellerCount

	var identity map[string]any
	row := pool.QueryRow(ctx, `
SELECT id, name, status::text, user_id, created_at
FROM seller_accounts WHERE id = 1`)
	var id, userID int64
	var name, status string
	var created time.Time
	if err := row.Scan(&id, &name, &status, &userID, &created); err == nil {
		identity = map[string]any{
			"id": id, "name": name, "status": status, "user_id": userID,
			"created_at": created.UTC().Format(time.RFC3339),
		}
	}
	report["seller_1_identity"] = identity

	var settings []byte
	_ = pool.QueryRow(ctx, `
SELECT COALESCE(to_jsonb(s), '{}'::jsonb)
FROM seller_analysis_settings s WHERE seller_account_id = 1`).Scan(&settings)
	if len(settings) > 0 {
		report["seller_1_analysis_settings"] = json.RawMessage(settings)
	}

	tables := []string{
		"seller_current_unified_publications", "seller_current_ai_snapshots",
		"unified_analysis_publication_audit", "unified_analysis_publications",
		"ai_pipeline_dispatches", "ai_pipeline_runs", "ai_usage_events",
		"ai_dashboard_data_snapshots", "ai_dashboard_narratives",
		"ai_daily_summaries", "ai_daily_summary_runs",
		"action_user_states", "action_entity_groups", "action_generation_category_runs",
		"action_items", "action_generation_runs",
		"chat_attachments", "chat_feedback", "chat_traces", "chat_messages", "chat_sessions",
		"recommendation_run_diagnostics", "recommendation_feedback", "recommendation_alert_links",
		"recommendation_runs", "recommendations", "alert_runs", "alerts",
		"ozon_api_capabilities", "ozon_enrichment_sync_runs", "ozon_product_info_snapshots",
		"ozon_related_sku_errors", "ozon_related_skus", "stock_analytics_snapshots", "ad_campaign_objects",
		"daily_sku_metrics", "daily_account_metrics", "product_analytics_daily",
		"ad_metrics_daily", "ad_campaign_skus", "ad_campaigns",
		"sku_effective_constraints", "pricing_constraint_rules",
		"sales", "orders", "stocks", "products",
		"import_jobs", "sync_jobs", "sync_cursors", "raw_payloads", "ozon_connections",
		"admin_action_logs", "seller_billing_state",
	}
	counts1 := map[string]int64{}
	countsOther := map[string]int64{}
	for _, table := range tables {
		var n1, nOther int64
		q1 := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE seller_account_id = 1`, table)
		qOther := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE seller_account_id <> 1`, table)
		if err := pool.QueryRow(ctx, q1).Scan(&n1); err != nil {
			counts1[table] = -1
		} else {
			counts1[table] = n1
		}
		if err := pool.QueryRow(ctx, qOther).Scan(&nOther); err != nil {
			countsOther[table] = -1
		} else {
			countsOther[table] = nOther
		}
	}
	report["seller_1_row_counts"] = counts1
	report["other_sellers_row_counts"] = countsOther

	var pubIDs, runIDs, actionIDs []int64
	_ = scanIDs(ctx, pool, `SELECT id FROM unified_analysis_publications WHERE seller_account_id=1 ORDER BY id`, &pubIDs)
	_ = scanIDs(ctx, pool, `SELECT id FROM ai_pipeline_runs WHERE seller_account_id=1 ORDER BY id`, &runIDs)
	_ = scanIDs(ctx, pool, `SELECT id FROM action_items WHERE seller_account_id=1 ORDER BY id`, &actionIDs)
	report["publication_ids"] = pubIDs
	report["pipeline_run_ids"] = runIDs
	report["action_item_ids"] = actionIDs

	var curPub, curRun *int64
	_ = pool.QueryRow(ctx, `SELECT publication_id FROM seller_current_unified_publications WHERE seller_account_id=1`).Scan(&curPub)
	_ = pool.QueryRow(ctx, `SELECT pipeline_run_id FROM seller_current_ai_snapshots WHERE seller_account_id=1`).Scan(&curRun)
	report["current_unified_publication_id"] = curPub
	report["current_ai_pipeline_run_id"] = curRun

	var activeRuns, activeDisp, activeSync, activeImport []int64
	_ = scanIDs(ctx, pool, `
SELECT id FROM ai_pipeline_runs WHERE seller_account_id=1
  AND status NOT IN ('failed','cancelled','publication_blocked','published','publication_failed','cleanup_completed','completed','completed_partial','context_failed','dashboard_failed','actions_failed','empty')`, &activeRuns)
	_ = scanIDs(ctx, pool, `SELECT id FROM ai_pipeline_dispatches WHERE seller_account_id=1 AND status NOT IN ('completed','failed')`, &activeDisp)
	_ = scanIDs(ctx, pool, `SELECT id FROM sync_jobs WHERE seller_account_id=1 AND status IN ('pending','running')`, &activeSync)
	_ = scanIDs(ctx, pool, `SELECT id FROM import_jobs WHERE seller_account_id=1 AND status IN ('pending','fetching','importing','retrying')`, &activeImport)
	report["non_terminal_pipeline_runs"] = activeRuns
	report["active_dispatches"] = activeDisp
	report["active_sync_jobs"] = activeSync
	report["active_import_jobs"] = activeImport

	objectKeys := []string{}
	s3Client, s3Err := storage.New(ctx, storage.S3Config{
		Endpoint: cfg.S3.Endpoint, AccessKey: cfg.S3.AccessKey, SecretKey: cfg.S3.SecretKey,
		UseSSL: cfg.S3.UseSSL, BucketRaw: cfg.S3.BucketRaw, BucketExports: cfg.S3.BucketExports, BucketArtifacts: cfg.S3.BucketArtifacts,
	})
	if s3Err == nil {
		prefix := devseed.SellerAnalysisPackPrefix(1)
		keys, listErr := storage.ListObjectKeysByPrefix(ctx, s3Client, cfg.S3.BucketArtifacts, prefix)
		if listErr == nil {
			objectKeys = keys
		} else {
			report["minio_list_error"] = listErr.Error()
		}
	} else {
		report["minio_error"] = s3Err.Error()
	}
	sort.Strings(objectKeys)
	report["seller_1_pack_object_keys"] = objectKeys

	raw, _ := json.MarshalIndent(report, "", "  ")
	path := filepath.Join(outDir, "snapshot.json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		fatal(err)
	}
	fmt.Println(path)
}

func scanIDs(ctx context.Context, pool *pgxpool.Pool, q string, into *[]int64) error {
	rows, err := pool.Query(ctx, q)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return err
		}
		*into = append(*into, id)
	}
	return rows.Err()
}

func redactDB(url string) string {
	if i := strings.Index(url, "@"); i >= 0 {
		return "postgres://***@" + url[i+1:]
	}
	return url
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
