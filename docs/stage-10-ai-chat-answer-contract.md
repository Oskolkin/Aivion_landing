# Stage 10. AI Chat Answerer - Prompt Contract

## Answerer Role

Answerer is the second model call. It generates user-facing answer from backend-provided `FactContext`.

Answerer must not:

- fetch data directly;
- call tools;
- claim executed actions;
- invent IDs/metrics/facts not present in context.

## Strict Output Contract

JSON object only:

```json
{
  "answer": "User-facing explanation",
  "summary": "Short summary",
  "supporting_facts": [
    { "source": "recommendation", "id": 123, "fact": "..." }
  ],
  "related_alert_ids": [101],
  "related_recommendation_ids": [123],
  "confidence_level": "medium",
  "limitations": []
}
```

## Required Semantics

- `answer` and `summary` are non-empty;
- `supporting_facts` are present;
- related IDs must exist in context;
- confidence is one of `low|medium|high`;
- context limitations must be reflected when relevant.

## Unsupported Requests

For `intent=unsupported`, answer should safely refuse automation and suggest manual next steps:

```json
{
  "answer": "I cannot perform this action automatically.",
  "summary": "Request requires an unsupported auto-action.",
  "supporting_facts": [
    { "source": "limitation", "id": null, "fact": "AI chat is read-only and does not execute actions." }
  ],
  "related_alert_ids": [],
  "related_recommendation_ids": [],
  "confidence_level": "high",
  "limitations": ["Auto-actions are out of scope for AI Chat MVP."]
}
```

## Guardrails

Forbidden claims include:

- "I changed price/campaign/budget";
- direct DB/Ozon access claims;
- secrets/raw payload references.

Answerer should provide recommendations for manual action, never claim execution.
