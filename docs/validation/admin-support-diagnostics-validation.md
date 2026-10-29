# Admin / support diagnostics validation

## Implemented (backend)

Admin routes under `/api/v1/admin/*` (require `ADMIN_EMAILS`):

- Clients list & detail, connection statuses
- Sync jobs, import errors, sync cursors
- Rerun sync, reset cursor (with audit logs where implemented)
- Rerun metrics, alerts, recommendations
- AI recommendation runs & raw AI (`GET .../ai/recommendations/{id}`) — **admin-only + audit**
- AI daily summary runs
- Chat traces & feedback
- Billing state (`seller_billing_state`) — **admin placeholder, not payment provider**
- OpenAI usage fields on runs/traces where stored

See `docs/stage-11-admin-support-validation.md` for historical scope.

## Security checks (automated)

- `backend/internal/admin/ai_view_audit_test.go` — raw AI view requires admin + audit
- `backend/internal/httpserver/handlers/recommendations_feedback_test.go` — seller API omits `raw_ai_response`
- `backend/internal/httpserver/handlers/admin_test.go` — admin raw endpoint includes raw payload

## Manual scenarios

| # | Action | Expected |
|---|--------|----------|
| 1 | Seller calls `/api/v1/admin/clients` | 403 Forbidden |
| 2 | Admin lists clients | 200 + rows |
| 3 | Admin opens client detail | Statuses, sync, billing visible |
| 4 | Admin opens recommendation raw AI | Audit log row; secrets redacted in logs |
| 5 | Seller GET `/api/v1/recommendations/{id}` | No `raw_ai_response` field |

## Known gaps (do not expand scope here)

- No full seller-facing admin UI for every diagnostic field; some data is API-only.
- Payment provider / subscription checkout **not implemented**.
