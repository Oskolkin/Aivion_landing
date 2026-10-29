#!/usr/bin/env bash
# Align OpenAI secret host permissions with non-root container UID 10001 (appuser).
# Target: root:aivion 0640 (not world-readable 0644). Never prints secret contents.
set -euo pipefail
GROUP=aivion
GID=10001
SECRET=/etc/aivion/secrets/openai_api_key

if ! getent group "$GROUP" >/dev/null; then
  sudo groupadd --gid "$GID" "$GROUP"
fi
# Ensure deploy can administer but not world-read
sudo chown root:"$GROUP" "$SECRET"
sudo chmod 640 "$SECRET"
sudo stat -c '%U:%G %a %n' "$SECRET"
echo openai_secret_perms_ok
