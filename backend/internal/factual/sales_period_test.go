package factual

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

func prompt98SalesContext(t *testing.T) factualapi.FactualReadContext {
	t.Helper()
	cut := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	ctx, err := factualapi.NewReadContext(factualapi.ContextSpec{
		SellerAccountID: 1,
		AnalysisDate:    time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC),
		CutoffAt:        cut,
		Mode:            factualapi.ModeLatestStable,
		Policies:        factualapi.CurrentPolicies(),
		Domains: []factualapi.DomainBinding{{
			Domain: "SALES_DEMAND", FactFamily: "SALES.ACCOUNT_DEMAND", DomainRunID: 1,
			FinalizedAt: factualapi.FinalizedAtPtr(cut.Add(-time.Hour)), ContractHash: "h",
		}},
		Capabilities: []factualapi.CapabilityBinding{
			{CapabilityKey: "SALES.ACCOUNT_DEMAND", Scope: "HISTORICAL_QUERY", FinalState: "READY", AssessmentID: 1, AssessedAt: cut},
			{CapabilityKey: "SALES.CATEGORY_DEMAND", Scope: "HISTORICAL_QUERY", FinalState: "READY", AssessmentID: 2, AssessedAt: cut},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return ctx
}

func prompt98Finish(t *testing.T, amount string, units int64, days int) factualapi.Demand {
	t.Helper()
	signer, err := factualapi.NewTokenSigner([]byte(strings.Repeat("t", 32)))
	if err != nil {
		t.Fatal(err)
	}
	s := &Service{signer: signer}
	p := factualapi.Period{
		From: time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC),
	}
	if periodDays(p) != 7 {
		t.Fatalf("fixture period days=%d want 7", periodDays(p))
	}
	d, err := s.finishDemand(context.Background(), prompt98SalesContext(t), "SALES.ACCOUNT_DEMAND", "ACCOUNT_DEMAND", 1, p, amount, units, days, "RUB", &periodDQAdjust{})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestFinishDemandSixOfSevenDaysUnknown(t *testing.T) {
	d := prompt98Finish(t, "100", 4, 6)
	if d.OrderedAmount.State != factualapi.ValueUnknown || d.OrderedAmount.ReasonCode != "MISSING_DAILY_FACT" {
		t.Fatalf("6/7 must be UNKNOWN MISSING_DAILY_FACT: %+v", d.OrderedAmount)
	}
	if d.OrderedUnits.State == factualapi.ValueKnown {
		t.Fatalf("6/7 units became known: %+v", d.OrderedUnits)
	}
}

func TestFinishDemandSevenOfSevenDaysKnown(t *testing.T) {
	d := prompt98Finish(t, "100", 4, 7)
	if d.OrderedAmount.State != factualapi.ValueKnown || d.OrderedAmount.Value == nil || d.OrderedAmount.Value.Amount.String() != "100" {
		t.Fatalf("7/7 must be KNOWN: %+v", d.OrderedAmount)
	}
	if d.OrderedUnits.State != factualapi.ValueKnown || d.OrderedUnits.Value == nil || *d.OrderedUnits.Value != 4 {
		t.Fatalf("7/7 units: %+v", d.OrderedUnits)
	}
}

func TestFinishDemandKnownZeroIsNotMissing(t *testing.T) {
	d := prompt98Finish(t, "0", 0, 7)
	if d.OrderedAmount.State != factualapi.ValueKnown || d.OrderedAmount.Value == nil || d.OrderedAmount.Value.Amount.String() != "0" {
		t.Fatalf("known zero became missing: %+v", d.OrderedAmount)
	}
	if d.OrderedUnits.State != factualapi.ValueKnown || d.OrderedUnits.Value == nil || *d.OrderedUnits.Value != 0 {
		t.Fatalf("known zero units: %+v", d.OrderedUnits)
	}
}
