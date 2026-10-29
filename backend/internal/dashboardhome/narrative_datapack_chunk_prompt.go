package dashboardhome

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/actions"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/aidatapack"
)

func systemPromptDashboardChunkAnalysis() string {
	return strings.TrimSpace(`
You analyze one technical chunk of SellerAIDataPack in a single JSON response.
Do not produce final dashboard content or final user-facing action cards.
Do not assume this chunk is the whole seller account.
Return JSON only. No markdown. No explanations outside JSON.

You have two tasks in one response:
1) Signals — what happened or was detected in this chunk.
2) Action Candidates — whether detected situations require practical action and what to do locally.

SIGNALS answer: "What happened or was detected in the data of this chunk?"
A signal describes observations, changes, local risks, local opportunities, data quality issues, or relationships.
A signal is NOT a user task.

ACTION CANDIDATES answer: "Does the detected situation require practical action and what should roughly be done?"
A candidate is a local structured draft, not a final Actions card.
Do not create a final title, long summary, action plan, completion criteria, expected effect, CTA, Top 3, or focus of the day.

You must use IDs exactly as provided.

entity_id examples:
- sku:801000100001
- campaign:882000011632
- category:17028922
- account:seller
- funnel:account

fact_id examples:
- sku.801000100001.last_7d.revenue
- campaign.882000011632.last_7d.drr_pct
- account.current_day.revenue

Signal rules:
1. fact_ids may contain only keys from allowed_fact_ids.
2. entity_ids may contain only keys from allowed_entity_ids.
3. Never put entity_id into fact_ids.
4. Never put fact_id into entity_ids.
5. Never invent fact_id.
6. Never transform fact_id format.
7. If you cannot find a supporting fact_id, do not create the signal.
8. Every signal must have at least one fact_id from allowed_fact_ids (except pure data_quality limitation notes with analysis_status=insufficient_data).
9. Every entity_id must exist in allowed_entity_ids.
10. Do not use SKU-level advertising spend attribution.
11. Do not discuss profit/margin if finance is unavailable.
12. Do not recommend price cuts or price increases.
13. Do not promise exact financial effect.

Action candidate rules:
1. Not every signal must become a candidate. Useful observations without required action should stay signals only.
2. Group similar objects with the same problem and same recommended action into one candidate within this chunk.
3. Do not merge different business problems that require different actions.
4. Use only entity_id and fact_id from this chunk catalogs.
5. source_signal_ids must reference signal_id values from the signals array in the same response.
6. Do not fill candidate_id or chunk_id — backend assigns them.
7. local_priority reflects importance only inside this chunk, not global store priority.
8. Do not create SKU-level advertising recommendations without explicit SKU-level ad metrics in this chunk.
9. Campaign-level ad data must lead to campaign-level conclusions only.
10. Do not calculate profit, margin, or promise guaranteed outcomes.
11. Do not recommend price changes.
12. Each candidate must be rich enough for a future operational action card:
    - problem: concrete local problem with objects/metrics from facts (not a generic theme);
    - why_actionable: specific business risk using cited metrics/periods from fact_ids;
    - recommended_action: draft operational direction for these objects (not "analyze" or "consider measures");
    - affected_entities: include ALL matching sku/campaign/category/warehouse objects when count ≤20; never use representative examples when full scope fits in 20;
    - evidence_fact_ids: include all facts needed to describe metrics (stock, orders, revenue, spend, DRR, idc_days, days_without_sales, drops, conversion);
    - each entity in affected_entities must have its own fact_ids for downstream Actions.
13. Do not invent numbers. Use only values available through fact_ids.
14. Action candidate taxonomy (choose type and matching category):
    - stock present but no sales → type=no_sales_with_stock, category=slow_turnover;
    - excess stock / low turnover → type=slow_turnover_review, category=slow_turnover;
    - stock_redistribution → only if chunk contains multi-warehouse data;
    - stock deficit / replenishment → type=stock_replenishment, category=stock.
15. Do not write object counts in problem/recommended_action that differ from len(affected_entities).

METRIC SEMANTICS (mandatory):
- idc_days = покрытие запасом в днях (сколько дней хватит текущего запаса при текущем спросе).
- Use «покрытие запасом» or «запаса хватит на N дней».
- NEVER use «срок хранения», «дни хранения», «срок годности» for idc_days.

For v3 prefer local refs from fact_ref_catalog / entity_ref_catalog instead of canonical IDs.

Response limits:
- signals: max 10
- action_candidates: max 5
- signal fact_ids: max 8 each
- signal entity_ids: max 5 each
- candidate evidence_fact_ids: max 12 each
- candidate affected_entities: max 20 each

If many rows are similar, summarize as a pattern and cite representative fact_ids.
Keep problem, recommended_action, and why_actionable concise but concrete — include object counts, metric names and periods when available in facts.
Do not write generic candidates like "products with excess stock and no sales" without listing affected entity_ids and supporting fact_ids.

INSUFFICIENT DATA:
If this chunk has no independent grounded business signal (change, risk, opportunity, or data-quality issue),
do not invent a signal just to fill the array.
Return analysis_status="insufficient_data" with signals=[] and action_candidates=[].
This is valid and not an error — especially for entity_dictionary chunks that only list reference entities.
Never return analysis_status="completed" unless you can produce at least one grounded signal with valid fact_ids.
Do not use entity IDs instead of fact IDs. Do not create an action candidate without a valid source signal.
`)
}

func userPromptDashboardChunkAnalysis(chunk aidatapack.DataPackChunk) string {
	allowedFacts := sortedAllowlistKeys(chunkLocalFactIDAllowlist(chunk))
	allowedEntities := sortedAllowlistKeys(chunkLocalEntityIDAllowlist(chunk))
	allowedTypes := chunkAnalysisActiveActionTypesForPrompt()
	allowedTypesJSON, _ := json.Marshal(allowedTypes)
	guide := strings.TrimSpace(`
ID GUIDE:
- Use fact_ids only from allowed_fact_ids.
- Use entity_ids only from allowed_entity_ids.
- fact_id and entity_id are different identifiers.
- A SKU entity_id is not a fact_id.
- A campaign entity_id is not a fact_id.
- source_signal_ids must reference signals from this same JSON response only.
`)
	allowedJSON, _ := json.Marshal(map[string]any{
		"allowed_fact_ids":   allowedFacts,
		"allowed_entity_ids": allowedEntities,
	})
	return fmt.Sprintf(strings.TrimSpace(`
Analyze the provided SellerAIDataPack chunk and return %s JSON in one response.
Signals and action_candidates must be produced together. No second AI call is expected.

%s

ALLOWED_IDS_JSON:
%s

ALLOWED_ACTION_TYPES_JSON:
%s

ALLOWED_LOCAL_PRIORITIES: critical, important, watch

Required root fields:
- schema_version: %q
- chunk_id: %q
- chunk_type: %q
- analysis_status: "completed" or "insufficient_data"
- signals: array of grounded signals (max 10); use [] when insufficient_data
- action_candidates: array of local action drafts (max 5); use [] when insufficient_data

Use analysis_status="insufficient_data" with empty signals and action_candidates when this chunk
has no independent grounded business signal. Never use completed without at least one valid signal.

Each signal must include:
- signal_id (unique within chunk, e.g. "%s-signal-01")
- signal_type: observation|change|risk_candidate|opportunity_candidate|data_quality|relationship
- entity_refs: from entity_ref_catalog when using v3 (preferred)
- fact_refs: from fact_ref_catalog when using v3 (preferred)
- entity_ids / fact_ids: legacy only when refs unavailable
- observation: concise Russian text grounded in cited refs

Each action_candidate must include:
- type: one of allowed action types
- category: matching category for the type
- local_priority: critical|important|watch
- problem
- recommended_action
- why_actionable
- affected_entities: at least one local object (entity_ref preferred)
- evidence_fact_refs: at least one local fact_ref (preferred)
- source_signal_ids: at least one signal_id from this response

Do not output candidate_id or chunk_id inside action_candidates.
Acknowledge chunk limitations in limitations_acknowledged when relevant.
`), DashboardChunkAnalysisSchemaVersion, guide, string(allowedJSON), string(allowedTypesJSON),
		DashboardChunkAnalysisSchemaVersion, chunk.ChunkID, chunk.ChunkType, chunk.ChunkID)
}

func chunkAnalysisActiveActionTypesForPrompt() []string {
	out := make([]string, 0, len(actions.ActionsAIFirstActiveActionTypes))
	for _, t := range actions.ActionsAIFirstActiveActionTypes {
		out = append(out, string(t))
	}
	return out
}

func chunkAnalysisContextForAI(chunk aidatapack.DataPackChunk) map[string]any {
	mapping := BuildChunkRefMapping(chunk)
	metrics := aidatapack.MetricsPresentInChunk(chunk)
	return map[string]any{
		"allowed_fact_ids":     sortedAllowlistKeys(chunkLocalFactIDAllowlist(chunk)),
		"allowed_entity_ids":   sortedAllowlistKeys(chunkLocalEntityIDAllowlist(chunk)),
		"fact_ref_catalog":     mapping.factRefCatalog(),
		"entity_ref_catalog":   mapping.entityRefCatalog(),
		"metric_semantics":     aidatapack.MetricSemanticsForCodes(metrics...),
		"allowed_action_types": chunkAnalysisActiveActionTypesForPrompt(),
		"allowed_local_priority": []string{
			string(actions.ActionPriorityCritical),
			string(actions.ActionPriorityImportant),
			string(actions.ActionPriorityWatch),
		},
		"chunk": chunk,
	}
}
