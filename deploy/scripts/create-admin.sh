#!/usr/bin/env bash
# Create (or optionally reset) an admin user inside a one-shot backend container.
# Password is read from a file via AIVION_ADMIN_PASSWORD_FILE (never CLI args).
#
# Usage:
#   AIVION_ADMIN_PASSWORD_FILE=/etc/aivion/secrets/admin_password \
#     create-admin.sh --email admin@example.com [--reset-password]
set -euo pipefail

AIVION_ROOT="${AIVION_ROOT:-/opt/aivion}"
AIVION_REPO="${AIVION_REPO:-${AIVION_ROOT}/repo}"
AIVION_SHARED="${AIVION_SHARED:-${AIVION_ROOT}/shared}"
COMPOSE_FILE="${COMPOSE_FILE:-${AIVION_REPO}/deploy/compose/docker-compose.yml}"
RUNTIME_ENV="${RUNTIME_ENV:-${AIVION_SHARED}/runtime.env}"
PASSWORD_FILE="${AIVION_ADMIN_PASSWORD_FILE:-/etc/aivion/secrets/admin_password}"

EMAIL=""
RESET_FLAG=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --email)
      EMAIL="${2:-}"
      shift 2
      ;;
    --reset-password)
      RESET_FLAG=(--reset-password)
      shift
      ;;
    -h|--help)
      echo "usage: $0 --email <email> [--reset-password]"
      exit 0
      ;;
    *)
      echo "unknown arg: $1" >&2
      exit 2
      ;;
  esac
done

if [[ -z "${EMAIL}" ]]; then
  echo "--email is required" >&2
  exit 2
fi
if [[ ! -f "${PASSWORD_FILE}" ]]; then
  echo "password file not found: ${PASSWORD_FILE}" >&2
  echo "set AIVION_ADMIN_PASSWORD_FILE or create /etc/aivion/secrets/admin_password" >&2
  exit 1
fi

export AIVION_IMAGE_TAG="${AIVION_IMAGE_TAG:-$(tr -d '[:space:]' <"${AIVION_ROOT}/releases/current_sha" 2>/dev/null || echo unset)}"
export ACME_EMAIL="${ACME_EMAIL:-ops@localhost}"
export COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-aivion}"
export AIVION_MIGRATIONS_PATH="${AIVION_MIGRATIONS_PATH:-${AIVION_REPO}/backend/migrations}"

compose() {
  docker compose -f "${COMPOSE_FILE}" --env-file "${RUNTIME_ENV}" "$@"
}

echo "==> running admin create for ${EMAIL}"
# Backend image must ship /app/admin. Password file is bind-mounted read-only.
compose run --rm --no-deps \
  -e "AIVION_ADMIN_PASSWORD_FILE=/run/admin_password" \
  -v "${PASSWORD_FILE}:/run/admin_password:ro" \
  --entrypoint /app/admin \
  backend-api \
  create --email "${EMAIL}" "${RESET_FLAG[@]+"${RESET_FLAG[@]}"}"

echo "==> admin create finished"
