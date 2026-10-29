# Unified Analysis v10 → v11 mapping

| v10 AI field / concept | v11 |
| --- | --- |
| Free-text numbers in narratives | `{{claim:*}}` / `{{threshold:*}}` placeholders |
| Embedded metric_claims in seller_pulse / ad narrative | Top-level `metric_claims[]` |
| `ad_efficiency.narrative.threshold_claims` (inline policy copy) | Top-level `threshold_claims[]` referencing `policy_id` only |
| Implicit problems in Actions | Explicit `findings[]` |
| `account_summary` | `dashboard.account_diagnosis` (no health_score/risk_drivers) |
| `what_changed` free cards | `what_changed` change_id + finding_id selection |
| `category_summaries` + scores | `category_insights` selection/narratives only |
| `sales_funnel` + `funnel_analysis` | `funnel_insight` (+ backend stages later) |
| `stock_coverage` | `stock_insight` (no segments/totals) |
| `ad_efficiency` | `advertising_insight` (no factual summary) |
| `risk_distribution` | Removed from AI; backend-owned |
| `seller_pulse` KPIs+narrative | `seller_pulse_insight` finding selection + templates |
| `sku_focus` rich cards | `sku_focus_selection` ids + finding + metric_code |
| `top_action_ids` / `recommended_action_ids` | Removed from AI; Priority Policy (Part 2) |
| Action.category | Backend from type |
| Action scope `mixed` | Removed |
| completion target_value / baseline_value from AI | `metric_improvement` / `policy_threshold` / `manual_confirmation` without AI targets |
| limitations string[] | structured `{code,scope,affected_blocks}` |
| Status ok/partial/error | available/limited/unavailable |

## Post-processors (v11 path fate)

See baseline §13. Part 1 does not delete v10 functions.
