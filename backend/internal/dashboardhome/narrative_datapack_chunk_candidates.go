package dashboardhome

import (
	"fmt"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
)

const (
	maxChunkActionCandidates          = 5
	maxChunkCandidateEvidenceFacts    = 12
	maxChunkCandidateAffectedEntities = 20
)

func formatChunkActionCandidateID(authoritativeChunkID string, oneBasedRawIndex int) string {
	return fmt.Sprintf("%s:candidate:%02d", authoritativeChunkID, oneBasedRawIndex)
}

// normalizeActionCandidateTechnicalFields overwrites backend-controlled candidate IDs.
func normalizeActionCandidateTechnicalFields(authoritativeChunkID string, candidates []actions.AIFirstActionCandidate) []actions.AIFirstActionCandidate {
	if len(candidates) == 0 {
		return candidates
	}
	out := make([]actions.AIFirstActionCandidate, len(candidates))
	for i, candidate := range candidates {
		candidate.CandidateID = formatChunkActionCandidateID(authoritativeChunkID, i+1)
		candidate.ChunkID = authoritativeChunkID
		out[i] = candidate
	}
	return out
}

func isValidChunkCandidateLocalPriority(p actions.ActionPriority) bool {
	switch p {
	case actions.ActionPriorityCritical, actions.ActionPriorityImportant, actions.ActionPriorityWatch:
		return true
	default:
		return false
	}
}

func validateChunkActionCandidates(
	chunk aidatapack.DataPackChunk,
	candidates []actions.AIFirstActionCandidate,
	validSignalIDs map[string]struct{},
	allowedFacts map[string]struct{},
	allowedEntities map[string]struct{},
	entityCatalog map[string]aidatapack.CompactEntity,
) (accepted []actions.AIFirstActionCandidate, repairableErrors []string, rejectedCount int) {
	entityCatalog = entityCatalogOrEmpty(entityCatalog)
	for i, candidate := range candidates {
		prefix := fmt.Sprintf("candidate_%d(%s)", i+1, candidate.CandidateID)
		if i >= maxChunkActionCandidates {
			rejectedCount++
			repairableErrors = append(repairableErrors, prefix+":candidate_exceeds_chunk_limit")
			continue
		}
		outcome := normalizeAndValidateOneChunkActionCandidate(
			candidate, validSignalIDs, allowedFacts, allowedEntities, entityCatalog,
		)
		for _, w := range outcome.Warnings {
			repairableErrors = append(repairableErrors, prefix+":"+w)
		}
		if !outcome.Accepted {
			rejectedCount++
			msg := prefix + ":" + outcome.RejectCode
			repairableErrors = append(repairableErrors, msg)
			continue
		}
		accepted = append(accepted, repairAcceptedChunkActionCandidate(outcome.Candidate, entityCatalog))
	}
	_ = chunk
	return accepted, repairableErrors, rejectedCount
}

func entityCatalogOrEmpty(catalog map[string]aidatapack.CompactEntity) map[string]aidatapack.CompactEntity {
	if catalog != nil {
		return catalog
	}
	return map[string]aidatapack.CompactEntity{}
}

func normalizeAndValidateOneChunkActionCandidate(
	candidate actions.AIFirstActionCandidate,
	validSignalIDs map[string]struct{},
	allowedFacts map[string]struct{},
	allowedEntities map[string]struct{},
	entityCatalog map[string]aidatapack.CompactEntity,
) chunkCandidateOutcome {
	out := chunkCandidateOutcome{Candidate: candidate}

	if _, ok := actions.ActionCategoryForType(candidate.Type); !ok {
		out.RejectCode = "candidate_unknown_action_type"
		out.RejectField = "type"
		out.RawValue = string(candidate.Type)
		return out
	}
	if !isValidChunkCandidateLocalPriority(candidate.LocalPriority) {
		out.RejectCode = "candidate_invalid_local_priority"
		out.RejectField = "local_priority"
		out.RawValue = string(candidate.LocalPriority)
		return out
	}
	if strings.TrimSpace(candidate.Problem) == "" {
		out.RejectCode = "candidate_problem_empty"
		out.RejectField = "problem"
		return out
	}
	if strings.TrimSpace(candidate.RecommendedAction) == "" {
		out.RejectCode = "candidate_recommended_action_empty"
		out.RejectField = "recommended_action"
		return out
	}
	if strings.TrimSpace(candidate.WhyActionable) == "" {
		out.RejectCode = "candidate_why_actionable_empty"
		out.RejectField = "why_actionable"
		return out
	}

	candidate.SourceSignalIDs, out = normalizeCandidateSignalRefs(candidate.SourceSignalIDs, validSignalIDs, out)
	if len(candidate.SourceSignalIDs) == 0 {
		out.RejectCode = "no_valid_source_signals_after_normalization"
		out.RejectField = "source_signal_ids"
		return out
	}

	candidate.EvidenceFactIDs, out = normalizeCandidateFactRefs(candidate.EvidenceFactIDs, allowedFacts, out, "evidence_fact_ids")
	if len(candidate.EvidenceFactIDs) == 0 {
		out.RejectCode = "no_valid_evidence_after_normalization"
		out.RejectField = "evidence_fact_ids"
		return out
	}
	if len(candidate.EvidenceFactIDs) > maxChunkCandidateEvidenceFacts {
		out.RejectCode = "candidate_too_many_facts"
		out.RejectField = "evidence_fact_ids"
		return out
	}

	candidate.AffectedEntities, out = normalizeCandidateEntityRefs(
		candidate.AffectedEntities, allowedEntities, entityCatalog, allowedFacts, out,
	)
	allowEmptyEntities := isAccountLevelCandidateType(candidate.Type) ||
		len(candidate.AffectedEntities) == 1 && isAccountLevelEntityID(candidate.AffectedEntities[0].EntityID)
	if len(candidate.AffectedEntities) == 0 && !allowEmptyEntities {
		out.RejectCode = "candidate_no_affected_entities"
		out.RejectField = "affected_entities"
		return out
	}
	if len(candidate.AffectedEntities) > maxChunkCandidateAffectedEntities {
		out.RejectCode = "candidate_too_many_entities"
		out.RejectField = "affected_entities"
		return out
	}

	out.Accepted = true
	out.Candidate = candidate
	return out
}

func normalizeCandidateSignalRefs(
	refs []string,
	validSignalIDs map[string]struct{},
	out chunkCandidateOutcome,
) ([]string, chunkCandidateOutcome) {
	clean := compactStringIDs(refs)
	kept := make([]string, 0, len(clean))
	for _, sid := range clean {
		if _, ok := validSignalIDs[sid]; ok {
			kept = append(kept, sid)
		} else {
			out.Repaired = true
			out.Repairable = true
			out.Warnings = append(out.Warnings, "unknown_source_signal_removed:"+sid)
		}
	}
	return kept, out
}

func normalizeCandidateFactRefs(
	refs []string,
	allowedFacts map[string]struct{},
	out chunkCandidateOutcome,
	field string,
) ([]string, chunkCandidateOutcome) {
	clean := compactStringIDs(refs)
	kept := make([]string, 0, len(clean))
	for _, fid := range clean {
		if _, ok := allowedFacts[fid]; ok {
			kept = append(kept, fid)
		} else {
			out.Repaired = true
			out.Repairable = true
			out.Warnings = append(out.Warnings, "unknown_fact_removed:"+fid)
			_ = field
		}
	}
	return kept, out
}

func normalizeCandidateEntityRefs(
	entities []actions.AIFirstAffectedEntityRef,
	allowedEntities map[string]struct{},
	entityCatalog map[string]aidatapack.CompactEntity,
	allowedFacts map[string]struct{},
	out chunkCandidateOutcome,
) ([]actions.AIFirstAffectedEntityRef, chunkCandidateOutcome) {
	kept := make([]actions.AIFirstAffectedEntityRef, 0, len(entities))
	seen := map[string]struct{}{}
	for _, ent := range entities {
		entityID := strings.TrimSpace(ent.EntityID)
		if entityID == "" {
			out.Repaired = true
			out.Repairable = true
			out.Warnings = append(out.Warnings, "empty_entity_id_removed")
			continue
		}
		if _, ok := allowedEntities[entityID]; !ok {
			out.Repaired = true
			out.Repairable = true
			out.Warnings = append(out.Warnings, "unknown_entity_removed:"+entityID)
			continue
		}
		if catalogEntity, ok := entityCatalog[entityID]; ok {
			if strings.TrimSpace(ent.EntityType) != "" && catalogEntity.EntityType != "" && ent.EntityType != catalogEntity.EntityType {
				out.Repaired = true
				out.Repairable = true
				out.Warnings = append(out.Warnings, "entity_type_mismatch_removed:"+entityID)
				continue
			}
			if strings.TrimSpace(ent.Name) == "" && catalogEntity.Name != "" {
				ent.Name = catalogEntity.Name
			}
		}
		cleanFacts := make([]string, 0, len(ent.FactIDs))
		for _, fid := range compactStringIDs(ent.FactIDs) {
			if _, ok := allowedFacts[fid]; ok {
				cleanFacts = append(cleanFacts, fid)
			} else {
				out.Repaired = true
				out.Repairable = true
				out.Warnings = append(out.Warnings, "unknown_entity_fact_removed:"+fid)
			}
		}
		ent.FactIDs = cleanFacts
		if _, dup := seen[entityID]; dup {
			out.Repaired = true
			out.Warnings = append(out.Warnings, "duplicate_entity_removed:"+entityID)
			continue
		}
		seen[entityID] = struct{}{}
		kept = append(kept, ent)
	}
	return kept, out
}

func repairAcceptedChunkActionCandidate(
	candidate actions.AIFirstActionCandidate,
	entityCatalog map[string]aidatapack.CompactEntity,
) actions.AIFirstActionCandidate {
	if canonicalCategory, ok := actions.ActionCategoryForType(candidate.Type); ok {
		candidate.Category = canonicalCategory
	}
	entities := make([]actions.AIFirstAffectedEntityRef, 0, len(candidate.AffectedEntities))
	for _, ent := range candidate.AffectedEntities {
		if catalogEntity, ok := entityCatalog[ent.EntityID]; ok && strings.TrimSpace(ent.Name) == "" && catalogEntity.Name != "" {
			ent.Name = catalogEntity.Name
		}
		ent.FactIDs = compactStringIDs(ent.FactIDs)
		entities = append(entities, ent)
	}
	candidate.AffectedEntities = entities
	candidate.EvidenceFactIDs = compactStringIDs(candidate.EvidenceFactIDs)
	candidate.SourceSignalIDs = compactStringIDs(candidate.SourceSignalIDs)
	candidate.Limitations = compactStringIDs(candidate.Limitations)
	return candidate
}
