package eval

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func testRunner() *Runner {
	arts := NewMemoryArtifacts()
	return &Runner{Executor: NewFrozenFixtureExecutor(), Store: NewMemoryStore(), Put: arts.Put}
}

func TestPrompt16FactualGolden(t *testing.T) {
	run, cases, err := testRunner().RunSuite(context.Background(), KindFactualGolden, SplitHoldout, PurposeAcceptance, "sha", nil)
	if err != nil {
		t.Fatal(err)
	}
	if run.CriticalFailuresCount != 0 || run.Verdict != VerdictPass {
		t.Fatalf("factual golden verdict=%s critical=%d cases=%d", run.Verdict, run.CriticalFailuresCount, len(cases))
	}
	if run.Status == "RUNNING" {
		t.Fatal("run left RUNNING")
	}
}

func TestPrompt16AcceptanceCannotUseDefaultActual(t *testing.T) {
	r := &Runner{}
	_, _, err := r.RunSuite(context.Background(), KindFactualGolden, SplitHoldout, PurposeAcceptance, "sha", nil)
	if err == nil || !strings.Contains(err.Error(), "executor") {
		t.Fatalf("want executor required, got %v", err)
	}
}

func TestPrompt16FixtureMismatchFailsFactualGolden(t *testing.T) {
	in := CaseInput{CaseKey: "sales_two_day_sum", Fixture: Fixture{SalesDays: []SalesDay{
		{OrderedAmount: "100", OrderedUnits: "2"},
		{OrderedAmount: "199", OrderedUnits: "3"},
	}}}
	act, err := NewFrozenFixtureExecutor().Execute(context.Background(), in, ExecuteOptions{})
	if err != nil {
		t.Fatal(err)
	}
	c := Case{Expected: Expected{Sales: &SalesExpectation{OrderedAmount: "300", OrderedUnits: "5"}}}
	got := GradeFactual(c, act.Values)
	if !got.CriticalFail {
		t.Fatalf("fixture 299 vs expected 300 must fail, got %+v actual=%v", got, act.Values)
	}
}

func TestPrompt16DRRGolden(t *testing.T) {
	c := Case{CaseKey: "drr", Expected: Expected{DRR: &DRRExpectation{AccountDRRPct: "20", ForbidAvgDaily: true, ForbidAttributedDenom: true}}}
	ok := GradeFactual(c, map[string]any{"account_drr_pct": "20", "used_avg_daily_drr": false, "used_attributed_ordered_amount_denom": false})
	if !ok.Pass {
		t.Fatal(ok)
	}
	if !GradeFactual(c, map[string]any{"account_drr_pct": "20", "used_avg_daily_drr": true}).CriticalFail {
		t.Fatal("avg daily DRR must fail")
	}
	if !GradeFactual(c, map[string]any{"account_drr_pct": "20", "used_attributed_ordered_amount_denom": true}).CriticalFail {
		t.Fatal("attributed denom must fail")
	}
}

func TestPrompt16StockGolden(t *testing.T) {
	c := Case{Expected: Expected{Stock: &StockExpectation{StockZeroDemandPositiveCoverageZero: true, NoDemandCoverageNull: true, StockUnavailableCoverageNull: true, InboundDoesNotIncreaseStock: true}}}
	got := GradeFactual(c, map[string]any{"coverage_oos_demand": "0", "coverage_no_demand": "NO_DEMAND", "coverage_stock_unavailable": "NULL", "inbound_increased_stock": false})
	if !got.Pass {
		t.Fatalf("%+v", got)
	}
}

func TestPrompt16FunnelGolden(t *testing.T) {
	c := Case{Expected: Expected{Funnel: &FunnelExpectation{MissingStageNotInvented: true, IncompatibleChain: "NO_COMPATIBLE_CHAIN", PremiumUnavailable: "UNAVAILABLE", PartialChain: "PARTIAL", ValidZeroActivity: "READY_EMPTY", ConversionDeltaPP: "-2.8"}}}
	got := GradeFactual(c, map[string]any{"missing_stage_invented": false, "incompatible_chain": "NO_COMPATIBLE_CHAIN", "premium_unavailable": "UNAVAILABLE", "partial_chain": "PARTIAL", "valid_zero_activity": "READY_EMPTY", "conversion_delta_pp": "-2.8"})
	if !got.Pass {
		t.Fatalf("%+v", got)
	}
}

func TestPrompt16MaterialityRecall(t *testing.T) {
	labels := []MaterialLabel{{Kind: "SKU", Key: "a", Label: LabelMustInclude}, {Kind: "SKU", Key: "b", Label: LabelShouldInclude}}
	got := GradeMateriality(labels, map[string]bool{"SKU:a": true, "SKU:b": true})
	if got.Metrics["must_include_recall"] != 1 || got.Metrics["overall_recall"] != 1 {
		t.Fatalf("%v", got.Metrics)
	}
}

func TestPrompt16DiscoveryGolden(t *testing.T) {
	exp := &DiscoveryExpectation{AllowNoMeaningful: true}
	got := GradeDiscovery(exp, nil, 0, 0)
	if !got.Pass {
		t.Fatal(got)
	}
	exp = &DiscoveryExpectation{Threads: []ThreadLabel{{ID: "critical:oos", Class: ThreadMandatory}, {ID: "bad", Class: ThreadForbidden}}}
	if !GradeDiscovery(exp, []string{"bad"}, 0, 0).CriticalFail {
		t.Fatal("forbidden thread")
	}
}

func TestPrompt16DeepCriticalSemantics(t *testing.T) {
	if !GradeDeepTraps([]string{"ordered_amount_not_revenue"}, "Это revenue магазина").CriticalFail {
		t.Fatal("revenue trap")
	}
	if !GradeDeepTraps([]string{"ordered_amount_not_revenue"}, "Заказано на сумму 300").Pass {
		t.Fatal("safe wording")
	}
}

func TestPrompt16ActionSemanticConstraints(t *testing.T) {
	got := GradeActions(9, 10, 0)
	if !got.Pass {
		t.Fatal(got)
	}
	if !GradeActions(0, 0, 1).CriticalFail {
		t.Fatal("harmful")
	}
}

func TestPrompt16HoldoutExpectedCannotEnterProviderInput(t *testing.T) {
	rec := &RecordingExecutor{Inner: NewFrozenFixtureExecutor()}
	var sc SuiteCase
	for _, c := range BuiltInCases() {
		if c.Case.CaseKey == "sales_two_day_sum" {
			sc = c
			break
		}
	}
	if sc.Case.Expected.HoldoutSentinel != HoldoutSentinel {
		t.Fatal("sentinel must live on Expected only")
	}
	act, err := rec.Execute(context.Background(), sc.Input, ExecuteOptions{Split: SplitHoldout})
	if err != nil {
		t.Fatal(err)
	}
	exp, _ := json.Marshal(sc.Case.Expected)
	if !strings.Contains(string(exp), HoldoutSentinel) {
		t.Fatal("expected json must contain sentinel")
	}
	if strings.Contains(string(rec.LastInputJSON), HoldoutSentinel) || strings.Contains(act.ProviderPrompt, HoldoutSentinel) {
		t.Fatal("provider input contained holdout sentinel")
	}
	if err := HoldoutExpectedMustNotEnterPrompt(SplitHoldout, act.ProviderPrompt, sc.Case.Expected); err != nil {
		t.Fatal(err)
	}
}

func TestPrompt16DevHoldoutIsolation(t *testing.T) {
	dev, _, err := testRunner().RunSuite(context.Background(), KindCapabilityMatrix, SplitDEV, PurposeAcceptance, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	hold, _, err := testRunner().RunSuite(context.Background(), KindCapabilityMatrix, SplitHoldout, PurposeAcceptance, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if dev.Split == hold.Split {
		t.Fatal("splits must stay distinct")
	}
}

func TestPrompt16BaselineComparison(t *testing.T) {
	a := Run{Verdict: VerdictPass, CriticalFailuresCount: 0}
	b := Run{Verdict: VerdictFail, CriticalFailuresCount: 1}
	cmp := CompareRuns(a, b)
	if !cmp.CriticalRegression {
		t.Fatal("expected critical regression")
	}
}

func TestPrompt16RealSellerReplayNotPass(t *testing.T) {
	run, _, err := testRunner().RunSuite(context.Background(), KindRealSellerReplay, SplitReplay, PurposeAcceptance, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if run.Verdict == VerdictPass || run.Status != StatusDatasetPending {
		t.Fatalf("replay must be DATASET_PENDING, got %s %s", run.Status, run.Verdict)
	}
}

func TestPrompt16SecretScan(t *testing.T) {
	if hits := ScanSecrets("log.txt", []byte("ok")); len(hits) != 0 {
		t.Fatal(hits)
	}
	if hits := ScanSecrets("x", []byte("Authorization: Bearer abcdef")); len(hits) == 0 {
		t.Fatal("expected bearer hit")
	}
}

func TestPrompt16HumanReviewNotAutoPass(t *testing.T) {
	run := Run{Verdict: VerdictPass, Split: SplitHoldout, Status: StatusHumanReviewPending}
	if err := CanPromote(run, nil); err == nil {
		t.Fatal("must not auto-promote without human review")
	}
	pkt := HumanReviewPacket(Case{CaseKey: "x"}, "facts", "base", "cand", map[string]any{"ok": true})
	if !strings.Contains(string(pkt), "must not write human_review=PASS") {
		t.Fatal(string(pkt))
	}
}

func TestPrompt16HumanReviewPersists(t *testing.T) {
	st := NewMemoryStore()
	id, err := st.InsertRun(context.Background(), Run{Status: "PASSED", Verdict: VerdictPass})
	if err != nil {
		t.Fatal(err)
	}
	crID, err := st.InsertCaseRun(context.Background(), CaseRun{EvalRunID: id, Status: "PASSED", Verdict: VerdictPass})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RecordHumanReview(context.Background(), st, HumanReview{CaseRunID: crID, Reviewer: "codex", Status: "PASS"}); err == nil {
		t.Fatal("agent PASS must be rejected")
	}
	h, err := RecordHumanReview(context.Background(), st, HumanReview{CaseRunID: crID, Reviewer: "analyst", Status: "PASS", Notes: "ok"})
	if err != nil {
		t.Fatal(err)
	}
	revs, err := st.ListHumanReviews(context.Background(), id)
	if err != nil || len(revs) != 1 {
		t.Fatalf("%v %v", revs, err)
	}
	_ = h
}

func TestPrompt16BaselinePromotionGuards(t *testing.T) {
	st := NewMemoryStore()
	run := Run{SuiteID: 1, Split: SplitHoldout, Status: "PASSED", Verdict: VerdictPass, CriticalFailuresCount: 0}
	id, _ := st.InsertRun(context.Background(), run)
	run.ID = id
	if err := PromoteIfGuarded(context.Background(), st, run, "analyst"); err == nil {
		t.Fatal("holdout promote requires human PASS")
	}
	crID, _ := st.InsertCaseRun(context.Background(), CaseRun{EvalRunID: id, Status: "PASSED"})
	_ = st.InsertHumanReview(context.Background(), HumanReview{CaseRunID: crID, Reviewer: "analyst", Status: "PASS"})
	if err := PromoteIfGuarded(context.Background(), st, run, "analyst"); err != nil {
		t.Fatal(err)
	}
	got, ok, err := st.CurrentBaseline(context.Background(), 1, SplitHoldout)
	if err != nil || !ok || got != id {
		t.Fatalf("baseline %d ok=%v err=%v", got, ok, err)
	}
}

func TestPrompt16ChatGoldenFake(t *testing.T) {
	run, cases, err := testRunner().RunSuite(context.Background(), KindChatGolden, SplitHoldout, PurposeAcceptance, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatal("expected chat cases")
	}
	if run.Status != StatusHumanReviewPending {
		t.Fatalf("chat golden requires human review, got %s", run.Status)
	}
}

func TestPrompt16RunnerPersistsCaseRunsAndArtifacts(t *testing.T) {
	st := NewMemoryStore()
	arts := NewMemoryArtifacts()
	r := &Runner{Executor: NewFrozenFixtureExecutor(), Store: st, Put: arts.Put}
	run, cases, err := r.RunSuite(context.Background(), KindFactualGolden, SplitHoldout, PurposeAcceptance, "sha", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 || st.CountGraders() == 0 {
		t.Fatalf("cases=%d graders=%d", len(cases), st.CountGraders())
	}
	crs, err := st.ListCaseRuns(context.Background(), run.ID)
	if err != nil || len(crs) == 0 {
		t.Fatal(crs, err)
	}
	if run.ReportArtifactKey == "" || run.ReportArtifactHash == "" {
		t.Fatal("missing report artifact")
	}
	if _, ok := arts.Data[run.ReportArtifactKey]; !ok {
		t.Fatal("report bytes missing")
	}
}

func TestPrompt16RunnerFinalizesEvalRun(t *testing.T) {
	r := &Runner{Executor: errExecutor{}, Store: NewMemoryStore()}
	run, _, err := r.RunSuite(context.Background(), KindFactualGolden, SplitHoldout, PurposeAcceptance, "sha", nil)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status == "RUNNING" {
		t.Fatal("must finalize")
	}
	if run.Status != "FAILED" {
		t.Fatalf("status=%s", run.Status)
	}
}

type errExecutor struct{}

func (errExecutor) Execute(context.Context, CaseInput, ExecuteOptions) (CaseActual, error) {
	return CaseActual{}, context.Canceled
}

func TestPrompt16LiveZeroRequestsFails(t *testing.T) {
	r := &Runner{Executor: NewFrozenFixtureExecutor(), Live: true, Budget: Budget{MaxRequests: 5}}
	_, _, err := r.RunSuite(context.Background(), KindFactualGolden, SplitHoldout, PurposeLive, "sha", nil)
	if err == nil || !strings.Contains(err.Error(), "0 OpenAI") {
		t.Fatalf("want zero-request fail, got %v", err)
	}
}

func TestPrompt16EvalSchemaAndArtifactIntegrity(t *testing.T) {
	if !strings.HasPrefix(ArtifactKey(KindFactualGolden, "c.json"), "eval/factual-golden/") {
		t.Fatal(ArtifactKey(KindFactualGolden, "c.json"))
	}
	key := CaseArtifactKey("FACTUAL_GOLDEN", "v1", "9", "sales_two_day_sum", "input.json")
	if key != "eval/FACTUAL_GOLDEN/v1/9/sales_two_day_sum/input.json" {
		t.Fatal(key)
	}
}

func TestPrompt16BudgetGuard(t *testing.T) {
	b := Budget{MaxRequests: 1}
	if err := b.Consume(); err != nil {
		t.Fatal(err)
	}
	if err := b.Consume(); err == nil {
		t.Fatal("expected budget exceeded")
	}
}

func TestPrompt16LLMGraderCannotOverrideFactRef(t *testing.T) {
	got := GradeSemantic(Case{}, "x", SemanticScores{Groundedness: 1}, false)
	if !got.CriticalFail || got.Code != "FACTREF_INVALID" {
		t.Fatalf("%+v", got)
	}
	g := LLMGrader{}
	res, err := g.GradeContext(context.Background(), LLMGradeInput{Candidate: "text", NumericFactRefsValid: false})
	if err != nil || !res.CriticalFail {
		t.Fatalf("%v %+v", err, res)
	}
}

func TestPrompt16LLMGraderRequiresClient(t *testing.T) {
	_, err := (LLMGrader{}).Grade([]byte("not empty"))
	if err == nil {
		t.Fatal("always-pass grader is forbidden")
	}
}
