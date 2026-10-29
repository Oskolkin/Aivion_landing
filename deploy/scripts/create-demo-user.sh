#!/usr/bin/env bash
# Ensure the canonical public-demo login user inside a one-shot backend container.
# Password is read from a file via AIVION_DEMO_PASSWORD_FILE (never CLI args / logs).
#
# Usage:
#   AIVION_DEMO_PASSWORD_FILE=/etc/aivion/secrets/demo_user_password \
#     create-demo-user.sh [--email demo@example.com] [--seller-account-id 1] \
#       [--reset-password] [--transfer-seller-ownership]
set -euo pipefail

AIVION_ROOT="${AIVION_ROOT:-/opt/aivion}"
AIVION_REPO="${AIVION_REPO:-${AIVION_ROOT}/repo}"
AIVION_SHARED="${AIVION_SHARED:-${AIVION_ROOT}/shared}"
COMPOSE_FILE="${COMPOSE_FILE:-${AIVION_REPO}/deploy/compose/docker-compose.yml}"
RUNTIME_ENV="${RUNTIME_ENV:-${AIVION_SHARED}/runtime.env}"
PASSWORD_FILE="${AIVION_DEMO_PASSWORD_FILE:-/etc/aivion/secrets/demo_user_password}"

EMAIL="demo@example.com"
SELLER_ID="1"
EXTRA_FLAGS=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --email)
      EMAIL="${2:-}"
      shift 2
      ;;
    --seller-account-id)
      SELLER_ID="${2:-}"
      shift 2
      ;;
    --reset-password|--transfer-seller-ownership)
      EXTRA_FLAGS+=("$1")
      shift
      ;;
    -h|--help)
      echo "usage: $0 [--email demo@example.com] [--seller-account-id 1] [--reset-password] [--transfer-seller-ownership]"
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
  echo "set AIVION_DEMO_PASSWORD_FILE or create /etc/aivion/secrets/demo_user_password (root:root 0600)" >&2
  exit 1
fi

export AIVION_IMAGE_TAG="${AIVION_IMAGE_TAG:-$(tr -d '[:space:]' <"${AIVION_ROOT}/releases/current_sha" 2>/dev/null || echo unset)}"
export ACME_EMAIL="${ACME_EMAIL:-ops@localhost}"
export COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-aivion}"
export AIVION_MIGRATIONS_PATH="${AIVION_MIGRATIONS_PATH:-${AIVION_REPO}/backend/migrations}"

compose() {
  docker compose -f "${COMPOSE_FILE}" --env-file "${RUNTIME_ENV}" "$@"
}

echo "==> ensuring demo user ${EMAIL} (seller_account_id=${SELLER_ID})"
# Backend image runs as uid 10001 (appuser). Root-owned 0600 secrets are unreadable
# inside the container, so stage a mode 0444 temp copy for this one-shot only.
STAGE_PASS="$(mktemp)"
cleanup() { rm -f "${STAGE_PASS}"; }
trap cleanup EXIT
install -m 0444 "${PASSWORD_FILE}" "${STAGE_PASS}"

# Reuse AIVION_ADMIN_PASSWORD_FILE env expected by adminbootstrap.ReadPasswordInteractively.
compose run --rm --no-deps \
  -e "AIVION_ADMIN_PASSWORD_FILE=/run/demo_user_password" \
  -v "${STAGE_PASS}:/run/demo_user_password:ro" \
  --entrypoint /app/admin \
  backend-api \
  ensure-demo-user --email "${EMAIL}" --seller-account-id "${SELLER_ID}" "${EXTRA_FLAGS[@]+"${EXTRA_FLAGS[@]}"}"

echo "==> demo user ensure finished"
