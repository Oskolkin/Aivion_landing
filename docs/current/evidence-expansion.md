# Evidence Expansion (Prompt №11)

Status: **implemented** — Stage 10 `EXPAND_EVIDENCE`. Both `SUCCEEDED` and `SUCCEEDED_WITH_LIMITATIONS` unlock Stage 11 Deep Pack. Evidence limitations remain visible in the Deep Pack; they are not a global Deep failure.

## Pipeline

Validated Discovery → exact accepted-attempt bindings → normalize + cross-thread dedup → closed resolver registry → same sealed `FactualReadContext` → bounded Factual API reads → typed Evidence items + Fact Handles → immutable `EVIDENCE_BUNDLE`.

Stage 10 is deterministic. It does not call OpenAI, Ozon, web, latest-state, or recursive Evidence.

## Catalog

Single source of truth: `internal/analysis/evidencecatalog`. Discovery and Evidence both import it. The catalog has no Factual/runtime/discovery dependency.

50 closed types. Enabled types have resolvers (`Menu ↔ Resolver` parity). Disabled types stay out of the Discovery Evidence Menu.

## Normalization

Retrieval key = hash(universe hash + type + type version + subject + related). Thread IDs and investigation questions are stored, not keyed. Same slice requested by two threads yields one item (`ev_00N`) and two relations.

**Continuity (Prompt №13.1):** `CONTINUITY_REQUIRED` requests are generated from previous Action `REQUIRED_SUPPORT.revalidation` specs and unioned with Discovery requests before normalization/dedup. `EvidenceConsumerRef` records whether a consumer is Discovery (`DISCOVERY_REQUESTED`) or continuity (`CONTINUITY_REQUIRED`). `ContinuitySupportEvidenceRelation` links previous support keys to materialized Evidence IDs. Hard gate: every previous required support must have a relation before Deep.

Versions: `evidence_request_normalization_v2`, `evidence_request_set_schema_v2`, `evidence_bundle_schema_v2`.

## Envelope

Items use typed DTOs marshaled to canonical JSON `data`. Statuses: `COMPLETE`, `EMPTY_VALID`, `PARTIAL`, `FAILED`. Ordinary item failure does not abort the stage. Integrity failures (`FACTUAL_UNIVERSE_INTEGRITY_FAILURE`) fail the stage terminally.

Zero Evidence requests (including `NO_MEANINGFUL_THREADS`) produce empty request set + empty bundle and `SUCCEEDED`.

`EMPTY_VALID` is not a limitation. Any `PARTIAL` / `FAILED` / limited readiness → stage `SUCCEEDED_WITH_LIMITATIONS`.

## Temporal profiles

- `FLOW_HISTORY_30D_90D` — Sales / Ads / Finance: observed daily 30d + weekly context; no fabricated missing days in the Evidence DTO
- `OBSERVATION_HISTORY` — Price / Stock: real observed versions/snapshots (`observation_at`, `sampling_kind`)
- `CURRENT_SNAPSHOT` — bound to cutoff
- `EVENT_HISTORY_30D`
- `PROVIDER_PERIOD_HISTORY` — Search / Funnel / Ratings (no fake daily sales series)

## Fact handles

Unified `ANALYSIS_FACT_HANDLE_REGISTRY` contains Discovery `f_*` plus new Evidence `ef_*`. Exact lineage fingerprint reuse keeps `f_*`. Discovery artifacts are not mutated.

## Artifacts (Stage 10 attempt)

- `EVIDENCE_REQUEST_SET` (audit of raw vs unique)
- `EVIDENCE_BUNDLE` (primary)
- `ANALYSIS_FACT_HANDLE_REGISTRY`
- `EVIDENCE_EXPANSION_REPORT`

## Runtime

`analysis_orchestration_policy_v6` (runtime). After Stage 10 success/limitations, Deep (11–13) runs; Evidence limitations stay in the Deep Pack. Frontier after Stage 13 is `VALIDATE_DEEP`.

Fake Discovery non-empty fixture: `DISCOVERY_FAKE_NONEMPTY=1` builds T001/T002 from the first SKU in the pack (overlapping price/sales/search requests).
