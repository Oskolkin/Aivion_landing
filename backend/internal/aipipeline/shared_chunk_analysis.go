package aipipeline

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
)

type chunkSetBuildFunc func(
	pack *aidatapack.SellerAIDataPack,
	opts aidatapack.ChunkOptions,
) (*aidatapack.ChunkSet, error)

type SharedChunkAnalysisTokenUsage struct {
	InputTokens             int `json:"input_tokens"`
	OutputTokens            int `json:"output_tokens"`
	ParseErrors             int `json:"parse_errors,omitempty"`
	InventedFactIDsRejected int `json:"invented_fact_ids_rejected,omitempty"`
	ChunkFactRefsReturned   int `json:"chunk_fact_refs_returned,omitempty"`
	ChunkFactRefsValid      int `json:"chunk_fact_refs_valid,omitempty"`
	ChunkFactRefsUnknown    int `json:"chunk_fact_refs_unknown,omitempty"`
	ChunkEntityRefsReturned int `json:"chunk_entity_refs_returned,omitempty"`
	ChunkEntityRefsValid    int `json:"chunk_entity_refs_valid,omitempty"`
	ChunkEntityRefsUnknown  int `json:"chunk_entity_refs_unknown,omitempty"`
}

// SharedChunkAnalysisArtifact is the compact required state for shared chunk analysis.
type SharedChunkAnalysisArtifact struct {
	SchemaVersion              string                               `json:"schema_version"`
	ChunkAnalysisSchemaVersion string                               `json:"chunk_analysis_schema_version"`
	SellerAccountID            int64                                `json:"seller_account_id"`
	MetricDate                 string                               `json:"metric_date"`
	DataPackHash               string                               `json:"data_pack_hash"`
	ChunkSetSchemaVersion      string                               `json:"chunk_set_schema_version"`
	ChunkHashes                []string                             `json:"chunk_hashes"`
	AnalysisStatus             string                               `json:"analysis_status"`
	ChunkCount                 int                                  `json:"chunk_count"`
	Completed                  int                                  `json:"completed"`
	Failed                     int                                  `json:"failed"`
	ProviderFailedCount        int                                  `json:"provider_failed_count,omitempty"`
	ValidationFailedCount      int                                  `json:"validation_failed_count,omitempty"`
	InsufficientDataCount      int                                  `json:"insufficient_data_count,omitempty"`
	ValidatedSignalCount       int                                  `json:"validated_signal_count"`
	RejectedSignalCount        int                                  `json:"rejected_signal_count"`
	ValidatedCandidateCount    int                                  `json:"validated_candidate_count"`
	RejectedCandidateCount     int                                  `json:"rejected_candidate_count"`
	ValidatedSignals           []dashboardhome.DashboardChunkSignal `json:"validated_signals"`
	ValidatedActionCandidates  []actions.AIFirstActionCandidate     `json:"validated_action_candidates"`
	FailedChunkIDs             []string                             `json:"failed_chunk_ids"`
	FailedChunkTypes           []string                             `json:"failed_chunk_types"`
	Limitations                []string                             `json:"limitations,omitempty"`
	TokenUsage                 SharedChunkAnalysisTokenUsage        `json:"token_usage"`
}

var (
	errSharedChunkAnalysisEmptyResult   = errors.New("shared chunk analysis returned empty result")
	errSharedChunkAnalysisInvalidRun    = errors.New("shared chunk analysis result is invalid")
	errSharedChunkAnalysisAllFailed     = errors.New("all chunks failed during shared chunk analysis")
	errSharedChunkAnalysisNilChunkSet   = errors.New("chunk set builder returned nil without error")
	errSharedChunkAnalysisEmptyChunkSet = errors.New("chunk set has no chunks")
)

func orderedChunkHashes(chunkSet *aidatapack.ChunkSet) ([]string, error) {
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

func normalizeStringSlice(in []string) []string {
	if len(in) == 0 {
		return []string{}
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func normalizeSignals(in []dashboardhome.DashboardChunkSignal) []dashboardhome.DashboardChunkSignal {
	if len(in) == 0 {
		return []dashboardhome.DashboardChunkSignal{}
	}
	out := make([]dashboardhome.DashboardChunkSignal, len(in))
	copy(out, in)
	return out
}

func normalizeCandidates(in []actions.AIFirstActionCandidate) []actions.AIFirstActionCandidate {
	if len(in) == 0 {
		return []actions.AIFirstActionCandidate{}
	}
	out := make([]actions.AIFirstActionCandidate, len(in))
	copy(out, in)
	return out
}

func deriveSharedAnalysisStatus(completed, failed, chunkCount int) (string, error) {
	if chunkCount <= 0 {
		return "", errSharedChunkAnalysisEmptyChunkSet
	}
	if completed > 0 && failed == 0 {
		return dashboardhome.ChunkAnalysisStatusCompleted, nil
	}
	if completed > 0 && failed > 0 {
		return dashboardhome.ChunkAnalysisStatusPartial, nil
	}
	if completed == 0 && failed > 0 {
		return "", errSharedChunkAnalysisAllFailed
	}
	return "", errSharedChunkAnalysisInvalidRun
}

func validateSharedChunkAnalysisRun(
	chunkSet *aidatapack.ChunkSet,
	run *dashboardhome.ChunkAnalysisRunResult,
) error {
	if run == nil {
		return errSharedChunkAnalysisEmptyResult
	}
	if chunkSet == nil {
		return fmt.Errorf("chunk set is nil")
	}
	if run.ChunkCount != len(chunkSet.Chunks) {
		return fmt.Errorf("chunk_count mismatch: run=%d set=%d", run.ChunkCount, len(chunkSet.Chunks))
	}
	if run.Completed < 0 || run.Failed < 0 {
		return fmt.Errorf("negative completed/failed counts")
	}
	if run.Completed+run.Failed != run.ChunkCount {
		return fmt.Errorf("completed+failed=%d chunk_count=%d", run.Completed+run.Failed, run.ChunkCount)
	}
	if run.ValidatedSignalCount != len(run.ValidatedSignals) {
		return fmt.Errorf("validated_signal_count mismatch")
	}
	if run.ValidatedCandidateCount != len(run.ValidatedActionCandidates) {
		return fmt.Errorf("validated_candidate_count mismatch")
	}
	if run.RejectedSignalCount < 0 || run.RejectedCandidateCount < 0 {
		return fmt.Errorf("negative rejected counts")
	}
	if len(run.FailedChunkIDs) != run.Failed {
		return fmt.Errorf("failed_chunk_ids length mismatch")
	}
	if len(run.FailedChunkTypes) != run.Failed {
		return fmt.Errorf("failed_chunk_types length mismatch")
	}
	chunkIDs := map[string]struct{}{}
	for _, ch := range chunkSet.Chunks {
		chunkIDs[ch.ChunkID] = struct{}{}
	}
	for _, id := range run.FailedChunkIDs {
		if _, ok := chunkIDs[id]; !ok {
			return fmt.Errorf("failed chunk id %q not in chunk set", id)
		}
	}
	for _, candidate := range run.ValidatedActionCandidates {
		if _, ok := chunkIDs[candidate.ChunkID]; !ok {
			return fmt.Errorf("candidate chunk id %q not in chunk set", candidate.ChunkID)
		}
	}
	if run.InputTokens < 0 || run.OutputTokens < 0 {
		return fmt.Errorf("negative token counts")
	}
	status, err := deriveSharedAnalysisStatus(run.Completed, run.Failed, run.ChunkCount)
	if err != nil {
		return err
	}
	if run.AnalysisStatus != "" &&
		run.AnalysisStatus != status &&
		run.AnalysisStatus != dashboardhome.ChunkAnalysisStatusFailed {
		return fmt.Errorf("analysis_status %q inconsistent with counts", run.AnalysisStatus)
	}
	return nil
}

func normalizeSharedChunkRun(run *dashboardhome.ChunkAnalysisRunResult) (*dashboardhome.ChunkAnalysisRunResult, []string, error) {
	if run == nil {
		return nil, nil, errSharedChunkAnalysisEmptyResult
	}
	status, err := deriveSharedAnalysisStatus(run.Completed, run.Failed, run.ChunkCount)
	if err != nil {
		return nil, nil, err
	}
	out := *run
	out.AnalysisStatus = status
	out.ValidatedSignals = normalizeSignals(run.ValidatedSignals)
	out.ValidatedActionCandidates = normalizeCandidates(run.ValidatedActionCandidates)
	out.FailedChunkIDs = normalizeStringSlice(run.FailedChunkIDs)
	out.FailedChunkTypes = normalizeStringSlice(run.FailedChunkTypes)
	out.ChunkHashes = normalizeStringSlice(run.ChunkHashes)

	var limitations []string
	if status == dashboardhome.ChunkAnalysisStatusPartial {
		limitations = []string{sharedChunkAnalysisPartialLimitation}
	}
	return &out, limitations, nil
}

func buildSharedChunkAnalysisArtifact(
	pack *aidatapack.SellerAIDataPack,
	dataPackHash string,
	chunkSet *aidatapack.ChunkSet,
	chunkHashes []string,
	run *dashboardhome.ChunkAnalysisRunResult,
	limitations []string,
) (SharedChunkAnalysisArtifact, error) {
	if pack == nil || chunkSet == nil || run == nil {
		return SharedChunkAnalysisArtifact{}, fmt.Errorf("missing shared chunk analysis inputs")
	}
	if len(limitations) == 0 {
		limitations = []string{}
	}
	return SharedChunkAnalysisArtifact{
		SchemaVersion:              SharedChunkAnalysisArtifactSchemaVersion,
		ChunkAnalysisSchemaVersion: dashboardhome.DashboardChunkAnalysisSchemaVersion,
		SellerAccountID:            pack.SellerContext.SellerAccountID,
		MetricDate:                 pack.SellerContext.MetricDate,
		DataPackHash:               dataPackHash,
		ChunkSetSchemaVersion:      chunkSet.SchemaVersion,
		ChunkHashes:                append([]string(nil), chunkHashes...),
		AnalysisStatus:             run.AnalysisStatus,
		ChunkCount:                 run.ChunkCount,
		Completed:                  run.Completed,
		Failed:                     run.Failed,
		ProviderFailedCount:        run.ProviderFailedCount,
		ValidationFailedCount:      run.ValidationFailedCount,
		InsufficientDataCount:      run.InsufficientDataCount,
		ValidatedSignalCount:       run.ValidatedSignalCount,
		RejectedSignalCount:        run.RejectedSignalCount,
		ValidatedCandidateCount:    run.ValidatedCandidateCount,
		RejectedCandidateCount:     run.RejectedCandidateCount,
		ValidatedSignals:           normalizeSignals(run.ValidatedSignals),
		ValidatedActionCandidates:  normalizeCandidates(run.ValidatedActionCandidates),
		FailedChunkIDs:             normalizeStringSlice(run.FailedChunkIDs),
		FailedChunkTypes:           normalizeStringSlice(run.FailedChunkTypes),
		Limitations:                append([]string(nil), limitations...),
		TokenUsage: SharedChunkAnalysisTokenUsage{
			InputTokens:             run.InputTokens,
			OutputTokens:            run.OutputTokens,
			ParseErrors:             run.ParseErrors,
			InventedFactIDsRejected: run.InventedFactIDsRejected,
			ChunkFactRefsReturned:   run.ChunkFactRefsReturned,
			ChunkFactRefsValid:      run.ChunkFactRefsValid,
			ChunkFactRefsUnknown:    run.ChunkFactRefsUnknown,
			ChunkEntityRefsReturned: run.ChunkEntityRefsReturned,
			ChunkEntityRefsValid:    run.ChunkEntityRefsValid,
			ChunkEntityRefsUnknown:  run.ChunkEntityRefsUnknown,
		},
	}, nil
}

func chunkAnalysisRunFromArtifact(artifact SharedChunkAnalysisArtifact) *dashboardhome.ChunkAnalysisRunResult {
	return &dashboardhome.ChunkAnalysisRunResult{
		SellerAccountID:           artifact.SellerAccountID,
		MetricDate:                artifact.MetricDate,
		ChunkCount:                artifact.ChunkCount,
		Completed:                 artifact.Completed,
		Failed:                    artifact.Failed,
		ProviderFailedCount:       artifact.ProviderFailedCount,
		ValidationFailedCount:     artifact.ValidationFailedCount,
		InsufficientDataCount:     artifact.InsufficientDataCount,
		ValidatedSignalCount:      artifact.ValidatedSignalCount,
		RejectedSignalCount:       artifact.RejectedSignalCount,
		ValidatedCandidateCount:   artifact.ValidatedCandidateCount,
		RejectedCandidateCount:    artifact.RejectedCandidateCount,
		ValidatedSignals:          normalizeSignals(artifact.ValidatedSignals),
		ValidatedActionCandidates: normalizeCandidates(artifact.ValidatedActionCandidates),
		FailedChunkIDs:            normalizeStringSlice(artifact.FailedChunkIDs),
		FailedChunkTypes:          normalizeStringSlice(artifact.FailedChunkTypes),
		ChunkHashes:               normalizeStringSlice(artifact.ChunkHashes),
		InputTokens:               artifact.TokenUsage.InputTokens,
		OutputTokens:              artifact.TokenUsage.OutputTokens,
		ParseErrors:               artifact.TokenUsage.ParseErrors,
		InventedFactIDsRejected:   artifact.TokenUsage.InventedFactIDsRejected,
		ChunkFactRefsReturned:     artifact.TokenUsage.ChunkFactRefsReturned,
		ChunkFactRefsValid:        artifact.TokenUsage.ChunkFactRefsValid,
		ChunkFactRefsUnknown:      artifact.TokenUsage.ChunkFactRefsUnknown,
		ChunkEntityRefsReturned:   artifact.TokenUsage.ChunkEntityRefsReturned,
		ChunkEntityRefsValid:      artifact.TokenUsage.ChunkEntityRefsValid,
		ChunkEntityRefsUnknown:    artifact.TokenUsage.ChunkEntityRefsUnknown,
		AnalysisStatus:            artifact.AnalysisStatus,
		Analyses:                  []dashboardhome.DashboardChunkAnalysis{},
	}
}

func validateStoredSharedChunkAnalysisArtifact(
	artifact SharedChunkAnalysisArtifact,
	sellerAccountID int64,
	metricDate string,
	dataPackHash string,
	chunkSet *aidatapack.ChunkSet,
	expectedChunkHashes []string,
	storedHash string,
	computedHash string,
) (string, error) {
	if artifact.SchemaVersion != SharedChunkAnalysisArtifactSchemaVersion {
		return "schema_version_mismatch", nil
	}
	if artifact.ChunkAnalysisSchemaVersion != dashboardhome.DashboardChunkAnalysisSchemaVersion {
		return "chunk_analysis_schema_version_mismatch", nil
	}
	if artifact.SellerAccountID != sellerAccountID {
		return "seller_account_id_mismatch", nil
	}
	if artifact.MetricDate != metricDate {
		return "metric_date_mismatch", nil
	}
	if artifact.DataPackHash != dataPackHash {
		return "data_pack_hash_mismatch", nil
	}
	if chunkSet == nil {
		return "chunk_set_missing", nil
	}
	if artifact.ChunkSetSchemaVersion != chunkSet.SchemaVersion {
		return "chunk_set_schema_version_mismatch", nil
	}
	if !slices.Equal(artifact.ChunkHashes, expectedChunkHashes) {
		return "chunk_hashes_mismatch", nil
	}
	if storedHash != "" && computedHash != "" && storedHash != computedHash {
		return "artifact_hash_mismatch", nil
	}
	if artifact.ChunkCount != len(expectedChunkHashes) {
		return "chunk_count_mismatch", nil
	}
	if artifact.Completed+artifact.Failed != artifact.ChunkCount {
		return "chunk_counts_inconsistent", nil
	}
	if artifact.ValidatedSignalCount != len(artifact.ValidatedSignals) {
		return "validated_signal_count_mismatch", nil
	}
	if artifact.ValidatedCandidateCount != len(artifact.ValidatedActionCandidates) {
		return "validated_candidate_count_mismatch", nil
	}
	if artifact.Completed <= 0 {
		return "completed_zero", nil
	}
	switch artifact.AnalysisStatus {
	case dashboardhome.ChunkAnalysisStatusCompleted, dashboardhome.ChunkAnalysisStatusPartial:
	default:
		return "analysis_status_not_reusable", nil
	}
	chunkIDs := map[string]struct{}{}
	for _, ch := range chunkSet.Chunks {
		chunkIDs[ch.ChunkID] = struct{}{}
	}
	for _, candidate := range artifact.ValidatedActionCandidates {
		if _, ok := chunkIDs[candidate.ChunkID]; !ok {
			return "candidate_chunk_id_unknown", nil
		}
	}
	run := chunkAnalysisRunFromArtifact(artifact)
	if err := validateSharedChunkAnalysisRun(chunkSet, run); err != nil {
		return "restored_run_invalid", nil
	}
	coverage := EvaluateSharedChunkCoverage(chunkSet, run)
	if !coverage.Passed {
		return "artifact_coverage_insufficient", nil
	}
	return "", nil
}

func decodeSharedChunkAnalysisArtifactPayload(payload any) (SharedChunkAnalysisArtifact, error) {
	if payload == nil {
		return SharedChunkAnalysisArtifact{}, fmt.Errorf("artifact payload is nil")
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return SharedChunkAnalysisArtifact{}, err
	}
	var artifact SharedChunkAnalysisArtifact
	if err := json.Unmarshal(raw, &artifact); err != nil {
		return SharedChunkAnalysisArtifact{}, err
	}
	return artifact, nil
}
