package dashboardhome

import (
	"context"
	"strings"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
)

func TestNarrativeService_AnalyzeDataPackChunks_NilPack(t *testing.T) {
	svc := &NarrativeService{}
	_, err := svc.AnalyzeDataPackChunks(context.Background(), nil, &aidatapack.ChunkSet{}, NarrativeRunOptions{})
	if err == nil || !strings.Contains(err.Error(), "seller AI data pack is nil") {
		t.Fatalf("err=%v", err)
	}
}

func TestNarrativeService_AnalyzeDataPackChunks_NilChunkSet(t *testing.T) {
	svc := &NarrativeService{}
	pack := minimalChunkValidationPack()
	_, err := svc.AnalyzeDataPackChunks(context.Background(), pack, nil, NarrativeRunOptions{})
	if err == nil || !strings.Contains(err.Error(), "chunk set is nil") {
		t.Fatalf("err=%v", err)
	}
}

func TestNarrativeService_AnalyzeDataPackChunks_InvalidChunkSetBeforeAI(t *testing.T) {
	svc := &NarrativeService{}
	pack := minimalChunkValidationPack()
	chunkSet := &aidatapack.ChunkSet{Chunks: []aidatapack.DataPackChunk{testChunkForCandidateValidation()}}
	_, err := svc.AnalyzeDataPackChunks(context.Background(), pack, chunkSet, NarrativeRunOptions{})
	if err == nil {
		t.Fatal("expected chunk set validation error")
	}
	if !strings.Contains(err.Error(), "analyze data pack chunks") {
		t.Fatalf("unexpected error: %v", err)
	}
}
