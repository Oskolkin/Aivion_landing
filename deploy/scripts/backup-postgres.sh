#!/usr/bin/env bash
# Dump Postgres from the running compose postgres service.
# Usage: backup-postgres.sh [label]
set -euo pipefail

AIVION_ROOT="${AIVION_ROOT:-/opt/aivion}"
AIVION_REPO="${AIVION_REPO:-${AIVION_ROOT}/repo}"
AIVION_SHARED="${AIVION_SHARED:-${AIVION_ROOT}/shared}"
AIVION_BACKUPS="${AIVION_BACKUPS:-${AIVION_ROOT}/backups}"
COMPOSE_FILE="${COMPOSE_FILE:-${AIVION_REPO}/deploy/compose/docker-compose.yml}"
RUNTIME_ENV="${RUNTIME_ENV:-${AIVION_SHARED}/runtime.env}"
LABEL="${1:-manual}"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
OUT_DIR="${AIVION_BACKUPS}/postgres"
OUT_FILE="${OUT_DIR}/marketplace_ai-${LABEL}-${STAMP}.sql.gz"

export AIVION_IMAGE_TAG="${AIVION_IMAGE_TAG:-$(tr -d '[:space:]' <"${AIVION_ROOT}/releases/current_sha" 2>/dev/null || echo unset)}"
export ACME_EMAIL="${ACME_EMAIL:-backup@localhost}"

mkdir -p "${OUT_DIR}"
compose() {
  docker compose -f "${COMPOSE_FILE}" --env-file "${RUNTIME_ENV}" "$@"
}

if ! compose ps --status running postgres 2>/dev/null | grep -q postgres; then
  echo "postgres service is not running; cannot backup" >&2
  exit 1
fi

echo "==> writing ${OUT_FILE}"
compose exec -T postgres pg_dump -U aivion -d marketplace_ai --no-owner --no-acl \
  | gzip -c >"${OUT_FILE}"
chmod 0640 "${OUT_FILE}"
echo "==> postgres backup complete ($(du -h "${OUT_FILE}" | awk '{print $1}'))"
