# Stage 7 Alert Idempotency

Alerts are idempotent by design to avoid duplicates on repeated engine runs.

## Fingerprint model

Each alert has:

- `fingerprint` (`TEXT NOT NULL`)
- unique key `(seller_account_id, fingerprint)`

Fingerprint is deterministic and built from:

1. `seller_account_id`
2. `alert_type`
3. `entity_type`
4. stable entity identity (`BuildEntityIdentity`)

It does **not** depend on `title`, `message`, `severity`, `urgency`, or `evidence_payload`.

## Upsert lifecycle

`UpsertAlert` uses `ON CONFLICT (seller_account_id, fingerprint)`.

Repeated upsert for same fingerprint:

- does not create a new row
- updates lifecycle fields (`last_seen_at`, `updated_at`) and latest alert payload

Status transitions:

- `open` + repeat -> remains `open`
- `resolved` + repeat -> reopens to `open` and clears `resolved_at`
- `dismissed` + repeat -> remains `dismissed` (no automatic reopen)

This keeps dismissed behavior stable while still refreshing recency metadata.

## Deferred behavior

Automatic stale/missing alert reconciliation is deferred in MVP and is not part of idempotency upsert semantics.
