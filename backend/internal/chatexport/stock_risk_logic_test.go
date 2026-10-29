package chatexport

import "testing"

func TestStockRiskIncludesMediumNotOnlyCritical(t *testing.T) {
	days := 5.0
	risk := classifyStockDepletionRisk(10, &days)
	if !isExportableStockRisk(risk) {
		t.Fatalf("medium risk should export: %s", risk)
	}
	if risk != "medium_depletion_risk" {
		t.Fatalf("got %s", risk)
	}
}

func TestStockRiskExcludesHealthySKU(t *testing.T) {
	days := 30.0
	risk := classifyStockDepletionRisk(100, &days)
	if isExportableStockRisk(risk) {
		t.Fatalf("healthy sku should not export: %s", risk)
	}
}
