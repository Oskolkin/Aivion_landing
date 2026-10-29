# Start API on port 18081 with project env (local Windows standard).
# Usage:
#   powershell -ExecutionPolicy Bypass -File scripts/dev/run-api.ps1
#   powershell -ExecutionPolicy Bypass -File scripts/dev/run-api.ps1 -FullRestart
#   powershell -ExecutionPolicy Bypass -File scripts/dev/start-backend.ps1   # alias
#
# By default only the backend API ports are stopped (8081/18081).
# Pass -FullRestart to also stop the frontend on 3000/3001/3002.

param(
  [switch]$FullRestart
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent
. (Join-Path $PSScriptRoot "load-env.ps1")

function Stop-ListenersOnPorts([int[]]$Ports) {
  foreach ($port in $Ports) {
    $pids = netstat -ano | ForEach-Object {
      if ($_ -match ":$port\s+\S+\s+\S+\s+LISTENING\s+(\d+)\s*$") { [int]$Matches[1] }
    } | Sort-Object -Unique
    foreach ($procId in $pids) {
      if ($procId -le 4) { continue }
      $proc = Get-Process -Id $procId -ErrorAction SilentlyContinue
      if (-not $proc) { continue }
      Write-Host "Stopping PID $procId ($($proc.ProcessName)) on port $port"
      Stop-Process -Id $procId -Force -ErrorAction SilentlyContinue
    }
  }
}

$backendPorts = @(8081, 18081)
$frontendPorts = @(3000, 3001, 3002)
if ($FullRestart) {
  Write-Host "FullRestart: stopping frontend + backend listeners..."
  Stop-ListenersOnPorts ($frontendPorts + $backendPorts)
} else {
  Write-Host "Stopping existing listeners on backend ports (8081/18081)..."
  Stop-ListenersOnPorts $backendPorts
}
Start-Sleep -Seconds 1

Import-LocalDevEnvironment -RepositoryRoot $repoRoot -Quiet:$false
Set-LocalDevDefaultEnvironment -RepositoryRoot $repoRoot

# Local API listen port (must match frontend BACKEND_PROXY_URL / next.config rewrites).
$env:APP_ENV = if ($env:APP_ENV) { $env:APP_ENV } else { "local" }
$env:BACKEND_PORT = "18081"
$env:WORKER_METRICS_PORT = if ($env:WORKER_METRICS_PORT) { $env:WORKER_METRICS_PORT } else { "19091" }

# Unified Analysis: enable read path for sellers in analysis_mode=unified.
# Auto-run / auto-publish stay off unless already set in .env.local.
# Force rollout=true for local launcher so Unified current publications are readable
# (auto-run/auto-publish remain opt-in and default false).
$env:UNIFIED_ANALYSIS_ROLLOUT_ENABLED = "true"
if (-not $env:UNIFIED_ANALYSIS_AUTO_RUN_ENABLED) { $env:UNIFIED_ANALYSIS_AUTO_RUN_ENABLED = "false" }
if (-not $env:UNIFIED_ANALYSIS_AUTO_PUBLISH_ENABLED) { $env:UNIFIED_ANALYSIS_AUTO_PUBLISH_ENABLED = "false" }
if (-not $env:UNIFIED_ANALYSIS_LEGACY_FALLBACK_ENABLED) { $env:UNIFIED_ANALYSIS_LEGACY_FALLBACK_ENABLED = "true" }

function Assert-Configured([string]$Name) {
  $v = [Environment]::GetEnvironmentVariable($Name, "Process")
  return -not [string]::IsNullOrWhiteSpace($v)
}

$enc = $env:ENCRYPTION_KEY
if ([string]::IsNullOrWhiteSpace($enc) -or $enc.Length -ne 32) {
  Write-Error "ENCRYPTION_KEY must be exactly 32 bytes (configured length=$($enc.Length)). Load .env.local or use scripts/dev/run-api.ps1 defaults."
  exit 1
}

Write-Host ""
Write-Host "=== API configuration summary (secrets redacted) ==="
Write-Host "API port: $($env:BACKEND_PORT)"
Write-Host "Database configured: $(Assert-Configured 'DATABASE_URL')"
Write-Host "Redis configured: $(Assert-Configured 'REDIS_ADDR')"
Write-Host "MinIO configured: $(Assert-Configured 'S3_ENDPOINT')"
Write-Host "Encryption key configured: $($enc.Length -eq 32)"
Write-Host "Unified rollout enabled: $($env:UNIFIED_ANALYSIS_ROLLOUT_ENABLED)"
Write-Host "Unified auto-run enabled: $($env:UNIFIED_ANALYSIS_AUTO_RUN_ENABLED)"
Write-Host "Unified auto-publish enabled: $($env:UNIFIED_ANALYSIS_AUTO_PUBLISH_ENABLED)"
Write-Host "Unified fallback enabled: $($env:UNIFIED_ANALYSIS_LEGACY_FALLBACK_ENABLED)"
Write-Host "===================================================="
Write-Host ""
Write-Host "Starting backend on http://localhost:$($env:BACKEND_PORT)"
Write-Host "Frontend should use BACKEND_PROXY_URL=http://localhost:$($env:BACKEND_PORT)"
Write-Host "Do not run bare: go run ./cmd/api  (env/encryption must be loaded)"

Set-Location (Join-Path $repoRoot "backend")
go run ./cmd/api
