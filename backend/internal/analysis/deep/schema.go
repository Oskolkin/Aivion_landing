package deep

func OutputJSONSchema() map[string]any {
	entity := map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": map[string]any{
			"kind": map[string]any{"type": "string", "enum": []string{"ACCOUNT", "CATEGORY", "SKU", "CAMPAIGN"}},
			"id":   map[string]any{"type": "string"},
		},
		"required": []string{"kind", "id"},
	}
	handles := map[string]any{"type": "array", "items": map[string]any{"type": "string"}}
	ints := map[string]any{"type": "array", "items": map[string]any{"type": "integer"}}
	strs := map[string]any{"type": "array", "items": map[string]any{"type": "string"}}
	money := map[string]any{
		"anyOf": []any{
			map[string]any{
				"type": "object", "additionalProperties": false,
				"properties": map[string]any{
					"amount":   map[string]any{"type": "number"},
					"currency": map[string]any{"type": "string"},
				},
				"required": []string{"amount", "currency"},
			},
			map[string]any{"type": "null"},
		},
	}
	nstr := map[string]any{"anyOf": []any{map[string]any{"type": "string"}, map[string]any{"type": "null"}}}
	nint := map[string]any{"anyOf": []any{map[string]any{"type": "integer"}, map[string]any{"type": "null"}}}
	params := map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": map[string]any{
			"recommended_price":              money,
			"recommended_units":              nint,
			"recommended_units_reduction":    nint,
			"campaign_ref":                   nstr,
			"recommended_bid":                money,
			"desired_state":                  nstr,
			"promotion_ref":                  nstr,
			"recommended_action_price":       money,
			"primary_rating_key":             nstr,
			"related_rating_keys":            strs,
			"provider_error_types":           strs,
			"direction":                      nstr,
			"affected_material_sku_refs":     strs,
			"intent":                         nstr,
			"related_material_campaign_refs": strs,
			"recommended_budget":             money,
			"budget_period":                  nstr,
			"desired_strategy":               nstr,
			"add_sku_refs":                   strs,
			"remove_sku_refs":                strs,
		},
		"required": []string{
			"recommended_price", "recommended_units", "recommended_units_reduction",
			"campaign_ref", "recommended_bid", "desired_state", "promotion_ref",
			"recommended_action_price", "primary_rating_key", "related_rating_keys",
			"provider_error_types", "direction", "affected_material_sku_refs", "intent",
			"related_material_campaign_refs", "recommended_budget", "budget_period",
			"desired_strategy", "add_sku_refs", "remove_sku_refs",
		},
	}
	rel := map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": map[string]any{
			"relationship_statement":   map[string]any{"type": "string"},
			"relationship_basis":       map[string]any{"type": "string", "enum": []string{"DESCRIPTIVE", "CO_MOVEMENT", "TEMPORAL_ASSOCIATION", "PLAUSIBLE_CONTRIBUTOR", "PROVIDER_ATTRIBUTED", "CAUSAL_ESTIMATE"}},
			"driver_or_factor":         map[string]any{"type": "string"},
			"outcome":                  map[string]any{"type": "string"},
			"supporting_fact_handles":  handles,
			"counter_fact_handles":     handles,
			"limitations":              strs,
			"alternative_explanations": strs,
		},
		"required": []string{"relationship_statement", "relationship_basis", "driver_or_factor", "outcome", "supporting_fact_handles", "counter_fact_handles", "limitations", "alternative_explanations"},
	}
	takeaway := map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": map[string]any{
			"statement":               map[string]any{"type": "string"},
			"direction":               map[string]any{"type": "string", "enum": []string{"POSITIVE", "NEGATIVE", "MIXED", "NEUTRAL", "UNCERTAIN"}},
			"primary_domain":          map[string]any{"type": "string", "enum": CanonicalPrimaryDomains()},
			"supporting_fact_handles": handles,
			"finding_ordinals":        ints,
		},
		"required": []string{"statement", "direction", "primary_domain", "supporting_fact_handles", "finding_ordinals"},
	}
	exec := map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": map[string]any{
			"headline":                    map[string]any{"type": "string"},
			"summary":                     map[string]any{"type": "string"},
			"key_takeaways":               map[string]any{"type": "array", "items": takeaway},
			"basis_fact_handles":          handles,
			"referenced_finding_ordinals": ints,
			"limitations":                 strs,
		},
		"required": []string{"headline", "summary", "key_takeaways", "basis_fact_handles", "referenced_finding_ordinals", "limitations"},
	}
	tres := map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": map[string]any{
			"discovery_thread_ref":       map[string]any{"type": "string"},
			"resolution":                 map[string]any{"type": "string", "enum": []string{"SUBSTANTIATED", "REFRAMED", "NOT_SUBSTANTIATED", "INCONCLUSIVE"}},
			"resolution_summary":         map[string]any{"type": "string"},
			"supporting_fact_handles":    handles,
			"counter_fact_handles":       handles,
			"limitations":                strs,
			"resulting_finding_ordinals": ints,
		},
		"required": []string{"discovery_thread_ref", "resolution", "resolution_summary", "supporting_fact_handles", "counter_fact_handles", "limitations", "resulting_finding_ordinals"},
	}
	finding := map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": map[string]any{
			"ordinal":                  map[string]any{"type": "integer"},
			"origin_type":              map[string]any{"type": "string", "enum": []string{"THREAD_DERIVED", "CROSS_THREAD_SYNTHESIS", "EVIDENCE_EMERGENT", "ACCOUNT_CONTEXT"}},
			"primary_entity":           entity,
			"related_entities":         map[string]any{"type": "array", "items": entity},
			"title":                    map[string]any{"type": "string"},
			"finding_statement":        map[string]any{"type": "string"},
			"business_direction":       map[string]any{"type": "string", "enum": []string{"POSITIVE", "NEGATIVE", "MIXED", "NEUTRAL", "UNCERTAIN"}},
			"primary_domain":           map[string]any{"type": "string", "enum": CanonicalPrimaryDomains()},
			"overview_bucket":          map[string]any{"type": "string", "enum": CanonicalOverviewBuckets()},
			"relationship_assessments": map[string]any{"type": "array", "items": rel},
			"supporting_fact_handles":  handles,
			"counter_fact_handles":     handles,
			"limitations":              strs,
			"source_thread_refs":       strs,
		},
		"required": []string{"ordinal", "origin_type", "primary_entity", "related_entities", "title", "finding_statement", "business_direction", "primary_domain", "overview_bucket", "relationship_assessments", "supporting_fact_handles", "counter_fact_handles", "limitations", "source_thread_refs"},
	}
	rec := map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": map[string]any{
			"ordinal":                  map[string]any{"type": "integer"},
			"target_entity":            entity,
			"recommendation_statement": map[string]any{"type": "string"},
			"rationale":                map[string]any{"type": "string"},
			"support_finding_ordinals": ints,
			"support_fact_handles":     handles,
			"limitations":              strs,
			"actionability":            map[string]any{"type": "string", "enum": []string{"ACTIONABLE_NOW", "MONITOR", "INSUFFICIENT_EVIDENCE"}},
		},
		"required": []string{"ordinal", "target_entity", "recommendation_statement", "rationale", "support_finding_ordinals", "support_fact_handles", "limitations", "actionability"},
	}
	revalidation := map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": map[string]any{
			"evidence_type": map[string]any{"type": "string"},
			"subject":       entity,
			"related_entity": map[string]any{
				"anyOf": []any{entity, map[string]any{"type": "null"}},
			},
		},
		"required": []string{"evidence_type", "subject", "related_entity"},
	}
	act := map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": map[string]any{
			"ordinal":                map[string]any{"type": "integer"},
			"action_type":            map[string]any{"type": "string"},
			"target":                 entity,
			"parameters":             params,
			"recommendation_ordinal": map[string]any{"type": "integer"},
			"finding_ordinals":       ints,
			"fact_handles":           handles,
			"supports": map[string]any{"type": "array", "items": map[string]any{
				"type": "object", "additionalProperties": false,
				"properties": map[string]any{
					"support_key":      map[string]any{"type": "string"},
					"ordinal":          map[string]any{"type": "integer"},
					"role":             map[string]any{"type": "string", "enum": []string{"REQUIRED_SUPPORT", "CONTEXT", "COUNTEREVIDENCE"}},
					"claim":            map[string]any{"type": "string"},
					"finding_ordinals": ints,
					"fact_handles":     handles,
					"revalidation":     revalidation,
				},
				"required": []string{"support_key", "ordinal", "role", "claim", "finding_ordinals", "fact_handles", "revalidation"},
			}},
		},
		"required": []string{"ordinal", "action_type", "target", "parameters", "recommendation_ordinal", "finding_ordinals", "fact_handles", "supports"},
	}
	prev := map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": map[string]any{
			"previous_action_ref":                map[string]any{"type": "string"},
			"assessment":                         map[string]any{"type": "string", "enum": []string{"FULLY_SUPPORTED", "PARTIALLY_SUPPORTED", "NOT_SUPPORTED", "UNVERIFIABLE", "SUPERSEDED"}},
			"assessment_summary":                 map[string]any{"type": "string"},
			"supporting_fact_handles":            handles,
			"counter_fact_handles":               handles,
			"limitations":                        strs,
			"resulting_action_proposal_ordinals": ints,
			"support_assessments": map[string]any{"type": "array", "items": map[string]any{
				"type": "object", "additionalProperties": false,
				"properties": map[string]any{
					"support_key":          map[string]any{"type": "string"},
					"status":               map[string]any{"type": "string", "enum": []string{"HOLDS", "CHANGED", "NOT_VERIFIABLE"}},
					"current_fact_handles": handles,
					"assessment_summary":   map[string]any{"type": "string"},
				},
				"required": []string{"support_key", "status", "current_fact_handles", "assessment_summary"},
			}},
		},
		"required": []string{"previous_action_ref", "assessment", "assessment_summary", "supporting_fact_handles", "counter_fact_handles", "limitations", "resulting_action_proposal_ordinals", "support_assessments"},
	}
	return map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": map[string]any{
			"executive_summary":           exec,
			"thread_resolutions":          map[string]any{"type": "array", "items": tres},
			"findings":                    map[string]any{"type": "array", "items": finding},
			"recommendations":             map[string]any{"type": "array", "items": rec},
			"action_proposals":            map[string]any{"type": "array", "items": act},
			"previous_action_assessments": map[string]any{"type": "array", "items": prev},
		},
		"required": []string{"executive_summary", "thread_resolutions", "findings", "recommendations", "action_proposals", "previous_action_assessments"},
	}
}
