package devseed

import (
	"os"
	"strings"
	"testing"
)

func TestAssertDevSeedAllowed_RequiresFlag(t *testing.T) {
	t.Setenv("ALLOW_DEV_SEED", "")
	t.Setenv("APP_ENV", "local")
	if err := AssertDevSeedAllowed(); err == nil {
		t.Fatal("expected error without ALLOW_DEV_SEED")
	}
}

func TestAssertDevSeedAllowed_BlocksProduction(t *testing.T) {
	t.Setenv("ALLOW_DEV_SEED", "1")
	t.Setenv("APP_ENV", "production")
	if err := AssertDevSeedAllowed(); err == nil {
		t.Fatal("expected production block")
	}
}

func TestAssertDevSeedAllowed_AllowsLocal(t *testing.T) {
	t.Setenv("ALLOW_DEV_SEED", "1")
	t.Setenv("APP_ENV", "local")
	if err := AssertDevSeedAllowed(); err != nil {
		t.Fatalf("expected allowed: %v", err)
	}
}

func TestDevAdEfficiencyDayIndexes_PartialHasFourDays(t *testing.T) {
	idx := devAdEfficiencyDayIndexes(AdEfficiencyScenarioPartial)
	if len(idx) != 4 {
		t.Fatalf("got %d want 4", len(idx))
	}
}

func TestAllocateDevAdEfficiencyDay_TargetDRRWithinTolerance(t *testing.T) {
	defs := devAdEfficiencyCampaignDefs()
	for dayIdx := range devAdEfficiencyTrendDRR {
		rows := allocateDevAdEfficiencyDay(dayIdx, defs)
		var spend, revenue float64
		for _, row := range rows {
			spend += row.spend
			revenue += row.revenue
		}
		if revenue <= 0 {
			t.Fatalf("day %d revenue must be > 0", dayIdx)
		}
		got := (spend / revenue) * 100
		want := devAdEfficiencyTrendDRR[dayIdx]
		if got < want-6 || got > want+6 {
			t.Fatalf("day %d drr got %.1f want %.1f", dayIdx, got, want)
		}
	}
}

func TestDevAdEfficiencyCampaignIDs_InFixtureRange(t *testing.T) {
	for _, def := range devAdEfficiencyCampaignDefs() {
		if def.ExternalID < DevAdEfficiencyCampaignIDMin || def.ExternalID > DevAdEfficiencyCampaignIDMax {
			t.Fatalf("campaign id %d out of fixture range", def.ExternalID)
		}
	}
}

func TestDevAdEfficiencyCampaignNames_AreSellerFacing(t *testing.T) {
	for _, def := range devAdEfficiencyCampaignDefs() {
		lower := strings.ToLower(def.Name)
		if strings.Contains(def.Name, "[DEV]") {
			t.Fatalf("campaign name contains [DEV]: %q", def.Name)
		}
		if strings.Contains(lower, "ad_waste") {
			t.Fatalf("campaign name contains ad_waste wording: %q", def.Name)
		}
		if strings.Contains(lower, "sku") {
			t.Fatalf("campaign name contains SKU wording: %q", def.Name)
		}
	}
}

func TestDevAdEfficiencyHighDRRRange_NotExtreme(t *testing.T) {
	defs := devAdEfficiencyCampaignDefs()
	rows := allocateDevAdEfficiencyDay(6, defs)
	for _, row := range rows {
		if row.revenue <= 0 {
			continue
		}
		drr := (row.spend / row.revenue) * 100
		if drr > 250 {
			t.Fatalf("unexpected extreme DRR %.1f for campaign %d", drr, row.externalID)
		}
	}
}

func TestDevAdEfficiencyTrend_DefaultFixtureIsMockLike(t *testing.T) {
	max := 0.0
	for _, v := range devAdEfficiencyTrendDRR {
		if v > max {
			max = v
		}
	}
	if max > 45 {
		t.Fatalf("available trend max is too high for visual fixture: %.1f", max)
	}
}

func TestAdEfficiencySeedGuard_NoProductionEnvFromCI(t *testing.T) {
	if os.Getenv("APP_ENV") == "production" && os.Getenv("ALLOW_DEV_SEED") == "1" {
		t.Fatal("do not run dev seed tests with production APP_ENV")
	}
}
