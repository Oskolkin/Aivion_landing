# Root wrapper for scripts/dev/run-ai-pipeline.ps1

$ErrorActionPreference = 'Stop'
$launcher = Join-Path $PSScriptRoot 'scripts/dev/run-ai-pipeline.ps1'
if (-not (Test-Path -LiteralPath $launcher)) {
    throw "Launcher not found: $launcher"
}
& $launcher @args
exit $LASTEXITCODE
