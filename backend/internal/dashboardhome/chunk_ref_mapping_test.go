package dashboardhome

import (
	"testing"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
)

func TestBuildChunkRefMapping(t *testing.T) {
	chunk := aidatapack.DataPackChunk{
		ChunkID: "stock_positions_001",
		LocalFactCatalog: map[string]aidatapack.CompactFact{
			"account.last_7d.orders": {FactID: "account.last_7d.orders", Metric: "orders"},
		},
		LocalEntityCatalog: map[string]aidatapack.CompactEntity{
			"account:seller": {EntityID: "account:seller", EntityType: "account"},
		},
	}
	m := BuildChunkRefMapping(chunk)
	id, ok := m.FactRefs["f1"]
	if !ok || id != "account.last_7d.orders" {
		t.Fatalf("fact mapping=%q ok=%v", id, ok)
	}
	eid, ok := m.EntityRefs["e1"]
	if !ok || eid != "account:seller" {
		t.Fatalf("entity mapping=%q ok=%v", eid, ok)
	}
}

func TestResolveChunkAnalysisRefs(t *testing.T) {
	chunk := aidatapack.DataPackChunk{
		LocalFactCatalog: map[string]aidatapack.CompactFact{
			"account.last_7d.orders": {FactID: "account.last_7d.orders"},
		},
		LocalEntityCatalog: map[string]aidatapack.CompactEntity{
			"account:seller": {EntityID: "account:seller"},
		},
	}
	m := BuildChunkRefMapping(chunk)
	analysis := DashboardChunkAnalysis{
		SchemaVersion: DashboardChunkAnalysisSchemaVersionV3,
		Signals: []DashboardChunkSignal{{
			SignalID: "s1", FactRefs: []string{"f1"}, EntityRefs: []string{"e1"},
		}},
	}
	unknownFacts, unknownEntities := ResolveChunkAnalysisRefs(&analysis, m)
	if unknownFacts != 0 || unknownEntities != 0 {
		t.Fatalf("unknown facts=%d entities=%d", unknownFacts, unknownEntities)
	}
	if len(analysis.Signals[0].FactIDs) != 1 || analysis.Signals[0].FactIDs[0] != "account.last_7d.orders" {
		t.Fatalf("facts=%v", analysis.Signals[0].FactIDs)
	}
}

func TestProductionChunkSchemaIsV3(t *testing.T) {
	if ProductionChunkAnalysisSchemaVersion() != DashboardChunkAnalysisSchemaVersionV3 {
		t.Fatalf("schema=%s", ProductionChunkAnalysisSchemaVersion())
	}
}
