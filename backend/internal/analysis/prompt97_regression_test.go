package analysis

import (
	"math/big"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/landscape"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/materiality"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
)

func prompt97DateRange(from, to string) landscape.DateRange {
	return landscape.DateRange{From: canonicaljson.Date(mustPrompt97Date(from)), To: canonicaljson.Date(mustPrompt97Date(to))}
}

func mustPrompt97Date(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func prompt97AdWindow(family string, cur, prev landscape.DateRange) landscape.DomainWindow {
	c, p := cur, prev
	return landscape.DomainWindow{Domain: "advertising", FactFamily: family, Current: &c, Previous: &p}
}

func TestPrompt97DRRTrafficReadyAttributionUnavailable(t *testing.T) {
	caps := map[string]string{
		"ADVERTISING.CAMPAIGN_TRAFFIC":     materiality.ReadinessReady,
		"ADVERTISING.CAMPAIGN_ATTRIBUTION": "UNAVAILABLE",
		"ADVERTISING.CAMPAIGN_DRR":         materiality.ReadinessReady,
	}
	core := materiality.CoreFacts{
		Account: materiality.AccountFeatures{
			SemanticKey: "a", PaidSpendCurrent7: materiality.KnownExact(materiality.Exact(big.NewInt(100_000).String())),
		},
		Campaigns: []materiality.CampaignFeatures{{
			SemanticKey: "camp-drr", CanonicalRefID: "9",
			TrafficAvailable: true, AttributionAvailable: true, DRRAvailable: true,
			PaidSpendCurrent7:  materiality.KnownExact(materiality.Exact(big.NewInt(20_000).String())),
			PaidSpendPrevious7: materiality.KnownExact(materiality.Exact(big.NewInt(18_000).String())),
			DRRCurrentPP:       materiality.KnownExact(materiality.Exact("20")),
			DRRPreviousPP:      materiality.KnownExact(materiality.Exact("10")),
		}},
		Capabilities: caps,
	}
	r := materiality.BuildMaterialityRanking(core, materiality.CurrentPolicy(), "")
	for _, e := range r.Campaigns {
		for _, m := range e.LaneMemberships {
			if m.LaneKey == "DRR_ABSOLUTE_PP_CHANGE" {
				t.Fatal("DRR: Traffic READY + Attribution UNAVAILABLE must keep lane absent")
			}
		}
	}
}

func TestPrompt97AdvertisingNilPeriodFailClosed(t *testing.T) {
	cur := prompt97DateRange("2026-08-18", "2026-08-24")
	prev := prompt97DateRange("2026-08-11", "2026-08-17")
	windows := []landscape.DomainWindow{
		prompt97AdWindow("ADVERTISING.TRAFFIC", cur, prev),
		prompt97AdWindow("ADVERTISING.ATTRIBUTION", cur, prev),
	}
	curP, prevP := cur, prev
	set := landscape.LandscapeSet{
		DomainWindows: windows,
		Account: landscape.AccountLandscape{
			Advertising: &landscape.AccountAdsBlock{
				Traffic: &landscape.AccountAdsTrafficBlock{
					State: landscape.BlockReady, CurrentPeriod: nil, PreviousPeriod: &prevP,
				},
			},
		},
	}
	if err := validateAdvertisingPeriodCoherenceV4(set); err == nil {
		t.Fatal("READY block + nil CurrentPeriod must FAIL")
	}
	set.Account.Advertising.Traffic.CurrentPeriod = &curP
	set.Account.Advertising.Traffic.PreviousPeriod = nil
	if err := validateAdvertisingPeriodCoherenceV4(set); err == nil {
		t.Fatal("READY block + nil PreviousPeriod must FAIL")
	}
	set.DomainWindows = nil
	set.Account.Advertising.Traffic.PreviousPeriod = &prevP
	if err := validateAdvertisingPeriodCoherenceV4(set); err == nil {
		t.Fatal("READY block + missing DomainWindow must FAIL")
	}
	set.DomainWindows = windows
	mismatch := prompt97DateRange("2026-01-01", "2026-01-07")
	set.Account.Advertising.Traffic.CurrentPeriod = &mismatch
	if err := validateAdvertisingPeriodCoherenceV4(set); err == nil {
		t.Fatal("current period mismatch must FAIL")
	}
	set.Account.Advertising.Traffic.CurrentPeriod = &curP
	set.Account.Advertising.Traffic.PreviousPeriod = &mismatch
	if err := validateAdvertisingPeriodCoherenceV4(set); err == nil {
		t.Fatal("previous period mismatch must FAIL")
	}
	set.Account.Advertising.Traffic.PreviousPeriod = &prevP
	if err := validateAdvertisingPeriodCoherenceV4(set); err != nil {
		t.Fatalf("matching periods must PASS: %v", err)
	}

	set.Campaigns = []landscape.CampaignLandscape{{
		Identity: landscape.CampaignIdentity{CampaignID: 1},
		Performance: &landscape.CampaignPerfBlock{
			Traffic:     &landscape.CampaignTrafficBlock{State: landscape.BlockReady, CurrentPeriod: &curP, PreviousPeriod: &prevP},
			Attribution: &landscape.CampaignAttributionBlock{State: landscape.BlockReady, CurrentPeriod: &curP, PreviousPeriod: &prevP},
		},
	}}
	set.SKUs = []landscape.SKULandscape{{
		Identity: landscape.SKUIdentity{CanonicalSKUID: 1},
		Advertising: &landscape.SKUAdsBlock{
			Traffic:     &landscape.SKUAdsTrafficBlock{State: landscape.BlockReady, CurrentPeriod: &curP, PreviousPeriod: &prevP},
			Attribution: &landscape.SKUAdsAttributionBlock{State: landscape.BlockReady, CurrentPeriod: &curP, PreviousPeriod: &prevP},
		},
	}}
	set.Categories = []landscape.CategoryLandscape{{
		Identity: landscape.CategoryIdentity{CategoryID: 1},
		Advertising: &landscape.CategoryAdsBlock{
			Traffic:     &landscape.CategoryAdsTrafficBlock{State: landscape.BlockReady, CurrentPeriod: &curP, PreviousPeriod: &prevP},
			Attribution: &landscape.CategoryAdsAttributionBlock{State: landscape.BlockReady, CurrentPeriod: &curP, PreviousPeriod: &prevP},
		},
	}}
	set.Account.Advertising.Attribution = &landscape.AccountAdsAttributionBlock{
		State: landscape.BlockReady, CurrentPeriod: &curP, PreviousPeriod: &prevP,
	}
	if err := validateAdvertisingPeriodCoherenceV4(set); err != nil {
		t.Fatalf("all advertising blocks matching periods must PASS: %v", err)
	}
}

func TestPrompt97DomainWindowsShuffledSameHash(t *testing.T) {
	a := landscape.DomainWindow{Domain: "advertising", FactFamily: "ADVERTISING.TRAFFIC"}
	b := landscape.DomainWindow{Domain: "advertising", FactFamily: "ADVERTISING.ATTRIBUTION"}
	c := landscape.DomainWindow{Domain: "sales"}
	mk := func(order []landscape.DomainWindow) landscape.LandscapeSet {
		return landscape.LandscapeSet{
			Context: landscape.LandscapeContext{
				SellerAccountID: 1, LandscapeSchemaVersion: landscape.LandscapeSchemaV4,
			},
			Account:       landscape.AccountLandscape{Identity: landscape.AccountIdentity{SellerAccountID: 1}},
			DomainWindows: append([]landscape.DomainWindow(nil), order...),
		}
	}
	_, h1, err := landscape.MarshalAndHash(mk([]landscape.DomainWindow{a, b, c}))
	if err != nil {
		t.Fatal(err)
	}
	_, h2, err := landscape.MarshalAndHash(mk([]landscape.DomainWindow{c, b, a}))
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Fatalf("shuffled DomainWindows changed hash: %s vs %s", h1, h2)
	}
}
