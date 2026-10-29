package actions

import (
	"fmt"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
)

func (in ActionsAIFirstInput) requireDataPack() (*aidatapack.SellerAIDataPack, error) {
	if in.DataPack == nil {
		return nil, fmt.Errorf("%s", ActionsErrorInputMissingDataPack)
	}
	return in.DataPack, nil
}

func (in ActionsAIFirstInput) factExists(factID string) bool {
	factID = strings.TrimSpace(factID)
	if factID == "" {
		return false
	}
	if in.IsV2CandidateInput() {
		_, ok := in.FactCatalog[factID]
		return ok
	}
	pack, err := in.requireDataPack()
	if err != nil {
		return false
	}
	_, ok := pack.FactRegistry[factID]
	return ok
}

func (in ActionsAIFirstInput) factByID(factID string) (aidatapack.Fact, bool) {
	factID = strings.TrimSpace(factID)
	if factID == "" {
		return aidatapack.Fact{}, false
	}
	if in.IsV2CandidateInput() {
		cf, ok := in.FactCatalog[factID]
		if !ok {
			return aidatapack.Fact{}, false
		}
		return compactFactToFact(cf), true
	}
	pack, err := in.requireDataPack()
	if err != nil {
		return aidatapack.Fact{}, false
	}
	f, ok := pack.FactRegistry[factID]
	return f, ok
}

func compactFactToFact(cf aidatapack.CompactFact) aidatapack.Fact {
	return aidatapack.Fact{
		FactID:         cf.FactID,
		EntityID:       cf.EntityID,
		EntityType:     cf.EntityType,
		Metric:         cf.Metric,
		Window:         cf.Window,
		Value:          cf.Value,
		Unit:           cf.Unit,
		FormattedValue: cf.FormattedValue,
	}
}

func (in ActionsAIFirstInput) entityExists(entityType, entityID string) bool {
	entityID = strings.TrimSpace(entityID)
	if entityID == "" {
		return false
	}
	if in.IsV2CandidateInput() {
		_, ok := in.EntityCatalog[entityID]
		return ok
	}
	pack, err := in.requireDataPack()
	if err != nil {
		return false
	}
	return entityExistsInRegistry(pack, entityType, entityID)
}

func entityExistsInRegistry(pack *aidatapack.SellerAIDataPack, entityType, entityID string) bool {
	if pack == nil {
		return false
	}
	switch strings.TrimSpace(entityType) {
	case "sku":
		_, ok := pack.EntityRegistry.SKUs[entityID]
		return ok
	case "sku_group":
		_, ok := pack.EntityRegistry.SKUGroups[entityID]
		return ok
	case "campaign":
		_, ok := pack.EntityRegistry.Campaigns[entityID]
		return ok
	case "category":
		_, ok := pack.EntityRegistry.Categories[entityID]
		return ok
	case "warehouse":
		_, ok := pack.EntityRegistry.Warehouses[entityID]
		return ok
	case "funnel", "account":
		return entityID == "funnel:account" || entityID == "account:seller"
	default:
		return false
	}
}

func buildCandidateIndex(candidates []AIFirstActionCandidate) map[string]AIFirstActionCandidate {
	out := make(map[string]AIFirstActionCandidate, len(candidates))
	for _, c := range candidates {
		out[strings.TrimSpace(c.CandidateID)] = c
	}
	return out
}

func allowedFactsForSourceCandidates(
	input ActionsAIFirstInput,
	sourceIDs []string,
	candidateByID map[string]AIFirstActionCandidate,
) map[string]struct{} {
	allowed := map[string]struct{}{}
	for _, id := range sourceIDs {
		c, ok := candidateByID[strings.TrimSpace(id)]
		if !ok {
			continue
		}
		for _, factID := range c.EvidenceFactIDs {
			if factID = strings.TrimSpace(factID); factID != "" {
				allowed[factID] = struct{}{}
			}
		}
		for _, ent := range c.AffectedEntities {
			for _, factID := range ent.FactIDs {
				if factID = strings.TrimSpace(factID); factID != "" {
					allowed[factID] = struct{}{}
				}
			}
		}
	}
	if !input.IsV2CandidateInput() {
		return allowed
	}
	_ = input
	return allowed
}

func allowedEntitiesForSourceCandidates(
	sourceIDs []string,
	candidateByID map[string]AIFirstActionCandidate,
) map[string]struct{} {
	allowed := map[string]struct{}{}
	for _, id := range sourceIDs {
		c, ok := candidateByID[strings.TrimSpace(id)]
		if !ok {
			continue
		}
		for _, ent := range c.AffectedEntities {
			if entID := strings.TrimSpace(ent.EntityID); entID != "" {
				allowed[entID] = struct{}{}
			}
		}
	}
	return allowed
}
