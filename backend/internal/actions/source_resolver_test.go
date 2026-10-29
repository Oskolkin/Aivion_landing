package actions

import (
	"context"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysissource"
)

type stubSourceResolver struct {
	source analysissource.CurrentAnalysisSource
	err    error
}

func (s stubSourceResolver) ResolveCurrent(ctx context.Context, sellerAccountID int64) (analysissource.CurrentAnalysisSource, error) {
	return s.source, s.err
}

func TestListActions_UsesAuthoritativeSourcePublicationID(t *testing.T) {
	md := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)
	runID := int64(10)
	item := sampleActionRecord(1, 7, "a1", ActionCategoryStock, ActionPriorityCritical, ActionStatusOpen)
	item.RunID = &runID
	item.MetricDate = &md
	repo := &mockRepo{
		latestRun: &ActionGenerationRunRecord{
			ID:                10,
			SellerAccountID:   7,
			MetricDate:        &md,
			Status:            "completed",
			FinalStatus:       GenerationFinalStatusCompleted,
			AiPipelineRunID:   406,
			GlobalContextHash: "legacy:should-be-ignored",
		},
		items: []ActionItemRecord{item},
	}
	svc := NewServiceWithSourceResolver(repo, stubSourceResolver{
		source: analysissource.CurrentAnalysisSource{
			Source:        analysissource.SourceUnified,
			PublicationID: 272,
			PipelineRunID: 406,
			MetricDate:    "2026-07-08",
		},
	})
	resp, err := svc.ListActions(context.Background(), 7, ListActionsFilters{Status: "all", Limit: 50})
	if err != nil {
		t.Fatalf("ListActions: %v", err)
	}
	if resp.AnalysisSource != analysissource.SourceUnified {
		t.Fatalf("analysis_source=%q", resp.AnalysisSource)
	}
	if resp.PublicationID != 272 {
		t.Fatalf("publication_id=%d", resp.PublicationID)
	}
	if resp.PipelineRunID != 406 {
		t.Fatalf("pipeline_run_id=%d", resp.PipelineRunID)
	}
	if resp.MetricDate != "2026-07-08" {
		t.Fatalf("metric_date=%q", resp.MetricDate)
	}
	if len(resp.Actions) != 1 {
		t.Fatalf("actions=%d", len(resp.Actions))
	}
}
