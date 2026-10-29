package dashboardhome

import (
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicontext"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
)

const (
	FinalPayloadSourceDatapackChunksSynthesis = "datapack_chunks_synthesis"
	FinalPayloadSourceLegacyDashboard         = "legacy_dashboard_context"
	FinalPayloadSourceFallbackSafe            = "fallback_safe_payload"

	ChunkPipelineStatusNotUsed          = "not_used"
	ChunkPipelineStatusCompleted        = "completed"
	ChunkPipelineStatusCompletedPartial = "completed_partial"
	ChunkPipelineStatusFailed           = "failed"
	ChunkPipelineStatusFallback         = "fallback"

	ChunkSynthesisStatusNotStarted       = "not_started"
	ChunkSynthesisStatusCompleted        = "completed"
	ChunkSynthesisStatusParseFailed      = "parse_failed"
	ChunkSynthesisStatusValidationFailed = "validation_failed"
	ChunkSynthesisStatusOpenAIFailed     = "openai_failed"
	ChunkSynthesisStatusFallback         = "fallback"

	ChunkAnalysisStatusNotStarted = "not_started"
	ChunkAnalysisStatusCompleted  = "completed"
	ChunkAnalysisStatusPartial    = "completed_partial"
	ChunkAnalysisStatusFailed     = "failed"
)

// ChunkPipelineOutcome tracks multi-pass datapack chunk pipeline honesty.
type ChunkPipelineOutcome struct {
	UsedLegacyFallback         bool
	FinalPayloadSource         string
	ChunkPipelineStatus        string
	ChunkSynthesisStatus       string
	ChunkAnalysisStatus        string
	ChunkParseErrorsCount      int
	SynthesisParseError        string
	FallbackReason             string
	FullDataPackFactCount      int
	ChunkLocalFactCount        int
	ValidatedSignalFactCount   int
	FullEntityCount            int
	ChunkLocalEntityCount      int
	ValidatedSignalEntityCount int
	InventedFactIDsRejected    int
}

func countSignalFactIDs(signals []DashboardChunkSignal) int {
	seen := map[string]struct{}{}
	for _, sig := range signals {
		for _, id := range sig.FactIDs {
			id = strings.TrimSpace(id)
			if id != "" {
				seen[id] = struct{}{}
			}
		}
	}
	return len(seen)
}

func countSignalEntityIDs(signals []DashboardChunkSignal) int {
	seen := map[string]struct{}{}
	for _, sig := range signals {
		for _, id := range sig.EntityIDs {
			id = strings.TrimSpace(id)
			if id != "" {
				seen[id] = struct{}{}
			}
		}
	}
	return len(seen)
}

func applyDashboardValidationToResult(result *NarrativeGenerationResult, validation DashboardValidationResult) {
	if result == nil {
		return
	}
	if validation.Mode != "" {
		result.DashboardValidationMode = validation.Mode
	} else {
		result.DashboardValidationMode = CurrentDashboardValidationMode()
	}
	result.DashboardValidationStatus = validation.Status()
	result.DashboardValidationWarningCount = len(validation.Warnings)
	result.DashboardValidationErrorCount = len(validation.BlockingErrors)
	result.DashboardValidationWarningCodes = validation.WarningCodes()
	result.DashboardValidationErrorCodes = validation.BlockingCodes()
	result.DashboardStage4WarningCount = validation.Stage4WarningCount()
}

func resolveFinalStatus(repaired bool, hardErrors []string, pipeline *ChunkPipelineOutcome, validationStatus string) string {
	if pipeline != nil && pipeline.UsedLegacyFallback {
		return "fallback"
	}
	if dashboardAIOnlyEnabled() {
		if pipeline != nil && pipeline.ChunkPipelineStatus == ChunkPipelineStatusFailed {
			return "failed"
		}
		if validationStatus == "completed_with_warnings" {
			return "completed_with_warnings"
		}
		if repaired {
			return "completed_repaired"
		}
		return "completed"
	}
	if pipeline != nil && pipeline.ChunkPipelineStatus == ChunkPipelineStatusCompletedPartial {
		return "completed_partial"
	}
	if pipeline != nil && pipeline.ChunkPipelineStatus == ChunkPipelineStatusFailed {
		return "failed"
	}
	if repaired || len(hardErrors) == 0 {
		if repaired {
			return "completed_repaired"
		}
		return "completed"
	}
	return "failed"
}

// narrativeStatusForDB maps logical generation status to ai_dashboard_narratives.status CHECK constraint values.
func narrativeStatusForDB(logical string) string {
	switch logical {
	case "completed_repaired", "completed_partial", "completed_with_warnings":
		return "completed"
	case "pending", "running", "completed", "failed", "fallback":
		return logical
	default:
		return "failed"
	}
}

func countChunkLocalFacts(chunkSet *aidatapack.ChunkSet) int {
	if chunkSet == nil {
		return 0
	}
	seen := map[string]struct{}{}
	for _, ch := range chunkSet.Chunks {
		for id := range ch.LocalFactCatalog {
			seen[id] = struct{}{}
		}
	}
	return len(seen)
}

func countChunkLocalEntities(chunkSet *aidatapack.ChunkSet) int {
	if chunkSet == nil {
		return 0
	}
	seen := map[string]struct{}{}
	for _, ch := range chunkSet.Chunks {
		for id := range chunkLocalEntityIDAllowlist(ch) {
			seen[id] = struct{}{}
		}
	}
	return len(seen)
}

func countFullPackEntities(pack *aidatapack.SellerAIDataPack) int {
	if pack == nil {
		return 0
	}
	n := 2 // account:seller, funnel:account
	n += len(pack.EntityRegistry.SKUs)
	n += len(pack.EntityRegistry.Campaigns)
	n += len(pack.EntityRegistry.Categories)
	n += len(pack.EntityRegistry.Warehouses)
	n += len(pack.EntityRegistry.SKUGroups)
	return n
}

func buildChunkPipelineOutcome(
	pack *aidatapack.SellerAIDataPack,
	chunkSet *aidatapack.ChunkSet,
	chunkRun ChunkAnalysisRunResult,
	synthesisStatus string,
	synthesisParseErr string,
	finalSource string,
) ChunkPipelineOutcome {
	out := ChunkPipelineOutcome{
		FinalPayloadSource:         finalSource,
		ChunkSynthesisStatus:       synthesisStatus,
		SynthesisParseError:        synthesisParseErr,
		FullDataPackFactCount:      len(pack.FactRegistry),
		ChunkLocalFactCount:        countChunkLocalFacts(chunkSet),
		ValidatedSignalFactCount:   countSignalFactIDs(chunkRun.ValidatedSignals),
		FullEntityCount:            countFullPackEntities(pack),
		ChunkLocalEntityCount:      countChunkLocalEntities(chunkSet),
		ValidatedSignalEntityCount: countSignalEntityIDs(chunkRun.ValidatedSignals),
		InventedFactIDsRejected:    chunkRun.InventedFactIDsRejected,
		ChunkParseErrorsCount:      chunkRun.ParseErrors,
		ChunkAnalysisStatus:        chunkRun.AnalysisStatus,
	}
	switch {
	case finalSource == FinalPayloadSourceDatapackChunksSynthesis:
		out.ChunkPipelineStatus = ChunkPipelineStatusCompleted
		if chunkRun.AnalysisStatus == ChunkAnalysisStatusPartial || chunkRun.Failed > 0 {
			out.ChunkPipelineStatus = ChunkPipelineStatusCompletedPartial
		}
	case synthesisStatus == ChunkSynthesisStatusParseFailed ||
		synthesisStatus == ChunkSynthesisStatusValidationFailed ||
		synthesisStatus == ChunkSynthesisStatusOpenAIFailed:
		out.ChunkPipelineStatus = ChunkPipelineStatusFailed
	}
	return out
}

func applyPipelineToResult(result *NarrativeGenerationResult, pipeline *ChunkPipelineOutcome) {
	if result == nil || pipeline == nil {
		return
	}
	result.UsedLegacyFallback = pipeline.UsedLegacyFallback
	result.FinalPayloadSource = pipeline.FinalPayloadSource
	result.ChunkPipelineStatus = pipeline.ChunkPipelineStatus
	result.ChunkSynthesisStatus = pipeline.ChunkSynthesisStatus
	result.ChunkAnalysisStatus = pipeline.ChunkAnalysisStatus
	result.ChunkParseErrorsCount = pipeline.ChunkParseErrorsCount
	result.SynthesisParseError = pipeline.SynthesisParseError
	result.FallbackReason = pipeline.FallbackReason
	result.FullDataPackFactCount = pipeline.FullDataPackFactCount
	result.ChunkLocalFactCount = pipeline.ChunkLocalFactCount
	result.ValidatedSignalFactCount = pipeline.ValidatedSignalFactCount
	result.FullEntityCount = pipeline.FullEntityCount
	result.ChunkLocalEntityCount = pipeline.ChunkLocalEntityCount
	result.ValidatedSignalEntityCount = pipeline.ValidatedSignalEntityCount
	result.InventedFactIDsRejected = pipeline.InventedFactIDsRejected
	result.DataPackFactCount = pipeline.FullDataPackFactCount
}

// NarrativeRunOptions configures optional narrative generation behavior.
type NarrativeRunOptions struct {
	DebugOutputDir string

	MetricDate *time.Time

	DataPackOverride *aidatapack.SellerAIDataPack
	DataPackHash     string

	GlobalContext     *aicontext.GlobalSemanticContext
	GlobalContextHash string

	PipelineRunID *int64

	ForceRegenerate bool

	// ChunkSetOverride is the authoritative ChunkSet from the unified pipeline.
	// When set, Dashboard must not rebuild chunks from the Data Pack.
	ChunkSetOverride *aidatapack.ChunkSet

	// PrecomputedChunkRun is the shared chunk analysis result from the unified pipeline.
	// When set, Dashboard must not call AnalyzeChunksOnly.
	PrecomputedChunkRun *ChunkAnalysisRunResult
}
