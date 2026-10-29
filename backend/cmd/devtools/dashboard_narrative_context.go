package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analytics"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/db"
)

func runDumpDashboardNarrativeContext(args []string) int {
	fs := flag.NewFlagSet("dump-dashboard-narrative-context", flag.ContinueOnError)
	var (
		sellerID      int64
		metricDateRaw string
		source        string
		outputPath    string
		pretty        bool
	)
	fs.Int64Var(&sellerID, "seller-id", 0, "seller_account_id (required)")
	fs.StringVar(&metricDateRaw, "metric-date", "", "metric date YYYY-MM-DD (required)")
	fs.StringVar(&source, "source", "data_pack", "data_pack (legacy not supported in dump)")
	fs.StringVar(&outputPath, "output", "", "write UTF-8 JSON to file")
	fs.BoolVar(&pretty, "pretty", true, "pretty-print JSON")
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
	if strings.TrimSpace(strings.ToLower(source)) != "data_pack" {
		fmt.Fprintln(os.Stderr, "only --source=data_pack is supported")
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

	packSvc := aidatapack.NewService(postgres.Pool)
	pack, err := packSvc.Build(ctx, sellerID, metricDate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "build data pack: %v\n", err)
		return 1
	}

	var legacy dashboardhome.DataPackNarrativeLegacyInput
	dashSvc := analytics.NewDashboardService(postgres.Pool)
	if dto, dtoErr := dashSvc.BuildDashboardMetrics(ctx, sellerID, &metricDate); dtoErr == nil {
		legacy.DTO = dto
	}

	out, err := dashboardhome.DumpDashboardNarrativeContextForDebug(pack, nil, legacy)
	if err != nil {
		fmt.Fprintf(os.Stderr, "build narrative context: %v\n", err)
		return 1
	}

	var raw []byte
	if pretty {
		raw, err = json.MarshalIndent(out, "", "  ")
	} else {
		raw, err = json.Marshal(out)
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
	return 0
}
