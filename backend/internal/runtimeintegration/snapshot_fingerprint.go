//go:build integration

package runtimeintegration

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/publishedsnapshot"
)

type sellerFacingFingerprint struct {
	PipelineRunID           int64
	DashboardDataSnapshotID int64
	DashboardRunID          string
	ActionsRunID            int64
	DataPackHash            string
	GlobalContextHash       string
	DashboardPayloadHash    string
	DashboardNarrativeHash  string
	ActionTitle             string
	EntityGroupKeys         []string
}

func loadSellerFacingFingerprint(t *testing.T, ctx context.Context, h *Harness, sellerID int64) sellerFacingFingerprint {
	t.Helper()
	snapshot, err := publishedsnapshot.NewSQLCResolver(h.Queries).GetCurrentPublishedSnapshot(ctx, sellerID)
	if err != nil {
		t.Fatalf("current snapshot: %v", err)
	}
	run, err := h.Queries.GetAIPipelineRun(ctx, snapshot.PipelineRunID)
	if err != nil {
		t.Fatal(err)
	}
	dashSnap, err := h.Queries.GetAIDashboardDataSnapshotByPipelineRunID(ctx, snapshot.PipelineRunID)
	if err != nil {
		t.Fatalf("dashboard data snapshot: %v", err)
	}
	dash, err := h.Queries.GetAIDashboardNarrativeByID(ctx, dbgen.GetAIDashboardNarrativeByIDParams{
		ID:              run.DashboardRunID,
		SellerAccountID: sellerID,
	})
	if err != nil {
		t.Fatal(err)
	}
	items, err := h.Queries.ListActionItemsByRunID(ctx, run.ActionsRunID)
	if err != nil || len(items) == 0 {
		t.Fatalf("actions items: %v", err)
	}
	groupRows, err := h.Pool.Query(ctx, `SELECT group_key FROM action_entity_groups WHERE action_generation_run_id = $1 ORDER BY group_key`, run.ActionsRunID.Int64)
	if err != nil {
		t.Fatalf("entity groups: %v", err)
	}
	defer groupRows.Close()
	keys := make([]string, 0)
	for groupRows.Next() {
		var key string
		if err := groupRows.Scan(&key); err != nil {
			t.Fatal(err)
		}
		keys = append(keys, key)
	}
	if err := groupRows.Err(); err != nil {
		t.Fatal(err)
	}
	return sellerFacingFingerprint{
		PipelineRunID:           snapshot.PipelineRunID,
		DashboardDataSnapshotID: dashSnap.ID,
		DashboardRunID:          snapshot.DashboardRunID,
		ActionsRunID:            snapshot.ActionsRunID,
		DataPackHash:            snapshot.DataPackHash,
		GlobalContextHash:       snapshot.GlobalContextHash,
		DashboardPayloadHash:    dashSnap.PayloadHash,
		DashboardNarrativeHash:  dash.ContextHash,
		ActionTitle:             items[0].Title,
		EntityGroupKeys:         keys,
	}
}

func assertSellerFacingFingerprintEqual(t *testing.T, before, after sellerFacingFingerprint) {
	t.Helper()
	beforeJSON, _ := json.Marshal(before)
	afterJSON, _ := json.Marshal(after)
	if string(beforeJSON) != string(afterJSON) {
		t.Fatalf("seller-facing fingerprint changed:\nbefore=%s\nafter=%s", beforeJSON, afterJSON)
	}
}
