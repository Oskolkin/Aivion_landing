package chatv2

import (
	"sort"
	"strings"
)

const (
	ToolGetPublicationOverview  = "get_publication_overview"
	ToolListPublicationFindings = "list_publication_findings"
	ToolListCurrentActions      = "list_current_actions"
	ToolGetActionContext        = "get_action_context"
	ToolSearchEntities          = "search_entities"
	ToolGetSKUContexts          = "get_sku_contexts"
	ToolGetCategoryContexts     = "get_category_contexts"
	ToolGetCampaignContexts     = "get_campaign_contexts"
	ToolGetAccountDomainFacts   = "get_account_domain_facts"
	ToolListSKUMetrics          = "list_sku_metrics"
	ToolListCategoryMetrics     = "list_category_metrics"
	ToolListCampaignMetrics     = "list_campaign_metrics"
	ToolSubmitTurnResult        = "submit_turn_result"
)

type ToolSpec struct {
	Key         string
	Version     string
	Description string
	Schema      map[string]any
}

func CatalogV1() []ToolSpec {
	periodProps := map[string]any{
		"period":    map[string]any{"type": []any{"string", "null"}, "enum": []any{"last_7d", "last_30d", "analysis_day", "custom", nil}},
		"date_from": map[string]any{"type": []any{"string", "null"}, "description": "YYYY-MM-DD for custom period"},
		"date_to":   map[string]any{"type": []any{"string", "null"}, "description": "YYYY-MM-DD for custom period"},
	}
	limit20 := map[string]any{"type": []any{"integer", "null"}, "minimum": 1, "maximum": 20}
	limit50 := map[string]any{"type": []any{"integer", "null"}, "minimum": 1, "maximum": 50}
	idArray := map[string]any{"type": "array", "items": map[string]any{"type": "integer"}, "maxItems": 20}
	return []ToolSpec{
		{Key: ToolGetPublicationOverview, Version: "v1", Description: "Current publication overview: identity, executive summary, limitations.", Schema: objectSchema(map[string]any{})},
		{Key: ToolListPublicationFindings, Version: "v1", Description: "Compact published findings index.", Schema: objectSchema(map[string]any{"limit": limit20})},
		{Key: ToolListCurrentActions, Version: "v1", Description: "Compact current published actions.", Schema: objectSchema(map[string]any{"limit": limit20})},
		{Key: ToolGetActionContext, Version: "v1", Description: "Bound Action group/case origin and current overlay.", Schema: objectSchema(map[string]any{})},
		{Key: ToolSearchEntities, Version: "v1", Description: "Seller-scoped entity search by query text.", Schema: objectSchema(map[string]any{"query": map[string]any{"type": "string"}, "limit": limit20})},
		{Key: ToolGetSKUContexts, Version: "v1", Description: "Batch SKU catalog/stock/price/sales context.", Schema: objectSchema(map[string]any{"sku_ids": idArray})},
		{Key: ToolGetCategoryContexts, Version: "v1", Description: "Batch category context.", Schema: objectSchema(map[string]any{"category_ids": idArray})},
		{Key: ToolGetCampaignContexts, Version: "v1", Description: "Batch campaign context.", Schema: objectSchema(map[string]any{"campaign_ids": idArray})},
		{Key: ToolGetAccountDomainFacts, Version: "v1", Description: "Account-level facts for one canonical domain.", Schema: objectSchema(merge(periodProps, map[string]any{"domain": map[string]any{"type": "string", "enum": []any{"SALES", "STOCK", "PRICE", "ADVERTISING", "FUNNEL", "SEARCH", "FINANCE", "PROMOTIONS", "ACCOUNT_HEALTH", "SUPPLY", "CATALOG"}}}))},
		{Key: ToolListSKUMetrics, Version: "v1", Description: "Bounded SKU metrics table via Factual API.", Schema: objectSchema(merge(periodProps, map[string]any{"limit": limit50, "sort": map[string]any{"type": []any{"string", "null"}, "enum": []any{"ordered_amount", "ordered_units", nil}}}))},
		{Key: ToolListCategoryMetrics, Version: "v1", Description: "Bounded category metrics table.", Schema: objectSchema(merge(periodProps, map[string]any{"limit": limit50}))},
		{Key: ToolListCampaignMetrics, Version: "v1", Description: "Bounded campaign metrics table.", Schema: objectSchema(merge(periodProps, map[string]any{"limit": limit50}))},
		{Key: ToolSubmitTurnResult, Version: "v1", Description: "Submit the structured final Chat result.", Schema: objectSchema(map[string]any{
			"answer": map[string]any{"type": "string"},
			"answer_mode": map[string]any{"type": "string", "enum": []any{
				"PUBLISHED_EXPLANATION", "FACTUAL_QUERY", "AD_HOC_ANALYSIS", "ACTION_ASSISTANCE",
				"ATTACHMENT_ANALYSIS", "MIXED_ANALYSIS", "EXPORT", "GENERAL_GUIDANCE",
			}},
			"referenced_entities": map[string]any{
				"type": "array",
				"items": objectSchema(map[string]any{
					"kind":  map[string]any{"type": "string"},
					"id":    map[string]any{"type": "string"},
					"label": map[string]any{"type": []any{"string", "null"}},
				}),
			},
			"grounding_handles": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"limitations":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"export_dataset": map[string]any{
				"type":                 []any{"object", "null"},
				"additionalProperties": false,
				"properties": map[string]any{
					"kind":   map[string]any{"type": []any{"string", "null"}, "enum": []any{"SKU_METRICS", "SALES", "SKU", nil}},
					"period": map[string]any{"type": []any{"string", "null"}, "enum": []any{"last_7d", "last_30d", "analysis_day", "previous_day", "current_7d", "trend_30d", nil}},
					"filters": map[string]any{
						"type": "array",
						"items": objectSchema(map[string]any{
							"field":    map[string]any{"type": "string", "enum": []any{"sku_id", "canonical_sku_id"}},
							"operator": map[string]any{"type": "string", "enum": []any{"eq"}},
							"value":    map[string]any{"type": "string"},
						}),
					},
				},
				"required": []string{"kind", "period", "filters"},
			},
		})},
	}
}

func objectSchema(props map[string]any) map[string]any {
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return map[string]any{"type": "object", "additionalProperties": false, "properties": props, "required": keys}
}

func merge(a, b map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

func RejectForbiddenArgs(args map[string]any) error {
	for k := range args {
		lk := strings.ToLower(strings.TrimSpace(k))
		switch lk {
		case "seller_id", "seller_account_id", "factual_universe_id", "universe_id", "analysis_cutoff_at", "cutoff", "cutoff_at":
			return ErrForbiddenToolArg
		}
	}
	return nil
}
