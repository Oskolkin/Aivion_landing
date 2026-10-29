#!/usr/bin/env bash
# Show Live Seller Review status without printing tokens.
set -euo pipefail

COMPOSE_FILE="${COMPOSE_FILE:-/opt/aivion/repo/deploy/compose/docker-compose.yml}"
RUNTIME_ENV="${RUNTIME_ENV:-/opt/aivion/shared/runtime.env}"
CADDY_LIVE_REVIEW="${CADDY_LIVE_REVIEW:-/etc/aivion/live-review.caddy}"
STATE_FILE="${STATE_FILE:-/etc/aivion/secrets/live_review_state.json}"
URL_FILE="${URL_FILE:-/etc/aivion/secrets/live_review_url}"
export AIVION_IMAGE_TAG="${AIVION_IMAGE_TAG:-$(cat /opt/aivion/releases/current_sha)}"

ENABLED=false
HOST=""
INVITE_ID=""
EXPIRES_AT=""
EXPIRED="n/a"
CADDY_LOADED=false
AUTH_STATUS="n/a"
MUTATIONS_BLOCKED="n/a"

if [[ -f "$STATE_FILE" ]]; then
  ENABLED=true
  HOST="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1])).get("host",""))' "$STATE_FILE")"
  INVITE_ID="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1])).get("invite_id",""))' "$STATE_FILE")"
  EXPIRES_AT="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1])).get("expires_at",""))' "$STATE_FILE")"
  EXPIRED="$(python3 -c '
import json,sys
from datetime import datetime, timezone
raw=json.load(open(sys.argv[1])).get("expires_at","")
try:
  exp=datetime.fromisoformat(raw.replace("Z","+00:00"))
  print("yes" if exp <= datetime.now(timezone.utc) else "no")
except Exception:
  print("unknown")
' "$STATE_FILE")"
fi

if [[ -f "$CADDY_LIVE_REVIEW" ]] && grep -qE '^[a-z0-9]+\.review\.aivion\.pro' "$CADDY_LIVE_REVIEW"; then
  CADDY_LOADED=true
  ENABLED=true
  if [[ -z "$HOST" ]]; then
    HOST="$(grep -E '^[a-z0-9]+\.review\.aivion\.pro' "$CADDY_LIVE_REVIEW" | head -n1 | awk '{print $1}')"
  fi
elif [[ -f "$CADDY_LIVE_REVIEW" ]] && grep -qi 'live review disabled' "$CADDY_LIVE_REVIEW"; then
  CADDY_LOADED=false
  if [[ ! -f "$STATE_FILE" ]]; then
    ENABLED=false
  fi
fi

if [[ -n "$HOST" ]]; then
  AUTH_STATUS="$(curl -sS -o /tmp/live_review_me.json -w '%{http_code}' --max-time 20 \
    "https://${HOST}/api/v1/auth/me" || echo fail)"
  # Ensure response body does not get printed; scrub temp file.
  if [[ -f /tmp/live_review_me.json ]]; then
    # Mutation probe (should be 403 when live review auth works).
    MUT_CODE="$(curl -sS -o /dev/null -w '%{http_code}' --max-time 20 \
      -X POST "https://${HOST}/api/v1/chat/ask" \
      -H 'Content-Type: application/json' \
      -d '{"question":"ping"}' || echo fail)"
    if [[ "$MUT_CODE" == "403" ]]; then
      MUTATIONS_BLOCKED=yes
    elif [[ "$MUT_CODE" == "401" || "$MUT_CODE" == "fail" ]]; then
      MUTATIONS_BLOCKED="auth_failed"
    else
      MUTATIONS_BLOCKED="no (got ${MUT_CODE})"
    fi
    rm -f /tmp/live_review_me.json
  fi
fi

echo "enabled=${ENABLED}"
echo "hostname=${HOST:-none}"
echo "invite_id=${INVITE_ID:-none}"
echo "expires_at=${EXPIRES_AT:-none}"
echo "expired=${EXPIRED}"
echo "caddy_config_loaded=${CADDY_LOADED}"
echo "auth_me_http_status=${AUTH_STATUS}"
echo "mutations_blocked=${MUTATIONS_BLOCKED}"
if [[ -f "$URL_FILE" ]]; then
  echo "url_file=${URL_FILE}"
fi
echo "token_omitted=true"
