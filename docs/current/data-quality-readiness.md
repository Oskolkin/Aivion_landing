# Data quality and readiness

DQ is system evidence, not a seller problem, score or repair engine. Runs and issues are deterministic and append-only. Dimensions cover schema, completeness, uniqueness, integrity, validity, semantic, resolution, reconciliation, temporal and freshness/maturity. Engineering severity is separate from readiness effect and retry class. DQ never edits canonical facts, guesses categories/dates, calls Ozon or creates Actions.

Catalog checks authoritative availability, identity/population and independent category/product-type resolution. Sales checks daily presence, reconciliation and maturity; it never reconciles Demand against Posting quantity. Lifecycle checks identity/link/dedupe/time-quality evidence and remains optional for the Core analysis date.

`sales_maturity_policy_v1` requires a completed day, usable reconciliation, and identical full normalized-day hashes from the latest two distinct authoritative Sales runs. A correction immediately resets maturity until reconfirmed. PASS and fully explained unresolved-SKU gaps are eligible for the **candidate day**; missing, UNEXPLAINED_GAP and NOT_COMPARABLE still block **that day**. Historical depth is not an Analysis Date gate.

The resolver starts at the newest completed Moscow business day and walks backwards to the first **candidate-day** Core-ready date (usable Canonical SKU universe, Sales fact for D, day-local maturity, allowed reconciliation). Empty Catalog returns `CONNECTED_NO_ANALYZABLE_DATA`; no usable Sales business day returns `NO_SALES_HISTORY`. Shorter history does **not** return a 14-day `INSUFFICIENT_ANALYSIS_HISTORY` wait. Lifecycle failure does not block an otherwise ready Core date.

Under `sales_maturity_policy_v2`, maturity is day-local: a structurally
completed EXPLAINED_GAP may mature from identical hashes in two distinct
accepted runs even when the containing run is PARTIAL. Complete-through ends
at the first missing Account day or blocking reconciliation status. Current
Catalog population is computed from cutoff-selected SKU versions and excludes
`UNOBSERVED` stable identities. DQ also emits deterministic evidence for daily
coverage holes, mixed currencies and missing lifecycle source SKUs.

`dq_policy_v3` adds deterministic optional-domain evidence. Stock checks snapshot completeness, missing-provider-response rows, batch unavailability, unresolved SKUs, ambiguous inventory pools and pending quantity semantics. Price checks unresolved/invalid observations, pending field semantics and cross-SKU mixed-currency aggregation risk. Advertising checks unknown campaign families, unknown target scopes, unresolved SKU identity, pending attribution, mixed currencies and **latest-as-known-at** spend reconciliation (historical DIFFERENT superseded by PASS does not remain an active blocker). Watermarks distinguish observed time, complete snapshot time, complete business date and mature attribution date. Advertising capabilities are source-family aware so optional CPO/attribution gaps do not automatically degrade campaign universe/traffic. These issues affect their own capability assessments and never move the Core Catalog + Sales analysis date.

Prompt №7 adds family-aware evidence for Funnel additive/direct periods, Finance ledger/balance, Search summary/detail/market data, Promotion definition/membership/eligibility, Account Health snapshots/history, and Supply state/bundle/acceptance. Partial optional subfamilies degrade only their closest capabilities; unknown rating keys and supply states are retained as explicit semantic issues. Analysis-date selection remains Catalog + Sales only.

Prompt №7.1: Search readiness counts `search_sku_summaries` only — legacy wrong-grain `search_seller_summaries` never yields READY seller summary. `SEARCH.SELLER_SUMMARY` stays independent of detail/Premium gaps. Supply acceptance beta/partial evidence (`BETA_ACCEPTANCE_UNAVAILABLE`, `SUPPLY_ACCEPTANCE_PARTIAL`) degrades only `SUPPLY.ACCEPTANCE_*`, not core Supply.

### Prompt №9.8 — short history is a limitation, not a wait

Final Physical Implementation Plan §172 supersedes the earlier 14-day Analysis Date wait.

- **Analysis Date** requires candidate-day Core readiness only: usable Canonical SKU universe, Sales Demand fact for D, current maturity policy on D, allowed Account↔SKU reconciliation on D, usable Core source semantics.
- **Historical depth** (`SALES_HISTORY_INSUFFICIENT`, `SALES_MATURITY_INSUFFICIENT`, historical `SALES_DAILY_COVERAGE_HOLE`) remains DQ/capability evidence with `DEGRADE_TO_PARTIAL`. It does not unconditionally `BLOCK_ANALYSIS_DATE`.
- `PARTIAL` + `INSUFFICIENT_HISTORY` is eligible for Analysis Date selection. `PARTIAL` + `SOURCE_SEMANTICS_PENDING_ACCEPTANCE`, `FAILED`, and `UNAVAILABLE` remain blocked.
- Day-local `sales_maturity_policy_v2` is unchanged: D itself must still be mature.
- Incomplete `CURRENT_7D` / `PREVIOUS_7D` / `TREND_30D` stay `PARTIAL` / `UNKNOWN` with `MISSING_DAILY_FACT`. Missing is never coerced to zero.

`dq_policy_v4` records this readiness-effect change. `dq_policy_v3` remains the previous immutable policy identity.

### Analysis resolve DQ (Prompt №8.2)

`RESOLVE_CONTEXT` loads DQ fail-closed: query/pool errors abort resolve. Absence of a completed DQ run at cutoff (`pgx.ErrNoRows`) is the only path that synthesizes an honest `NOT_EVALUATED` snapshot — `DQPolicyVersion` / `FactFamily` markers are `NOT_EVALUATED`, severity stays schema-legal `NONE`, and no DQ run id is invented. That is distinct from a clean evaluated run with severity `NONE`.
