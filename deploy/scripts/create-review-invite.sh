#!/usr/bin/env bash
# Create a temporary external review invite on the VPS (token never printed).
set -euo pipefail
HOURS="${1:-24}"
OUT_FILE="${2:-/etc/aivion/secrets/review_invite_url}"
COMPOSE_FILE="${COMPOSE_FILE:-/opt/aivion/repo/deploy/compose/docker-compose.yml}"
RUNTIME_ENV="${RUNTIME_ENV:-/opt/aivion/shared/runtime.env}"
export AIVION_IMAGE_TAG="${AIVION_IMAGE_TAG:-$(cat /opt/aivion/releases/current_sha)}"

docker compose -f "$COMPOSE_FILE" --env-file "$RUNTIME_ENV" exec -T backend-api \
  /app/admin review-invite create \
  --seller-account-id 1 \
  --hours "$HOURS" \
  --base-url https://aivion.pro \
  --out /tmp/review_invite_url \
  --label external-auditor

# Copy from container to host secret path without echoing token.
CID="$(docker compose -f "$COMPOSE_FILE" --env-file "$RUNTIME_ENV" ps -q backend-api)"
docker cp "${CID}:/tmp/review_invite_url" "$OUT_FILE"
chmod 600 "$OUT_FILE"
chown root:root "$OUT_FILE" 2>/dev/null || true
docker compose -f "$COMPOSE_FILE" --env-file "$RUNTIME_ENV" exec -T backend-api rm -f /tmp/review_invite_url
echo "url_file=${OUT_FILE}"
echo "token_omitted=true"
