package factual

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

func TestFactFamilyRoutingNoAmbiguousDomain(t *testing.T) {
	cut := time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)
	fromTraffic := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	toTraffic := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	fromAttr := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	toAttr := time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC)

	ctx, err := factualapi.NewReadContext(factualapi.ContextSpec{
		SellerAccountID: 1,
		AnalysisDate:    time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC),
		CutoffAt:        cut,
		Mode:            factualapi.ModeLatestStable,
		Policies:        factualapi.V1Policies(),
		Domains: []factualapi.DomainBinding{
			{
				Domain: "ADVERTISING", FactFamily: "ADVERTISING.TRAFFIC", DomainRunID: 10,
				FinalizedAt: factualapi.FinalizedAtPtr(cut.Add(-time.Hour)), ContractHash: "traffic",
				CoverageFrom: &fromTraffic, CoverageTo: &toTraffic,
			},
			{
				Domain: "ADVERTISING", FactFamily: "ADVERTISING.ATTRIBUTION", DomainRunID: 20,
				FinalizedAt: factualapi.FinalizedAtPtr(cut.Add(-2 * time.Hour)), ContractHash: "attr",
				CoverageFrom: &fromAttr, CoverageTo: &toAttr,
			},
		},
		Capabilities: []factualapi.CapabilityBinding{
			{CapabilityKey: "ADVERTISING.CAMPAIGN_TRAFFIC", Scope: "HISTORICAL_QUERY", FinalState: "READY", AssessmentID: 1, AssessedAt: cut},
			{CapabilityKey: "ADVERTISING.CAMPAIGN_ATTRIBUTION", Scope: "HISTORICAL_QUERY", FinalState: "READY", AssessmentID: 2, AssessedAt: cut},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !ctx.HasBinding("ADVERTISING", "ADVERTISING.TRAFFIC") || !ctx.HasBinding("ADVERTISING", "ADVERTISING.ATTRIBUTION") {
		t.Fatal("expected both ADVERTISING fact-family bindings preserved")
	}
	if _, ok := ctx.DomainBinding("ADVERTISING"); ok {
		t.Fatal("DomainBinding(ADVERTISING) must return ok=false when multiple families exist")
	}

	signer, err := factualapi.NewTokenSigner([]byte(strings.Repeat("k", 32)))
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(nil, signer, 200, 100)

	trafficMeta, err := svc.meta(ctx, "ADVERTISING.CAMPAIGN_TRAFFIC", "AD_DAILY", 1, nil, "h1")
	if err != nil {
		t.Fatal(err)
	}
	if trafficMeta.CoverageFrom == nil || !trafficMeta.CoverageFrom.Equal(fromTraffic) ||
		trafficMeta.CoverageTo == nil || !trafficMeta.CoverageTo.Equal(toTraffic) {
		t.Fatalf("CAMPAIGN_TRAFFIC coverage=%v/%v want TRAFFIC binding", trafficMeta.CoverageFrom, trafficMeta.CoverageTo)
	}

	attrMeta, err := svc.meta(ctx, "ADVERTISING.CAMPAIGN_ATTRIBUTION", "CPO_ATTRIBUTION", 1, nil, "h2")
	if err != nil {
		t.Fatal(err)
	}
	if attrMeta.CoverageFrom == nil || !attrMeta.CoverageFrom.Equal(fromAttr) ||
		attrMeta.CoverageTo == nil || !attrMeta.CoverageTo.Equal(toAttr) {
		t.Fatalf("CAMPAIGN_ATTRIBUTION coverage=%v/%v want ATTRIBUTION binding", attrMeta.CoverageFrom, attrMeta.CoverageTo)
	}
}

func TestRequireFactFamilyBinding(t *testing.T) {
	cut := time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)
	ctx, err := factualapi.NewReadContext(factualapi.ContextSpec{
		SellerAccountID: 1,
		AnalysisDate:    time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC),
		CutoffAt:        cut,
		Mode:            factualapi.ModeLatestStable,
		Policies:        factualapi.V1Policies(),
		Domains: []factualapi.DomainBinding{
			{Domain: "STOCK", FactFamily: "STOCK.SNAPSHOT", DomainRunID: 7, FinalizedAt: factualapi.FinalizedAtPtr(cut.Add(-time.Hour)), ContractHash: "s"},
		},
		Capabilities: []factualapi.CapabilityBinding{
			{CapabilityKey: "STOCK.SKU_CURRENT", Scope: "HISTORICAL_QUERY", FinalState: "READY", AssessmentID: 1, AssessedAt: cut},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	b, err := requireFactFamilyBinding(ctx, "STOCK", "STOCK.SNAPSHOT")
	if err != nil || b.DomainRunID != 7 {
		t.Fatalf("binding=%+v err=%v", b, err)
	}
	_, err = requireFactFamilyBinding(ctx, "STOCK", "STOCK.WAREHOUSE")
	if err == nil {
		t.Fatal("expected CodeContextInvalid for missing family")
	}
	var fe *factualapi.Error
	if !errors.As(err, &fe) || fe.Code != factualapi.CodeContextInvalid {
		t.Fatalf("err=%v", err)
	}
}
