package dashboardhome

import (
	"context"
	"strconv"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicontext"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aitokens"
)

func russianTokenGuardSynthesisInput(pack *aidatapack.SellerAIDataPack, chunkSet *aidatapack.ChunkSet, run ChunkAnalysisRunResult) DashboardSynthesisInput {
	build, _, _, err := buildDashboardNarrativeContextFromDataPackFull(pack, nil, dataPackNarrativeLegacyInput{})
	if err != nil {
		panic(err)
	}
	build.ContextSource = ContextSourceSellerAIDataPackChunks
	build.GlobalSemanticContext = &aicontext.GlobalSemanticContext{
		SchemaVersion: aicontext.GlobalSemanticContextSchemaVersion,
		OverallStory:  "Краткое резюме: выручка снизилась на 12%, остатки по ключевым SKU в зоне дефицита.",
		PrimaryRisks: []aicontext.SemanticRisk{{
			RiskID: "risk-stock", Category: "stock", Severity: "high",
			Title: "Дефицит остатков", Rationale: "Покрытие менее трёх дней по части SKU.",
		}},
	}
	return buildDashboardSynthesisInput(
		build,
		extractManifestSummary(chunkSet),
		append([]string{"finance_not_available"}, "Часть кампаний без заказов за период"),
		cloneDashboardChunkSignals(run.ValidatedSignals),
		pack,
		NarrativeRunOptions{GlobalContextHash: "gc-hash-test"},
	)
}

func TestTokenGuard_RussianPayload_EstimatesSerializedSynthesisInput(t *testing.T) {
	pack, chunkSet, run := precomputedTestFixtures(t)
	run.ValidatedSignals = []DashboardChunkSignal{
		{
			SignalID: "signal_ru_a", SignalType: ChunkSignalObservation,
			Observation:    "Выручка по аккаунту снизилась на 12% за последние 7 дней.",
			WhyItMayMatter: "Снижение может отражать спад спроса или проблемы с наличием.",
			FactIDs:        []string{"account.last_7d.revenue"},
			EntityIDs:      []string{"sku:801"},
		},
		{
			SignalID: "signal_ru_b", SignalType: ChunkSignalRiskCandidate,
			Observation: "У SKU 801 остаток в зоне DEFICIT при сохраняющемся спросе.",
			FactIDs:     []string{"sku.801.last_7d.revenue"},
			EntityIDs:   []string{"sku:801"},
		},
		{
			SignalID: "signal_ru_c", SignalType: ChunkSignalDataQuality,
			Observation: "Финансовые метрики недоступны; выводы ограничены операционными данными.",
		},
	}
	run.ValidatedSignalCount = len(run.ValidatedSignals)

	input := russianTokenGuardSynthesisInput(pack, chunkSet, run)
	est, err := EstimateSynthesisInputTokensForTest(input)
	if err != nil {
		t.Fatal(err)
	}
	estViaCanonical, err := aitokens.EstimateJSONTokens(input)
	if err != nil {
		t.Fatal(err)
	}
	if est != estViaCanonical || est <= 0 {
		t.Fatalf("serialized estimate mismatch: dashboard=%d canonical=%d", est, estViaCanonical)
	}
	if input.ValidatedSignals[0].Observation == "" || input.GlobalSemanticContext == nil {
		t.Fatal("expected Russian signals and global context in serialized payload")
	}
}

func TestTokenGuard_RussianPayload_BelowLimit_AllowsSingleSynthesis(t *testing.T) {
	withLegacyDashboardMode(t)
	pack, chunkSet, run := precomputedTestFixtures(t)
	run.ValidatedSignals = []DashboardChunkSignal{{
		SignalID: "signal_ru", SignalType: ChunkSignalObservation,
		Observation: "Стабильная динамика выручки по аккаунту.",
		FactIDs:     []string{"account.last_7d.revenue"},
	}}
	run.ValidatedSignalCount = 1
	input := russianTokenGuardSynthesisInput(pack, chunkSet, run)
	est, err := EstimateSynthesisInputTokensForTest(input)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("AI_MAX_INPUT_TOKENS_PER_REQUEST", strconv.Itoa(est+1000))

	build, _, _, err := buildDashboardNarrativeContextFromDataPackFull(pack, nil, dataPackNarrativeLegacyInput{})
	if err != nil {
		t.Fatal(err)
	}
	build.ContextSource = ContextSourceSellerAIDataPackChunks

	ResetBuildChunkSetForPackCallsForTest()
	ai := &recordingDashboardAI{}
	orch := &DataPackChunksOrchestrator{AI: ai}
	_ = orch.SynthesizeFromChunkRun(context.Background(), pack, chunkSet, run, build, NarrativeRunOptions{})
	if BuildChunkSetForPackCallsForTest() != 0 {
		t.Fatalf("chunk build calls=%d", BuildChunkSetForPackCallsForTest())
	}
	if ai.chunkCalls != 0 {
		t.Fatalf("chunk analysis calls=%d", ai.chunkCalls)
	}
	if ai.synthesisCalls != 1 {
		t.Fatalf("synthesis calls=%d", ai.synthesisCalls)
	}
}

func TestTokenGuard_RussianPayload_AboveLimit_BlocksSynthesis(t *testing.T) {
	withLegacyDashboardMode(t)
	pack, chunkSet, run := precomputedTestFixtures(t)
	run.ValidatedSignals = []DashboardChunkSignal{{
		SignalID: "signal_ru", SignalType: ChunkSignalObservation,
		Observation: "Длинное русскоязычное наблюдение о динамике продаж и остатков по нескольким SKU.",
		FactIDs:     []string{"account.last_7d.revenue", "sku.801.last_7d.revenue"},
		EntityIDs:   []string{"sku:801"},
	}}
	run.ValidatedSignalCount = 1
	build, _, _, err := buildDashboardNarrativeContextFromDataPackFull(pack, nil, dataPackNarrativeLegacyInput{})
	if err != nil {
		t.Fatal(err)
	}
	build.ContextSource = ContextSourceSellerAIDataPackChunks
	build.GlobalSemanticContext = &aicontext.GlobalSemanticContext{
		SchemaVersion: aicontext.GlobalSemanticContextSchemaVersion,
		OverallStory:  "Краткое резюме: выручка снизилась на 12%, остатки по ключевым SKU в зоне дефицита.",
		PrimaryRisks: []aicontext.SemanticRisk{{
			RiskID: "risk-stock", Category: "stock", Severity: "high",
			Title: "Дефицит остатков", Rationale: "Покрытие менее трёх дней по части SKU.",
		}},
	}
	limitations := append([]string(nil), pack.DataQuality.Limitations...)
	input := buildDashboardSynthesisInput(
		build,
		extractManifestSummary(chunkSet),
		limitations,
		cloneDashboardChunkSignals(run.ValidatedSignals),
		pack,
		NarrativeRunOptions{GlobalContextHash: "gc-hash-test"},
	)
	est, err := EstimateSynthesisInputTokensForTest(input)
	if err != nil {
		t.Fatal(err)
	}
	if est <= 1 {
		t.Fatalf("expected large serialized payload for guard test, est=%d", est)
	}
	t.Setenv("AI_MAX_INPUT_TOKENS_PER_REQUEST", strconv.Itoa(est-1))

	ResetBuildChunkSetForPackCallsForTest()
	ai := &recordingDashboardAI{}
	orch := &DataPackChunksOrchestrator{AI: ai}
	gen := orch.SynthesizeFromChunkRun(context.Background(), pack, chunkSet, run, build, NarrativeRunOptions{GlobalContextHash: "gc-hash-test"})
	if !gen.Fatal {
		t.Fatal("expected token guard failure")
	}
	if NarrativeErrorCode(gen.Error) != NarrativeErrorDashboardInputTooLarge {
		t.Fatalf("code=%q err=%v", NarrativeErrorCode(gen.Error), gen.Error)
	}
	if BuildChunkSetForPackCallsForTest() != 0 || ai.chunkCalls != 0 || ai.synthesisCalls != 0 {
		t.Fatalf("build=%d chunk=%d synthesis=%d", BuildChunkSetForPackCallsForTest(), ai.chunkCalls, ai.synthesisCalls)
	}
}
