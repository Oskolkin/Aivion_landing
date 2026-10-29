package dashboardhome

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
)

const DashboardChunkAnalysisSchemaVersionV3 = "dashboard_chunk_analysis_v3"

// ChunkRefMapping maps short local refs to canonical IDs for one chunk AI call.
type ChunkRefMapping struct {
	ChunkID    string            `json:"chunk_id"`
	FactRefs   map[string]string `json:"fact_refs"`
	EntityRefs map[string]string `json:"entity_refs"`
}

// BuildChunkRefMapping assigns deterministic local refs for chunk AI input.
func BuildChunkRefMapping(chunk aidatapack.DataPackChunk) ChunkRefMapping {
	factIDs := sortedAllowlistKeys(chunkLocalFactIDAllowlist(chunk))
	entityIDs := sortedAllowlistKeys(chunkLocalEntityIDAllowlist(chunk))
	m := ChunkRefMapping{
		ChunkID:    chunk.ChunkID,
		FactRefs:   make(map[string]string, len(factIDs)),
		EntityRefs: make(map[string]string, len(entityIDs)),
	}
	for i, id := range factIDs {
		m.FactRefs[fmt.Sprintf("f%d", i+1)] = id
	}
	for i, id := range entityIDs {
		m.EntityRefs[fmt.Sprintf("e%d", i+1)] = id
	}
	return m
}

func (m ChunkRefMapping) factRefCatalog() []map[string]any {
	type row struct {
		ref string
		id  string
	}
	rows := make([]row, 0, len(m.FactRefs))
	for ref, id := range m.FactRefs {
		rows = append(rows, row{ref, id})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ref < rows[j].ref })
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		entry := map[string]any{"fact_ref": r.ref, "fact_id": r.id}
		out = append(out, entry)
	}
	return out
}

func (m ChunkRefMapping) entityRefCatalog() []map[string]any {
	type row struct {
		ref string
		id  string
	}
	rows := make([]row, 0, len(m.EntityRefs))
	for ref, id := range m.EntityRefs {
		rows = append(rows, row{ref, id})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ref < rows[j].ref })
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{"entity_ref": r.ref, "entity_id": r.id})
	}
	return out
}

func resolveFactRef(mapping ChunkRefMapping, ref string) (string, bool) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", false
	}
	id, ok := mapping.FactRefs[ref]
	return id, ok
}

func resolveEntityRef(mapping ChunkRefMapping, ref string) (string, bool) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", false
	}
	id, ok := mapping.EntityRefs[ref]
	return id, ok
}

// ResolveChunkAnalysisRefs converts v3 local refs to canonical IDs in-place.
func ResolveChunkAnalysisRefs(analysis *DashboardChunkAnalysis, mapping ChunkRefMapping) (factUnknown, entityUnknown int) {
	if analysis == nil {
		return 0, 0
	}
	for i := range analysis.Signals {
		sig := &analysis.Signals[i]
		if len(sig.FactRefs) > 0 {
			for _, ref := range sig.FactRefs {
				if id, ok := resolveFactRef(mapping, ref); ok {
					sig.FactIDs = appendUniqueString(sig.FactIDs, id)
				} else {
					factUnknown++
				}
			}
		}
		if len(sig.EntityRefs) > 0 {
			for _, ref := range sig.EntityRefs {
				if id, ok := resolveEntityRef(mapping, ref); ok {
					sig.EntityIDs = appendUniqueString(sig.EntityIDs, id)
				} else {
					entityUnknown++
				}
			}
		}
	}
	for i := range analysis.ActionCandidates {
		c := &analysis.ActionCandidates[i]
		if len(c.EvidenceFactRefs) > 0 {
			for _, ref := range c.EvidenceFactRefs {
				if id, ok := resolveFactRef(mapping, ref); ok {
					c.EvidenceFactIDs = appendUniqueString(c.EvidenceFactIDs, id)
				} else {
					factUnknown++
				}
			}
		}
		for j := range c.AffectedEntities {
			ent := &c.AffectedEntities[j]
			if ent.EntityRef != "" {
				if id, ok := resolveEntityRef(mapping, ent.EntityRef); ok {
					ent.EntityID = id
				} else {
					entityUnknown++
				}
			}
		}
	}
	return factUnknown, entityUnknown
}

func appendUniqueString(items []string, v string) []string {
	v = strings.TrimSpace(v)
	if v == "" {
		return items
	}
	for _, existing := range items {
		if existing == v {
			return items
		}
	}
	return append(items, v)
}

// chunkRefDiagnostics tracks local ref resolution for one chunk call.
type chunkRefDiagnostics struct {
	FactRefsReturned   int `json:"chunk_fact_refs_returned"`
	FactRefsValid      int `json:"chunk_fact_refs_valid"`
	FactRefsUnknown    int `json:"chunk_fact_refs_unknown"`
	EntityRefsReturned int `json:"chunk_entity_refs_returned"`
	EntityRefsValid    int `json:"chunk_entity_refs_valid"`
	EntityRefsUnknown  int `json:"chunk_entity_refs_unknown"`
}

// BuildChunkRefDiagnostics summarizes local ref resolution for one chunk response.
func BuildChunkRefDiagnostics(analysis DashboardChunkAnalysis, factUnknown, entityUnknown int) chunkRefDiagnostics {
	returnedFacts, returnedEntities := countChunkRefUsage(analysis)
	validFacts := returnedFacts - factUnknown
	if validFacts < 0 {
		validFacts = 0
	}
	validEntities := returnedEntities - entityUnknown
	if validEntities < 0 {
		validEntities = 0
	}
	return chunkRefDiagnostics{
		FactRefsReturned:   returnedFacts,
		FactRefsValid:      validFacts,
		FactRefsUnknown:    factUnknown,
		EntityRefsReturned: returnedEntities,
		EntityRefsValid:    validEntities,
		EntityRefsUnknown:  entityUnknown,
	}
}

func CountChunkRefUsage(analysis DashboardChunkAnalysis) (returnedFacts, returnedEntities int) {
	return countChunkRefUsage(analysis)
}

// ResolveChunkAnalysisRefStats returns unknown ref counts after resolution.
func ResolveChunkAnalysisRefStats(analysis *DashboardChunkAnalysis, mapping ChunkRefMapping) (validFacts, unknownFacts, unknownEntities int) {
	if analysis == nil {
		return 0, 0, 0
	}
	clone := *analysis
	unknownFacts, unknownEntities = ResolveChunkAnalysisRefs(&clone, mapping)
	returnedFacts, _ := countChunkRefUsage(clone)
	validFacts = returnedFacts - unknownFacts
	if validFacts < 0 {
		validFacts = 0
	}
	return validFacts, unknownFacts, unknownEntities
}

func countChunkRefUsage(analysis DashboardChunkAnalysis) (returnedFacts, returnedEntities int) {
	for _, sig := range analysis.Signals {
		returnedFacts += len(sig.FactRefs)
		returnedEntities += len(sig.EntityRefs)
	}
	for _, c := range analysis.ActionCandidates {
		returnedFacts += len(c.EvidenceFactRefs)
		for _, ent := range c.AffectedEntities {
			if ent.EntityRef != "" {
				returnedEntities++
			}
		}
	}
	return returnedFacts, returnedEntities
}
