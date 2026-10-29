# Deep Analysis (Prompt №12 + Prompt №13 / 13.1 continuity retrofit)

Status: **CLOSED for Deep AI**; Prompt №13.1 bumped `deep_output_schema_v3` / `deep_validation_v3` / `deep_analysis_prompt_v3`. Schema name `aivion_deep_analysis_output_v3`. Production frontier is `POST_PUBLISH` after stages 14–19.

Each Action Proposal must include `supports[]` with ≥1 `REQUIRED_SUPPORT` (claim + findings + fact handles + **revalidation spec**). Validated artifact assigns canonical `B001` keys. Previous actions require 100% required-support reassessment (`HOLDS`/`CHANGED`/`NOT_VERIFIABLE`). Backend does not invent required supports from Action Type.

Continuity: first run `previous_actions=[]`; later runs project previous atomic Actions from the base Publication (`CONTINUITY_FROM_PUBLICATION_V1`). `continuity_support_evidence_links` bind previous supports to current Evidence IDs in the Deep Pack.

Deep Pack compacts without dropping factual Evidence: `data` blobs stay typed and observation-bearing; fact handles appear once in `fact_registry` (not duplicated per evidence item); over-budget → `DEEP_PACK_TOO_LARGE` (no semantic truncation). Budget: `deep_pack_budget_v1` (safety margin calibrated for continuity P2 estimator slack).

Deep is the second and final Daily Analysis v1 AI reasoning call. It is **stateless**: no Discovery `previous_response_id`, no tools, no Factual/SQL/Ozon reads during Stage 11–13 AI.

## Pipeline

```
accepted DISCOVERY_PACK + DISCOVERY_VALIDATED_OUTPUT
+ EVIDENCE_BUNDLE + ANALYSIS_FACT_HANDLE_REGISTRY
+ CONTINUITY_CONTEXT + Contract Bundle
        ↓
BUILD_DEEP_PACK   (artifact-only; 0 business reads)
        ↓
DEEP_AI           (one Responses call, at most one exact full rerun)
        ↓
VALIDATE_DEEP     (same pure validator as Stage 12 preflight)
        ↓
IMPLEMENTED_FRONTIER_REACHED { last_stage: VALIDATE_DEEP }
```

## Deep Pack

Purpose-built projection (not Discovery Pack + raw Evidence Bundle). AI-visible payload:

- `analysis_context`, `domain_windows`, `capabilities_and_limitations`
- `account_context` (always present, including `NO_MEANINGFUL_THREADS`)
- `discovery_result`, `thread_entity_context` (MAIN vs SUPPORTING)
- `evidence_registry` + `thread_evidence_links`
- `fact_registry` (compact projection of the unified handle registry)
- `relationship_claim_policy`, `action_menu`, `continuity`

Finding primary entities and Action targets must be MAIN (`ACCOUNT` / leaf `CATEGORY` / `SKU` / `CAMPAIGN` from Discovery Pack). Evidence-only entities are SUPPORTING.

Binding mismatches (`same run/seller/universe/date/cutoff` and typed hashes) fail Stage 11 as `DEEP_INPUT_BINDING_MISMATCH`. Artifact SHA is never compared to a semantic hash.

## Relationship claim policy

`deep_relationship_claim_policy_v1` is a **set**, not a rank. Allowed bases: `DESCRIPTIVE`, `CO_MOVEMENT`, `TEMPORAL_ASSOCIATION`, `PLAUSIBLE_CONTRIBUTOR`, `PROVIDER_ATTRIBUTED`. `CAUSAL_ESTIMATE` is always rejected (`DEEP_RELATIONSHIP_CLAIM_EXCEEDED`). Discovery `f_*` defaults to `DESCRIPTIVE`. Exceeding the ceiling rejects the whole model attempt (no semantic repair).

## Action Catalog / Menu

`internal/analysis/actioncatalog` is a code-only catalog of **exactly 19** Action Types. Stage 11 builds a run-specific Action Menu (catalog ∩ capabilities ∩ factual context ∩ MAIN ∩ hard eligibility). Semantic “should we raise price?” is not decided by the backend.

Prompt 12 **does not** persist Action Cases by itself. Prompt №13 persists Cases only inside Stage 18 Publish.

Continuity: `CONTINUITY_INITIAL_NO_BASE_V1` → empty previous actions; `CONTINUITY_FROM_PUBLICATION_V1` loads previous atomic Actions + REQUIRED_SUPPORT.

## OpenAI contract

Reuse `internal/openaix`. Request: `tools=[]`, `store=false`, `truncation=disabled`, no `previous_response_id`, strict `json_schema` `aivion_deep_analysis_output_v2`. Model from `OPENAI_DEEP_MODEL` with fallback `OPENAI_MODEL`. Optional `OPENAI_DEEP_REASONING_EFFORT` / `OPENAI_DEEP_VERBOSITY`. Timeout policy `ai_deep` (`OPENAI_DEEP_TIMEOUT_SECONDS`). Attempt policy: max 2 identical model responses. Persistence failure after attempt 1 blocks attempt 2.

`DEEP_PACK_TOO_LARGE` forbids running Deep AI (no dropping Evidence/Threads/Facts).
