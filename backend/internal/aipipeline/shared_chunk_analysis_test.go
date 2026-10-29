package aipipeline

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
)

func TestOrderedChunkHashesPreservesBuilderOrder(t *testing.T) {
	_, set, err := PipelineChunkFixtureForTests()
	if err != nil {
		t.Fatal(err)
	}
	hashes, err := orderedChunkHashes(set)
	if err != nil {
		t.Fatal(err)
	}
	if len(hashes) != len(set.Chunks) {
		t.Fatalf("hash count=%d chunks=%d", len(hashes), len(set.Chunks))
	}
	for i, ch := range set.Chunks {
		if hashes[i] != ch.ChunkHash {
			t.Fatalf("hash order mismatch at %d", i)
		}
	}
}

func TestDeriveSharedAnalysisStatus(t *testing.T) {
	status, err := deriveSharedAnalysisStatus(2, 0, 2)
	if err != nil || status != dashboardhome.ChunkAnalysisStatusCompleted {
		t.Fatalf("completed: status=%s err=%v", status, err)
	}
	status, err = deriveSharedAnalysisStatus(1, 1, 2)
	if err != nil || status != dashboardhome.ChunkAnalysisStatusPartial {
		t.Fatalf("partial: status=%s err=%v", status, err)
	}
	if _, err := deriveSharedAnalysisStatus(0, 2, 2); err == nil {
		t.Fatal("expected all failed error")
	}
}

func TestSharedChunkArtifactRoundTrip(t *testing.T) {
	pack, set, err := PipelineChunkFixtureForTests()
	if err != nil {
		t.Fatal(err)
	}
	hashes, err := orderedChunkHashes(set)
	if err != nil {
		t.Fatal(err)
	}
	run := &dashboardhome.ChunkAnalysisRunResult{
		SellerAccountID:           1,
		MetricDate:                "2026-06-25",
		ChunkCount:                len(set.Chunks),
		Completed:                 len(set.Chunks),
		Failed:                    0,
		ValidatedSignalCount:      1,
		ValidatedSignals:          []dashboardhome.DashboardChunkSignal{{SignalID: "s1", SignalType: dashboardhome.ChunkSignalObservation, Observation: "ok"}},
		ValidatedCandidateCount:   0,
		ValidatedActionCandidates: []actions.AIFirstActionCandidate{},
		ChunkHashes:               hashes,
		InputTokens:               10,
		OutputTokens:              5,
		AnalysisStatus:            dashboardhome.ChunkAnalysisStatusCompleted,
	}
	normalized, limitations, err := normalizeSharedChunkRun(run)
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := buildSharedChunkAnalysisArtifact(pack, "hash-1", set, hashes, normalized, limitations)
	if err != nil {
		t.Fatal(err)
	}
	restored := chunkAnalysisRunFromArtifact(artifact)
	if restored.ValidatedSignalCount != 1 || restored.ChunkCount != len(set.Chunks) {
		t.Fatalf("restored=%+v", restored)
	}
	miss, _ := validateStoredSharedChunkAnalysisArtifact(
		artifact, 1, "2026-06-25", "hash-1", set, hashes, "", "",
	)
	if miss != "" {
		t.Fatalf("miss=%s", miss)
	}
}

func TestValidateStoredArtifactRejectsHashMismatch(t *testing.T) {
	pack, set, err := PipelineChunkFixtureForTests()
	if err != nil {
		t.Fatal(err)
	}
	hashes, _ := orderedChunkHashes(set)
	artifact, _ := buildSharedChunkAnalysisArtifact(pack, "hash-1", set, hashes, &dashboardhome.ChunkAnalysisRunResult{
		SellerAccountID: 1, MetricDate: "2026-06-25", ChunkCount: len(set.Chunks),
		Completed: len(set.Chunks), AnalysisStatus: dashboardhome.ChunkAnalysisStatusCompleted,
		ValidatedSignals: []dashboardhome.DashboardChunkSignal{}, ValidatedActionCandidates: []actions.AIFirstActionCandidate{},
	}, nil)
	miss, _ := validateStoredSharedChunkAnalysisArtifact(
		artifact, 1, "2026-06-25", "other-hash", set, hashes, "", "",
	)
	if miss != "data_pack_hash_mismatch" {
		t.Fatalf("miss=%s", miss)
	}
}

func TestValidateStoredArtifactMismatchTable(t *testing.T) {
	pack, set, err := PipelineChunkFixtureForTests()
	if err != nil {
		t.Fatal(err)
	}
	hashes, err := orderedChunkHashes(set)
	if err != nil {
		t.Fatal(err)
	}
	baseRun := &dashboardhome.ChunkAnalysisRunResult{
		SellerAccountID: 1, MetricDate: "2026-06-25", ChunkCount: len(set.Chunks),
		Completed: len(set.Chunks), AnalysisStatus: dashboardhome.ChunkAnalysisStatusCompleted,
		ValidatedSignals: []dashboardhome.DashboardChunkSignal{}, ValidatedActionCandidates: []actions.AIFirstActionCandidate{},
	}
	normalized, limitations, err := normalizeSharedChunkRun(baseRun)
	if err != nil {
		t.Fatal(err)
	}
	baseArtifact, err := buildSharedChunkAnalysisArtifact(pack, "hash-1", set, hashes, normalized, limitations)
	if err != nil {
		t.Fatal(err)
	}

	reordered := append([]string(nil), hashes...)
	if len(reordered) >= 2 {
		reordered[0], reordered[1] = reordered[1], reordered[0]
	}

	cases := []struct {
		name string
		mut  func(*SharedChunkAnalysisArtifact)
		want string
	}{
		{"schema", func(a *SharedChunkAnalysisArtifact) { a.SchemaVersion = "other" }, "schema_version_mismatch"},
		{"chunk_analysis_schema", func(a *SharedChunkAnalysisArtifact) { a.ChunkAnalysisSchemaVersion = "v0" }, "chunk_analysis_schema_version_mismatch"},
		{"seller", func(a *SharedChunkAnalysisArtifact) { a.SellerAccountID = 2 }, "seller_account_id_mismatch"},
		{"metric_date", func(a *SharedChunkAnalysisArtifact) { a.MetricDate = "2020-01-01" }, "metric_date_mismatch"},
		{"data_pack_hash", func(a *SharedChunkAnalysisArtifact) { a.DataPackHash = "other" }, "data_pack_hash_mismatch"},
		{"chunk_set_schema", func(a *SharedChunkAnalysisArtifact) { a.ChunkSetSchemaVersion = "other" }, "chunk_set_schema_version_mismatch"},
		{"chunk_hash", func(a *SharedChunkAnalysisArtifact) { a.ChunkHashes[0] = "bad" }, "chunk_hashes_mismatch"},
		{"hash_order", func(a *SharedChunkAnalysisArtifact) { a.ChunkHashes = reordered }, "chunk_hashes_mismatch"},
		{"chunk_count", func(a *SharedChunkAnalysisArtifact) { a.ChunkCount = a.ChunkCount + 1 }, "chunk_count_mismatch"},
		{"status_failed", func(a *SharedChunkAnalysisArtifact) { a.AnalysisStatus = dashboardhome.ChunkAnalysisStatusFailed }, "analysis_status_not_reusable"},
		{"completed_zero", func(a *SharedChunkAnalysisArtifact) { a.Completed = 0; a.Failed = len(hashes) }, "completed_zero"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			artifact := baseArtifact
			artifact.ChunkHashes = append([]string(nil), baseArtifact.ChunkHashes...)
			artifact.ValidatedSignals = append([]dashboardhome.DashboardChunkSignal(nil), baseArtifact.ValidatedSignals...)
			artifact.ValidatedActionCandidates = append([]actions.AIFirstActionCandidate(nil), baseArtifact.ValidatedActionCandidates...)
			artifact.Limitations = append([]string(nil), baseArtifact.Limitations...)
			tc.mut(&artifact)
			miss, err := validateStoredSharedChunkAnalysisArtifact(
				artifact, 1, "2026-06-25", "hash-1", set, hashes, "", "",
			)
			if err != nil {
				t.Fatal(err)
			}
			if miss != tc.want {
				t.Fatalf("miss=%q want %q", miss, tc.want)
			}
		})
	}
}

func TestDecodeSharedChunkAnalysisArtifactPayloadRejectsInvalidShape(t *testing.T) {
	_, err := decodeSharedChunkAnalysisArtifactPayload([]byte("not-json"))
	if err == nil {
		t.Fatal("expected decode error")
	}
}

func TestBuildChunkSetForPackValidatesCoverage(t *testing.T) {
	pack, _, err := PipelineChunkFixtureForTests()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dashboardhome.BuildChunkSetForPack(pack, aidatapack.DefaultChunkOptions()); err != nil {
		t.Fatalf("expected valid chunk set: %v", err)
	}
}
