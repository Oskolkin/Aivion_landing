package deep

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/continuity"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/evidence"
)

func TestDeepPackPreservesEvidenceData(t *testing.T) {
	in := sampleInput()
	in.Bundle.Items[0].Data = json.RawMessage(`{"current_period":{"ordered_amount":"1200"},"previous_period":{"ordered_amount":"900"},"coverage":{"mapped_share":"1.0"}}`)
	built, err := BuildPack(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(built.Payload.EvidenceRegistry) == 0 {
		t.Fatal("no evidence")
	}
	raw := string(built.Payload.EvidenceRegistry[0].Data)
	if raw == "" || raw == "null" {
		t.Fatal("evidence data stripped")
	}
	if !strings.Contains(raw, "ordered_amount") {
		t.Fatalf("missing factual fields: %s", raw)
	}
}

func TestDeepPackContinuityLinksResolveWithData(t *testing.T) {
	in := sampleInput()
	sku := in.Validated.Threads[0].PrimaryEntity
	in.Continuity = continuity.ContinuityContextV1{
		ContinuityContractVersion: continuity.ContractFromPublicationV1,
		PreviousActions: []continuity.PreviousAtomicAction{{
			PreviousActionRef: "prop1", CaseRef: "case1", ProposalRef: "prop1",
			ActionType: "SKU_ADJUST_PRICE", Target: sku,
			RequiredSupports: []continuity.PackedSupport{{
				SupportKey: "B001", Role: "REQUIRED_SUPPORT", Claim: "price basis",
				Revalidation: continuity.SupportRevalidationSpec{EvidenceType: "SKU_PRICE_HISTORY", Subject: sku},
			}},
		}},
	}
	in.Bundle.ContinuitySupportRelations = []evidence.ContinuitySupportEvidenceRelation{{
		PreviousCaseRef: "case1", PreviousProposalRef: "prop1", SupportKey: "B001", EvidenceID: "ev_001",
	}}
	built, err := BuildPack(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(built.Payload.ContinuitySupportEvidenceLinks) != 1 {
		t.Fatalf("links=%d", len(built.Payload.ContinuitySupportEvidenceLinks))
	}
	for _, it := range built.Payload.EvidenceRegistry {
		if it.EvidenceID == "ev_001" && len(it.Data) == 0 {
			t.Fatal("continuity evidence missing data")
		}
	}
}

func TestDeepPackOverBudgetFailsClosed(t *testing.T) {
	t.Setenv("OPENAI_DEEP_MAX_INPUT_TOKENS", "512")
	in := sampleInput()
	_, err := BuildPack(in)
	if err == nil {
		t.Fatal("expected DEEP_PACK_TOO_LARGE")
	}
	if !strings.Contains(err.Error(), CodePackTooLarge) {
		t.Fatalf("err=%v", err)
	}
}

func TestDeepPackFactHandlesOnlyInRegistry(t *testing.T) {
	in := sampleInput()
	want := in.Bundle.Items[0].FactHandles[0]
	if want == "" {
		want = "ef_test"
		in.Bundle.Items[0].FactHandles = []string{want}
	}
	built, err := BuildPack(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(built.Payload.EvidenceRegistry[0].FactHandles) != 0 {
		t.Fatal("fact handles should not duplicate in evidence item")
	}
	found := false
	for _, f := range built.Payload.FactRegistry {
		if f.Handle == want {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing handle %s in registry", want)
	}
}

func TestDeepPackWithinBudgetDeterministic(t *testing.T) {
	in := sampleInput()
	a, err := BuildPack(in)
	if err != nil {
		t.Fatal(err)
	}
	b, err := BuildPack(in)
	if err != nil {
		t.Fatal(err)
	}
	if a.Pack.AuditEnvelope.PayloadHash != b.Pack.AuditEnvelope.PayloadHash {
		t.Fatal("payload hash not deterministic")
	}
	if a.EstimatedTokens != b.EstimatedTokens {
		t.Fatal("token estimate not deterministic")
	}
}

func TestFactRegistryCoversReferencedHandles(t *testing.T) {
	in := sampleInput()
	built, err := BuildPack(in)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, f := range built.Payload.FactRegistry {
		seen[f.Handle] = true
	}
	for _, th := range in.Validated.Threads {
		for _, h := range th.BasisFactHandles {
			if !seen[h] {
				t.Fatalf("missing handle %s in registry", h)
			}
		}
	}
	for _, it := range built.Payload.EvidenceRegistry {
		for _, h := range in.Bundle.Items {
			if h.EvidenceID == it.EvidenceID {
				for _, fh := range h.FactHandles {
					if fh != "" && !seen[fh] {
						t.Fatalf("missing evidence handle %s", fh)
					}
				}
			}
		}
	}
}

func TestDeepPackGoldenSnapshotReadable(t *testing.T) {
	in := sampleInput()
	in.Bundle.Items = append(in.Bundle.Items, evidence.EvidenceItem{
		EvidenceID: "ev_002", EvidenceType: "SKU_STOCK_HISTORY", Subject: in.Validated.Threads[0].PrimaryEntity,
		Readiness: "READY", MaterializationStatus: evidence.StatusComplete,
		Data:        json.RawMessage(`{"units":42,"observation_at":"2026-08-30T12:00:00Z"}`),
		FactHandles: []string{"ef_0002"},
	})
	in.Handles.Entries = append(in.Handles.Entries, evidence.UnifiedHandle{
		Handle: "ef_0002", Origin: "EVIDENCE", EvidenceID: "ev_002",
		Entity: in.Validated.Threads[0].PrimaryEntity, SemanticPath: "stock.units",
	})
	built, err := BuildPack(in)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.MarshalIndent(built.Payload.EvidenceRegistry, "", "  ")
	if !strings.Contains(string(raw), "units") {
		t.Fatalf("golden evidence missing values: %s", raw)
	}
}
