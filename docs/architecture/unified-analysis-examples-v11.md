# Unified Analysis v11 examples

Fixtures live in `backend/internal/unifiedanalysis/contract/testdata/v11/`.

## Valid (12)

| Fixture | Intent |
| --- | --- |
| `material_multiple_problems` | Funnel + stock + ads + positive sales; 3 Actions |
| `no_material_actions` | Healthy / positive only; actions=[] |
| `insufficient_history` | history_missing; what_changed limited |
| `insufficient_advertising` | advertising unavailable |
| `positive_only` | Positive dynamics, no required Actions |
| `stockout_risk` | stockout Action |
| `critical_coverage` | coverage below policy threshold |
| `high_drr` | threshold claim + ad_high_drr_review |
| `spend_without_orders` | spend without orders |
| `weak_funnel_transition` | impression→view decline |
| `sku_stock_and_ads_independent` | Same commercial context, independent stock+ads Actions |
| `hypothesis_without_proven_causality` | Hypothesis stock amplifier; required Action from observed funnel only |

Each valid fixture has `.README.md` and `.facts.json`.

## Invalid (13)

| Fixture | Layer |
| --- | --- |
| `unknown_fact_id` | deferred_part2 (Fact Registry) |
| `unknown_metric_code` | semantic |
| `raw_number_in_narrative` | text |
| `stock_finding_in_funnel` | semantic |
| `action_without_finding` | semantic |
| `required_action_hypothesis_only` | semantic |
| `invalid_sku` | deferred_part2 |
| `duplicate_action` | semantic |
| `conflicting_action` | semantic |
| `invented_threshold` | semantic |
| `arbitrary_completion_target` | text |
| `positive_critical_finding` | semantic |
| `invalid_placeholder` | text |

## Placeholder example

Good:

`Конверсия составила {{claim:c_funnel_conversion.current}} и изменилась на {{claim:c_funnel_conversion.delta}}.`

Bad:

`Конверсия составила 13,3% и снизилась на 1,3 п.п.`
