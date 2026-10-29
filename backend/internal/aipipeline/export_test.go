package aipipeline

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicontext"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicost"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/recommendations"
)

// GlobalContextSystemPromptForTest exposes the legacy V1 system prompt for unit tests.
func GlobalContextSystemPromptForTest() string {
	return globalContextSystemPrompt()
}

// GlobalContextSystemPromptV2ForTest exposes the shared-signals system prompt for unit tests.
func GlobalContextSystemPromptV2ForTest() string {
	return globalContextSystemPromptV2()
}

// BuildGlobalContextArtifactForTest builds a final global context artifact for tests.
func BuildGlobalContextArtifactForTest(
	input GlobalContextGenerationInput,
	ctx *aicontext.GlobalSemanticContext,
	inputHash, contextHash string,
) (GlobalContextArtifact, error) {
	return buildGlobalContextArtifact(input, ctx, inputHash, contextHash)
}

// MarshalGlobalContextArtifactDocumentForTest returns stored artifact JSON and document hash.
func MarshalGlobalContextArtifactDocumentForTest(runID int64, artifact GlobalContextArtifact) ([]byte, string, error) {
	meta := pipelineArtifactMeta(runID, map[string]any{
		"seller_account_id":         artifact.SellerAccountID,
		"metric_date":               artifact.MetricDate,
		"data_pack_hash":            artifact.DataPackHash,
		"global_context_hash":       artifact.GlobalContextHash,
		"global_context_input_hash": artifact.GlobalContextInputHash,
	})
	doc := ArtifactDocument{Meta: meta, Payload: scrubArtifactPayload(artifact)}
	hash, err := HashArtifactPayload(doc)
	if err != nil {
		return nil, "", err
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, "", err
	}
	return raw, hash, nil
}

// PipelineArtifactDocumentHashForTest hashes a stored artifact payload_json document.
func PipelineArtifactDocumentHashForTest(raw []byte) (string, error) {
	doc, err := artifactDocumentPayload(raw)
	if err != nil {
		return "", err
	}
	return HashArtifactPayload(doc)
}

// SetGlobalContextClientForTest overrides the global context AI client in tests.
func SetGlobalContextClientForTest(s *SellerAIPipelineService, client GlobalContextAIClient) {
	if s == nil {
		return
	}
	s.gcClient = client
}

// NewPipelineDiagnosticsForTest exposes diagnostics construction for tests.
func NewPipelineDiagnosticsForTest(providers PipelineProviders) PipelineDiagnostics {
	return newPipelineDiagnostics(providers)
}

// ChunkAnalyzerForTest exposes the configured chunk analysis runner for tests.
func ChunkAnalyzerForTest(s *SellerAIPipelineService) ChunkAnalysisRunner {
	if s == nil {
		return nil
	}
	return s.chunkAnalyzer
}

// SetChunkSetBuilderForTest overrides the shared ChunkSet builder in tests.
func SetChunkSetBuilderForTest(s *SellerAIPipelineService, fn chunkSetBuildFunc) {
	if s == nil {
		return
	}
	s.buildChunkSet = fn
}

// MaxAIInputTokensPerRequestForTest exposes the canonical input token limit for cross-package consistency tests.
func MaxAIInputTokensPerRequestForTest() int {
	return MaxAIInputTokensPerRequest()
}

// MaxAIOutputTokensPerRequestForTest exposes max_output_tokens budget for tests.
func MaxAIOutputTokensPerRequestForTest() int {
	return MaxAIOutputTokensPerRequest()
}

// GlobalContextRequestedMaxOutputTokensForTest exposes GC output limit for tests.
func GlobalContextRequestedMaxOutputTokensForTest() int {
	return GlobalContextRequestedMaxOutputTokens()
}

// GlobalContextEffectiveMaxOutputTokensForTest exposes effective GC output limit for tests.
func GlobalContextEffectiveMaxOutputTokensForTest() int {
	return GlobalContextEffectiveMaxOutputTokens()
}

// ClassifyGlobalContextFailureForTest exposes failure classification for tests.
func ClassifyGlobalContextFailureForTest(raw *GlobalContextRawResponse, parseErr error, validationStatus string) string {
	return ClassifyGlobalContextFailure(raw, parseErr, validationStatus)
}

// EstimateGlobalContextRequestMetricsForTest exposes request size diagnostics for tests.
func EstimateGlobalContextRequestMetricsForTest(
	input GlobalContextGenerationInput,
	promptModel string,
	timeoutSeconds int,
	maxInputTokens int,
	maxOutputTokens int,
) (GlobalContextRequestMetrics, error) {
	return estimateGlobalContextRequestMetrics(input, promptModel, timeoutSeconds, maxInputTokens, maxOutputTokens)
}

// PipelineResumeModeForTest exposes resume detection for tests.
func PipelineResumeModeForTest(req RunSellerAIPipelineRequest) bool {
	return pipelineResumeMode(req)
}

// PopulateGlobalContextRequestDiagnosticsForTest copies request metrics into diagnostics.
func PopulateGlobalContextRequestDiagnosticsForTest(diag *PipelineDiagnostics, metrics GlobalContextRequestMetrics) {
	populateGlobalContextRequestDiagnostics(diag, metrics)
}

// SelectGlobalContextOpenAIClientForTest exposes stage-scoped OpenAI client wiring for tests.
func SelectGlobalContextOpenAIClientForTest(
	narrativeSvc *dashboardhome.NarrativeService,
	cfg *config.Config,
) (*recommendations.OpenAIClient, int, error) {
	return SelectGlobalContextOpenAIClient(narrativeSvc, cfg)
}

// SelectActionsOpenAIClientForTest exposes Actions stage-scoped OpenAI client wiring for tests.
func SelectActionsOpenAIClientForTest(
	narrativeSvc *dashboardhome.NarrativeService,
	cfg *config.Config,
) (*recommendations.OpenAIClient, int, error) {
	return SelectActionsOpenAIClient(narrativeSvc, cfg)
}

// SelectActionsOpenAIClientFromBaseForTest clones a base client with Actions timeout for tests.
func SelectActionsOpenAIClientFromBaseForTest(
	base *recommendations.OpenAIClient,
	cfg *config.Config,
) (*recommendations.OpenAIClient, int, error) {
	return SelectActionsOpenAIClientFromBase(base, cfg)
}

// PipelineStageRestartModeForTest exposes stage restart detection for tests.
func PipelineStageRestartModeForTest(req RunSellerAIPipelineRequest) bool {
	return pipelineStageRestartMode(req)
}

// ResolvePipelineStageRestartPlanForTest exposes stage restart plan resolver for tests.
func ResolvePipelineStageRestartPlanForTest(req RunSellerAIPipelineRequest) PipelineStageRestartPlan {
	return ResolvePipelineStageRestartPlan(req)
}

// OrderedChunkHashesForTest exposes chunk hash ordering for tests.
func OrderedChunkHashesForTest(chunkSet *aidatapack.ChunkSet) ([]string, error) {
	return orderedChunkHashes(chunkSet)
}

// BuildSharedChunkAnalysisArtifactForTest builds a shared chunk analysis artifact for tests.
func BuildSharedChunkAnalysisArtifactForTest(
	pack *aidatapack.SellerAIDataPack,
	dataPackHash string,
	chunkSet *aidatapack.ChunkSet,
	chunkHashes []string,
	chunkRun *dashboardhome.ChunkAnalysisRunResult,
	limitations []string,
) (SharedChunkAnalysisArtifact, error) {
	return buildSharedChunkAnalysisArtifact(pack, dataPackHash, chunkSet, chunkHashes, chunkRun, limitations)
}

// BuildGlobalContextGenerationInputForTest builds global context input for tests.
func BuildGlobalContextGenerationInputForTest(
	pack *aidatapack.SellerAIDataPack,
	dataPackHash string,
	chunkSet *aidatapack.ChunkSet,
	chunkRun *dashboardhome.ChunkAnalysisRunResult,
) (GlobalContextGenerationInput, error) {
	return BuildGlobalContextGenerationInput(pack, dataPackHash, chunkSet, chunkRun)
}

// ResumePlanInputForTest builds resume plan resolver input for unit tests.
type ResumePlanInputForTest struct {
	Req                   RunSellerAIPipelineRequest
	Persisted             dbgen.AiPipelineRun
	DataPackHash          string
	GlobalContextHash     string
	DashboardReusable     bool
	ActionsReusable       bool
	SharedChunkReusable   bool
	GlobalContextReusable bool
	Published             bool
}

// ResolvePipelineResumePlanForTest exposes resume plan resolver for tests.
func ResolvePipelineResumePlanForTest(in ResumePlanInputForTest) PipelineResumePlan {
	return ResolvePipelineResumePlan(resumePlanInput{
		Req:                   in.Req,
		Persisted:             in.Persisted,
		DataPackHash:          in.DataPackHash,
		GlobalContextHash:     in.GlobalContextHash,
		DashboardReusable:     in.DashboardReusable,
		ActionsReusable:       in.ActionsReusable,
		SharedChunkReusable:   in.SharedChunkReusable,
		GlobalContextReusable: in.GlobalContextReusable,
		Published:             in.Published,
	})
}

// NewCaptureUsageRecorderForTest records usage events in memory for stage 9 tests.
func NewCaptureUsageRecorderForTest() *CaptureUsageRecorder {
	return newCaptureUsageRecorder()
}

// CaptureUsageRecorder is an in-memory usage recorder for tests.
type CaptureUsageRecorder = captureUsageRecorder

// CountUsageComponentForTest counts usage rows for a component.
func CountUsageComponentForTest(events []aicost.UsageEventInput, component string) int {
	return countUsageComponent(events, component)
}

// SumUsageComponentTokensForTest sums tokens for a component.
func SumUsageComponentTokensForTest(events []aicost.UsageEventInput, component string) (int, int) {
	return sumUsageComponentTokens(events, component)
}

// FindUsageEventForTest returns the first usage event for a component.
func FindUsageEventForTest(events []aicost.UsageEventInput, component string) (aicost.UsageEventInput, bool) {
	for _, ev := range events {
		if ev.Component == component {
			return ev, true
		}
	}
	return aicost.UsageEventInput{}, false
}

// ParseChunkUsageRequestIDForTest exposes chunk usage request id parsing for tests.
func ParseChunkUsageRequestIDForTest(encoded string) ParsedChunkUsageRequestID {
	return ParseChunkUsageRequestID(encoded)
}

// PipelineChunkFixtureForTests returns a valid data pack and chunk set for shared stage tests.
func PipelineChunkFixtureForTests() (*aidatapack.SellerAIDataPack, *aidatapack.ChunkSet, error) {
	metricDate := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)
	pack := &aidatapack.SellerAIDataPack{
		SchemaVersion: aidatapack.SchemaVersion,
		SellerContext: aidatapack.SellerContext{SellerAccountID: 1, MetricDate: "2026-06-25"},
		DataWindows:   aidatapack.ComputeDataWindows(metricDate),
		DataQuality:   aidatapack.DataQuality{Sources: map[string]any{}, Limitations: []string{}},
		EntityRegistry: aidatapack.EntityRegistry{
			SKUs: map[string]aidatapack.SKUEntity{}, SKUGroups: map[string]aidatapack.SKUGroupEntity{},
			Campaigns: map[string]aidatapack.CampaignEntity{}, Warehouses: map[string]aidatapack.WarehouseEntity{},
			Categories: map[string]aidatapack.CategoryEntity{},
		},
		FactRegistry: map[string]aidatapack.Fact{
			"account.current_day.revenue": {
				FactID: "account.current_day.revenue", EntityType: "account", EntityID: "account:seller",
				Metric: "revenue", Value: 1.0, Unit: aidatapack.UnitRUB,
				Source: aidatapack.FactSource{Table: "daily_account_metrics", Field: "revenue"},
			},
		},
		Tables: aidatapack.Tables{
			AccountRollup: aidatapack.AccountRollupTable{DataStatus: "available", CurrentDay: map[string]any{"revenue": 1}},
			FinanceRollup: aidatapack.FinanceRollupTable{DataStatus: "not_available"},
		},
		Constraints: aidatapack.ConstraintsBlock{SKUPriceConstraints: []any{}},
	}
	for i := 1; i <= 3; i++ {
		sku := "80100010000" + string(rune('0'+i))
		eid := "sku:" + sku
		pack.EntityRegistry.SKUs[eid] = aidatapack.SKUEntity{
			EntityID: eid, EntityType: "sku", SKU: sku, OfferID: "OFFER-" + sku, Name: "Product " + sku,
		}
		coverFact := "sku." + sku + ".coverage_days"
		pack.FactRegistry[coverFact] = aidatapack.Fact{
			FactID: coverFact, EntityType: "sku", EntityID: eid, Metric: "coverage_days", Window: "current",
			Value: float64(i), Unit: aidatapack.UnitDays,
			Source: aidatapack.FactSource{Table: "stock_positions", Field: "coverage_days"},
		}
		pack.Tables.StockPositions = append(pack.Tables.StockPositions, aidatapack.StockPositionRow{
			EntityID: eid, SnapshotDate: "2026-06-25", DataStatus: "available", FactIDs: []string{coverFact},
		})
	}
	if err := aidatapack.ValidateDataPack(pack); err != nil {
		return nil, nil, err
	}
	set, err := aidatapack.BuildChunks(pack, aidatapack.DefaultChunkOptions())
	if err != nil {
		return nil, nil, err
	}
	if err := aidatapack.ValidateChunkSet(pack, set); err != nil {
		return nil, nil, err
	}
	return pack, set, nil
}

// Run68StyleChunkSetForTests returns a 24-chunk set matching the run 68 layout for coverage tests.
func Run68StyleChunkSetForTests() *aidatapack.ChunkSet {
	chunks := []aidatapack.DataPackChunk{
		{ChunkID: "account_funnel_quality_constraints", ChunkType: "account_level_tables", SourceTable: "account_funnel_finance", ChunkHash: "h-account"},
	}
	for i := 1; i <= 4; i++ {
		id := "sku_rollups_00" + strconv.Itoa(i)
		chunks = append(chunks, aidatapack.DataPackChunk{ChunkID: id, ChunkType: "sku_rollups", SourceTable: "sku_rollups", ChunkHash: "h-" + id})
		id = "stock_positions_00" + strconv.Itoa(i)
		chunks = append(chunks, aidatapack.DataPackChunk{ChunkID: id, ChunkType: "stock_positions", SourceTable: "stock_positions", ChunkHash: "h-" + id})
	}
	chunks = append(chunks,
		aidatapack.DataPackChunk{ChunkID: "campaign_rollups_001", ChunkType: "campaign_rollups", SourceTable: "campaign_rollups", ChunkHash: "h-campaign"},
		aidatapack.DataPackChunk{ChunkID: "category_rollups_001", ChunkType: "category_rollups", SourceTable: "category_rollups", ChunkHash: "h-category"},
		aidatapack.DataPackChunk{ChunkID: "product_enrichment_001", ChunkType: "product_enrichment", SourceTable: "product_enrichment", ChunkHash: "h-pe"},
		aidatapack.DataPackChunk{ChunkID: "sku_groups_001", ChunkType: "entity_dictionary", SourceTable: "sku_groups", ChunkHash: "h-sg"},
		aidatapack.DataPackChunk{ChunkID: "campaign_product_links_001", ChunkType: "campaign_product_links", SourceTable: "campaign_product_links", ChunkHash: "h-cpl"},
	)
	for i := 14; i <= 23; i++ {
		id := "optional_" + strconv.Itoa(i)
		chunks = append(chunks, aidatapack.DataPackChunk{ChunkID: id, ChunkType: "product_enrichment", SourceTable: "product_enrichment", ChunkHash: "h-" + id})
	}
	return &aidatapack.ChunkSet{
		SchemaVersion:   aidatapack.ChunkSetSchemaVersion,
		SellerAccountID: 1,
		MetricDate:      "2026-06-25",
		Chunks:          chunks,
	}
}

// Run68StyleFailedChunkRunForTests simulates run 68 coverage failure for pipeline integration tests.
func Run68StyleFailedChunkRunForTests(chunkSet *aidatapack.ChunkSet) *dashboardhome.ChunkAnalysisRunResult {
	if chunkSet == nil {
		return &dashboardhome.ChunkAnalysisRunResult{}
	}
	failedSet := map[string]struct{}{
		"account_funnel_quality_constraints": {},
		"sku_rollups_001":                    {}, "sku_rollups_002": {}, "sku_rollups_003": {}, "sku_rollups_004": {},
		"stock_positions_001": {}, "stock_positions_002": {}, "stock_positions_003": {}, "stock_positions_004": {},
		"campaign_rollups_001": {}, "category_rollups_001": {},
		"optional_14": {}, "optional_15": {}, "optional_16": {}, "optional_17": {},
	}
	failedIDs := make([]string, 0)
	failedTypes := make([]string, 0)
	for _, ch := range chunkSet.Chunks {
		if _, bad := failedSet[ch.ChunkID]; bad {
			failedIDs = append(failedIDs, ch.ChunkID)
			failedTypes = append(failedTypes, ch.ChunkType)
		}
	}
	completed := len(chunkSet.Chunks) - len(failedIDs)
	return &dashboardhome.ChunkAnalysisRunResult{
		SellerAccountID:       chunkSet.SellerAccountID,
		MetricDate:            chunkSet.MetricDate,
		ChunkCount:            len(chunkSet.Chunks),
		Completed:             completed,
		Failed:                len(failedIDs),
		ProviderFailedCount:   len(failedIDs) - 2,
		ValidationFailedCount: 2,
		FailedChunkIDs:        failedIDs,
		FailedChunkTypes:      failedTypes,
		ValidatedSignalCount:  1,
		ValidatedSignals: []dashboardhome.DashboardChunkSignal{
			{SignalID: "s1", SignalType: dashboardhome.ChunkSignalObservation, Observation: "ok"},
		},
		AnalysisStatus: dashboardhome.ChunkAnalysisStatusPartial,
	}
}
