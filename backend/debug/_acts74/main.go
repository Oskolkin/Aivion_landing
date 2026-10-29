package main

import (
	"context"
	"fmt"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
	"os"
	"strings"
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
	rows, err := pg.Pool.Query(ctx, `SELECT column_name FROM information_schema.columns WHERE table_name='action_items' ORDER BY ordinal_position`)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var c string
		_ = rows.Scan(&c)
		cols = append(cols, c)
		fmt.Println("col", c)
	}
	var n int
	_ = pg.Pool.QueryRow(ctx, `SELECT count(*) FROM action_items WHERE seller_account_id=1`).Scan(&n)
	fmt.Println("seller1_actions", n)
	for _, c := range cols {
		if strings.Contains(c, "run") || strings.Contains(c, "pub") || strings.Contains(c, "evidence") || strings.Contains(c, "source") {
			q := fmt.Sprintf(`SELECT %s::text, count(*) FROM action_items WHERE seller_account_id=1 GROUP BY 1 ORDER BY 2 DESC NULLS LAST LIMIT 8`, c)
			r, err := pg.Pool.Query(ctx, q)
			if err != nil {
				fmt.Println("fail", c, err)
				continue
			}
			fmt.Println("by", c)
			for r.Next() {
				var v *string
				var cnt int64
				_ = r.Scan(&v, &cnt)
				if v == nil {
					fmt.Printf("  null -> %d\n", cnt)
				} else {
					fmt.Printf("  %s -> %d\n", *v, cnt)
				}
			}
			r.Close()
		}
	}
}
