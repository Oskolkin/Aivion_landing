#!/usr/bin/env bash
# Mirror MinIO buckets to a host backup directory using the mc image.
# Usage: backup-minio.sh [label]
set -euo pipefail

AIVION_ROOT="${AIVION_ROOT:-/opt/aivion}"
AIVION_REPO="${AIVION_REPO:-${AIVION_ROOT}/repo}"
AIVION_SHARED="${AIVION_SHARED:-${AIVION_ROOT}/shared}"
AIVION_BACKUPS="${AIVION_BACKUPS:-${AIVION_ROOT}/backups}"
AIVION_SECRETS="${AIVION_SECRETS:-/etc/aivion/secrets}"
COMPOSE_FILE="${COMPOSE_FILE:-${AIVION_REPO}/deploy/compose/docker-compose.yml}"
RUNTIME_ENV="${RUNTIME_ENV:-${AIVION_SHARED}/runtime.env}"
LABEL="${1:-manual}"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
OUT_DIR="${AIVION_BACKUPS}/minio/${LABEL}-${STAMP}"

export AIVION_IMAGE_TAG="${AIVION_IMAGE_TAG:-$(tr -d '[:space:]' <"${AIVION_ROOT}/releases/current_sha" 2>/dev/null || echo unset)}"
export ACME_EMAIL="${ACME_EMAIL:-backup@localhost}"

mkdir -p "${OUT_DIR}"
chmod 0750 "${OUT_DIR}"

compose() {
  docker compose -f "${COMPOSE_FILE}" --env-file "${RUNTIME_ENV}" "$@"
}

if ! compose ps --status running minio 2>/dev/null | grep -q minio; then
  echo "minio service is not running; cannot backup" >&2
  exit 1
fi

echo "==> mirroring buckets to ${OUT_DIR}"
docker run --rm \
  --network aivion_internal \
  -v "${OUT_DIR}:/backup" \
  -v "${AIVION_SECRETS}/minio_root_user:/run/secrets/minio_root_user:ro" \
  -v "${AIVION_SECRETS}/minio_root_password:/run/secrets/minio_root_password:ro" \
  --entrypoint /bin/sh \
  minio/mc:RELEASE.2025-08-13T08-35-41Z@sha256:a7fe349ef4bd8521fb8497f55c6042871b2ae640607cf99d9bede5e9bdf11727 \
  -c '
    set -euo pipefail
    USER="$(cat /run/secrets/minio_root_user)"
    PASS="$(cat /run/secrets/minio_root_password)"
    mc alias set local http://minio:9000 "$USER" "$PASS"
    for b in raw-payloads exports artifacts; do
      mkdir -p "/backup/$b"
      mc mirror --overwrite "local/$b" "/backup/$b"
    done
  '

cat >"${OUT_DIR}/MANIFEST.txt" <<EOF
created_at_utc=${STAMP}
buckets=raw-payloads,exports,artifacts
label=${LABEL}
EOF
chmod -R go-rwx "${OUT_DIR}" || true
echo "==> minio backup complete: ${OUT_DIR}"
