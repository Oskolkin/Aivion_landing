#!/usr/bin/env bash
# Static validation for production compose contracts (Caddy zombies / published ports).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
COMPOSE="${ROOT}/deploy/compose/docker-compose.yml"
CADDYFILE="${ROOT}/deploy/caddy/Caddyfile"

fail() { echo "FAIL: $*" >&2; exit 1; }
ok() { echo "OK: $*"; }

[[ -f "${COMPOSE}" ]] || fail "missing ${COMPOSE}"
[[ -f "${CADDYFILE}" ]] || fail "missing ${CADDYFILE}"

# Render with dummy required vars (no daemon needed for `config` if we only grep source).
# Prefer source inspection so CI runners without Docker still work.
grep -qE '^\s+init:\s*true\s*$' "${COMPOSE}" || fail "caddy service must set init: true"
ok "caddy init: true"

# Reject HTTPS / public hosts / :80|:443 in the healthcheck *test* command only.
hc_test="$(
  awk '
    $0 ~ /^  caddy:/ {in_caddy=1; next}
    in_caddy && $0 ~ /^  [a-z]/ {in_caddy=0}
    in_caddy && $0 ~ /healthcheck:/ {in_hc=1; next}
    in_caddy && in_hc && $0 ~ /^\s+test:/ {print; exit}
  ' "${COMPOSE}"
)"
[[ -n "${hc_test}" ]] || fail "caddy healthcheck test line missing"
echo "${hc_test}" | grep -q '127.0.0.1:9080/healthz' || fail "caddy healthcheck must probe http://127.0.0.1:9080/healthz"
ok "caddy healthcheck targets 127.0.0.1:9080/healthz"
if echo "${hc_test}" | grep -Ei 'https://|staging\.aivion|app\.aivion|:443|127\.0\.0\.1:80([^0-9]|$)'; then
  fail "caddy healthcheck must not use HTTPS, public domains, :80, or :443"
fi
ok "caddy healthcheck avoids HTTPS/public ports"

if grep -E '^\s+-\s*"?9080:' "${COMPOSE}"; then
  fail "port 9080 must not be published on the host"
fi
ok "port 9080 not published"

grep -q '127.0.0.1:9080' "${CADDYFILE}" || fail "Caddyfile missing http://127.0.0.1:9080 health site"
grep -q '/healthz' "${CADDYFILE}" || fail "Caddyfile missing /healthz"
ok "Caddyfile has localhost :9080 /healthz"

grep -q 'handle /external-audit/\*' "${CADDYFILE}" || fail "Caddyfile missing /external-audit handle"
grep -q '/srv/external-audit/published' "${CADDYFILE}" || fail "Caddyfile missing external-audit file root"
grep -q 'external-audit/published:/srv/external-audit/published' "${COMPOSE}" || fail "compose missing external-audit volume"
ok "external-audit static route wired"

# No floating :latest on any service image line.
if grep -E '^\s+image:.*:latest($|[^a-zA-Z0-9_.-])' "${COMPOSE}"; then
  fail "floating :latest image tags are forbidden"
fi
ok "no :latest image tags"

# Infrastructure images should be digest-pinned.
for name in caddy postgres redis 'minio/minio' 'minio/mc' 'migrate/migrate' 'xtls/xray-core'; do
  if ! grep -E "image:[[:space:]]*[^\n]*${name}[^[:space:]]*@sha256:[0-9a-f]{64}" "${COMPOSE}" >/dev/null; then
    fail "expected digest pin for ${name}"
  fi
done
ok "infra images digest-pinned"

# Reject sing-box as ai-egress runtime (HAPP exports Xray JSON; no approximate conversion).
if grep -E '^\s+ai-egress:' -A50 "${COMPOSE}" | grep -q 'sing-box'; then
  fail "ai-egress must use official Xray (ghcr.io/xtls/xray-core), not sing-box"
fi
ok "ai-egress is not sing-box"

EXAMPLE_CFG="${ROOT}/deploy/ai-egress/config.example.json"
[[ -f "${EXAMPLE_CFG}" ]] || fail "missing ${EXAMPLE_CFG}"
grep -q '"protocol"[[:space:]]*:[[:space:]]*"http"' "${EXAMPLE_CFG}" || fail "example config must declare HTTP inbound protocol"
grep -qE '"port"[[:space:]]*:[[:space:]]*10809' "${EXAMPLE_CFG}" || fail "example config must listen on port 10809"
# Example must not look like a real VLESS credential dump.
if grep -Ei '"id"[[:space:]]*:[[:space:]]*"[0-9a-f-]{36}"' "${EXAMPLE_CFG}"; then
  fail "example config must not contain UUID credentials"
fi
if grep -Ei 'pbk|publicKey|shortId|serverName' "${EXAMPLE_CFG}"; then
  fail "example config must not contain Reality/VLESS credential fields"
fi
ok "ai-egress example is credential-free Xray HTTP template"

# Secret path must not be vendored into the repo as a real secret file.
if [[ -f "${ROOT}/etc/aivion/secrets/ai-egress-config.json" ]]; then
  fail "ai-egress-config.json must not live in the Git tree"
fi
if git -C "${ROOT}" ls-files | grep -Eq '(^|/)ai-egress-config\.json$'; then
  fail "ai-egress-config.json must not be tracked by Git"
fi
ok "ai-egress operator config is not in Git"

# ai-egress contract
ai_egress_block="$(awk '
  $0 ~ /^  ai-egress:/ {in_svc=1; print; next}
  in_svc && $0 ~ /^  [a-z]/ {exit}
  in_svc {print}
' "${COMPOSE}")"
[[ -n "${ai_egress_block}" ]] || fail "ai-egress service missing"
ok "ai-egress service present"
echo "${ai_egress_block}" | grep -q 'ghcr.io/xtls/xray-core' || fail "ai-egress must use ghcr.io/xtls/xray-core"
ok "ai-egress image is official xray-core"
echo "${ai_egress_block}" | grep -qE '^\s+init:\s*true\s*$' || fail "ai-egress must set init: true"
ok "ai-egress init: true"
echo "${ai_egress_block}" | grep -q 'ai-egress-config.json' || fail "ai-egress must mount /etc/aivion/secrets/ai-egress-config.json"
ok "ai-egress mounts host secret config"
echo "${ai_egress_block}" | grep -qE '/etc/xray/config.json' || fail "ai-egress must mount config at /etc/xray/config.json"
ok "ai-egress mounts /etc/xray/config.json"
if echo "${ai_egress_block}" | grep -qE '^\s+ports:'; then
  fail "ai-egress must not publish ports"
fi
ok "ai-egress has no ports: section"
hc_ai="$(
  awk '
    $0 ~ /^  ai-egress:/ {in_svc=1; next}
    in_svc && $0 ~ /^  [a-z]/ {in_svc=0}
    in_svc && $0 ~ /healthcheck:/ {in_hc=1; next}
    in_svc && in_hc && $0 ~ /^\s+test:/ {print; exit}
  ' "${COMPOSE}"
)"
[[ -n "${hc_ai}" ]] || fail "ai-egress healthcheck test missing"
echo "${hc_ai}" | grep -q 'xray' || fail "ai-egress healthcheck must invoke xray"
if echo "${hc_ai}" | grep -Ei 'openai|api\.openai|https://'; then
  fail "ai-egress healthcheck must not call OpenAI or external HTTPS"
fi
ok "ai-egress healthcheck is local xray -test"
# AI_HTTPS_PROXY only on backend services
api_proxy="$(awk '
  $0 ~ /^  backend-api:/ {in_svc=1; next}
  in_svc && $0 ~ /^  [a-z]/ {in_svc=0}
  in_svc && $0 ~ /AI_HTTPS_PROXY:/ {print; exit}
' "${COMPOSE}")"
echo "${api_proxy}" | grep -q 'http://ai-egress:10809' || fail "backend-api must set AI_HTTPS_PROXY=http://ai-egress:10809"
ok "backend-api AI_HTTPS_PROXY -> ai-egress:10809"
worker_proxy="$(awk '
  $0 ~ /^  backend-worker:/ {in_svc=1; next}
  in_svc && $0 ~ /^  [a-z]/ {in_svc=0}
  in_svc && $0 ~ /AI_HTTPS_PROXY:/ {print; exit}
' "${COMPOSE}")"
echo "${worker_proxy}" | grep -q 'http://ai-egress:10809' || fail "backend-worker must set AI_HTTPS_PROXY=http://ai-egress:10809"
ok "backend-worker AI_HTTPS_PROXY -> ai-egress:10809"
frontend_block="$(awk '
  $0 ~ /^  frontend:/ {in_svc=1; print; next}
  in_svc && $0 ~ /^  [a-z]/ {exit}
  in_svc {print}
' "${COMPOSE}")"
if echo "${frontend_block}" | grep -q 'AI_HTTPS_PROXY'; then
  fail "frontend must not receive AI_HTTPS_PROXY"
fi
ok "frontend has no AI_HTTPS_PROXY"
if grep -E '^\s+HTTP_PROXY:|^\s+HTTPS_PROXY:' "${COMPOSE}"; then
  fail "HTTP_PROXY/HTTPS_PROXY must not appear in production compose"
fi
ok "no process-wide HTTP(S)_PROXY in compose"
if grep -E '^\s+-\s*"?10809:' "${COMPOSE}"; then
  fail "ai-egress port 10809 must not be published on the host"
fi
ok "ai-egress 10809 not published"
if grep -E '^\s+-\s*"?10808:' "${COMPOSE}"; then
  fail "ai-egress SOCKS port 10808 must not be published on the host"
fi
ok "ai-egress 10808 not published"

# Dedicated ai-egress network: only api/worker/ai-egress (not frontend/caddy).
grep -qE '^\s+aivion_ai_egress:' "${COMPOSE}" || fail "aivion_ai_egress network missing"
ok "aivion_ai_egress network defined"
ai_nets="$(awk '
  $0 ~ /^  ai-egress:/ {in_svc=1; next}
  in_svc && $0 ~ /^  [a-z]/ {in_svc=0}
  in_svc && $0 ~ /networks:/ {in_net=1; next}
  in_svc && in_net && $0 ~ /^\s+-\s+/ {print; next}
  in_svc && in_net && $0 !~ /^\s+-/ && $0 !~ /^\s*$/ {exit}
' "${COMPOSE}")"
echo "${ai_nets}" | grep -q 'aivion_ai_egress' || fail "ai-egress must join aivion_ai_egress"
if echo "${ai_nets}" | grep -q 'aivion_internal'; then
  fail "ai-egress must not join aivion_internal (keep proxy plane isolated)"
fi
ok "ai-egress only on aivion_ai_egress"
for svc in backend-api backend-worker; do
  svc_nets="$(awk -v svc="$svc" '
    $0 ~ "^  "svc":" {in_svc=1; next}
    in_svc && $0 ~ /^  [a-z]/ {in_svc=0}
    in_svc && $0 ~ /networks:/ {in_net=1; next}
    in_svc && in_net && $0 ~ /^\s+-\s+/ {print; next}
    in_svc && in_net && $0 !~ /^\s+-/ && $0 !~ /^\s*$/ {exit}
  ' "${COMPOSE}")"
  echo "${svc_nets}" | grep -q 'aivion_ai_egress' || fail "${svc} must join aivion_ai_egress"
  echo "${svc_nets}" | grep -q 'aivion_internal' || fail "${svc} must also join aivion_internal"
  ok "${svc} joins aivion_internal + aivion_ai_egress"
done
for svc in frontend caddy postgres redis minio; do
  svc_nets="$(awk -v svc="$svc" '
    $0 ~ "^  "svc":" {in_svc=1; next}
    in_svc && $0 ~ /^  [a-z]/ {in_svc=0}
    in_svc && $0 ~ /networks:/ {in_net=1; next}
    in_svc && in_net && $0 ~ /^\s+-\s+/ {print; next}
    in_svc && in_net && $0 !~ /^\s+-/ && $0 !~ /^\s*$/ {exit}
  ' "${COMPOSE}")"
  if echo "${svc_nets}" | grep -q 'aivion_ai_egress'; then
    fail "${svc} must not join aivion_ai_egress"
  fi
  ok "${svc} not on aivion_ai_egress"
done

echo "compose-validate: all checks passed"
