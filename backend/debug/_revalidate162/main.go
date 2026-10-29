package main

import (
	"context"
	"fmt"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
)

func main() {
	_ = config.LoadEnvFiles()
	cfg, _ := config.Load()
	ctx := context.Background()
	pg, _ := db.New(ctx, cfg.DB.URL)
	defer pg.Close()
	tag, _ := pg.Pool.Exec(ctx, `
UPDATE ai_pipeline_runs SET status='provider_completed', error_code=NULL, error_message=NULL, completed_at=NULL, updated_at=NOW()
WHERE id=162`)
	fmt.Printf("reset=%d\n", tag.RowsAffected())
	tag, _ = pg.Pool.Exec(ctx, `
DELETE FROM ai_pipeline_artifacts WHERE pipeline_run_id=162
  AND kind IN ('unified_output_validated','unified_output_validation','unified_output_summary','output_validated','output_validation','output_summary')`)
	fmt.Printf("deleted=%d\n", tag.RowsAffected())
}
