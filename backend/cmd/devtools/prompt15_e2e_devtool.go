package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

func runSwitchAnalysisCurrentPublication(args []string) int {
	fs := flag.NewFlagSet("switch-analysis-current-publication", flag.ContinueOnError)
	var sellerID, publicationID int64
	fs.Int64Var(&sellerID, "seller-id", 0, "seller_account_id (required)")
	fs.Int64Var(&publicationID, "publication-id", 0, "analysis_publications.id (required)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if sellerID <= 0 || publicationID <= 0 {
		fmt.Fprintln(os.Stderr, "seller-id and publication-id are required")
		return 2
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is required")
		return 2
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect: %v\n", err)
		return 1
	}
	defer pool.Close()

	var pubSeller int64
	if err := pool.QueryRow(ctx, `SELECT seller_account_id FROM analysis_publications WHERE id=$1`, publicationID).Scan(&pubSeller); err != nil {
		fmt.Fprintf(os.Stderr, "publication lookup: %v\n", err)
		return 1
	}
	if pubSeller != sellerID {
		fmt.Fprintf(os.Stderr, "publication %d belongs to seller %d, not %d\n", publicationID, pubSeller, sellerID)
		return 1
	}
	if _, err := pool.Exec(ctx, `INSERT INTO seller_current_publications(seller_account_id,publication_id,updated_at)
		VALUES($1,$2,NOW()) ON CONFLICT (seller_account_id) DO UPDATE SET publication_id=$2, updated_at=NOW()`, sellerID, publicationID); err != nil {
		fmt.Fprintf(os.Stderr, "switch current publication: %v\n", err)
		return 1
	}
	out := map[string]any{"seller_id": sellerID, "current_publication_id": publicationID}
	b, _ := json.Marshal(out)
	fmt.Println(string(b))
	return 0
}
