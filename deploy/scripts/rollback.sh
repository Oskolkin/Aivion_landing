#!/usr/bin/env bash
# Roll application images back to a previous SHA.
# WARNING: database migrations are NOT automatically reversed. If the newer
# release applied schema changes, restore a postgres backup first or confirm
# forward-compatible schema before rolling back app containers.
#
# Usage: rollback.sh [IMAGE_SHA]
#   If IMAGE_SHA is omitted, uses /opt/aivion/releases/previous_sha.
set -euo pipefail

AIVION_ROOT="${AIVION_ROOT:-/opt/aivion}"
AIVION_REPO="${AIVION_REPO:-${AIVION_ROOT}/repo}"
AIVION_SHARED="${AIVION_SHARED:-${AIVION_ROOT}/shared}"
AIVION_RELEASES="${AIVION_RELEASES:-${AIVION_ROOT}/releases}"
COMPOSE_FILE="${COMPOSE_FILE:-${AIVION_REPO}/deploy/compose/docker-compose.yml}"
LOCK_FILE="${LOCK_FILE:-/var/lock/aivion-deploy.lock}"
RUNTIME_ENV="${RUNTIME_ENV:-${AIVION_SHARED}/runtime.env}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

TARGET_SHA="${1:-}"
if [[ -z "${TARGET_SHA}" ]]; then
  if [[ ! -f "${AIVION_RELEASES}/previous_sha" ]]; then
    echo "no IMAGE_SHA argument and ${AIVION_RELEASES}/previous_sha missing" >&2
    exit 2
  fi
  TARGET_SHA="$(tr -d '[:space:]' <"${AIVION_RELEASES}/previous_sha")"
fi

if [[ -z "${TARGET_SHA}" ]]; then
  echo "empty rollback target SHA" >&2
  exit 2
fi
if [[ -z "${ACME_EMAIL:-}" ]]; then
  echo "ACME_EMAIL must be set in the environment" >&2
  exit 1
fi

echo "WARNING: rollback does not reverse SQL migrations."
echo "         Confirm schema compatibility or restore postgres backup before continuing."
echo "         Target image SHA: ${TARGET_SHA}"
read -r -p "Type 'rollback' to continue: " confirm
if [[ "${confirm}" != "rollback" ]]; then
  echo "aborted" >&2
  exit 1
fi

mkdir -p "${AIVION_RELEASES}"
exec 9>"${LOCK_FILE}"
if ! flock -n 9; then
  echo "another deploy/rollback holds ${LOCK_FILE}" >&2
  exit 1
fi

export AIVION_IMAGE_TAG="${TARGET_SHA}"
export AIVION_MIGRATIONS_PATH="${AIVION_MIGRATIONS_PATH:-${AIVION_REPO}/backend/migrations}"
export COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-aivion}"

compose() {
  docker compose -f "${COMPOSE_FILE}" --env-file "${RUNTIME_ENV}" "$@"
}

echo "==> pulling rollback images"
docker pull "ghcr.io/oskolkin/aivion-backend:${TARGET_SHA}"
docker pull "ghcr.io/oskolkin/aivion-worker:${TARGET_SHA}"
docker pull "ghcr.io/oskolkin/aivion-frontend:${TARGET_SHA}"

echo "==> switching app services to ${TARGET_SHA}"
# Do not re-run migrations on rollback.
compose up -d backend-api backend-worker frontend caddy

echo "==> waiting for readiness"
ready=0
for _ in $(seq 1 60); do
  if compose exec -T backend-api wget -q -O - http://127.0.0.1:8081/health/ready >/dev/null 2>&1 \
    && compose exec -T frontend wget -q -O - http://127.0.0.1:3000/health >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 5
done
if [[ "${ready}" -ne 1 ]]; then
  echo "rollback target did not become ready" >&2
  compose ps
  exit 1
fi

RELEASE_DIR="${AIVION_RELEASES}/rollback-${TARGET_SHA}-$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -p "${RELEASE_DIR}"
cat >"${RELEASE_DIR}/metadata.json" <<EOF
{
  "image_sha": "${TARGET_SHA}",
  "rolled_back_at_utc": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "note": "app images rolled back; migrations not reversed"
}
EOF
if [[ -f "${AIVION_RELEASES}/current_sha" ]]; then
  cp -f "${AIVION_RELEASES}/current_sha" "${AIVION_RELEASES}/previous_sha"
fi
ln -sfn "${RELEASE_DIR}" "${AIVION_RELEASES}/current"
echo "${TARGET_SHA}" >"${AIVION_RELEASES}/current_sha"

"${SCRIPT_DIR}/smoke.sh"
echo "==> rollback complete: ${TARGET_SHA}"
