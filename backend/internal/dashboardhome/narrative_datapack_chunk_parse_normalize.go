package dashboardhome

import (
	"encoding/json"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
)

func parseFlexibleActionCandidates(raw json.RawMessage) ([]actions.AIFirstActionCandidate, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return []actions.AIFirstActionCandidate{}, nil
	}

	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		if candidate, ok := parseOneFlexibleActionCandidate(raw); ok {
			return []actions.AIFirstActionCandidate{candidate}, nil
		}
		return []actions.AIFirstActionCandidate{}, nil
	}

	out := make([]actions.AIFirstActionCandidate, 0, len(items))
	for _, item := range items {
		if candidate, ok := parseOneFlexibleActionCandidate(item); ok {
			out = append(out, candidate)
		}
	}
	return out, nil
}

func parseOneFlexibleActionCandidate(raw json.RawMessage) (actions.AIFirstActionCandidate, bool) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return actions.AIFirstActionCandidate{}, false
	}

	var candidate actions.AIFirstActionCandidate
	unmarshalJSONField(fields, "candidate_id", &candidate.CandidateID)
	unmarshalJSONField(fields, "chunk_id", &candidate.ChunkID)
	unmarshalJSONField(fields, "type", &candidate.Type)
	unmarshalJSONField(fields, "category", &candidate.Category)
	unmarshalJSONField(fields, "local_priority", &candidate.LocalPriority)
	unmarshalJSONField(fields, "problem", &candidate.Problem)
	unmarshalJSONField(fields, "recommended_action", &candidate.RecommendedAction)
	unmarshalJSONField(fields, "why_actionable", &candidate.WhyActionable)

	if rawEntities, ok := fields["affected_entities"]; ok {
		candidate.AffectedEntities = normalizeAffectedEntitiesJSON(rawEntities)
	} else {
		candidate.AffectedEntities = []actions.AIFirstAffectedEntityRef{}
	}
	if rawFacts, ok := fields["evidence_fact_ids"]; ok {
		candidate.EvidenceFactIDs = normalizeStringIDArrayJSON(rawFacts)
	} else {
		candidate.EvidenceFactIDs = []string{}
	}
	if rawFactRefs, ok := fields["evidence_fact_refs"]; ok {
		candidate.EvidenceFactRefs = normalizeStringIDArrayJSON(rawFactRefs)
	}
	if rawSignals, ok := fields["source_signal_ids"]; ok {
		candidate.SourceSignalIDs = normalizeStringIDArrayJSON(rawSignals)
	} else {
		candidate.SourceSignalIDs = []string{}
	}
	if rawLimits, ok := fields["limitations"]; ok {
		candidate.Limitations = normalizeStringIDArrayJSON(rawLimits)
	}

	return candidate, true
}

func unmarshalJSONField(fields map[string]json.RawMessage, key string, dest any) {
	raw, ok := fields[key]
	if !ok {
		return
	}
	_ = json.Unmarshal(raw, dest)
}

func normalizeAffectedEntitiesJSON(raw json.RawMessage) []actions.AIFirstAffectedEntityRef {
	if len(raw) == 0 || string(raw) == "null" {
		return []actions.AIFirstAffectedEntityRef{}
	}

	var refs []actions.AIFirstAffectedEntityRef
	if err := json.Unmarshal(raw, &refs); err == nil {
		return normalizeEntityRefSlice(refs)
	}

	var one actions.AIFirstAffectedEntityRef
	if err := json.Unmarshal(raw, &one); err == nil {
		if ref, ok := normalizeEntityRef(one); ok {
			return []actions.AIFirstAffectedEntityRef{ref}
		}
	}

	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		return entityRefsFromStringIDs([]string{single})
	}

	var strs []string
	if err := json.Unmarshal(raw, &strs); err == nil {
		return entityRefsFromStringIDs(strs)
	}

	return []actions.AIFirstAffectedEntityRef{}
}

func normalizeEntityRefSlice(refs []actions.AIFirstAffectedEntityRef) []actions.AIFirstAffectedEntityRef {
	out := make([]actions.AIFirstAffectedEntityRef, 0, len(refs))
	for _, ref := range refs {
		if normalized, ok := normalizeEntityRef(ref); ok {
			out = append(out, normalized)
		}
	}
	return out
}

func normalizeEntityRef(ref actions.AIFirstAffectedEntityRef) (actions.AIFirstAffectedEntityRef, bool) {
	entityID := strings.TrimSpace(ref.EntityID)
	if entityID == "" {
		return actions.AIFirstAffectedEntityRef{}, false
	}
	entityType := strings.TrimSpace(ref.EntityType)
	if entityType == "" {
		entityType = inferEntityTypeFromEntityID(entityID)
	}
	if entityType == "" {
		return actions.AIFirstAffectedEntityRef{}, false
	}
	ref.EntityID = entityID
	ref.EntityType = entityType
	ref.FactIDs = compactStringIDs(ref.FactIDs)
	return ref, true
}

func entityRefsFromStringIDs(ids []string) []actions.AIFirstAffectedEntityRef {
	out := make([]actions.AIFirstAffectedEntityRef, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		entityType := inferEntityTypeFromEntityID(id)
		if entityType == "" {
			continue
		}
		out = append(out, actions.AIFirstAffectedEntityRef{
			EntityType: entityType,
			EntityID:   id,
		})
	}
	return out
}

func inferEntityTypeFromEntityID(entityID string) string {
	entityID = strings.TrimSpace(strings.ToLower(entityID))
	switch {
	case strings.HasPrefix(entityID, "sku:"), strings.HasPrefix(entityID, "sku_"):
		return "sku"
	case strings.HasPrefix(entityID, "campaign:"), strings.HasPrefix(entityID, "campaign_"):
		return "campaign"
	case strings.HasPrefix(entityID, "category:"), strings.HasPrefix(entityID, "category_"):
		return "category"
	case strings.HasPrefix(entityID, "warehouse:"), strings.HasPrefix(entityID, "warehouse_"):
		return "warehouse"
	case strings.HasPrefix(entityID, "account:"), strings.HasPrefix(entityID, "account_"):
		return "account"
	case strings.HasPrefix(entityID, "product:"), strings.HasPrefix(entityID, "product_"):
		return "product"
	case strings.HasPrefix(entityID, "funnel:"):
		return "funnel"
	default:
		return ""
	}
}

func normalizeStringIDArrayJSON(raw json.RawMessage) []string {
	if len(raw) == 0 || string(raw) == "null" {
		return []string{}
	}
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		return compactStringIDs(arr)
	}
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		return compactStringIDs([]string{single})
	}
	return []string{}
}

func parseFlexibleChunkSignals(raw json.RawMessage) ([]DashboardChunkSignal, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return []DashboardChunkSignal{}, nil
	}

	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		if signal, ok := parseOneFlexibleChunkSignal(raw); ok {
			return []DashboardChunkSignal{signal}, nil
		}
		return []DashboardChunkSignal{}, nil
	}

	out := make([]DashboardChunkSignal, 0, len(items))
	for _, item := range items {
		if signal, ok := parseOneFlexibleChunkSignal(item); ok {
			out = append(out, signal)
		}
	}
	return out, nil
}

func parseOneFlexibleChunkSignal(raw json.RawMessage) (DashboardChunkSignal, bool) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return DashboardChunkSignal{}, false
	}

	var signal DashboardChunkSignal
	unmarshalJSONField(fields, "signal_id", &signal.SignalID)
	unmarshalJSONField(fields, "signal_type", &signal.SignalType)
	unmarshalJSONField(fields, "observation", &signal.Observation)
	unmarshalJSONField(fields, "why_it_may_matter", &signal.WhyItMayMatter)

	if rawEntities, ok := fields["entity_ids"]; ok {
		signal.EntityIDs = normalizeStringIDArrayJSON(rawEntities)
	} else {
		signal.EntityIDs = []string{}
	}
	if rawFacts, ok := fields["fact_ids"]; ok {
		signal.FactIDs = normalizeStringIDArrayJSON(rawFacts)
	} else {
		signal.FactIDs = []string{}
	}
	if rawFactRefs, ok := fields["fact_refs"]; ok {
		signal.FactRefs = normalizeStringIDArrayJSON(rawFactRefs)
	}
	if rawEntityRefs, ok := fields["entity_refs"]; ok {
		signal.EntityRefs = normalizeStringIDArrayJSON(rawEntityRefs)
	}
	if rawMetrics, ok := fields["related_metrics"]; ok {
		signal.RelatedMetrics = normalizeStringIDArrayJSON(rawMetrics)
	}
	if rawLimits, ok := fields["limitations"]; ok {
		signal.Limitations = normalizeStringIDArrayJSON(rawLimits)
	}

	return signal, true
}
