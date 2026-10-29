# Stage 10. AI Chat Planner - Prompt Contract

## Planner Role

Planner is the first model call. It must return a strict JSON tool plan, not a user-facing answer.

Planner decides:

- `intent`;
- `confidence`;
- `language`;
- `tool_calls` from allowlist;
- `assumptions`;
- optional `unsupported_reason` for unsupported requests.

## Strict Output Contract

Planner must return JSON object only (no markdown, no text before/after JSON):

```json
{
  "intent": "priorities",
  "confidence": 0.9,
  "language": "ru",
  "tool_calls": [
    {
      "name": "get_open_recommendations",
      "args": { "limit": 5 }
    }
  ],
  "assumptions": [],
  "unsupported_reason": null
}
```

For unsupported requests:

```json
{
  "intent": "unsupported",
  "confidence": 0.95,
  "language": "ru",
  "tool_calls": [],
  "assumptions": [],
  "unsupported_reason": "Request requires auto-action not allowed in AI Chat MVP."
}
```

## Allowed Intents

`priorities`, `explain_recommendation`, `unsafe_ads`, `ad_loss`, `sales`, `stock`, `advertising`, `pricing`, `alerts`, `recommendations`, `abc_analysis`, `general_overview`, `unknown`, `unsupported`.

## Tool Rules

- tools must come only from backend allowlist;
- tools must be read-only;
- planner must not pass `seller_account_id` or auth-sensitive args;
- planner must not request SQL/raw/write semantics.

## Default Heuristics

- default period: last 30 days when user does not provide one;
- max period: 90 days;
- use minimal data volume needed for response quality.

## Backend Is Final Authority

Planner output is advisory. Backend must validate everything before execution.
