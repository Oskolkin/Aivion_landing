package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
)

func runAuditDashboardAIFirst(args []string) int {
	fs := flag.NewFlagSet("audit-dashboard-ai-first", flag.ContinueOnError)
	var input, output string
	fs.StringVar(&input, "input", "", "path to /dashboard/home JSON export (required)")
	fs.StringVar(&output, "output", "dashboard-ai-first-audit.json", "output report path")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if input == "" {
		fmt.Fprintln(os.Stderr, "--input is required")
		return 2
	}
	resp, err := dashboardhome.LoadDashboardHomeResponseFromFile(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load dashboard json: %v\n", err)
		return 1
	}
	report := dashboardhome.RunDashboardAIFirstAudit(resp)
	if err := dashboardhome.WriteDashboardAIFirstAuditReport(output, report); err != nil {
		fmt.Fprintf(os.Stderr, "write report: %v\n", err)
		return 1
	}
	fmt.Printf("ai-first audit: status=%s violations=%d -> %s\n",
		report.OverallStatus, len(report.Violations), output)
	if report.OverallStatus != "passed" {
		return 1
	}
	return 0
}
