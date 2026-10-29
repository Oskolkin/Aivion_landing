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
	var st47, pack47 string
	var run47 int64
	_ = conn.QueryRow(ctx, `SELECT status, run_id, pack_hash FROM unified_analysis_publications WHERE id=47`).Scan(&st47, &run47, &pack47)
	fmt.Printf("old pub47 status=%s run=%d pack=%s\n", st47, run47, trunc(pack47))
	var st53, pack53, out53, prompt, schema, bundle string
	var run53 int64
	_ = conn.QueryRow(ctx, `SELECT status, run_id, pack_hash, output_hash, prompt_version, schema_version, bundle_hash FROM unified_analysis_publications WHERE id=53`).Scan(&st53, &run53, &pack53, &out53, &prompt, &schema, &bundle)
	fmt.Printf("new pub53 status=%s run=%d prompt=%s schema=%s\n", st53, run53, prompt, schema)
	fmt.Printf("pack=%s out=%s bundle=%s\n", trunc(pack53), trunc(out53), trunc(bundle))
}

func trunc(s string) string {
	if len(s) > 20 {
		return s[:20] + "..."
	}
	return s
}
