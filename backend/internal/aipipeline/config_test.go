package aipipeline

import "testing"

func TestSharedChunkAnalysisEnabled_DefaultFalse(t *testing.T) {
	t.Setenv("AI_PIPELINE_SHARED_CHUNK_ANALYSIS_ENABLED", "")
	if SharedChunkAnalysisEnabled() {
		t.Fatal("expected false when env is unset")
	}
}

func TestSharedChunkAnalysisEnabled_ExplicitFalse(t *testing.T) {
	t.Setenv("AI_PIPELINE_SHARED_CHUNK_ANALYSIS_ENABLED", "false")
	if SharedChunkAnalysisEnabled() {
		t.Fatal("expected false when env=false")
	}
}

func TestSharedChunkAnalysisEnabled_ExplicitTrue(t *testing.T) {
	t.Setenv("AI_PIPELINE_SHARED_CHUNK_ANALYSIS_ENABLED", "true")
	if !SharedChunkAnalysisEnabled() {
		t.Fatal("expected true when env=true")
	}
}

func TestSharedChunkAnalysisEnabled_OneIsTrue(t *testing.T) {
	t.Setenv("AI_PIPELINE_SHARED_CHUNK_ANALYSIS_ENABLED", "1")
	if !SharedChunkAnalysisEnabled() {
		t.Fatal("expected true when env=1")
	}
}
