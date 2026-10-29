package dashboardhome

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/recommendations"
)

const (
	chunkAnalysisMaxOutputTokens   = 6000
	synthesisMaxOutputTokens       = 6000
	synthesisMaxOutputTokensAIOnly = 12000
	synthesisMaxSignals            = 35
)

type structuredJSONClient interface {
	GenerateStructuredJSON(ctx context.Context, input recommendations.GenerateStructuredJSONInput) (*recommendations.GenerateRecommendationsOutput, error)
}

// DataPackChunksOrchestrator runs multi-pass chunk analysis and synthesis.
type DataPackChunksOrchestrator struct {
	AI    structuredJSONClient
	Model string
	Debug *NarrativeDebugRecorder
}

type DataPackChunksGenerationResult struct {
	Payload                           *dashboardNarrativePayload
	ChunkRun                          ChunkAnalysisRunResult
	SynthesisInputTokens              int
	SynthesisOutputTokens             int
	SynthesisRequestID                string
	SynthesisLatencyMs                int64
	SynthesisModel                    string
	RawSynthesisResponse              string
	SynthesisProviderStatus           string
	SynthesisFinishReason             string
	SynthesisIncompleteReason         string
	SynthesisOutputTruncated          bool
	SynthesisRequestedMaxOutputTokens int
	SynthesisParseError               string
	Pipeline                          ChunkPipelineOutcome
	Validation                        DashboardValidationResult
	Error                             error
	Fatal                             bool
}

func (o *DataPackChunksOrchestrator) Generate(
	ctx context.Context,
	pack *aidatapack.SellerAIDataPack,
	chunkSet *aidatapack.ChunkSet,
	build dashboardNarrativeBuildResult,
	opts NarrativeRunOptions,
) DataPackChunksGenerationResult {
	out := DataPackChunksGenerationResult{}
	if pack == nil || chunkSet == nil {
		out.Fatal = true
		out.Error = fmt.Errorf("pack or chunk set is nil")
		out.Pipeline = failedPipelineOutcome(pack, chunkSet, ChunkSynthesisStatusNotStarted, out.Error.Error())
		return out
	}
	chunkRun, err := o.AnalyzeChunksOnly(ctx, pack, chunkSet)
	if err != nil {
		out.Fatal = true
		out.Error = err
		out.Pipeline = failedPipelineOutcome(pack, chunkSet, ChunkSynthesisStatusNotStarted, err.Error())
		return out
	}
	return o.SynthesizeFromChunkRun(ctx, pack, chunkSet, chunkRun, build, opts)
}

// SynthesizeFromChunkRun performs Dashboard synthesis from an existing chunk analysis result.
func (o *DataPackChunksOrchestrator) SynthesizeFromChunkRun(
	ctx context.Context,
	pack *aidatapack.SellerAIDataPack,
	chunkSet *aidatapack.ChunkSet,
	chunkRun ChunkAnalysisRunResult,
	build dashboardNarrativeBuildResult,
	opts NarrativeRunOptions,
) DataPackChunksGenerationResult {
	out := DataPackChunksGenerationResult{ChunkRun: chunkRun}
	if pack == nil || chunkSet == nil {
		out.Fatal = true
		out.Error = fmt.Errorf("pack or chunk set is nil")
		out.Pipeline = failedPipelineOutcome(pack, chunkSet, ChunkSynthesisStatusNotStarted, out.Error.Error())
		return out
	}
	if err := aidatapack.ValidateChunkSet(pack, chunkSet); err != nil {
		out.Fatal = true
		out.Error = fmt.Errorf("validate chunk set: %w", err)
		out.Pipeline = failedPipelineOutcome(pack, chunkSet, ChunkSynthesisStatusNotStarted, out.Error.Error())
		return out
	}
	if chunkRun.Completed == 0 && chunkRun.Failed > 0 {
		out.Fatal = true
		out.Error = fmt.Errorf("all chunk analyses failed (%d)", chunkRun.Failed)
		out.Pipeline = failedPipelineOutcome(pack, chunkSet, ChunkSynthesisStatusNotStarted, out.Error.Error())
		out.Pipeline.ChunkAnalysisStatus = ChunkAnalysisStatusFailed
		out.Pipeline.ChunkPipelineStatus = ChunkPipelineStatusFailed
		return out
	}

	manifestSummary := extractManifestSummary(chunkSet)
	limitations := pack.DataQuality.Limitations
	if chunkRun.AnalysisStatus == ChunkAnalysisStatusPartial {
		limitations = appendUniqueStrings(limitations, sharedChunkAnalysisPartialLimitation)
	}
	signalsCopy := cloneDashboardChunkSignals(chunkRun.ValidatedSignals)
	synthesisInput := buildDashboardSynthesisInput(build, manifestSummary, limitations, signalsCopy, pack, opts)
	synthInputApprox, estErr := estimateSynthesisInputTokens(synthesisInput)
	if estErr != nil {
		out.Fatal = true
		out.Error = &NarrativePipelineError{
			Code: NarrativeErrorDashboardInputTokenEstFailed,
			Err:  fmt.Errorf("estimate synthesis input tokens: %w", estErr),
		}
		out.Pipeline = failedPipelineOutcome(pack, chunkSet, ChunkSynthesisStatusNotStarted, out.Error.Error())
		return out
	}
	tokenLimit := maxDashboardSynthesisInputTokens()
	if synthInputApprox > tokenLimit {
		out.Fatal = true
		out.Error = &NarrativePipelineError{
			Code: NarrativeErrorDashboardInputTooLarge,
			Err:  fmt.Errorf("dashboard synthesis input estimated_tokens=%d limit=%d", synthInputApprox, tokenLimit),
		}
		out.Pipeline = failedPipelineOutcome(pack, chunkSet, ChunkSynthesisStatusNotStarted, out.Error.Error())
		return out
	}

	maxSynthTokens := synthesisMaxOutputTokens
	if dashboardAIOnlyEnabled() {
		maxSynthTokens = synthesisMaxOutputTokensAIOnly
	}
	synthStart := time.Now()
	synthOut, err := o.AI.GenerateStructuredJSON(ctx, recommendations.GenerateStructuredJSONInput{
		SystemPrompt:    systemPromptDashboardSynthesis(),
		UserPrompt:      userPromptDashboardSynthesis(),
		Context:         synthesisInput,
		MaxOutputTokens: maxSynthTokens,
	})
	synthLatency := time.Since(synthStart).Milliseconds()
	if err != nil {
		out.Fatal = true
		out.Error = fmt.Errorf("synthesis openai: %w", err)
		out.Pipeline = buildChunkPipelineOutcome(pack, chunkSet, chunkRun, ChunkSynthesisStatusOpenAIFailed, "", "")
		out.Pipeline.ChunkPipelineStatus = ChunkPipelineStatusFailed
		return out
	}
	out.SynthesisInputTokens = synthOut.InputTokens
	out.SynthesisOutputTokens = synthOut.OutputTokens
	out.SynthesisRequestID = synthOut.RequestID
	out.SynthesisLatencyMs = synthLatency
	out.SynthesisModel = synthOut.Model
	out.RawSynthesisResponse = synthOut.Content
	out.SynthesisProviderStatus = synthOut.ResponseStatus
	out.SynthesisFinishReason = synthOut.FinishReason
	out.SynthesisIncompleteReason = synthOut.IncompleteReason
	out.SynthesisRequestedMaxOutputTokens = maxSynthTokens
	out.SynthesisOutputTruncated = synthOut.ResponseStatus == "incomplete" || synthOut.IncompleteReason == "max_output_tokens"

	parseResult := parseDashboardSynthesisPayloadDetailed(synthOut.Content)
	if o.Debug != nil && o.Debug.Enabled() {
		o.Debug.RecordSynthesis(synthesisInput, synthInputApprox, synthOut.Content, parseResult, nil, nil, parseResult.Payload)
	}
	if parseResult.ParseError != nil {
		out.Fatal = true
		out.SynthesisParseError = parseResult.ParseError.Error()
		out.Error = fmt.Errorf("parse synthesis json: %w", parseResult.ParseError)
		out.Pipeline = buildChunkPipelineOutcome(pack, chunkSet, chunkRun, ChunkSynthesisStatusParseFailed, parseResult.ParseError.Error(), "")
		out.Pipeline.ChunkPipelineStatus = ChunkPipelineStatusFailed
		return out
	}
	payload := parseResult.Payload

	accepted, repaired, repair, acceptanceValidation := acceptGeneratedDashboardNarrativeWithValidation(payload, build)
	if !accepted {
		out.Fatal = true
		out.Error = fmt.Errorf("narrative acceptance failed: %s", strings.Join(acceptanceValidation.BlockingCodes(), ", "))
		out.Validation = acceptanceValidation
		out.Pipeline = buildChunkPipelineOutcome(pack, chunkSet, chunkRun, ChunkSynthesisStatusValidationFailed, "", "")
		out.Pipeline.ChunkPipelineStatus = ChunkPipelineStatusFailed
		return out
	}

	if dashboardAIOnlyEnabled() {
		mode := CurrentDashboardValidationMode()
		blockValidation := evaluateStage4BlockValidation(repaired, mode)
		crossValidation := evaluateStage4CrossBlockReview(repaired, mode)
		out.Validation = mergeDashboardValidationResults(acceptanceValidation, blockValidation, crossValidation)
		slog.Info(dashboardValidationLogMessage(crossValidation, "stage4 cross-block review"),
			"dashboard_validation_mode", mode,
			"warning_count", len(out.Validation.Warnings),
			"error_count", len(out.Validation.BlockingErrors),
		)
		if !out.Validation.Valid {
			out.Fatal = true
			out.Error = fmt.Errorf("%s", dashboardValidationLogMessage(out.Validation, "dashboard validation failed"))
			out.Pipeline = buildChunkPipelineOutcome(pack, chunkSet, chunkRun, ChunkSynthesisStatusValidationFailed, "", "")
			out.Pipeline.ChunkPipelineStatus = ChunkPipelineStatusFailed
			return out
		}
	} else {
		out.Validation = acceptanceValidation
	}

	if !dashboardAIOnlyEnabled() {
		synthValidation := ValidateDashboardSynthesis(repaired, signalsCopy, pack, build)
		mode := CurrentDashboardValidationMode()
		synthClassified := classifyBlockingCodes(synthValidation.HardErrors, mode, "synthesis_validation")
		out.Validation = mergeDashboardValidationResults(out.Validation, synthClassified)
		if !synthClassified.Valid {
			out.Fatal = true
			out.Error = fmt.Errorf("synthesis validation failed: %s", strings.Join(synthClassified.BlockingCodes(), ", "))
			out.Pipeline = buildChunkPipelineOutcome(pack, chunkSet, chunkRun, ChunkSynthesisStatusValidationFailed, "", "")
			out.Pipeline.ChunkPipelineStatus = ChunkPipelineStatusFailed
			if o.Debug != nil && o.Debug.Enabled() {
				o.Debug.RecordSynthesis(synthesisInput, synthInputApprox, synthOut.Content, parseResult, synthValidation.HardErrors, synthValidation.RepairableErrors, repaired)
			}
			return out
		}
		if len(synthClassified.Warnings) > 0 {
			slog.Info(dashboardValidationLogMessage(synthClassified, "synthesis validation"),
				"dashboard_validation_mode", mode,
				"warning_count", len(synthClassified.Warnings),
			)
		}
	}

	out.Payload = repaired
	out.Pipeline = buildChunkPipelineOutcome(pack, chunkSet, chunkRun, ChunkSynthesisStatusCompleted, "", FinalPayloadSourceDatapackChunksSynthesis)
	if o.Debug != nil && o.Debug.Enabled() {
		o.Debug.RecordSynthesis(synthesisInput, synthInputApprox, synthOut.Content, parseResult, nil, repair.RepairableErrors, repaired)
	}
	return out
}

func appendUniqueStrings(dst []string, items ...string) []string {
	seen := map[string]struct{}{}
	out := append([]string(nil), dst...)
	for _, item := range out {
		seen[strings.TrimSpace(item)] = struct{}{}
	}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	if len(out) == 0 {
		return []string{}
	}
	return out
}

const sharedChunkAnalysisPartialLimitation = "Часть данных не была проанализирована из-за технической ошибки."

func failedPipelineOutcome(pack *aidatapack.SellerAIDataPack, chunkSet *aidatapack.ChunkSet, synthStatus, reason string) ChunkPipelineOutcome {
	out := buildChunkPipelineOutcome(pack, chunkSet, ChunkAnalysisRunResult{}, synthStatus, reason, "")
	out.ChunkPipelineStatus = ChunkPipelineStatusFailed
	return out
}

func (o *DataPackChunksOrchestrator) AnalyzeChunksOnly(
	ctx context.Context,
	pack *aidatapack.SellerAIDataPack,
	chunkSet *aidatapack.ChunkSet,
) (ChunkAnalysisRunResult, error) {
	if err := aidatapack.ValidateChunkSet(pack, chunkSet); err != nil {
		return ChunkAnalysisRunResult{}, err
	}
	return o.analyzeAllChunks(ctx, pack, chunkSet), nil
}

func (o *DataPackChunksOrchestrator) analyzeAllChunks(ctx context.Context, pack *aidatapack.SellerAIDataPack, chunkSet *aidatapack.ChunkSet) ChunkAnalysisRunResult {
	result := ChunkAnalysisRunResult{
		SellerAccountID: pack.SellerContext.SellerAccountID,
		MetricDate:      pack.SellerContext.MetricDate,
		ChunkCount:      len(chunkSet.Chunks),
		AnalysisStatus:  ChunkAnalysisStatusCompleted,
	}
	for _, ch := range chunkSet.Chunks {
		result.ChunkHashes = append(result.ChunkHashes, ch.ChunkHash)
		analysis, raw, repaired, providerOut, err := o.analyzeOneChunk(ctx, ch)
		result.InputTokens += providerOut.InputTokens
		result.OutputTokens += providerOut.OutputTokens
		callUsage := ChunkCallUsage{
			ChunkID:                  ch.ChunkID,
			ChunkType:                ch.ChunkType,
			Model:                    providerOut.Model,
			RequestID:                providerOut.RequestID,
			InputTokens:              providerOut.InputTokens,
			OutputTokens:             providerOut.OutputTokens,
			LatencyMs:                providerOut.LatencyMs,
			UsageSource:              providerOut.UsageSource,
			ProviderStatus:           providerOut.ProviderStatus,
			FinishReason:             providerOut.FinishReason,
			IncompleteReason:         providerOut.IncompleteReason,
			RequestedMaxOutputTokens: providerOut.RequestedMaxOutputTokens,
			OutputTruncated:          providerOut.OutputTruncated,
			RawBody:                  raw,
			ParseStatus:              providerOut.ParseStatus,
			ParseError:               providerOut.ParseError,
		}
		if err != nil {
			if providerOut.Skipped {
				callUsage.Status = ChunkCallStatusSkipped
				callUsage.Error = err.Error()
			} else {
				callUsage.Status = ChunkCallStatusFailed
				callUsage.Error = err.Error()
				result.Failed++
				result.ProviderFailedCount++
				result.FailedChunkIDs = append(result.FailedChunkIDs, ch.ChunkID)
				result.FailedChunkTypes = append(result.FailedChunkTypes, ch.ChunkType)
				if strings.Contains(strings.ToLower(err.Error()), "parse") || strings.Contains(strings.ToLower(err.Error()), "json") {
					result.ParseErrors++
				}
			}
			result.ChunkCalls = append(result.ChunkCalls, callUsage)
			slog.Warn("chunk analysis failed", "chunk_id", ch.ChunkID, "error", err.Error())
			if o.Debug != nil && o.Debug.Enabled() {
				o.Debug.RecordChunk(ch, raw, err, repaired, nil, ChunkValidationResult{HardErrors: []string{err.Error()}})
			}
			continue
		}
		result.Analyses = append(result.Analyses, *analysis)

		v := ValidateDashboardChunkAnalysis(ch, *analysis, pack)
		result.InventedFactIDsRejected += v.InventedFactRejected
		result.RejectedSignalCount += v.RejectedSignalCount
		result.RejectedCandidateCount += v.RejectedCandidateCount
		result.ChunkFactRefsReturned += providerOut.ChunkFactRefsReturned
		result.ChunkFactRefsValid += providerOut.ChunkFactRefsValid
		result.ChunkFactRefsUnknown += providerOut.ChunkFactRefsUnknown
		result.ChunkEntityRefsReturned += providerOut.ChunkEntityRefsReturned
		result.ChunkEntityRefsValid += providerOut.ChunkEntityRefsValid
		result.ChunkEntityRefsUnknown += providerOut.ChunkEntityRefsUnknown

		if v.Valid {
			callUsage.Status = ChunkCallStatusCompleted
			result.Completed++
			if analysis.AnalysisStatus == "insufficient_data" && len(v.Signals) == 0 {
				result.InsufficientDataCount++
			}
			if len(v.Signals) > 0 {
				result.ValidatedSignals = append(result.ValidatedSignals, v.Signals...)
				result.ValidatedSignalCount += len(v.Signals)
			}
			if len(v.ActionCandidates) > 0 {
				result.ValidatedActionCandidates = append(result.ValidatedActionCandidates, v.ActionCandidates...)
				result.ValidatedCandidateCount += len(v.ActionCandidates)
			}
		} else {
			callUsage.Status = ChunkCallStatusValidationFailed
			callUsage.Error = strings.Join(v.HardErrors, ", ")
			result.Failed++
			result.ValidationFailedCount++
			result.FailedChunkIDs = append(result.FailedChunkIDs, ch.ChunkID)
			result.FailedChunkTypes = append(result.FailedChunkTypes, ch.ChunkType)
			slog.Warn("chunk analysis validation failed", "chunk_id", ch.ChunkID, "errors", v.HardErrors)
		}
		result.ChunkCalls = append(result.ChunkCalls, callUsage)

		if o.Debug != nil && o.Debug.Enabled() {
			o.Debug.RecordChunk(ch, raw, nil, repaired, analysis, v)
		}
	}
	if result.Failed > 0 && result.Completed > 0 {
		result.AnalysisStatus = ChunkAnalysisStatusPartial
	} else if result.Failed > 0 {
		result.AnalysisStatus = ChunkAnalysisStatusFailed
	}
	return result
}

func (r ChunkAnalysisRunResult) validatedSignals() []DashboardChunkSignal {
	return append([]DashboardChunkSignal(nil), r.ValidatedSignals...)
}

type chunkProviderOutcome struct {
	InputTokens              int
	OutputTokens             int
	RequestID                string
	Model                    string
	LatencyMs                int64
	UsageSource              string
	Skipped                  bool
	ProviderStatus           string
	FinishReason             string
	IncompleteReason         string
	RequestedMaxOutputTokens int
	OutputTruncated          bool
	ParseStatus              string
	ParseError               string
	ChunkFactRefsReturned    int
	ChunkFactRefsValid       int
	ChunkFactRefsUnknown     int
	ChunkEntityRefsReturned  int
	ChunkEntityRefsValid     int
	ChunkEntityRefsUnknown   int
}

func chunkUsageSourceFromTokens(inputTokens, outputTokens int) string {
	if inputTokens > 0 || outputTokens > 0 {
		return "provider"
	}
	return "provider_missing"
}

func (o *DataPackChunksOrchestrator) analyzeOneChunk(ctx context.Context, chunk aidatapack.DataPackChunk) (*DashboardChunkAnalysis, string, string, chunkProviderOutcome, error) {
	outcome := chunkProviderOutcome{UsageSource: "provider_missing"}
	if chunk.ApproxTokens > 18000 {
		outcome.Skipped = true
		return nil, "", "", outcome, fmt.Errorf("chunk %s approx_tokens=%d exceeds hard limit 18000", chunk.ChunkID, chunk.ApproxTokens)
	}
	start := time.Now()
	out, err := o.AI.GenerateStructuredJSON(ctx, recommendations.GenerateStructuredJSONInput{
		SystemPrompt:    systemPromptDashboardChunkAnalysis(),
		UserPrompt:      userPromptDashboardChunkAnalysis(chunk),
		Context:         chunkAnalysisContextForAI(chunk),
		MaxOutputTokens: chunkAnalysisMaxOutputTokens,
	})
	outcome.LatencyMs = time.Since(start).Milliseconds()
	if err != nil {
		return nil, "", "", outcome, err
	}
	outcome.InputTokens = out.InputTokens
	outcome.OutputTokens = out.OutputTokens
	outcome.RequestID = out.RequestID
	outcome.Model = out.Model
	outcome.UsageSource = chunkUsageSourceFromTokens(out.InputTokens, out.OutputTokens)
	outcome.ProviderStatus = out.ResponseStatus
	outcome.FinishReason = out.FinishReason
	outcome.IncompleteReason = out.IncompleteReason
	outcome.RequestedMaxOutputTokens = chunkAnalysisMaxOutputTokens
	outcome.OutputTruncated = out.ResponseStatus == "incomplete" || out.IncompleteReason == "max_output_tokens"
	if out.IncompleteReason == "max_output_tokens" || strings.EqualFold(out.ResponseStatus, "incomplete") {
		return nil, out.Content, "", outcome, fmt.Errorf("chunk output truncated: max_output_tokens")
	}
	repaired := ""
	raw := out.Content
	analysis, err := parseDashboardChunkAnalysis(out.Content)
	if err != nil {
		repairedJSON, _, repairErr := repairRawDashboardNarrativeJSON(out.Content)
		if repairErr == nil {
			repaired = repairedJSON
			analysis, err = parseDashboardChunkAnalysis(repairedJSON)
		}
		if err != nil {
			outcome.ParseStatus = "invalid_json"
			outcome.ParseError = err.Error()
			return nil, raw, repaired, outcome, fmt.Errorf("parse chunk analysis json: %w", err)
		}
	}
	outcome.ParseStatus = "parsed"
	if analysis.ChunkID == "" {
		analysis.ChunkID = chunk.ChunkID
	}
	if analysis.ChunkType == "" {
		analysis.ChunkType = chunk.ChunkType
	}
	if analysis.SchemaVersion == "" {
		analysis.SchemaVersion = DashboardChunkAnalysisSchemaVersion
	}
	analysis.ActionCandidates = normalizeActionCandidateTechnicalFields(chunk.ChunkID, analysis.ActionCandidates)
	mapping := BuildChunkRefMapping(chunk)
	if analysis.SchemaVersion == DashboardChunkAnalysisSchemaVersionV3 || len(analysis.Signals) > 0 && hasChunkLocalRefs(*analysis) {
		factUnknown, entityUnknown := ResolveChunkAnalysisRefs(analysis, mapping)
		refDiag := BuildChunkRefDiagnostics(*analysis, factUnknown, entityUnknown)
		outcome.ChunkFactRefsReturned = refDiag.FactRefsReturned
		outcome.ChunkFactRefsValid = refDiag.FactRefsValid
		outcome.ChunkFactRefsUnknown = refDiag.FactRefsUnknown
		outcome.ChunkEntityRefsReturned = refDiag.EntityRefsReturned
		outcome.ChunkEntityRefsValid = refDiag.EntityRefsValid
		outcome.ChunkEntityRefsUnknown = refDiag.EntityRefsUnknown
	}
	return analysis, raw, repaired, outcome, nil
}

func hasChunkLocalRefs(analysis DashboardChunkAnalysis) bool {
	for _, sig := range analysis.Signals {
		if len(sig.FactRefs) > 0 || len(sig.EntityRefs) > 0 {
			return true
		}
	}
	for _, c := range analysis.ActionCandidates {
		if len(c.EvidenceFactRefs) > 0 {
			return true
		}
		for _, ent := range c.AffectedEntities {
			if ent.EntityRef != "" {
				return true
			}
		}
	}
	return false
}

func extractManifestSummary(chunkSet *aidatapack.ChunkSet) map[string]any {
	summary := map[string]any{
		"metric_date":       chunkSet.MetricDate,
		"seller_account_id": chunkSet.SellerAccountID,
		"chunk_count":       chunkSet.Manifest.ChunkCount,
		"dataset_counts":    chunkSet.Manifest.DatasetCounts,
		"limitations":       []string{},
	}
	for _, ch := range chunkSet.Chunks {
		if ch.ChunkID != "manifest_context" {
			continue
		}
		if payload, ok := ch.Payload.(map[string]any); ok {
			summary["manifest"] = payload
			if lim, ok := payload["limitations"].([]string); ok {
				summary["limitations"] = lim
			} else if limAny, ok := payload["limitations"].([]any); ok {
				var lims []string
				for _, item := range limAny {
					if s, ok := item.(string); ok {
						lims = append(lims, s)
					}
				}
				summary["limitations"] = lims
			}
		} else {
			raw, _ := json.Marshal(ch.Payload)
			var payloadMap map[string]any
			_ = json.Unmarshal(raw, &payloadMap)
			summary["manifest"] = payloadMap
		}
		break
	}
	return summary
}

// BuildChunkSetForPack builds and validates chunks for a data pack.
func BuildChunkSetForPack(pack *aidatapack.SellerAIDataPack, opts aidatapack.ChunkOptions) (*aidatapack.ChunkSet, error) {
	chunkSet, err := aidatapack.BuildChunks(pack, opts)
	if err != nil {
		return nil, err
	}
	if err := aidatapack.ValidateChunkSet(pack, chunkSet); err != nil {
		return nil, err
	}
	return chunkSet, nil
}
