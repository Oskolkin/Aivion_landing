package aipipeline_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aipipeline"
)

func TestRunPipelineOfflineFakeDashboard(t *testing.T) {
	t.Setenv("SELLER_AI_PIPELINE_ENABLED", "1")
	t.Setenv("ACTIONS_AI_FIRST", "1")

	pack := testSamplePack()
	narrativeStore := &recordingFakeDashboardStore{}
	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: pack},
		aipipeline.NewFakeDashboardRunner(narrativeStore),
		&fakeActionsRunner{},
		&fakeStore{},
		aipipeline.NewDeterministicGlobalContextFakeClient(),
	)
	svc.SetSnapshotPublisher(testSnapshotPublisher{})

	res, err := svc.Run(context.Background(), aipipeline.RunSellerAIPipelineRequest{
		SellerAccountID:   1,
		MetricDate:        time.Date(2026, 6, 24, 0, 0, 0, 0, time.UTC),
		GenerateDashboard: true,
		GenerateActions:   true,
		AIEnabled:         false,
		Trigger:           "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.DashboardStatus != aipipeline.StepStatusCompleted {
		t.Fatalf("expected dashboard completed, got %s", res.DashboardStatus)
	}
	if res.DashboardRunID == "" {
		t.Fatal("expected dashboard_run_id")
	}
	if narrativeStore.params == nil {
		t.Fatal("expected fake dashboard persistence")
	}
	if narrativeStore.params.DataPackHash == "" || narrativeStore.params.GlobalContextHash == "" {
		t.Fatalf("expected hashes on narrative persist, got %+v", narrativeStore.params)
	}
	if res.GlobalContextHash != narrativeStore.params.GlobalContextHash {
		t.Fatalf("global context hash mismatch: %s vs %s", res.GlobalContextHash, narrativeStore.params.GlobalContextHash)
	}
}

func TestPipelineDebugArtifacts(t *testing.T) {
	t.Setenv("SELLER_AI_PIPELINE_ENABLED", "1")
	t.Setenv("ACTIONS_AI_FIRST", "1")

	pack := testSamplePack()
	store := &recordingArtifactStore{}
	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: pack},
		&fakeDashboardRunner{},
		&fakeActionsRunner{},
		store,
		aipipeline.NewDeterministicGlobalContextFakeClient(),
	)
	svc.SetSnapshotPublisher(testSnapshotPublisher{})

	_, err := svc.Run(context.Background(), aipipeline.RunSellerAIPipelineRequest{
		SellerAccountID:   1,
		MetricDate:        time.Date(2026, 6, 24, 0, 0, 0, 0, time.UTC),
		GenerateDashboard: true,
		GenerateActions:   true,
		AIEnabled:         false,
		DebugArtifacts:    true,
	})
	if err != nil {
		t.Fatal(err)
	}

	wantKinds := []string{
		aipipeline.ArtifactKindDataPack,
		aipipeline.ArtifactKindMetricSemanticsSnapshot,
		aipipeline.ArtifactKindGlobalContextInput,
		aipipeline.ArtifactKindGlobalContextPrompt,
		aipipeline.ArtifactKindGlobalContextResponse,
		aipipeline.ArtifactKindGlobalContextRawOutput,
		aipipeline.ArtifactKindGlobalContextValidation,
		aipipeline.ArtifactKindGlobalContextFinal,
		aipipeline.ArtifactKindDashboardInput,
		aipipeline.ArtifactKindDashboardOutput,
		aipipeline.ArtifactKindDashboardRawOutput,
		aipipeline.ArtifactKindDashboardValidation,
		aipipeline.ArtifactKindActionsInput,
		aipipeline.ArtifactKindActionsOutput,
		aipipeline.ArtifactKindActionsRawOutput,
		aipipeline.ArtifactKindActionsValidation,
		aipipeline.ArtifactKindPublicationGuardDecision,
		aipipeline.ArtifactKindFinalOutputSemanticValidation,
		aipipeline.ArtifactKindActionsExecutionDiagnostics,
		aipipeline.ArtifactKindPipelineSummary,
	}
	got := store.kinds()
	for _, kind := range wantKinds {
		if !got[kind] {
			t.Fatalf("missing artifact kind %s, got %+v", kind, got)
		}
	}
	for _, art := range store.artifacts {
		doc, err := decodeArtifactDocument(art.Payload)
		if err != nil {
			t.Fatalf("artifact %s decode: %v", art.Kind, err)
		}
		if doc.Meta == nil || doc.Payload == nil {
			t.Fatalf("artifact %s missing meta/payload", art.Kind)
		}
	}
}

func TestPipelineDebugArtifactsDisabled(t *testing.T) {
	t.Setenv("SELLER_AI_PIPELINE_ENABLED", "1")
	t.Setenv("ACTIONS_AI_FIRST", "1")

	store := &recordingArtifactStore{}
	svc := aipipeline.NewSellerAIPipelineService(
		&fakeDataPackBuilder{pack: testSamplePack()},
		&fakeDashboardRunner{},
		&fakeActionsRunner{},
		store,
		aipipeline.NewDeterministicGlobalContextFakeClient(),
	)
	svc.SetSnapshotPublisher(testSnapshotPublisher{})
	_, err := svc.Run(context.Background(), aipipeline.RunSellerAIPipelineRequest{
		SellerAccountID:   1,
		MetricDate:        time.Date(2026, 6, 24, 0, 0, 0, 0, time.UTC),
		GenerateDashboard: true,
		GenerateActions:   true,
		AIEnabled:         false,
		DebugArtifacts:    false,
	})
	if err != nil {
		t.Fatal(err)
	}
	required := []string{
		aipipeline.ArtifactKindDataPack,
		aipipeline.ArtifactKindMetricSemanticsSnapshot,
		aipipeline.ArtifactKindGlobalContextFinal,
		aipipeline.ArtifactKindGlobalContextValidation,
		aipipeline.ArtifactKindDashboardValidation,
		aipipeline.ArtifactKindPublicationGuardDecision,
	}
	kinds := map[string]bool{}
	for _, art := range store.artifacts {
		kinds[art.Kind] = true
	}
	for _, kind := range required {
		if !kinds[kind] {
			t.Fatalf("missing required kind %s, kinds=%v", kind, kinds)
		}
	}
	debugOnly := []string{
		aipipeline.ArtifactKindFinalOutputSemanticValidation,
		aipipeline.ArtifactKindActionsExecutionDiagnostics,
		aipipeline.ArtifactKindSharedChunkRefMapping,
	}
	for _, kind := range debugOnly {
		if kinds[kind] {
			t.Fatalf("debug-only kind %s saved without debug flag", kind)
		}
	}
}

func TestSaveRequiredDocumentIgnoresDebugFlag(t *testing.T) {
	store := &recordingArtifactStore{}
	rec := aipipeline.NewArtifactRecorder(store, 42, false)
	meta := map[string]any{"pipeline_run_id": int64(42)}
	payload := map[string]any{"field": "value", "api_key": "secret"}
	if err := rec.SaveRequiredDocument(context.Background(), aipipeline.ArtifactKindSharedChunkAnalysis, meta, payload); err != nil {
		t.Fatal(err)
	}
	if len(store.artifacts) != 1 {
		t.Fatalf("artifacts=%d", len(store.artifacts))
	}
	art := store.artifacts[0]
	if art.Kind != aipipeline.ArtifactKindSharedChunkAnalysis {
		t.Fatalf("kind=%s", art.Kind)
	}
	if art.Hash == "" {
		t.Fatal("expected hash")
	}
	doc, err := decodeArtifactDocument(art.Payload)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Meta["pipeline_run_id"] != float64(42) {
		t.Fatalf("meta=%v", doc.Meta)
	}
	payloadMap, _ := doc.Payload.(map[string]any)
	if payloadMap["api_key"] != "[redacted]" {
		t.Fatalf("payload=%v", doc.Payload)
	}
}

func TestSaveDocumentStillRespectsDebugFlag(t *testing.T) {
	store := &recordingArtifactStore{}
	rec := aipipeline.NewArtifactRecorder(store, 1, false)
	if err := rec.SaveDocument(context.Background(), aipipeline.ArtifactKindDataPack, map[string]any{"pipeline_run_id": int64(1)}, map[string]any{"x": 1}); err != nil {
		t.Fatal(err)
	}
	if len(store.artifacts) != 0 {
		t.Fatalf("expected no save, got %d", len(store.artifacts))
	}
}

func TestSaveRequiredDocumentReturnsStorageError(t *testing.T) {
	store := &recordingArtifactStore{}
	store.createErr = errors.New("storage down")
	rec := aipipeline.NewArtifactRecorder(store, 1, false)
	err := rec.SaveRequiredDocument(context.Background(), aipipeline.ArtifactKindSharedChunkAnalysis, map[string]any{"pipeline_run_id": int64(1)}, map[string]any{"x": 1})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestArtifactHashUsesPayload(t *testing.T) {
	meta := map[string]any{
		"pipeline_run_id":     int64(3),
		"data_pack_hash":      "sha256:abc",
		"global_context_hash": "sha256:ctx",
	}
	docA := aipipeline.ArtifactDocument{
		Meta:    meta,
		Payload: map[string]any{"field": "one"},
	}
	docB := aipipeline.ArtifactDocument{
		Meta:    meta,
		Payload: map[string]any{"field": "two"},
	}
	hashA, err := aipipeline.HashArtifactPayload(docA)
	if err != nil {
		t.Fatal(err)
	}
	hashB, err := aipipeline.HashArtifactPayload(docB)
	if err != nil {
		t.Fatal(err)
	}
	if hashA == hashB {
		t.Fatal("expected different hashes for different payloads")
	}
	hashA2, err := aipipeline.HashArtifactPayload(docA)
	if err != nil {
		t.Fatal(err)
	}
	if hashA != hashA2 {
		t.Fatal("expected stable hash for same payload")
	}
	if hashA == meta["global_context_hash"] {
		t.Fatal("artifact hash must not equal global_context_hash when payload differs")
	}
}

type recordingFakeDashboardStore struct {
	params *aipipeline.PersistFakeDashboardParams
}

func (s *recordingFakeDashboardStore) PersistFakeDashboardNarrative(ctx context.Context, params aipipeline.PersistFakeDashboardParams) (string, error) {
	cp := params
	s.params = &cp
	return "test-dashboard-uuid", nil
}

type recordedArtifact struct {
	Kind    string
	Hash    string
	Payload any
}

type recordingArtifactStore struct {
	fakeStore
	artifacts []recordedArtifact
	createErr error
}

func (s *recordingArtifactStore) CreateArtifact(ctx context.Context, params aipipeline.CreateArtifactParams) (int64, error) {
	if s.createErr != nil {
		return 0, s.createErr
	}
	s.artifacts = append(s.artifacts, recordedArtifact{
		Kind:    params.Kind,
		Hash:    params.Hash,
		Payload: params.Payload,
	})
	return int64(len(s.artifacts)), nil
}

func (s *recordingArtifactStore) kinds() map[string]bool {
	out := make(map[string]bool, len(s.artifacts))
	for _, art := range s.artifacts {
		out[art.Kind] = true
	}
	return out
}

type artifactDocument struct {
	Meta    map[string]any `json:"meta"`
	Payload any            `json:"payload"`
}

func decodeArtifactDocument(payload any) (artifactDocument, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return artifactDocument{}, err
	}
	var doc artifactDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		return artifactDocument{}, err
	}
	return doc, nil
}
