# Discovery Runtime (Prompt №10.1 / №11)

Status: **implemented** — Stages 7–9 remain as Prompt 10.1. Stage 10 Evidence Expansion and Stages 11–13 Deep Analysis are implemented. Production frontier: `VALIDATE_DEEP`. Evidence Menu uses `evidencecatalog` + `discovery_evidence_menu_policy_v3`. Cross-thread duplicate Evidence Requests are valid (Stage 10 dedups).

## Ownership

Backend owns factual truth, availability, temporal semantics, entity selection, FactRef grounding, Evidence Menu applicability, and technical validity.

Discovery AI only chooses which observed facts form investigation-worthy situations and which closed Evidence Types to request. It is not a SQL agent, Ozon agent, root-cause engine, Action generator, or prioritization engine.

## Stages 7–10

7. `BUILD_DISCOVERY_PACK` — deterministic pack from sealed universe + accepted Materiality selection + Landscape Detail. Sealed Factual API reads ground only AI-visible projected facts (batched, fail-closed).
8. `DISCOVERY_AI` — one OpenAI Responses call (or at most one exact full rerun). Each provider call is durable-recorded before the next attempt. `AI_PROVIDER_MODE=fake` is forbidden in production/staging; `real`/`openai` fail closed on missing key or HTTP client errors. `DISCOVERY_FAKE_NONEMPTY=1` emits a two-thread overlapping Evidence fixture from the first pack SKU.
9. `VALIDATE_DISCOVERY` — same technical validator as Stage 8 preflight; assigns `T001…` thread IDs. Duplicate Evidence type+subject is forbidden **within** a thread, allowed **across** threads (`discovery_validation_v3`).
10. `EXPAND_EVIDENCE` — see [evidence-expansion.md](evidence-expansion.md). Deep follows; see [deep-analysis.md](deep-analysis.md).

Stage 14 `PLAN_ACTION_RECONCILIATION` stays out of `ProductionImplementedStageNames` and must not become READY.

## Pack

`DiscoveryPackArtifact` audit envelope includes seller/run/universe IDs, contract versions, payload/map/menu hashes, selection semantic hash vs selection artifact SHA (never compared as the same string), and token estimate of the full serialized Responses request.

AI-visible JSON is an allowlisted projection DTO — not raw Landscape blocks. Each business factual object carries a visible `fact_handle` (`f_xxxx`) next to the values. Hidden `FactHandleMap` is a separate artifact.

Structural metadata without handles: entity identity, readiness, limitations, temporal windows (`current` + `previous` when present).

## Fact Handles

Pack-local `f_0001…` handles map to sealed Factual API `FactRef`s and/or a versioned derivation. Two-sided comparisons require both sides usable (`READY` / allowed `PARTIAL` / `STALE`). Empty readiness is not `READY`. `READY+FAILED` / `READY+UNKNOWN` emit no comparison handle. Price last observed change is a single fact, not a fake two-sided comparison.

UNAVAILABLE/FAILED sources get no usable handle. STALE cannot be the sole Thread grounding. Allowed PARTIAL is explicit (Search/Funnel lag-style paths).

The handle map is a hidden immutable artifact (`DISCOVERY_FACT_HANDLE_MAP`). The model never sees the map; it sees `fact_handle` on each projected object.

## Evidence Menu

Closed catalog of 50 semantic keys. Applicability is Global Catalog ∩ **sealed universe `DomainNames()`** ∩ capabilities ∩ selected entity kinds — not Landscape `DomainWindows`. Types the current Factual API cannot honestly expand are `EnabledForDiscoveryV1=false`. `related_entity != null` is INVALID when the type has empty `RelatedKinds`.

## OpenAI contract

`POST /v1/responses`, configured `OPENAI_DISCOVERY_MODEL` (fallback `OPENAI_MODEL`), stable `instructions` first, canonical payload as `input`, `tools: []`, `store: false`, `truncation: disabled`, Structured Outputs `json_schema` + `strict: true`. No `previous_response_id`. One HTTP call per attempt (runtime retries transport; Discovery policy allows max 2 model responses with the identical request fingerprint `discovery_request_contract_v2`).

Token estimate uses the full serialized request (instructions + payload + JSON Schema + wrapper).

Refusal and incomplete are terminal attempt classes, not schema-valid output. Hidden reasoning content is not persisted.

## Validator

Technical only: state/threads consistency, entity index membership, handle lineage, evidence menu membership, budgets, per-thread duplicate Evidence Requests, related-entity allowlist. No semantic repair.

`NO_MEANINGFUL_THREADS` + `threads: []` is valid.

## Artifacts

| Kind | Stage |
|------|--------|
| `DISCOVERY_PACK` | 7 |
| `DISCOVERY_FACT_HANDLE_MAP` | 7 |
| `DISCOVERY_REQUEST_MANIFEST` | 7 |
| `DISCOVERY_OUTPUT` / `DISCOVERY_OUTPUT_ATTEMPT` | 8 |
| `DISCOVERY_VALIDATED_OUTPUT` / `DISCOVERY_VALIDATION_REPORT` | 9 |
| `EVIDENCE_REQUEST_SET` / `EVIDENCE_BUNDLE` / `ANALYSIS_FACT_HANDLE_REGISTRY` / `EVIDENCE_EXPANSION_REPORT` | 10 |

Generic `analysis_*` tables only. Stage 8/9 input hashes bind to the **accepted Stage 7 attempt** (`ArtifactByStageAttemptAndKind`), not run-wide latest-of-kind. Missing supporting artifact lookup is a hard failure.

## Timeouts / lease

`analysis_orchestration_policy_v5` applies registry `TimeoutPolicy`: Stage 8 uses `ai_default`. Claimed RUNNING stages heartbeat `lease_expires_at`. Lease loss cancels the handler and does not FinalizeSuccess as owner.
