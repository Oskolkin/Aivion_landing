#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CONFIG="${ROOT}/.gitleaks.toml"
IMAGE="zricethezav/gitleaks:v8.24.3"

# Scan full git history (not untracked local .env*). Always redact matches in output.
if command -v gitleaks >/dev/null 2>&1; then
  exec gitleaks detect --source="${ROOT}" --config="${CONFIG}" --log-opts="--all" --verbose --redact
fi

if command -v docker >/dev/null 2>&1; then
  exec docker run --rm -v "${ROOT}:/repo" "${IMAGE}" detect \
    --source=/repo \
    --config=/repo/.gitleaks.toml \
    --log-opts="--all" \
    --verbose \
    --redact
fi

echo "gitleaks binary or docker is required" >&2
exit 1
