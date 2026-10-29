package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/chat"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
)

func runRepairChatTitles(args []string) int {
	fs := flag.NewFlagSet("repair-chat-titles", flag.ExitOnError)
	dryRun := fs.Bool("dry-run", false, "report only")
	apply := fs.Bool("apply", false, "apply repairs")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *dryRun == *apply {
		fmt.Fprintln(os.Stderr, "specify exactly one of --dry-run or --apply")
		return 2
	}

	_ = config.LoadEnvFiles()
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	ctx := context.Background()
	postgres, err := db.New(ctx, cfg.DB.URL)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer postgres.Close()

	result, err := chat.RepairChatTitles(ctx, postgres.Pool, *dryRun)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(result)
	return 0
}
