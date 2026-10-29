package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
)

func main() {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		panic(err)
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx, `
		SELECT id, status, COALESCE(error_code,''), COALESCE(error_message,''),
		       COALESCE(prompt_version,''), created_at::text
		FROM ai_pipeline_runs
		WHERE seller_account_id = 1
		ORDER BY id DESC
		LIMIT 8`)
	if err != nil {
		// discover table
		fmt.Println("err", err)
		cols, _ := conn.Query(ctx, `
			SELECT tablename FROM pg_tables WHERE schemaname='public' AND tablename LIKE '%pipeline%' OR tablename LIKE '%run%'
			ORDER BY 1`)
		for cols.Next() {
			var t string
			_ = cols.Scan(&t)
			fmt.Println(t)
		}
		return
	}
	for rows.Next() {
		var id int64
		var status, code, msg, prompt, created string
		_ = rows.Scan(&id, &status, &code, &msg, &prompt, &created)
		fmt.Printf("run=%d status=%s code=%s prompt=%s created=%s msg=%s\n", id, status, code, prompt, created, trunc(msg))
	}
	rows.Close()

	var pubID, runID int64
	var metricDate string
	_ = conn.QueryRow(ctx, `
		SELECT c.publication_id, p.run_id, c.metric_date::text
		FROM seller_current_unified_publications c
		JOIN unified_analysis_publications p ON p.id = c.publication_id
		WHERE c.seller_account_id = 1`).Scan(&pubID, &runID, &metricDate)
	fmt.Printf("CURRENT pointer pub=%d run=%d date=%s\n", pubID, runID, metricDate)
}

func trunc(s string) string {
	if len(s) > 80 {
		return s[:80] + "..."
	}
	return s
}
