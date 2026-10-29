#!/usr/bin/env bash
# Validate published external audit bundle (structure + anonymous HTTP checks).
# Usage: sudo bash validate-external-audit.sh <bundle-id>
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/external-audit-common.sh
source "$SCRIPT_DIR/lib/external-audit-common.sh"

BUNDLE_ID="${1:-}"
[[ -n "$BUNDLE_ID" ]] || die "usage: $0 <bundle-id>"
echo "validate_start id_len=${#BUNDLE_ID}"

PUB="$AUDIT_PUBLISHED/$BUNDLE_ID"
[[ -d "$PUB" ]] || die "published bundle missing"

command -v node >/dev/null 2>&1 || die "node required"
(cd "$AUDIT_TOOL" && node src/cli.mjs validate --revision "$PUB")

URL="${BASE_URL}/external-audit/${BUNDLE_ID}"
fail=0
check() {
  local path="$1" expect="$2" ctype="${3:-}"
  local code ct
  code="$(curl -sS -o /tmp/audit-val-body -D /tmp/audit-val-hdr -w '%{http_code}' "${URL}${path}" || echo 000)"
  ct="$(grep -i '^Content-Type:' /tmp/audit-val-hdr | head -1 | tr -d '\r' || true)"
  if [[ "$code" != "$expect" ]]; then
    echo "FAIL path=${path} code=${code} expected=${expect}"
    fail=1
  else
    echo "PASS path=${path} code=${code}"
  fi
  if [[ -n "$ctype" ]] && ! grep -qi "$ctype" <<<"$ct"; then
    echo "FAIL path=${path} content-type=${ct}"
    fail=1
  fi
  if grep -qi '^Set-Cookie:' /tmp/audit-val-hdr; then
    echo "FAIL path=${path} set-cookie_present"
    fail=1
  fi
}

check "/" 200
check "/links.html" 200
check "/manifest.json" 200 "json"
check "/screenshots/desktop/dashboard-full.png" 200 "image/png"
check "/screenshots/mobile/dashboard-full.png" 200 "image/png"
check "/visible-content.txt" 200 "text/plain"
check "/metrics-reconciliation.txt" 200 "text/plain"
check "/revision-diff.txt" 200 "text/plain"
check "/visible-content.md" 200
check "/data/dashboard.sanitized.json" 200 "json"
check "/data/actions.sanitized.json" 200 "json"
check "/revision-diff.md" 200

# Source HTML must expose static PNG anchors for web readers.
python3 - "$PUB/index.html" "$PUB/links.html" <<'PY'
import re, sys
index = open(sys.argv[1], encoding="utf-8").read()
links = open(sys.argv[2], encoding="utf-8").read()
a = len(re.findall(r'<a\s+[^>]*href=["\'][^"\']+\.png', index, flags=re.I))
img = len(re.findall(r'<img\s+[^>]*src=["\'][^"\']+\.png', index, flags=re.I))
la = len(re.findall(r'<a\s+[^>]*href=["\'][^"\']+\.png', links, flags=re.I))
assert a >= 30, a
assert img >= 30, img
assert la >= 30, la
assert "links.html" in index
assert "<script" not in index.lower()
print(f"html_anchors_ok a={a} img={img} links_a={la}")
PY


post_code="$(curl -sS -o /dev/null -w '%{http_code}' -X POST "${URL}/" || echo 000)"
if [[ "$post_code" == "405" ]]; then echo "PASS method_post=405"; else echo "FAIL method_post=${post_code}"; fail=1; fi

trav_code="$(curl -sS -o /dev/null -w '%{http_code}' "${URL}/../" || echo 000)"
if [[ "$trav_code" == "404" || "$trav_code" == "400" || "$trav_code" == "403" ]]; then
  echo "PASS traversal_blocked=${trav_code}"
else
  echo "WARN traversal_code=${trav_code}"
fi

bad_code="$(curl -sS -o /dev/null -w '%{http_code}' "${BASE_URL}/external-audit/$(openssl rand -hex 32)/" || echo 000)"
[[ "$bad_code" == "404" ]] && echo "PASS bad_id=404" || echo "FAIL bad_id=${bad_code}"

# Expiry check
python3 - "$PUB/manifest.json" <<'PY' || true
import json,sys,datetime
m=json.load(open(sys.argv[1]))
exp=datetime.datetime.fromisoformat(m["expires_at"].replace("Z","+00:00"))
now=datetime.datetime.now(datetime.timezone.utc)
print("expires_at", m["expires_at"], "expired" if now>exp else "active")
if now>exp:
  sys.exit(2)
PY

[[ "$fail" -eq 0 ]] || exit 1
echo "validate_ok=1"
