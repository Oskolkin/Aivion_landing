#!/usr/bin/env bash
# Promote aivion.pro from stub/maintenance to the live shared demo app.
# Swaps Caddyfile to Caddyfile.promoted and reloads Caddy.
#
# Target topology (SHARED_PUBLIC_DEMO_STACK_WITH_PROTECTED_STAGING_ENTRYPOINT):
#   aivion.pro          → live app (no Basic Auth)
#   www.aivion.pro      → 308 → aivion.pro{uri}
#   app.aivion.pro      → 308 → aivion.pro{uri}
#   staging.aivion.pro  → same app + Basic Auth (unchanged)
set -euo pipefail

AIVION_ROOT="${AIVION_ROOT:-/opt/aivion}"
AIVION_REPO="${AIVION_REPO:-${AIVION_ROOT}/repo}"
AIVION_SHARED="${AIVION_SHARED:-${AIVION_ROOT}/shared}"
COMPOSE_FILE="${COMPOSE_FILE:-${AIVION_REPO}/deploy/compose/docker-compose.yml}"
RUNTIME_ENV="${RUNTIME_ENV:-${AIVION_SHARED}/runtime.env}"
CADDY_DIR="${CADDY_DIR:-${AIVION_REPO}/deploy/caddy}"
LOCK_FILE="${AIVION_ROOT}/releases/.promote.lock"

if [[ ! -f "${CADDY_DIR}/Caddyfile.promoted" ]]; then
  echo "missing ${CADDY_DIR}/Caddyfile.promoted" >&2
  exit 1
fi

echo "This will serve the app on https://aivion.pro (public demo)."
echo "www/app will 308-redirect to aivion.pro; staging Basic Auth stays."
read -r -p "Type 'promote' to continue: " confirm
if [[ "${confirm}" != "promote" ]]; then
  echo "aborted"
  exit 1
fi

exec 9>"${LOCK_FILE}"
if ! flock -n 9; then
  echo "another deploy/rollback/promote holds ${LOCK_FILE}" >&2
  exit 1
fi

BACKUP="${CADDY_DIR}/Caddyfile.pre-promote.$(date -u +%Y%m%dT%H%M%SZ)"
cp -a "${CADDY_DIR}/Caddyfile" "${BACKUP}"
cp -a "${CADDY_DIR}/Caddyfile.promoted" "${CADDY_DIR}/Caddyfile"
echo "==> wrote promoted Caddyfile (backup: ${BACKUP})"

compose() {
  docker compose -f "${COMPOSE_FILE}" --env-file "${RUNTIME_ENV}" "$@"
}

export AIVION_IMAGE_TAG="${AIVION_IMAGE_TAG:-$(tr -d '[:space:]' <"${AIVION_ROOT}/releases/current_sha" 2>/dev/null || echo unset)}"
export ACME_EMAIL="${ACME_EMAIL:-ops@localhost}"
export COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-aivion}"

# Recreate caddy so the bind-mounted Caddyfile is reloaded cleanly.
compose up -d --force-recreate --no-deps caddy

compose exec -T caddy caddy validate --config /etc/caddy/Caddyfile || {
  echo "Caddyfile validation failed; restoring previous file" >&2
  cp -a "${BACKUP}" "${CADDY_DIR}/Caddyfile"
  compose up -d --force-recreate --no-deps caddy
  exit 1
}

echo "==> promote complete"
echo "    verify: curl -I https://aivion.pro/"
echo "    verify: curl -I https://www.aivion.pro/login"
echo "    verify: curl -I https://app.aivion.pro/login"
echo "    rollback: cp -a ${BACKUP} ${CADDY_DIR}/Caddyfile && compose up -d --force-recreate --no-deps caddy"
