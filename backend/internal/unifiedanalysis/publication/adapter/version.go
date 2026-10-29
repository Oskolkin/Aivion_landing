// Package adapter projects a validated contract.UnifiedAnalysisOutputV1 into
// the shapes the existing dashboard/actions persistence layer expects
// (dashboardhome.FrozenDashboardData, a dashboard narrative payload, and
// actions.UpsertActionItemParams-compatible action projections).
package adapter

import (
	"crypto/sha256"
	"encoding/hex"
)

// Version identifies the Stage 5 dashboard/actions projection logic. Bump
// whenever the projection mapping changes in a way that could alter stored
// output for the same UnifiedAnalysisOutputV1 input.
const Version = "unified_publication_adapter_v1"

// Hash returns a stable sha256 hex digest derived from the adapter Version
// string. It is stored on unified_analysis_publications.adapter_hash so a
// publication row records exactly which projection logic produced it.
func Hash() string {
	sum := sha256.Sum256([]byte(Version))
	return hex.EncodeToString(sum[:])
}
