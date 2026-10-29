#!/usr/bin/env bash
# Shared helpers for external-audit scripts. Never echo bundle URLs or passwords.
set -euo pipefail

AUDIT_ROOT="${AUDIT_ROOT:-/opt/aivion/shared/external-audit}"
AUDIT_BUNDLES="${AUDIT_BUNDLES:-$AUDIT_ROOT/bundles}"
AUDIT_PUBLISHED="${AUDIT_PUBLISHED:-$AUDIT_ROOT/published}"
AUDIT_WORK="${AUDIT_WORK:-$AUDIT_ROOT/work}"
REPO_ROOT="${REPO_ROOT:-/opt/aivion/repo}"
AUDIT_TOOL="${AUDIT_TOOL:-$REPO_ROOT/deploy/external-audit}"
URL_SECRET_FILE="${URL_SECRET_FILE:-/etc/aivion/secrets/external_audit_url}"
DEMO_PASSWORD_FILE="${DEMO_PASSWORD_FILE:-/etc/aivion/secrets/demo_user_password}"
BASE_URL="${BASE_URL:-https://aivion.pro}"
COMPOSE_FILE="${COMPOSE_FILE:-/opt/aivion/repo/deploy/compose/docker-compose.yml}"
RUNTIME_ENV="${RUNTIME_ENV:-/opt/aivion/shared/runtime.env}"

die() { echo "ERROR: $*" >&2; exit 1; }

require_root() {
  if [[ "$(id -u)" -ne 0 ]]; then
    die "must run as root (sudo)"
  fi
}

ensure_dirs() {
  install -d -m 0750 -o root -g root "$AUDIT_ROOT" "$AUDIT_BUNDLES" "$AUDIT_PUBLISHED" "$AUDIT_WORK"
  install -d -m 0700 -o root -g root "$(dirname "$URL_SECRET_FILE")"
}

deployed_sha() {
  cat /opt/aivion/releases/current_sha 2>/dev/null || git -C "$REPO_ROOT" rev-parse HEAD
}

gen_bundle_id() {
  # 256 bits hex
  openssl rand -hex 32
}

node_audit() {
  # Run tooling with system node if available; else playwright docker image.
  if command -v node >/dev/null 2>&1; then
    if [[ ! -d "$AUDIT_TOOL/node_modules/playwright" ]]; then
      (cd "$AUDIT_TOOL" && npm ci --omit=dev)
    fi
    (cd "$AUDIT_TOOL" && node "$@")
    return
  fi
  local image="${PLAYWRIGHT_IMAGE:-mcr.microsoft.com/playwright:v1.51.1-jammy}"
  docker run --rm \
    -v "$AUDIT_TOOL:/audit:rw" \
    -v "$AUDIT_WORK:/work:rw" \
    -v "$DEMO_PASSWORD_FILE:/secrets/demo_password:ro" \
    -e DEMO_PASSWORD_FILE=/secrets/demo_password \
    -e BASE_URL="$BASE_URL" \
    -w /audit \
    "$image" \
    bash -lc 'if [[ ! -d node_modules/playwright ]]; then npm ci --omit=dev; fi; node "$@"' -- "$@"
}

write_url_secret() {
  local url="$1"
  umask 077
  printf '%s\n' "$url" >"$URL_SECRET_FILE"
  chmod 600 "$URL_SECRET_FILE"
  chown root:root "$URL_SECRET_FILE"
  # Optional operator copy without printing
  if [[ -n "${LOCAL_URL_COPY:-}" ]]; then
    printf '%s\n' "$url" >"$LOCAL_URL_COPY"
    chmod 600 "$LOCAL_URL_COPY" || true
  fi
  echo "url_file=${URL_SECRET_FILE}"
  echo "token_omitted=true"
}
