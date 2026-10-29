# Alias: start API via the canonical launcher.
# Usage: powershell -ExecutionPolicy Bypass -File scripts/dev/start-backend.ps1

$ErrorActionPreference = "Stop"
& (Join-Path $PSScriptRoot "run-api.ps1")
