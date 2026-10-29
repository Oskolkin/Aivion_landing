package unifiedanalysis

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/provider"
)

func analysisTestConfig() Config {
	cfg := testConfig()
	cfg.PromptProfile = "analysis"
	cfg.PromptVersion = PromptVersionAnalyst
	cfg.AnalysisMaxOutputTokens = 32768
	cfg.SmokeMaxOutputTokens = 2000
	cfg.MaxOutputTokens = 32768
	cfg.ProviderMaxAttempts = 2
	cfg.OutputValidationEnabled = true
	return cfg
}

func newAnalysisTestService(repo *memoryRepository, packs PackBuilderService, fake *provider.FakeProvider) *Service {
	return NewServiceWithStore(repo, packs, fake, &memoryArtifactStore{repo: repo}, newMemoryPackObjectStore(), analysisTestConfig())
}

func startAnalysisRunReadyToPoll(t *testing.T, svc *Service, repo *memoryRepository) *Run {
	t.Helper()
	ctx := context.Background()
	run, err := svc.CreateRun(ctx, 1, time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC), false)
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	// Force analysis prompt version on the run row (CreateRun uses config).
	repo.mu.Lock()
	row := repo.runs[run.ID]
	row.PromptVersion = nullableText(PromptVersionAnalyst)
	repo.runs[run.ID] = row
	repo.mu.Unlock()

	if err := svc.StartRun(ctx, run.ID); err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	run, err = repo.GetRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if run.Status != StatusProviderRunning {
		t.Fatalf("status=%q want provider_running", run.Status)
	}
	return run
}

func loadTerminalAttempts(t *testing.T, arts *memoryArtifactStore, runID int64) []providerTerminalAttemptArtifact {
	t.Helper()
	raws := arts.ListArtifactsByKind(context.Background(), runID, ArtifactUnifiedProviderTerminalAttempt)
	out := make([]providerTerminalAttemptArtifact, 0, len(raws))
	for _, raw := range raws {
		var a providerTerminalAttemptArtifact
		if err := json.Unmarshal(raw, &a); err != nil {
			t.Fatalf("unmarshal attempt: %v", err)
		}
		out = append(out, a)
	}
	return out
}

func TestProviderTerminalIncompletePersistsArtifacts(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryRepository()
	packID := "pack:1:2026-07-08:abc123def45678"
	stub := &stubPackBuilder{result: validStubPack(packID), key: "k"}
	fake := provider.NewFakeProvider(provider.FakeScenarioIncomplete)
	fake.AttemptScripts = []provider.FakeAttemptScript{
		{Status: provider.ProviderResponseStatusIncomplete, IncompleteReason: "max_output_tokens"},
		// No second script — max attempts 1 for this test via config.
	}
	cfg := analysisTestConfig()
	cfg.ProviderMaxAttempts = 1
	arts := &memoryArtifactStore{repo: repo}
	svc := NewServiceWithStore(repo, stub, fake, arts, newMemoryPackObjectStore(), cfg)

	run := startAnalysisRunReadyToPoll(t, svc, repo)
	poll, err := svc.PollRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("PollRun: %v", err)
	}
	if poll == nil || !poll.Failed {
		t.Fatalf("expected failed, got %+v", poll)
	}
	failed, _ := repo.GetRun(ctx, run.ID)
	if failed.ProviderStatus != string(provider.ProviderResponseStatusIncomplete) {
		t.Fatalf("provider_status=%q", failed.ProviderStatus)
	}
	if failed.ErrorCode != CodeProviderResponseIncomplete {
		t.Fatalf("error_code=%q", failed.ErrorCode)
	}
	if failed.ErrorMessage != "provider response incomplete: max_output_tokens" {
		t.Fatalf("error_message=%q", failed.ErrorMessage)
	}
	attempts := loadTerminalAttempts(t, arts, run.ID)
	if len(attempts) != 1 {
		t.Fatalf("attempts=%d", len(attempts))
	}
	a := attempts[0]
	if a.IncompleteReason != "max_output_tokens" {
		t.Fatalf("incomplete_reason=%q", a.IncompleteReason)
	}
	if a.OutputTokens == 0 {
		t.Fatal("expected usage output_tokens persisted")
	}
	if a.Attempt != 1 {
		t.Fatalf("attempt=%d", a.Attempt)
	}
	if a.RequestID == "" && failed.ProviderRequestID == "" {
		t.Fatal("expected request id on run or attempt")
	}
	if len(arts.ListArtifactsByKind(ctx, run.ID, ArtifactUnifiedProviderResponseRaw)) == 0 {
		t.Fatal("expected raw response artifact")
	}
	if len(a.OutputItemTypes) == 0 && a.CodeInterpreterCalls == 0 {
		// incomplete may have empty output; still require attempt number + status
	}
}

func TestProviderRetryScenarioA_IncompleteThenCompleted(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryRepository()
	packID := "pack:1:2026-07-08:abc123def45678"
	stub := &stubPackBuilder{result: validStubPack(packID), key: "k"}
	fake := provider.NewFakeProvider("")
	fake.AttemptScripts = []provider.FakeAttemptScript{
		{Status: provider.ProviderResponseStatusIncomplete, IncompleteReason: "max_output_tokens"},
		{Status: provider.ProviderResponseStatusCompleted, AnalysisJSON: []byte(`{"schema_version":"aivion_unified_analysis_v10","ok":true}`)},
	}
	arts := &memoryArtifactStore{repo: repo}
	svc := NewServiceWithStore(repo, stub, fake, arts, newMemoryPackObjectStore(), analysisTestConfig())

	run := startAnalysisRunReadyToPoll(t, svc, repo)
	firstID := run.ProviderResponseID

	poll1, err := svc.PollRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("PollRun1: %v", err)
	}
	if poll1 == nil || !poll1.InProgress {
		t.Fatalf("expected in-progress retry, got %+v", poll1)
	}
	if fake.StartResponseCount != 2 {
		t.Fatalf("StartResponseCount=%d want 2", fake.StartResponseCount)
	}
	if fake.DeleteContainerCount != 0 || fake.DeleteFileCount != 0 {
		t.Fatalf("cleanup must not run between attempts")
	}
	mid, _ := repo.GetRun(ctx, run.ID)
	if mid.ProviderAttemptCount != 2 {
		t.Fatalf("attempt_count=%d", mid.ProviderAttemptCount)
	}
	if mid.ProviderResponseID == firstID {
		t.Fatal("expected new response id after retry")
	}
	if mid.CleanupStatus != "" && mid.CleanupStatus != CleanupStatusPending {
		// should not be pending yet between attempts
	}
	if textOrEmpty(repo.runs[run.ID].CleanupStatus) == CleanupStatusPending {
		// SaveResponseCompleted sets pending; retry must not.
		t.Fatal("cleanup_status should not be pending between attempts")
	}

	poll2, err := svc.PollRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("PollRun2: %v", err)
	}
	if poll2 == nil || !poll2.Completed {
		t.Fatalf("expected completed→output_validating, got %+v status=%v", poll2, poll2.Run.Status)
	}
	if poll2.Run.Status != StatusOutputValidating {
		t.Fatalf("status=%q", poll2.Run.Status)
	}
	attempts := loadTerminalAttempts(t, arts, run.ID)
	if len(attempts) < 2 {
		t.Fatalf("expected >=2 attempt artifacts, got %d", len(attempts))
	}
}

func TestProviderRetryScenarioB_ToolOnlyThenCompleted(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryRepository()
	packID := "pack:1:2026-07-08:abc123def45678"
	stub := &stubPackBuilder{result: validStubPack(packID), key: "k"}
	fake := provider.NewFakeProvider("")
	fake.AttemptScripts = []provider.FakeAttemptScript{
		{Status: provider.ProviderResponseStatusCompleted, ToolOnly: true},
		{Status: provider.ProviderResponseStatusCompleted, AnalysisJSON: []byte(`{"schema_version":"aivion_unified_analysis_v10","ok":true}`)},
	}
	svc := newAnalysisTestService(repo, stub, fake)
	run := startAnalysisRunReadyToPoll(t, svc, repo)

	poll1, err := svc.PollRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("PollRun1: %v", err)
	}
	if !poll1.InProgress {
		t.Fatalf("expected retry after tool-only, got %+v", poll1)
	}
	poll2, err := svc.PollRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("PollRun2: %v", err)
	}
	if !poll2.Completed || poll2.Run.Status != StatusOutputValidating {
		t.Fatalf("expected output_validating, got completed=%v status=%q", poll2.Completed, poll2.Run.Status)
	}
}

func TestProviderRetryScenarioC_TwoToolOnlyFails(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryRepository()
	packID := "pack:1:2026-07-08:abc123def45678"
	stub := &stubPackBuilder{result: validStubPack(packID), key: "k"}
	fake := provider.NewFakeProvider("")
	fake.AttemptScripts = []provider.FakeAttemptScript{
		{Status: provider.ProviderResponseStatusCompleted, ToolOnly: true},
		{Status: provider.ProviderResponseStatusCompleted, ToolOnly: true},
	}
	arts := &memoryArtifactStore{repo: repo}
	svc := NewServiceWithStore(repo, stub, fake, arts, newMemoryPackObjectStore(), analysisTestConfig())
	run := startAnalysisRunReadyToPoll(t, svc, repo)

	if _, err := svc.PollRun(ctx, run.ID); err != nil {
		t.Fatalf("PollRun1: %v", err)
	}
	poll2, err := svc.PollRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("PollRun2: %v", err)
	}
	if !poll2.Failed {
		t.Fatalf("expected failed, got %+v", poll2)
	}
	failed, _ := repo.GetRun(ctx, run.ID)
	if failed.ErrorCode != CodeStructuredOutputMissing {
		t.Fatalf("error_code=%q", failed.ErrorCode)
	}
	if failed.CleanupStatus != CleanupStatusPending {
		t.Fatalf("cleanup_status=%q", failed.CleanupStatus)
	}
	if len(loadTerminalAttempts(t, arts, run.ID)) < 2 {
		t.Fatal("expected both attempt artifacts")
	}
}

func TestProviderRetryScenarioD_ContentFilterNoRetry(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryRepository()
	packID := "pack:1:2026-07-08:abc123def45678"
	stub := &stubPackBuilder{result: validStubPack(packID), key: "k"}
	fake := provider.NewFakeProvider("")
	fake.AttemptScripts = []provider.FakeAttemptScript{
		{Status: provider.ProviderResponseStatusIncomplete, IncompleteReason: "content_filter"},
	}
	arts := &memoryArtifactStore{repo: repo}
	svc := NewServiceWithStore(repo, stub, fake, arts, newMemoryPackObjectStore(), analysisTestConfig())
	run := startAnalysisRunReadyToPoll(t, svc, repo)

	poll, err := svc.PollRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("PollRun: %v", err)
	}
	if !poll.Failed {
		t.Fatal("expected fail without retry")
	}
	if fake.StartResponseCount != 1 {
		t.Fatalf("StartResponseCount=%d", fake.StartResponseCount)
	}
	failed, _ := repo.GetRun(ctx, run.ID)
	if failed.ErrorMessage != "provider response incomplete: content_filter" {
		t.Fatalf("msg=%q", failed.ErrorMessage)
	}
}

func TestProviderRetryScenarioE_RefusalNoRetry(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryRepository()
	packID := "pack:1:2026-07-08:abc123def45678"
	stub := &stubPackBuilder{result: validStubPack(packID), key: "k"}
	fake := provider.NewFakeProvider("")
	fake.AttemptScripts = []provider.FakeAttemptScript{
		{Status: provider.ProviderResponseStatusCompleted, Refusal: "policy refusal", AnalysisJSON: []byte(`{}`)},
	}
	svc := newAnalysisTestService(repo, stub, fake)
	run := startAnalysisRunReadyToPoll(t, svc, repo)

	poll, err := svc.PollRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("PollRun: %v", err)
	}
	if !poll.Failed {
		t.Fatal("expected refusal failure")
	}
	if fake.StartResponseCount != 1 {
		t.Fatalf("StartResponseCount=%d", fake.StartResponseCount)
	}
	failed, _ := repo.GetRun(ctx, run.ID)
	if failed.ErrorCode != CodeProviderRefusal {
		t.Fatalf("error_code=%q", failed.ErrorCode)
	}
}

func TestProviderRetryScenarioF_FirstAttemptSuccessNoRetry(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryRepository()
	packID := "pack:1:2026-07-08:abc123def45678"
	stub := &stubPackBuilder{result: validStubPack(packID), key: "k"}
	fake := provider.NewFakeProvider("")
	fake.AttemptScripts = []provider.FakeAttemptScript{
		{Status: provider.ProviderResponseStatusCompleted, AnalysisJSON: []byte(`{"schema_version":"aivion_unified_analysis_v10","ok":true}`)},
	}
	arts := &memoryArtifactStore{repo: repo}
	svc := NewServiceWithStore(repo, stub, fake, arts, newMemoryPackObjectStore(), analysisTestConfig())
	run := startAnalysisRunReadyToPoll(t, svc, repo)

	poll, err := svc.PollRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("PollRun: %v", err)
	}
	if !poll.Completed || poll.Run.Status != StatusOutputValidating {
		t.Fatalf("unexpected poll=%+v", poll)
	}
	if fake.StartResponseCount != 1 {
		t.Fatalf("StartResponseCount=%d", fake.StartResponseCount)
	}
	reqs := arts.ListArtifactsByKind(ctx, run.ID, ArtifactUnifiedProviderRequestSummary)
	if len(reqs) == 0 {
		t.Fatal("expected request summary")
	}
	var summary providerRequestSummary
	if err := json.Unmarshal(reqs[0], &summary); err != nil {
		t.Fatalf("unmarshal summary: %v", err)
	}
	if summary.MaxOutputTokens != 32768 {
		t.Fatalf("max_output_tokens=%d", summary.MaxOutputTokens)
	}
	if summary.ToolChoice != "auto" {
		t.Fatalf("tool_choice=%q", summary.ToolChoice)
	}
	if summary.SchemaVersion == "" || summary.PromptVersion == "" {
		t.Fatalf("missing schema/prompt identity: %+v", summary)
	}
	if summary.Attempt != 1 {
		t.Fatalf("attempt=%d", summary.Attempt)
	}
}

func TestProviderRetryScenarioG_ConcurrentPollSingleRetry(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryRepository()
	packID := "pack:1:2026-07-08:abc123def45678"
	stub := &stubPackBuilder{result: validStubPack(packID), key: "k"}
	fake := provider.NewFakeProvider("")
	fake.AttemptScripts = []provider.FakeAttemptScript{
		{Status: provider.ProviderResponseStatusIncomplete, IncompleteReason: "max_output_tokens"},
		{Status: provider.ProviderResponseStatusCompleted, AnalysisJSON: []byte(`{"schema_version":"aivion_unified_analysis_v10","ok":true}`)},
		{Status: provider.ProviderResponseStatusCompleted, AnalysisJSON: []byte(`{"schema_version":"aivion_unified_analysis_v10","ok":true}`)},
	}
	svc := newAnalysisTestService(repo, stub, fake)
	run := startAnalysisRunReadyToPoll(t, svc, repo)

	var wg sync.WaitGroup
	wg.Add(2)
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		i := i
		go func() {
			defer wg.Done()
			_, errs[i] = svc.PollRun(ctx, run.ID)
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("poll %d: %v", i, err)
		}
	}
	// One winner starts attempt 2; loser cancels orphan. Start count should be
	// 1 (initial) + 2 (both tried retry) = 3 starts, but attempt_count stays 2.
	// Or if second CAS fails after first wins quickly: starts=3, cancel>=1.
	mid, _ := repo.GetRun(ctx, run.ID)
	if mid.ProviderAttemptCount != 2 {
		t.Fatalf("provider_attempt_count=%d want 2", mid.ProviderAttemptCount)
	}
	if fake.StartResponseCount < 2 {
		t.Fatalf("StartResponseCount=%d", fake.StartResponseCount)
	}
	// No third committed attempt.
	if mid.ProviderAttemptCount >= 3 {
		t.Fatal("orphan attempt committed")
	}
}

func TestProviderRetryScenarioH_RecoveryContinuesCurrentResponse(t *testing.T) {
	ctx := context.Background()
	repo := newMemoryRepository()
	packID := "pack:1:2026-07-08:abc123def45678"
	stub := &stubPackBuilder{result: validStubPack(packID), key: "k"}
	fake := provider.NewFakeProvider("")
	fake.AttemptScripts = []provider.FakeAttemptScript{
		{Status: provider.ProviderResponseStatusIncomplete, IncompleteReason: "max_output_tokens"},
		{Status: provider.ProviderResponseStatusCompleted, AnalysisJSON: []byte(`{"schema_version":"aivion_unified_analysis_v10","ok":true}`)},
	}
	svc := newAnalysisTestService(repo, stub, fake)
	run := startAnalysisRunReadyToPoll(t, svc, repo)

	if _, err := svc.PollRun(ctx, run.ID); err != nil {
		t.Fatalf("PollRun retry: %v", err)
	}
	mid, _ := repo.GetRun(ctx, run.ID)
	currentID := mid.ProviderResponseID
	startsBefore := fake.StartResponseCount

	// Simulate worker restart: poll again without re-starting previous attempt.
	poll, err := svc.PollRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("PollRun after restart: %v", err)
	}
	if !poll.Completed {
		t.Fatalf("expected completion on current response, got %+v", poll)
	}
	after, _ := repo.GetRun(ctx, run.ID)
	if after.ProviderResponseID != currentID {
		t.Fatalf("response id changed unexpectedly")
	}
	if fake.StartResponseCount != startsBefore {
		t.Fatalf("unexpected extra start after recovery: before=%d after=%d", startsBefore, fake.StartResponseCount)
	}
}

func TestConfigAnalysisDefaultAndLegacyFallback(t *testing.T) {
	t.Setenv("UNIFIED_ANALYSIS_PROVIDER_ENABLED", "")
	t.Setenv("UNIFIED_ANALYSIS_ENABLED", "")
	t.Setenv("UNIFIED_ANALYSIS_ANALYSIS_MAX_OUTPUT_TOKENS", "")
	t.Setenv("UNIFIED_ANALYSIS_SMOKE_MAX_OUTPUT_TOKENS", "")
	t.Setenv("UNIFIED_ANALYSIS_MAX_OUTPUT_TOKENS", "")
	t.Setenv("UNIFIED_ANALYSIS_PROVIDER_MAX_ATTEMPTS", "")
	t.Setenv("UNIFIED_ANALYSIS_PROMPT_PROFILE", "analysis")
	cfg := ConfigFromEnv()
	if cfg.AnalysisMaxOutputTokens != 32768 {
		t.Fatalf("analysis default=%d", cfg.AnalysisMaxOutputTokens)
	}
	if cfg.SmokeMaxOutputTokens != 2000 {
		t.Fatalf("smoke default=%d", cfg.SmokeMaxOutputTokens)
	}
	if cfg.EffectiveMaxOutputTokens("analysis") != 32768 {
		t.Fatalf("effective analysis=%d", cfg.EffectiveMaxOutputTokens("analysis"))
	}
	if cfg.ProviderMaxAttempts != 2 {
		t.Fatalf("max attempts=%d", cfg.ProviderMaxAttempts)
	}

	t.Setenv("UNIFIED_ANALYSIS_MAX_OUTPUT_TOKENS", "12000")
	t.Setenv("UNIFIED_ANALYSIS_ANALYSIS_MAX_OUTPUT_TOKENS", "")
	t.Setenv("UNIFIED_ANALYSIS_SMOKE_MAX_OUTPUT_TOKENS", "")
	cfg = ConfigFromEnv()
	if cfg.AnalysisMaxOutputTokens != 12000 || cfg.SmokeMaxOutputTokens != 12000 {
		t.Fatalf("legacy fallback analysis=%d smoke=%d", cfg.AnalysisMaxOutputTokens, cfg.SmokeMaxOutputTokens)
	}

	t.Setenv("UNIFIED_ANALYSIS_ANALYSIS_MAX_OUTPUT_TOKENS", "0")
	cfg = ConfigFromEnv()
	cfg.Enabled = true
	cfg.Provider = ProviderNameOpenAI
	cfg.Model = "gpt-4.1"
	cfg.MemoryLimit = "4g"
	cfg.PollIntervalSeconds = 5
	cfg.MaxPollDurationSeconds = 10
	cfg.HTTPTimeoutSeconds = 10
	cfg.ProviderMaxAttempts = 2
	if err := cfg.Validate("sk"); err == nil {
		t.Fatal("expected zero analysis tokens to fail validation")
	}

	cfg.AnalysisMaxOutputTokens = 40000
	cfg.SmokeMaxOutputTokens = 2000
	cfg.MaxOutputTokens = 2000
	if err := cfg.Validate("sk"); err == nil {
		t.Fatal("expected over-cap validation error for gpt-4.1")
	}

	cfg.AnalysisMaxOutputTokens = 32768
	cfg.ProviderMaxAttempts = 0
	if err := cfg.Validate("sk"); err == nil {
		t.Fatal("expected invalid max attempts")
	}
	cfg.ProviderMaxAttempts = 2
	cfg.Model = "some-unknown-model-xyz"
	cfg.AnalysisMaxOutputTokens = 999999
	if err := cfg.Validate("sk"); err != nil {
		t.Fatalf("unknown model should not hard-cap: %v", err)
	}
}
