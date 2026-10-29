package dashboardhome

import (
	"sync/atomic"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
)

var buildChunkSetForPackHook func(*aidatapack.SellerAIDataPack, aidatapack.ChunkOptions) (*aidatapack.ChunkSet, error)

var buildChunkSetForPackCalls atomic.Int32

var trackPersistFrozenDashboardSnapshotCalls atomic.Bool
var persistFrozenDashboardSnapshotCalls atomic.Int32

var legacyAlertsLoads atomic.Int32
var legacyRecommendationsLoads atomic.Int32

func buildChunkSetForNarrative(pack *aidatapack.SellerAIDataPack, opts aidatapack.ChunkOptions) (*aidatapack.ChunkSet, error) {
	buildChunkSetForPackCalls.Add(1)
	if buildChunkSetForPackHook != nil {
		return buildChunkSetForPackHook(pack, opts)
	}
	return BuildChunkSetForPack(pack, opts)
}
