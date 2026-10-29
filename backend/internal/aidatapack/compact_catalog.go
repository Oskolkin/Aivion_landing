package aidatapack

import (
	"fmt"
	"sort"
	"strings"
)

// BuildCompactFactCatalog returns compact facts for the requested fact IDs only.
func BuildCompactFactCatalog(pack *SellerAIDataPack, factIDs []string) (map[string]CompactFact, error) {
	if pack == nil {
		return nil, fmt.Errorf("seller AI data pack is nil")
	}
	ids := uniqueNonEmptyIDs(factIDs)
	out := make(map[string]CompactFact, len(ids))
	for _, id := range ids {
		f, ok := pack.FactRegistry[id]
		if !ok {
			return nil, fmt.Errorf("compact_fact_not_found: %s", id)
		}
		fv := f.FormattedValue
		if fv == "" {
			fv = FormatValue(f.Unit, f.Value)
		}
		out[id] = CompactFact{
			FactID:         f.FactID,
			EntityID:       f.EntityID,
			EntityType:     f.EntityType,
			Metric:         f.Metric,
			Window:         f.Window,
			Value:          f.Value,
			Unit:           f.Unit,
			FormattedValue: fv,
		}
	}
	return out, nil
}

// BuildCompactEntityCatalog returns compact entities for the requested entity IDs only.
func BuildCompactEntityCatalog(pack *SellerAIDataPack, entityIDs []string) (map[string]CompactEntity, error) {
	if pack == nil {
		return nil, fmt.Errorf("seller AI data pack is nil")
	}
	ids := uniqueNonEmptyIDs(entityIDs)
	out := make(map[string]CompactEntity, len(ids))
	for _, id := range ids {
		ent, ok := compactEntityFromRegistry(pack, id)
		if !ok {
			return nil, fmt.Errorf("compact_entity_not_found: %s", id)
		}
		out[id] = ent
	}
	return out, nil
}

func uniqueNonEmptyIDs(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, raw := range in {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func compactEntityFromRegistry(pack *SellerAIDataPack, entityID string) (CompactEntity, bool) {
	if pack == nil {
		return CompactEntity{}, false
	}
	switch entityID {
	case "account:seller":
		return CompactEntity{EntityID: entityID, EntityType: "account", Name: "Seller"}, true
	case "funnel:account":
		return CompactEntity{EntityID: entityID, EntityType: "funnel", Name: "Account funnel"}, true
	}
	if e, ok := pack.EntityRegistry.SKUs[entityID]; ok {
		return CompactEntity{
			EntityID: e.EntityID, EntityType: e.EntityType, Name: e.Name,
			SKU: e.SKU, OfferID: e.OfferID, ProductID: e.OzonProductID, CategoryID: e.CategoryID,
		}, true
	}
	if e, ok := pack.EntityRegistry.Campaigns[entityID]; ok {
		return CompactEntity{
			EntityID: e.EntityID, EntityType: e.EntityType, Name: e.Name,
			CampaignExternalID: e.CampaignExternalID,
		}, true
	}
	if e, ok := pack.EntityRegistry.Categories[entityID]; ok {
		return CompactEntity{
			EntityID: e.EntityID, EntityType: e.EntityType, Name: e.Name, CategoryID: e.CategoryID,
		}, true
	}
	if e, ok := pack.EntityRegistry.Warehouses[entityID]; ok {
		return CompactEntity{
			EntityID: e.EntityID, EntityType: e.EntityType, Name: e.Name,
		}, true
	}
	if e, ok := pack.EntityRegistry.SKUGroups[entityID]; ok {
		return CompactEntity{
			EntityID: e.EntityID, EntityType: e.EntityType, ProductID: e.ProductID,
		}, true
	}
	return CompactEntity{}, false
}
