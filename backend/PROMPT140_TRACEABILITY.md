# PROMPT №14 TRACEABILITY

`PROMPT_14_STATUS = CLOSED`

Closed by Prompt 14.1 verification. Evidence: `backend/PROMPT141_TRACEABILITY.md`.

Dashboard replacement read path and Actions replacement read/write path are implemented on Current Publication + projections + Action Case overlay. Mandatory Prompt 14.1 verification (integration, N+1, Playwright) is recorded in `PROMPT141_TRACEABILITY.md`.

Remaining after Prompt14 (explicit):

- Chat v2 = NOT STARTED
- Evals/Observability completion = NOT STARTED
- Production canonical collection/analysis cutover = NOT STARTED
- Legacy backend/database removal = NOT STARTED

Next step: Prompt 15 — Chat v2. Not started.

---

## 1. Baseline

| Item | Value |
|------|-------|
| branch | `codex/canonical-sales-hardening` |
| SHA | `7ba24fd1d8f061930a462ff7214237a30f9548af` (`7ba24fd lll`) |
| working tree | dirty Prompt14 implementation (no commit requested) |
| Go | `go1.26.1 windows/amd64` |
| Node | `v24.14.0` / npm `11.9.0` |
| migration head | `000092_action_support_revalidation` |
| NEW_MIGRATION | **NO** |

## 2. Current-path audit

| Path | Source |
|------|--------|
| legacy Dashboard | `internal/dashboardhome` still in repo; **not** wired to `GET /api/v1/dashboard/home` |
| legacy Actions | `internal/actions` + `/{action_key}/check` still registered for old tests; seller UI uses `actionview` |
| new Publication path | `seller_current_publications` → `analysis_publications` → `publication_projections` + pinned row tables + `publication_action_groups` + `action_cases` overlay |

## 3. Carry-over projection semantics

Stock/SKU Focus (`projection/rows.go`, `missing_zero_test.go`):

- KNOWN 0 → `sellable_units=0`, state KNOWN, band `out_of_stock`
- KNOWN 40 → 40, KNOWN
- UNKNOWN + KnownSubtotal 40 → `sellable_units` omitted, reason retained, `known_subtotal_units=40`, `known_subtotal_is_authoritative=false`
- UNKNOWN + subtotal 0 → not authoritative zero

Advertising campaign rows: unknown budget/spend/DRR omit fabricated numbers; known 0 budget retained.

Unit tests: `go test ./internal/analysis/projection/...` **PASS**.

## 4. Projection contracts / versions

| Projection | old | new | why |
|------------|-----|-----|-----|
| Overview | `overview_projection_v1` | `overview_projection_v2` | 3 KPI ordered_amount/units/ad_share, takeaways/findings metadata, periods |
| Funnel | `funnel_projection_v1` | `funnel_projection_v2` | state machine, compatible chains only |
| Stock | `stock_projection_v1` | `stock_projection_v2` | Missing≠Zero, coverage bands |
| Advertising | `advertising_projection_v1` | `advertising_projection_v2` | `account_ad_spend_share`, campaign attribution states |
| ActionsSnapshot | `actions_snapshot_projection_v1` | `actions_snapshot_projection_v2` | group presentation fields |
| Manifest | v1 | `publication_projection_manifest_v2` | policy v2 |
| Large rows | v1 | `stock_sku_rows_v2` / `ad_campaign_rows_v2` / `sku_focus_rows_v2` | state fields |

Persisted `schema_version` is taken from payload JSON at publish (`projectionSchemaVersion`). Historical v1 blobs are not rewritten.

Determinism: projection `Build` uses sealed universe + Factual API at Stage16 only; HTTP dashboard does not call Factual.

## 5. Presentation metadata (Deep)

Deep output schema **v3 → v4** (`deep_output_schema_v4`, prompt policy v4).

Added (no text heuristics):

- Finding: `primary_domain`, `overview_bucket`
- Takeaway: `primary_domain`

Canonical domains and buckets in `deep/presentation.go`. Validator + fake + pack builder updated.

## 6. Dashboard backend

`GET /api/v1/dashboard/home` → `dashboard.Service.Home`:

- no current publication → `PRE_ANALYSIS` / `NO_CURRENT_PUBLICATION` (no fake pub, no unified fallback)
- corrupt projections → `FAILED` / `PROJECTION_CORRUPT`
- READY: one publication id for overview/funnel/stock/advertising + focus overlay

Static import gate: `internal/dashboard` must not import dashboardhome, unifiedanalysis, ozon, openai, factualapi — `TestDashboardPackageHasNoForbiddenImports` PASS.

Pre-analysis **may** later use connection/readiness; current implementation only reads `seller_current_publications`.

## 7. Lazy Dashboard APIs

- `GET /api/v1/dashboard/stock/skus?publication_id=`
- `GET /api/v1/dashboard/advertising/campaigns?publication_id=`

Pinning: listed rows for requested publication id even if current is newer.

Seller isolation: unknown publication for seller → `pgx.ErrNoRows` → 404.

Pagination: id cursor, `ORDER BY id`, limit+has_more.

Integration: `TestPrompt14LazyRowsStayPinnedToRequestedPublication`, `TestPrompt14CrossSellerPublicationDenied` **PASS** (Postgres `TEST_DATABASE_URL` :55432).

## 8–11. Overview / Funnel / Stock / Advertising (frontend)

Overview: 3 KPI cards (`md:grid-cols-3`), labels Заказано на сумму / Заказано товаров / Доля рекламы; AOV not in new DTO; Focus max 5 from overlay; What Changed tabs from `overview_bucket`; SKU-фокус tabs Заказы/Ключевые using `focus_tabs` from findings entity+domain when present, otherwise unfiltered preview.

Funnel: visualization + insight + related actions; states READY/PARTIAL/READY_EMPTY/UNAVAILABLE/FAILED; no fake BROAD chain (`funnel_chain_test.go`).

Stock: bands out_of_stock / 0–3 / 4–7 / 8–14; UNKNOWN → `—`; known 0 → Нет остатка.

Advertising: `account_ad_spend_share` as ДРР кабинета with copy distinguishing campaign attribution.

## 12–13. Actions backend / mutations

Package `internal/actionview`. Opaque `action_view_id` = base64url `v1.{g|c}.{uuid}`.

Active: publication groups with OPEN+ACTIVE overlay **plus** ungrouped current-publication cases as 1-member views (grouping policy only persists groups with ≥2 members).

History: DONE vs IRRELEVANT (dismissed / non-active system).

Mutations: `ApplyUserEvents` TX, advisory lock; DONE no 30-day suppress; DISMISSED sets signature `suppressed_until`; REOPENED clears suppression; bulk all-or-nothing; idempotent if already in target user state.

Integration overlay DONE/DISMISS/REOPEN/bulk **PASS** on first publication fixture (`TestPrompt14ActionOverlayDoneDoesNotChangePublicationHash`, `TestPrompt14DismissReopenBulk`). Typical fake Deep emits **1** visible case (not 4); persisted 4-member groups require 2+ same-type intents.

## 14. 19 Action presentations

| action_type | template key | family |
|-------------|--------------|--------|
| ACCOUNT_RATING_REMEDIATION | ACCOUNT_RATING_REMEDIATION | ACCOUNT_HEALTH |
| CATEGORY_REPRICING_PLAN | CATEGORY_REPRICING_PLAN | PRICING |
| CATEGORY_REPLENISHMENT_PLAN | CATEGORY_REPLENISHMENT_PLAN | INVENTORY |
| CATEGORY_ADVERTISING_REALLOCATION | CATEGORY_ADVERTISING_REALLOCATION | ADVERTISING |
| CATEGORY_PROMOTION_PLAN | CATEGORY_PROMOTION_PLAN | PROMOTION |
| CATEGORY_ASSORTMENT_RATIONALIZATION | CATEGORY_ASSORTMENT_RATIONALIZATION | ASSORTMENT |
| SKU_ADJUST_PRICE | SKU_ADJUST_PRICE | PRICING |
| SKU_INCREASE_REPLENISHMENT | SKU_INCREASE_REPLENISHMENT | INVENTORY |
| SKU_REDUCE_REPLENISHMENT | SKU_REDUCE_REPLENISHMENT | INVENTORY |
| SKU_ADJUST_AD_BID | SKU_ADJUST_AD_BID | ADVERTISING |
| SKU_CHANGE_AD_PARTICIPATION | SKU_CHANGE_AD_PARTICIPATION | ADVERTISING |
| SKU_JOIN_PROMOTION | SKU_JOIN_PROMOTION | PROMOTION |
| SKU_LEAVE_PROMOTION | SKU_LEAVE_PROMOTION | PROMOTION |
| SKU_ADJUST_PROMOTION_PRICE | SKU_ADJUST_PROMOTION_PRICE | PROMOTION |
| SKU_CHANGE_CATALOG_STATE | SKU_CHANGE_CATALOG_STATE | ASSORTMENT |
| CAMPAIGN_ADJUST_BUDGET | CAMPAIGN_ADJUST_BUDGET | ADVERTISING |
| CAMPAIGN_CHANGE_STATE | CAMPAIGN_CHANGE_STATE | ADVERTISING |
| CAMPAIGN_ADJUST_STRATEGY | CAMPAIGN_ADJUST_STRATEGY | ADVERTISING |
| CAMPAIGN_ADJUST_PRODUCT_SET | CAMPAIGN_ADJUST_PRODUCT_SET | ADVERTISING |

Frontend gate: `prompt14-dashboard.test.ts` (19 keys). Backend: `TestCanonicalActionTypesAreNineteen`.

## 15–17. Frontend

Shared DTO: `lib/publication-dashboard.ts`. Provider loads `/dashboard/home` only. Formatter `metric-format.ts` (no formulas). Chat from Actions: `/app/chat` without action context.

Legacy files (`seller-pulse-view.ts`, `dashboard-home-types.ts`, `actions-api.ts`) remain in repo for Chat and unused modules; **seller Dashboard/Actions screens do not call them for metrics**. Dual *runtime* fallback (`if publication else legacy dashboardhome`) is not in `DashboardDataProvider`.

Loading: dashboard skeleton; Actions list loading; mutations local `mutating` flag.

## 18. Publication consistency

Home binds all domains to current publication id. Lazy rows require `publication_id`. P1→P2 refetch replaces envelope atomically (single home object). Integration P1/P2 current = P2 **PASS**.

## 19. Performance

Not instrumented this session (SQL counts / 500–2000 SKU / N+1). Overlay uses batch `ListActionGroupMembers` + one ungrouped query. **NOT MEASURED**.

## 20. Versions

Deep output v3→v4. Projection keys v1→v2 as table above. Chat context projection remains `chat_context_projection_v1` (Prompt15).

## 21. Migration

**NO.** Existing Publication/Action tables used.

## 22. Backend tests

| Command | Result |
|---------|--------|
| `gofmt` on touched packages | applied |
| `go vet ./...` | PASS (with focused package run) |
| `go build ./cmd/api ./cmd/worker` | PASS |
| `go test ./internal/analysis/projection/...` | PASS |
| `go test ./internal/dashboard/...` | PASS |
| `go test ./internal/actionview/...` | PASS |
| `go test ./internal/analysis/deep/...` | PASS |
| `go test ./internal/httpserver/handlers/` | PASS |
| `go test -tags=integration -run TestPrompt14 ./internal/analysis/` | PASS |
| `go test -p 1 ./...` | PASS* (only `unifiedanalysis/prompt` `prompt.test.exe` Access denied; renamed `ua_prompt_unit.test.exe` PASS) |
| `go test -tags=integration -p 1 ./internal/analysis/...` (full package) | **NOT RUN** this session (subset Prompt14 only) |

## 23. Frontend tests

| Command | Result |
|---------|--------|
| `npm run lint` | PASS (warnings only, 0 errors) |
| `npm run test` (package.json Prompt14-focused) | PASS |
| `npx tsc --noEmit` | PASS |
| `npm run build` | PASS |

## 24. E2E

`frontend/e2e/prompt14-publication.spec.ts` exists (route-mocked Overview/Funnel/Stock/Advertising/Actions, mobile KPI). **NOT RUN** this session (`npx playwright test e2e/prompt14-publication.spec.ts`).

Publication switch P1→P2 in browser, DONE/DISMISS live, overflow audit: **NOT RUN**.

## 25. Static architecture

New dashboard/actionview packages: no Factual/Ozon/OpenAI imports. HTTP serving stored projections. Seller isolation tests PASS for home/rows/actions PRE_ANALYSIS for other seller.

Forbidden seller-facing **runtime** path: dashboardhome/unified/action_items/live Factual on GET = **NO** for new handlers.

Frontend leftover strings (`average_order_value`, `analysis_source`) still exist in Chat/legacy libs and old e2e specs — not used by migrated Dashboard pages. Documented as leftover, not as dual Dashboard engine.

## 26. Remaining blockers

1. Full `go test -p 1 ./...` completion evidence
2. Full analysis integration suite
3. Playwright Prompt14 (and likely broken pre-Prompt14 e2e that still assert `analysis_source`)
4. Performance measurements
5. Persisted 4-member Action Group fixture (policy groups only at ≥2 identical intents; fake Deep usually 1 case)
6. Legacy frontend file deletion incomplete (Chat still `action_key`)
7. Action list pagination/cursor not fully applied after filter (limit honored loosely)

## 27. Final Prompt14 state

Dashboard read migration = **implemented, verify pending**  
Actions read/write migration = **implemented, verify pending**  
Frontend Overview/Funnel/Stock/Advertising/Actions = **migrated in code**  
19 presentation types = **covered in registry**

## 28. Next step

Prompt 15 — Chat v2. **NOT STARTED.**
