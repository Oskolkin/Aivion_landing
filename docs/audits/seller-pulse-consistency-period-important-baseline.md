# Seller Pulse consistency / period / important-facts — baseline

Captured **before** code changes (except a temporary `inspect-unified-bundle --dump-dir` dump helper).

Date of capture: 2026-07-21.

## Current publication identity

| Field | Value |
| --- | --- |
| publication_id | **59** |
| run_id | **89** |
| seller_account_id | 1 |
| metric_date | 2026-07-08 |
| status | published (current) |
| prompt_version | `unified_analyst_v1_18` |
| schema_version | `aivion_unified_analysis_v6` |
| pack_id | `pack:1:2026-07-08:cf420c1b5819ba3a` |
| pack_hash | `cf420c1b5819ba3a70f7987a54eb928cac98dbe3471a4643972af77d1069bbcd` |
| facts_hash | `90eefb36b398404c8481378b9db83b408a5a8973ca82ea71e7356287c1ef9fc5` |
| output_hash | `b9c4e0dadfb0791726324e755c9bba494a7b6755d508e0938b7410c3036c021f` |
| bundle_hash | `68ebdbf199417080de9e19d537a856b46fe91422eda53f74263015cb1152aecd` |
| provider_response_id | `resp_064868f4b6ac5965006a5e6c956f8c819cab782be2e427b382` |
| provider_container_id | `cntr_6a5e6c942bf08191969647aa6257bc8f06fa6f905983e56e` |
| code_interpreter_calls | 1 |
| tokens in/out | 13016 / 9515 |
| started_at → completed_at | 2026-07-20T18:44:38Z → 2026-07-20T18:46:57Z |

Artifacts dumped to `backend/debug/_baseline_pub59/`:

- `dashboard_facts.json`
- `frozen_dashboard.json`
- `narrative_payload.json`

AI output extract: `backend/debug/_baseline_run89_pulse_extract.txt`.

## Period roles (as designed)

| Layer | Current | Previous / comparison | Chart |
| --- | --- | --- | --- |
| KPI cards | day 2026-07-08 | day 2026-07-07 | 2026-07-02 … 2026-07-08 |
| AI weekly analysis | 2026-07-02 … 2026-07-08 | 2026-06-25 … 2026-07-01 | n/a |

Backend `period_context` captions (pale FE rendering):

```text
Данные на 8 июля · изменение к 7 июля · графики за последние 7 дней
Недельный анализ: 2–8 июля · сравнение: 25 июня–1 июля
```

## Reconciliation table (baseline)

| Метрика | AI-резюме | KPI-карточка | График (8 июля) | Период AI vs KPI | Fact ID | Результат |
| --- | ---: | ---: | ---: | --- | --- | --- |
| Выручка | 437 100 ₽ (+4% vs 419 900) «за 7 дней» | 120 791 ₽ (−3% к вчера) | 120 791.25 | week vs day | AI: `fact1` (fake) | **FAIL** — wrong period comparison in UI + AI weekly value ≠ facts (`revenue_last_7d=975 389.79`, WoW **−8%**) |
| Заказы | 216 (+3,8% vs 208) | 83 (−7,8% к вчера) | 83 | week vs day | AI: `fact2` (fake) | **FAIL** — period mix + AI weekly orders ≠ sum of trend days (672) |
| CTR | not stated in narrative | 13.2% (−1.5 п.п.) | 13.22 | day only on cards | — | **N/A in AI** (no grounded CTR claim) |
| ДРР в общей выручке | «низкие ДРР по основным кампаниям» (qualitative) | 19.4% (day) | 19.37 | unclear | — | **WEAK** — no numeric day/week claim grounded |

Facts weekly (dashboard_metrics.account):

- `revenue_last_7d` = **975 389.79**
- `previous_week_revenue` = **1 057 742.52**
- `revenue_week_to_week_delta.pct` = **−0.08 (−8%)**

AI claimed growth; facts show weekly decline. Day cards also decline. Directionally day and week both down, but AI says up with invented magnitudes.

## Root cause — AI summary vs KPI

**Primary:** AI narrative asserts **weekly** aggregates with **incorrect values** (not matching pack/facts `revenue_last_7d` / WoW delta), while KPI cards correctly show **day-level** facts from the same pack/facts/publication. The UI period caption exists but is a low-contrast fused sentence, so users read weekly AI numbers as if they were the same metric as the cards.

Contributing factors:

1. **Period semantics not structured** — signals carry free-text `period`, not `period_type` / start / end / value; validator only checks `fact_ids` non-empty, not catalog existence or value match.
2. **Fake fact IDs** — `fact1`, `fact2`, `fact_stock1`, `fact_ads1` are not facts-artifact IDs.
3. **No cross-period claim validation** — weekly vs daily claims can coexist without labels enforced in schema.
4. **Not** wrong pack/publication identity — pack_hash / facts_hash / publication 59 / run 89 align across facts, frozen, narrative.
5. **Not** frontend number substitution — FE renders API narrative text and facts-backed KPI values as-is.
6. **Not** day/week values required to be equal — they differ by design; baseline failure is (a) unexplained UI, (b) AI weekly numbers wrong vs weekly facts.

## Root cause — «Что важно знать» disappeared

**Primary (frontend):** intentional hide when `important_notes` are non-empty:

```ts
// frontend/components/dashboard/seller-pulse-section.tsx
const keyPointsInResume = view.knowledge.length > 0;
const showKnowledgeCard = !keyPointsInResume;
```

`important_facts` (AI) → adapter `important_notes` → FE `view.knowledge` rendered **inside** AI-резюме as key points, and the sibling card «Что важно знать» is suppressed. Tests assert this behavior.

**Secondary (schema):** there is **no separate** AI field for key points vs important facts — one array serves both roles. Restoring the card requires a distinct field (or split mapping), not a FE template fallback.

Backend still has `important_notes` in frozen/API (4 items on pub 59). Field is not lost in adapter/snapshot.

## Period presentation defect

Captions use `dashboardTypography.moduleDescription` / `text-slate-500` with ` · ` separators — single fused lines, equal visual weight, insufficient contrast. No semantic label/value items.

## DOM / layout (current)

Order today:

1. Пульс продавца
2. Period caption (2 pale lines)
3. AI-резюме (headline + summary + **key points from important_notes**)
4. KPI cards
5. Быстрые действия only (knowledge card hidden)

## Fix direction (locked after baseline)

1. Separate AI `key_points` vs `important_facts`; map to distinct API fields; always render «Что важно знать» from `important_facts` / `important_notes`.
2. Require structured period/value refs; validate against weekly+daily facts; block publish on mismatch.
3. Semantic period context UI from structured `period_context` dates.
4. Prompt: weekly analysis with explicit periods; may mention day only with date; do not invent ranges.

## Final acceptance (2026-07-21, after fixes)

| Field | Value |
| --- | --- |
| publication_id | **68** (current) |
| run_id | **141** |
| prompt_version | `unified_analyst_v1_20` |
| schema_version | `aivion_unified_analysis_v7` |
| pack_hash | `cf420c1b5819ba3a70f7987a54eb928cac98dbe3471a4643972af77d1069bbcd` |
| facts_hash | `90eefb36b398404c8481378b9db83b408a5a8973ca82ea71e7356287c1ef9fc5` |
| output_hash | `997b0c4b12c5dd41354b06fc54ce19d5af2327c323f7a399989313eb9bbf3ffc` |
| bundle_hash | `e0f98db393945e24b30f96fa7bfd76e7f36f43d6c10f127170899cb07230e295` |
| provider_response_id | `resp_09eab7055092291c006a5f512f8ba081928bd04c816943e01a` |
| code_interpreter_calls | 9 |
| tokens in/out/total | 25242 / 19946 / 45188 |
| latency_ms | 170786 |

### Reconciliation (pub 68)

| Метрика | AI значение | AI период | KPI значение | KPI период | Facts | Result |
| --- | ---: | --- | ---: | --- | ---: | --- |
| Выручка | 975 389.79 (−7.8%) | current_7d 2–8 Jul | 120 791 (−3% к вчера) | day 8 Jul | match week+day | **PASS** (periods differ by design) |
| Заказы | 672 (−6.0%) | current_7d | 83 (−7.8% к вчера) | day | week=pack orders_7d | **PASS** |
| CTR | not claimed weekly | — | 13.2% day | day | cards only | **PASS** |
| ДРР | ~17.5% store week | current_7d | 19.4% day | day | week=ad_spend/revenue | **PASS** |

### Root-cause addendum (why pub 65 slipped)

Claim grounding against `account_summary.csv` silently no-oped because `csv.Reader` record-buffer reuse overwrote header names → empty `accountMetrics` map → validator skipped value checks. Fixed by copying header before next Read; also inject Pack grounding lines into prompt input.

