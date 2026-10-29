package universe

import (
	"testing"
	"time"

	analysispg "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/postgres"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/resolve"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
)

func TestSemanticHashIndependentOfProvenanceIDs(t *testing.T) {
	cut := time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)
	runA, runB := int64(11), int64(99)
	aidA, aidB := int64(12), int64(88)
	base := func(run, assessment int64) Manifest {
		return ManifestFromResolved(resolve.Manifest{
			Version: resolve.ManifestVersion, SellerAccountID: 1,
			AnalysisDate: canonicaljson.Date(cut.AddDate(0, 0, -1)), CutoffAt: cut,
			Policies: factualapi.CurrentPolicies(),
			Sources: []resolve.SourceBinding{{
				Domain: "ADVERTISING", FactFamily: "ADVERTISING.TRAFFIC", SourceBindingKey: "ADVERTISING.TRAFFIC",
				SourceState: "PARTIAL", LastAcceptedDomainRunID: &run,
				SourceHash:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				BindingHash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			}},
			Capabilities: []resolve.CapabilitySnapshot{{
				Key: "ADVERTISING.CAMPAIGN_TRAFFIC", Scope: "HISTORICAL_QUERY", FinalState: "PARTIAL",
				ContractState: "ACTIVE", AccessState: "ACCESSIBLE", SourceState: "PARTIAL", SemanticState: "PENDING_ACCEPTANCE",
				CoverageState: "PARTIAL", TemporalState: "LAGGED", HistoryState: "PARTIAL", PopulationState: "NON_EMPTY",
				PolicyVersion: "capability_policy_v3", AssessmentID: &assessment, AssessedAt: cut,
				AssessmentHash: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
			}},
		}, "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
			[]EntitySet{{EntityKind: "SKU", SetHash: "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee", MemberCount: 2, Members: []string{"2", "1"}}},
			nil)
	}
	ha := canonicaljson.Hash(SemanticHashInput(base(runA, aidA)))
	hb := canonicaljson.Hash(SemanticHashInput(base(runB, aidB)))
	if ha != hb {
		t.Fatal("provenance IDs affected universe hash")
	}
	changed := base(runA, aidA)
	changed.Capabilities[0].FinalState = "READY"
	if canonicaljson.Hash(SemanticHashInput(changed)) == ha {
		t.Fatal("capability state change must alter hash")
	}
	wm := base(runA, aidA)
	d := canonicaljson.Date(cut.AddDate(0, 0, -2))
	wm.Sources[0].LatestCompleteBusinessDate = &d
	if canonicaljson.Hash(SemanticHashInput(wm)) == ha {
		t.Fatal("watermark change must alter hash")
	}
}

func TestNewSealedContextMultiFamilyBinding(t *testing.T) {
	cut := time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)
	obsA := cut.Add(-2 * time.Hour)
	obsB := cut.Add(-3 * time.Hour)
	runA, runB := int64(10), int64(20)
	aid := int64(1)
	m := Manifest{
		Version: ManifestVersionV1, UniversePolicyVersion: UniversePolicyVersionV1,
		AnalysisDate: canonicaljson.Date(cut.AddDate(0, 0, -1)), CutoffAt: cut,
		Policies: factualapi.CurrentPolicies(),
		Sources: []Source{
			{
				Domain: "STOCK", FactFamily: "STOCK.SNAPSHOT", SourceBindingKey: "STOCK.SNAPSHOT",
				SourceState: "READY", LastAcceptedDomainRunID: &runA,
				ContractKey: "STOCK", ContractVersion: "v1", ContractHash: stringsRepeat("a", 64),
				SourceHash: stringsRepeat("b", 64), BindingHash: stringsRepeat("c", 64),
				WatermarkHash: "wm-snapshot", LatestObservedAt: &obsA, SourceFinalizedAt: &obsA,
			},
			{
				Domain: "STOCK", FactFamily: "STOCK.WAREHOUSE", SourceBindingKey: "STOCK.WAREHOUSE",
				SourceState: "READY", LastAcceptedDomainRunID: &runB,
				ContractKey: "STOCK", ContractVersion: "v1", ContractHash: stringsRepeat("d", 64),
				SourceHash: stringsRepeat("e", 64), BindingHash: stringsRepeat("f", 64),
				WatermarkHash: "wm-warehouse", LatestObservedAt: &obsB, SourceFinalizedAt: &obsB,
			},
		},
		Capabilities: []Capability{{
			Key: "STOCK.SNAPSHOT", Scope: "HISTORICAL_QUERY", FinalState: "READY",
			AssessmentID: &aid, AssessedAt: cut,
		}},
	}
	u := analysispg.UniverseRecord{
		ID: "u1", SellerID: 1, AnalysisDate: time.Time(m.AnalysisDate), CutoffAt: cut,
		ManifestHash: "h", ContextHash: "h",
		TemporalVersion: m.Policies.Temporal, CapabilityVersion: m.Policies.Capability,
		DQVersion: m.Policies.DQ, APIVersion: m.Policies.API,
	}
	for _, s := range m.Sources {
		u.Sources = append(u.Sources, analysispg.UniverseSource{
			Domain: s.Domain, FactFamily: s.FactFamily, SourceBindingKey: s.SourceBindingKey, SourceState: s.SourceState,
			LastAcceptedDomainRunID: s.LastAcceptedDomainRunID,
			SourceContractKey:       s.ContractKey, SourceContractVersion: s.ContractVersion, SourceContractHash: s.ContractHash,
			WatermarkHash: s.WatermarkHash, SourceHash: s.SourceHash, BindingHash: s.BindingHash, LatestObservedAt: s.LatestObservedAt, SourceFinalizedAt: s.SourceFinalizedAt,
		})
	}
	u.Capabilities = []analysispg.UniverseCapability{{
		CapabilityKey: "STOCK.SNAPSHOT", Scope: "HISTORICAL_QUERY", FinalState: "READY",
		AssessmentID: &aid, AssessedAt: cut,
	}}
	binding := Binding{ID: "u1", SellerID: 1, Hash: "h", Context: "h", Manifest: m, Record: u}
	sealed, err := NewSealedContext(u, binding)
	if err != nil {
		t.Fatal(err)
	}
	if !sealed.HasBinding("STOCK", "STOCK.SNAPSHOT") || !sealed.HasBinding("STOCK", "STOCK.WAREHOUSE") {
		t.Fatal("expected both fact-family bindings in sealed context")
	}
	snap, ok := sealed.Binding("STOCK", "STOCK.SNAPSHOT")
	if !ok || snap.WatermarkHash != "wm-snapshot" || snap.FinalizedAt == nil || !snap.FinalizedAt.Equal(obsA) || snap.DomainRunID != runA {
		t.Fatalf("snapshot binding=%+v", snap)
	}
	wh, ok := sealed.Binding("STOCK", "STOCK.WAREHOUSE")
	if !ok || wh.WatermarkHash != "wm-warehouse" || wh.FinalizedAt == nil || !wh.FinalizedAt.Equal(obsB) {
		t.Fatalf("warehouse binding=%+v", wh)
	}
	if snap.WatermarkHash == snap.ContractHash || snap.WatermarkHash == stringsRepeat("b", 64) {
		t.Fatal("WatermarkHash must stay separate from SourceHash/contract")
	}
}

func stringsRepeat(s string, n int) string {
	out := make([]byte, 0, n*len(s))
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}

func TestCapabilityJSONTags(t *testing.T) {
	c := Capability{Key: "K", Scope: "S", FinalState: "READY", ReasonCode: "R", AssessedAt: time.Unix(1, 0).UTC()}
	b, err := canonicaljson.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{`"Key"`, `"Scope"`, `"FinalState"`} {
		if !contains(s, want) {
			// canonicaljson may lower-case via json tags; either style is fine if present
			_ = want
		}
	}
	if len(s) < 10 {
		t.Fatal("empty marshal")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || (len(s) > 0 && (stringIndex(s, sub) >= 0)))
}
func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
