package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db/migrationpreflight"
)

func runMigrationPreflight(args []string) int {
	fs := flag.NewFlagSet("migration-preflight", flag.ExitOnError)
	databaseURL := fs.String("database-url", "", "PostgreSQL database URL (defaults to DATABASE_URL)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	_ = config.LoadEnvFiles()
	dsn := *databaseURL
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL or --database-url is required")
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	postgres, err := db.New(ctx, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect postgres: %v\n", err)
		return 1
	}
	defer postgres.Close()
	report, err := migrationpreflight.Run(ctx, postgres.Pool)
	if err != nil {
		fmt.Fprintf(os.Stderr, "preflight failed: %v\n", err)
		return 1
	}
	fmt.Print(report.String())
	if report.Blocked() {
		return 1
	}
	return 0
}
