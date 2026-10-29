package actions

import (
	"fmt"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aicontext"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
)

// BuildActionsAIFirstInputFromCandidates assembles V2 Actions input from validated
// action candidates and compact catalogs. It does not accept dashboardhome types.
func BuildActionsAIFirstInputFromCandidates(
	pack *aidatapack.SellerAIDataPack,
	dataPackHash string,
	globalContext *aicontext.GlobalSemanticContext,
	globalContextHash string,
	candidates []AIFirstActionCandidate,
	failedChunkIDs []string,
	dataLimitations []string,
	opts GenerateActionsOptions,
) (ActionsAIFirstInput, error) {
	if pack == nil {
		return ActionsAIFirstInput{}, &ActionsGenerationError{
			Code: ActionsErrorInputBuildFailed,
			Err:  fmt.Errorf("data pack is nil"),
		}
	}
	if strings.TrimSpace(dataPackHash) == "" {
		return ActionsAIFirstInput{}, &ActionsGenerationError{
			Code: ActionsErrorInputBuildFailed,
			Err:  fmt.Errorf("data_pack_hash is required"),
		}
	}
	if globalContext == nil {
		return ActionsAIFirstInput{}, &ActionsGenerationError{
			Code: ActionsErrorInputBuildFailed,
			Err:  fmt.Errorf("global context is required"),
		}
	}
	if strings.TrimSpace(globalContextHash) == "" {
		return ActionsAIFirstInput{}, &ActionsGenerationError{
			Code: ActionsErrorInputBuildFailed,
			Err:  fmt.Errorf("global_context_hash is required"),
		}
	}

	candidatesCopy, err := validateAndCloneActionCandidates(candidates)
	if err != nil {
		return ActionsAIFirstInput{}, err
	}

	factIDs := collectCandidateFactIDs(candidatesCopy)
	entityIDs := collectCandidateEntityIDs(candidatesCopy)

	factCatalog, err := aidatapack.BuildCompactFactCatalog(pack, factIDs)
	if err != nil {
		return ActionsAIFirstInput{}, &ActionsGenerationError{
			Code: ActionsErrorInputBuildFailed,
			Err:  fmt.Errorf("build fact catalog: %w", err),
		}
	}
	entityCatalog, err := aidatapack.BuildCompactEntityCatalog(pack, entityIDs)
	if err != nil {
		return ActionsAIFirstInput{}, &ActionsGenerationError{
			Code: ActionsErrorInputBuildFailed,
			Err:  fmt.Errorf("build entity catalog: %w", err),
		}
	}

	maxActions := opts.MaxActionsOutput
	if maxActions <= 0 {
		maxActions = DefaultGenerateActionsOptions().MaxActionsOutput
	}
	if maxActions > 20 {
		maxActions = 20
	}

	limitations := normalizeStringList(dataLimitations)
	failedIDs := normalizeStringList(failedChunkIDs)
	if limitations == nil {
		limitations = []string{}
	}
	if failedIDs == nil {
		failedIDs = []string{}
	}

	return ActionsAIFirstInput{
		SchemaVersion:         ActionsAIFirstSchemaVersionV2,
		SellerAccountID:       pack.SellerContext.SellerAccountID,
		MetricDate:            pack.SellerContext.MetricDate,
		DataPackSchemaVersion: pack.SchemaVersion,
		DataPackHash:          dataPackHash,
		DataPack:              pack,
		GlobalContext:         globalContext,
		GlobalContextHash:     globalContextHash,
		ActionCandidates:      candidatesCopy,
		FactCatalog:           factCatalog,
		EntityCatalog:         entityCatalog,
		DataLimitations:       limitations,
		FailedChunkIDs:        failedIDs,
		AllowedActionTypes:    append([]ActionType(nil), ActionsAIFirstActiveActionTypes...),
		AllowedCategories:     append([]ActionCategory(nil), ActionsAIFirstActiveCategories...),
		AllowedPriorities:     []ActionPriority{ActionPriorityCritical, ActionPriorityImportant, ActionPriorityWatch},
		ForbiddenClaims:       append([]string(nil), aiFirstForbiddenClaims...),
		MaxActions:            maxActions,
	}, nil
}

func validateAndCloneActionCandidates(candidates []AIFirstActionCandidate) ([]AIFirstActionCandidate, error) {
	if candidates == nil {
		return []AIFirstActionCandidate{}, nil
	}
	seen := map[string]struct{}{}
	out := make([]AIFirstActionCandidate, 0, len(candidates))
	for _, c := range candidates {
		copy := cloneActionCandidate(c)
		id := strings.TrimSpace(copy.CandidateID)
		if id == "" {
			return nil, &ActionsGenerationError{
				Code: ActionsErrorInputBuildFailed,
				Err:  fmt.Errorf("candidate_id is required"),
			}
		}
		if strings.TrimSpace(copy.ChunkID) == "" {
			return nil, &ActionsGenerationError{
				Code: ActionsErrorInputBuildFailed,
				Err:  fmt.Errorf("chunk_id is required for candidate %s", id),
			}
		}
		if _, ok := seen[id]; ok {
			return nil, &ActionsGenerationError{
				Code: ActionsErrorInputDuplicateCandidateID,
				Err:  fmt.Errorf("duplicate candidate_id %s", id),
			}
		}
		seen[id] = struct{}{}
		if !isActiveAIFirstActionType(copy.Type) {
			return nil, &ActionsGenerationError{
				Code: ActionsErrorInputBuildFailed,
				Err:  fmt.Errorf("invalid action type for candidate %s", id),
			}
		}
		wantCategory, ok := ActionCategoryForType(copy.Type)
		if !ok {
			return nil, &ActionsGenerationError{
				Code: ActionsErrorInputBuildFailed,
				Err:  fmt.Errorf("invalid action type for candidate %s", id),
			}
		}
		if copy.Category == "" {
			copy.Category = wantCategory
		} else if copy.Category != wantCategory {
			return nil, &ActionsGenerationError{
				Code: ActionsErrorInputBuildFailed,
				Err:  fmt.Errorf("category mismatch for candidate %s", id),
			}
		}
		if strings.TrimSpace(copy.Problem) == "" ||
			strings.TrimSpace(copy.RecommendedAction) == "" ||
			strings.TrimSpace(copy.WhyActionable) == "" {
			return nil, &ActionsGenerationError{
				Code: ActionsErrorInputBuildFailed,
				Err:  fmt.Errorf("candidate %s missing required narrative fields", id),
			}
		}
		out = append(out, copy)
	}
	return out, nil
}

func cloneActionCandidate(c AIFirstActionCandidate) AIFirstActionCandidate {
	out := c
	out.AffectedEntities = append([]AIFirstAffectedEntityRef(nil), c.AffectedEntities...)
	out.EvidenceFactIDs = append([]string(nil), c.EvidenceFactIDs...)
	out.SourceSignalIDs = append([]string(nil), c.SourceSignalIDs...)
	out.Limitations = append([]string(nil), c.Limitations...)
	for i := range out.AffectedEntities {
		out.AffectedEntities[i].FactIDs = append([]string(nil), c.AffectedEntities[i].FactIDs...)
	}
	return out
}

func collectCandidateFactIDs(candidates []AIFirstActionCandidate) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, c := range candidates {
		for _, id := range c.EvidenceFactIDs {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
		for _, ent := range c.AffectedEntities {
			for _, id := range ent.FactIDs {
				id = strings.TrimSpace(id)
				if id == "" {
					continue
				}
				if _, ok := seen[id]; ok {
					continue
				}
				seen[id] = struct{}{}
				out = append(out, id)
			}
		}
	}
	return out
}

func collectCandidateEntityIDs(candidates []AIFirstActionCandidate) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, c := range candidates {
		for _, ent := range c.AffectedEntities {
			id := strings.TrimSpace(ent.EntityID)
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	return out
}

func normalizeStringList(in []string) []string {
	if in == nil {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, raw := range in {
		s := strings.TrimSpace(raw)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func isActiveAIFirstActionType(t ActionType) bool {
	for _, allowed := range ActionsAIFirstActiveActionTypes {
		if allowed == t {
			return true
		}
	}
	return false
}
