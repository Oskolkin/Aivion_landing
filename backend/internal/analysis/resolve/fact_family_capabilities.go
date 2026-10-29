package resolve

import "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"

// FACT_FAMILY_CAPABILITY_REGISTRY_V1 mirrors the shared factualapi registry version.
const FACT_FAMILY_CAPABILITY_REGISTRY_V1 = factualapi.FACT_FAMILY_CAPABILITY_REGISTRY_V1

// FactFamilyKey identifies a readiness watermark binding grain.
type FactFamilyKey = factualapi.FactFamilyKey

// RelatedCapabilityKeys returns the explicit registry capability keys for a
// Domain+FactFamily binding. Empty when the family is unknown to the registry.
func RelatedCapabilityKeys(domain, factFamily string) []string {
	return factualapi.RelatedCapabilityKeys(domain, factFamily)
}

// matchCapabilities returns all capability snapshots related to the fact family
// via the explicit registry (HISTORICAL_QUERY preferred when both scopes exist).
func matchCapabilities(caps []CapabilitySnapshot, domain, factFamily string) []CapabilitySnapshot {
	want := RelatedCapabilityKeys(domain, factFamily)
	if len(want) == 0 {
		return nil
	}
	index := make(map[string][]CapabilitySnapshot, len(caps))
	for i := range caps {
		index[caps[i].Key] = append(index[caps[i].Key], caps[i])
	}
	var out []CapabilitySnapshot
	for _, key := range want {
		matches := index[key]
		if len(matches) == 0 {
			continue
		}
		var chosen *CapabilitySnapshot
		for i := range matches {
			c := &matches[i]
			if c.Scope == "HISTORICAL_QUERY" {
				chosen = c
				break
			}
			if chosen == nil {
				chosen = c
			}
		}
		if chosen != nil {
			out = append(out, *chosen)
		}
	}
	return out
}
