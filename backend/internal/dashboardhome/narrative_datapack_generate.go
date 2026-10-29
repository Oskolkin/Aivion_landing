package dashboardhome

import (
	"context"
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/recommendations"
)

type narrativeGenerateOutcome struct {
	meta        narrativeOpenAIOutput
	payload     *dashboardNarrativePayload
	repair      narrativeRepairResult
	jsonRepairs []string
	chunkRun    *ChunkAnalysisRunResult
	pipeline    *ChunkPipelineOutcome
	debug       *NarrativeDebugRecorder

	chunkSource                       string
	chunkSetSource                    string
	synthesisSignalCount              int
	synthesisInputEstTokens           int
	synthesisInputTokenLimit          int
	synthesisInputTokens              int
	synthesisOutputTokens             int
	synthesisRequestID                string
	synthesisLatencyMs                int64
	synthesisModel                    string
	synthesisRawResponse              string
	synthesisProviderStatus           string
	synthesisFinishReason             string
	synthesisIncompleteReason         string
	synthesisOutputTruncated          bool
	synthesisRequestedMaxOutputTokens int
	synthesisParseError               string
	internalChunkAnalysisCalls        int
	synthesisAICalls                  int
	chunkUsageIncluded                bool
	dashboardValidation               DashboardValidationResult
}

func (s *NarrativeService) generateDashboardNarrativePayload(
	ctx context.Context,
	sellerAccountID int64,
	metricDate time.Time,
	build dashboardNarrativeBuildResult,
	model string,
	opts NarrativeRunOptions,
) (*narrativeGenerateOutcome, error) {
	if build.ContextSource == ContextSourceSellerAIDataPackChunks {
		return s.generateViaDataPackChunks(ctx, sellerAccountID, metricDate, build, model, opts)
	}
	start := time.Now()
	out, err := s.ai.GenerateStructuredJSON(ctx, recommendations.GenerateStructuredJSONInput{
		SystemPrompt: narrativeSystemPromptForBuild(build),
		UserPrompt:   narrativeUserPromptForBuild(build),
		Context:      narrativeAIContextForBuild(build),
	})
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return nil, err
	}
	pipeline := &ChunkPipelineOutcome{
		ChunkPipelineStatus:  ChunkPipelineStatusNotUsed,
		ChunkSynthesisStatus: ChunkSynthesisStatusNotStarted,
		ChunkAnalysisStatus:  ChunkAnalysisStatusNotStarted,
		FinalPayloadSource:   FinalPayloadSourceLegacyDashboard,
	}
	return &narrativeGenerateOutcome{
		meta: narrativeOpenAIOutput{
			Content:          out.Content,
			Model:            out.Model,
			RequestID:        out.RequestID,
			InputTokens:      out.InputTokens,
			OutputTokens:     out.OutputTokens,
			LatencyMs:        latency,
			ResponseStatus:   out.ResponseStatus,
			IncompleteReason: out.IncompleteReason,
		},
		synthesisInputTokens:  out.InputTokens,
		synthesisOutputTokens: out.OutputTokens,
		synthesisRequestID:    out.RequestID,
		synthesisLatencyMs:    latency,
		synthesisModel:        out.Model,
		pipeline:              pipeline,
	}, nil
}

func (s *NarrativeService) generateViaDataPackChunks(
	ctx context.Context,
	sellerAccountID int64,
	metricDate time.Time,
	build dashboardNarrativeBuildResult,
	model string,
	opts NarrativeRunOptions,
) (*narrativeGenerateOutcome, error) {
	pack := opts.DataPackOverride
	var err error
	if pack == nil {
		pack, err = s.dataPack.Build(ctx, sellerAccountID, metricDate)
		if err != nil {
			return nil, fmt.Errorf("rebuild data pack for chunks: %w", err)
		}
	}

	debug := NewNarrativeDebugRecorder(opts.DebugOutputDir)
	orch := &DataPackChunksOrchestrator{AI: s.ai, Model: model, Debug: debug}

	outcomeMeta := func(gen DataPackChunksGenerationResult, chunkRun ChunkAnalysisRunResult, precomputed bool) *narrativeGenerateOutcome {
		meta := narrativeGenerateOutcome{
			chunkRun: &chunkRun,
			pipeline: &gen.Pipeline,
			debug:    debug,
		}
		if precomputed {
			meta.chunkSource = "precomputed"
			meta.chunkSetSource = "override"
			meta.internalChunkAnalysisCalls = 0
			meta.chunkUsageIncluded = false
		} else if opts.ChunkSetOverride != nil {
			meta.chunkSource = "standalone"
			meta.chunkSetSource = "override"
			meta.internalChunkAnalysisCalls = 1
			meta.chunkUsageIncluded = true
		} else {
			meta.chunkSource = "standalone"
			meta.chunkSetSource = "built"
			meta.internalChunkAnalysisCalls = 1
			meta.chunkUsageIncluded = true
		}
		if gen.Fatal {
			return &meta
		}
		meta.synthesisAICalls = 1
		meta.synthesisInputTokens = gen.SynthesisInputTokens
		meta.synthesisOutputTokens = gen.SynthesisOutputTokens
		meta.synthesisRequestID = gen.SynthesisRequestID
		meta.synthesisLatencyMs = gen.SynthesisLatencyMs
		meta.synthesisModel = gen.SynthesisModel
		meta.synthesisRawResponse = gen.RawSynthesisResponse
		meta.synthesisProviderStatus = gen.SynthesisProviderStatus
		meta.synthesisFinishReason = gen.SynthesisFinishReason
		meta.synthesisIncompleteReason = gen.SynthesisIncompleteReason
		meta.synthesisOutputTruncated = gen.SynthesisOutputTruncated
		meta.synthesisRequestedMaxOutputTokens = gen.SynthesisRequestedMaxOutputTokens
		meta.synthesisParseError = gen.SynthesisParseError
		if precomputed || !meta.chunkUsageIncluded {
			meta.meta = narrativeOpenAIOutput{
				Content:      gen.RawSynthesisResponse,
				Model:        model,
				RequestID:    gen.SynthesisRequestID,
				InputTokens:  gen.SynthesisInputTokens,
				OutputTokens: gen.SynthesisOutputTokens,
				LatencyMs:    gen.SynthesisLatencyMs,
			}
		} else {
			meta.meta = narrativeOpenAIOutput{
				Content:      gen.RawSynthesisResponse,
				Model:        model,
				RequestID:    gen.SynthesisRequestID,
				InputTokens:  chunkRun.InputTokens + gen.SynthesisInputTokens,
				OutputTokens: chunkRun.OutputTokens + gen.SynthesisOutputTokens,
				LatencyMs:    gen.SynthesisLatencyMs,
			}
		}
		meta.payload = gen.Payload
		meta.synthesisSignalCount = len(compactSignalsForSynthesis(chunkRun.ValidatedSignals))
		meta.dashboardValidation = gen.Validation
		return &meta
	}

	if opts.PrecomputedChunkRun != nil {
		chunkSet := opts.ChunkSetOverride
		if chunkSet == nil {
			return outcomeMeta(DataPackChunksGenerationResult{}, ChunkAnalysisRunResult{}, true),
				&NarrativePipelineError{Code: NarrativeErrorPrecomputedChunkSetMissing, Err: fmt.Errorf("chunk set override is required with precomputed chunk run")}
		}
		if err := ValidatePrecomputedChunkRun(pack, chunkSet, opts.PrecomputedChunkRun); err != nil {
			return outcomeMeta(DataPackChunksGenerationResult{ChunkRun: CloneChunkAnalysisRunResult(opts.PrecomputedChunkRun)}, *opts.PrecomputedChunkRun, true),
				&NarrativePipelineError{Code: NarrativeErrorPrecomputedChunkRunInvalid, Err: err}
		}
		chunkRun := CloneChunkAnalysisRunResult(opts.PrecomputedChunkRun)
		gen := orch.SynthesizeFromChunkRun(ctx, pack, chunkSet, chunkRun, build, opts)
		out := outcomeMeta(gen, chunkRun, true)
		if gen.Fatal {
			if debug.Enabled() {
				_ = debug.WriteRunSummary(nil, out.pipeline)
			}
			return out, gen.Error
		}
		limitations := append([]string(nil), pack.DataQuality.Limitations...)
		if chunkRun.AnalysisStatus == ChunkAnalysisStatusPartial {
			limitations = appendUniqueStrings(limitations, sharedChunkAnalysisPartialLimitation)
		}
		synthesisInput := buildDashboardSynthesisInput(
			build,
			extractManifestSummary(chunkSet),
			limitations,
			cloneDashboardChunkSignals(chunkRun.ValidatedSignals),
			pack,
			opts,
		)
		out.synthesisInputEstTokens, _ = estimateSynthesisInputTokens(synthesisInput)
		out.synthesisInputTokenLimit = maxDashboardSynthesisInputTokens()
		return out, nil
	}

	chunkSet := opts.ChunkSetOverride
	if chunkSet == nil {
		chunkSet, err = buildChunkSetForNarrative(pack, aidatapack.DefaultChunkOptions())
		if err != nil {
			return nil, fmt.Errorf("build chunks: %w", err)
		}
	}

	if opts.ChunkSetOverride != nil {
		chunkRun, analyzeErr := orch.AnalyzeChunksOnly(ctx, pack, chunkSet)
		if analyzeErr != nil {
			return nil, fmt.Errorf("analyze chunks: %w", analyzeErr)
		}
		gen := orch.SynthesizeFromChunkRun(ctx, pack, chunkSet, chunkRun, build, opts)
		out := outcomeMeta(gen, chunkRun, false)
		if gen.Fatal {
			if debug.Enabled() {
				_ = debug.WriteRunSummary(nil, out.pipeline)
			}
			return out, gen.Error
		}
		return out, nil
	}

	gen := orch.Generate(ctx, pack, chunkSet, build, opts)
	chunkRun := gen.ChunkRun
	out := outcomeMeta(gen, chunkRun, false)
	if gen.Fatal {
		if debug.Enabled() {
			_ = debug.WriteRunSummary(nil, out.pipeline)
		}
		return out, fmt.Errorf("datapack chunks pipeline failed: %w", gen.Error)
	}
	return out, nil
}

func applyChunkStatsToResult(result *NarrativeGenerationResult, outcome *narrativeGenerateOutcome) {
	if result == nil || outcome == nil || outcome.chunkRun == nil {
		return
	}
	cr := outcome.chunkRun
	result.ChunkCount = cr.ChunkCount
	result.ChunkAnalysisCompleted = cr.Completed
	result.ChunkAnalysisFailed = cr.Failed
	result.ValidatedSignalCount = cr.ValidatedSignalCount
	result.RejectedSignalCount = cr.RejectedSignalCount
}

func enrichGenerationResultFromOutcome(result *NarrativeGenerationResult, build *dashboardNarrativeBuildResult, outcome *narrativeGenerateOutcome) {
	if result == nil {
		return
	}
	applyChunkStatsToResult(result, outcome)
	if outcome != nil && outcome.pipeline != nil {
		applyPipelineToResult(result, outcome.pipeline)
	} else if build != nil {
		result.DataPackFactCount = len(build.DataPackFactIDs)
		result.FullDataPackFactCount = len(build.DataPackFactIDs)
	}
	applyDashboardRunDiagnostics(result, outcome)
}

func applyDashboardRunDiagnostics(result *NarrativeGenerationResult, outcome *narrativeGenerateOutcome) {
	if result == nil || outcome == nil {
		return
	}
	result.DashboardChunkSource = outcome.chunkSource
	result.DashboardChunkSetSource = outcome.chunkSetSource
	result.DashboardSynthesisSignalCount = outcome.synthesisSignalCount
	result.DashboardSynthesisInputEstTokens = outcome.synthesisInputEstTokens
	result.DashboardSynthesisInputTokenLimit = outcome.synthesisInputTokenLimit
	result.DashboardSynthesisAICalls = outcome.synthesisAICalls
	result.DashboardInternalChunkAnalysisCalls = outcome.internalChunkAnalysisCalls
	result.DashboardChunkUsageIncluded = outcome.chunkUsageIncluded
	result.DashboardSynthesisInputTokens = outcome.synthesisInputTokens
	result.DashboardSynthesisOutputTokens = outcome.synthesisOutputTokens
	result.DashboardSynthesisRequestID = outcome.synthesisRequestID
	result.DashboardSynthesisLatencyMs = outcome.synthesisLatencyMs
	result.DashboardSynthesisModel = outcome.synthesisModel
	result.DashboardSynthesisRawResponse = outcome.synthesisRawResponse
	result.DashboardSynthesisProviderStatus = outcome.synthesisProviderStatus
	result.DashboardSynthesisFinishReason = outcome.synthesisFinishReason
	result.DashboardSynthesisIncompleteReason = outcome.synthesisIncompleteReason
	result.DashboardSynthesisOutputTruncated = outcome.synthesisOutputTruncated
	result.DashboardSynthesisRequestedMaxOutputTokens = outcome.synthesisRequestedMaxOutputTokens
	if outcome.synthesisParseError != "" {
		result.SynthesisParseError = outcome.synthesisParseError
	}
	if outcome.chunkRun != nil {
		result.DashboardPrecomputedSignalCount = len(outcome.chunkRun.ValidatedSignals)
		result.DashboardPrecomputedCandidateCount = len(outcome.chunkRun.ValidatedActionCandidates)
	}
	if outcome.dashboardValidation.Mode != "" || len(outcome.dashboardValidation.Warnings) > 0 || len(outcome.dashboardValidation.BlockingErrors) > 0 {
		applyDashboardValidationToResult(result, outcome.dashboardValidation)
	}
}
