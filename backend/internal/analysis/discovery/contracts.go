package discovery

import (
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/analysis/evidencecatalog"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/canonicaljson"
)

func hashProfile(v any) string { return canonicaljson.Hash(v) }

type EntityKind = evidencecatalog.EntityKind

const (
	KindAccount  = evidencecatalog.KindAccount
	KindCategory = evidencecatalog.KindCategory
	KindSKU      = evidencecatalog.KindSKU
	KindCampaign = evidencecatalog.KindCampaign
)

type PackEntity struct {
	Kind EntityKind `json:"kind"`
	ID   string     `json:"id"`
}

type Usability string

const (
	UsabilityReady   Usability = "READY"
	UsabilityPartial Usability = "PARTIAL"
	UsabilityStale   Usability = "STALE"
)

type DiscoveryState string

const (
	StateThreadsFound        DiscoveryState = "THREADS_FOUND"
	StateNoMeaningfulThreads DiscoveryState = "NO_MEANINGFUL_THREADS"
)

func DefaultMaxOutputTokens() int { return 8192 }
