# PROMPT 12 TRACEABILITY

PROMPT_12_STATUS = CLOSED

## Baseline

- Branch: `codex/canonical-sales-hardening`
- HEAD SHA: `09ea3e8de1aa0efcbd839412a21ebdb5bbbcb2be`
- Working tree: uncommitted Prompt 12 implementation on top of Prompt 11 (not committed)
- Go: `go 1.25.6` / toolchain `go1.26.1 windows/amd64`
- Latest migration: `000089_prompt93_collection_provenance` (NEW_MIGRATION = NO)

## Audited paths

`internal/analysis/stages`, `runtime`, `contracts`, `artifacts`, `postgres`, `discovery`, `evidence`, `evidencecatalog`, `continuity`, `openaix`, `aitokens`, `canonicaljson`, Action Contract + Deep architecture extracts.

## Versions (before → after)

| Seal | Before | After |
|---|---|---|
| orchestration | analysis_orchestration_policy_v5 | analysis_orchestration_policy_v6 |
| factual_api | factual_api_v6 | unchanged |
| Discovery semantic contracts | v2/v3 as Prompt 10.1/11 | unchanged |
| Evidence expansion | evidence composite v1 | unchanged |
| DeepPack/Prompt/Output/Validation | NOT_IMPLEMENTED | composite seals |
| ActionReconciliation | NOT_IMPLEMENTED | NOT_IMPLEMENTED |
| BundleVersion | contract_bundle_v1 | unchanged |

Deep seals: `deep_pack_schema_v1`, `deep_fact_registry_projection_v1`, `deep_entity_index_v1`, `deep_relationship_claim_policy_v1`, `action_catalog_v1`, `action_menu_policy_v1`, `deep_pack_budget_v1`, `deep_analysis_prompt_v1`, `deep_output_schema_v1`, `deep_output_budget_v1`, `deep_validation_v1`, `deep_model_policy_v1`, `deep_ai_attempt_policy_v1`, `deep_request_contract_v1`.

## Deep Pack

AI-visible: analysis_context, domain_windows, capabilities_and_limitations, account_context, discovery_result, thread_entity_context, evidence_registry, thread_evidence_links, fact_registry, relationship_claim_policy, action_menu, continuity.

Bindings: Discovery pack universe/seller/date/cutoff ↔ Evidence bundle universe/date ↔ run; validated threads must exist in Discovery pack index. Mismatch → `DEEP_INPUT_BINDING_MISMATCH`.

Worst-case fixture (10 threads, 24 evidence items): `deep_pack_bytes=17346`, `estimated_tokens=7420`, Structured Output schema `~2132` tokens. Within `aitokens` / `OPENAI_DEEP_MAX_INPUT_TOKENS` budget (effective input subtracts `max_output_tokens` once + safety margin).

Stage 11 business reads: 0 (artifacts only).

## Relationship claim policy

Set not rank. Discovery `f_*` → DESCRIPTIVE. Time-bearing Evidence types may allow CO_MOVEMENT / TEMPORAL_ASSOCIATION. PLAUSIBLE_CONTRIBUTOR only listed pairs (sales history + price/stock/ads/search/promotion). PROVIDER_ATTRIBUTED: `AD_ATTRIBUTION_DETAIL`, `ACCOUNT_FBS_ERROR_POSTINGS`. CAUSAL_ESTIMATE always rejected.

## Action Catalog

Exactly 19 types; each has family, target kind, version, required context, parameter validator. Run menu omits unsupported capability, non-MAIN, and hard-impossible state. Eligible SKU_ADJUST_PRICE present when current price known.

## Continuity

Production: `CONTINUITY_INITIAL_NO_BASE_V1` → `mode=INITIAL_NO_BASE`, `previous_actions=[]`. Validator requires assessments count == pack previous_actions (synthetic 3==3 unit-tested). Legacy actions not read.

## Model / request / attempts

`OPENAI_DEEP_MODEL` fallback `OPENAI_MODEL` (default `gpt-5.4` if unset). Optional reasoning effort / verbosity. `max_output_tokens` default 32768. tools=[], store=false, truncation=disabled, previous_response_id omitted. Request hash includes model/effort/verbosity/instructions/payload/schema/max_output/tools/store/truncation/previous flags. Max 2 identical calls. Persistence failure blocks attempt 2. Fake forbidden in production/staging.

## Artifacts

Stage11: DEEP_PACK (primary), DEEP_REQUEST_MANIFEST, DEEP_ACTION_MENU, DEEP_RELATIONSHIP_POLICY  
Stage12: DEEP_OUTPUT (primary), DEEP_OUTPUT_ATTEMPT, analysis_ai_attempts  
Stage13: DEEP_VALIDATED_OUTPUT (primary), DEEP_VALIDATION_REPORT  

Exact accepted-attempt binding (not LatestArtifactByKind).

## Runtime

ProductionImplementedStageNames = 1–13. TimeoutPolicy Deep AI = `ai_deep`. Frontier after Stage 13 = VALIDATE_DEEP. Stage 14 ImplementedInCurrentBuild=false, not READY.

## Tests

| Command | Result |
|---|---|
| go test ./internal/analysis/actioncatalog | PASS |
| go test ./internal/analysis/deep | PASS |
| go test ./internal/analysis/contracts | PASS |
| go test ./internal/analysis/stages | PASS |
| go test ./internal/analysis/runtime | PASS |
| go test ./internal/openaix | PASS |
| go test ./internal/analysis/discovery evidence evidencecatalog | PASS |
| go test ./internal/readiness factualapi factual | PASS |
| go test ./internal/analysis/... | PASS (unit; integration files excluded without -tags) |
| go vet (changed packages) | PASS |
| go build ./cmd/api ./cmd/worker | PASS |
| go test -tags=integration -p 1 ./internal/analysis/ -run TestPrompt12 | PASS (Postgres 127.0.0.1:55432, AI_PROVIDER_MODE=fake) |
| go test ./... | NOT RUN in full (scoped packages above PASS) |
| Real OpenAI smoke | NOT RUN (AI_PROVIDER_MODE=fake; no paid key in this session) |

## Integration

Non-empty 1→13 (`DISCOVERY_FAKE_NONEMPTY=1`): DEEP_PACK, analysis_ai_attempts, DEEP_OUTPUT, DEEP_VALIDATED_OUTPUT, thread/finding/recommendation chain, 0 Stage 14, frontier VALIDATE_DEEP. PASS.

NO_MEANINGFUL_THREADS: Deep still runs; thread_resolutions=[], action_proposals=[], account-level summary. PASS.

Evidence SUCCEEDED_WITH_LIMITATIONS: unit pack preserves limitations; Stage 10 both completion classes still unlock Stage 11 via `loadStageBody`.

## Stage14

NOT_IMPLEMENTED. Remaining work is Prompt №13 PLAN_ACTION_RECONCILIATION + Action Cases / Publication. No architecture blocker from generic runtime (no new Deep tables).
