package aipipeline_test

import (
	"context"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicost"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
)

func TestStage9DashboardSynthesisUsageMetadata(t *testing.T) {
	sharedChunkPipelineEnv(t)
	svc := newSharedChunkPipelineService(t, &fakeStore{}, aipipeline.NewDeterministicChunkAnalysisRunner())
	rec := aipipeline.NewCaptureUsageRecorderForTest()
	svc.SetUsageRecorder(rec)

	res, err := svc.Run(context.Background(), sharedChunkPipelineRequest())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	ev, ok := aipipeline.FindUsageEventForTest(rec.Events(), aipipeline.AIUsageComponentDashboardSynthesis)
	if !ok {
		t.Fatal("missing dashboard_synthesis usage row")
	}
	if ev.RequestID != "req_dash_123" {
		t.Fatalf("request_id=%q", ev.RequestID)
	}
	if ev.InputTokens != 400 || ev.OutputTokens != 40 {
		t.Fatalf("tokens=%d/%d", ev.InputTokens, ev.OutputTokens)
	}
	if ev.LatencyMs < 0 {
		t.Fatalf("latency_ms=%d", ev.LatencyMs)
	}
	if ev.Attempt != 1 {
		t.Fatalf("attempt=%d", ev.Attempt)
	}
	if ev.Model == "" || ev.Provider == "" {
		t.Fatalf("provider/model empty: provider=%q model=%q", ev.Provider, ev.Model)
	}
	if res.Diagnostics == nil || res.Diagnostics.RetryCount != 0 {
		t.Fatalf("retry_count=%d", res.Diagnostics.RetryCount)
	}
	if res.Diagnostics.Dashboard.SynthesisRequestID != "req_dash_123" {
		t.Fatalf("diag request_id=%q", res.Diagnostics.Dashboard.SynthesisRequestID)
	}
}

func TestStage9GlobalContextUsageFromProvider(t *testing.T) {
	sharedChunkPipelineEnv(t)
	svc := newSharedChunkPipelineService(t, &fakeStore{}, aipipeline.NewDeterministicChunkAnalysisRunner())
	rec := aipipeline.NewCaptureUsageRecorderForTest()
	svc.SetUsageRecorder(rec)

	if _, err := svc.Run(context.Background(), sharedChunkPipelineRequest()); err != nil {
		t.Fatal(err)
	}
	ev, ok := aipipeline.FindUsageEventForTest(rec.Events(), aipipeline.AIUsageComponentGlobalContext)
	if !ok {
		t.Fatal("missing global_context usage")
	}
	if ev.InputTokens != 256 || ev.OutputTokens != 128 {
		t.Fatalf("tokens=%d/%d want provider fake 256/128", ev.InputTokens, ev.OutputTokens)
	}
	if ev.RequestID != "fake-gc-initial" {
		t.Fatalf("request_id=%q", ev.RequestID)
	}
}

func TestStage9ActionsSynthesisUsageFromProvider(t *testing.T) {
	sharedChunkPipelineEnv(t)
	svc := newSharedChunkPipelineService(t, &fakeStore{}, aipipeline.NewDeterministicChunkAnalysisRunner())
	rec := aipipeline.NewCaptureUsageRecorderForTest()
	svc.SetUsageRecorder(rec)

	if _, err := svc.Run(context.Background(), sharedChunkPipelineActionsAIRequest()); err != nil {
		t.Fatal(err)
	}
	ev, ok := aipipeline.FindUsageEventForTest(rec.Events(), aipipeline.AIUsageComponentActionsSynthesis)
	if !ok {
		t.Fatal("missing actions_synthesis usage")
	}
	if ev.InputTokens != 512 || ev.OutputTokens != 128 {
		t.Fatalf("tokens=%d/%d want provider fake 512/128", ev.InputTokens, ev.OutputTokens)
	}
	if ev.Attempt != 1 {
		t.Fatalf("attempt=%d", ev.Attempt)
	}
}

func TestStage9ChunkAnalysisUsageFromProvider(t *testing.T) {
	sharedChunkPipelineEnv(t)
	svc := newSharedChunkPipelineService(t, &fakeStore{}, aipipeline.NewDeterministicChunkAnalysisRunner())
	rec := aipipeline.NewCaptureUsageRecorderForTest()
	svc.SetUsageRecorder(rec)

	if _, err := svc.Run(context.Background(), sharedChunkPipelineRequest()); err != nil {
		t.Fatal(err)
	}
	events := rec.Events()
	var chunkRows []aicost.UsageEventInput
	for _, ev := range events {
		if ev.Component == aipipeline.AIUsageComponentChunkAnalysis {
			chunkRows = append(chunkRows, ev)
		}
	}
	if len(chunkRows) == 0 {
		t.Fatal("missing chunk_analysis usage rows")
	}
	in, out := aipipeline.SumUsageComponentTokensForTest(events, aipipeline.AIUsageComponentChunkAnalysis)
	if in <= 0 || out <= 0 {
		t.Fatalf("chunk totals=%d/%d", in, out)
	}
	for _, ev := range chunkRows {
		parsed := aipipeline.ParseChunkUsageRequestIDForTest(ev.RequestID)
		if !parsed.Encoded || parsed.ChunkID == "" {
			t.Fatalf("encoded request_id=%q parsed=%+v", ev.RequestID, parsed)
		}
		if ev.Attempt != 1 {
			t.Fatalf("attempt=%d", ev.Attempt)
		}
	}
}

func TestStage9UsageRecordsPerComponent(t *testing.T) {
	sharedChunkPipelineEnv(t)
	svc := newSharedChunkPipelineService(t, &fakeStore{}, aipipeline.NewDeterministicChunkAnalysisRunner())
	rec := aipipeline.NewCaptureUsageRecorderForTest()
	svc.SetUsageRecorder(rec)

	res, err := svc.Run(context.Background(), sharedChunkPipelineActionsAIRequest())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	events := rec.Events()
	if count := aipipeline.CountUsageComponentForTest(events, aipipeline.AIUsageComponentChunkAnalysis); count == 0 {
		t.Fatalf("chunk_analysis usage rows=0")
	}
	if count := aipipeline.CountUsageComponentForTest(events, aipipeline.AIUsageComponentGlobalContext); count != 1 {
		t.Fatalf("global_context usage rows=%d", count)
	}
	if count := aipipeline.CountUsageComponentForTest(events, aipipeline.AIUsageComponentDashboardSynthesis); count != 1 {
		t.Fatalf("dashboard_synthesis usage rows=%d", count)
	}
	if count := aipipeline.CountUsageComponentForTest(events, aipipeline.AIUsageComponentActionsSynthesis); count != 1 {
		t.Fatalf("actions_synthesis usage rows=%d", count)
	}
	for _, ev := range events {
		if ev.Attempt != 1 {
			t.Fatalf("attempt=%d component=%s", ev.Attempt, ev.Component)
		}
	}
	if res.Diagnostics == nil {
		t.Fatal("missing diagnostics")
	}
	if res.Diagnostics.RetryCount != 0 {
		t.Fatalf("retry_count=%d", res.Diagnostics.RetryCount)
	}
	if res.Diagnostics.TotalInputTokens <= 0 || res.Diagnostics.TotalOutputTokens <= 0 {
		t.Fatalf("totals in=%d out=%d", res.Diagnostics.TotalInputTokens, res.Diagnostics.TotalOutputTokens)
	}
}

func TestStage9ArtifactResumeNoNewChunkOrGCUsage(t *testing.T) {
	sharedChunkPipelineEnv(t)
	store := &persistingArtifactStore{}
	svc := newSharedChunkPipelineService(t, store, aipipeline.NewDeterministicChunkAnalysisRunner())
	rec := aipipeline.NewCaptureUsageRecorderForTest()
	svc.SetUsageRecorder(rec)

	first, err := svc.Run(context.Background(), sharedChunkPipelineRequest())
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	firstEvents := rec.Events()
	firstChunk := aipipeline.CountUsageComponentForTest(firstEvents, aipipeline.AIUsageComponentChunkAnalysis)
	firstGC := aipipeline.CountUsageComponentForTest(firstEvents, aipipeline.AIUsageComponentGlobalContext)
	if firstChunk == 0 || firstGC != 1 {
		t.Fatalf("first chunk=%d gc=%d", firstChunk, firstGC)
	}

	rec2 := aipipeline.NewCaptureUsageRecorderForTest()
	svc.SetUsageRecorder(rec2)
	req := sharedChunkPipelineRequest()
	req.ExistingPipelineRunID = first.PipelineRunID
	req.DispatchID = 1
	req.LeaseGuard = noopLeaseGuard{}
	second, err := svc.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	secondEvents := rec2.Events()
	if got := aipipeline.CountUsageComponentForTest(secondEvents, aipipeline.AIUsageComponentChunkAnalysis); got != 0 {
		t.Fatalf("second chunk usage rows=%d", got)
	}
	if got := aipipeline.CountUsageComponentForTest(secondEvents, aipipeline.AIUsageComponentGlobalContext); got != 0 {
		t.Fatalf("second gc usage rows=%d", got)
	}
	if second.Diagnostics == nil ||
		second.Diagnostics.SharedChunkAnalysis.Source != "artifact" ||
		second.Diagnostics.GlobalContext.Source != "artifact" {
		t.Fatalf("sources chunk=%q gc=%q",
			second.Diagnostics.SharedChunkAnalysis.Source,
			second.Diagnostics.GlobalContext.Source,
		)
	}
}

func TestStage9ChunkUsageDeterministicTotals(t *testing.T) {
	sharedChunkPipelineEnv(t)
	svc := newSharedChunkPipelineService(t, &fakeStore{}, aipipeline.NewDeterministicChunkAnalysisRunner())
	rec := aipipeline.NewCaptureUsageRecorderForTest()
	svc.SetUsageRecorder(rec)

	res, err := svc.Run(context.Background(), sharedChunkPipelineRequest())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	in, out := aipipeline.SumUsageComponentTokensForTest(rec.Events(), aipipeline.AIUsageComponentChunkAnalysis)
	if in <= 0 || out <= 0 {
		t.Fatalf("chunk usage in=%d out=%d", in, out)
	}
	if res.Diagnostics == nil || res.Diagnostics.SharedChunkAnalysis.InputTokens != in {
		t.Fatalf("diag in=%d usage in=%d", res.Diagnostics.SharedChunkAnalysis.InputTokens, in)
	}
	if res.Diagnostics.SharedChunkAnalysis.OutputTokens != out {
		t.Fatalf("diag out=%d usage out=%d", res.Diagnostics.SharedChunkAnalysis.OutputTokens, out)
	}
}
