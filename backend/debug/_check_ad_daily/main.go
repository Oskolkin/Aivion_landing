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
    SELECT metric_date::text,
           SUM(spend::float8) AS spend,
           SUM(revenue::float8) AS revenue
    FROM ad_metrics_daily
    WHERE seller_account_id = 1
      AND metric_date BETWEEN '2026-07-02' AND '2026-07-08'
    GROUP BY metric_date
    ORDER BY metric_date`)
	if err != nil {
		panic(err)
	}
	defer rows.Close()
	var s7, r7 float64
	for rows.Next() {
		var d string
		var s, r float64
		if err := rows.Scan(&d, &s, &r); err != nil {
			panic(err)
		}
		drr := 0.0
		if r > 0 {
			drr = s / r * 100
		}
		fmt.Printf("%s spend=%.0f rev=%.0f drr=%.2f%%\n", d, s, r, drr)
		s7 += s
		r7 += r
	}
	fmt.Printf("7d spend=%.0f rev=%.0f drr=%.2f%%\n", s7, r7, s7/r7*100)
}
