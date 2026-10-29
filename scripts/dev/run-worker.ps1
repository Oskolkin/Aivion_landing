# Start the background worker with the same local host-dev env as run-api.ps1.
# Usage:
#   powershell -ExecutionPolicy Bypass -File scripts/dev/run-worker.ps1
#
# Official host-dev ports:
#   frontend: 3000
#   backend API: 18081
#   PostgreSQL: 55432
#   Redis: 6379
#   MinIO: 19000/19001

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent
. (Join-Path $PSScriptRoot "load-env.ps1")

Import-LocalDevEnvironment -RepositoryRoot $repoRoot -Quiet:$false
Set-LocalDevDefaultEnvironment -RepositoryRoot $repoRoot

$env:APP_ENV = if ($env:APP_ENV) { $env:APP_ENV } else { "local" }
$env:BACKEND_PORT = if ($env:BACKEND_PORT) { $env:BACKEND_PORT } else { "18081" }
# Prefer 19091 for host-dev to avoid clashing with docker/prometheus on 9091.
$env:WORKER_METRICS_PORT = if ($env:WORKER_METRICS_PORT) { $env:WORKER_METRICS_PORT } else { "19091" }

# Keep Unified flags aligned with run-api.ps1 so worker publish path matches API reads.
$env:UNIFIED_ANALYSIS_ROLLOUT_ENABLED = if ($env:UNIFIED_ANALYSIS_ROLLOUT_ENABLED) { $env:UNIFIED_ANALYSIS_ROLLOUT_ENABLED } else { "true" }
$env:UNIFIED_ANALYSIS_PROVIDER_ENABLED = if ($env:UNIFIED_ANALYSIS_PROVIDER_ENABLED) { $env:UNIFIED_ANALYSIS_PROVIDER_ENABLED } else { "true" }
$env:UNIFIED_ANALYSIS_PROMPT_PROFILE = if ($env:UNIFIED_ANALYSIS_PROMPT_PROFILE) { $env:UNIFIED_ANALYSIS_PROMPT_PROFILE } else { "analysis" }
if (-not $env:UNIFIED_ANALYSIS_AUTO_RUN_ENABLED) { $env:UNIFIED_ANALYSIS_AUTO_RUN_ENABLED = "false" }
if (-not $env:UNIFIED_ANALYSIS_AUTO_PUBLISH_ENABLED) { $env:UNIFIED_ANALYSIS_AUTO_PUBLISH_ENABLED = "false" }
if (-not $env:UNIFIED_ANALYSIS_LEGACY_FALLBACK_ENABLED) { $env:UNIFIED_ANALYSIS_LEGACY_FALLBACK_ENABLED = "true" }

function Assert-Configured([string]$Name) {
  $v = [Environment]::GetEnvironmentVariable($Name, "Process")
  return -not [string]::IsNullOrWhiteSpace($v)
}

$enc = $env:ENCRYPTION_KEY
if ([string]::IsNullOrWhiteSpace($enc) -or $enc.Length -ne 32) {
  Write-Error "ENCRYPTION_KEY must be exactly 32 bytes (configured length=$($enc.Length)). Load .env.local or use scripts/dev/run-worker.ps1."
  exit 1
}

if (-not (Assert-Configured "DATABASE_URL")) {
  Write-Error "DATABASE_URL is required for the worker."
  exit 1
}
if (-not (Assert-Configured "REDIS_ADDR")) {
  Write-Error "REDIS_ADDR is required for the worker."
  exit 1
}

Write-Host ""
Write-Host "=== Worker configuration summary (secrets redacted) ==="
Write-Host "Database configured: $(Assert-Configured 'DATABASE_URL')"
Write-Host "Redis configured: $(Assert-Configured 'REDIS_ADDR')"
Write-Host "MinIO configured: $(Assert-Configured 'S3_ENDPOINT')"
Write-Host "OpenAI configured: $(Assert-Configured 'OPENAI_API_KEY')"
Write-Host "Encryption key configured: $($enc.Length -eq 32)"
Write-Host "Unified rollout enabled: $($env:UNIFIED_ANALYSIS_ROLLOUT_ENABLED)"
Write-Host "Unified provider enabled: $($env:UNIFIED_ANALYSIS_PROVIDER_ENABLED)"
Write-Host "Unified prompt profile: $($env:UNIFIED_ANALYSIS_PROMPT_PROFILE)"
Write-Host "Unified auto-run enabled: $($env:UNIFIED_ANALYSIS_AUTO_RUN_ENABLED)"
Write-Host "Unified auto-publish enabled: $($env:UNIFIED_ANALYSIS_AUTO_PUBLISH_ENABLED)"
Write-Host "Worker metrics port: $($env:WORKER_METRICS_PORT)"
Write-Host "======================================================="
Write-Host ""
Write-Host "Starting worker (do not use bare go run ./cmd/worker without env)"

Set-Location (Join-Path $repoRoot "backend")
go run ./cmd/worker
