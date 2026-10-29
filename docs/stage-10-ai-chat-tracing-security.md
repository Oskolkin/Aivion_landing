# Stage 10. AI Chat - Tracing and Security

## Purpose

Define what is stored in chat traces and how sensitive data is protected.

## Trace Scope

Each chat question should produce a trace with:

- lifecycle status (`running|completed|failed`);
- planner/answer prompt versions and models;
- detected intent;
- planner and answer raw responses (sanitized);
- tool plan, validated plan, tool results (sanitized);
- fact context payload (sanitized);
- answer validation payload;
- token usage and estimated cost.

## Secrets and Sensitive Data

Never persist or return:

- API keys (`OPENAI_API_KEY`, `sk-*`);
- authorization headers / bearer tokens;
- cookies/session tokens/passwords;
- raw external payloads with secrets;
- cross-account data.

## Required Sanitization

Before trace persistence, payloads must be recursively sanitized.

Forbidden keys are redacted (e.g. `api_key`, `token`, `authorization`, `secret`, `cookie`, `raw_payload`), and sensitive string patterns are masked:

- `sk-...` -> `[REDACTED_OPENAI_KEY]`;
- `Bearer ...` -> `Bearer [REDACTED]`;
- `OPENAI_API_KEY` markers -> `[REDACTED]`.

## Frontend Contract

Regular chat API responses must not expose trace internals. UI may receive only safe fields (answer, summary, confidence, related IDs, limitations, trace_id reference).

## Missing API Key Behavior

App may start without `OPENAI_API_KEY`, but chat ask requests must fail safely:

- no panic;
- no secret leakage in logs/response;
- safe provider error handling only.
