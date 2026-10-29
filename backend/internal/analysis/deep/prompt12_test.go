package deep

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aitokens"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/actioncatalog"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/continuity"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/discovery"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/evidence"
)

func sampleInput() BuildInput {
	sku := discovery.PackEntity{Kind: discovery.KindSKU, ID: "417"}
	acc := discovery.PackEntity{Kind: discovery.KindAccount, ID: "account"}
	price := "2300 RUB"
	p := discovery.PayloadV1{
		Context:      discovery.PayloadContext{AnalysisDate: "2026-08-30", AnalysisCutoffAt: time.Date(2026, 8, 30, 21, 0, 0, 0, time.UTC).Format(time.RFC3339Nano), BusinessTimezone: "Europe/Moscow"},
		Capabilities: []discovery.PayloadCapability{{Key: "PRICE.SKU", Readiness: "READY"}, {Key: "SALES.SKU_DEMAND", Readiness: "READY"}},
		Account:      discovery.PayloadAccount{Entity: acc},
		SKUs: []discovery.PayloadSKU{{
			Entity: sku,
			Price:  &discovery.PayloadPrice{State: "READY", ObservedPrice: &discovery.PayloadScalar{Value: &price, FactHandle: "f_0001"}},
			Sales:  &discovery.PayloadSales{State: "READY", OrderedAmount: &discovery.PayloadMoneyComparison{FactHandle: "f_0001"}},
		}},
	}
	validated := discovery.ValidatedOutputV1{
		SchemaVersion: discovery.ValidatedOutputSchemaV1, DiscoveryState: discovery.StateThreadsFound,
		Threads: []discovery.ValidatedThread{{
			ThreadID: "T001", Ordinal: 1, PrimaryEntity: sku, Observation: "sales drop", InvestigationFocus: "price",
			BasisFactHandles: []string{"f_0001"},
		}},
	}
	bundle := evidence.BundleV1{
		SchemaVersion: evidence.BundleSchemaV1, FactualUniverseHash: strings64("u"),
		AnalysisDate: "2026-08-30", Items: []evidence.EvidenceItem{{
			EvidenceID: "ev_001", EvidenceType: "SKU_PRICE_HISTORY", Subject: sku,
			Readiness: "READY", MaterializationStatus: evidence.StatusComplete,
			TemporalScope: evidence.TemporalScope{From: "2026-07-01", To: "2026-08-30"},
			FactHandles:   []string{"ef_0001"}, Limitations: []evidence.Limitation{{Code: "LAG", Message: "2d"}},
			Data: json.RawMessage(`{"points":[]}`),
		}},
		ThreadEvidenceRelations: []evidence.ThreadRequestRelation{{ThreadID: "T001", EvidenceID: "ev_001"}},
	}
	handles := evidence.UnifiedHandleRegistryV1{SchemaVersion: evidence.FactHandleRegistryV1, Entries: []evidence.UnifiedHandle{
		{Handle: "f_0001", Origin: "DISCOVERY", Entity: sku, SemanticPath: "sales.ordered_amount"},
		{Handle: "ef_0001", Origin: "EVIDENCE", EvidenceID: "ev_001", Entity: sku, SemanticPath: "price.history"},
	}}
	cutoff := time.Date(2026, 8, 30, 21, 0, 0, 0, time.UTC)
	return BuildInput{
		SellerAccountID: 1, RunID: 9, UniverseHash: strings64("u"),
		AnalysisDate: time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC), CutoffAt: cutoff, ContractHash: strings64("c"),
		DiscoveryPackSHA: strings64("p"), ValidatedDiscoverySHA: strings64("v"), EvidenceBundleSHA: strings64("e"),
		FactRegistrySHA: strings64("f"), ContinuitySHA: strings64("n"),
		DiscoveryEnvelope: discovery.AuditEnvelope{FactualUniverseHash: strings64("u"), SellerAccountID: 1},
		DiscoveryPayload:  p, Validated: validated, Bundle: bundle, Handles: handles,
		Continuity: continuity.ContinuityContextV1{ContinuityContractVersion: continuity.ContractInitialNoBaseV1},
	}
}

func strings64(s string) string {
	b := make([]byte, 64)
	for i := 0; i < 64; i++ {
		b[i] = s[i%len(s)]
	}
	return string(b)
}

func TestPackDeterministicAndGates(t *testing.T) {
	in := sampleInput()
	a, err := BuildPack(in)
	if err != nil {
		t.Fatal(err)
	}
	b, err := BuildPack(in)
	if err != nil {
		t.Fatal(err)
	}
	if string(a.PackBytes) != string(b.PackBytes) {
		t.Fatal("pack bytes not deterministic")
	}
	if a.Pack.AuditEnvelope.PayloadHash != b.Pack.AuditEnvelope.PayloadHash {
		t.Fatal("payload hash")
	}
	foundAcc := false
	for _, e := range a.Payload.ThreadEntityContext {
		if e.Entity.Kind == discovery.KindAccount {
			foundAcc = true
		}
		if strings.Contains(strings.ToLower(e.Role), "score") {
			t.Fatal("materiality leakage")
		}
	}
	if !foundAcc {
		t.Fatal("account always included")
	}
	if len(a.Payload.EvidenceRegistry) != 1 || a.Payload.EvidenceRegistry[0].Limitations[0].Code != "LAG" {
		t.Fatal("limitations not preserved")
	}
	blob := string(a.PayloadBytes)
	if strings.Contains(blob, "rrf") || strings.Contains(blob, "materiality_score") || strings.Contains(blob, "MinIO") {
		t.Fatal("leakage")
	}
	in2 := in
	in2.UniverseHash = strings64("z")
	if _, err = BuildPack(in2); err == nil {
		t.Fatal("binding mismatch expected")
	}
}

func TestNoMeaningfulPack(t *testing.T) {
	in := sampleInput()
	in.Validated = discovery.ValidatedOutputV1{DiscoveryState: discovery.StateNoMeaningfulThreads, Threads: []discovery.ValidatedThread{}}
	in.Bundle.Items = nil
	in.Bundle.ThreadEvidenceRelations = nil
	built, err := BuildPack(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(built.Payload.DiscoveryResult.Threads) != 0 {
		t.Fatal("threads")
	}
}

func TestRelationshipPolicy(t *testing.T) {
	pol := CurrentRelationshipPolicy()
	items := []evidenceItemView{{EvidenceID: "ev_001", EvidenceType: "SKU_SALES_HISTORY", TimeBearing: true}, {EvidenceID: "ev_002", EvidenceType: "SKU_PRICE_HISTORY", TimeBearing: true}}
	sales := FactProjection{Handle: "ef_1", SourceKind: "EVIDENCE", SourceRef: "ev_001"}
	price := FactProjection{Handle: "ef_2", SourceKind: "EVIDENCE", SourceRef: "ev_002"}
	disc := FactProjection{Handle: "f_1", SourceKind: "DISCOVERY", SourceRef: "f_1"}
	if err := AllowRelationship(BasisDescriptive, []FactProjection{disc}, items, pol); err != nil {
		t.Fatal(err)
	}
	if err := AllowRelationship(BasisCausalEstimate, []FactProjection{sales}, items, pol); err == nil {
		t.Fatal("causal")
	}
	if err := AllowRelationship(BasisPlausibleContributor, []FactProjection{sales, price}, items, pol); err != nil {
		t.Fatal(err)
	}
	if err := AllowRelationship(BasisPlausibleContributor, []FactProjection{sales}, items, pol); err == nil {
		t.Fatal("unapproved pair")
	}
	attrItems := []evidenceItemView{{EvidenceID: "ev_a", EvidenceType: "AD_ATTRIBUTION_DETAIL", TimeBearing: true}}
	attr := FactProjection{Handle: "ef_a", SourceKind: "EVIDENCE", SourceRef: "ev_a"}
	if err := AllowRelationship(BasisProviderAttributed, []FactProjection{attr}, attrItems, pol); err != nil {
		t.Fatal(err)
	}
	if err := AllowRelationship(BasisProviderAttributed, []FactProjection{sales}, items, pol); err == nil {
		t.Fatal("sales not provider attributed")
	}
	static := []evidenceItemView{{EvidenceID: "ev_s", EvidenceType: "SKU_STOCK_CONTEXT", TimeBearing: false}}
	st := FactProjection{Handle: "ef_s", SourceKind: "EVIDENCE", SourceRef: "ev_s"}
	if err := AllowRelationship(BasisTemporalAssociation, []FactProjection{st}, static, pol); err == nil {
		t.Fatal("static temporal")
	}
}

func TestValidatorThreadsFindingsActions(t *testing.T) {
	in := sampleInput()
	built, err := BuildPack(in)
	if err != nil {
		t.Fatal(err)
	}
	raw := FixtureJSON(built.PayloadBytes)
	var out OutputV1
	if err = json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatal(err)
	}
	rep := Validate(ValidateInput{Output: out, Payload: built.Payload, Snapshot: RebuildSnapshot(built.Payload), Budget: CurrentOutputBudget()})
	if !rep.Valid {
		t.Fatalf("%v", rep.Errors)
	}
	clone := func(o OutputV1) OutputV1 {
		b, _ := json.Marshal(o)
		var c OutputV1
		_ = json.Unmarshal(b, &c)
		return c
	}
	bad := clone(out)
	bad.ThreadResolutions = nil
	if Validate(ValidateInput{Output: bad, Payload: built.Payload, Snapshot: RebuildSnapshot(built.Payload), Budget: CurrentOutputBudget()}).Valid {
		t.Fatal("missing thread")
	}
	dup := clone(out)
	dup.ThreadResolutions = append(append([]ThreadResolution{}, dup.ThreadResolutions...), dup.ThreadResolutions[0])
	if Validate(ValidateInput{Output: dup, Payload: built.Payload, Snapshot: RebuildSnapshot(built.Payload), Budget: CurrentOutputBudget()}).Valid {
		t.Fatal("duplicate thread")
	}
	extra := clone(out)
	extra.ThreadResolutions[0].DiscoveryThreadRef = "T004"
	if Validate(ValidateInput{Output: extra, Payload: built.Payload, Snapshot: RebuildSnapshot(built.Payload), Budget: CurrentOutputBudget()}).Valid {
		t.Fatal("extra thread")
	}
	mon := clone(out)
	if len(mon.Recommendations) > 0 {
		mon.Recommendations[0].Actionability = "MONITOR"
		if len(mon.ActionProposals) > 0 {
			if Validate(ValidateInput{Output: mon, Payload: built.Payload, Snapshot: RebuildSnapshot(built.Payload), Budget: CurrentOutputBudget()}).Valid {
				t.Fatal("monitor action")
			}
		}
	}
	prev := clone(out)
	prev.PreviousActionAssessments = []PreviousActionAssessment{{PreviousActionRef: "a1", Assessment: "FULLY_SUPPORTED"}}
	pack := built.Payload
	pack.Continuity.PreviousActions = []PreviousAction{{PreviousActionRef: "a1"}, {PreviousActionRef: "a2"}, {PreviousActionRef: "a3"}}
	if Validate(ValidateInput{Output: prev, Payload: pack, Snapshot: RebuildSnapshot(pack), Budget: CurrentOutputBudget()}).Valid {
		t.Fatal("previous coverage 1!=3")
	}
	okPrev := clone(out)
	okPrev.PreviousActionAssessments = []PreviousActionAssessment{
		{PreviousActionRef: "a1", Assessment: "FULLY_SUPPORTED", AssessmentSummary: "ok", SupportAssessments: []PreviousSupportAssessment{}},
		{PreviousActionRef: "a2", Assessment: "NOT_SUPPORTED", AssessmentSummary: "ok", SupportAssessments: []PreviousSupportAssessment{}},
		{PreviousActionRef: "a3", Assessment: "UNVERIFIABLE", AssessmentSummary: "ok", SupportAssessments: []PreviousSupportAssessment{}},
	}
	if rep := Validate(ValidateInput{Output: okPrev, Payload: pack, Snapshot: RebuildSnapshot(pack), Budget: CurrentOutputBudget()}); !rep.Valid {
		t.Fatalf("3==3 should pass coverage %v", rep.Errors)
	}
}

func TestSchemaClosed(t *testing.T) {
	s := OutputJSONSchema()
	if s["additionalProperties"] != false {
		t.Fatal("root")
	}
	raw, _ := json.Marshal(s)
	txt := strings.ToLower(string(raw))
	for _, bad := range []string{"confidence", "severity", "priority", "expected_uplift", "forecast", "request_more_evidence"} {
		if strings.Contains(txt, bad) {
			t.Fatalf("schema contains %s", bad)
		}
	}
}

func TestRequestHashAndHTTPContract(t *testing.T) {
	req := Request{Model: "m1", Instructions: StableInstructionsV1, PayloadJSON: []byte(`{"x":1}`), Schema: OutputJSONSchema(), SchemaName: OutputSchemaNameV1, MaxOutputTokens: 100}
	h1 := HashRequest(req)
	req.Model = "m2"
	if HashRequest(req) == h1 {
		t.Fatal("model")
	}
	req.Model = "m1"
	req.ReasoningEffort = "high"
	if HashRequest(req) == h1 {
		t.Fatal("effort")
	}
	req.ReasoningEffort = ""
	req.Verbosity = "low"
	if HashRequest(req) == h1 {
		t.Fatal("verbosity")
	}
	wire := BuildDeepResponsesRequest(Request{Model: "deep-model", Instructions: StableInstructionsV1, PayloadJSON: []byte(`{"a":true}`), Schema: OutputJSONSchema(), SchemaName: OutputSchemaNameV1, MaxOutputTokens: 32768, ReasoningEffort: "medium", Verbosity: "low"})
	if wire.Store || wire.Truncation != "disabled" || wire.PreviousResponseID != nil || len(wire.Tools) != 0 {
		t.Fatalf("%+v", wire)
	}
	if wire.Text.Format.Name != OutputSchemaNameV1 || !wire.Text.Format.Strict {
		t.Fatal("schema")
	}
	if wire.Reasoning == nil || wire.Reasoning.Effort != "medium" || wire.Text.Verbosity != "low" {
		t.Fatal("optional fields")
	}
}

func TestAttempts(t *testing.T) {
	in := sampleInput()
	built, err := BuildPack(in)
	if err != nil {
		t.Fatal(err)
	}
	vin := ValidateInput{Payload: built.Payload, Snapshot: RebuildSnapshot(built.Payload), Budget: CurrentOutputBudget()}
	req := Request{Model: "m", Instructions: StableInstructionsV1, PayloadJSON: built.PayloadBytes, Schema: OutputJSONSchema(), SchemaName: OutputSchemaNameV1, MaxOutputTokens: 100}
	good := Response{Status: "completed", OutputText: FixtureJSON(built.PayloadBytes)}
	fp := &FakeProvider{Responses: []Response{good}}
	atts, _, err := RunAttempts(context.Background(), fp, req, vin, 2)
	if err != nil || len(atts) != 1 || fp.Calls != 1 {
		t.Fatalf("valid first %v n=%d calls=%d", err, len(atts), fp.Calls)
	}
	h := HashRequest(req)
	bad := Response{Status: "completed", OutputText: `{"executive_summary":{}}`}
	fp2 := &FakeProvider{Responses: []Response{bad, good}}
	atts, _, err = RunAttempts(context.Background(), fp2, req, vin, 2)
	if err != nil || len(atts) != 2 || atts[0].RequestHash != h || atts[1].RequestHash != h {
		t.Fatalf("rerun %v %#v", err, atts)
	}
	fp3 := &FakeProvider{Responses: []Response{bad, bad}}
	_, _, err = RunAttempts(context.Background(), fp3, req, vin, 2)
	if err == nil || fp3.Calls != 2 {
		t.Fatal("two invalid")
	}
	n := 0
	fp4 := &FakeProvider{Responses: []Response{bad, good}}
	_, _, err = RunAttempts(context.Background(), fp4, req, vin, 2, func(ctx context.Context, at ModelAttempt) error {
		n++
		if n == 1 {
			return context.Canceled
		}
		return nil
	})
	if err == nil || fp4.Calls != 1 {
		t.Fatalf("persist fail must not call 2: calls=%d err=%v", fp4.Calls, err)
	}
	fp5 := &FakeProvider{Responses: []Response{{Status: "completed", Refusal: "no"}}}
	_, _, err = RunAttempts(context.Background(), fp5, req, vin, 2)
	if err == nil {
		t.Fatal("refusal accepted")
	}
}

func TestPIIAndImportGuard(t *testing.T) {
	in := sampleInput()
	built, err := BuildPack(in)
	if err != nil {
		t.Fatal(err)
	}
	s := string(built.PayloadBytes) + string(built.PackBytes)
	for _, bad := range []string{"buyer_name", "authorization", "postgres://", "sk-live"} {
		if strings.Contains(strings.ToLower(s), bad) {
			t.Fatal(bad)
		}
	}
	err = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		txt := string(b)
		for _, bad := range []string{"integrations/ozon", "jackc/pgx", "dbgen", "unifiedanalysis", "aipipeline"} {
			if strings.Contains(txt, bad) {
				t.Errorf("%s %s", path, bad)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestAssignRefsDeterministic(t *testing.T) {
	out := OutputV1{Findings: []Finding{{Ordinal: 1}, {Ordinal: 2}}, Recommendations: []Recommendation{{Ordinal: 1}}}
	a := AssignBackendRefs(out)
	b := AssignBackendRefs(out)
	if a.FindingRefs[0] != "F001" || a.FindingRefs[1] != "F002" || a.RecommendationRefs[0] != "R001" {
		t.Fatal(a)
	}
	if a.FindingRefs[0] != b.FindingRefs[0] {
		t.Fatal("refs")
	}
}

func TestQualityFixturesExist(t *testing.T) {
	if !strings.Contains(QualityRubric(), "TEMPORAL") {
		t.Fatal("rubric")
	}
}

func TestWorstCaseTokenEstimate(t *testing.T) {
	in := sampleInput()
	var threads []discovery.ValidatedThread
	sku := in.Validated.Threads[0].PrimaryEntity
	for i := 1; i <= 10; i++ {
		threads = append(threads, discovery.ValidatedThread{
			ThreadID: fmt.Sprintf("T%03d", i), Ordinal: i, PrimaryEntity: sku,
			Observation: strings.Repeat("о", 40), InvestigationFocus: "focus", BasisFactHandles: []string{"f_0001"},
		})
	}
	in.Validated.Threads = threads
	items := make([]evidence.EvidenceItem, 0, 24)
	rels := []evidence.ThreadRequestRelation{}
	for i := 1; i <= 24; i++ {
		id := fmt.Sprintf("ev_%03d", i)
		items = append(items, evidence.EvidenceItem{
			EvidenceID: id, EvidenceType: "SKU_SALES_HISTORY", Subject: sku,
			Readiness: "READY", MaterializationStatus: evidence.StatusComplete,
			TemporalScope: evidence.TemporalScope{From: "2026-07-01", To: "2026-08-30"},
			FactHandles:   []string{"ef_0001"}, Data: json.RawMessage(`{"n":1}`),
		})
		rels = append(rels, evidence.ThreadRequestRelation{ThreadID: fmt.Sprintf("T%03d", (i%10)+1), EvidenceID: id})
	}
	in.Bundle.Items = items
	in.Bundle.ThreadEvidenceRelations = rels
	built, err := BuildPack(in)
	if err != nil {
		t.Fatal(err)
	}
	schemaTok := aitokens.EstimateBytesTokens(mustJSON(OutputJSONSchema()))
	t.Logf("deep_pack_bytes=%d estimated_tokens=%d schema_tokens=%d", len(built.PackBytes), built.EstimatedTokens, schemaTok)
	if built.EstimatedTokens > CurrentPackBudget().MaxInputTokens {
		t.Fatalf("over budget %d", built.EstimatedTokens)
	}
}

func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }

func TestCatalogMenuStill19(t *testing.T) {
	if len(actioncatalog.Catalog()) != 19 {
		t.Fatal("19")
	}
}
