#!/usr/bin/env bash
# Create temporary external audit static bundle.
# Usage: sudo bash create-external-audit.sh --seller-id=1 --ttl-hours=24
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/external-audit-common.sh
source "$SCRIPT_DIR/lib/external-audit-common.sh"

SELLER_ID=1
TTL_HOURS=24
for arg in "$@"; do
  case "$arg" in
    --seller-id=*) SELLER_ID="${arg#*=}" ;;
    --ttl-hours=*) TTL_HOURS="${arg#*=}" ;;
    --help|-h) echo "Usage: $0 --seller-id=1 --ttl-hours=24"; exit 0 ;;
    *) die "unknown arg: $arg" ;;
  esac
done

require_root
ensure_dirs
[[ -s "$DEMO_PASSWORD_FILE" ]] || die "missing demo password file"

BUNDLE_ID="$(gen_bundle_id)"
BUNDLE_ROOT="$AUDIT_BUNDLES/$BUNDLE_ID"
STAGE="$AUDIT_WORK/create-$BUNDLE_ID"
META_JSON="$STAGE.meta.json"
CAPTURE_META="$STAGE.capture.json"
CREATED_AT="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
if date -u -d "+${TTL_HOURS} hours" +"%Y-%m-%dT%H:%M:%SZ" >/dev/null 2>&1; then
  EXPIRES_AT="$(date -u -d "+${TTL_HOURS} hours" +"%Y-%m-%dT%H:%M:%SZ")"
else
  EXPIRES_AT="$(python3 -c "from datetime import datetime,timedelta,timezone;print((datetime.now(timezone.utc)+timedelta(hours=int('$TTL_HOURS'))).strftime('%Y-%m-%dT%H:%M:%SZ'))")"
fi
SHA="$(deployed_sha)"

rm -rf "$STAGE"
mkdir -p "$BUNDLE_ROOT" "$STAGE"
chmod 700 "$BUNDLE_ROOT"

on_fail() {
  echo "create_failed=1" >&2
  rm -rf "$STAGE" 2>/dev/null || true
}
trap on_fail ERR

run_capture() {
  if command -v node >/dev/null 2>&1; then
    if [[ ! -d "$AUDIT_TOOL/node_modules/playwright" ]]; then
      (cd "$AUDIT_TOOL" && npm ci --omit=dev)
    fi
    (cd "$AUDIT_TOOL" && node src/cli.mjs capture \
      --out "$STAGE" \
      --base-url "$BASE_URL" \
      --email demo@example.com \
      --password-file "$DEMO_PASSWORD_FILE" \
      --seller-id "$SELLER_ID") >"$CAPTURE_META"
    return
  fi
  local image="${PLAYWRIGHT_IMAGE:-mcr.microsoft.com/playwright:v1.51.1-jammy}"
  echo "step=capture_via_docker"
  docker run --rm --network host \
    -v "$AUDIT_TOOL:/audit:rw" \
    -v "$STAGE:/stage:rw" \
    -v "$DEMO_PASSWORD_FILE:/secrets/demo_password:ro" \
    -w /audit \
    "$image" \
    bash -lc 'npm ci --omit=dev >/dev/null && node src/cli.mjs capture --out /stage --base-url '"$BASE_URL"' --email demo@example.com --password-file /secrets/demo_password --seller-id '"$SELLER_ID" \
    >"$CAPTURE_META"
}

echo "step=capture"
run_capture

python3 - "$CAPTURE_META" "$META_JSON" "$SELLER_ID" <<'PY'
import json, pathlib, sys
cap = json.loads(pathlib.Path(sys.argv[1]).read_text())
meta = {
  "seller_id": int(sys.argv[3]),
  "publication_id": cap.get("publication_id"),
  "source_run_id": cap.get("source_run_id"),
  "source_mode": cap.get("source_mode"),
  "metric_date": cap.get("metric_date"),
  "dashboard_block_count": cap.get("dashboard_block_count"),
  "action_count": cap.get("action_count"),
  "action_detail_count": cap.get("action_detail_count"),
  "screenshot_count": cap.get("screenshot_count"),
  "chat_context_present": cap.get("chat_context_present"),
}
pathlib.Path(sys.argv[2]).write_text(json.dumps(meta))
print("step=meta_ok")
PY

echo "step=assemble"
(
  cd "$AUDIT_TOOL"
  export AUDIT_OMIT_BUNDLE_ID=1
  node src/cli.mjs assemble \
    --work "$STAGE" \
    --bundle-id "$BUNDLE_ID" \
    --revision 1 \
    --meta-in "$META_JSON" \
    --deployed-sha "$SHA" \
    --deployed-frontend-sha "$SHA" \
    --deployed-backend-sha "$SHA" \
    --deployed-worker-sha "$SHA" \
    --created-at "$CREATED_AT" \
    --refreshed-at "$CREATED_AT" \
    --expires-at "$EXPIRES_AT"
)

echo "step=activate"
(
  cd "$AUDIT_TOOL"
  export AUDIT_OMIT_BUNDLE_ID=1
  node src/cli.mjs activate \
    --bundle-root "$BUNDLE_ROOT" \
    --revision-name rev-001 \
    --revision-dir "$STAGE" \
    --published-root "$AUDIT_PUBLISHED" \
    --bundle-id "$BUNDLE_ID"
)

# Persist private meta (not served)
cat >"$BUNDLE_ROOT/meta.json" <<EOF
{
  "bundle_id": "$BUNDLE_ID",
  "seller_id": $SELLER_ID,
  "created_at": "$CREATED_AT",
  "expires_at": "$EXPIRES_AT",
  "ttl_hours": $TTL_HOURS,
  "active_revision": 1
}
EOF
chmod 600 "$BUNDLE_ROOT/meta.json"

URL="${BASE_URL}/external-audit/${BUNDLE_ID}/"
write_url_secret "$URL"

# Best-effort anonymous smoke (no cookies)
code="$(curl -sS -o /dev/null -w '%{http_code}' "$URL" || true)"
echo "anonymous_index_http=${code}"
[[ "$code" == "200" ]] || echo "WARN: expected 200 on audit index (is Caddy route deployed?)" >&2

trap - ERR
rm -f "$CAPTURE_META" "$META_JSON"
echo "create_ok=1 revision=1"
echo "expires_at=${EXPIRES_AT}"
