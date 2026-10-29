# Unified Analysis Pack Builder (Stage 2)

Документ описывает реализацию этапа 2: формирование детерминированного ZIP Analysis Pack
из `AnalysisDataSnapshot` без вызова OpenAI и без нового AI runtime.

Связанный контракт: [`unified-analysis-contracts-v1.md`](./unified-analysis-contracts-v1.md).

Stage 3 provider orchestration (OpenAI smoke, без publication): [`unified-analysis-provider.md`](./unified-analysis-provider.md).

## Архитектура

```text
PostgreSQL
    ↓
analysisdata.SQLLoader.LoadWithOptions(HistoryDays=30)
    ↓
AnalysisDataSnapshot (+ LoadedHistory)
    ↓
unifiedanalysis/pack.PackBuilder
    ├── indexes (SKU/campaign universes, by-date maps)
    ├── calculations (delta/CTR/DRR/coverage/…)
    ├── 8 CSV + dictionary/policy/limitations
    ├── content hash → pack_id → manifest
    └── deterministic ZIP (12 entries)
    ↓
Technical Validator
    ↓
локальный файл и/или MinIO (S3_BUCKET_ARTIFACTS)
```

Пакет: `backend/internal/unifiedanalysis/pack/`.

Orchestration: `pack.Service.Build` / `BuildAndStore`.

Legacy путь не меняется:

```text
Load(HistoryDays=14) → aidatapack.BuildPack → SellerAIDataPack
```

Поле `LoadedHistory` игнорируется legacy `BuildPack`.

## Devtool

```powershell
cd backend
go run ./cmd/devtools build-unified-analysis-pack `
  --seller-id=1 `
  --metric-date=2026-07-08 `
  --output=./debug/unified-analysis-pack `
  --upload=false `
  --validate=true
```

Флаги: `--seller-id`, `--metric-date` (обязательные); `--output`, `--upload`, `--validate`,
`--print-manifest`, `--print-limitations`, `--overwrite`.

Артефакты в `--output`:

- `analysis_pack.zip`
- `manifest.json`
- `limitations.json`
- `validation.json`
- `summary.json`

## Периоды (metricDate = D, UTC date-only)

| Period | Range (inclusive) |
| --- | --- |
| current_7d | D-6 … D |
| previous_7d | D-13 … D-7 |
| trend_30d | D-29 … D |

SQL history for Unified Pack: `from = D-29`, `to = D` (30 calendar days).

Legacy Load: 14 days.

## SKU / Campaign universe

**SKU universe** = union of identifiers from products, daily SKU metrics, stock snapshots,
campaign–SKU mappings, enrichment / product-info sources present in the snapshot.
Sorted `sku_id ASC`. Orphan metadata → empty identity fields + limitation `orphan_sku_metadata`.

**Campaign universe** = union of campaigns, daily ad metrics, campaign–SKU mappings.
Sorted `campaign_id ASC`. Orphan campaign metadata → limitation `orphan_campaign_metadata`.

## Source mapping

| Pack field group | Database / snapshot source | Formula / helper |
| --- | --- | --- |
| account_summary sales / deltas | `daily_account_metrics` | sum window; `DeltaAbs` / `DeltaPct` |
| account_summary AOV | account revenue/orders | `AOV` |
| account_summary funnel | `product_analytics_daily` account scope | sum; `ConversionPct` |
| account_summary active_sku_count | SKU 7d revenue/orders | revenue_7d>0 \|\| orders_7d>0 (alerts.sku_importance) |
| account_summary stock counts | stock aggregates + turnover_grade | DEFICIT/SURPLUS/stockout/no-sales-with-stock |
| account_summary ads | `ad_metrics_daily` | sum spend/revenue/orders; `DRRPct` |
| account_daily | `daily_account_metrics` (actual rows only) | passthrough + derived CTR/AOV where applicable |
| sku_summary sales | `daily_sku_metrics` | sum 7d/prev/30d; deltas; `RevenueSharePct` |
| sku_summary stock | `stock_analytics_snapshots` (+ daily stock) | `analytics.AverageDailyVelocity`; `CoverageDays` |
| sku_summary days_without_sales | snapshot field or metric walkback | capped to 30d window |
| sku_summary ads | intentionally empty when only campaign-level ads | limitation `sku_ad_metrics_unavailable` |
| sku_daily | `daily_sku_metrics` actual rows | no SKU×30 cartesian fill |
| campaign_summary / daily | `ad_campaigns` + `ad_metrics_daily` | `CTRPct`, `CPC`, `ConversionPct`, `DRRPct` |
| campaign_sku | `ad_campaign_sku` mappings | IDs only unless SKU-level ad metrics exist |
| product_enrichment | enrichment snapshots / products | no invented scores |
| data_dictionary / policy | frozen contract | deterministic JSON |
| limitations | observed gaps only | sorted by `code ASC` |

## Formulas

- **delta_abs** = current − previous (always numeric)
- **delta_pct** = (current−previous)/previous×100; if previous=0 and current=0 → `0`; if previous=0 and current≠0 → null
- **conversion_pct** = downstream/upstream×100; denominator≤0 → null
- **drr_pct** = ad_spend/ad_revenue×100; revenue≤0 → null (`analytics.CampaignDRRPct`)
- **ctr_pct** = clicks/impressions×100; impressions≤0 → null
- **cpc** = spend/clicks; clicks≤0 → null (`analytics.CampaignCPC`)
- **aov** = revenue/orders; orders≤0 → null
- **revenue_share_pct** = sku_revenue_7d/account_revenue_7d×100; account=0 → null
- **average_daily_sales** = orders_sum/days (`analytics.AverageDailyVelocity`); units semantics match stock analytics (orders velocity)
- **stock_coverage_days** = available_stock / average_daily_sales; avg≤0 → null
- **days_without_sales**: prefer snapshot; else consecutive zero-order days back from D within 30d; 30 if none in window

Money/rates use project `float64` + `round2`/`round4` (same as aidatapack/analytics). No second formula path for DRR/CTR/CPC/velocity.

## Null semantics

| Representation | Meaning |
| --- | --- |
| CSV `0` | known zero |
| CSV empty cell | unknown / unavailable / inapplicable |
| JSON `null` | nullable field unavailable |
| header-only CSV | source empty; `row_count=0`; limitation recorded |

Never write literal `null` / `NaN` / `Inf` into numeric CSV cells.

## Row IDs

Stable, no UUID/run id:

- `account:<seller_account_id>`
- `account_daily:<seller_account_id>:<YYYY-MM-DD>`
- `sku:<sku_id>`
- `sku_daily:<sku_id>:<YYYY-MM-DD>`
- `campaign:<campaign_id>`
- `campaign_daily:<campaign_id>:<YYYY-MM-DD>`
- `campaign_sku:<campaign_id>:<sku_id>`
- `product_enrichment:<sku_id>`

## Pack hash / Pack ID

1. Build 11 files excluding `manifest.json`.
2. SHA-256 each file.
3. `pack_hash` = SHA256(schema_version + ordered name + file SHA256 for those 11 files).
4. `pack_id` = `pack:<seller_id>:<metric_date>:<first_16(pack_hash)>`.
5. Write `manifest.json` with pack_id and table hashes.
6. Build ZIP.

Logical pack identity is **content hash**, not ZIP bytes. `archive_sha256` is separate transport checksum.

## ZIP canonicalization

- Exactly 12 entries, frozen names/order, no directories/symlinks/`..`
- Deflate, fixed entry time `1980-01-01T00:00:00Z`, fixed modes
- UTF-8 CSV, `,` delimiter, LF, no BOM

## Technical validation

Validates archive structure, JSON parse/schema_version, CSV headers/row counts/SHA/row_id uniqueness,
size limits. Does **not** judge business quality (DRR “too high”, stock “dangerous”, etc.).

## MinIO

Uses existing `storage.Client` / artifacts bucket.

Object key:

```text
ai-analysis-packs/seller-<id>/<metric_date>/<pack_hash>/analysis_pack.zip
```

Content-Type: `application/zip`.

Metadata: schema_version, seller_account_id, metric_date, pack_id, pack_hash, archive_sha256,
compressed_bytes, uncompressed_bytes.

No DB artifact row on Stage 2.

## Active SKU definition

`active_sku_count`: SKU with `revenue_7d > 0` **or** `orders_7d > 0` (same importance signal used by alerts/`sku_importance` semantics). Documented in `data_dictionary.json`.

## Daily coverage policy

`account_daily` / `sku_daily` / `campaign_daily` emit **only rows present in source tables**.
Missing calendar days are not zero-filled; incomplete coverage is recorded in `limitations.json`
(`daily_history_incomplete` when applicable).

## Advertising to SKU

Campaign spend is **not** allocated across mapped SKUs. SKU ad metric columns stay empty;
limitation `sku_ad_metrics_unavailable` when campaign data exists without SKU-level metrics.

## Golden fixture

`backend/internal/unifiedanalysis/pack/testdata/`

Update only:

```powershell
$env:UPDATE_UNIFIED_PACK_GOLDEN="1"
go test ./internal/unifiedanalysis/pack -run TestGoldenUnifiedAnalysisPack
```

## Что не входит в этап 2

OpenAI Files/Containers/Code Interpreter/Responses API, Unified Analyst prompt, новый
`ai_pipeline_run`, Asynq, persistence Dashboard/Actions, publication, frontend, отключение legacy.

## Тесты

```powershell
go test ./internal/analysisdata/...
go test ./internal/aidatapack/...
go test ./internal/unifiedanalysis/contract/...
go test ./internal/unifiedanalysis/pack/...

$env:TEST_DATABASE_URL="postgres://postgres:postgres@localhost:55432/marketplace_ai?sslmode=disable"
go test -tags=integration -p 1 ./internal/analysisdata/...
go test -tags=integration -p 1 ./internal/aidatapack/...
go test -tags=integration -p 1 ./internal/unifiedanalysis/pack/...
```
