# Canonical Account Health

`ACCOUNT_HEALTH` stores Ozon's official account ratings and rating evidence. It does not calculate an Aivion composite score and does not promote provider `OK`, `WARNING`, or `CRITICAL` into data-quality severity.

## Sources

All requests use seller-bound private OAuth Bearer transport.

- `POST /v1/rating/summary` — current snapshot.
- `POST /v1/rating/history` — provider periods, retained as periods rather than expanded to days.
- `POST /v1/rating/index/fbs/info` — FBS error-index observation.
- `POST /v1/rating/index/fbs/posting/list` — paginated posting-error evidence.

JSON decoding uses exact numbers. Payloads use `OZON_ACCOUNT_HEALTH_REDACTION_V1`.

## Identity and time

Metric identity is `(seller, provider, provider_rating_key)`. Group and display names are versioned metadata. Unknown keys are retained with `mapping_status=UNMAPPED`.

Summary `past_value` is stored as `provider_previous_value`; it is not interpreted as `PREVIOUS_7D`. History retains the exact provider period and the provider status observed with that revision. Threshold changes append versions and never rewrite historical status.

## Deterministic hashes (Prompt 7.1)

Optional pointer fields in metric / penalty / FBS hashes use a `NULL` marker or a dereferenced scalar — never `fmt.Sprint(pointer)`. Equal values under separately allocated pointers must hash identically.

## Ownership boundaries

- Rating cost context is not a Finance accrual.
- Price-related ratings do not modify canonical Price.
- Supply-related ratings do not create Supply events.
- Rating posting errors resolve to an existing canonical Posting or remain `UNRESOLVED`; they never create one.

## Factual and readiness

The factual API exposes account summary, current ratings, one rating, rating history, thresholds, FBS index, and FBS posting errors under a cutoff-bound `FactualReadContext`.

Capabilities are independent: `HEALTH.CURRENT_RATINGS`, `HEALTH.RATING_HISTORY`, `HEALTH.THRESHOLDS`, `HEALTH.PENALTY_EVENTS`, `HEALTH.FBS_ERROR_INDEX`, and `HEALTH.FBS_POSTING_ERRORS`.

Watermarks remain separate for `CURRENT_SNAPSHOT`, `RATING_HISTORY`, `FBS_INDEX`, and `FBS_ERROR_EVENTS`.
