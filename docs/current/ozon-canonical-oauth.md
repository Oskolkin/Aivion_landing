# Canonical Ozon OAuth transport

Status: **DERIVED** from `docs/architecture/v1/00-architecture-baseline.md`.

`/oauth/start` requires the authenticated seller session, creates a signed short-lived state and redirects to Ozon. `/oauth/callback` verifies the state, exchanges the authorization code server-side, encrypts access/refresh tokens using the existing secret codec, persists scopes/expiry/status, and redirects without returning token material.

`SellerBoundTransport` is the canonical boundary. It resolves a valid token by seller ID and sends `Authorization: Bearer …` to Seller and Performance APIs. Expiring tokens are refreshed under `SELECT … FOR UPDATE`; a concurrent worker reuses the freshly persisted token. Canonical code has no Client-Id/API-Key, Performance client-credentials, `/api/client/token`, or `client_credentials` fallback.

The old credential endpoints and transport remain only for legacy production compatibility. `collect-catalog`, Canonical Sales/Lifecycle and the Stock/Price/Advertising collectors use the canonical transport and fail with an explicit not-connected error when Private OAuth is absent. Stock uses `/v1/analytics/stocks`, Price uses `/v5/product/info/prices`, and every Performance request obtains the same seller-scoped private token; none can fall back to legacy headers or client credentials.

Disconnect updates connection/retention state and clears OAuth grants. It never deletes `seller_accounts`; physical purge remains a separate administrative operation.
