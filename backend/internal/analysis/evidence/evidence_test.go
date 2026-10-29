package evidence

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/discovery"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/evidencecatalog"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

func TestMenuResolverParity(t *testing.T) {
	reg := ProductionRegistry(APIReads{})
	for _, tp := range evidencecatalog.Enabled() {
		if !reg.Has(tp.Key) {
			t.Fatalf("enabled %s missing resolver", tp.Key)
		}
	}
	for _, tp := range evidencecatalog.Catalog() {
		if !tp.EnabledForDiscoveryV1 && reg.Has(tp.Key) {
			t.Fatalf("disabled %s has resolver", tp.Key)
		}
	}
}

func TestNormalizeDedup(t *testing.T) {
	u := strings.Repeat("a", 64)
	v := discovery.ValidatedOutputV1{DiscoveryState: discovery.StateThreadsFound, Threads: []discovery.ValidatedThread{
		{ThreadID: "T001", EvidenceRequests: []discovery.EvidenceRequest{
			{EvidenceType: "SKU_SALES_HISTORY", Subject: discovery.PackEntity{Kind: discovery.KindSKU, ID: "417"}, InvestigationQuestion: "q1"},
			{EvidenceType: "SKU_PRICE_HISTORY", Subject: discovery.PackEntity{Kind: discovery.KindSKU, ID: "417"}, InvestigationQuestion: "q2"},
		}},
		{ThreadID: "T002", EvidenceRequests: []discovery.EvidenceRequest{
			{EvidenceType: "SKU_PRICE_HISTORY", Subject: discovery.PackEntity{Kind: discovery.KindSKU, ID: "417"}, InvestigationQuestion: "q3"},
			{EvidenceType: "SKU_SEARCH_QUERIES", Subject: discovery.PackEntity{Kind: discovery.KindSKU, ID: "417"}, InvestigationQuestion: "q4"},
		}},
	}}
	n, err := NormalizeRequests(u, v)
	if err != nil {
		t.Fatal(err)
	}
	if len(CollectRawRequests(v)) != 4 || len(n) != 3 {
		t.Fatalf("raw=%d unique=%d", len(CollectRawRequests(v)), len(n))
	}
	var price *NormalizedEvidenceRequest
	for i := range n {
		if n[i].EvidenceType == "SKU_PRICE_HISTORY" {
			price = &n[i]
		}
	}
	if price == nil || len(price.ThreadRefs) != 2 {
		t.Fatalf("price threads %+v", price)
	}
	n2, _ := NormalizeRequests(u, v)
	if n[0].RequestKey != n2[0].RequestKey || EvidenceID(n[0].Ordinal) != "ev_001" {
		t.Fatal("determinism")
	}
	otherU := strings.Repeat("b", 64)
	n3, _ := NormalizeRequests(otherU, v)
	if n3[0].RequestKey == n[0].RequestKey {
		t.Fatal("universe must change key")
	}
}

func TestZeroRequestsSucceeded(t *testing.T) {
	rc, err := factualapi.NewSealedUniverseContext(factualapi.ContextSpec{
		SellerAccountID: 1, AnalysisDate: time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC),
		CutoffAt: time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC), Policies: factualapi.CurrentPolicies(),
	}, seal{"u", strings.Repeat("a", 64)})
	if err != nil {
		t.Fatal(err)
	}
	out, err := Expand(context.Background(), ExpandInput{
		SellerID: 1, UniverseHash: strings.Repeat("a", 64), AnalysisDate: rc.AnalysisDate(), CutoffAt: rc.CutoffAt(),
		Validated:     discovery.ValidatedOutputV1{DiscoveryState: discovery.StateNoMeaningfulThreads},
		ValidatedHash: strings.Repeat("c", 64), Sealed: rc, Registry: ProductionRegistry(stubReads{}), Budget: CurrentBudget(),
	})
	if err != nil || out.Limited || len(out.Bundle.Items) != 0 {
		t.Fatalf("empty expand err=%v limited=%v items=%d", err, out.Limited, len(out.Bundle.Items))
	}
}

func TestPartialAndFailedCompletion(t *testing.T) {
	reg := NewRegistry()
	reg.Register(fnResolver{key: "SKU_SALES_HISTORY", fn: func(ctx context.Context, rc factualapi.FactualReadContext, reqs []NormalizedEvidenceRequest) []MaterializationResult {
		out := make([]MaterializationResult, len(reqs))
		for i, req := range reqs {
			st := StatusComplete
			if i == 0 {
				st = StatusPartial
			}
			if i == 1 {
				st = StatusFailed
			}
			out[i] = MaterializationResult{Request: req, Status: st, Readiness: "READY", Data: map[string]any{"ok": true}}
		}
		return out
	}})
	v := discovery.ValidatedOutputV1{DiscoveryState: discovery.StateThreadsFound, Threads: []discovery.ValidatedThread{{
		ThreadID: "T001", EvidenceRequests: []discovery.EvidenceRequest{
			{EvidenceType: "SKU_SALES_HISTORY", Subject: discovery.PackEntity{Kind: discovery.KindSKU, ID: "1"}},
			{EvidenceType: "SKU_SALES_HISTORY", Subject: discovery.PackEntity{Kind: discovery.KindSKU, ID: "2"}},
		},
	}}}
	rc, _ := factualapi.NewSealedUniverseContext(factualapi.ContextSpec{
		SellerAccountID: 1, AnalysisDate: time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC),
		CutoffAt: time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC), Policies: factualapi.CurrentPolicies(),
	}, seal{"u", strings.Repeat("a", 64)})
	out, err := Expand(context.Background(), ExpandInput{
		SellerID: 1, UniverseHash: strings.Repeat("a", 64), AnalysisDate: rc.AnalysisDate(), CutoffAt: rc.CutoffAt(),
		Validated: v, ValidatedHash: strings.Repeat("c", 64), Sealed: rc, Registry: reg, Budget: CurrentBudget(),
	})
	if err != nil || !out.Limited || out.Bundle.Summary.Partial != 1 || out.Bundle.Summary.Failed != 1 {
		t.Fatalf("err=%v limited=%v sum=%+v", err, out.Limited, out.Bundle.Summary)
	}
}

func TestIntegrityFatal(t *testing.T) {
	reg := NewRegistry()
	reg.Register(fnResolver{key: "SKU_SALES_HISTORY", fn: func(ctx context.Context, rc factualapi.FactualReadContext, reqs []NormalizedEvidenceRequest) []MaterializationResult {
		return []MaterializationResult{{Request: reqs[0], Integrity: Integrity("cross-seller")}}
	}})
	v := discovery.ValidatedOutputV1{DiscoveryState: discovery.StateThreadsFound, Threads: []discovery.ValidatedThread{{
		ThreadID: "T001", EvidenceRequests: []discovery.EvidenceRequest{{EvidenceType: "SKU_SALES_HISTORY", Subject: discovery.PackEntity{Kind: discovery.KindSKU, ID: "1"}}},
	}}}
	rc, _ := factualapi.NewSealedUniverseContext(factualapi.ContextSpec{
		SellerAccountID: 1, AnalysisDate: time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC),
		CutoffAt: time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC), Policies: factualapi.CurrentPolicies(),
	}, seal{"u", strings.Repeat("a", 64)})
	_, err := Expand(context.Background(), ExpandInput{
		SellerID: 1, UniverseHash: strings.Repeat("a", 64), AnalysisDate: rc.AnalysisDate(), CutoffAt: rc.CutoffAt(),
		Validated: v, Registry: reg, Budget: CurrentBudget(), Sealed: rc,
	})
	if err == nil || !strings.Contains(err.Error(), CodeIntegrityFailure) {
		t.Fatalf("want integrity, got %v", err)
	}
}

func TestPIIGate(t *testing.T) {
	if err := ScanPII([]byte(`{"buyer_name":"x"}`)); err == nil {
		t.Fatal("expected pii")
	}
}

func TestArchitectureNoForbiddenImports(t *testing.T) {
	root := "."
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		s := string(b)
		for _, ban := range []string{"jackc/pgx", "sqlc/dbgen", "internal/factual/repository", "internal/db", "integrations/ozon", "internal/openaix", "unifiedanalysis", "aipipeline"} {
			if strings.Contains(s, ban) {
				t.Errorf("%s imports forbidden %s", path, ban)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestHandleReuseAndNew(t *testing.T) {
	disc := discovery.FactHandleMapV1{Entries: []discovery.HandleEntry{{
		Handle: "f_0001", Entity: discovery.PackEntity{Kind: discovery.KindSKU, ID: "1"}, SemanticPath: "sales.demand.day.2026-08-24", FactRefs: []string{"ref1"}, AnalyticalUsability: discovery.UsabilityReady,
	}}}
	req := NormalizedEvidenceRequest{Subject: discovery.PackEntity{Kind: discovery.KindSKU, ID: "1"}, EvidenceType: "SKU_SALES_HISTORY", Ordinal: 0}
	items := []EvidenceItem{{EvidenceID: "ev_001", EvidenceType: "SKU_SALES_HISTORY", Subject: req.Subject}}
	res := []MaterializationResult{{
		Request: req,
		Facts: []DeclaredFact{
			{SemanticPath: "sales.demand.day.2026-08-24", FactRefs: []string{"ref1"}},
			{SemanticPath: "sales.demand.day.2026-08-23", FactRefs: []string{"ref2"}},
		},
	}}
	reg, out, err := assignHandles(items, res, disc)
	if err != nil {
		t.Fatal(err)
	}
	if len(out[0].FactHandles) != 2 || out[0].FactHandles[0] != "f_0001" || out[0].FactHandles[1] != "ef_0001" {
		t.Fatalf("handles %+v registry=%d", out[0].FactHandles, len(reg.Entries))
	}
}

func TestWorstCaseUniqueBound(t *testing.T) {
	if CurrentBudget().MaxUniqueRequests != 24 {
		t.Fatal("max unique")
	}
	b := CurrentBudget()
	worst := b.MaxUniqueRequests * 80_000
	if worst > b.MaxBundleBytes*2 {
		t.Fatalf("worst per-type bytes %d vs global %d", worst, b.MaxBundleBytes)
	}
}

func TestNPlusOneBatchSales(t *testing.T) {
	st := &countingSales{}
	reg := NewRegistry()
	registerSales(reg, st)
	reqs := make([]NormalizedEvidenceRequest, 8)
	for i := range reqs {
		reqs[i] = NormalizedEvidenceRequest{EvidenceType: "SKU_SALES_HISTORY", Subject: discovery.PackEntity{Kind: discovery.KindSKU, ID: itoa(int64(i + 1))}, RequestKey: itoa(int64(i + 1))}
	}
	rc, _ := factualapi.NewSealedUniverseContext(factualapi.ContextSpec{
		SellerAccountID: 1, AnalysisDate: time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC),
		CutoffAt: time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC), Policies: factualapi.CurrentPolicies(),
	}, seal{"u", strings.Repeat("a", 64)})
	out := reg.by["SKU_SALES_HISTORY"].ResolveBatch(context.Background(), rc, reqs)
	if len(out) != 8 || st.batchCalls != 1 {
		t.Fatalf("batchCalls=%d results=%d", st.batchCalls, len(out))
	}
}

type seal struct{ id, hash string }

func (s seal) SealedBindingHash() string { return s.hash }
func (s seal) SealedUniverseID() string  { return s.id }

type countingSales struct {
	stubReads
	batchCalls int
}

func (c *countingSales) BatchGetSKUDemandSeries(ctx context.Context, rc factualapi.FactualReadContext, ids []int64, sp factualapi.StandardPeriod, ep *factualapi.Period) (factualapi.BatchSKUDemandSeriesResult, error) {
	c.batchCalls++
	var out factualapi.BatchSKUDemandSeriesResult
	for _, id := range ids {
		out.Items = append(out.Items, factualapi.BatchSKUDemandSeriesItem{ID: id, Status: factualapi.BatchFoundEmpty, Series: &factualapi.SKUDemandSeries{CanonicalSKUID: id}})
	}
	return out, nil
}

func TestTruncateMetadata(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	got, tr := boundList(items, 3, "x", "y", nil)
	if !tr.Truncated || len(got) != 3 || tr.Returned != 3 {
		t.Fatalf("%+v %v", tr, got)
	}
}
