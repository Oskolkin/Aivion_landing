package dashboardhome

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/recommendations"
)

func orchestratorCounterTestChunk(chunkID string) aidatapack.DataPackChunk {
	ch := testChunkForCandidateValidation()
	ch.ChunkID = chunkID
	return ch
}

func TestAnalyzeAllChunks_ValidSignalCountsCompleted(t *testing.T) {
	pack := minimalChunkValidationPack()
	chunk := testChunkForCandidateValidation()
	analysis := validSignalAndCandidateAnalysis(chunk.ChunkID)
	raw, _ := json.Marshal(analysis)
	fake := &fakeChunkAnalysisAI{
		fn: func(input recommendations.GenerateStructuredJSONInput) (*recommendations.GenerateRecommendationsOutput, error) {
			return &recommendations.GenerateRecommendationsOutput{Content: string(raw), InputTokens: 10, OutputTokens: 5}, nil
		},
	}
	orch := &DataPackChunksOrchestrator{AI: fake}
	result := orch.analyzeAllChunks(context.Background(), pack, &aidatapack.ChunkSet{Chunks: []aidatapack.DataPackChunk{chunk}})
	if result.Completed != 1 || result.Failed != 0 {
		t.Fatalf("completed=%d failed=%d", result.Completed, result.Failed)
	}
	if result.ChunkCount != 1 || result.Completed+result.Failed != result.ChunkCount {
		t.Fatalf("invariant broken: %+v", result)
	}
}

func TestAnalyzeAllChunks_ValidationFailureCountsFailed(t *testing.T) {
	pack := minimalChunkValidationPack()
	chunk := testChunkForCandidateValidation()
	analysis := validSignalAndCandidateAnalysis(chunk.ChunkID)
	analysis.Signals[0].FactIDs = []string{"invented.fact"}
	raw, _ := json.Marshal(analysis)
	fake := &fakeChunkAnalysisAI{
		fn: func(input recommendations.GenerateStructuredJSONInput) (*recommendations.GenerateRecommendationsOutput, error) {
			return &recommendations.GenerateRecommendationsOutput{Content: string(raw), InputTokens: 10, OutputTokens: 5}, nil
		},
	}
	orch := &DataPackChunksOrchestrator{AI: fake}
	result := orch.analyzeAllChunks(context.Background(), pack, &aidatapack.ChunkSet{Chunks: []aidatapack.DataPackChunk{chunk}})
	if result.Completed != 0 || result.Failed != 1 {
		t.Fatalf("completed=%d failed=%d", result.Completed, result.Failed)
	}
	if len(result.FailedChunkIDs) != 1 || result.FailedChunkIDs[0] != chunk.ChunkID {
		t.Fatalf("failed ids=%v", result.FailedChunkIDs)
	}
	if len(result.ChunkCalls) != 1 || result.ChunkCalls[0].Status != ChunkCallStatusValidationFailed {
		t.Fatalf("call status=%s", result.ChunkCalls[0].Status)
	}
	if result.ChunkCalls[0].InputTokens != 10 || result.ChunkCalls[0].OutputTokens != 5 {
		t.Fatalf("usage lost: %+v", result.ChunkCalls[0])
	}
	if result.ValidationFailedCount != 1 || result.ProviderFailedCount != 0 {
		t.Fatalf("provider_failed=%d validation_failed=%d", result.ProviderFailedCount, result.ValidationFailedCount)
	}
}

func TestAnalyzeAllChunks_InsufficientDataCountsCompleted(t *testing.T) {
	pack := minimalChunkValidationPack()
	chunk := testChunkForCandidateValidation()
	chunk.ChunkType = "entity_dictionary"
	raw, _ := json.Marshal(DashboardChunkAnalysis{
		SchemaVersion:  DashboardChunkAnalysisSchemaVersionV2,
		ChunkID:        chunk.ChunkID,
		ChunkType:      chunk.ChunkType,
		AnalysisStatus: "insufficient_data",
		Signals:        nil,
	})
	fake := &fakeChunkAnalysisAI{
		fn: func(input recommendations.GenerateStructuredJSONInput) (*recommendations.GenerateRecommendationsOutput, error) {
			return &recommendations.GenerateRecommendationsOutput{Content: string(raw)}, nil
		},
	}
	orch := &DataPackChunksOrchestrator{AI: fake}
	result := orch.analyzeAllChunks(context.Background(), pack, &aidatapack.ChunkSet{Chunks: []aidatapack.DataPackChunk{chunk}})
	if result.Completed != 1 || result.Failed != 0 || result.InsufficientDataCount != 1 {
		t.Fatalf("completed=%d failed=%d insufficient=%d", result.Completed, result.Failed, result.InsufficientDataCount)
	}
}

func TestAnalyzeAllChunks_CompletedWithNoValidSignalsFails(t *testing.T) {
	pack := minimalChunkValidationPack()
	chunk := testChunkForCandidateValidation()
	analysis := validSignalAndCandidateAnalysis(chunk.ChunkID)
	analysis.Signals[0].FactIDs = []string{"invented.fact"}
	analysis.AnalysisStatus = "completed"
	raw, _ := json.Marshal(analysis)
	fake := &fakeChunkAnalysisAI{
		fn: func(input recommendations.GenerateStructuredJSONInput) (*recommendations.GenerateRecommendationsOutput, error) {
			return &recommendations.GenerateRecommendationsOutput{Content: string(raw)}, nil
		},
	}
	orch := &DataPackChunksOrchestrator{AI: fake}
	result := orch.analyzeAllChunks(context.Background(), pack, &aidatapack.ChunkSet{Chunks: []aidatapack.DataPackChunk{chunk}})
	if result.Completed != 0 || result.Failed != 1 {
		t.Fatalf("completed=%d failed=%d", result.Completed, result.Failed)
	}
}

func TestAnalyzeAllChunks_PartialSignalRejectionStillUsable(t *testing.T) {
	pack := minimalChunkValidationPack()
	chunk := testChunkForCandidateValidation()
	analysis := validSignalAndCandidateAnalysis(chunk.ChunkID)
	bad := analysis.Signals[0]
	bad.SignalID = chunk.ChunkID + "-signal-02"
	bad.FactIDs = []string{"invented.fact"}
	analysis.Signals = append(analysis.Signals, bad)
	raw, _ := json.Marshal(analysis)
	fake := &fakeChunkAnalysisAI{
		fn: func(input recommendations.GenerateStructuredJSONInput) (*recommendations.GenerateRecommendationsOutput, error) {
			return &recommendations.GenerateRecommendationsOutput{Content: string(raw)}, nil
		},
	}
	orch := &DataPackChunksOrchestrator{AI: fake}
	result := orch.analyzeAllChunks(context.Background(), pack, &aidatapack.ChunkSet{Chunks: []aidatapack.DataPackChunk{chunk}})
	if result.Completed != 1 || result.Failed != 0 {
		t.Fatalf("completed=%d failed=%d", result.Completed, result.Failed)
	}
	if result.RejectedSignalCount < 1 || result.ValidatedSignalCount != 1 {
		t.Fatalf("rejected=%d validated=%d", result.RejectedSignalCount, result.ValidatedSignalCount)
	}
}

func TestAnalyzeAllChunks_AllSignalsRejectedFailsChunk(t *testing.T) {
	pack := minimalChunkValidationPack()
	chunk := testChunkForCandidateValidation()
	analysis := validSignalAndCandidateAnalysis(chunk.ChunkID)
	for i := range analysis.Signals {
		analysis.Signals[i].FactIDs = []string{"invented.fact"}
	}
	raw, _ := json.Marshal(analysis)
	fake := &fakeChunkAnalysisAI{
		fn: func(input recommendations.GenerateStructuredJSONInput) (*recommendations.GenerateRecommendationsOutput, error) {
			return &recommendations.GenerateRecommendationsOutput{Content: string(raw)}, nil
		},
	}
	orch := &DataPackChunksOrchestrator{AI: fake}
	result := orch.analyzeAllChunks(context.Background(), pack, &aidatapack.ChunkSet{Chunks: []aidatapack.DataPackChunk{chunk}})
	if result.Completed != 0 || result.Failed != 1 {
		t.Fatalf("completed=%d failed=%d", result.Completed, result.Failed)
	}
}
