#!/usr/bin/env bash
# Delete external audit bundle and URL secret.
# Usage: sudo bash delete-external-audit.sh <bundle-id>
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/external-audit-common.sh
source "$SCRIPT_DIR/lib/external-audit-common.sh"

BUNDLE_ID="${1:-}"
[[ -n "$BUNDLE_ID" ]] || die "usage: $0 <bundle-id>"
echo "delete_start id_len=${#BUNDLE_ID}"

require_root

rm -rf "$AUDIT_BUNDLES/$BUNDLE_ID" \
  "$AUDIT_PUBLISHED/$BUNDLE_ID" \
  "$AUDIT_PUBLISHED/${BUNDLE_ID}.next" \
  "$AUDIT_PUBLISHED/${BUNDLE_ID}.prev" \
  "$AUDIT_WORK"/*"$BUNDLE_ID"* 2>/dev/null || true

if [[ -f "$URL_SECRET_FILE" ]]; then
  # Only remove if it points at this bundle
  if grep -q "$BUNDLE_ID" "$URL_SECRET_FILE" 2>/dev/null; then
    shred -u "$URL_SECRET_FILE" 2>/dev/null || rm -f "$URL_SECRET_FILE"
    echo "url_secret_removed=1"
  fi
fi

code="$(curl -sS -o /dev/null -w '%{http_code}' "${BASE_URL}/external-audit/${BUNDLE_ID}/" || echo 000)"
png="$(curl -sS -o /dev/null -w '%{http_code}' "${BASE_URL}/external-audit/${BUNDLE_ID}/screenshots/desktop/dashboard-full.png" || echo 000)"
echo "index_after_delete=${code}"
echo "png_after_delete=${png}"
[[ "$code" == "404" || "$code" == "410" ]] || echo "WARN: expected 404 after delete" >&2

echo "delete_ok=1"
