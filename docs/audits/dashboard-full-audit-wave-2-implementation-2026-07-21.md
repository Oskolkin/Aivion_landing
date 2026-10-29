# Dashboard full audit — Wave 2 implementation (2026-07-21)

## Status

**Wave 2 code + pre-acceptance fixes + automated tests: DONE pending commit + §9.**

Pre-acceptance (2026-07-22) closed before real-run:

1. Immutable prompt bodies: `analyst_v1_22.txt` / `_input` (baseline `e0dfc32`) and `analyst_v1_24.txt` / `_input`; `v1_23` registry `status=never_published` (no invented body).
2. Sparse SKU Focus: cards retained with `trend_status` / `trend_message`; unresolved identity / missing issue still block publication.

Baseline HEAD before this commit:

```text
e0dfc3227e9d92b2a28a8590fec7090d651907a4
```

---

## 1. Schema / prompt versioning

### Problem found

Wave 1 edited `aivion_unified_analysis_v8` / `unified_analyst_v1_22` **in-place** (`seller_pulse_key_point`, `account_drr_pct` grounding). That violated immutability: old and new runs could share version strings with different content.

### Fix

| Item | Value |
|------|-------|
| Schema current | `aivion_unified_analysis_v9` |
| Schema file | `backend/internal/unifiedanalysis/contract/schema/unified_analysis_output_v9.schema.json` |
| Schema hash | `sha256:a4c3ede2825329de46f822edda057c22837d4345276b4bbddc4dad296f6a8717` |
| Schema v8 (immutable) | restored; **no** `seller_pulse_key_point`; loadable via `SchemaJSONByVersion(v8)` |
| Prompt current | `unified_analyst_v1_24` |
| Prompt hash | `fd41711bfb17d783e3efce7e66e7003cb214538662990ae5ff1a2a061aa955d6` |
| Intermediate label | `unified_analyst_v1_23` kept in registry (schema-v9 bump); **v1_24** adds required `ad_efficiency.narrative.metric_claims` |

Note on preferred `v1_23`: user preference was `v1_23`. Wave 2 needed a second immutable bump for advertising metric_claims, so production current is **v1_24**. `v1_22` / `v1_23` remain version constants; shared `analyst_v1.txt` body is the production template (historical per-label body immutability is still limited by the single-file prompt embed pattern).

Hashes are used by:

- publication policy (`AcceptedPromptHash` / `AcceptedSchemaHash`);
- run stamps / validation artifacts;
- publication identity / allowlist;
- Inspector / audit surfaces that read run prompt/schema version+hash.

Compatibility: fixtures bumped to v9; `SchemaJSONByVersion` serves v8 and v9 separately.

---

## 2. Backend Key SKU 80% cohort

### Algorithm (`dashboardfacts`, policy `key_sku_cohort_v1`)

1. Take all SKUs with `revenue_7d > 0`.
2. `store_total_revenue_7d = sum(revenue_7d)`.
3. Sort `revenue_7d DESC`, tie-break stable SKU / ozon_product_id ASC.
4. `revenue_share_pct = revenue_7d / store_total × 100` (store denominator — **not** key-SKU subtotal).
5. Cumulative share; include until cumulative ≥ **80%**.
6. Zero store revenue → cohort `not_enough_data`, empty items (no fake 0% shares).

### AI interaction

- Membership / rank / shares: **backend only** (`projectKeySKUCardsFromCohort`).
- AI may overlay product-name copy for cohort members; cannot define membership.
- Bundle validator: `unified_dashboard_key_sku_cohort_mismatch` enforces published Key SKUs match cohort order, ids, shares, store denominator.

Tests: `dashboardfacts/key_sku_cohort_test.go` + bundlevalidator membership check.

---

## 3. Advertising text rewrite removed

### Removed

- `groundStoreRevenueShareMentions`
- `groundAttributedShareMentions`
- `groundAccountDRRMentions`
- `groundPercentMentions`
- percent-rewrite call sites in `projectAdEfficiencyNarrative`

Adapter now **passthrough** AI headline/summary/checklist/limitations.

### Replaced with claim validation (`outputvalidator/advertising_claims.go`)

Canonical metric codes:

```text
ad_spend_share_of_total_store_revenue_pct
attributed_campaign_drr_pct
ad_spend
store_total_revenue
attributed_ad_revenue
ad_orders
```

Blocks (examples): unknown metric code, missing period, value mismatch, free-text `%` without claim, «ДРР» for store share, revenue denominator confusion.

v9 schema: `ad_efficiency.narrative.metric_claims` required. Original AI text preserved; no semantic rewrite.

---

## 4. Frozen Top 3 on GET

- `shouldOverrideTopActionsFromFinal(promptVersion)` → **false** for Unified adapter prompt version.
- `overrideTopActionsFromFinalActions` only on legacy path.
- Bundle codes already cover max 3, unresolved ids, duplicates, artificial fill.

Tests: `wave2_freeze_test.go`.

---

## 5. Frozen SKU Focus semantics on GET

| Operation | Unified GET |
|-----------|-------------|
| `omitDeprecatedSKUFocusSurplus` (surplus→slow_turnover) | **skipped** |
| `enrichFrozenSKUFocusFromMetrics` | **skipped** (`ApplyFrozenDashboardDataUnified`) |
| Unresolved AI focus entity at publish | **blocks** via `sku_focus_unresolved_entity` → `unified_dashboard_sku_focus_unresolved` |
| Sparse sparkline (≥2 points missing) | card skipped (identity resolved); not silent unresolved drop |

Taxonomy tabs remain: `order_drop`, `stock_risk`, `no_sales_with_stock`, `slow_turnover`, `surplus` (legacy remap only), `key_skus`.

---

## 6. Health drivers — backend registry

Package: `backend/internal/dashboardhome/healthpolicy` (`health_policy_v1`).

- Backend builds candidate registry from facts (known driver ids incl. sales/revenue decline, stockout, surplus, funnel declines, high DRR, spend without orders, ad share growth, data_limitation, …).
- **Score = full registry** via `ComputeHealthScore` → `ComputeHealthScoreBreakdown`.
- AI may select/order/explain known ids only; unknown ids → `ai_risk_driver_unknown_id` → publication blocked.
- Publication stamps `health_policy_version` + `health_facts_hash`.

Invariants covered by tests: same facts → same score; different AI selections → same score; AI text changes do not move score; missing optional ads does not tank score; insufficient core data → nil score.

---

## 7. GET mutation inventory (`BuildHome` / finalize)

Unified path loads frozen snapshot → `ApplyFrozenDashboardDataUnified` → narrative overlay → `finalizeSellerFacingDashboardResponse` (with Unified guards).

| Function | Unified вызывается? | Меняет факты? | Меняет смысл? | Presentation only? | Решение |
|----------|--------------------:|--------------:|--------------:|-------------------:|---------|
| `ApplyFrozenDashboardDataUnified` | yes | no | no | yes (copy frozen) | keep |
| `enrichFrozenSKUFocusFromMetrics` | **no** | — | — | — | legacy only |
| `overrideTopActionsFromFinalActions` | **no** | — | — | — | legacy only |
| `repairSellerFacingWhatChangedConclusion` | **no** | — | — | — | legacy only |
| `omitDeprecatedSKUFocusSurplus` | **no** | — | — | — | legacy only |
| `applyHealthStatusFromScore` | yes | no | no* | yes (status labels from score) | keep presentation |
| `applyPeriodContextFromMetricDate` | yes | no | no | yes | keep |
| `localizeSellerPulseKPILabels` | yes | no† | no† | yes (RU labels; key alias normalize) | keep / monitor |
| `reconcileSellerPulseQuickActionsWithTopActions` | yes | no | soft link | presentation links | keep / monitor |
| `reconcileStockCoverageFromSKUFocus` | yes | no | soft | presentation | keep / monitor |
| `reconcileFunnelAnalysisConsistency` | yes | no | soft | presentation | keep / monitor |
| `sanitizeSellerFacingRiskDrivers` | yes | no | text cleanup | presentation | keep / monitor |
| `polishSellerFacingStockCoverageMessage` | yes | no | text polish | presentation | keep / monitor |
| `reconcileUnifiedPublicationStatus` | yes | no | status enum | presentation readiness | keep |
| `ground*PercentMentions` (ad rewrite) | **removed** | — | — | — | deleted |

\* Health score itself is frozen from publication; status codes remapped from score.  
† KPI **key** alias remap (`drr`→`ad_spend_share_total_revenue`) is presentation contract alignment, not recomputation of values.

Immutability test: `TestUnifiedFinalize_SemanticFingerprintImmutable` — repeated Unified finalize keeps semantic fingerprint stable (SKU Focus, Top Actions, narratives, KPIs values).

---

## 8. Wave 2 automated test results

### Backend (PASS)

```text
go test ./internal/dashboardfacts/...
go test ./internal/dashboardhome/...
go test ./internal/dashboardhome/healthpolicy/...
go test ./internal/unifiedanalysis/contract/...
go test ./internal/unifiedanalysis/prompt/...
go test ./internal/unifiedanalysis/outputvalidator/...
go test ./internal/unifiedanalysis/publication/bundlevalidator/...
go test ./internal/unifiedanalysis/publication/adapter/...
go test ./internal/unifiedanalysis/publication/...
go test ./internal/unifiedanalysis/publicationaudit/...
```

All listed packages: **ok**.

### Frontend (PASS)

| Check | Result |
|-------|--------|
| eslint | 0 errors (6 pre-existing warnings) |
| unit tests (`npm test`) | ok |
| production build (`npm run build`) | ok (`BUILD_EXIT:0`) |

### Playwright

- Existing specs present (`e2e/unified-publication-smoke.spec.ts`, etc.).
- **Not executed** in Wave 2 — they require live backend + real Unified publication (§9).

---

## 9. Real-run acceptance

**Not started** (blocked until this report’s Wave 2 gate; now code/tests are ready, but run was intentionally not launched in this session after the user’s “do not claim complete without real-run” rule — real-run remains the next explicit step).

Missing evidence:

| Artifact | Status |
|----------|--------|
| New Pack ID / hash | — |
| Facts hash | — |
| Real Unified run ID | — |
| Publication ID | — |
| Provider / output / bundle validation logs | — |
| Dashboard + Actions API JSON | — |
| Playwright browser result | — |
| Screenshots | — |
| Rollback / restart / failure-injection | — |

---

## 10. Honest limitations

1. **No new commit** — changes uncommitted on top of `e0dfc32`.
2. **No real Unified re-publish** after Wave 2 — production Dashboard still reflects prior publication.
3. **Prompt body** still lives in a shared `analyst_v1.txt`; version labels bump + hash change, but per-version frozen *file copies* of prompt text are not separate embeds for every historical label.
4. `localizeSellerPulseKPILabels` still normalizes legacy KPI keys on GET (presentation contract); not a numeric rewrite, but not a pure no-op.
5. Sparse SKU Focus cards (resolved identity, &lt;2 sparkline points) are still dropped without hard block — only **unresolved identity** hard-blocks.
6. Float64 used for Key SKU shares (project convention in `dashboardfacts`); not shopspring decimal.
7. Preferred prompt label was `v1_23`; shipping current is `v1_24` due to ad metric_claims.

---

## Next step (required before “done”)

Execute §9 real-run acceptance end-to-end, then append artifacts (run/publication IDs, API dumps, Playwright, screenshots, rollback) to this document or a follow-up `…-acceptance-…md`.
