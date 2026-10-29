package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
)

func runValidateDashboardUIContract(args []string) int {
	fs := flag.NewFlagSet("validate-dashboard-ui-contract", flag.ContinueOnError)
	var input, output string
	fs.StringVar(&input, "input", "", "path to /dashboard/home JSON export (required)")
	fs.StringVar(&output, "output", "dashboard-ui-contract-validation.json", "output report path")
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
	report := dashboardhome.RunDashboardUIContractValidation(resp)
	if err := dashboardhome.WriteDashboardUIContractValidationReport(output, report); err != nil {
		fmt.Fprintf(os.Stderr, "write report: %v\n", err)
		return 1
	}
	fmt.Printf("ui contract validation: status=%s critical=%d warning=%d -> %s\n",
		report.Status, report.Summary.Critical, report.Summary.Warning, output)
	if report.Status != "passed" {
		return 1
	}
	return 0
}
