# Current AI-first architecture

## Pipeline

```
Data Pack
  ↓
Global Context
  ↓
Dashboard generation
  ↓
Actions generation
  ↓
Dashboard Top-3 from final Actions
  ↓
Action Detail
  ↓
AI-chat by action
```

Dashboard shows diagnosis. Actions show the operational plan. Both are built from the same Data Pack and Global Context.

## Reviewer (removed)

AI Consistency Reviewer (Stage 4) was removed from MVP. Patch-based review is not used.

Historical note: [removed-ai-consistency-reviewer.md](./removed-ai-consistency-reviewer.md)

## Consistency model

1. Dashboard and Actions are built from one Data Pack.
2. Dashboard and Actions share one Global Context.
3. Dashboard Top-3 is taken from final Actions (`source=actions_final`).
4. Admin diagnostics links `pipeline_run_id`, `data_pack_hash`, `global_context_hash`, `dashboard_run_id`, and `actions_run_id`.

## Dashboard Top-3 API

`GET /api/v1/dashboard/home` exposes:

- `top_actions[]` at the **response root** (primary contract).
- `ai_diagnostics.top_actions[]` as a backward-compatible alias (same items when sourced from Actions).
- `ai_diagnostics.top_actions_source` (e.g. `actions_final`).
- `pipeline.actions_run_id`, `pipeline.pipeline_run_id`.

## Legacy candidate-first global check

Candidate-first Actions generation (`ACTIONS_AI_FIRST=false`) may still run an optional legacy global ordering step:

- Env: `ACTIONS_LEGACY_GLOBAL_CHECK_ENABLED` (default `true` when unset in options).
- This is **not** the removed Stage 4 AI Consistency Reviewer.

## OpenAI networking

1. `AI_HTTPS_PROXY` applies only to OpenAI/AI clients (chat, recommendations, summaries, unified AI pipeline).
2. Ozon API traffic goes direct — not proxied.
3. VPN/VLESS config must not be stored in the repo.

See [ai-proxy-configuration.md](./ai-proxy-configuration.md).

## Stage 5 product flow

```
Dashboard → Top-3 → Action Detail → Ask AI → Export / Admin diagnostics
```

Admin diagnostics `dashboard_top_actions.action_ids` matches `dashboard/home.top_actions[].action_id` in the same order.
