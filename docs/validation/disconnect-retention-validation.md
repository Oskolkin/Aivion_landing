# Disconnect & retention validation

## Code references

- Disconnect: `PUT /api/v1/integrations/ozon/disconnect` → `ozon.Service.DisconnectUserConnection`
- Credentials cleared via `DisconnectOzonConnection` (encrypted empty strings)
- Status: `disconnected_retention`, `disconnected_at`, `retention_delete_after`
- Retention days: `DISCONNECTED_ACCOUNT_RETENTION_DAYS` (default **90**)
- Periodic sync: `ListOzonConnectionsForPeriodicSync` only `connection_status = 'connected'` with valid APIs
- Cleanup job: `DISCONNECTED_ACCOUNT_CLEANUP_ENABLED` + `disconnected_account:cleanup` worker task

## Manual scenarios

| Scenario | Expected |
|----------|----------|
| Connected → disconnect with confirmation `ОТКЛЮЧИТЬ` | Status `disconnected_retention`; credentials deactivated |
| Seller opens app after disconnect | Connection gate; no seller-facing sync |
| Reconnect within retention | `PUT /connection` works; can return to `connected` |
| Worker periodic sync | Disconnected seller not in periodic sync list |
| After `retention_delete_after` (with cleanup enabled) | Cleanup job enqueues purge per existing implementation |
| Admin client detail | Shows disconnected / retention fields |

## Automated checks

- `backend/internal/integrations/ozon/connection_state_test.go` — `ConnectionReady` false when disconnected
- `backend/internal/integrations/ozon/connection_put_test.go` — disconnect clears encrypted credential fields
