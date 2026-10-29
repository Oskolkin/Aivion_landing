package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
)

func main() {
	_ = config.LoadEnvFiles()
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ctx := context.Background()
	pg, err := db.New(ctx, cfg.DB.URL)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer pg.Close()

	cmds := []struct {
		name string
		sql  string
	}{
		{
			name: "pipeline_run_47",
			sql: `UPDATE ai_pipeline_runs
SET status='failed', error_code='stale_pre_reset', error_message='terminated before seller-1 full reseed',
    completed_at=NOW(), updated_at=NOW()
WHERE seller_account_id=1 AND id=47
  AND status NOT IN ('failed','cancelled','published','publication_failed','cleanup_completed','completed','completed_partial','publication_blocked','empty')`,
		},
		{
			name: "sync_job_14",
			sql: `UPDATE sync_jobs
SET status='failed', error_message='terminated before seller-1 full reseed', finished_at=NOW(), updated_at=NOW()
WHERE seller_account_id=1 AND id=14 AND status IN ('pending','running')`,
		},
		{
			name: "import_jobs",
			sql: `UPDATE import_jobs
SET status='failed', error_message='terminated before seller-1 full reseed', finished_at=NOW(), updated_at=NOW()
WHERE seller_account_id=1 AND id = ANY(ARRAY[65,67,70,74]::bigint[])
  AND status IN ('pending','fetching','importing','retrying')`,
		},
	}
	for _, c := range cmds {
		tag, err := pg.Pool.Exec(ctx, c.sql)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", c.name, err)
			os.Exit(1)
		}
		fmt.Printf("%s rows=%d\n", c.name, tag.RowsAffected())
	}
}
