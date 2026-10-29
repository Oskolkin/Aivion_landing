# Root cause: advertising_total_revenue_share_mismatch (run 205 / publication 91)

## Exact validator

- Package: `backend/internal/unifiedanalysis/publication/bundlevalidator`
- Function: `checkAdvertisingNarrativeArithmetic` → (pre-fix) Russian sentence heuristics in the same file
- Called from: `checkAccountDRRConsistency` → `ValidatePublicationBundle`
- Stage: **publication bundle validation** (after output validation already PASS)

## Exact JSON path

```text
ad_efficiency.narrative.summary
(field reported on Issue; percent also appears in headline)
```

## Extracted value

```text
53.3
```

Source: regex `accountDRRNarrativeRe` over a sentence that includes both
store-cabinet wording and attributed DRR wording. The first `ДРР…%` hit was
`ДРР кампаний составляет 53,3%`.

## Expected value (incorrectly chosen)

```text
~26.6
```

Source: `TotalSpendToday / TotalStoreRevenueToday * 100` (day-level store
advertising spend share), compared because `cabinetDRRContextRe` matched the
same sentence (`кабинета` / `общей выручке`).

## Structured claims that already grounded the narrative (output validation PASS)

| Text | metric_code | scope | fact_id | claim value |
|------|-------------|-------|---------|-------------|
| 53.3% | `attributed_campaign_drr_pct` | account | `account:attributed_campaign_drr_pct:current_7d` | 53.33 |
| 26.2% | `ad_spend_share_of_total_store_revenue_pct` | account | `account:ad_spend_share_of_total_store_revenue_pct:current_7d` | 26.23 |
| (policy) | threshold `campaign_drr_pct` gt 80 | campaign | policy `high_drr_campaign_threshold` | 80 |

Note: 53.3% is **account attributed campaign DRR**, not a per-campaign
`campaign_drr_pct` row. Preliminary “campaign DRR vs store share” wording was
close but incomplete.

## Why output validation passed but publication failed

1. `outputvalidator` already validates metric_claims / threshold_claims against
   Pack facts and grounds free-text percents by **claim value** (no Russian
   metric identity).
2. `projectAdEfficiencyNarrative` dropped `metric_claims` / `threshold_claims`
   from the frozen dashboard projection.
3. Bundle validator re-scanned Russian narrative and treated any `%` near
   cabinet wording as store spend share.
4. `attributedRevenueContextRe` uses `\w` after `атрибут`, which in Go RE2 is
   ASCII-only — so `атрибутированный` never set `mentionsAttr`, leaving
   `mentionsStore=true` alone for the dual-metric sentence.

## Semantic misclassification

```text
attributed_campaign_drr_pct (53.3)
  → incorrectly compared to
store ad spend / store revenue (26.6)
```

## Fix direction (this commit)

- Preserve claims through ProjectDashboard projection.
- Bundle advertising arithmetic is claim-aware when claims are present.
- Legacy Russian percent scan remains only for claim-less historical publications.
- Narrative text is not rewritten.
