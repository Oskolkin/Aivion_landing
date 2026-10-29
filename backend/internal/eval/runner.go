package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/observability"
)

type ArtifactPut func(ctx context.Context, key string, body []byte) (hash string, err error)

type Runner struct {
	Store    Store
	Put      ArtifactPut
	Executor CaseExecutor
	Live     bool
	Budget   Budget
}

type Budget struct {
	MaxRequests int
	Requests    int
}

func LiveEnabled() bool {
	return strings.TrimSpace(os.Getenv("OPENAI_LIVE_EVAL")) == "1" && strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) != ""
}

func LoadBudget() Budget {
	max := 20
	if v := strings.TrimSpace(os.Getenv("OPENAI_LIVE_EVAL_MAX_REQUESTS")); v != "" {
		if n, err := parseInt(v); err == nil && n > 0 {
			max = n
		}
	}
	return Budget{MaxRequests: max}
}

func (b *Budget) Consume() error {
	if b == nil || b.MaxRequests <= 0 {
		return nil
	}
	b.Requests++
	if b.Requests > b.MaxRequests {
		return fmt.Errorf(EvalBudgetExceeded)
	}
	return nil
}

func (r *Runner) RunSuite(ctx context.Context, kind, split, purpose, codeSHA string, _ map[string]map[string]any) (run Run, caseRuns []CaseRun, err error) {
	start := time.Now()
	if split == "" {
		split = SplitHoldout
	}
	if purpose == "" {
		purpose = PurposeAcceptance
	}
	suites := GoldenSuites()
	var suite Suite
	for _, s := range suites {
		if s.DatasetKind == kind || s.SuiteKey == kind {
			suite = s
			break
		}
	}
	if suite.SuiteKey == "" {
		return Run{}, nil, fmt.Errorf("unknown suite %s", kind)
	}
	if r.Store != nil {
		id, e := r.Store.UpsertSuite(ctx, suite)
		if e != nil {
			return Run{}, nil, e
		}
		suite.ID = id
	}
	run = Run{
		SuiteID: suite.ID, Split: split, Purpose: purpose, Status: "RUNNING", Verdict: VerdictPending,
		CodeSHA: codeSHA, ResolvedModels: ResolveConfiguredModels(),
		ChatPromptVersion: "chat_prompt_v1", ChatToolPolicyVersion: "chat_tool_policy_v1", ChatModelPolicyVersion: "chat_model_policy_v1",
		StartedAt: start,
	}
	if r.Store != nil {
		id, e := r.Store.InsertRun(ctx, run)
		if e != nil {
			return Run{}, nil, e
		}
		run.ID = id
	}
	finalize := func() {
		now := time.Now()
		run.CompletedAt = &now
		if run.Status == "RUNNING" {
			run.Status = "FAILED"
			if run.Verdict == VerdictPending {
				run.Verdict = VerdictFail
			}
		}
		if r.Store != nil {
			_ = r.Store.UpdateRun(context.Background(), run)
		}
		observability.ObserveEvalRun(suite.DatasetKind, split, run.Verdict, time.Since(start).Seconds(), run.CriticalFailuresCount, run.Aggregate.InputTokens, run.Aggregate.OutputTokens)
	}
	defer func() {
		if err != nil || run.Status == "RUNNING" {
			finalize()
		}
	}()

	if suite.DatasetKind == KindRealSellerReplay {
		run.Status = StatusDatasetPending
		run.Verdict = VerdictPassKnownLimitations
		run.Limitations = []KnownLimitation{{
			Limitation: "REAL_SELLER_REPLAY dataset is not available",
			Suite:      KindRealSellerReplay,
			Domain:     "replay",
			WhyNotCrit: "No anonymized production replay is checked in; suite is architecturally present and must not be marked PASS",
			Owner:      "eval",
			FollowUp:   "ingest anonymized seller replay before claiming REAL_SELLER_REPLAY PASS",
		}}
		if r.Store != nil {
			_ = r.Store.UpdateRun(ctx, run)
		}
		observability.ObserveEvalRun(suite.DatasetKind, split, run.Verdict, time.Since(start).Seconds(), 0, 0, 0)
		return run, caseRuns, nil
	}

	if r.Executor == nil {
		err = fmt.Errorf("eval case executor is required for purpose %s (DefaultActual is forbidden)", purpose)
		run.Status = "FAILED"
		run.Verdict = VerdictFail
		return run, nil, err
	}

	critical := 0
	ordinal := 0
	var inTok, outTok int64
	liveRequests := 0

	for _, sc := range BuiltInCases() {
		if !caseBelongs(suite.DatasetKind, sc.Case.CaseKey, sc.Case.Tags) {
			continue
		}
		if sc.Split != split {
			continue
		}
		ordinal++
		sc.Ordinal = ordinal
		sc.SuiteID = suite.ID
		if r.Store != nil {
			cid, e := r.Store.InsertCase(ctx, sc.Case)
			if e != nil {
				err = e
				run.Status = "FAILED"
				run.Verdict = VerdictFail
				return run, caseRuns, err
			}
			sc.CaseID = cid
			sc.Case.ID = cid
			if e = r.Store.AttachCase(ctx, sc); e != nil {
				err = e
				run.Status = "FAILED"
				run.Verdict = VerdictFail
				return run, caseRuns, err
			}
		}
		cr := CaseRun{EvalRunID: run.ID, CaseID: sc.Case.ID, Split: sc.Split, Status: "RUNNING"}
		opts := ExecuteOptions{Live: r.Live, Budget: &r.Budget, Purpose: purpose, Split: sc.Split}
		act, e := r.Executor.Execute(ctx, sc.Input, opts)
		if e != nil {
			cr.Status = "FAILED"
			cr.Verdict = VerdictFail
			cr.ErrorCode = e.Error()
			critical++
			caseRuns = append(caseRuns, cr)
			if r.Store != nil {
				id, ie := r.Store.InsertCaseRun(ctx, cr)
				if ie == nil {
					cr.ID = id
				}
			}
			continue
		}
		if sc.Split == SplitHoldout {
			if he := HoldoutExpectedMustNotEnterPrompt(sc.Split, act.ProviderPrompt, sc.Case.Expected); he != nil {
				e = he
				cr.Status = "FAILED"
				cr.Verdict = VerdictFail
				cr.ErrorCode = he.Error()
				critical++
				caseRuns = append(caseRuns, persistCase(ctx, r, cr, sc, suite, run.ID, act, GradeResult{CriticalFail: true, Code: "HOLDOUT_LEAK"})...)
				continue
			}
		}
		res := gradeExecuted(sc, act)
		liveRequests += act.ProviderRequests
		inTok += act.InputTokens
		outTok += act.OutputTokens
		cr.InputTokens = act.InputTokens
		cr.OutputTokens = act.OutputTokens
		cr.LatencyMS = act.LatencyMS
		cr.AnalysisRunID = act.AnalysisRunID
		if sc.Case.Expected.DatasetPending {
			cr.Status = StatusDatasetPending
			cr.Verdict = VerdictPending
		} else if res.CriticalFail {
			cr.Status = "FAILED"
			cr.Verdict = VerdictFail
			cr.ErrorCode = res.Code
			critical++
		} else {
			cr.Status = "PASSED"
			cr.Verdict = VerdictPass
		}
		if r.Put != nil {
			inBody, _ := json.Marshal(sc.Input)
			outBody, _ := json.Marshal(act)
			gradeIn, _ := json.Marshal(map[string]any{"expected": sc.Case.Expected, "actual": act.Values})
			gradeOut, _ := json.Marshal(res)
			rid := fmt.Sprintf("%d", run.ID)
			if h, pe := r.Put(ctx, CaseArtifactKey(suite.SuiteKey, suite.Version, rid, sc.Case.CaseKey, "input.json"), inBody); pe == nil {
				cr.ArtifactKey = CaseArtifactKey(suite.SuiteKey, suite.Version, rid, sc.Case.CaseKey, "output.json")
				_, _ = h, pe
			}
			if h, pe := r.Put(ctx, CaseArtifactKey(suite.SuiteKey, suite.Version, rid, sc.Case.CaseKey, "output.json"), outBody); pe == nil {
				cr.ArtifactHash = h
			}
			_, _ = r.Put(ctx, CaseArtifactKey(suite.SuiteKey, suite.Version, rid, sc.Case.CaseKey, "grader_input.json"), gradeIn)
			_, _ = r.Put(ctx, CaseArtifactKey(suite.SuiteKey, suite.Version, rid, sc.Case.CaseKey, "grader_result.json"), gradeOut)
			if len(act.Tools) > 0 {
				traj, _ := json.Marshal(act.Tools)
				_, _ = r.Put(ctx, CaseArtifactKey(suite.SuiteKey, suite.Version, rid, sc.Case.CaseKey, "trajectory.json"), traj)
			}
		}
		if r.Store != nil {
			id, ie := r.Store.InsertCaseRun(ctx, cr)
			if ie != nil {
				err = ie
				run.Status = "FAILED"
				run.Verdict = VerdictFail
				return run, caseRuns, err
			}
			cr.ID = id
			_ = r.Store.InsertGraderResult(ctx, GraderResult{
				CaseRunID: cr.ID, GraderKey: "deterministic", GraderVersion: DeterministicVersion,
				Score: map[string]any{"code": res.Code, "pass": res.Pass}, CriticalFail: res.CriticalFail,
				ArtifactKey: cr.ArtifactKey, ArtifactHash: cr.ArtifactHash,
			})
		}
		observability.ObserveEvalCase(suite.DatasetKind, sc.Split, cr.Status)
		caseRuns = append(caseRuns, cr)
	}

	if r.Live && liveRequests == 0 && r.Budget.Requests == 0 {
		err = fmt.Errorf("provider=openai produced 0 OpenAI requests")
		run.Status = "FAILED"
		run.Verdict = VerdictFail
		run.CriticalFailuresCount = critical + 1
		return run, caseRuns, err
	}

	run.CriticalFailuresCount = critical
	run.Aggregate.InputTokens = inTok
	run.Aggregate.OutputTokens = outTok
	acc := Decide(run, caseRuns, suite.DatasetKind)
	run.Status = acc.Status
	run.Verdict = acc.Verdict
	run.Limitations = acc.Limitations
	now := time.Now()
	run.CompletedAt = &now
	if r.Put != nil {
		rep := BuildReport(suite, run)
		body, _ := json.Marshal(rep)
		key := CaseArtifactKey(suite.SuiteKey, suite.Version, fmt.Sprintf("%d", run.ID), "_run", "report.json")
		if h, pe := r.Put(ctx, key, body); pe == nil {
			run.ReportArtifactKey = key
			run.ReportArtifactHash = h
		}
	}
	if r.Store != nil {
		if e := r.Store.UpdateRun(ctx, run); e != nil {
			err = e
			return run, caseRuns, err
		}
	}
	observability.ObserveEvalRun(suite.DatasetKind, split, run.Verdict, time.Since(start).Seconds(), critical, inTok, outTok)
	return run, caseRuns, nil
}

func persistCase(ctx context.Context, r *Runner, cr CaseRun, sc SuiteCase, suite Suite, runID int64, act CaseActual, res GradeResult) []CaseRun {
	if r.Store != nil {
		id, _ := r.Store.InsertCaseRun(ctx, cr)
		cr.ID = id
		_ = r.Store.InsertGraderResult(ctx, GraderResult{CaseRunID: cr.ID, GraderKey: "deterministic", GraderVersion: DeterministicVersion, Score: map[string]any{"code": res.Code}, CriticalFail: true})
	}
	_ = suite
	_ = runID
	_ = act
	_ = sc
	return []CaseRun{cr}
}

func gradeExecuted(sc SuiteCase, act CaseActual) GradeResult {
	res := GradeFactual(sc.Case, act.Values)
	if sc.Case.Expected.Materiality != nil {
		sel := map[string]bool{}
		if raw, ok := act.Values["selected"].(map[string]bool); ok {
			sel = raw
		}
		res = GradeMateriality(sc.Case.Expected.Materiality, sel)
	}
	if sc.Case.Expected.Discovery != nil {
		found := act.Threads
		if found == nil {
			found, _ = act.Values["threads"].([]string)
		}
		res = GradeDiscovery(sc.Case.Expected.Discovery, found, intFrom(act.Values["duplicates"]), intFrom(act.Values["unsupported_critical"]))
	}
	if len(sc.Case.Expected.DeepTraps) > 0 {
		out := act.Output
		if out == "" {
			out = strFrom(act.Values["output"])
		}
		res = GradeDeepTraps(sc.Case.Expected.DeepTraps, out)
	}
	if spec := sc.Case.Expected.Trajectory; spec != nil {
		tools := act.Tools
		if tools == nil {
			tools, _ = act.Values["tools"].([]string)
		}
		tres := GradeTrajectory(spec, tools)
		if tres.CriticalFail {
			res = tres
		}
	}
	if sc.Case.Expected.ChatScript != nil && sc.Case.Expected.ChatScript.RequireClarification {
		if !strings.Contains(strings.ToLower(act.Output), "уточните") && !strings.Contains(strings.ToLower(act.Output), "clarif") {
			return GradeResult{CriticalFail: true, Code: "CLARIFICATION_REQUIRED"}
		}
	}
	numericOK := !res.CriticalFail
	if !numericOK {
		return res
	}
	return res
}

func caseBelongs(kind, caseKey string, tags []string) bool {
	switch kind {
	case KindFactualGolden:
		return containsAny(tags, "sales", "drr", "stock", "funnel", "reconciliation", "null", "seller_isolation") ||
			strings.HasPrefix(caseKey, "sales_") || strings.HasPrefix(caseKey, "drr_") || strings.HasPrefix(caseKey, "stock_") || strings.HasPrefix(caseKey, "funnel_") || strings.HasPrefix(caseKey, "reconciliation")
	case KindCapabilityMatrix:
		return containsAny(tags, "capability")
	case KindAIGolden:
		return containsAny(tags, "trap", "deep")
	case KindLongitudinal:
		return strings.HasPrefix(caseKey, "continuity_")
	case KindChatGolden:
		return strings.HasPrefix(caseKey, "chat_")
	case KindAttachmentGolden:
		return strings.HasPrefix(caseKey, "attachment_")
	case KindRealSellerReplay:
		return strings.HasPrefix(caseKey, "real_seller_")
	default:
		return true
	}
}

func containsAny(tags []string, keys ...string) bool {
	for _, t := range tags {
		for _, k := range keys {
			if t == k {
				return true
			}
		}
	}
	return false
}

func intFrom(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	default:
		return 0
	}
}

func strFrom(v any) string {
	s, _ := v.(string)
	return s
}

func parseInt(s string) (int, error) {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("not int")
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

// DefaultActual is a unit-test helper only. Runner must never call it for ACCEPTANCE/HOLDOUT/LIVE/BASELINE.
func DefaultActual(c Case) map[string]any {
	m := map[string]any{}
	if c.Expected.Sales != nil {
		m["ordered_amount"] = c.Expected.Sales.OrderedAmount
		m["ordered_units"] = c.Expected.Sales.OrderedUnits
	}
	return m
}
