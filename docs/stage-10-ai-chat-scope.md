# Stage 10. AI Chat MVP - Scope and Boundaries

## Purpose

Stage 10 adds a safe natural-language interface to seller data:

- user asks a question in `/app/chat`;
- backend plans data access via allowlisted tools;
- backend assembles factual context;
- model generates answer;
- backend validates answer and stores trace.

Core architecture:

```text
planner -> backend tools -> fact context -> answerer
```

## Security Model

- ChatGPT does not access DB directly.
- ChatGPT does not run SQL.
- ChatGPT does not execute write/update/delete actions.
- seller scope is backend-only (auth context).
- only read-only backend tools are allowed.

## In Scope

- chat storage (`chat_sessions`, `chat_messages`, `chat_traces`, `chat_feedback`);
- domain package `backend/internal/chat`;
- tool registry + validation guardrails;
- read-only tools and tool data repository;
- fact context assembler;
- planner + answerer OpenAI client integration;
- answer validation;
- chat API and chat UI;
- trace logging with safety controls.

## Out of Scope

- auto-actions (price changes, ad management, replenishment creation, etc.);
- write tools;
- direct OpenAI from frontend;
- direct DB access by LLM;
- admin/billing UI;
- streaming protocol redesign.

## Supported MVP Themes

- priorities and "what to do today";
- recommendation explanation;
- alerts and risks;
- sales/stock/advertising/pricing analysis;
- ABC analysis.

## Completion Criteria

Stage 10 is complete when:

- `/app/chat` works end-to-end;
- backend stores user + assistant messages and traces;
- tool execution is allowlisted and read-only;
- unsafe/unsupported requests are safely refused;
- no secrets leak to UI or traces.
