# Prompt №9.4 Architecture Traceability Matrix

Baseline: branch `codex/canonical-sales-hardening`, migration through 000089.

| Part | Requirement | Fix | Test |
|------|-------------|-----|------|
| A | Capability fail-closed | `factual_source.go` `capabilityRequired` / no unbound reads | `factual_source_capability_test.go`, E2E |
| B–G | V3 profiles + schema | `profile_registry_v3.go`, `profile_validate_v3.go`, `versions.go` | `profile_replay_test.go`, prompt94 E2E |
| H–L | Source-native windows | `windows.go`, `ResolveFactFamilyPeriodPair` | prompt92 SEARCH lag |
| M | Query count gates | pgx tracer + batch chunking + period DQ cache | `query_counter_test.go` |
| N–P | Materiality V1_3 + lane manifest | `materiality/versions.go`, `active_lane_manifest.go` | prompt93 all lanes |
| Q–R | Batch Detail context | `detail_source.go` batch APIs | detail query gate (4 SQL) |
| AA | PreDiscovery V3 | `prediscovery.go` expanded validator | `TestPrompt94AccountV3DerivedBlocksAndPreDiscovery` |

## Query counts (measured)

| Gate | Result |
|------|--------|
| Core 500 SKU | 46 SQL (ceiling 100) |
| Core 2000 SKU | 76 SQL (ceiling Q500+50) |
| Detail 15/40/15 | 4 SQL (ceiling 40) |

## Machine-readable status

```
CORE_QUERY_COUNT_500 = PASS
CORE_QUERY_SHAPE_2000 = PASS
DETAIL_QUERY_COUNT_MAX_BUDGET = PASS
PRODUCTION_ALL_ACTIVE_LANES_E2E = PASS
PRE_DISCOVERY_FOUNDATION_VALIDATOR = PASS
STAGES_1_TO_6 = PASS
STAGE_7_NOT_EXECUTED = PASS
DOCS_CURRENT_V3 = PASS
REDIS_ASYNQ_INTEGRATION = SKIP (TEST_REDIS_URL)
PROMPT_09_4_STATUS = CODE_COMPLETE
```
