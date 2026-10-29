package scenarios

import (
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysisdata"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/contract"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/eval"
)

// SnapshotHarness is the minimal surface scenario snapshot builders need.
type SnapshotHarness interface {
	Helper()
	Fatalf(format string, args ...any)
}

// Definition describes one Stage 4 eval scenario pack fixture.
type Definition struct {
	Name     string
	Snapshot func(h SnapshotHarness) *analysisdata.AnalysisDataSnapshot
	Checks   eval.ExpectedChecks
}

// All returns every registered scenario in stable order.
func All() []Definition {
	maxZero := 0
	maxFour := 4
	return []Definition{
		{
			Name:     "scenario_stockout_key_sku",
			Snapshot: SnapshotStockoutKeySKU,
			Checks: eval.ExpectedChecks{
				Outcome:            contract.AnalysisOutcomeMaterialActionsFound,
				RequiredCategories: []string{"stock"},
				RequiredSKUIDs:     []string{"901042000001"},
				RequireTopAction:   true,
				NoDuplicateSKU:     true,
			},
		},
		{
			Name:     "scenario_ad_spend_without_revenue",
			Snapshot: SnapshotAdSpendWithoutRevenue,
			Checks: eval.ExpectedChecks{
				Outcome:             contract.AnalysisOutcomeMaterialActionsFound,
				RequiredCategories:  []string{"advertising"},
				RequiredCampaignIDs: []string{"882042000001"},
				RequireTopAction:    true,
				NoDuplicateSKU:      true,
			},
		},
		{
			Name:     "scenario_funnel_conversion_drop",
			Snapshot: SnapshotFunnelConversionDrop,
			Checks: eval.ExpectedChecks{
				Outcome:            contract.AnalysisOutcomeMaterialActionsFound,
				RequiredCategories: []string{"funnel"},
				RequireTopAction:   true,
				NoDuplicateSKU:     true,
			},
		},
		{
			Name:     "scenario_healthy_no_actions",
			Snapshot: SnapshotHealthyNoActions,
			Checks: eval.ExpectedChecks{
				Outcome:    contract.AnalysisOutcomeNoMaterialActions,
				MaxActions: &maxZero,
			},
		},
		{
			Name:     "scenario_insufficient_data",
			Snapshot: SnapshotInsufficientData,
			Checks: eval.ExpectedChecks{
				Outcome:    contract.AnalysisOutcomeInsufficientData,
				MaxActions: &maxZero,
			},
		},
		{
			Name:     "scenario_multi_sku_grouping",
			Snapshot: SnapshotMultiSKUGrouping,
			Checks: eval.ExpectedChecks{
				Outcome:            contract.AnalysisOutcomeMaterialActionsFound,
				RequiredCategories: []string{"stock", "sku"},
				RequiredSKUIDs:     []string{"901042000001", "901042000003"},
				MaxActions:         &maxFour,
				NoDuplicateSKU:     true,
			},
		},
		{
			Name:     "scenario_slow_turnover_surplus",
			Snapshot: SnapshotSlowTurnoverSurplus,
			Checks: eval.ExpectedChecks{
				Outcome:            contract.AnalysisOutcomeMaterialActionsFound,
				RequiredCategories: []string{"slow_turnover"},
				RequiredSKUIDs:     []string{"901042000005"},
				NoDuplicateSKU:     true,
			},
		},
		{
			Name:     "scenario_mixed_account",
			Snapshot: SnapshotMixedAccount,
			Checks: eval.ExpectedChecks{
				Outcome:            contract.AnalysisOutcomeMaterialActionsFound,
				RequiredCategories: []string{"stock", "advertising", "funnel"},
				RequiredSKUIDs:     []string{"901042000001"},
				RequireTopAction:   true,
				NoDuplicateSKU:     true,
			},
		},
	}
}

// ByName returns a scenario definition by directory name.
func ByName(name string) (Definition, bool) {
	for _, def := range All() {
		if def.Name == name {
			return def, true
		}
	}
	return Definition{}, false
}
