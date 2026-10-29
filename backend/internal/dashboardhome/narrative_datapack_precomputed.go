package dashboardhome

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
)

const (
	NarrativeErrorPrecomputedChunkSetMissing   = "precomputed_chunk_set_missing"
	NarrativeErrorPrecomputedChunkRunInvalid   = "precomputed_chunk_run_invalid"
	NarrativeErrorDashboardInputTooLarge       = "dashboard_input_too_large"
	NarrativeErrorDashboardInputTokenEstFailed = "dashboard_input_token_estimation_failed"
)

// NarrativePipelineError is a controlled dashboard pipeline failure with a stable code.
type NarrativePipelineError struct {
	Code string
	Err  error
}

func (e *NarrativePipelineError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Code
}

func (e *NarrativePipelineError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// NarrativeErrorCode extracts a stable dashboard error code when present.
func NarrativeErrorCode(err error) string {
	var coded *NarrativePipelineError
	if errors.As(err, &coded) && coded != nil && strings.TrimSpace(coded.Code) != "" {
		return coded.Code
	}
	return ""
}

// UsesUnifiedSharedChunkFlow reports whether Dashboard should skip internal chunk analysis.
func UsesUnifiedSharedChunkFlow(opts NarrativeRunOptions) bool {
	return opts.PrecomputedChunkRun != nil
}

// UsesUnifiedDataPackContext reports whether legacy alerts/recommendations should be skipped.
// Only the unified shared-chunk path (precomputed ChunkRun) skips legacy SQL loads.
// DataPackOverride or ChunkSetOverride without PrecomputedChunkRun remains standalone fallback
// and still loads legacy alerts/recommendations.
func UsesUnifiedDataPackContext(opts NarrativeRunOptions) bool {
	return opts.PrecomputedChunkRun != nil
}

// BuildPrecomputedSynthesisNarrativeContext prepares narrative build input for precomputed chunk synthesis.
func BuildPrecomputedSynthesisNarrativeContext(pack *aidatapack.SellerAIDataPack) (dashboardNarrativeBuildResult, error) {
	build, _, _, err := buildDashboardNarrativeContextFromDataPackFull(pack, nil, dataPackNarrativeLegacyInput{})
	if err != nil {
		return dashboardNarrativeBuildResult{}, err
	}
	build.ContextSource = ContextSourceSellerAIDataPackChunks
	return build, nil
}

// CloneChunkAnalysisRunResult returns a defensive copy of a shared chunk run.
func CloneChunkAnalysisRunResult(run *ChunkAnalysisRunResult) ChunkAnalysisRunResult {
	if run == nil {
		return ChunkAnalysisRunResult{}
	}
	out := *run
	out.ValidatedSignals = cloneDashboardChunkSignals(run.ValidatedSignals)
	out.ValidatedActionCandidates = cloneActionCandidates(run.ValidatedActionCandidates)
	out.FailedChunkIDs = append([]string(nil), run.FailedChunkIDs...)
	out.FailedChunkTypes = append([]string(nil), run.FailedChunkTypes...)
	out.ChunkHashes = append([]string(nil), run.ChunkHashes...)
	out.Analyses = append([]DashboardChunkAnalysis(nil), run.Analyses...)
	out.ChunkCalls = append([]ChunkCallUsage(nil), run.ChunkCalls...)
	return out
}

func cloneDashboardChunkSignals(in []DashboardChunkSignal) []DashboardChunkSignal {
	if len(in) == 0 {
		return []DashboardChunkSignal{}
	}
	out := make([]DashboardChunkSignal, len(in))
	for i, signal := range in {
		out[i] = DashboardChunkSignal{
			SignalID:       signal.SignalID,
			SignalType:     signal.SignalType,
			Observation:    signal.Observation,
			WhyItMayMatter: signal.WhyItMayMatter,
			FactIDs:        append([]string(nil), signal.FactIDs...),
			EntityIDs:      append([]string(nil), signal.EntityIDs...),
			RelatedMetrics: append([]string(nil), signal.RelatedMetrics...),
			Limitations:    append([]string(nil), signal.Limitations...),
		}
	}
	return out
}

func cloneActionCandidates(in []actions.AIFirstActionCandidate) []actions.AIFirstActionCandidate {
	if len(in) == 0 {
		return []actions.AIFirstActionCandidate{}
	}
	out := make([]actions.AIFirstActionCandidate, len(in))
	copy(out, in)
	return out
}

// ValidatePrecomputedChunkRun performs lightweight deterministic checks before synthesis.
func ValidatePrecomputedChunkRun(
	pack *aidatapack.SellerAIDataPack,
	chunkSet *aidatapack.ChunkSet,
	chunkRun *ChunkAnalysisRunResult,
) error {
	if pack == nil {
		return fmt.Errorf("data pack is nil")
	}
	if chunkSet == nil {
		return fmt.Errorf("chunk set is nil")
	}
	if chunkRun == nil {
		return fmt.Errorf("chunk run is nil")
	}
	if chunkRun.SellerAccountID != pack.SellerContext.SellerAccountID {
		return fmt.Errorf("seller_account_id mismatch")
	}
	if chunkRun.MetricDate != pack.SellerContext.MetricDate {
		return fmt.Errorf("metric_date mismatch")
	}
	if chunkRun.ChunkCount != len(chunkSet.Chunks) {
		return fmt.Errorf("chunk_count mismatch: run=%d set=%d", chunkRun.ChunkCount, len(chunkSet.Chunks))
	}
	if chunkRun.Completed < 0 || chunkRun.Failed < 0 {
		return fmt.Errorf("negative completed/failed counts")
	}
	if chunkRun.Completed+chunkRun.Failed != chunkRun.ChunkCount {
		return fmt.Errorf("completed+failed=%d chunk_count=%d", chunkRun.Completed+chunkRun.Failed, chunkRun.ChunkCount)
	}
	if chunkRun.Completed <= 0 {
		return fmt.Errorf("completed must be > 0")
	}
	if chunkRun.ValidatedSignalCount != len(chunkRun.ValidatedSignals) {
		return fmt.Errorf("validated_signal_count mismatch")
	}
	if chunkRun.ValidatedCandidateCount != len(chunkRun.ValidatedActionCandidates) {
		return fmt.Errorf("validated_candidate_count mismatch")
	}
	if chunkRun.RejectedSignalCount < 0 || chunkRun.RejectedCandidateCount < 0 {
		return fmt.Errorf("negative rejected counts")
	}
	if len(chunkRun.FailedChunkIDs) != chunkRun.Failed {
		return fmt.Errorf("failed_chunk_ids length mismatch")
	}
	if len(chunkRun.FailedChunkTypes) != chunkRun.Failed {
		return fmt.Errorf("failed_chunk_types length mismatch")
	}
	chunkIDs := map[string]struct{}{}
	for _, ch := range chunkSet.Chunks {
		chunkIDs[ch.ChunkID] = struct{}{}
	}
	for _, id := range chunkRun.FailedChunkIDs {
		if _, ok := chunkIDs[id]; !ok {
			return fmt.Errorf("failed chunk id %q not in chunk set", id)
		}
	}
	for _, candidate := range chunkRun.ValidatedActionCandidates {
		if _, ok := chunkIDs[candidate.ChunkID]; !ok {
			return fmt.Errorf("candidate chunk id %q not in chunk set", candidate.ChunkID)
		}
	}
	if chunkRun.InputTokens < 0 || chunkRun.OutputTokens < 0 {
		return fmt.Errorf("negative token counts")
	}
	switch chunkRun.AnalysisStatus {
	case ChunkAnalysisStatusCompleted, ChunkAnalysisStatusPartial:
	default:
		return fmt.Errorf("analysis_status %q is not reusable", chunkRun.AnalysisStatus)
	}
	if len(chunkRun.ChunkHashes) > 0 {
		expected, err := orderedChunkHashesFromChunkSet(chunkSet)
		if err != nil {
			return err
		}
		if !slices.Equal(chunkRun.ChunkHashes, expected) {
			return fmt.Errorf("chunk_hashes mismatch")
		}
	}
	return nil
}

func orderedChunkHashesFromChunkSet(chunkSet *aidatapack.ChunkSet) ([]string, error) {
	if chunkSet == nil {
		return nil, fmt.Errorf("chunk set is nil")
	}
	out := make([]string, 0, len(chunkSet.Chunks))
	for _, ch := range chunkSet.Chunks {
		hash := strings.TrimSpace(ch.ChunkHash)
		if hash == "" {
			return nil, fmt.Errorf("chunk %s has empty chunk_hash", ch.ChunkID)
		}
		out = append(out, hash)
	}
	return out, nil
}
