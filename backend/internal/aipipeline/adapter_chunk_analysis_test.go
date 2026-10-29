package aipipeline

import "testing"

func TestDashboardNarrativeAdapter_ImplementsChunkAnalysisRunner(t *testing.T) {
	var (
		_ DashboardRunner     = (*DashboardNarrativeAdapter)(nil)
		_ ChunkAnalysisRunner = (*DashboardNarrativeAdapter)(nil)
	)
}
