#!/usr/bin/env bash
# High-level server bootstrap for Aivion staging/production.
# Idempotent where practical. Review before running on a live host.
# See also: docs/deployment/server-bootstrap.md
set -euo pipefail

AIVION_ROOT="${AIVION_ROOT:-/opt/aivion}"
AIVION_SECRETS="${AIVION_SECRETS:-/etc/aivion/secrets}"
DEPLOY_USER="${DEPLOY_USER:-aivion}"
SWAP_GB="${SWAP_GB:-2}"

echo "==> Aivion server bootstrap"
echo "    AIVION_ROOT=${AIVION_ROOT}"
echo "    AIVION_SECRETS=${AIVION_SECRETS}"
echo "    DEPLOY_USER=${DEPLOY_USER}"

if [[ "$(id -u)" -ne 0 ]]; then
  echo "run as root (or with sudo)" >&2
  exit 1
fi

echo "==> packages (Debian/Ubuntu)"
export DEBIAN_FRONTEND=noninteractive
apt-get update -y
apt-get install -y --no-install-recommends \
  ca-certificates curl gnupg lsb-release jq gzip ufw fail2ban \
  apache2-utils openssl

if ! command -v docker >/dev/null 2>&1; then
  echo "==> installing Docker Engine"
  install -m 0755 -d /etc/apt/keyrings
  if [[ ! -f /etc/apt/keyrings/docker.asc ]]; then
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
    chmod a+r /etc/apt/keyrings/docker.asc
  fi
  # Adjust for the host distro if needed; see Docker docs for Debian.
  . /etc/os-release
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/${ID} ${VERSION_CODENAME} stable" \
    >/etc/apt/sources.list.d/docker.list
  apt-get update -y
  apt-get install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
  systemctl enable --now docker
else
  echo "==> Docker already installed: $(docker --version)"
fi

echo "==> deploy user"
if ! id -u "${DEPLOY_USER}" >/dev/null 2>&1; then
  useradd --system --create-home --home-dir "/home/${DEPLOY_USER}" --shell /bin/bash "${DEPLOY_USER}"
fi
usermod -aG docker "${DEPLOY_USER}"

echo "==> directories"
mkdir -p \
  "${AIVION_ROOT}/repo" \
  "${AIVION_ROOT}/shared" \
  "${AIVION_ROOT}/releases" \
  "${AIVION_ROOT}/backups/postgres" \
  "${AIVION_ROOT}/backups/minio" \
  "${AIVION_ROOT}/backups/config" \
  "${AIVION_ROOT}/backups/evidence" \
  "${AIVION_SECRETS}"
chown -R "${DEPLOY_USER}:${DEPLOY_USER}" "${AIVION_ROOT}"
chown root:"${DEPLOY_USER}" "${AIVION_SECRETS}"
chmod 0750 "${AIVION_SECRETS}"
chmod 0750 "${AIVION_ROOT}/shared" "${AIVION_ROOT}/backups"

echo "==> secret placeholders (empty files; fill manually, chmod 0640)"
for f in \
  openai_api_key \
  postgres_password \
  redis_password \
  minio_root_user \
  minio_root_password \
  jwt_secret \
  encryption_key \
  metrics_auth_token \
  database_url \
  staging_basic_auth \
  admin_password
do
  path="${AIVION_SECRETS}/${f}"
  if [[ ! -e "${path}" ]]; then
    install -m 0640 -o root -g "${DEPLOY_USER}" /dev/null "${path}"
    echo "    created empty ${path}"
  fi
done

echo "==> swap (${SWAP_GB}G) if missing"
if ! swapon --show | grep -q .; then
  if [[ ! -f /swapfile ]]; then
    fallocate -l "${SWAP_GB}G" /swapfile || dd if=/dev/zero of=/swapfile bs=1M count=$((SWAP_GB * 1024))
    chmod 600 /swapfile
    mkswap /swapfile
  fi
  swapon /swapfile || true
  if ! grep -q '^/swapfile' /etc/fstab; then
    echo '/swapfile none swap sw 0 0' >>/etc/fstab
  fi
else
  echo "    swap already active"
fi

echo "==> UFW notes (not auto-enforcing destructive rules)"
echo "    Recommended:"
echo "      ufw default deny incoming"
echo "      ufw default allow outgoing"
echo "      ufw allow OpenSSH"
echo "      ufw allow 80/tcp"
echo "      ufw allow 443/tcp"
echo "      ufw enable"
echo "    Do NOT publish postgres/redis/minio/backend ports on the host."

echo "==> next steps (manual)"
cat <<'EOF'
  1. Clone or sync the repo into /opt/aivion/repo (deploy user).
  2. Fill /etc/aivion/secrets/* with real values (openssl rand -base64 48, etc.).
  3. Generate staging basic_auth snippet:
       HASH=$(caddy hash-password --plaintext '...')
       printf 'basic_auth {\n\tstaging_user %s\n}\n' "$HASH" > /etc/aivion/secrets/staging_basic_auth
  4. Build /opt/aivion/shared/runtime.env from deploy/env/runtime.env.example
     (include DATABASE_URL built from postgres password; never commit it).
  5. Also write the same DATABASE_URL into /etc/aivion/secrets/database_url.
  6. Log in to GHCR: echo $CR_PAT | docker login ghcr.io -u USER --password-stdin
  7. export ACME_EMAIL=ops@example.com
  8. Run: /opt/aivion/repo/deploy/scripts/deploy.sh <IMAGE_SHA>
  9. Run: create-admin.sh --email oskolkinm@yandex.ru
 10. Capture evidence under /opt/aivion/backups/evidence/ (see docs/audits/stage-2-deployment/).
EOF

echo "==> bootstrap finished"
