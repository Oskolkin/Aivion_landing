package dashboardhome

import (
	"sort"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
)

func looksLikeEntityID(id string) bool {
	id = strings.TrimSpace(id)
	if id == "" {
		return false
	}
	if id == "account:seller" || id == "funnel:account" {
		return true
	}
	return strings.Contains(id, ":")
}

func looksLikeFactID(id string) bool {
	id = strings.TrimSpace(id)
	if id == "" {
		return false
	}
	if strings.Contains(id, ":") {
		return false
	}
	return strings.Contains(id, ".")
}

func chunkLocalFactIDAllowlist(chunk aidatapack.DataPackChunk) map[string]struct{} {
	out := map[string]struct{}{}
	for id := range chunk.LocalFactCatalog {
		out[id] = struct{}{}
	}
	return out
}

func chunkLocalEntityIDAllowlist(chunk aidatapack.DataPackChunk) map[string]struct{} {
	out := map[string]struct{}{}
	for id := range chunk.LocalEntityCatalog {
		out[id] = struct{}{}
	}
	for _, cf := range chunk.LocalFactCatalog {
		if cf.EntityID != "" {
			out[cf.EntityID] = struct{}{}
		}
	}
	switch chunk.ChunkType {
	case "manifest", "account_level_tables":
		out["account:seller"] = struct{}{}
		out["funnel:account"] = struct{}{}
	}
	return out
}

func sortedAllowlistKeys(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
