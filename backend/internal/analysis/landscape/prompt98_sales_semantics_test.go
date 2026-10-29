package landscape

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

func unknownDemand(reason string) factualapi.Demand {
	return factualapi.Demand{
		OrderedUnits:  factualapi.Missing[int64](factualapi.ValueUnknown, reason),
		OrderedAmount: factualapi.Missing[factualapi.Money](factualapi.ValueUnknown, reason),
		Meta:          factualapi.FactMeta{Readiness: "PARTIAL", Limitations: []factualapi.Limitation{{Code: reason, Message: "incomplete source period"}}},
	}
}

func knownDemand(amount string, units int64) factualapi.Demand {
	a, err := factualapi.ParseDecimal(amount)
	if err != nil {
		panic(err)
	}
	return factualapi.Demand{
		OrderedUnits:  factualapi.Known(units),
		OrderedAmount: factualapi.Known(factualapi.Money{Amount: a, CurrencyCode: "RUB"}),
		Meta:          factualapi.FactMeta{Readiness: "READY"},
	}
}

func TestPrompt98BatchLimitedSKUDemandIsNotReadyEmpty(t *testing.T) {
	limited := factualapi.SKUDemand{CanonicalSKUID: 7, Demand: unknownDemand("MISSING_DAILY_FACT")}
	items := []factualapi.BatchSKUDemandItem{{
		ID: 7, Status: factualapi.BatchLimited, Demand: &limited,
	}}
	got := indexSKUDemandBatch(items)
	if _, ok := got[7]; !ok {
		t.Fatal("BatchLimited SKU demand was dropped")
	}
	cp := got[7]
	block := skuSalesBlockFromDemand(&cp, nil, nil, true, false, false)
	if block.State == BlockReadyEmpty {
		t.Fatal("limited SKU demand must not become READY_EMPTY")
	}
	if block.State != BlockPartial {
		t.Fatalf("state=%s want PARTIAL", block.State)
	}
	if len(block.Limitations) == 0 {
		t.Fatal("limited SKU sales block missing limitation")
	}
}

func TestPrompt98AccountUnknownCurrentKeepsKnownAnalysisDay(t *testing.T) {
	cur := unknownDemand("MISSING_DAILY_FACT")
	day := knownDemand("10", 1)
	block := accountSalesFromDemand(&cur, nil, &day, nil, nil, nil, nil, nil, nil, false)
	if block.State != BlockPartial {
		t.Fatalf("state=%s want PARTIAL", block.State)
	}
	if block.AnalysisDayAmount == nil || block.AnalysisDayAmount.Current == nil || string(block.AnalysisDayAmount.Current.Amount) != "10" {
		t.Fatalf("analysis day was dropped: %+v", block.AnalysisDayAmount)
	}
	if block.OrderedAmount.Current != nil {
		t.Fatalf("unknown CURRENT_7D leaked as known: %+v", block.OrderedAmount)
	}
}

func TestPrompt98CategoryIncompletePreviousIsPartial(t *testing.T) {
	cur := knownDemand("40", 2)
	prev := unknownDemand("MISSING_DAILY_FACT")
	block := categorySalesFromDemand(
		&factualapi.CategoryDemand{Demand: cur},
		&factualapi.CategoryDemand{Demand: prev},
		nil, nil,
	)
	if block.State != BlockPartial {
		t.Fatalf("state=%s want PARTIAL", block.State)
	}
	if block.OrderedAmount.ChangeState != ChangeNoBase {
		t.Fatalf("fabricated comparison from incomplete previous: %+v", block.OrderedAmount)
	}
	if block.OrderedAmount.AbsoluteDelta != nil {
		t.Fatal("incomplete previous must not produce a comparison delta")
	}
}

func TestPrompt98KnownZeroSalesIsReadyNotReadyEmpty(t *testing.T) {
	zero := knownDemand("0", 0)
	sku := factualapi.SKUDemand{CanonicalSKUID: 1, Demand: zero}
	block := skuSalesBlockFromDemand(&sku, &sku, nil, true, true, false)
	if block.State != BlockReady {
		t.Fatalf("known zero must be READY, got %s", block.State)
	}
	if block.OrderedAmount.Current == nil || string(block.OrderedAmount.Current.Amount) != "0" {
		t.Fatalf("known zero amount dropped: %+v", block.OrderedAmount)
	}
}

func TestPrompt98V5SealsDifferFromV4(t *testing.T) {
	if CoreProfileVersionSealV5() == CoreProfileVersionSealV4() {
		t.Fatal("landscape core V5 seal must differ from V4")
	}
	if DetailProfileVersionSealV5() == DetailProfileVersionSealV4() {
		t.Fatal("landscape detail V5 seal must differ from V4")
	}
	if LandscapeSchemaV5 == LandscapeSchemaV4 {
		t.Fatal("schema V5 identifier must differ")
	}
}
