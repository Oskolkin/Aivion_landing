# Validate local environment for AI pipeline runs.

[CmdletBinding()]
param(
    [switch]$OverrideEnvironment,
    [switch]$StartInfrastructure
)

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot '_common.ps1')
. (Join-Path $PSScriptRoot 'load-env.ps1')
. (Join-Path $PSScriptRoot 'docker-discovery.ps1')

$repoRoot = Get-RepositoryRoot -StartPath $PSScriptRoot
Import-LocalDevEnvironment -RepositoryRoot $repoRoot -OverrideEnvironment:$OverrideEnvironment -Quiet
Resolve-DockerInfrastructure -RepositoryRoot $repoRoot -StartInfrastructure:$StartInfrastructure
Set-LocalDevDefaultEnvironment -RepositoryRoot $repoRoot

Write-Host 'AIVION LOCAL ENVIRONMENT'
Write-Host ''

$repoPass = $true
try {
    $null = Get-RepositoryRoot -StartPath $PSScriptRoot
    Write-Host 'Repository root: PASS'
}
catch {
    $repoPass = $false
    Write-Host 'Repository root: FAIL'
}

$envFile = Get-ResolvedEnvFileName -RepositoryRoot $repoRoot
if ($null -eq $envFile) {
    Write-Host 'Env file: (missing — copy .env.example to .env.local)'
}
else {
    Write-Host "Env file: $envFile"
}

$appEnv = [Environment]::GetEnvironmentVariable('APP_ENV', [EnvironmentVariableTarget]::Process)
if ([string]::IsNullOrWhiteSpace($appEnv)) {
    Write-Host 'APP_ENV: FAIL (unset)'
}
else {
    Write-Host "APP_ENV: PASS ($appEnv)"
}

Write-Host ''

$checkerResult = Invoke-GoLocalEnvChecker -RepositoryRoot $repoRoot
foreach ($line in $checkerResult.Output) {
    if ([string]$line -match '^Ready to run AI pipeline:') {
        continue
    }
    Write-GoCommandOutputLine -Line $line
}

$checkExitCode = $checkerResult.ExitCode

Write-Host ''
if ($checkExitCode -eq 0 -and $repoPass) {
    Write-Host 'Ready to run AI pipeline: YES'
    exit 0
}

Write-Host 'Ready to run AI pipeline: NO'
if (-not $repoPass) {
    exit 2
}
exit $checkExitCode
