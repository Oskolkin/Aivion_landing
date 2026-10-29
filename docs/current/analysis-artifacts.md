# Analysis Artifacts

Status: **Prompt №12**

## Store

`ArtifactStore`: `PutImmutable`, `Get`, `Verify`, `Exists`.

Content-addressed keys:

`analysis/seller/<seller>/run/<run>/stage/<stage>/kind/<kind>/sha256/<hash>.json`

Same bytes → same key (safe reuse). Different bytes → different key (no silent overwrite).

## Kinds

- `CONTRACT_BUNDLE`
- `RESOLVED_CONTEXT_MANIFEST`
- `FACTUAL_UNIVERSE_MANIFEST`
- `LANDSCAPE_CORE_SET`
- `LANDSCAPE_DETAIL_SET`
- `CONTINUITY_CONTEXT`
- `MATERIALITY_RANKING`
- `MATERIALITY_SELECTION`
- `DISCOVERY_PACK`
- `DISCOVERY_FACT_HANDLE_MAP`
- `DISCOVERY_REQUEST_MANIFEST`
- `DISCOVERY_OUTPUT`
- `DISCOVERY_OUTPUT_ATTEMPT`
- `DISCOVERY_VALIDATED_OUTPUT`
- `DISCOVERY_VALIDATION_REPORT`
- `EVIDENCE_REQUEST_SET`
- `EVIDENCE_BUNDLE`
- `ANALYSIS_FACT_HANDLE_REGISTRY`
- `EVIDENCE_EXPANSION_REPORT`
- `DEEP_PACK`
- `DEEP_REQUEST_MANIFEST`
- `DEEP_ACTION_MENU`
- `DEEP_RELATIONSHIP_POLICY`
- `DEEP_OUTPUT_ATTEMPT`
- `DEEP_OUTPUT`
- `DEEP_VALIDATED_OUTPUT`
- `DEEP_VALIDATION_REPORT`
- `ACTION_RECONCILIATION_PLAN` / `ACTION_RECONCILIATION_REPORT`
- `PUBLICATION_CANDIDATE` / `PUBLICATION_COMPONENT_MANIFEST`
- `PUBLICATION_PROJECTION_MANIFEST` plus typed `OVERVIEW_PROJECTION`, `FUNNEL_PROJECTION`, `STOCK_PROJECTION`, `ADVERTISING_PROJECTION`, `ACTIONS_SNAPSHOT_PROJECTION`, `CHAT_CONTEXT_PROJECTION`, `STOCK_SKU_ROWS`, `AD_CAMPAIGN_ROWS`, `SKU_FOCUS_ROWS`
- `PUBLICATION_VALIDATED_CANDIDATE` / `PUBLICATION_VALIDATION_REPORT`
- `PUBLISH_RECEIPT` (optional; Stage 18 success is DB commit)
- `POST_PUBLISH_REPORT`
- `TEST_STAGE_OUTPUT` (tests only)

## Artifact-before-state

Build → canonicalize → hash → PutImmutable → Verify → then DB transaction (metadata + attempt + stage success), **except Stage 18 PUBLISH**: Publication rows + pointer + outbox + stage success share one PostgreSQL transaction after artifacts 15–17 already exist. MinIO is not required inside Publish.

Universe seal: MinIO first, then short PG transaction creating PREPARING children and flipping to SEALED atomically. Orphan MinIO objects after failed DB commit are acceptable and immutable.

## Referential integrity

`factual_universes.manifest_artifact_id` references `analysis_artifacts(id)` with `ON DELETE RESTRICT` (deferrable for same-tx insert). Cross-run stage/attempt/artifact acceptance is rejected.

## Metadata

`analysis_artifacts` is append-only. New bytes → new artifact row. Manifests must not contain OAuth tokens, Authorization headers, API secrets, or raw PII.

Primary artifact metadata may use `AddArtifactRecord` with explicit `schema_version` and exact `stage_attempt_id`. `AddArtifact` remains a latest-attempt wrapper for stages 1–6. Discovery supporting artifacts (`DISCOVERY_FACT_HANDLE_MAP`, `DISCOVERY_REQUEST_MANIFEST`) are loaded via `ArtifactByStageAttemptAndKind` bound to the accepted Stage 7 attempt.
