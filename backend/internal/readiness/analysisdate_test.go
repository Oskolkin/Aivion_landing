package readiness

import (
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

func TestBuildContextMultiFamilySEARCH(t *testing.T) {
	cut := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	finishedAt := cut.Add(-6 * time.Hour)
	summaryTo := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	detailTo := time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC)
	marketTo := time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC)
	contractHash := factualapi.DeriveSourceContractHash("OZON_SEARCH", "1.0.0", "SEARCH_ANALYTICS")
	mappingHash := factualapi.StableHash("semantic_mapping", "search-v1")

	svc := &Service{}
	runID := int64(7)
	r := Result{
		SellerAccountID: 1,
		DQRunID:         42,
		CutoffAt:        cut,
		AnalysisDate:    ptrDate(cut.AddDate(0, 0, -1)),
		Policies:        factualapi.CurrentPolicies(),
		Watermarks: []Watermark{
			{Domain: "SEARCH", FactFamily: "SEARCH.SELLER_SUMMARY", StateHash: "wm-summary", SourceContractHash: contractHash, SemanticMappingHash: mappingHash, HistoryTo: &summaryTo, DomainRunID: &runID},
			{Domain: "SEARCH", FactFamily: "SEARCH.QUERY_DETAILS", StateHash: "wm-detail", SourceContractHash: contractHash, SemanticMappingHash: mappingHash, HistoryTo: &detailTo, DomainRunID: &runID},
			{Domain: "SEARCH", FactFamily: "SEARCH.MARKET_SEARCH", StateHash: "wm-market", SourceContractHash: contractHash, SemanticMappingHash: mappingHash, HistoryTo: &marketTo, DomainRunID: &runID},
		},
	}
	ds := []domainEvidence{{
		domain: "SEARCH", runID: runID, finishedAt: finishedAt,
		contractKey: "OZON_SEARCH", contractVersion: "1.0.0", endpointFamily: "SEARCH_ANALYTICS",
		mappingVersion: "search-v1", semanticMappingHash: mappingHash,
	}}

	ctx, err := svc.buildContext(r, ds)
	if err != nil {
		t.Fatal(err)
	}
	searchBindings := ctx.BindingsForDomain("SEARCH")
	if len(searchBindings) != 3 {
		t.Fatalf("expected 3 SEARCH bindings, got %d: %+v", len(searchBindings), searchBindings)
	}
	byFamily := map[string]factualapi.DomainBinding{}
	for _, b := range searchBindings {
		byFamily[b.FactFamily] = b
	}
	wantCoverage := map[string]time.Time{
		"SEARCH.SELLER_SUMMARY": summaryTo,
		"SEARCH.QUERY_DETAILS":  detailTo,
		"SEARCH.MARKET_SEARCH":  marketTo,
	}
	for family, wantTo := range wantCoverage {
		b, ok := byFamily[family]
		if !ok {
			t.Fatalf("missing binding for %s", family)
		}
		if b.FinalizedAt == nil || !b.FinalizedAt.Equal(finishedAt) {
			t.Fatalf("%s FinalizedAt=%v want %v", family, b.FinalizedAt, finishedAt)
		}
		if b.CoverageTo == nil || !b.CoverageTo.Equal(wantTo) {
			t.Fatalf("%s CoverageTo=%v want %v", family, b.CoverageTo, wantTo)
		}
		if b.ContractKey != "OZON_SEARCH" || b.ContractVersion != "1.0.0" || b.ContractHash != contractHash {
			t.Fatalf("%s contract fields wrong: %+v", family, b)
		}
	}
	if _, ok := ctx.DomainBinding("SEARCH"); ok {
		t.Fatal("DomainBinding(SEARCH) must be ambiguous with 3 fact families")
	}
}

func TestBuildContextNullFinalizedAt(t *testing.T) {
	cut := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	obs := cut.Add(-2 * time.Hour)
	runID := int64(3)
	r := Result{
		SellerAccountID: 1, DQRunID: 1, CutoffAt: cut,
		AnalysisDate: ptrDate(cut.AddDate(0, 0, -1)),
		Policies:     factualapi.CurrentPolicies(),
		Watermarks: []Watermark{
			{Domain: "CATALOG", FactFamily: "CATALOG.SKU_UNIVERSE", StateHash: "wm", LatestObservedAt: &obs, DomainRunID: &runID},
		},
	}
	ds := []domainEvidence{{domain: "CATALOG", runID: runID}} // finishedAt zero
	ctx, err := (&Service{}).buildContext(r, ds)
	if err != nil {
		t.Fatal(err)
	}
	b, ok := ctx.Binding("CATALOG", "CATALOG.SKU_UNIVERSE")
	if !ok || b.FinalizedAt != nil {
		t.Fatalf("FinalizedAt=%v want nil when run finished_at unknown", b.FinalizedAt)
	}
}

func ptrDate(v time.Time) *time.Time { return &v }
