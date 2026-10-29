#!/usr/bin/env bash
# Install operator-provided Xray JSON for ai-egress.
# Usage: sudo bash install-ai-egress-config.sh /path/to/config.json
# Never prints config contents.
#
# Expects Xray JSON with an HTTP inbound on port 10809 for AI_HTTPS_PROXY.
# Official ghcr.io/xtls/xray-core runs as UID/GID 65532 (distroless nonroot).
set -euo pipefail

SRC="${1:-}"
DEST=/etc/aivion/secrets/ai-egress-config.json
# Match distroless/static:nonroot in ghcr.io/xtls/xray-core
XRAY_UID=65532
XRAY_GID=65532

if [[ -z "${SRC}" || ! -f "${SRC}" ]]; then
  echo "usage: $0 /path/to/ai-egress-config.json" >&2
  exit 2
fi

install -d -m 0750 -o root -g root /etc/aivion/secrets
install -m 0640 -o root -g "${XRAY_GID}" "${SRC}" "${DEST}"
# Confirm metadata only — never cat the file.
stat -c '%U:%G %a %n' "${DEST}"

# Structural checks without dumping secrets (Xray uses "port", not sing-box "listen_port").
if ! grep -q '"protocol"[[:space:]]*:[[:space:]]*"http"' "${DEST}"; then
  echo "WARN: config may be missing HTTP inbound protocol (required for AI_HTTPS_PROXY)" >&2
fi
if ! grep -qE '"port"[[:space:]]*:[[:space:]]*10809' "${DEST}"; then
  echo "WARN: config may be missing HTTP inbound port 10809 (verify before deploy)" >&2
fi
# Reject accidental sing-box-shaped keys as the sole inbound contract.
if grep -q '"listen_port"' "${DEST}" && ! grep -q '"protocol"' "${DEST}"; then
  echo "FAIL: file looks like sing-box JSON; ai-egress expects Xray JSON" >&2
  exit 1
fi
echo ai_egress_config_installed
