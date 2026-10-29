#!/usr/bin/env bash
# Revoke a review invite and all related review sessions.
set -euo pipefail
INVITE_ID="${1:?usage: revoke-review-invite.sh <invite_id>}"
COMPOSE_FILE="${COMPOSE_FILE:-/opt/aivion/repo/deploy/compose/docker-compose.yml}"
RUNTIME_ENV="${RUNTIME_ENV:-/opt/aivion/shared/runtime.env}"
export AIVION_IMAGE_TAG="${AIVION_IMAGE_TAG:-$(cat /opt/aivion/releases/current_sha)}"

docker compose -f "$COMPOSE_FILE" --env-file "$RUNTIME_ENV" exec -T backend-api \
  /app/admin review-invite revoke --id "$INVITE_ID"

rm -f /etc/aivion/secrets/review_invite_url
echo "revoked_invite_id=${INVITE_ID}"
