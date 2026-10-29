# Ozon connection and onboarding

## Required credentials

Both integrations are mandatory for MVP:

1. **Ozon Seller API** — `Client ID` + `API Key` (личный кабинет Ozon Seller → Настройки → Seller API).
2. **Ozon Performance API** — `client_id` + `client_secret` (рекламный кабинет → API-ключи / сервисный аккаунт).

Users do **not** enter Performance `access_token`. The backend requests it via:

`POST https://api-performance.ozon.ru/api/client/token`

```json
{
  "client_id": "...",
  "client_secret": "...",
  "grant_type": "client_credentials"
}
```

The token is stored encrypted with `expires_at` and refreshed automatically before expiry.

## `connection_ready`

A seller can use the product only when:

- `seller_api_valid = true`
- `performance_api_valid = true`
- `connection_status = connected`
- not `disconnected_retention`

User-facing APIs return `409` with `error: connection_required` when blocked.

## User UI

- Single screen: **Подключение кабинета Ozon** (onboarding) or **Настройки подключения Ozon** (after connect).
- After connect, seller navigation includes **Товары** (`/app/products`) instead of the removed MVP pricing-constraints screen. Legacy `/app/pricing-constraints` redirects to `/app/products`.
- Shows only credentials, human-readable errors, disconnect, and masked Client ID / “ключ сохранён” flags — no secrets.
- No sync status page, last sync, job IDs, or manual sync buttons for sellers (`/app/sync-status` redirects to dashboard).
- Technical sync/import diagnostics: **admin support only** (`/app/admin/clients/{id}`).
- After successful connect, initial sync is enqueued automatically; background sync uses `SYNC_INTERVAL` (default `5m`).

## Disconnect and retention

- User confirms with phrase `ОТКЛЮЧИТЬ`.
- Status becomes `disconnected_retention`; credentials cleared; `retention_delete_after = now + 90 days`.
- Reconnecting with valid credentials clears retention fields and restores access.
- Hard data purge after retention is handled by `DISCONNECTED_ACCOUNT_CLEANUP_ENABLED` worker job (skeleton logs intent; full cascade delete is a follow-up).

## Temporary Ozon outages

If validation fails with a temporary error (`temporary_unavailable`, rate limits, 5xx) and the cabinet was already connected, existing credentials are **not** overwritten.

## Admin / support

Admin routes (`/api/v1/admin/*`) are not gated by `connection_ready`. Technical sync/import diagnostics remain on admin client detail screens.

## API

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/api/v1/integrations/ozon/connection` | User-facing connection state |
| PUT | `/api/v1/integrations/ozon/connection` | Validate + save both APIs, start initial sync |
| POST | `/api/v1/integrations/ozon/disconnect` | Soft disconnect with confirmation |

Legacy endpoints under `/api/v1/integrations/ozon/` remain for compatibility.
