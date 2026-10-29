#!/usr/bin/env bash
# Post-deploy smoke checks (internal + optional public).
set -euo pipefail

AIVION_ROOT="${AIVION_ROOT:-/opt/aivion}"
AIVION_REPO="${AIVION_REPO:-${AIVION_ROOT}/repo}"
AIVION_SHARED="${AIVION_SHARED:-${AIVION_ROOT}/shared}"
COMPOSE_FILE="${COMPOSE_FILE:-${AIVION_REPO}/deploy/compose/docker-compose.yml}"
RUNTIME_ENV="${RUNTIME_ENV:-${AIVION_SHARED}/runtime.env}"

export AIVION_IMAGE_TAG="${AIVION_IMAGE_TAG:-$(tr -d '[:space:]' <"${AIVION_ROOT}/releases/current_sha" 2>/dev/null || echo unset)}"
export ACME_EMAIL="${ACME_EMAIL:-smoke@localhost}"
export COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-aivion}"

compose() {
  docker compose -f "${COMPOSE_FILE}" --env-file "${RUNTIME_ENV}" "$@"
}

fail=0

check() {
  local name="$1"
  shift
  echo -n "check ${name}... "
  if "$@" >/dev/null 2>&1; then
    echo "ok"
  else
    echo "FAIL"
    fail=1
  fi
}

check "backend live" compose exec -T backend-api wget -q -O - http://127.0.0.1:8081/health/live
check "backend ready" compose exec -T backend-api wget -q -O - http://127.0.0.1:8081/health/ready
check "frontend health" compose exec -T frontend wget -q -O - http://127.0.0.1:3000/health
check "postgres ready" compose exec -T postgres pg_isready -U aivion -d marketplace_ai
check "redis ping" compose exec -T redis /bin/sh -c 'redis-cli -a "$(cat /run/secrets/redis_password)" ping | grep -q PONG'
check "minio live" compose exec -T minio curl -sf http://127.0.0.1:9000/minio/health/live

# Optional public checks (may require Basic Auth for staging HTML/API).
if [[ "${SMOKE_PUBLIC:-0}" == "1" ]]; then
  check "public staging TLS" curl -fsS -o /dev/null -w "%{http_code}" --connect-timeout 10 https://staging.aivion.pro/ || true
  # Expect 401 without credentials when basic_auth is enabled.
  code="$(curl -sS -o /dev/null -w "%{http_code}" --connect-timeout 10 https://staging.aivion.pro/ || echo 000)"
  echo "check staging http code (expect 401 without auth)... ${code}"
  code_app="$(curl -sS -o /dev/null -w "%{http_code}" --connect-timeout 10 https://app.aivion.pro/ || echo 000)"
  echo "check app http code (503 pre-promote or 200/3xx post)... ${code_app}"
fi

if [[ "${fail}" -ne 0 ]]; then
  echo "smoke checks failed" >&2
  exit 1
fi
echo "smoke checks passed"
