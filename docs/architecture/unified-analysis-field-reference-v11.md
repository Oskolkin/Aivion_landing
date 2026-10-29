# Unified Analysis Field Reference v11

Full field specification for `aivion_unified_analysis_v11`.  
**Ownership:** AI = authored by model; Backend = filled later by Part 2 assemblers; Validator = `v11validate` (+ Part 2 semantic layers).

Legend for **On error:** schema reject | semantic reject | text reject | publication block (Part 2).

---

## Root

### `schema_version`
- Type: string enum `aivion_unified_analysis_v11`
- Required
- Good: `aivion_unified_analysis_v11`
- Bad: `aivion_unified_analysis_v10`
- Owner: AI (must match request)
- Validator: schema
- On error: schema reject

### `analysis_meta`
See Analysis Meta.

### `analysis_outcome`
- Enum: `material_actions_found` | `no_material_actions` | `insufficient_data` | `analysis_failed`
- Rules: material ⇒ ≥1 material negative finding + ≥1 Action; no_material/insufficient/failed ⇒ `actions=[]`
- Owner: AI
- Validator: schema + semantic outcome rules (Part 2 completes outcome↔coverage)

### `metric_claims[]` / `threshold_claims[]` / `findings[]` / `dashboard` / `actions[]` / `limitations[]`
See sections below. All required arrays/objects (may be empty where allowed).

---

## Analysis Meta (`analysis_meta`)

| Path | Type | Req/Null | Limits | Good | Bad | Allowed | Forbidden | Owner | Validator | On error |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `pack_id` | string | required | copy from Pack | `pack_seller_1_2026_07_08_ab12` | `latest_pack` | exact Pack id | invent/shorten | AI copy | semantic meta match (Part 2) | reject |
| `pack_hash` | string | required | exact | full hash | truncated | exact Pack hash | recompute | AI copy | Part 2 | reject |
| `metric_date` | string | required | YYYY-MM-DD | `2026-07-08` | today / model date | Pack date | any other date | AI copy | Part 2 | reject |

---

## Metric Claim (`metric_claims[]`)

| Path | Type | Req/Null | Enum/Pattern | Good | Bad | Owner | Validator | On error |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `claim_id` | string | req | `^c_[a-z0-9_]{3,80}$` unique | `c_funnel_conversion` | `claim-1` | AI | v11validate | reject |
| `metric_code` | string | req | data dictionary / policy | `campaign_drr_pct` | `bad_ad_performance` | AI select | v11validate + Part 2 | reject |
| `scope_type` | string | req | account\|funnel\|sku\|campaign\|category | `funnel` | `mixed` | AI | schema | reject |
| `entity_id` | string\|null | req null | Pack entity | `801…` / null | product title | AI | Part 2 entity | reject |
| `current_fact_id` | string | req | Pack fact | `account:…:current_7d` | invented | AI | Part 2 Fact Registry | reject |
| `previous_fact_id` | string\|null | req null | comparable fact | previous or null | mismatched metric | AI | Part 2 | reject |
| `render_format` | string | req | money\|integer\|percent\|days\|units\|ratio\|decimal | `percent` | `currency_symbol` | AI | schema | reject |
| `delta_mode` | string | req | none\|absolute\|percent\|percentage_points | `percentage_points` | custom | AI | schema + Part 2 rules | reject |

Rules: `percentage_points` only for percent metrics; `previous_fact_id` required iff `delta_mode != none`.

---

## Threshold Claim (`threshold_claims[]`)

| Path | Type | Good | Bad | Owner | Validator |
| --- | --- | --- | --- | --- | --- |
| `claim_id` | `^t_[a-z0-9_]{3,80}$` | `t_high_drr_campaign` | same as metric claim id | AI | v11validate |
| `policy_id` | policy v2 id | `high_drr_campaign_threshold` | invented / value 80 inline | AI | v11validate/policy |
| `metric_claim_id` | existing claim | `c_campaign_drr` | missing | AI | v11validate |

AI must not emit operator/value/unit — backend renders from policy.

---

## Finding (`findings[]`)

| Path | Type | Enum/Pattern | Good | Bad | Owner | Validator |
| --- | --- | --- | --- | --- | --- | --- |
| `finding_id` | string | `^f_[a-z0-9_]{3,100}$` | `f_key_sku_stock_risk` | `12345` | AI | v11validate |
| `domain` | string | sales\|funnel\|stock\|advertising\|sku\|category\|data_quality | `funnel` | `account` | AI | schema+policy |
| `finding_type` | string | closed 36-type registry | `impression_to_view_decline` | free text | AI | policy |
| `primary_block` | string | account\|what_changed\|category\|funnel\|stock\|advertising\|seller_pulse\|sku_focus | `funnel` | dual blocks | AI | schema |
| `direction` | string | negative\|positive\|neutral | `negative` | — | AI | schema |
| `severity` | string | critical\|important\|watch\|none | `important` | positive+critical | AI | semantic |
| `interpretation` | string | observed\|derived\|hypothesis | `observed` | hypothesis+high confidence | AI | semantic |
| `title` | string | ≤120, no numbers | concrete thought | recommendation with numbers | AI | text rules |
| `explanation_template` | string | ≤350, placeholders OK | claim-based | raw % | AI | text |
| `business_impact_template` | string\|null | ≤250 | business meaning | exact $ promise | AI | text |
| `scope.scope_type` | string | account\|funnel\|sku\|campaign\|category | matching domain | mixed | AI | schema |
| `scope.entity_ids` | string[] | Pack ids | real ids | titles | AI | Part 2 |
| `metric_claim_ids` | string[] | existing | ≥1 for observed/derived | empty observed | AI | refs |
| `threshold_claim_ids` | string[] | existing | compatible | orphan | AI | refs |
| `confidence` | string | high\|medium\|low | `high` | hypothesis+high | AI | semantic |
| `limitation_codes` | string[] | Pack/policy codes | known code | invented | AI | Part 2 |

---

## Dashboard blocks

### Common
- `status`: available\|limited\|unavailable
- `limitation_codes`: string[]

### `account_diagnosis`
AI: headline, executive_summary_template, primary_finding_ids (≤3 material negative), positive_finding_ids (≤2), improvement_levers (≤4).  
Backend later: health_score, severity, risk_drivers, metric_date.  
Lever fields: title, explanation_template, finding_id, linked_action_id, expected_direction ∈ risk_decrease\|revenue_support\|conversion_improvement\|cost_efficiency\|stock_stability\|data_quality_improvement.

### `what_changed`
AI: summary_template; arrays worsened/new_risks/unchanged_risks/improved with `{change_id, finding_id, linked_action_id}`.  
Backend: labels, values, deltas, entities, deep links from change registry.

### `category_insights`
AI: items[{category_id, headline, diagnosis_template, finding_ids, linked_action_id, confidence}] ≤10.  
Backend: name, score, severity, factual metrics.  
Forbidden AI: category_score, severity, category_name, generic fallbacks.

### `funnel_insight`
AI: weakest_transition_key, headline, diagnosis/business_meaning/positive_context templates, recommended_checks (≤3), finding_ids (funnel domain), confidence.  
Backend: stages, values, conversions, deltas, labels.

### `stock_insight` / `advertising_insight`
AI: headline, diagnosis_template, primary/positive finding ids, recommended_checks, confidence.  
Backend: segments/counts/spend/DRR/etc.

### `seller_pulse_insight`
AI: headline, summary_template, key_point_finding_ids (2–4), important_fact_finding_ids (2–4), disjoint sets.  
Backend: KPIs, deltas, sparklines, trends.

### `sku_focus_selection`
AI: five arrays of `{sku_id, finding_id, primary_metric_code}` (≤7 each).  
Backend: names, images, metrics, display.

---

## Action (`actions[]`)

| Path | Notes |
| --- | --- |
| `analysis_action_id` | `^a_[a-z0-9_]{3,100}$` |
| `finding_ids` | 1–3; required Action cannot be hypothesis-only |
| `type` | exact 13 v10 types |
| `priority` | critical\|important\|watch |
| `execution_tier` | required\|recommended; watch≠required |
| `title` / `situation_template` / `goal` / `expected_effect_template` | no raw numbers; qualitative effect only |
| `scope` | account\|sku\|campaign\|funnel\|category; no mixed |
| `entity_ids` | mirror/subset of scope entities (schema requires top-level array) |
| `plan[]` | order 1..n; instruction/rationale/expected_result/entity_ids |
| `completion` | mode all\|any; conditions anyOf metric_improvement\|policy_threshold\|manual_confirmation (**no AI target_value**) |
| `object_instructions[]` | only when entities need different instructions |
| `limitation_codes` / `confidence` | Pack codes; confidence ≤ findings |

Max Actions: **15** (policy).

---

## Limitations (`limitations[]`)

`{ code, scope, affected_blocks }` — code from registry; AI does not author user-facing limitation text.

---

## User-facing fields (raw-number prohibition)

Applied to titles, headlines, templates (explanation/diagnosis/summary/situation/goal/effect/rationale/instruction/expected_result), lever titles/explanations.  
Not applied to IDs, enums, dates in meta, structured numerics (`order`, `period_days`), technical metadata.

---

## Validator error codes (Part 1)

`claim_invalid_id`, `claim_duplicate_id`, `claim_unknown_metric_code`, `threshold_*`, `finding_*`, `action_*`, `dashboard_finding_missing`, `dashboard_action_missing`, `raw_business_number`, `placeholder_unknown_part`, `html_forbidden`, `internal_field_name`, `section_domain_mismatch`, `action_hypothesis_only_required`, `action_conflict`, `threshold_unknown_policy`.
