# Unified Analysis Contract v11

**Status:** Part 1 — registered, not production-current  
**Schema:** `aivion_unified_analysis_v11`  
**Prompt:** `unified_analyst_v1_32`  
**Policy:** `analysis_policy_v2`  
**Production remains:** `aivion_unified_analysis_v10` + `unified_analyst_v1_31`

## Goal

Single Unified Analysis AI call that returns interpretation only:

`backend fact → metric claim → finding → Dashboard block → Action → publication`

AI must not invent business numbers, thresholds, KPI, category scores, health score, Top Actions, or risk distribution.

## Top-level shape

```json
{
  "schema_version": "aivion_unified_analysis_v11",
  "analysis_meta": { "pack_id": "...", "pack_hash": "...", "metric_date": "YYYY-MM-DD" },
  "analysis_outcome": "material_actions_found|no_material_actions|insufficient_data|analysis_failed",
  "metric_claims": [],
  "threshold_claims": [],
  "findings": [],
  "dashboard": {
    "account_diagnosis": {},
    "what_changed": {},
    "category_insights": {},
    "funnel_insight": {},
    "stock_insight": {},
    "advertising_insight": {},
    "seller_pulse_insight": {},
    "sku_focus_selection": {}
  },
  "actions": [],
  "limitations": []
}
```

## Ownership split

| Owner | Responsibility |
| --- | --- |
| Backend | Facts, numbers, periods, units, entities, dictionaries, thresholds, health score, risk distribution, Top Actions, completion baselines, publication, tenant isolation |
| AI | Interpretation, findings, relationships, wording, object selection, operational plan, qualitative expected effect |
| Validator | Reject AI leaving its authority |

## Placeholders (user-facing text only)

- `{{claim:<id>.current|previous|delta|period_current|period_previous}}`
- `{{threshold:<id>.value|operator}}`

Raw business numbers in user-facing fields are forbidden.

## IDs

| Kind | Pattern |
| --- | --- |
| Metric claim | `^c_[a-z0-9_]{3,80}$` |
| Threshold claim | `^t_[a-z0-9_]{3,80}$` |
| Finding | `^f_[a-z0-9_]{3,100}$` |
| Action | `^a_[a-z0-9_]{3,100}$` |

## Block status

`available` | `limited` | `unavailable` (replaces ok/partial/error).

## Removed from AI (vs v10)

health_score, risk_drivers, risk_distribution, top_action_ids, recommended_action_ids, Action.category, mixed scope, AI factual funnel/stock/ad/KPI arrays, category_score fallbacks, arbitrary completion target_value.

## OpenAI schema notes

Provider schema uses strict Structured Outputs subset: `additionalProperties:false`, all properties required, nullable via `["string","null"]`, `anyOf` for completion conditions. Regex/string length/status conditionals are enforced in `contract/v11validate` + policy (not all keywords are OpenAI-strict safe).

## Files

| Artifact | Path |
| --- | --- |
| Schema | `backend/internal/unifiedanalysis/contract/schema/unified_analysis_output_v11.schema.json` |
| Go types | `backend/internal/unifiedanalysis/contract/output_v11.go` |
| Policy | `backend/internal/unifiedanalysis/policy/v2/` |
| Prompt | `backend/internal/unifiedanalysis/prompt/analyst_v1_32*.txt` |
| Validator | `backend/internal/unifiedanalysis/contract/v11validate/` |
| Fixtures | `backend/internal/unifiedanalysis/contract/testdata/v11/` |

See also: field reference, policy doc, examples, v10→v11 mapping, v10 baseline.
