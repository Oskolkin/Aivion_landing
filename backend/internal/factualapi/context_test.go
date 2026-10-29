package factualapi

import (
	"strings"
	"testing"
	"time"
)

func testSpec() ContextSpec {
	cut := time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)
	return ContextSpec{SellerAccountID: 1, AnalysisDate: time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC), CutoffAt: cut, Mode: ModeLatestStable, Policies: V1Policies(), Domains: []DomainBinding{{Domain: "CATALOG", FactFamily: "CATALOG.SKU_UNIVERSE", DomainRunID: 1, FinalizedAt: FinalizedAtPtr(cut.Add(-time.Hour)), ContractHash: "h"}}, Capabilities: []CapabilityBinding{{CapabilityKey: "CATALOG.SKU_UNIVERSE", Scope: "HISTORICAL_QUERY", FinalState: "READY", AssessmentID: 1, AssessedAt: cut}}}
}
func TestReadContextHashDeterminismAndInputs(t *testing.T) {
	a, e := NewReadContext(testSpec())
	if e != nil {
		t.Fatal(e)
	}
	b, _ := NewReadContext(testSpec())
	if a.ContextHash() != b.ContextHash() {
		t.Fatal("same context changed hash")
	}
	variants := []func(*ContextSpec){func(s *ContextSpec) { s.SellerAccountID = 2 }, func(s *ContextSpec) {
		s.CutoffAt = s.CutoffAt.Add(time.Second)
		s.Capabilities[0].AssessedAt = s.Capabilities[0].AssessedAt.Add(time.Second)
	}, func(s *ContextSpec) { s.Policies.DQ = "dq_v2" }, func(s *ContextSpec) { s.Domains[0].DomainRunID = 2 }}
	for i, v := range variants {
		s := testSpec()
		v(&s)
		x, e := NewReadContext(s)
		if e != nil {
			t.Fatalf("variant %d: %v", i, e)
		}
		if x.ContextHash() == a.ContextHash() {
			t.Fatalf("variant %d did not change hash", i)
		}
	}
}
func TestReadContextCopiesBindingsAndSealedRequiresBinding(t *testing.T) {
	s := testSpec()
	c, e := NewReadContext(s)
	if e != nil {
		t.Fatal(e)
	}
	s.Domains[0].DomainRunID = 99
	b, _ := c.DomainBinding("CATALOG")
	if b.DomainRunID != 1 {
		t.Fatal("context leaked caller mutation")
	}
	s = testSpec()
	s.Mode = ModeSealedUniverse
	if _, e = NewReadContext(s); e == nil {
		t.Fatal("unvalidated sealed context accepted")
	}
	s = testSpec()
	s.AnalysisDate = time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	if _, e = NewReadContext(s); e == nil {
		t.Fatal("unfinished Moscow business day accepted as analysis date")
	}
}
func TestPeriodsMissingZeroAndSignedTokens(t *testing.T) {
	c, _ := NewReadContext(testSpec())
	p, e := ResolvePeriod(c, PeriodPrevious7D, nil)
	if e != nil || p.From.Format("2006-01-02") != "2026-08-11" || p.To.Format("2006-01-02") != "2026-08-17" {
		t.Fatalf("period=%+v err=%v", p, e)
	}
	zero := Known(int64(0))
	if zero.State != ValueKnown || zero.Value == nil || *zero.Value != 0 {
		t.Fatal("known zero lost")
	}
	missing := Missing[int64](ValueUnavailable, "NO_SOURCE")
	if missing.Value != nil || missing.State == ValueKnown {
		t.Fatal("missing became zero")
	}
	signer, _ := NewTokenSigner([]byte(strings.Repeat("k", 32)))
	h := StableHash("f")
	tok, _ := signer.PageToken(PageCursor{API: APIVersionV1, ContextHash: c.ContextHash(), Method: "ListSKUs", FilterHash: h, Ordering: "id_asc_v1", LastID: 10})
	if _, e = signer.ParsePageToken(tok, c, "ListSKUs", h); e != nil {
		t.Fatal(e)
	}
	if _, e = signer.ParsePageToken(tok+"x", c, "ListSKUs", h); e == nil {
		t.Fatal("tampered token accepted")
	}
	if _, e = signer.ParsePageToken(tok, c, "ListSKUs", StableHash("other")); e == nil {
		t.Fatal("filter mismatch accepted")
	}
	ref, _ := signer.FactRef(c, TraceInput{Kind: "SKU", EntityID: 1, ResultHash: "r"})
	if _, e = signer.ParseFactRef(ref, c); e != nil {
		t.Fatal(e)
	}
	if _, e = signer.ParseFactRef(ref+"x", c); e == nil {
		t.Fatal("tampered FactRef accepted")
	}
	other := testSpec()
	other.SellerAccountID = 2
	otherContext, _ := NewReadContext(other)
	if _, e = signer.ParseFactRef(ref, otherContext); e == nil {
		t.Fatal("cross-context FactRef accepted")
	}
}

func TestMultiFamilyBindingIdentityAndHash(t *testing.T) {
	cut := time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)
	base := ContextSpec{
		SellerAccountID: 1, AnalysisDate: time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC), CutoffAt: cut,
		Mode: ModeLatestStable, Policies: V1Policies(),
		Capabilities: []CapabilityBinding{{CapabilityKey: "STOCK.SNAPSHOT", Scope: "HISTORICAL_QUERY", FinalState: "READY", AssessmentID: 1, AssessedAt: cut}},
	}
	a := base
	a.Domains = []DomainBinding{
		{Domain: "STOCK", FactFamily: "STOCK.SNAPSHOT", DomainRunID: 1, FinalizedAt: FinalizedAtPtr(cut.Add(-time.Hour)), ContractHash: "a", WatermarkHash: "wm-a"},
		{Domain: "STOCK", FactFamily: "STOCK.WAREHOUSE", DomainRunID: 2, FinalizedAt: FinalizedAtPtr(cut.Add(-2 * time.Hour)), ContractHash: "b", WatermarkHash: "wm-b"},
	}
	ctxA, err := NewReadContext(a)
	if err != nil {
		t.Fatal(err)
	}
	if !ctxA.HasBinding("STOCK", "STOCK.SNAPSHOT") || !ctxA.HasBinding("STOCK", "STOCK.WAREHOUSE") {
		t.Fatal("expected both fact-family bindings")
	}
	if _, ok := ctxA.DomainBinding("STOCK"); ok {
		t.Fatal("DomainBinding must not succeed when multiple families exist")
	}
	if _, err = ctxA.UniqueDomainBinding("STOCK"); err == nil {
		t.Fatal("UniqueDomainBinding must error on ambiguity")
	}
	snap, ok := ctxA.Binding("STOCK", "STOCK.SNAPSHOT")
	if !ok || snap.DomainRunID != 1 || snap.WatermarkHash != "wm-a" {
		t.Fatalf("snapshot binding=%+v ok=%v", snap, ok)
	}
	wh, ok := ctxA.Binding("STOCK", "STOCK.WAREHOUSE")
	if !ok || wh.DomainRunID != 2 {
		t.Fatalf("warehouse binding=%+v ok=%v", wh, ok)
	}
	names := ctxA.DomainNames()
	if len(names) != 1 || names[0] != "STOCK" {
		t.Fatalf("DomainNames=%v", names)
	}
	all := ctxA.AllBindings()
	if len(all) != 2 || all[0].FactFamily != "STOCK.SNAPSHOT" || all[1].FactFamily != "STOCK.WAREHOUSE" {
		t.Fatalf("AllBindings order=%+v", all)
	}
	byDomain := ctxA.BindingsForDomain("STOCK")
	if len(byDomain) != 2 || byDomain[0].FactFamily != "STOCK.SNAPSHOT" {
		t.Fatalf("BindingsForDomain=%+v", byDomain)
	}

	// Same pairs in reverse input order must hash identically (sort by Domain then FactFamily).
	b := base
	b.Domains = []DomainBinding{
		{Domain: "STOCK", FactFamily: "STOCK.WAREHOUSE", DomainRunID: 2, FinalizedAt: FinalizedAtPtr(cut.Add(-2 * time.Hour)), ContractHash: "b", WatermarkHash: "wm-b"},
		{Domain: "STOCK", FactFamily: "STOCK.SNAPSHOT", DomainRunID: 1, FinalizedAt: FinalizedAtPtr(cut.Add(-time.Hour)), ContractHash: "a", WatermarkHash: "wm-a"},
	}
	ctxB, err := NewReadContext(b)
	if err != nil {
		t.Fatal(err)
	}
	if ctxA.ContextHash() != ctxB.ContextHash() {
		t.Fatal("multi-family ContextHash not order-independent")
	}

	// Distinct FactFamily must not overwrite — duplicate key errors.
	dup := base
	dup.Domains = []DomainBinding{
		{Domain: "STOCK", FactFamily: "STOCK.SNAPSHOT", DomainRunID: 1, FinalizedAt: FinalizedAtPtr(cut.Add(-time.Hour)), ContractHash: "a"},
		{Domain: "STOCK", FactFamily: "STOCK.SNAPSHOT", DomainRunID: 9, FinalizedAt: FinalizedAtPtr(cut.Add(-time.Hour)), ContractHash: "overwrite"},
	}
	if _, err = NewReadContext(dup); err == nil {
		t.Fatal("duplicate (Domain,FactFamily) accepted")
	}

	// Empty FactFamily rejected.
	empty := base
	empty.Domains = []DomainBinding{{Domain: "STOCK", FactFamily: "", DomainRunID: 1, FinalizedAt: FinalizedAtPtr(cut.Add(-time.Hour)), ContractHash: "a"}}
	if _, err = NewReadContext(empty); err == nil {
		t.Fatal("empty FactFamily accepted")
	}

	// Different FactFamily set changes ContextHash.
	c := base
	c.Domains = []DomainBinding{
		{Domain: "STOCK", FactFamily: "STOCK.SNAPSHOT", DomainRunID: 1, FinalizedAt: FinalizedAtPtr(cut.Add(-time.Hour)), ContractHash: "a", WatermarkHash: "wm-a"},
	}
	ctxC, err := NewReadContext(c)
	if err != nil {
		t.Fatal(err)
	}
	if ctxC.ContextHash() == ctxA.ContextHash() {
		t.Fatal("adding a second fact family must change ContextHash")
	}
	u, err := ctxC.UniqueDomainBinding("STOCK")
	if err != nil || u.FactFamily != "STOCK.SNAPSHOT" {
		t.Fatalf("unique=%+v err=%v", u, err)
	}
}
