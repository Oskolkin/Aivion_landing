package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
)

func runDumpAIDataPack(args []string) int {
	fs := flag.NewFlagSet("dump-ai-data-pack", flag.ContinueOnError)
	var (
		sellerID      int64
		metricDateRaw string
		pretty        bool
		validate      bool
		outputPath    string
		audit         bool
	)
	fs.Int64Var(&sellerID, "seller-id", 0, "seller_account_id (required)")
	fs.StringVar(&metricDateRaw, "metric-date", "", "metric date YYYY-MM-DD (required)")
	fs.BoolVar(&pretty, "pretty", true, "pretty-print JSON")
	fs.BoolVar(&validate, "validate", true, "validate pack before output")
	fs.StringVar(&outputPath, "output", "", "write UTF-8 JSON to file (recommended on Windows PowerShell)")
	fs.BoolVar(&audit, "audit", false, "print audit summary JSON to stderr after dump")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if sellerID <= 0 {
		fmt.Fprintln(os.Stderr, "seller-id is required and must be > 0")
		return 2
	}
	if strings.TrimSpace(metricDateRaw) == "" {
		fmt.Fprintln(os.Stderr, "metric-date is required (YYYY-MM-DD)")
		return 2
	}
	metricDate, err := parseMetricDateCLI(metricDateRaw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "metric-date: %v\n", err)
		return 2
	}

	_ = config.LoadEnvFiles()
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		return 1
	}
	ctx := context.Background()
	postgres, err := db.New(ctx, cfg.DB.URL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect postgres: %v\n", err)
		return 1
	}
	defer postgres.Close()

	svc := aidatapack.NewService(postgres.Pool)
	pack, err := svc.Build(ctx, sellerID, metricDate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "build data pack: %v\n", err)
		return 1
	}
	if validate {
		if err := aidatapack.ValidateDataPack(pack); err != nil {
			fmt.Fprintf(os.Stderr, "validate data pack: %v\n", err)
			return 1
		}
	}

	var raw []byte
	if pretty {
		raw, err = json.MarshalIndent(pack, "", "  ")
	} else {
		raw, err = json.Marshal(pack)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal json: %v\n", err)
		return 1
	}

	if strings.TrimSpace(outputPath) != "" {
		if err := os.WriteFile(outputPath, raw, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "write output: %v\n", err)
			return 1
		}
	} else {
		fmt.Println(string(raw))
	}

	if audit {
		report := aidatapack.AuditDataPack(pack)
		auditRaw, _ := json.MarshalIndent(report, "", "  ")
		fmt.Fprintf(os.Stderr, "%s\n", string(auditRaw))
	}
	return 0
}
