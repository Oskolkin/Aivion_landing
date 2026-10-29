# Unified Analysis Publication (Stage 5)

## Status

**Stage 5 status: ACCEPTED**

Stage 5 publishes a validated Unified Analysis output as one atomic
Dashboard + Actions bundle and switches synchronized current publication
pointers. Full post-sync → provider → validation → Asynq publish → API →
frontend evidence is complete. Mass rollout remains off; default seller mode
remains `legacy`.

**Final acceptance formulation:** Этап 5 окончательно принят. Полный
post-sync Unified flow проверен end-to-end, реальные material, healthy и
insufficient publications отображаются существующим frontend без mock,
Worker восстанавливает публикации после реального process restart,
duplicate и failed sync обрабатываются идемпотентно, tenant isolation
подтверждена HTTP-тестами, а live OpenAI pilot проходит от обновления
данных до атомарно опубликованного Dashboard и Actions.

## Live OpenAI pilot (2026-07-17)

| Field | Value |
| --- | --- |
| seller_id | 1 |
| metric_date | 2026-07-13 |
| run_id | 381 |
| pack_id | `pack:1:2026-07-13:701da97cdfe19087` |
| pack_hash | `701da97cdfe19087f7d0cd18fdbe324e05507af1c1e43d9c63bc9fba97ad95a3` |
| prompt_version | `unified_analyst_v1_5` |
| prompt_hash | `58bdc53ab1dd0a5ddd423c4cb9d44d92b31589008be03a3f1b1940c54df42ad3` |
| schema_version | `aivion_unified_analysis_v1` |
| schema_hash | `sha256:ae19282951d65ba1e3fe09fd7e808f5610f618a61bf168be630d64673a26424d` |
| response_id | `resp_0510cd31f936a33f006a59f989c15481a18d0ba0aa03f6a653` |
| container_id | `cntr_6a59f987a9688191a9a2ff6c5c8514570dab2ae9c1bc871e` |
| Code Interpreter calls | 11 |
| input/output tokens | 22007 / 10531 |
| provider latency | ~186s |
| validation | valid → `publication_ready` |
| analysis_outcome | `material_actions_found` |
| actions | 2 |
| output_hash | `6d89482cd4f8879b4d9564ce0accad06117b8728365f22e61feefca290b5777f` |
| publication_id | 261 |
| adapter | `unified_publication_adapter_v1` |
| bundle_hash | `5d7f3540cc2e2ae2a866be7deb3ae956737a7c0ad625ea3750be613f6e28afcb` |
| visibility | visible |
| publish path | Asynq `unified_analysis.publish` (not manual `PublishRun`) |
| pointers | Unified + compatibility both → run 381 |
| browser | Dashboard + 2 Actions; no console/network errors |
| cleanup | completed |

Stage 4 boundary on the same day: runs **380** and **381** reached
`publication_ready` with pinned prompt/schema before Stage 5 publish.
Stage 3 smoke regression: run **389** ended at `provider_completed` with
cleanup completed (no publication).

## Current pointers

| Pointer | Role |
| --- | --- |
| `seller_current_unified_publications` | **Authoritative** Unified current publication |
| `seller_current_ai_snapshots` | **Compatibility** projection for existing Dashboard/Actions read path |

Invariant for a visible Unified publication:

```text
seller_current_unified_publications.publication_id
  → unified_analysis_publications.run_id
  → seller_current_ai_snapshots.pipeline_run_id
```

Both pointers are updated in the **same PostgreSQL transaction** under a
seller-level advisory lock, after Dashboard + Actions writes, before commit.

`ReconcileCurrentPublicationPointers` treats the Unified pointer as ground
truth and only repairs the compatibility pointer. Worker startup calls
`ReconcileAllCurrentPublicationPointers`. Devtool:
`reconcile-unified-publication --seller-id=N [--dry-run]`.

## Flow

```text
successful sync
→ post-sync dispatcher
→ EffectiveAutoRun
→ Unified run creation
→ unified_analysis.start
→ poll
→ validate_output
→ publication_ready
→ unified_analysis.publish (when EffectiveAutoPublish)
→ published
→ synchronized pointers
→ existing Dashboard/Actions API
→ existing frontend
```

On TX failure at any failure-injection point: full rollback; previous
pointers unchanged; no partial visible bundle; retry succeeds.

## Rollout modes

| Mode | Visible source | Unified run | Unified publication |
| --- | --- | --- | --- |
| legacy | legacy snapshot | off | off |
| shadow | legacy snapshot | optional auto-run | stored, visibility=shadow |
| unified | unified publication | optional auto-run | visibility=visible, pointer switch |

Effective flags:

```text
effective_auto_run     = GLOBAL_ROLLOUT && GLOBAL_AUTO_RUN && seller.auto_run && mode∈{shadow,unified}
effective_auto_publish = GLOBAL_ROLLOUT && GLOBAL_AUTO_PUBLISH && seller.auto_publish && mode∈{shadow,unified}
```

Env kill switches (defaults safe):

```text
UNIFIED_ANALYSIS_ROLLOUT_ENABLED=false
UNIFIED_ANALYSIS_AUTO_RUN_ENABLED=false
UNIFIED_ANALYSIS_AUTO_PUBLISH_ENABLED=false
UNIFIED_ANALYSIS_LEGACY_FALLBACK_ENABLED=true
```

Per-seller settings live in `seller_analysis_settings` (missing row = legacy).

## Acceptance evidence (Stage 5 final)

| Area | Evidence |
| --- | --- |
| Real browser material/healthy/insufficient/rollback/shadow/promote | `frontend/e2e/unified-publication-smoke.spec.ts` (no Dashboard/Actions mocks) + `prepare-unified-browser-scenario` |
| Post-sync legacy/shadow/unified + kill switches + failed/partial sync + duplicate completion | `go test -tags=asynq_integration -run TestPostSync ./internal/unifiedanalysis/` |
| Worker OS process restart (publication_ready, committed mismatch, pointer reconcile) | `go test -tags=worker_restart -run TestWorkerRestart ./internal/unifiedanalysis/` |
| HTTP Action tenant isolation (get/by-id/check/uncheck/dismiss/export) | `internal/httpserver/handlers/actions_test.go` |
| Shadow HTTP isolation | no seller publication routes; source selector never resolves shadow to Unified visible bundle |
| Live OpenAI → Asynq publish → browser | run 381 / publication 261 |
| Unified publication Dashboard read (no v9 placeholder) | `acceptStoredUnifiedPublicationNarrative` + `enforceAIOnly` skip; headers `X-Aivion-*` |
| Stage 3 smoke | run 389 `provider_completed` |
| Stage 4 production | runs 380/381 `publication_ready` then Stage 5 publish |
| AI Chat | `e2e/main-flow.spec.ts` PASS |
| Frontend | `npm run lint`, `npx tsc --noEmit`, `npm test`, `npm run build`, Playwright |
| Backend | `go test ./...`, `go vet ./...`, builds, scoped `gofmt -l` empty |

### Display fix (post-ACCEPTED)

Unified publications stamp `ai_dashboard_narratives.prompt_version=unified_publication_adapter_v1`.
The legacy AI-only accept path previously required `dashboard_narrative_v9_ai_only_chunks`,
rejected the narrative, and `enforceAIOnlyGenerationRequired` replaced the Dashboard with
the English placeholder. Fixed by accepting adapter narratives and skipping the v9 gate
for Unified publications. Local API launcher: `scripts/dev/run-api.ps1` (port **18081**).

## Stale / concurrency policy

- Older `metric_date` cannot replace a newer current visible publication
  (`publication_stale_candidate`).
- Same `metric_date`: higher `run_id` wins; older run cannot move the pointer
  backward.
- Same-run concurrent publish → one publication / one pointer / same bundle hash.
- Explicit rollback is the only intentional path to an older publication.
- Seller advisory lock is per-seller (not global).

## Publication policy

Auto-publication accepts only:

```text
prompt_version = unified_analyst_v1_5
prompt_hash    = 58bdc53ab1dd0a5ddd423c4cb9d44d92b31589008be03a3f1b1940c54df42ad3
schema_version = aivion_unified_analysis_v1
schema_hash    = ae19282951d65ba1e3fe09fd7e808f5610f618a61bf168be630d64673a26424d
adapter        = unified_publication_adapter_v1
```

`analysis_failed` → `publication_failed`, pointer unchanged.
If publication is already `published` but run is still `publishing`,
`PublishRun` finalizes the run to `published` (idempotent recovery).

## DB (migration 000056)

- `seller_analysis_settings`
- `unified_analysis_publications` (`UNIQUE(run_id)`)
- `seller_current_unified_publications`
- `unified_publication_action_map` (`UNIQUE(publication_id, analysis_action_id)`)
- `unified_analysis_publication_audit`
- `ai_pipeline_runs.unified_publication_id`
- status constraint + unique active-run index include `publishing` / `published` / `publication_failed`

## Asynq

```text
unified_analysis.publish  TaskID=unified-analysis-publish:<run_id>  Unique=15m  Timeout=10m
```

Enqueued after `publication_ready` when `EffectiveAutoPublish` is true.
Recovery plans `publish` for `publication_ready` / `publishing`.
Worker startup: `PlanRecovery` + enqueue + pointer reconcile.

## Post-sync

`RecalculateAfterSyncHandler.SetUnifiedAutoRunTrigger` (Worker) resolves
effective settings and creates/enqueues a Unified run only when
`EffectiveAutoRun` is true. Trigger errors are logged and do not fail
post-sync recalculation. Failed/partial sync does not reach this path.

## Devtools

```powershell
go run ./cmd/devtools set-unified-analysis-mode --seller-id=1 --mode=shadow --auto-run=true --auto-publish=true
go run ./cmd/devtools publish-unified-analysis --run-id=128 --visibility=shadow --execution=queue --wait=true
go run ./cmd/devtools promote-shadow-publication --publication-id=2
go run ./cmd/devtools inspect-unified-publication --publication-id=2
go run ./cmd/devtools list-unified-publications --seller-id=1
go run ./cmd/devtools rollback-unified-publication --seller-id=1 --target-publication-id=261 --reason=pilot
go run ./cmd/devtools prepare-unified-browser-scenario --seller-id=1 --scenario=material
go run ./cmd/devtools check-unified-publication-readiness --seller-id=1
go run ./cmd/devtools compare-analysis-sources --seller-id=1
go run ./cmd/devtools reconcile-unified-publication --seller-id=1 --dry-run=true
```

## Adapter notes

- Unified account severity `important` maps to dashboard narrative severity `high`
  (`chk_ai_dashboard_narratives_severity`).
- Action local IDs (`action_001`…) map via `unified_publication_action_map`.
- Empty outcomes (`no_material_actions`, `insufficient_data`) create an empty
  actions run (`final_status=empty`); pointer switch hides previous actions.

## Scope boundaries

In: atomic publication, pointer, rollout, shadow/promote/rollback, API via
existing snapshot pointer, controlled post-sync auto-run, recovery reconcile,
devtools, tests, existing frontend contract.

Out: AI Reviewer, second AI request, UI redesign, legacy deletion, mass rollout.
