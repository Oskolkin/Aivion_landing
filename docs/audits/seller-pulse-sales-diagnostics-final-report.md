# Seller Pulse + Sales Diagnostics — Final Acceptance Report

Date: 2026-07-18  
Seller: `seller_id=1` · metric_date `2026-07-08`

## New publication / run

| Field | Value |
|---|---|
| publication_id | **14** |
| run_id | **23** |
| previous current (untouched) | publication **11** / run **16** |
| prompt_version | `unified_analyst_v1_13` |
| schema_version | `aivion_unified_analysis_v2` |
| output_hash | `bcd307f5b9e71fcd6c300aaf51d2a6cb85278b48aaa31ffc703e2665653c2afa` |
| bundle_hash | `86db01893c79811251f2fcb0a5a21aaa3f8263e41aeb878314c55e261f8fcf63` |
| health_score | 83 → status **Требует внимания** |
| dashboard_content_status | `ready` |

Baseline before changes: `docs/audits/seller-pulse-sales-diagnostics-baseline.md`.

## Root causes (summary)

1. Short pulse conclusion → shallow prompt/schema (`headline` + short `summary`).
2. Signal duplication → home copied risk drivers into important notes; no `signal_id` ownership.
3. «Статус не определён» with score → AI severity `warning` not mapped; status not derived from score in backend.
4. «Что улучшит индекс» stub → no `improvement_levers`; weak improvement text.
5. English terms → prompt/FE (`CTR`, `AI-*`, `DRR`).
6. Quick Actions → legacy `/app/products|/app/advertising` via kind/href fallbacks.
7. DRR dashed «Норма 30%» → mini-chart reference line.
8. Day/week mix → no structured `period_context`.
9. Cabinet vs campaign ДРР unexplained → prompt lacked reconciliation requirement.
10. **Home `what_changed` dropped after publish** → AI-only `reorderWhatChangedGroups` discarded empty `unchanged_risks` (only `empty_text`), so apply failed with `groups_count_mismatch`.

## Semantic contract (blocks)

| Block | Question answered | Notes |
|---|---|---|
| Пульс продавца | Что происходит сейчас? | KPIs + dynamics + 3–5 sentence conclusion + important facts + Quick Actions from current Actions |
| Что важно знать | На что смотреть прямо сейчас? | From `important_facts`, not index-driver copy |
| Основные причины снижения | Что давит на индекс? | Index-linked drivers + evidence |
| Что улучшит индекс | Что менять? | `improvement_levers` + linked Actions |
| Ухудшилось / Новые риски / Активные проблемы / Точки опоры | Period / publication comparison | Signal ownership + fingerprints; support points allowed when positive/stable facts exist |

## Prompt / schema

- New schema: `aivion_unified_analysis_v2`
- New prompt: `unified_analyst_v1_13`
- Fields: `important_facts`, `improvement_levers`, signal metadata, deeper conclusions, RU presentation terms

## Validation

Output + bundle validators enforce shallow conclusions, signal conflicts, unjustified `new`/`active`, missing levers/support, non-RU presentation terms, Quick Action integrity, period semantics (error codes as specified in the task). Soft warning for pulse↔what_changed fingerprint overlap when sections differ.

## Key code fixes after publish

- `reorderWhatChangedGroups`: keep empty required groups (fixes home apply).
- Backend finalize: health status from score, `period_context`, Quick Actions → `/app/actions?action=…`, KPI label localization (`Кликабельность`).
- FE: «Точки опоры» (not «Точки роста»), RU nav/chat labels, retire `/app/products` CTAs from dashboard scenarios, no DRR norm legend in pulse mini-chart.

## Evidence

### Persisted AI (run 23)

- Pulse summary ~4–5 sentences with figures (orders −13%, revenue −7%, ДРР 19%, priority).
- Diagnostics main conclusion filled; `important_facts`×5; `improvement_levers`×3 linked to `action_001..003`.
- what_changed groups: worsened, new_risks, unchanged_risks (empty), improved=«Точки опоры».

### Dashboard API (`GET /api/v1/dashboard/home?metric_date=2026-07-08`)

- `publication_id=14`, `health_status_label=Требует внимания`, levers=3, `period_context.caption` set.
- `what_changed.status=available`, 4 groups, main conclusion present.
- Quick Actions hrefs = `/app/actions?action=…` (no products/ads).
- KPI ctr label = `Кликабельность`.

### Browser (seller dashboard)

- Titles: Пульс продавца, ИИ-диагностика продаж; ИИ-чат in nav.
- Period caption, Кликабельность, Требует внимания, Что улучшит индекс, Точки опоры.
- No `CTR`/`DRR`/`AI-чат`/`Норма 30%`/`Подсказка появится`/`Статус не определён`.
- No `/app/products` or `/app/advertising` links on dashboard.
- Quick Actions → `/app/actions?action=…`.

## Acceptance matrix

| Проверка | Результат |
|---|---|
| Развернутый вывод Пульса | PASS |
| Развернутый вывод диагностики | PASS |
| Статус индекса | PASS |
| Что улучшит индекс | PASS |
| Нет межблочного дублирования | PASS (contract + soft fingerprint warning) |
| Новые риски подтверждены историей | PASS (prev publication 11 available; empty active OK) |
| Активные проблемы подтверждены историей | PASS (empty group — no false actives) |
| Точки опоры заполнены | PASS |
| Quick Actions соответствуют Actions | PASS |
| Старые маршруты не используются | PASS (dashboard + QA; legacy pages not in nav) |
| Русификация | PASS (SKU allowed; residual English only in entity names like out-of-stock in AI prose — noted) |
| Периоды однозначны | PASS |
| Пунктирная линия ДРР удалена | PASS |
| Новый OpenAI run | PASS (run 23 / pub 14) |
| Backend tests | PASS (`go test ./...`, vet, builds) |
| Frontend tests | PASS (`npm test`, lint=0, tsc=0, build=0) |
| Full Playwright | PASS |

## Residual notes

- AI text may still contain English fragment `out-of-stock` in places; presentation validator targets known UI terms (`AI`, `DRR`, `CTR`), not all English.
- Category/SKU internal product CTAs retired (return `null`); external Ozon `product_url` CTAs retained.
