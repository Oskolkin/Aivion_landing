package discovery

func OutputJSONSchema() map[string]any {
	entity := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"kind": map[string]any{"type": "string", "enum": []string{"ACCOUNT", "CATEGORY", "SKU", "CAMPAIGN"}},
			"id":   map[string]any{"type": "string"},
		},
		"required": []string{"kind", "id"},
	}
	evidence := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"evidence_type":          map[string]any{"type": "string"},
			"subject":                entity,
			"related_entity":         map[string]any{"anyOf": []any{entity, map[string]any{"type": "null"}}},
			"investigation_question": map[string]any{"type": "string"},
		},
		"required": []string{"evidence_type", "subject", "related_entity", "investigation_question"},
	}
	thread := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"primary_entity":      entity,
			"related_entities":    map[string]any{"type": "array", "items": entity},
			"observation":         map[string]any{"type": "string"},
			"investigation_focus": map[string]any{"type": "string"},
			"basis_fact_handles":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"evidence_requests":   map[string]any{"type": "array", "items": evidence},
		},
		"required": []string{"primary_entity", "related_entities", "observation", "investigation_focus", "basis_fact_handles", "evidence_requests"},
	}
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"discovery_state": map[string]any{"type": "string", "enum": []string{"THREADS_FOUND", "NO_MEANINGFUL_THREADS"}},
			"threads":         map[string]any{"type": "array", "items": thread},
		},
		"required": []string{"discovery_state", "threads"},
	}
}
