package actions

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestPersistence_ReRunUpdatesItemsPreservesCheckedState(t *testing.T) {
	repo := newFakeGenerationRepo()
	metricDate := testMetricDate()
	key := BuildActionKey(metricDate.Format("2006-01-02"), ActionTypeStockReplenishment, []string{"c1"}, "g1")

	// seed checked user state via item status
	repo.items[key] = ActionItemRecord{
		ID: 1, SellerAccountID: 1, ActionKey: key,
		Type: ActionTypeStockReplenishment, Category: ActionCategoryStock,
		Priority: ActionPriorityImportant, Status: ActionStatusChecked,
		Title: "Old title", Reason: "old", NextStep: "old",
		MetricDate: &metricDate,
	}
	repo.userStates[key] = string(ActionStatusChecked)
	repo.mockRepo.items = []ActionItemRecord{repo.items[key]}

	evidence, _ := json.Marshal([]ActionEvidence{})
	ctas, _ := json.Marshal([]ActionCTA{})
	err := repo.UpsertActionItem(context.Background(), UpsertActionItemParams{
		SellerAccountID:      1,
		RunID:                2,
		ActionKey:            key,
		Type:                 ActionTypeStockReplenishment,
		Category:             ActionCategoryStock,
		Priority:             ActionPriorityCritical,
		Title:                "New title",
		Reason:               "new reason",
		NextStep:             "new step",
		EvidenceJSON:         evidence,
		CTAJSON:              ctas,
		LimitationsJSON:      []byte("[]"),
		AffectedEntitiesJSON: []byte("{}"),
		SourceCandidateIDs:   []byte(`["c1"]`),
		EvidenceFactIDs:      []byte("[]"),
		PayloadJSON:          []byte("{}"),
		MetricDate:           metricDate,
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	item := repo.items[key]
	if item.Title != "New title" {
		t.Fatalf("title not updated: %q", item.Title)
	}
	if item.Status != ActionStatusChecked {
		t.Fatalf("checked state lost: %q", item.Status)
	}
}

func TestPersistence_FailedGenerationDoesNotDeleteOldActions(t *testing.T) {
	repo := newFakeGenerationRepo()
	metricDate := testMetricDate()
	oldKey := "old.action"
	repo.items[oldKey] = ActionItemRecord{
		ID: 1, SellerAccountID: 1, ActionKey: oldKey,
		Type: ActionTypeStockReplenishment, Category: ActionCategoryStock,
		Priority: ActionPriorityWatch, Status: ActionStatusOpen,
		Title: "Keep me", Reason: "r", NextStep: "n",
		MetricDate: &metricDate,
	}

	// Simulate failed generation: no persist atomic call with stale delete
	runID, err := repo.CreateActionGenerationRun(context.Background(), 1, metricDate, GenerationRunStatusRunning)
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	_ = repo.UpdateActionGenerationRun(context.Background(), UpdateGenerationRunParams{
		RunID: runID, Status: GenerationRunStatusCompleted, FinalStatus: GenerationFinalStatusFailed,
	})

	if _, ok := repo.items[oldKey]; !ok {
		t.Fatalf("old action should remain after failed generation")
	}
}

func TestPersistence_StaleDeleteRemovesOldKeysWithinRunOnly(t *testing.T) {
	repo := newFakeGenerationRepo()
	metricDate := testMetricDate()
	runID := int64(10)
	otherRunID := int64(20)
	repo.items["keep"] = ActionItemRecord{
		ID: 1, SellerAccountID: 1, ActionKey: "keep", RunID: &runID,
		Type: ActionTypeStockReplenishment, Category: ActionCategoryStock,
		Priority: ActionPriorityWatch, Status: ActionStatusOpen,
		Title: "keep", Reason: "r", NextStep: "n", MetricDate: &metricDate,
	}
	repo.items["stale"] = ActionItemRecord{
		ID: 2, SellerAccountID: 1, ActionKey: "stale", RunID: &runID,
		Type: ActionTypeStockReplenishment, Category: ActionCategoryStock,
		Priority: ActionPriorityWatch, Status: ActionStatusOpen,
		Title: "stale", Reason: "r", NextStep: "n", MetricDate: &metricDate,
	}
	publishedKey := "published.keep"
	repo.items[publishedKey] = ActionItemRecord{
		ID: 3, SellerAccountID: 1, ActionKey: publishedKey, RunID: &otherRunID,
		Type: ActionTypeStockReplenishment, Category: ActionCategoryStock,
		Priority: ActionPriorityWatch, Status: ActionStatusOpen,
		Title: "published", Reason: "r", NextStep: "n", MetricDate: &metricDate,
	}

	err := repo.PersistGenerationAtomic(context.Background(), PersistGenerationParams{
		RunUpdate: UpdateGenerationRunParams{RunID: runID, Status: GenerationRunStatusCompleted, FinalStatus: GenerationFinalStatusCompleted},
		StaleDelete: &StaleDeleteParams{
			RunID: runID, SellerAccountID: 1, MetricDate: metricDate, KeepKeys: []string{"keep"},
		},
	})
	if err != nil {
		t.Fatalf("persist atomic: %v", err)
	}
	if _, ok := repo.items["keep"]; !ok {
		t.Fatalf("keep key removed")
	}
	if _, ok := repo.items["stale"]; ok {
		t.Fatalf("stale key not removed")
	}
	if _, ok := repo.items[publishedKey]; !ok {
		t.Fatalf("other run action must remain")
	}
}

func TestPersistence_CreateRunAndCategoryRuns(t *testing.T) {
	repo := newFakeGenerationRepo()
	metricDate := time.Date(2026, 6, 24, 0, 0, 0, 0, time.UTC)
	runID, err := repo.CreateActionGenerationRun(context.Background(), 1, metricDate, GenerationRunStatusRunning)
	if err != nil || runID == 0 {
		t.Fatalf("create run: id=%d err=%v", runID, err)
	}
	_, err = repo.CreateActionGenerationCategoryRun(context.Background(), CreateCategoryRunParams{
		RunID: runID, SellerAccountID: 1, Domain: ActionDomainStock, Status: CategoryRunStatusCompleted,
	})
	if err != nil {
		t.Fatalf("create category run: %v", err)
	}
	if len(repo.categoryRuns) != 1 {
		t.Fatalf("category runs=%d", len(repo.categoryRuns))
	}
}
