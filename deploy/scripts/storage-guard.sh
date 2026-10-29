#!/usr/bin/env bash
# Production Docker storage guard. This script never prunes volumes or touches
# containerd paths directly; Docker is the only deletion mechanism used here.
set -euo pipefail

MIN_FREE_DISK_GB="${MIN_FREE_DISK_GB:-15}"

usage() {
  cat <<'EOF'
Usage:
  storage-guard.sh preflight
  storage-guard.sh cleanup --dry-run|--apply \
    --frontend-rollback <image-ref> \
    --backend-rollback <image-ref> \
    --worker-rollback <image-ref>

`cleanup` protects every image used by a running container plus the three
explicit rollback images. It only considers ghcr.io/oskolkin/aivion-{frontend,
backend,worker} images. It never prunes Docker volumes.
EOF
}

free_root_gb() {
  df -Pk / | awk 'NR == 2 { print int($4 / 1024 / 1024) }'
}

print_disk_summary() {
  echo "Disk free on /: $(free_root_gb) GB (minimum: ${MIN_FREE_DISK_GB} GB)"
  docker system df
}

preflight() {
  local free_gb
  free_gb="$(free_root_gb)"
  echo "Disk before build: ${free_gb} GB free; required minimum: ${MIN_FREE_DISK_GB} GB"
  if (( free_gb < MIN_FREE_DISK_GB )); then
    echo "Insufficient disk space for production build: ${free_gb} GB free, ${MIN_FREE_DISK_GB} GB required" >&2
    exit 1
  fi
}

image_id_or_fail() {
  local ref="$1"
  local id
  id="$(docker image inspect --format '{{.Id}}' "$ref" 2>/dev/null || true)"
  if [[ -z "$id" ]]; then
    echo "rollback image is unavailable; cleanup stopped: $ref" >&2
    exit 1
  fi
  printf '%s\n' "$id"
}

cleanup() {
  local mode=""
  local frontend_rollback=""
  local backend_rollback=""
  local worker_rollback=""
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --dry-run|--apply) mode="${1#--}" ;;
      --frontend-rollback) frontend_rollback="${2:-}"; shift ;;
      --backend-rollback) backend_rollback="${2:-}"; shift ;;
      --worker-rollback) worker_rollback="${2:-}"; shift ;;
      *) echo "unknown cleanup option: $1" >&2; usage >&2; exit 2 ;;
    esac
    shift
  done
  [[ -n "$mode" && -n "$frontend_rollback" && -n "$backend_rollback" && -n "$worker_rollback" ]] || {
    echo "cleanup requires a mode and all explicit rollback image refs" >&2
    usage >&2
    exit 2
  }

  declare -A protected=()
  local id
  while IFS= read -r id; do
    [[ -n "$id" ]] && protected["$id"]=1
  done < <(docker ps -q | xargs -r docker inspect --format '{{.Image}}')
  protected["$(image_id_or_fail "$frontend_rollback")"]=1
  protected["$(image_id_or_fail "$backend_rollback")"]=1
  protected["$(image_id_or_fail "$worker_rollback")"]=1

  declare -A candidates=()
  while IFS='|' read -r repo image_id; do
    [[ "$repo" =~ ^ghcr\.io/oskolkin/aivion-(frontend|backend|worker)$ ]] || continue
    candidates["$image_id"]=1
  done < <(docker image ls --no-trunc --format '{{.Repository}}|{{.ID}}')

  local -a delete_ids=()
  echo "Protected current and rollback image IDs: ${#protected[@]}"
  for id in "${!candidates[@]}"; do
    if [[ -n "${protected[$id]:-}" ]]; then
      echo "KEEP   $id"
    else
      echo "DELETE $id"
      delete_ids+=("$id")
    fi
  done

  if [[ "$mode" == "dry-run" ]]; then
    echo "Dry run only: no image or cache was removed. Volumes are never considered."
    return 0
  fi

  for id in "${delete_ids[@]}"; do
    docker image rm "$id"
  done
  # No --all: remove only currently unused BuildKit cache.
  docker builder prune -f
  echo "Disk after cleanup:"
  print_disk_summary
}

command="${1:-}"
case "$command" in
  preflight) shift; [[ $# -eq 0 ]] || { usage >&2; exit 2; }; preflight ;;
  cleanup) shift; cleanup "$@" ;;
  *) usage >&2; exit 2 ;;
esac
