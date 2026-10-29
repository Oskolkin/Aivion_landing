# Start Next.js dev server (NOT next start). Clears .next by default.
# Usage: powershell -ExecutionPolicy Bypass -File scripts/dev/start-frontend.ps1
#        powershell -ExecutionPolicy Bypass -File scripts/dev/start-frontend.ps1 -NoClean

param([switch]$NoClean)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent
$frontend = Join-Path $repoRoot "frontend"
Set-Location $frontend

$envExample = Join-Path $frontend ".env.local.example"
$envLocal = Join-Path $frontend ".env.local"
if (-not (Test-Path $envLocal) -and (Test-Path $envExample)) {
  Copy-Item $envExample $envLocal
  Write-Host "Created .env.local from .env.local.example"
}

if (-not $NoClean -and (Test-Path ".next")) {
  Write-Host "Removing stale .next cache..."
  Remove-Item -Recurse -Force .next
}

Write-Host "Starting Next.js dev on http://localhost:3000"
Write-Host "API base URL must be http://localhost:18081 — check http://localhost:3000/dev/system"
Write-Host "Do NOT use 'npm start' during UI work — it serves an old production bundle."
npm run dev
