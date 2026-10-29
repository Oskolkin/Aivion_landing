package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/jobs"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
)

// One-shot helper: insert completed sync_job + enqueue ozon.post_sync_recalculation.
// Usage: go run ./debug/enqueue_post_sync_once.go <seller_id>
func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./debug/enqueue_post_sync_once.go <seller_id>")
		os.Exit(2)
	}
	sellerID, err := strconv.ParseInt(os.Args[1], 10, 64)
	if err != nil || sellerID <= 0 {
		fmt.Fprintln(os.Stderr, "invalid seller_id")
		os.Exit(2)
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL required")
		os.Exit(1)
	}
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "127.0.0.1:6379"
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "db: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	var syncJobID int64
	err = pool.QueryRow(ctx, `
INSERT INTO sync_jobs (seller_account_id, type, status, started_at, finished_at, created_at, updated_at)
VALUES ($1, 'incremental_sync', 'completed', now(), now(), now(), now())
RETURNING id`, sellerID).Scan(&syncJobID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "insert sync_job: %v\n", err)
		os.Exit(1)
	}

	task, err := jobs.NewRecalculateAfterSyncTask(sellerID, syncJobID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "build task: %v\n", err)
		os.Exit(1)
	}
	client := asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr})
	defer client.Close()
	info, err := client.Enqueue(task,
		asynq.TaskID(fmt.Sprintf("post-sync:%d:recalc", syncJobID)),
		asynq.Unique(48*time.Hour),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "enqueue: %v\n", err)
		os.Exit(1)
	}
	out := map[string]any{
		"seller_account_id": sellerID,
		"sync_job_id":       syncJobID,
		"asynq_task_id":     info.ID,
		"asynq_queue":       info.Queue,
		"asynq_type":        task.Type(),
		"enqueued_at":       time.Now().UTC().Format(time.RFC3339),
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}
