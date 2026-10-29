# PROMPT №14.1 TRACEABILITY

`PROMPT_14_1_STATUS = CLOSED`

`PROMPT_14_STATUS = CLOSED`

Verification gate for Prompt 14. No Chat v2. No new SQL migration.

---

## 1. Baseline (this session)

| Item | Value |
|------|-------|
| branch | `codex/canonical-sales-hardening` |
| SHA | `c5d323737ce1520c236435d3541498a767e6f600` (`c5d3237 lll`) |
| working tree | dirty Prompt 14.1 verification/fixes (not committed) |
| Go | `go1.26.1 windows/amd64` |
| Node | `v24.14.0` / npm `11.9.0` |
| migration head | `000092_action_support_revalidation` |
| NEW_MIGRATION | **NO** |
| gofmt | empty after `gofmt -w` on Prompt 14.1 files |

## 2. Gap audit (before 14.1)

Prompt 14 was `CODE_COMPLETE_VERIFY_PENDING`. Remaining:

1. Action cursor pagination incomplete
2. Real 4-member group 4→0 not proven
3. Full `go test -tags=integration -p 1 ./internal/analysis/...` not run in 14 session
4. N+1/perf not measured
5. Playwright written but not executed
6. Registry mostly `template = action_type` without type-specific formatters
7. P1→P2 / lazy pin / responsive / loading not browser-executed

## 3. Closure matrix

| Gate | Implementation | Verification | Result |
|------|----------------|--------------|--------|
| Action cursor pagination | `actionview/cursor.go` + `List` limit/cursor/`has_more`/`next_cursor`; mismatch → 400 `cursor_mismatch` | `TestPageGroupsThreePagesUniqueStable`; `TestPrompt141ActionCursorThreePages` | PASS |
| 4-member group 4→3→0 | overlay DONE; groups immutable | `TestPrompt141FourMemberGroupLifecycle` | PASS |
| Deep v4 regression | `deep.OutputSchemaV4` | `go test ./internal/analysis/deep` + full analysis integration | PASS |
| Projection v2 regression | `overview_projection_v2` etc. | `go test ./internal/analysis/projection` + full analysis integration | PASS |
| Full analysis integration | unchanged historical suites | `go test -tags=integration -p 1 ./internal/analysis/...` | PASS |
| Dashboard N+1 | Home: current + 4 projections + overlay | SQL=7 with 100 groups / 2500 SKU rows | PASS |
| Stock 500/2000 | `ListPinnedRows` 1 count + 1 page | SQL=2 both scales, 50 rows/page | PASS |
| Advertising N+1 | same pinned-row page | SQL=2, 42 campaign rows in page payload | PASS |
| Actions N+1 | overlay + batch members | SQL=5, page 50 of 100 seeded groups | PASS |
| Playwright Overview | mocked home | `prompt14 overview kpis desktop` | PASS |
| Playwright Funnel | READY/PARTIAL/READY_EMPTY/UNAVAILABLE/FAILED | 5 tests | PASS |
| Playwright Stock | UNKNOWN `—`, KNOWN 0 band, filters | `prompt14 stock unknown vs zero filters lazy` | PASS |
| Playwright Advertising | unavailable `—`, known 0 budget, attribution note | 2 tests | PASS |
| Playwright Actions | group, DONE, DISMISS, pagination, filters | `prompt14 actions group done dismiss pagination filters` | PASS |
| Browser P1→P2 | mocked current switch + refetch | `prompt14 P1 to P2 publication consistency` | PASS |
| Lazy pinning browser | stock before envelope refetch uses P1 id | `prompt14 lazy pinning…` | PASS |
| Mobile / tablet / desktop | 390 / 768 / 1400 overflow ≤2px | `prompt14 mobile tablet desktop overflow` | PASS |
| Loading/error/empty | skeleton, PRE_ANALYSIS, FAILED; actions 500 + empty | 2 tests | PASS |
| Accessibility | labels, tablist, Back, aria-busy; no axe | `prompt14 accessibility labels` + assertions | PASS (basic; axe not installed) |
| Legacy Dashboard usage | `internal/dashboard` import deny-list | `imports_test.go` + static | PASS |
| Legacy Actions usage | seller UI `publication-actions-api` case routes | static on `actions-screen` / API client | PASS |

## 4. Performance (measured)

`TestPrompt141DashboardAndRowQueryShape` (Postgres `:55432`, fake AI):

```text
Dashboard Home
SQL: 7
duration: 16.0093ms
bytes: 10323

Stock 500
SQL: 2
rows: 50
duration: 10.1558ms
bytes: 6656

Stock 2000
SQL: 2
rows: 50
duration: 2.7278ms
bytes: 6730

Advertising
SQL: 2
rows: 42
duration: 3.1768ms
bytes: 3747

Actions
groups seeded: 100
cases seeded: 500
SQL: 5
groups_page: 50
duration: 65.2587ms
bytes: 75526
```

N+1 verdict:

```text
Dashboard N+1 = PASS
Stock N+1 = PASS
Advertising N+1 = PASS
Actions N+1 = PASS
```

Query count does not scale 1:1 with SKU/campaign/member count. No latency SLA.

## 5. 4-member Action evidence

```text
Publication ID = 2005
Group ID = 2d42a740-94fa-444b-96ce-b95ff0220803
members initial = 4
after DONE #1 = 3
after DONE #2 = 2
after DONE #3 = 1
after DONE #4 = 0
Active group visible: 4 YES / 3 YES / 2 YES / 1 YES / 0 NO
Publication hash before = c512df605f71d191a4353791341a327e19aa4421a472c2529a7704f08ac3eeae
Publication hash after  = c512df605f71d191a4353791341a327e19aa4421a472c2529a7704f08ac3eeae
equal = YES
```

History: four DONE cases listed under `status=done`. DISMISS member: `TestPrompt141DismissMemberKeepsGroup` active=3, group rows unchanged.

Opposite-intent Prompt13 suites remain in full analysis integration.

## 6. Pagination evidence

```text
total Action views in fixture walk = 9
page size = 3
number of pages = 3
duplicate IDs = 0
missing IDs = 0 (unique=9)
order stable = YES (display_order then action_view_id)
seller isolation = YES
cursor mismatch rejected = YES
```

Contract: `limit`, `cursor`, `has_more`, `next_cursor`. Bound to seller, status, family, search, sort=`aivion_display_order`.

## 7. Browser Publication evidence

```text
P1 ordered_amount = 120000
P2 ordered_amount = 99000
P1 stock sentinel = p1-sku
P2 stock sentinel = p2-sku
P1 campaign sentinel = Camp-P1
P2 campaign sentinel = Camp-P2

before refetch = all P1
after refetch = all P2
mixed state observed = NO
lazy pin: stock requested with publication_id=101 while current already P2; after dashboard refetch stock uses 202
```

## 8. 19 Action registry

Frontend `CANONICAL_ACTION_TYPES` length 19 = backend `actioncatalog.MustHave19()`. Each entry has family, `presentation_template_key`, icon role, columns, `parameter_formatter`. Formatters: sku_price, sku_replenishment, sku_promotion, campaign_budget, campaign_product_set, category_plan, account_rating, none. Unknown type: `presentationOrFallback` + `console.warn("unknown_action_type")`, no fabricated fields.

## 9. Versions

| Artifact | Version |
|----------|---------|
| Deep output | `deep_output_schema_v4` |
| Overview / Funnel / Stock / Advertising / Actions snapshot | `*_projection_v2` |
| Historical v1/v3 blobs | not rewritten; schema_version on payload |

Finding `primary_domain` / `overview_bucket` and takeaway `primary_domain` remain catalog fields (no Russian heuristic).

## 10. Legacy audit

```text
new Dashboard imports dashboardhome = NO
new Dashboard reads unified publication = NO
frontend Dashboard fallback to legacy = NO (publication-dashboard path)

new Actions reads action_items = NO
frontend Actions calls action_key mutation = NO

old Chat action_key files remain = YES
classification = OUT OF SCOPE — Prompt15
```

Registered legacy `POST /api/v1/actions/{action_key}/check|uncheck|dismiss` remain for old tests/Chat until cutover.

## 11. Tests run

Backend:

- `gofmt -l .` empty (after format of 14.1 files)
- `go vet ./...` PASS
- `go build ./cmd/api` PASS
- `go build ./cmd/worker` PASS
- `go test ./internal/actionview/...` PASS
- `go test -p 1 ./...` PASS except documented Windows EDR `unifiedanalysis/prompt` `prompt.test.exe` Access is denied; `go test -c -o ua_prompt_unit.test.exe ./internal/unifiedanalysis/prompt/` then run binary **PASS**
- `go test -tags=integration -p 1 ./internal/analysis/...` PASS (~472s parent package + subpackages)

Frontend:

- `npm run lint` 0 errors (pre-existing warnings elsewhere; unused `Link` in dashboard-shell removed)
- `npm run test` PASS
- `npx tsc --noEmit` PASS
- `npm run build` PASS

E2E:

```text
command: E2E_SKIP_SEED=1 npx playwright test e2e/prompt14-publication.spec.ts
tests: 17
passed: 17
failed: 0
skipped: 0
duration: 24.6s
```

Isolated migrate seed of `marketplace_ai` failed on `MATERIALITY_REGISTRY_IMMUTABLE` (out of Prompt 14.1 scope). Browser suite used deterministic route mocks + existing demo login. No live Ozon/OpenAI.

Axe not in `package.json`. Not added. Basic a11y assertions only.

## 12. Remaining after CLOSED

```text
Chat v2 = NOT STARTED
Evals / Observability completion = NOT STARTED
Production cutover = NOT STARTED
Legacy backend/database removal = NOT STARTED
Remaining blockers before Prompt 15 = NONE
```

NEXT = Prompt 15 — Chat v2. NOT STARTED.
