package devseed

import (
	"context"
	"encoding/json"
	"testing"
)

func TestSeedOzonEnrichment_RequiresAllowDevSeed(t *testing.T) {
	t.Setenv("ALLOW_DEV_SEED", "")
	t.Setenv("APP_ENV", "local")
	_, err := SeedOzonEnrichment(context.Background(), nil, OzonEnrichmentSeedOptions{
		SellerAccountID: 1,
		Scenario:        OzonEnrichmentScenarioRich,
	})
	if err == nil {
		t.Fatal("expected error without ALLOW_DEV_SEED")
	}
}

func TestProductInfoVariantDistribution(t *testing.T) {
	t.Parallel()
	counts := map[productInfoVariant]int{}
	for i := 0; i < 100; i++ {
		counts[productInfoVariantForIndex(i)]++
	}
	if counts[productVariantNormal] < 60 {
		t.Fatalf("expected majority normal, got %v", counts)
	}
	if counts[productVariantNoStock] < 5 || counts[productVariantValidation] < 5 || counts[productVariantPriceIndex] < 5 {
		t.Fatalf("expected edge variants, got %v", counts)
	}
}

func TestStockScenarioForIndexCoversAll(t *testing.T) {
	t.Parallel()
	seen := map[stockScenario]bool{}
	for i := 0; i < 20; i++ {
		seen[stockScenarioForIndex(i)] = true
	}
	for _, want := range []stockScenario{
		stockScenarioDeficit,
		stockScenarioWaitingForSupply,
		stockScenarioSurplus,
		stockScenarioNoSales,
		stockScenarioActual,
	} {
		if !seen[want] {
			t.Fatalf("missing scenario %s", want)
		}
	}
}

func TestDevSeedEnvelopeMarksSource(t *testing.T) {
	t.Parallel()
	raw := devSeedEnvelope(OzonEnrichmentScenarioRich, map[string]any{"table": "test"})
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["source"] != DevOzonEnrichmentSeedSource {
		t.Fatalf("source=%v", payload["source"])
	}
	if payload["scenario"] != string(OzonEnrichmentScenarioRich) {
		t.Fatalf("scenario=%v", payload["scenario"])
	}
}

func TestBidAmountForAutoCampaignIsOptional(t *testing.T) {
	t.Parallel()
	_, hasBid := bidAmountForSKU(1, campaignKindAutoRunning)
	if hasBid {
		t.Fatal("auto campaign should not require bid amount")
	}
	amount, hasBid := bidAmountForSKU(1, campaignKindManualRunning)
	if !hasBid || amount <= 0 {
		t.Fatalf("manual campaign bid=%v hasBid=%v", amount, hasBid)
	}
}

func TestNormalizeMicroRublesHelper(t *testing.T) {
	t.Parallel()
	if microRubles(5.5) != 5_500_000 {
		t.Fatalf("got %d", microRubles(5.5))
	}
}

func TestNumericMoneyAmountZeroIsValid(t *testing.T) {
	t.Parallel()
	n := numericMoneyAmount(0)
	if !n.Valid {
		t.Fatal("expected Valid=true for zero revenue/spend")
	}
	f, err := n.Float64Value()
	if err != nil || !f.Valid || f.Float64 != 0 {
		t.Fatalf("expected 0.00, got valid=%v value=%v err=%v", f.Valid, f.Float64, err)
	}
}
