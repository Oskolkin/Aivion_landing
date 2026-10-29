#!/usr/bin/env bash
# Deploy a pinned Aivion release by image SHA/tag.
# Usage: deploy.sh <IMAGE_SHA>
set -euo pipefail

IMAGE_SHA="${1:-}"
if [[ -z "${IMAGE_SHA}" ]]; then
  echo "usage: $0 <IMAGE_SHA>" >&2
  exit 2
fi

AIVION_ROOT="${AIVION_ROOT:-/opt/aivion}"
AIVION_REPO="${AIVION_REPO:-${AIVION_ROOT}/repo}"
AIVION_SHARED="${AIVION_SHARED:-${AIVION_ROOT}/shared}"
AIVION_RELEASES="${AIVION_RELEASES:-${AIVION_ROOT}/releases}"
AIVION_BACKUPS="${AIVION_BACKUPS:-${AIVION_ROOT}/backups}"
AIVION_SECRETS="${AIVION_SECRETS:-/etc/aivion/secrets}"
COMPOSE_FILE="${COMPOSE_FILE:-${AIVION_REPO}/deploy/compose/docker-compose.yml}"
LOCK_FILE="${LOCK_FILE:-/var/lock/aivion-deploy.lock}"
RUNTIME_ENV="${RUNTIME_ENV:-${AIVION_SHARED}/runtime.env}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

BACKEND_IMAGE="ghcr.io/oskolkin/aivion-backend"
WORKER_IMAGE="ghcr.io/oskolkin/aivion-worker"
FRONTEND_IMAGE="ghcr.io/oskolkin/aivion-frontend"

if [[ ! -f "${COMPOSE_FILE}" ]]; then
  echo "compose file not found: ${COMPOSE_FILE}" >&2
  exit 1
fi
if [[ ! -f "${RUNTIME_ENV}" ]]; then
  echo "runtime env not found: ${RUNTIME_ENV}" >&2
  exit 1
fi
if [[ -z "${ACME_EMAIL:-}" ]]; then
  echo "ACME_EMAIL must be set in the environment" >&2
  exit 1
fi

mkdir -p "${AIVION_RELEASES}" "${AIVION_BACKUPS}/postgres" "${AIVION_BACKUPS}/evidence"
exec 9>"${LOCK_FILE}"
if ! flock -n 9; then
  echo "another deploy/rollback holds ${LOCK_FILE}" >&2
  exit 1
fi

export AIVION_IMAGE_TAG="${IMAGE_SHA}"
export COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-aivion}"

# Fail before pulling/building when the production root filesystem lacks room.
"${SCRIPT_DIR}/storage-guard.sh" preflight

# Keep the release currently recorded by the normal deploy flow as a rollback
# candidate. Cleanup fails closed if the corresponding image refs are absent.
PREVIOUS_IMAGE_SHA="$(tr -d '[:space:]' <"${AIVION_RELEASES}/current_sha" 2>/dev/null || true)"

compose() {
  docker compose -f "${COMPOSE_FILE}" --env-file "${RUNTIME_ENV}" "$@"
}

echo "==> pulling images for tag ${IMAGE_SHA}"
docker pull "${BACKEND_IMAGE}:${IMAGE_SHA}"
docker pull "${WORKER_IMAGE}:${IMAGE_SHA}"
docker pull "${FRONTEND_IMAGE}:${IMAGE_SHA}"

# Extract migrations from the immutable backend image so migrate/migrate
# does not depend on a mutable git checkout path.
MIG_DIR="${AIVION_RELEASES}/${IMAGE_SHA}/migrations"
mkdir -p "${AIVION_RELEASES}/${IMAGE_SHA}"
MIG_TMP="$(mktemp -d)"
CID="$(docker create "${BACKEND_IMAGE}:${IMAGE_SHA}")"
docker cp "${CID}:/app/migrations" "${MIG_TMP}/migrations"
docker rm -f "${CID}" >/dev/null
rm -rf "${MIG_DIR}"
mv "${MIG_TMP}/migrations" "${MIG_DIR}"
rm -rf "${MIG_TMP}"
export AIVION_MIGRATIONS_PATH="${MIG_DIR}"

digest_of() {
  local ref="$1"
  docker image inspect --format '{{index .RepoDigests 0}}' "${ref}" 2>/dev/null || true
}

BACKEND_DIGEST="$(digest_of "${BACKEND_IMAGE}:${IMAGE_SHA}")"
WORKER_DIGEST="$(digest_of "${WORKER_IMAGE}:${IMAGE_SHA}")"
FRONTEND_DIGEST="$(digest_of "${FRONTEND_IMAGE}:${IMAGE_SHA}")"

if [[ -z "${BACKEND_DIGEST}" || -z "${WORKER_DIGEST}" || -z "${FRONTEND_DIGEST}" ]]; then
  echo "failed to resolve RepoDigests after pull" >&2
  exit 1
fi

echo "==> image digests:"
echo "    backend:  ${BACKEND_DIGEST}"
echo "    worker:   ${WORKER_DIGEST}"
echo "    frontend: ${FRONTEND_DIGEST}"

verify_one_digest() {
  local label="$1"
  local full_ref="$2"
  local expected="$3"
  [[ -z "${expected}" ]] && return 0
  expected="${expected#sha256:}"
  local got="${full_ref##*@}"
  got="${got#sha256:}"
  if [[ "${got}" != "${expected}" ]]; then
    echo "digest mismatch for ${label}: got sha256:${got}, expected sha256:${expected}" >&2
    exit 1
  fi
  echo "==> verified ${label} digest"
}

# Optional per-image digests. AIVION_IMAGE_DIGEST (if set) verifies the backend image only.
verify_one_digest "backend" "${BACKEND_DIGEST}" "${AIVION_BACKEND_DIGEST:-${AIVION_IMAGE_DIGEST:-}}"
verify_one_digest "worker" "${WORKER_DIGEST}" "${AIVION_WORKER_DIGEST:-}"
verify_one_digest "frontend" "${FRONTEND_DIGEST}" "${AIVION_FRONTEND_DIGEST:-}"

echo "==> pre-deploy postgres backup"
"${SCRIPT_DIR}/backup-postgres.sh" "pre-deploy-${IMAGE_SHA}"

echo "==> starting dependencies"
compose up -d postgres redis minio
compose up -d --wait postgres redis minio || {
  echo "dependency health wait failed" >&2
  compose ps
  exit 1
}
compose run --rm minio-init

echo "==> running migrations"
compose run --rm migrations

echo "==> starting application services"
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
  echo "services did not become ready in time" >&2
  compose ps
  exit 1
fi

RELEASE_DIR="${AIVION_RELEASES}/${IMAGE_SHA}-$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -p "${RELEASE_DIR}"
cat >"${RELEASE_DIR}/metadata.json" <<EOF
{
  "image_sha": "${IMAGE_SHA}",
  "deployed_at_utc": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "backend_digest": "${BACKEND_DIGEST}",
  "worker_digest": "${WORKER_DIGEST}",
  "frontend_digest": "${FRONTEND_DIGEST}",
  "compose_file": "${COMPOSE_FILE}",
  "app_env": "$(grep -E '^APP_ENV=' "${RUNTIME_ENV}" | head -n1 | cut -d= -f2- || true)"
}
EOF
if [[ -f "${AIVION_RELEASES}/current_sha" ]]; then
  cp -f "${AIVION_RELEASES}/current_sha" "${AIVION_RELEASES}/previous_sha"
fi
ln -sfn "${RELEASE_DIR}" "${AIVION_RELEASES}/current"
echo "${IMAGE_SHA}" >"${AIVION_RELEASES}/current_sha"

echo "==> smoke checks"
"${SCRIPT_DIR}/smoke.sh" || {
  echo "smoke checks failed" >&2
  exit 1
}

if [[ -n "${PREVIOUS_IMAGE_SHA}" ]]; then
  echo "==> controlled post-deploy Docker cleanup"
  "${SCRIPT_DIR}/storage-guard.sh" cleanup --apply \
    --frontend-rollback "${FRONTEND_IMAGE}:${PREVIOUS_IMAGE_SHA}" \
    --backend-rollback "${BACKEND_IMAGE}:${PREVIOUS_IMAGE_SHA}" \
    --worker-rollback "${WORKER_IMAGE}:${PREVIOUS_IMAGE_SHA}" || {
      echo "post-deploy cleanup skipped: rollback image set could not be verified" >&2
    }
else
  echo "==> post-deploy cleanup skipped: no verified previous release tag"
fi

echo "==> deploy complete: ${IMAGE_SHA}"
echo "    release metadata: ${RELEASE_DIR}/metadata.json"
