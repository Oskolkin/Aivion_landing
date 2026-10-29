# Chat session lifecycle

## Modes

### New general draft
- User opens `/app/chat` or clicks **Новый чат**
- No `chat_sessions` row is created
- Composer is empty; prompt suggestions are visible
- First manual **Send** creates session + user message and calls OpenAI

### Draft action chat
- First **Спросить AI** when no session exists navigates to `/app/chat?action_id=<id>&mode=draft`
- Legacy query `action_key` remains supported for backward compatibility
- ChatScreen runs a one-shot handoff state machine:
  `resolving_action → creating_or_loading_conversation → attaching_context → sending_initial_message → ready`
- Action context is loaded from authoritative APIs (never from URL payload)
- Exactly one initial user message is sent with a stable `client_request_id` (sessionStorage-backed handoff key)
- On success the URL becomes `/app/chat?session=<id>` and stays there
- Repeat Ask AI / remount / Strict Mode remount reuse the same handoff key → backend idempotency returns the prior result
- If a session already exists for the action, handoff only resumes it (no second initial message)

### Existing session
- Selecting a history item or repeat Ask AI opens `/app/chat?session=<id>`
- Messages load from API; composer starts empty after send

## One action → one session
- **Uniqueness scope (MVP):** `(seller_account_id, action_key)` — shared seller-cabinet chat, not per-user
- Partial unique index: `(seller_account_id, action_key) WHERE action_key IS NOT NULL AND deleted_at IS NULL`
- Soft-deleted action sessions no longer block creating a new chat for the same action
- Backend `resolveSession` uses get-or-create with unique-conflict retry
- Primary lookup: `GET /api/v1/chat/sessions/by-action/{action_id}`
- Long keys: `POST /api/v1/chat/sessions/lookup-by-action` with JSON body
- Legacy: `GET /api/v1/chat/sessions/by-action?action_key=…` (read-only)

## Idempotency
- Client sends `client_request_id` (UUID) on each intentional submit
- Action handoff uses a stable per-action key (`aivion:action-handoff:<action_id>` in sessionStorage)
- Duplicate submits return the prior result via unique `(seller_account_id, client_request_id)`

## Archive vs delete
- **Archive:** session stays in DB with `status=archived`; repeat Ask AI reopens it; new message reactivates
- **Delete:** soft delete (`deleted_at` set); session hidden from lists; repeat Ask AI can create a new session (unique index excludes deleted rows)

## Session menu
- Rename: `PATCH /api/v1/chat/sessions/{id}` with `{ "title": "…" }` (1–80 chars)
- Archive: `POST /api/v1/chat/sessions/{id}/archive`
- Delete: `DELETE /api/v1/chat/sessions/{id}`
- Action sessions: **Открыть действие** → `/app/actions?action=<action_key>`

## Manual send (general chat)
- Page load, New chat, and prompt chips never call `POST /api/v1/chat/ask` until the user sends
- Action handoff is the only automatic single send, guarded by the state machine + idempotency key
