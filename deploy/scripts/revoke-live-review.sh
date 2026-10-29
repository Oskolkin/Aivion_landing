#!/usr/bin/env bash
# Revoke the active Live Seller Review invite and disable the temporary host.
set -euo pipefail

COMPOSE_FILE="${COMPOSE_FILE:-/opt/aivion/repo/deploy/compose/docker-compose.yml}"
RUNTIME_ENV="${RUNTIME_ENV:-/opt/aivion/shared/runtime.env}"
CADDY_LIVE_REVIEW="${CADDY_LIVE_REVIEW:-/etc/aivion/live-review.caddy}"
STATE_FILE="${STATE_FILE:-/etc/aivion/secrets/live_review_state.json}"
URL_FILE="${URL_FILE:-/etc/aivion/secrets/live_review_url}"
TOKEN_FILE="${TOKEN_FILE:-/etc/aivion/secrets/live_review_token}"
REVOKE_AUDIT="${REVOKE_AUDIT:-/etc/aivion/secrets/live_review_revoke_audit.json}"
STUB_SRC="${STUB_SRC:-/opt/aivion/repo/deploy/caddy/live-review.caddy.stub}"
export AIVION_IMAGE_TAG="${AIVION_IMAGE_TAG:-$(cat /opt/aivion/releases/current_sha)}"

if [[ ! -f "$STATE_FILE" ]]; then
  echo "no live review state at ${STATE_FILE}" >&2
  exit 1
fi

INVITE_ID="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["invite_id"])' "$STATE_FILE")"
HOST="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1])).get("host",""))' "$STATE_FILE")"
PREV_URL="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1])).get("url",""))' "$STATE_FILE")"

if [[ -z "$INVITE_ID" || "$INVITE_ID" == "0" ]]; then
  echo "invalid invite_id in state file" >&2
  exit 1
fi

docker compose -f "$COMPOSE_FILE" --env-file "$RUNTIME_ENV" exec -T backend-api \
  /app/admin review-invite revoke --id "$INVITE_ID"

if [[ -f "$STUB_SRC" ]]; then
  install -m 644 "$STUB_SRC" "$CADDY_LIVE_REVIEW"
else
  printf '# live review disabled\n' > "$CADDY_LIVE_REVIEW"
  chmod 644 "$CADDY_LIVE_REVIEW"
fi

docker compose -f "$COMPOSE_FILE" --env-file "$RUNTIME_ENV" exec -T caddy \
  caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile
docker compose -f "$COMPOSE_FILE" --env-file "$RUNTIME_ENV" exec -T caddy \
  caddy reload --config /etc/caddy/Caddyfile --adapter caddyfile

# Securely remove token + URL; keep a token-free revoke audit.
REVOKED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
umask 077
cat > "$REVOKE_AUDIT" <<EOF
{
  "invite_id": ${INVITE_ID},
  "host": "${HOST}",
  "url": "${PREV_URL}",
  "revoked_at": "${REVOKED_AT}",
  "token_omitted": true
}
EOF
chmod 600 "$REVOKE_AUDIT"

rm -f "$URL_FILE" "$TOKEN_FILE" "$STATE_FILE"

# Best-effort check that the previous host no longer serves seller data.
if [[ -n "$HOST" ]]; then
  CODE="$(curl -sS -o /dev/null -w '%{http_code}' --max-time 15 "https://${HOST}/api/v1/auth/me" || true)"
  echo "post_revoke_auth_me_status=${CODE:-unknown}"
fi

echo "status=disabled"
echo "revoked_invite_id=${INVITE_ID}"
echo "token_omitted=true"
