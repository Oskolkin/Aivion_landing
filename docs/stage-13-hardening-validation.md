# Stage 13 — AI Hardening (MVP validation)

This checklist covers **MVP-level** hardening added for AI recommendations, AI chat, cost estimates, OpenAI outage handling, and optional cleanup jobs.  
**It does not implement Stage 12 billing limits** (token budgets per subscription, hard billing caps, invoicing).

## Preconditions

- Backend and worker built from the same revision.
- `OPENAI_API_KEY` set for environments that should call OpenAI.
- Env vars documented in `.env.example` under **STAGE 13 — AI HARDENING**.

## 1. Recommendation context caps

**Goal:** Large accounts still produce bounded JSON context; truncation is visible in context metadata.

**Checks**

- `AI_RECOMMENDATION_MAX_CONTEXT_ITEMS` and `AI_RECOMMENDATION_MAX_CONTEXT_BYTES` load via config.
- Recommendation `AIRecommendationContext` JSON includes `context_truncated` / `context_truncation_reason` / `context_approx_uncompressed_bytes` when trimming applies.
- No raw secrets added to truncation metadata.

## 2. Chat context caps

**Goal:** Fact context respects byte and per-section item limits from config.

**Checks**

- `AI_CHAT_MAX_CONTEXT_ITEMS` and `AI_CHAT_MAX_CONTEXT_BYTES` wire into `ContextAssembler`.
- `fact_context.context_stats.truncation_reason` set when byte shrink runs (`max_context_bytes_exceeded`).
- Top-level `context_truncated` / `context_truncation_reason` mirror stats for trace payloads.

## 3. Approximate input token pre-check

**Goal:** Absurdly large OpenAI requests fail before HTTP when `AI_MAX_INPUT_TOKENS_APPROX` &gt; 0.

**Checks**

- Lower `AI_MAX_INPUT_TOKENS_APPROX` in a test env and confirm recommendation run fails with `[error_code=context_budget_exceeded]` in `recommendation_runs.error_message` (no partial recommendations).
- Chat `/ask` returns an error without leaking stack traces (expect structured handler response).

## 4. OpenAI outage / transport failures

**Goal:** Provider outages map to controlled failures; chat users see a safe message.

**Checks**

- Recommendations: failed run with `[error_code=openai_unavailable]` when OpenAI returns 5xx/429 or network errors after retries.
- Chat: HTTP **503** with body `AI temporarily unavailable, try again later` when wrapped `ErrAITemporarilyUnavailable`; structured log + Sentry capture in handler path.
- No auto-generated AI recommendations when OpenAI is down (run stays `failed`, no upserts).

## 5. Estimated cost (MVP table)

**Goal:** Centralized USD estimate from token usage for known models; unknown models store **0** cost with `unknown_price` semantics (no DB migration for a separate column).

**Checks**

- Known model (e.g. `gpt-4.1-mini`) produces `estimated_cost` &gt; 0 on successful recommendation runs and chat traces when usage is non-zero.
- Unknown model yields `estimated_cost = 0` (documented as unknown pricing table).

## 6. Cleanup job (optional)

**Goal:** Worker can archive **stale active** chat sessions without deleting business entities.

**Checks**

- With `CLEANUP_ENABLED=true`, worker periodically enqueues `maintenance.cleanup` (interval from `CLEANUP_SCHEDULE`, Go duration).
- Task runs SQL **UPDATE** on `chat_sessions` only (no `DELETE` on recommendations, alerts, products, etc.).
- `CLEANUP_RETENTION_DAYS` must be &gt; 0 when cleanup is enabled.

## 7. Billing scope boundary

**Explicit non-goals for this stage**

- Per-tenant billing enforcement, prepaid token wallets, Stripe metering, and admin “hard stop” limits are **Stage 12 / billing** — not implemented here.

## Automated tests (developer)

```bash
cd backend
go test ./internal/aicost/... ./internal/openaix/... ./internal/recommendations/... ./internal/chat/... ./internal/cleanup/... ./internal/jobs/... ./internal/config/...
```

Expected: PASS.
