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
	tag, err := pg.Pool.Exec(ctx, `UPDATE ai_pipeline_runs SET status='publication_ready', error_code=NULL, error_message=NULL WHERE id=586`)
	if err != nil {
		panic(err)
	}
	fmt.Println("updated", tag.RowsAffected())
}
