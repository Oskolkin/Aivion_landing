#Requires -Version 5.1
$ErrorActionPreference = 'Stop'
$Root = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$Config = Join-Path $Root '.gitleaks.toml'
$Image = 'zricethezav/gitleaks:v8.24.3'

# Scan full git history (not untracked local .env*). Always redact matches in output.
$gitleaks = Get-Command gitleaks -ErrorAction SilentlyContinue
if ($gitleaks) {
  & gitleaks detect --source=$Root --config=$Config --log-opts='--all' --verbose --redact
  exit $LASTEXITCODE
}

$docker = Get-Command docker -ErrorAction SilentlyContinue
if ($docker) {
  docker run --rm -v "${Root}:/repo" $Image detect `
    --source=/repo `
    --config=/repo/.gitleaks.toml `
    --log-opts='--all' `
    --verbose `
    --redact
  exit $LASTEXITCODE
}

Write-Error 'gitleaks binary or docker is required'
exit 1
