#!/usr/bin/env bash
# Refresh external audit bundle in-place (same URL). Does not extend TTL unless --extend-hours=N.
# Usage: sudo bash refresh-external-audit.sh <bundle-id> [--extend-hours=0]
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/external-audit-common.sh
source "$SCRIPT_DIR/lib/external-audit-common.sh"

BUNDLE_ID="${1:-}"
EXTEND_HOURS=0
shift || true
for arg in "$@"; do
  case "$arg" in
    --extend-hours=*) EXTEND_HOURS="${arg#*=}" ;;
    *) die "unknown arg: $arg" ;;
  esac
done
[[ -n "$BUNDLE_ID" ]] || die "usage: $0 <bundle-id> [--extend-hours=0]"
echo "refresh_start id_len=${#BUNDLE_ID}"

require_root
ensure_dirs
[[ -s "$DEMO_PASSWORD_FILE" ]] || die "missing demo password file"
command -v node >/dev/null 2>&1 || die "node is required"

BUNDLE_ROOT="$AUDIT_BUNDLES/$BUNDLE_ID"
[[ -d "$BUNDLE_ROOT" ]] || die "bundle not found"
[[ -f "$BUNDLE_ROOT/meta.json" ]] || die "bundle meta missing"

PREV_REV="$(python3 -c "import json;print(json.load(open('$BUNDLE_ROOT/meta.json')).get('active_revision',1))")"
NEXT_REV=$((PREV_REV + 1))
REV_NAME="$(printf 'rev-%03d' "$NEXT_REV")"
PREV_DIR="$BUNDLE_ROOT/revisions/$(printf 'rev-%03d' "$PREV_REV")"
STAGE="$AUDIT_WORK/refresh-$BUNDLE_ID-$$"
META_JSON="$STAGE.meta.json"
CAPTURE_META="$STAGE.capture.json"
CREATED_AT="$(python3 -c "import json;print(json.load(open('$BUNDLE_ROOT/meta.json'))['created_at'])")"
EXPIRES_AT="$(python3 -c "import json;print(json.load(open('$BUNDLE_ROOT/meta.json'))['expires_at'])")"
if [[ "$EXTEND_HOURS" != "0" ]]; then
  EXPIRES_AT="$(python3 -c "from datetime import datetime,timedelta,timezone;print((datetime.now(timezone.utc)+timedelta(hours=int('$EXTEND_HOURS'))).strftime('%Y-%m-%dT%H:%M:%SZ'))")"
fi
REFRESHED_AT="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
SHA="$(deployed_sha)"
SELLER_ID="$(python3 -c "import json;print(json.load(open('$BUNDLE_ROOT/meta.json')).get('seller_id',1))")"

rm -rf "$STAGE"
mkdir -p "$STAGE"

on_fail() {
  echo "refresh_failed=1 keeping_previous_revision=${PREV_REV}" >&2
  rm -rf "$STAGE" 2>/dev/null || true
}
trap on_fail ERR

echo "step=capture"
if [[ ! -d "$AUDIT_TOOL/node_modules/playwright" ]]; then
  (cd "$AUDIT_TOOL" && npm ci --omit=dev)
fi
(cd "$AUDIT_TOOL" && node src/cli.mjs capture \
  --out "$STAGE" \
  --base-url "$BASE_URL" \
  --email demo@example.com \
  --password-file "$DEMO_PASSWORD_FILE" \
  --seller-id "$SELLER_ID") >"$CAPTURE_META"

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
PREV_ARGS=()
if [[ -d "$PREV_DIR" ]]; then
  PREV_ARGS=(--previous-revision-dir "$PREV_DIR" --previous-manifest "$PREV_DIR/manifest.json")
fi
(
  cd "$AUDIT_TOOL"
  export AUDIT_OMIT_BUNDLE_ID=1
  node src/cli.mjs assemble \
    --work "$STAGE" \
    --bundle-id "$BUNDLE_ID" \
    --revision "$NEXT_REV" \
    --meta-in "$META_JSON" \
    "${PREV_ARGS[@]}" \
    --deployed-sha "$SHA" \
    --deployed-frontend-sha "$SHA" \
    --deployed-backend-sha "$SHA" \
    --deployed-worker-sha "$SHA" \
    --created-at "$CREATED_AT" \
    --refreshed-at "$REFRESHED_AT" \
    --expires-at "$EXPIRES_AT"
)

echo "step=activate"
(
  cd "$AUDIT_TOOL"
  export AUDIT_OMIT_BUNDLE_ID=1
  node src/cli.mjs activate \
    --bundle-root "$BUNDLE_ROOT" \
    --revision-name "$REV_NAME" \
    --revision-dir "$STAGE" \
    --published-root "$AUDIT_PUBLISHED" \
    --bundle-id "$BUNDLE_ID"
)

python3 - "$BUNDLE_ROOT/meta.json" "$NEXT_REV" "$EXPIRES_AT" <<'PY'
import json, pathlib, sys
p = pathlib.Path(sys.argv[1])
meta = json.loads(p.read_text())
meta["active_revision"] = int(sys.argv[2])
meta["expires_at"] = sys.argv[3]
p.write_text(json.dumps(meta, indent=2) + "\n")
print("meta_updated")
PY

URL="${BASE_URL}/external-audit/${BUNDLE_ID}/"
write_url_secret "$URL"

trap - ERR
rm -f "$CAPTURE_META" "$META_JSON"
echo "refresh_ok=1 revision=${NEXT_REV}"
echo "expires_at=${EXPIRES_AT}"
