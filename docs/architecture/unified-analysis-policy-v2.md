# Unified Analysis Policy v2

**Version:** `analysis_policy_v2`  
**Package:** `backend/internal/unifiedanalysis/policy/v2`  
**Not wired into production packs** (production still ships `analysis_policy.json` v1 + `thresholdpolicy` Go registry).

## Contents

- `policy_version`
- `finding_type_registry` (36 types across sales/funnel/stock/advertising/sku/category/data_quality)
- `action_type_registry` (exact 13 v10 types)
- `action_type_to_category` (backend-owned mapping)
- `action_limits` (max_actions=15, max_top_actions=3, min material floors=1)
- `text_limits`, `block_status_rules`, `section_domain_rules`
- `metric_rendering_rules`
- `thresholds` (production values unchanged):
  - `high_drr_campaign_threshold`: campaign_drr_pct `>` 80 percent
  - `low_stock_coverage_days_threshold`: stock_coverage_days_7d `<` 7 days
- `completion_rules`, `priority_constraints`, `scope_rules`, `overlap_rules`
- `limitation_registry` (Pack codes + history_missing / source_incomplete / data_stale / future_supply_visibility_limited)
- `change_group_rules`, `sku_focus_limits` (7), `category_limits` (10)
- `recommended_check_types`, `causality_rules`
- `user_facing_text_fields` (raw-number prohibition paths)

## Finding severities

`critical` | `important` | `watch` | `none`

## Loader

```go
p, err := policyv2.Load()
h, err := policyv2.Hash() // must equal CanonicalPolicyHashV2
```

Immutable after hash pin. Changing JSON requires updating `CanonicalPolicyHashV2`.

## Mapping to v10 Action types

Each actionable finding type maps to one or more of the 13 existing Action types. No aliases. Category is never AI-authored.
