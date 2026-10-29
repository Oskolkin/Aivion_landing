# Factual Universe

Status: **Prompt №8.1 + №8.2 (entity sets / bindings)**

## What it is

A Factual Universe is an immutable description of the factual reality available to Aivion for one seller, analysis date, knowledge cutoff (`analysis_cutoff_at`), and policy bundle.

It does **not** copy canonical fact rows. Canonical facts stay in UCDM. The Universe freezes **how** they are read: source bindings, capability assessments, DQ summary, entity-set hashes, and policy versions.

## Seal path (Prompt №8.1 / №8.2)

```
RESOLVE_CONTEXT → immutable RESOLVED_CONTEXT_MANIFEST
        ↓
SEAL_UNIVERSE builds entity sets via Factual API under the exact resolved read context
        ↓
canonical Universe manifest → MinIO PutImmutable → Verify
        ↓
PostgreSQL transaction: PREPARING children → SEALED root
```

Entity-set members are semantic only (`ozon_sku`, `provider_campaign_id`, provider category id + ozon_sku). Analysis never queries `canonical_skus` / `advertising_campaigns` / `sku_category_assignments` for Universe content.

## Temporal meaning

- `analysis_date` — business date from AnalysisDateResolver / Core readiness
- `analysis_cutoff_at` — knowledge-time cutoff (AS KNOWN AT); never re-captured with `time.Now()`

Observations arriving after the cutoff must not appear when reading through a sealed Universe.

## Physical model

- `factual_universes` — identity + manifest hash + policy versions + `seal_status` (`PREPARING`→`SEALED`)
- `factual_universe_sources` — per domain/fact_family binding including watermarks/coverage
- `factual_universe_capabilities` — full multidimensional freeze
- `factual_universe_dq_summary` — frozen DQ rollup + issue-set hash
- `factual_universe_entity_sets` — set key / member_count / set_hash (members in MinIO only)
- `manifest_artifact_id` → `analysis_artifacts` (`ON DELETE RESTRICT`, deferrable)

Child INSERT allowed only while `PREPARING`. Child UPDATE/DELETE always rejected after insert. Root flip to `SEALED` is one-way.

## Fact families

Bindings are keyed by `(domain, fact_family)` — e.g. `ADVERTISING.TRAFFIC` and `ADVERTISING.ATTRIBUTION`, or `SEARCH.SELLER_SUMMARY` and `SEARCH.QUERY_DETAILS`, must both survive resolve→seal without collapsing to a single Domain slot.

Collapsing to `*.DEFAULT` is forbidden. Source states are factual (`PARTIAL`/`PENDING_ACCEPTANCE`/…), never forced `SUCCESS`.

Watermarks: the live `domain_watermarks` projection may change; Analysis cutoff reads historical watermark revisions (`domain_watermark_revisions` / AS KNOWN AT) so a later readiness update cannot erase the frozen binding.

## Hashing

SHA-256 over canonical JSON semantic input. Excludes DB PKs, assessment/domain-run IDs, insert order, created_at, artifact id. Includes source/capability/DQ/entity semantic fields and policy versions.

Idempotent seal: unique `(seller, analysis_date, analysis_cutoff_at, manifest_hash)` — retry reuses sealed Universe without mutating children.

## Integrity

`LoadValidated` requires agreement of:

1. root `manifest_hash`
2. DB children reconstructed semantic hash
3. MinIO artifact bytes + content hash

Any mismatch → `UNIVERSE_INTEGRITY_FAILURE` (fail closed).

## Persisted source provenance (Prompt №9.2)

`factual_universe_sources` independently stores all fields needed to rebuild manifest source entries without reading MinIO:

`source_contract_key`, `source_contract_version`, `source_endpoint_family`, `source_contract_hash`, `semantic_mapping_version`, `semantic_mapping_hash`, `source_revision_hash`, `watermark_hash`, coverage windows, latest observed/complete/mature/snapshot timestamps, and **`source_finalized_at`** (collection domain run `finished_at` — not `latest_observed_at` or cutoff).

`LoadValidated` compares three independent views: root manifest hash, DB child semantic hash (rebuilt from PG columns only), and MinIO artifact bytes. Tampering DB (blocked by immutability trigger) or MinIO manifest bytes fails validation.

Legacy rows without honest contract hash remain marked partial; semantic mapping hashes are never reinterpreted as contract hashes.

## SEALED_UNIVERSE

`universe.NewSealedContext` loads only persisted Universe state. Optional provenance (`DomainRunID`/`AssessmentID`) may be nil/0 — never fabricated as `1`. No latest analysis-date, capability, watermark, or DQ re-resolution. Page tokens and FactRefs bind `ContextHash` + `UniverseID`.

## Boundary

`internal/analysis/**` must not import `internal/integrations/ozon` for business facts and must not SQL-read canonical business tables. Facts go through Factual API / resolved readiness state only. Analysis metadata SQL (`analysis_*`, `factual_universe_*`, `materiality_*`, watermark/capability/DQ readiness tables) remains allowed.
