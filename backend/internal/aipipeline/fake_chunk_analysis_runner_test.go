package aipipeline

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
)

func fakeRunnerFixture(t *testing.T) (*aidatapack.SellerAIDataPack, *aidatapack.ChunkSet) {
	t.Helper()
	metricDate := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)
	pack := &aidatapack.SellerAIDataPack{
		SchemaVersion: aidatapack.SchemaVersion,
		SellerContext: aidatapack.SellerContext{SellerAccountID: 1, MetricDate: "2026-06-25"},
		DataWindows:   aidatapack.ComputeDataWindows(metricDate),
		DataQuality:   aidatapack.DataQuality{Sources: map[string]any{}, Limitations: []string{}},
		EntityRegistry: aidatapack.EntityRegistry{
			SKUs:       map[string]aidatapack.SKUEntity{},
			SKUGroups:  map[string]aidatapack.SKUGroupEntity{},
			Campaigns:  map[string]aidatapack.CampaignEntity{},
			Warehouses: map[string]aidatapack.WarehouseEntity{},
			Categories: map[string]aidatapack.CategoryEntity{},
		},
		FactRegistry: map[string]aidatapack.Fact{
			"account.current_day.revenue": {
				FactID: "account.current_day.revenue", EntityType: "account", EntityID: "account:seller",
				Metric: "revenue", Value: 1.0, Unit: aidatapack.UnitRUB,
				Source: aidatapack.FactSource{Table: "daily_account_metrics", Field: "revenue"},
			},
		},
		Tables: aidatapack.Tables{
			AccountRollup: aidatapack.AccountRollupTable{DataStatus: "available", CurrentDay: map[string]any{"revenue": 1}},
			FinanceRollup: aidatapack.FinanceRollupTable{DataStatus: "not_available"},
		},
		Constraints: aidatapack.ConstraintsBlock{SKUPriceConstraints: []any{}},
	}

	for i := 1; i <= 5; i++ {
		sku := "80100010000" + string(rune('0'+i))
		eid := "sku:" + sku
		pack.EntityRegistry.SKUs[eid] = aidatapack.SKUEntity{
			EntityID: eid, EntityType: "sku", SKU: sku, OfferID: "OFFER-" + sku, Name: "Product " + sku,
		}
		coverFact := "sku." + sku + ".coverage_days"
		pack.FactRegistry[coverFact] = aidatapack.Fact{
			FactID: coverFact, EntityType: "sku", EntityID: eid, Metric: "coverage_days", Window: "current",
			Value: float64(i), Unit: aidatapack.UnitDays,
			Source: aidatapack.FactSource{Table: "stock_positions", Field: "coverage_days"},
		}
		pack.Tables.StockPositions = append(pack.Tables.StockPositions, aidatapack.StockPositionRow{
			EntityID: eid, SnapshotDate: "2026-06-25", DataStatus: "available", FactIDs: []string{coverFact},
		})
	}

	for i := 1; i <= 2; i++ {
		extID := "88200000160" + string(rune('0'+i))
		cid := "campaign:" + extID
		pack.EntityRegistry.Campaigns[cid] = aidatapack.CampaignEntity{
			EntityID: cid, EntityType: "campaign", CampaignExternalID: extID, Name: "Campaign " + extID,
		}
		spendFact := "campaign." + extID + ".last_7d.spend"
		pack.FactRegistry[spendFact] = aidatapack.Fact{
			FactID: spendFact, EntityType: "campaign", EntityID: cid, Metric: "spend", Window: "last_7d",
			Value: float64(i * 500), Unit: aidatapack.UnitRUB,
			Source: aidatapack.FactSource{Table: "ad_metrics", Field: "spend"},
		}
		pack.Tables.CampaignRollups = append(pack.Tables.CampaignRollups, aidatapack.CampaignRollupRow{
			EntityID: cid, CampaignExternalID: extID, Name: "Campaign " + extID,
			Last7d: aidatapack.CampaignMetricsWindow{
				Spend:               float64(i * 500),
				SpendWithoutOrders:  true,
				SpendWithoutRevenue: true,
			},
			FactIDs: []string{spendFact},
		})
	}

	if err := aidatapack.ValidateDataPack(pack); err != nil {
		t.Fatalf("fixture pack invalid: %v", err)
	}
	set, err := aidatapack.BuildChunks(pack, aidatapack.DefaultChunkOptions())
	if err != nil {
		t.Fatalf("BuildChunks: %v", err)
	}
	return pack, set
}

func TestDeterministicChunkAnalysisRunner_Deterministic(t *testing.T) {
	pack, set := fakeRunnerFixture(t)
	runner := NewDeterministicChunkAnalysisRunner()
	r1, err := runner.AnalyzeDataPackChunks(context.Background(), pack, set, dashboardhome.NarrativeRunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	r2, err := runner.AnalyzeDataPackChunks(context.Background(), pack, set, dashboardhome.NarrativeRunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r1, r2) {
		t.Fatal("expected deterministic results")
	}
}

func TestDeterministicChunkAnalysisRunner_MultipleSignalsAndCandidates(t *testing.T) {
	pack, set := fakeRunnerFixture(t)
	result, err := NewDeterministicChunkAnalysisRunner().AnalyzeDataPackChunks(
		context.Background(), pack, set, dashboardhome.NarrativeRunOptions{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.ValidatedSignalCount < 2 {
		t.Fatalf("signals=%d", result.ValidatedSignalCount)
	}
	if result.ValidatedCandidateCount < 2 {
		t.Fatalf("candidates=%d", result.ValidatedCandidateCount)
	}
}

func TestDeterministicChunkAnalysisRunner_MultipleCategories(t *testing.T) {
	pack, set := fakeRunnerFixture(t)
	result, err := NewDeterministicChunkAnalysisRunner().AnalyzeDataPackChunks(
		context.Background(), pack, set, dashboardhome.NarrativeRunOptions{},
	)
	if err != nil {
		t.Fatal(err)
	}
	categories := map[actions.ActionCategory]struct{}{}
	for _, candidate := range result.ValidatedActionCandidates {
		categories[candidate.Category] = struct{}{}
	}
	if len(categories) < 2 {
		t.Fatalf("categories=%v", categories)
	}
}

func TestDeterministicChunkAnalysisRunner_GroupsEntities(t *testing.T) {
	pack, set := fakeRunnerFixture(t)
	result, err := NewDeterministicChunkAnalysisRunner().AnalyzeDataPackChunks(
		context.Background(), pack, set, dashboardhome.NarrativeRunOptions{},
	)
	if err != nil {
		t.Fatal(err)
	}
	grouped := false
	for _, candidate := range result.ValidatedActionCandidates {
		if len(candidate.AffectedEntities) > 1 {
			grouped = true
			break
		}
	}
	if !grouped {
		t.Fatal("expected grouped candidate")
	}
}

func TestDeterministicChunkAnalysisRunner_FewerCandidatesThanEntities(t *testing.T) {
	pack, set := fakeRunnerFixture(t)
	result, err := NewDeterministicChunkAnalysisRunner().AnalyzeDataPackChunks(
		context.Background(), pack, set, dashboardhome.NarrativeRunOptions{},
	)
	if err != nil {
		t.Fatal(err)
	}
	entityCount := len(pack.EntityRegistry.SKUs) + len(pack.EntityRegistry.Campaigns)
	if result.ValidatedCandidateCount >= entityCount {
		t.Fatalf("candidates=%d entities=%d", result.ValidatedCandidateCount, entityCount)
	}
}

func TestDeterministicChunkAnalysisRunner_LocalIDsOnly(t *testing.T) {
	pack, set := fakeRunnerFixture(t)
	result, err := NewDeterministicChunkAnalysisRunner().AnalyzeDataPackChunks(
		context.Background(), pack, set, dashboardhome.NarrativeRunOptions{},
	)
	if err != nil {
		t.Fatal(err)
	}
	chunkByID := map[string]aidatapack.DataPackChunk{}
	for _, ch := range set.Chunks {
		chunkByID[ch.ChunkID] = ch
	}
	signalIDs := map[string]struct{}{}
	for _, sig := range result.ValidatedSignals {
		signalIDs[sig.SignalID] = struct{}{}
		chID, _, ok := strings.Cut(sig.SignalID, ":fake-signal:")
		if !ok {
			t.Fatalf("unexpected signal id %s", sig.SignalID)
		}
		ch := chunkByID[chID]
		for _, eid := range sig.EntityIDs {
			if _, ok := ch.LocalEntityCatalog[eid]; !ok {
				t.Fatalf("unknown entity %s in signal %s", eid, sig.SignalID)
			}
		}
		for _, fid := range sig.FactIDs {
			if _, ok := ch.LocalFactCatalog[fid]; !ok {
				t.Fatalf("unknown fact %s in signal %s", fid, sig.SignalID)
			}
		}
	}
	for _, candidate := range result.ValidatedActionCandidates {
		ch := chunkByID[candidate.ChunkID]
		for _, sid := range candidate.SourceSignalIDs {
			if _, ok := signalIDs[sid]; !ok {
				t.Fatalf("unknown source signal %s", sid)
			}
		}
		for _, ent := range candidate.AffectedEntities {
			if _, ok := ch.LocalEntityCatalog[ent.EntityID]; !ok {
				t.Fatalf("unknown entity %s in candidate %s", ent.EntityID, candidate.CandidateID)
			}
		}
		for _, fid := range candidate.EvidenceFactIDs {
			if _, ok := ch.LocalFactCatalog[fid]; !ok {
				t.Fatalf("unknown fact %s in candidate %s", fid, candidate.CandidateID)
			}
		}
	}
}

func TestDeterministicChunkAnalysisRunner_CategoryViaCanonicalHelper(t *testing.T) {
	pack, set := fakeRunnerFixture(t)
	result, err := NewDeterministicChunkAnalysisRunner().AnalyzeDataPackChunks(
		context.Background(), pack, set, dashboardhome.NarrativeRunOptions{},
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range result.ValidatedActionCandidates {
		expected, ok := actions.ActionCategoryForType(candidate.Type)
		if !ok {
			t.Fatalf("unknown type %s", candidate.Type)
		}
		if candidate.Category != expected {
			t.Fatalf("category=%s expected=%s", candidate.Category, expected)
		}
	}
}

func TestDeterministicChunkAnalysisRunner_CountConsistency(t *testing.T) {
	pack, set := fakeRunnerFixture(t)
	result, err := NewDeterministicChunkAnalysisRunner().AnalyzeDataPackChunks(
		context.Background(), pack, set, dashboardhome.NarrativeRunOptions{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.ValidatedSignalCount != len(result.ValidatedSignals) {
		t.Fatalf("signal count mismatch")
	}
	if result.ValidatedCandidateCount != len(result.ValidatedActionCandidates) {
		t.Fatalf("candidate count mismatch")
	}
	if result.Completed+result.Failed != result.ChunkCount {
		t.Fatalf("completed=%d failed=%d chunk_count=%d", result.Completed, result.Failed, result.ChunkCount)
	}
}

func TestDeterministicChunkAnalysisRunner_ProviderTokens(t *testing.T) {
	pack, set := fakeRunnerFixture(t)
	result, err := NewDeterministicChunkAnalysisRunner().AnalyzeDataPackChunks(
		context.Background(), pack, set, dashboardhome.NarrativeRunOptions{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.InputTokens <= 0 || result.OutputTokens <= 0 {
		t.Fatalf("tokens in=%d out=%d", result.InputTokens, result.OutputTokens)
	}
	if len(result.ChunkCalls) != result.ChunkCount {
		t.Fatalf("chunk_calls=%d chunk_count=%d", len(result.ChunkCalls), result.ChunkCount)
	}
}

func TestDeterministicChunkAnalysisRunner_EmptySuitableData(t *testing.T) {
	metricDate := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)
	pack := &aidatapack.SellerAIDataPack{
		SchemaVersion: aidatapack.SchemaVersion,
		SellerContext: aidatapack.SellerContext{SellerAccountID: 1, MetricDate: "2026-06-25"},
		DataWindows:   aidatapack.ComputeDataWindows(metricDate),
		DataQuality:   aidatapack.DataQuality{Sources: map[string]any{}},
		EntityRegistry: aidatapack.EntityRegistry{
			SKUs: map[string]aidatapack.SKUEntity{}, SKUGroups: map[string]aidatapack.SKUGroupEntity{},
			Campaigns: map[string]aidatapack.CampaignEntity{}, Warehouses: map[string]aidatapack.WarehouseEntity{},
			Categories: map[string]aidatapack.CategoryEntity{},
		},
		FactRegistry: map[string]aidatapack.Fact{
			"account.current_day.revenue": {
				FactID: "account.current_day.revenue", EntityType: "account", EntityID: "account:seller",
				Metric: "revenue", Value: 1.0, Unit: aidatapack.UnitRUB,
				Source: aidatapack.FactSource{Table: "daily_account_metrics", Field: "revenue"},
			},
		},
		Tables: aidatapack.Tables{
			AccountRollup: aidatapack.AccountRollupTable{DataStatus: "available", CurrentDay: map[string]any{"revenue": 1}},
			FinanceRollup: aidatapack.FinanceRollupTable{DataStatus: "not_available"},
		},
		Constraints: aidatapack.ConstraintsBlock{SKUPriceConstraints: []any{}},
	}
	set, err := aidatapack.BuildChunks(pack, aidatapack.DefaultChunkOptions())
	if err != nil {
		t.Fatal(err)
	}
	result, err := NewDeterministicChunkAnalysisRunner().AnalyzeDataPackChunks(
		context.Background(), pack, set, dashboardhome.NarrativeRunOptions{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.ValidatedCandidateCount != 0 || result.ValidatedSignalCount != 0 {
		t.Fatalf("expected empty validated output, signals=%d candidates=%d", result.ValidatedSignalCount, result.ValidatedCandidateCount)
	}
}

func TestDeterministicChunkAnalysisRunner_NilInputs(t *testing.T) {
	runner := NewDeterministicChunkAnalysisRunner()
	if _, err := runner.AnalyzeDataPackChunks(context.Background(), nil, &aidatapack.ChunkSet{}, dashboardhome.NarrativeRunOptions{}); err == nil {
		t.Fatal("expected nil pack error")
	}
	pack, _ := fakeRunnerFixture(t)
	if _, err := runner.AnalyzeDataPackChunks(context.Background(), pack, nil, dashboardhome.NarrativeRunOptions{}); err == nil {
		t.Fatal("expected nil chunk set error")
	}
}

func TestDeterministicChunkAnalysisRunner_InvalidChunkSet(t *testing.T) {
	pack, set := fakeRunnerFixture(t)
	set.SellerAccountID = 999
	_, err := NewDeterministicChunkAnalysisRunner().AnalyzeDataPackChunks(
		context.Background(), pack, set, dashboardhome.NarrativeRunOptions{},
	)
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestDeterministicChunkAnalysisRunner_ContextCanceled(t *testing.T) {
	pack, set := fakeRunnerFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := NewDeterministicChunkAnalysisRunner().AnalyzeDataPackChunks(ctx, pack, set, dashboardhome.NarrativeRunOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled, got %v", err)
	}
}
