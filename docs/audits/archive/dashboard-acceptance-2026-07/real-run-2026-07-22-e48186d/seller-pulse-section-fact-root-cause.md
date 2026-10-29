# Root cause — run 204 seller_pulse_ai_fact_mismatch

## Source of invalid reference

**AI output** (not backend finalizer / publication adapter).

Exact path:

```text
$.dashboard.seller_pulse.narrative.important_facts[1].claims[0].fact_id
```

Also mirrored on the same item:

```text
important_facts[1].fact_ids = [campaign:882000021528:DRR, campaign:882000031535:DRR]
important_facts[1].metric / metric_code = campaign_drr_pct
entity_ids = campaign IDs
```

Title/explanation (account-language packaging of a campaign issue):

```text
title: 2 рекламные кампании перевалили за 80% ДРР
explanation: В двух кампаниях зарегистрирован перерасход бюджета: ДРР превысил критический порог.
```

## Fact scope vs section scope

| Axis | Value |
| --- | --- |
| Fact scope | `campaign` (`campaign:<id>:DRR`) |
| Metric | `campaign_drr_pct` |
| Section | Seller Pulse (`primary_section=seller_pulse_important`) |
| Section scope required | account / store / seller aggregates only |

## Why prior validation behaved poorly

1. Prompt `v1_28` told the model to reconcile store share vs campaign DRR in Seller Pulse **summary text**, which the model over-applied into `important_facts` with campaign `fact_ids`.
2. Pack grounding input listed account facts but did not mark campaign facts as disallowed for Seller Pulse.
3. `validateSellerPulseClaimGrounding` only looked up `account_summary` metrics; missing campaign facts with `drr` in the metric name hard-failed as `seller_pulse_ai_fact_mismatch` with a generic “not found in account_summary” message — correct fail-closed, but **no section-aware repair** and no shared policy with the prompt.
4. No `FinalizeDashboardSectionFactReferences` existed; campaign refs were never stripped before strict validation.

## Fix direction (this commit)

- Closed `section_fact_policy_v1` shared by prompt hints / finalizer / validator.
- Deterministic finalizer drops Seller Pulse campaign/SKU/category refs; drops items that then lack account evidence.
- Prompt `unified_analyst_v1_29` forbids campaign facts in Seller Pulse; keeps them for Advertising/Actions/health entity evidence.
- Schema v10 unchanged; risk-driver finalization unchanged.
