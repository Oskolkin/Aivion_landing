package alerts

import "testing"

// allDeclaredAlertTypes lists every AlertType constant in types.go.
// When adding a new alert type, update activeAlertTypes or legacyAlertTypes in generation.go.
func allDeclaredAlertTypes() []AlertType {
	return []AlertType{
		AlertTypeSalesRevenueDrop,
		AlertTypeSalesOrdersDrop,
		AlertTypeSKUOrdersDrop,
		AlertTypeSKUNegativeContribution,
		AlertTypeStockLowCoverage,
		AlertTypeStockOOSRisk,
		AlertTypeStockCriticalSKULowStock,
		AlertTypeFBOSurplusStorageRisk,
		AlertTypeAdWeakCampaignEfficiency,
		AlertTypeSKURevenueDrop,
		AlertTypeStockAdvertisedSKULowStock,
		AlertTypeAdSpendWithoutResult,
		AlertTypeAdBudgetOnWeakSKU,
		AlertTypeAdBudgetOnLowStockSKU,
		AlertTypePriceBelowMinConstraint,
		AlertTypePriceAboveMaxConstraint,
		AlertTypeMarginRiskAtCurrentPrice,
		AlertTypeMarginRiskAtMinPrice,
		AlertTypeMissingPricingConstraintsForKeySKU,
	}
}

func TestAllKnownAlertTypesClassified(t *testing.T) {
	known := allDeclaredAlertTypes()
	if len(known) != len(activeAlertTypes)+len(legacyAlertTypes) {
		t.Fatalf("declared=%d active=%d legacy=%d (must cover every constant)",
			len(known), len(activeAlertTypes), len(legacyAlertTypes))
	}

	for _, tpe := range known {
		active := IsActiveAlertType(tpe)
		legacy := IsLegacyAlertType(tpe)
		if active == legacy {
			t.Fatalf("%s: must be exactly one of active or legacy (active=%v legacy=%v)", tpe, active, legacy)
		}
		if !active && !legacy {
			t.Fatalf("%s: not classified as active or legacy", tpe)
		}
	}
}
