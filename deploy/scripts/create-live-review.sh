#!/usr/bin/env bash
# Create a temporary Live Seller Review URL for an external AI auditor.
# Raw invite token is never printed to stdout.
set -euo pipefail

TTL_RAW="${1:-2h}"
COMPOSE_FILE="${COMPOSE_FILE:-/opt/aivion/repo/deploy/compose/docker-compose.yml}"
RUNTIME_ENV="${RUNTIME_ENV:-/opt/aivion/shared/runtime.env}"
CADDY_LIVE_REVIEW="${CADDY_LIVE_REVIEW:-/etc/aivion/live-review.caddy}"
STATE_FILE="${STATE_FILE:-/etc/aivion/secrets/live_review_state.json}"
URL_FILE="${URL_FILE:-/etc/aivion/secrets/live_review_url}"
TOKEN_FILE="${TOKEN_FILE:-/etc/aivion/secrets/live_review_token}"
STUB_SRC="${STUB_SRC:-/opt/aivion/repo/deploy/caddy/live-review.caddy.stub}"
export AIVION_IMAGE_TAG="${AIVION_IMAGE_TAG:-$(cat /opt/aivion/releases/current_sha)}"

parse_hours() {
  local raw="$1"
  if [[ "$raw" =~ ^([0-9]+)h$ ]]; then
    echo "${BASH_REMATCH[1]}"
    return
  fi
  if [[ "$raw" =~ ^[0-9]+$ ]]; then
    echo "$raw"
    return
  fi
  echo "usage: create-live-review.sh <hours|Nh>  (max 24h)" >&2
  exit 2
}

HOURS="$(parse_hours "$TTL_RAW")"
if (( HOURS <= 0 || HOURS > 24 )); then
  echo "TTL must be between 1 and 24 hours" >&2
  exit 2
fi

umask 077
mkdir -p /etc/aivion /etc/aivion/secrets
if [[ ! -f "$CADDY_LIVE_REVIEW" ]]; then
  if [[ -f "$STUB_SRC" ]]; then
    cp "$STUB_SRC" "$CADDY_LIVE_REVIEW"
  else
    printf '# live review disabled\n' > "$CADDY_LIVE_REVIEW"
  fi
  chmod 644 "$CADDY_LIVE_REVIEW"
fi

TMP_INVITE="$(mktemp /tmp/live_review_invite.XXXXXX)"
chmod 600 "$TMP_INVITE"
trap 'rm -f "$TMP_INVITE"' EXIT

CREATE_OUT="$(
  docker compose -f "$COMPOSE_FILE" --env-file "$RUNTIME_ENV" exec -T backend-api \
    /app/admin review-invite create \
    --seller-account-id 1 \
    --hours "$HOURS" \
    --base-url https://aivion.pro \
    --out /tmp/live_review_invite_url \
    --label live-ai-auditor
)"

CID="$(docker compose -f "$COMPOSE_FILE" --env-file "$RUNTIME_ENV" ps -q backend-api)"
docker cp "${CID}:/tmp/live_review_invite_url" "$TMP_INVITE"
docker compose -f "$COMPOSE_FILE" --env-file "$RUNTIME_ENV" exec -T backend-api rm -f /tmp/live_review_invite_url

INVITE_URL="$(tr -d '\r\n' < "$TMP_INVITE")"
RAW_TOKEN="${INVITE_URL##*/review/}"
if [[ ! "$RAW_TOKEN" =~ ^[0-9a-f]{64}$ ]]; then
  echo "failed to parse review invite token from invite file" >&2
  exit 1
fi

INVITE_ID="$(printf '%s\n' "$CREATE_OUT" | sed -n 's/^review_invite_id=//p' | head -n1)"
EXPIRES_AT="$(printf '%s\n' "$CREATE_OUT" | sed -n 's/^expires_at_utc=//p' | head -n1)"
if [[ -z "$INVITE_ID" || -z "$EXPIRES_AT" ]]; then
  echo "failed to parse invite metadata from admin CLI" >&2
  exit 1
fi

HOST_LABEL="$(openssl rand -hex 16)"
HOST="${HOST_LABEL}.review.aivion.pro"
LIVE_URL="https://${HOST}/app/dashboard"
CREATED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

printf '%s\n' "$RAW_TOKEN" > "$TOKEN_FILE"
chmod 600 "$TOKEN_FILE"
chown root:root "$TOKEN_FILE" 2>/dev/null || true

TMP_CADDY="$(mktemp /tmp/live-review.caddy.XXXXXX)"
chmod 600 "$TMP_CADDY"
cat > "$TMP_CADDY" <<EOF
${HOST} {
	encode gzip zstd

	header {
		Strict-Transport-Security "max-age=31536000"
		Cache-Control "no-store, private, max-age=0"
		Pragma "no-cache"
		Expires "0"
		X-Robots-Tag "noindex, nofollow, noarchive, nosnippet"
		Referrer-Policy "no-referrer"
		X-Content-Type-Options "nosniff"
		X-Frame-Options "DENY"
		-Server
		-Set-Cookie
	}

	reverse_proxy frontend:3000 {
		header_up -X-Aivion-Live-Review
		header_up -X-Aivion-Live-Review-Token
		header_up -X-Aivion-Live-Review-Host

		header_up X-Aivion-Live-Review "1"
		header_up X-Aivion-Live-Review-Token "${RAW_TOKEN}"
		header_up X-Aivion-Live-Review-Host "${HOST}"

		transport http {
			read_timeout 900s
			write_timeout 900s
			dial_timeout 10s
		}

		flush_interval -1
	}
}
EOF

# Scrub token from shell history risk: overwrite RAW_TOKEN variable after write.
RAW_TOKEN=""

install -m 644 "$TMP_CADDY" "$CADDY_LIVE_REVIEW"
rm -f "$TMP_CADDY"

TMP_STATE="$(mktemp /tmp/live_review_state.XXXXXX)"
chmod 600 "$TMP_STATE"
cat > "$TMP_STATE" <<EOF
{
  "invite_id": ${INVITE_ID},
  "host": "${HOST}",
  "url": "${LIVE_URL}",
  "expires_at": "${EXPIRES_AT}",
  "created_at": "${CREATED_AT}"
}
EOF
install -m 600 "$TMP_STATE" "$STATE_FILE"
chown root:root "$STATE_FILE" 2>/dev/null || true
rm -f "$TMP_STATE"

printf '%s\n' "$LIVE_URL" > "$URL_FILE"
chmod 600 "$URL_FILE"
chown root:root "$URL_FILE" 2>/dev/null || true

# Validate + reload Caddy inside the compose service.
docker compose -f "$COMPOSE_FILE" --env-file "$RUNTIME_ENV" exec -T caddy \
  caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile
docker compose -f "$COMPOSE_FILE" --env-file "$RUNTIME_ENV" exec -T caddy \
  caddy reload --config /etc/caddy/Caddyfile --adapter caddyfile

# Best-effort health: internal Caddy healthz + optional live host probe.
docker compose -f "$COMPOSE_FILE" --env-file "$RUNTIME_ENV" exec -T caddy \
  wget -q --spider http://127.0.0.1:9080/healthz || true

echo "status=enabled"
echo "expires_at=${EXPIRES_AT}"
echo "url_file=${URL_FILE}"
echo "token_omitted=true"
