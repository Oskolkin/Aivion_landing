package aipipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

type fakeInspectorStore struct {
	run       dbgen.AiPipelineRun
	artifacts []dbgen.AiPipelineArtifact
}

func (f *fakeInspectorStore) GetPipelineRun(ctx context.Context, id int64) (dbgen.AiPipelineRun, error) {
	if id != f.run.ID {
		return dbgen.AiPipelineRun{}, context.Canceled
	}
	return f.run, nil
}

func (f *fakeInspectorStore) ListPipelineArtifacts(ctx context.Context, pipelineRunID int64) ([]dbgen.AiPipelineArtifact, error) {
	return f.artifacts, nil
}

func TestExportRunInspector_CreatesHTML(t *testing.T) {
	dir := t.TempDir()
	store := &fakeInspectorStore{
		run: dbgen.AiPipelineRun{
			ID: 72, SellerAccountID: 1, Status: "completed_partial",
			DataPackHash: "abc", MetricDate: pgtype.Date{Time: time.Date(2026, 6, 24, 0, 0, 0, 0, time.UTC), Valid: true},
			CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		},
		artifacts: []dbgen.AiPipelineArtifact{
			{ID: 1, PipelineRunID: 72, Kind: ArtifactKindDataPack, PayloadJson: []byte(`{"meta":{},"payload":{"rows":1}}`)},
			{ID: 2, PipelineRunID: 72, Kind: ArtifactKindActionsOutput, PayloadJson: []byte(`{"meta":{},"payload":{"legacy":true}}`)},
		},
	}
	if err := ExportRunInspector(context.Background(), store, RunInspectorExportOptions{
		RunID: 72, OutputDir: dir, IncludeRaw: true, PrettyJSON: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "index.html")); err != nil {
		t.Fatalf("index.html: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "inspector-data.json")); err != nil {
		t.Fatalf("inspector-data.json: %v", err)
	}
	html, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(html)
	if containsHTTPURL(body) {
		t.Fatal("html must not reference external http(s) urls")
	}
}

func containsHTTPURL(s string) bool {
	for _, prefix := range []string{"https://", "http://", "cdn"} {
		if len(s) > 0 {
			for i := 0; i+len(prefix) <= len(s); i++ {
				if s[i:i+len(prefix)] == prefix {
					// allow only if inside json string values from data - conservative check on link/script tags
					before := s[max(0, i-20):i]
					if !containsAny(before, []string{"href=", "src=", "url("}) {
						continue
					}
					return true
				}
			}
		}
	}
	return false
}

func containsAny(s string, parts []string) bool {
	for _, p := range parts {
		if len(p) > 0 && len(s) >= len(p) {
			for i := 0; i+len(p) <= len(s); i++ {
				if s[i:i+len(p)] == p {
					return true
				}
			}
		}
	}
	return false
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func TestExportRunInspector_MissingRun(t *testing.T) {
	store := &fakeInspectorStore{run: dbgen.AiPipelineRun{ID: 1}}
	err := ExportRunInspector(context.Background(), store, RunInspectorExportOptions{RunID: 99, OutputDir: t.TempDir()})
	if err == nil {
		t.Fatal("expected error for missing run")
	}
}

type spyInspectorStore struct {
	fakeInspectorStore
	getCalls  int
	listCalls int
}

func (s *spyInspectorStore) GetPipelineRun(ctx context.Context, id int64) (dbgen.AiPipelineRun, error) {
	s.getCalls++
	return s.fakeInspectorStore.GetPipelineRun(ctx, id)
}

func (s *spyInspectorStore) ListPipelineArtifacts(ctx context.Context, pipelineRunID int64) ([]dbgen.AiPipelineArtifact, error) {
	s.listCalls++
	return s.fakeInspectorStore.ListPipelineArtifacts(ctx, pipelineRunID)
}

func TestExportRunInspector_ReadOnly(t *testing.T) {
	dir := t.TempDir()
	store := &spyInspectorStore{
		fakeInspectorStore: fakeInspectorStore{
			run: dbgen.AiPipelineRun{
				ID: 73, SellerAccountID: 1, Status: "completed",
				DataPackHash: "abc", CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
			},
			artifacts: []dbgen.AiPipelineArtifact{
				{ID: 1, PipelineRunID: 73, Kind: ArtifactKindDataPack, PayloadJson: []byte(`{"meta":{},"payload":{}}`)},
			},
		},
	}
	if err := ExportRunInspector(context.Background(), store, RunInspectorExportOptions{
		RunID: 73, OutputDir: dir, IncludeRaw: true, PrettyJSON: true,
	}); err != nil {
		t.Fatal(err)
	}
	if store.getCalls != 1 || store.listCalls != 1 {
		t.Fatalf("unexpected store calls get=%d list=%d", store.getCalls, store.listCalls)
	}
}
