# Export AI run Inspector artifacts.

[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [int]$RunID,

    [string]$OutputDir,

    [switch]$DiagnosticSummary,
    [switch]$Open
)

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot '_common.ps1')
. (Join-Path $PSScriptRoot 'load-env.ps1')
. (Join-Path $PSScriptRoot 'docker-discovery.ps1')

$repoRoot = Get-RepositoryRoot -StartPath $PSScriptRoot
Import-LocalDevEnvironment -RepositoryRoot $repoRoot -Quiet
Resolve-DockerInfrastructure -RepositoryRoot $repoRoot
Set-LocalDevDefaultEnvironment -RepositoryRoot $repoRoot

if ([string]::IsNullOrWhiteSpace($OutputDir)) {
    $OutputDir = "./debug/run-$RunID"
}

$arguments = @(
    'run', './cmd/devtools', 'export-ai-run-inspector',
    "--run-id=$RunID",
    "--output=$OutputDir"
)
if ($DiagnosticSummary) {
    $arguments += '--diagnostic-summary'
}
if ($Open) {
    $arguments += '--open'
}

$exitCode = Invoke-GoDevtools -RepositoryRoot $repoRoot -Arguments $arguments
exit $exitCode
