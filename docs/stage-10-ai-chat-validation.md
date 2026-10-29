# Stage 10. AI Chat MVP - Validation Checklist

## Purpose

Checklist for final acceptance of Stage 10 end-to-end behavior.

## Core E2E Flow

Validate:

1. user sends question via `/api/v1/chat/ask`;
2. session/message creation works;
3. planner output is validated;
4. only read-only allowlisted tools are executed;
5. fact context is assembled;
6. answerer output is validated;
7. assistant message is stored;
8. trace is completed or failed correctly.

## Functional Scenarios

- priorities: "what should I do today";
- explain recommendation;
- unsafe ads / ad loss;
- stock risk;
- pricing risk;
- ABC analysis;
- no-data fallback;
- unsupported request refusal.

## Security Scenarios

- invalid planner JSON -> safe fail;
- invalid tool plan (SQL/write/forbidden args) -> rejected before execution;
- partial tool failure -> limitation-aware response;
- invalid answer / auto-action claim -> rejected by validator;
- no direct DB access by model;
- no auto-actions executed.

## Trace and Privacy Checks

- trace saved for success and failure paths;
- payloads sanitized before persistence;
- no secrets in traces (`sk-*`, bearer, tokens, keys);
- no raw trace internals in normal frontend responses.

## API/UI Checks

- `/app/chat` renders and sends requests;
- sessions and messages load;
- feedback works for assistant answer messages only;
- archived session is read-only.

## Required Commands

Backend:

```bash
cd backend
go test ./internal/chat
go test ./internal/httpserver/...
go test ./cmd/api
go test ./...
```

Frontend (optional if unchanged):

```bash
cd frontend
npm run build
npm run lint
```

## Stage 10 Done When

- all mandatory tests pass;
- unsupported and error flows are safe;
- trace sanitization is enforced;
- API and UI behavior match MVP boundaries.
