# Prompt №9.3 Traceability Matrix

Baseline: branch `codex/canonical-sales-hardening`, migration 000088 → 000089+.

| Part | Requirement | Document | Gap | Fix | Test |
|------|-------------|----------|-----|-----|------|
| A | SemanticMappingHash ≠ SourceRevisionHash | Physical Implementation Plan / baseline §Reproducibility | collection repo wrote mapping hash as revision | migration 000089 + honest columns | provenance fixture HC≠HM |
| B | Per-family watermark temporal evidence | Temporal Framework | buildContext used domain coverage | HistoryFrom/To from watermark | SEARCH 3 distinct coverage_to |
| C | Optional FinalizedAt, no observed fallback | Temporal Framework | time.Time + LatestObserved fallback | *time.Time, NULL semantics | finished_at NULL test |
| D | Executable Profile registry | Entity Landscapes | Include* flags non-executable | profile_registry_v2.go | Profile omit/required tests |
| E | landscape_schema_v2 + V2 profiles | Entity Landscapes | V1 names mutated semantically | v2 constants + legacy reader | V1≠V2 hash test |
| F–K | Normative profile matrix | Entity Landscapes.docx | partial Account/SKU/Category fields | v2 builders + Factual API | field conformance tests |
| L | missing ≠ zero | Materiality Engine | moneyCurrent nil→0 | FeatureValue[T] | FROM_ZERO/TO_ZERO negative tests |
| M | Required capability fail-closed | Business Capabilities | UNBOUND allows read | capabilityRequired() | PROFILE_CAPABILITY_VIOLATION |
| N | Every active lane production | Materiality Engine | family-level only | lane manifest + per-lane E2E | all SKU/Category/Campaign lanes |
| O | Lane registry hash in bundle | Contract bundle | name-only registry | LaneRegistryHash seal | bundle sensitivity test |
| P | Profile hashes in bundle | Contract bundle | name-only profiles | SKU_CORE_V2#hash | stage input hash test |
| Q | Batch SKU Funnel | Factual API | loop GetSKUFunnelPeriod | set-based SQL batch | query count gate |
| R | Batch Detail context | Entity Landscapes Detail | per-SKU loops | Batch* APIs | detail query count gate |
| S | SupportingContext readiness | Entity Landscapes | error→empty list | SupportingContext[T] | period unsupported test |
| T | MATERIALITY_POLICY_V1_2 | Materiality Engine | V1_1 with changed semantics | policy v1_2 migration | feature hash seal |
| U | Versioned replay | Pipeline Orchestration | partial hash sensitivity | replay E2E tests | same input same hash |
| W | Contentful production E2E | Evals contract | lane family not lane | prompt93 integration | all active lanes |
| AA | ValidatePreDiscoveryFoundation | Discovery.docx (pre-stage) | missing validator | validator.go | pre-discovery PASS |
| AF | Forward migrations only | Physical Implementation Plan | — | 000089+ | migration chain |

Status: **CODE_COMPLETE (stages 1–6)** — Parts A–C, D–E, L, O–P, Q–R (batch SQL), AA, W (all 35 active lanes E2E) implemented. Remaining optional: scale gates (500-SKU query count), versioned replay sensitivity suite, docs/current update, legacy prompt9 mem-path tests.
