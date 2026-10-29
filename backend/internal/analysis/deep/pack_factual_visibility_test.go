package deep

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/continuity"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/discovery"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/evidence"
)

func TestDeepPackFactualVisibilitySalesStockPriceAdsContinuity(t *testing.T) {
	in := sampleInput()
	sku := in.Validated.Threads[0].PrimaryEntity
	in.Bundle.Items = []evidence.EvidenceItem{
		{
			EvidenceID: "ev_sales", EvidenceType: "SKU_SALES_HISTORY", Subject: sku,
			Readiness: "READY", MaterializationStatus: evidence.StatusComplete,
			Coverage:    evidence.Coverage{Notes: []string{"mapped"}},
			Limitations: []evidence.Limitation{{Code: "LAG", Message: "1d"}},
			Data:        json.RawMessage(`{"current_period":{"ordered_amount":"1500"},"previous_period":{"ordered_amount":"900"},"series":[{"period":"2026-08","amount":"1500"}],"coverage":{"mapped_share":"1.0"}}`),
			FactHandles: []string{"ef_sales"},
		},
		{
			EvidenceID: "ev_stock", EvidenceType: "SKU_STOCK_HISTORY", Subject: sku,
			Readiness: "READY", MaterializationStatus: evidence.StatusComplete,
			Data:        json.RawMessage(`{"observations":[{"units":40,"observation_at":"2026-08-30T12:00:00Z","coverage":"snapshot"}],"sampling":"OBSERVATION_HISTORY"}`),
			FactHandles: []string{"ef_stock"},
		},
		{
			EvidenceID: "ev_price", EvidenceType: "SKU_PRICE_HISTORY", Subject: sku,
			Readiness: "READY", MaterializationStatus: evidence.StatusComplete,
			Data:        json.RawMessage(`{"observations":[{"customer_price":"2300","observed_at":"2026-08-30T10:00:00Z","currency_code":"RUB"}]}`),
			FactHandles: []string{"ef_price"},
		},
		{
			EvidenceID: "ev_ads", EvidenceType: "SKU_ADVERTISING_HISTORY", Subject: sku,
			Readiness: "READY", MaterializationStatus: evidence.StatusComplete,
			Data:        json.RawMessage(`{"provider_period":{"from":"2026-08-01","to":"2026-08-30"},"spend":"120.5","clicks":42,"scope":"SKU","limitations":["attribution_lag"]}`),
			FactHandles: []string{"ef_ads"},
		},
	}
	in.Bundle.ContinuitySupportRelations = []evidence.ContinuitySupportEvidenceRelation{{
		PreviousCaseRef: "case1", PreviousProposalRef: "prop1", SupportKey: "B001", EvidenceID: "ev_stock",
	}}
	in.Bundle.ThreadEvidenceRelations = []evidence.ThreadRequestRelation{
		{ThreadID: "T001", EvidenceID: "ev_sales"},
		{ThreadID: "T001", EvidenceID: "ev_price"},
	}
	in.Continuity = continuity.ContinuityContextV1{
		ContinuityContractVersion: continuity.ContractFromPublicationV1,
		PreviousActions: []continuity.PreviousAtomicAction{{
			PreviousActionRef: "prop1", CaseRef: "case1", ProposalRef: "prop1",
			ActionType: "SKU_ADJUST_PRICE", Target: sku,
			RequiredSupports: []continuity.PackedSupport{{
				SupportKey: "B001", Role: "REQUIRED_SUPPORT", Claim: "stock basis",
				Revalidation: continuity.SupportRevalidationSpec{EvidenceType: "SKU_STOCK_HISTORY", Subject: sku},
			}},
		}},
	}
	in.Handles.Entries = []evidence.UnifiedHandle{
		{Handle: "f_0001", Origin: "DISCOVERY", Entity: sku, SemanticPath: "sales.ordered_amount"},
		{Handle: "ef_sales", Origin: "EVIDENCE", EvidenceID: "ev_sales", Entity: sku, SemanticPath: "sales.history"},
		{Handle: "ef_stock", Origin: "EVIDENCE", EvidenceID: "ev_stock", Entity: sku, SemanticPath: "stock.units"},
		{Handle: "ef_price", Origin: "EVIDENCE", EvidenceID: "ev_price", Entity: sku, SemanticPath: "price.history"},
		{Handle: "ef_ads", Origin: "EVIDENCE", EvidenceID: "ev_ads", Entity: sku, SemanticPath: "ads.spend"},
		{Handle: "ef_unused", Origin: "EVIDENCE", EvidenceID: "ev_other", Entity: sku, SemanticPath: "other"},
	}
	built, err := BuildPack(in)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(built.Payload.EvidenceRegistry)
	s := string(raw)
	for _, want := range []string{"1500", "900", `"units":40`, "2300", "120.5", "provider_period"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing factual %q in %s", want, s)
		}
	}
	if len(built.Payload.ContinuitySupportEvidenceLinks) != 1 {
		t.Fatal("continuity link missing")
	}
	seen := map[string]bool{}
	for _, f := range built.Payload.FactRegistry {
		seen[f.Handle] = true
	}
	for _, h := range []string{"ef_sales", "ef_stock", "ef_price", "ef_ads", "f_0001"} {
		if !seen[h] {
			t.Fatalf("missing needed handle %s", h)
		}
	}
	if seen["ef_unused"] {
		t.Fatal("unrelated handle leaked into fact_registry")
	}
	_ = discovery.KindSKU
}
