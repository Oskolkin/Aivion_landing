#!/usr/bin/env bash
# Backup non-secret deployment configuration (never copies secret values).
# Usage: backup-config.sh [label]
set -euo pipefail

AIVION_ROOT="${AIVION_ROOT:-/opt/aivion}"
AIVION_REPO="${AIVION_REPO:-${AIVION_ROOT}/repo}"
AIVION_SHARED="${AIVION_SHARED:-${AIVION_ROOT}/shared}"
AIVION_RELEASES="${AIVION_RELEASES:-${AIVION_ROOT}/releases}"
AIVION_BACKUPS="${AIVION_BACKUPS:-${AIVION_ROOT}/backups}"
AIVION_SECRETS="${AIVION_SECRETS:-/etc/aivion/secrets}"
LABEL="${1:-manual}"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
OUT_DIR="${AIVION_BACKUPS}/config/${LABEL}-${STAMP}"

mkdir -p "${OUT_DIR}"
chmod 0750 "${OUT_DIR}"

echo "==> collecting config snapshot (redacted)"

# Caddyfile(s) — no secrets expected in-repo copies.
if [[ -f "${AIVION_REPO}/deploy/caddy/Caddyfile" ]]; then
  cp -a "${AIVION_REPO}/deploy/caddy/Caddyfile" "${OUT_DIR}/Caddyfile"
fi
if [[ -f "${AIVION_REPO}/deploy/caddy/Caddyfile.promoted" ]]; then
  cp -a "${AIVION_REPO}/deploy/caddy/Caddyfile.promoted" "${OUT_DIR}/Caddyfile.promoted"
fi
if [[ -f "${AIVION_REPO}/deploy/compose/docker-compose.yml" ]]; then
  cp -a "${AIVION_REPO}/deploy/compose/docker-compose.yml" "${OUT_DIR}/docker-compose.yml"
fi

# Redacted runtime.env: keep keys, mask values that look sensitive.
if [[ -f "${AIVION_SHARED}/runtime.env" ]]; then
  awk -F= '
    BEGIN { OFS="=" }
    /^[[:space:]]*#/ || NF == 0 { print; next }
    {
      key=$1
      val=substr($0, index($0, "=") + 1)
      lower=tolower(key)
      if (lower ~ /(password|secret|token|key|database_url|encryption)/) {
        print key, "***REDACTED***"
      } else {
        print key, val
      }
    }
  ' "${AIVION_SHARED}/runtime.env" >"${OUT_DIR}/runtime.env.redacted"
fi

# Secret filenames only (never contents).
if [[ -d "${AIVION_SECRETS}" ]]; then
  find "${AIVION_SECRETS}" -maxdepth 1 -type f -printf '%f\n' 2>/dev/null \
    | sort >"${OUT_DIR}/secrets-file-list.txt" || \
  find "${AIVION_SECRETS}" -maxdepth 1 -type f -exec basename {} \; 2>/dev/null \
    | sort >"${OUT_DIR}/secrets-file-list.txt"
fi

if [[ -f "${AIVION_RELEASES}/current_sha" ]]; then
  cp -a "${AIVION_RELEASES}/current_sha" "${OUT_DIR}/current_sha"
fi
if [[ -f "${AIVION_RELEASES}/previous_sha" ]]; then
  cp -a "${AIVION_RELEASES}/previous_sha" "${OUT_DIR}/previous_sha"
fi
if [[ -e "${AIVION_RELEASES}/current/metadata.json" ]]; then
  cp -a "${AIVION_RELEASES}/current/metadata.json" "${OUT_DIR}/release-metadata.json"
fi

# Compose service status (no env dumps).
if command -v docker >/dev/null 2>&1; then
  docker compose -f "${AIVION_REPO}/deploy/compose/docker-compose.yml" \
    --env-file "${AIVION_SHARED}/runtime.env" ps >"${OUT_DIR}/compose-ps.txt" 2>/dev/null || true
fi

cat >"${OUT_DIR}/README.txt" <<EOF
Aivion config backup (no secret values).
created_at_utc=${STAMP}
label=${LABEL}
EOF

chmod -R go-rwx "${OUT_DIR}" || true
echo "==> config backup complete: ${OUT_DIR}"
