# PROMPT 110 / Prompt №11 TRACEABILITY

## Status

`PROMPT_11_STATUS = CLOSED`

Non-empty 1→10 integration: `TestPrompt11ExpandEvidenceNonEmptyStages1to10` PASS (`go test -tags=integration -p 1 -run TestPrompt11ExpandEvidenceNonEmptyStages1to10 ./internal/analysis`).

## Baseline

- Branch: `codex/canonical-sales-hardening`
- Working tree: Prompt 10.1 uncommitted work preserved; Prompt 11 added
- Latest migration: `000089_prompt93_collection_provenance`
- `NEW_MIGRATION = NO`

## Catalog audit (50)

Enabled after (40): all catalog keys except the 10 disabled below.

Disabled (canonical/API missing or unsafe aggregation):

| Type | Reason |
|------|--------|
| SKU_PRICING_STRATEGY_CONTEXT | PriceIndex is not a closed strategy evidence recipe |
| CATEGORY_PRICE_CHANGE_CONTEXT | no category price-change expansion API |
| CAMPAIGN_CONFIGURATION_HISTORY | no campaign configuration history Factual API |
| CATEGORY_FINANCE_BREAKDOWN | no truthful category finance expansion recipe |
| SKU_FINANCE_BREAKDOWN | FinanceAccrualFilter has no SKU grain; allocation forbidden |
| CAMPAIGN_FINANCE_CONTEXT | campaign finance is not a distinct canonical expansion API |
| SUPPLY_FINANCE_CONTEXT | no supply-finance expansion API |
| CATEGORY_SUPPLY_CONTEXT | no category inbound expansion API |
| SKU_CATEGORY_HISTORY | no SKU category history Factual API |
| SKU_CATALOG_STATE_HISTORY | no SKU catalog state history Factual API |

Enabled before (Discovery menu policy v2): 34. Enabled after (menu policy v3): 40.

Newly enabled via existing/new Factual convenience: ACCOUNT_SALES_HISTORY, CATEGORY_SALES_HISTORY, ACCOUNT_SALES_CONTRIBUTORS, CATEGORY_SALES_CONTRIBUTORS, ACCOUNT_FUNNEL_HISTORY, plus previously enabled types kept.

## Factual API

New public methods (`factual_api_v5` → `factual_api_v6`):

- `GetAccountDemandSeries`
- `GetCategoryDemandSeries` (historical SKU→category mapping)
- `BatchGetSKUDemandSeries`

## Versions before → after

| Contract | Before | After |
|----------|--------|-------|
| orchestration | v4 | v5 |
| factual API | v5 | v6 |
| evidence catalog | v1 | v1 (SoT moved to `evidencecatalog`) |
| evidence menu policy | v2 | v3 |
| discovery validation | v2 | v3 (cross-thread duplicate requests allowed) |
| evidence expansion | NOT_IMPLEMENTED | composite implemented seal |
| deep | NOT_IMPLEMENTED | NOT_IMPLEMENTED |

## Architecture

- `internal/analysis/evidencecatalog` — semantic catalog only
- `internal/analysis/evidence` — normalize, registry, typed resolvers, bundle, handles; no pgx/ozon/openai
- Stage 10 handler uses accepted Stage 7/9 artifacts and the same sealed Factual context
- Completion: `SUCCEEDED` / `SUCCEEDED_WITH_LIMITATIONS`
- Frontier: `EXPAND_EVIDENCE`
- Stage 11 not READY

## Integration

`TestPrompt11ExpandEvidenceNonEmptyStages1to10` with `DISCOVERY_FAKE_NONEMPTY=1`.

Must be run with Postgres/Redis against the prompt91 seed.
