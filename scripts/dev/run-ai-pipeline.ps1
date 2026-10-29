# Local AI pipeline launcher for Windows PowerShell.

[CmdletBinding(DefaultParameterSetName = 'FullRun')]
param(
    [Parameter(Mandatory = $true, ParameterSetName = 'FullRun')]
    [int]$SellerID,

    [Parameter(Mandatory = $true, ParameterSetName = 'FullRun')]
    [ValidatePattern('^\d{4}-\d{2}-\d{2}$')]
    [string]$MetricDate,

    [Parameter(Mandatory = $true, ParameterSetName = 'Resume')]
    [int]$ResumeRunID,

    [Parameter(ParameterSetName = 'Resume')]
    [ValidateSet('shared_chunk_validation', 'shared_chunk_analysis', 'global_context', 'actions')]
    [string]$RestartFromStage,

    [switch]$Dashboard,
    [switch]$Actions,
    [switch]$AIEnabled,
    [switch]$Force,
    [switch]$DebugArtifacts,
    [switch]$SkipReadiness,
    [switch]$StartInfrastructure,
    [switch]$OverrideEnvironment,
    [switch]$ExportInspector,
    [switch]$OpenInspector,
    [switch]$MarkStaleRunFailed,
    [switch]$Confirm
)

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot '_common.ps1')
. (Join-Path $PSScriptRoot 'load-env.ps1')
. (Join-Path $PSScriptRoot 'docker-discovery.ps1')

function Resolve-SwitchDefault {
    param(
        [string]$Name,
        [bool]$DefaultValue = $true
    )
    if ($PSBoundParameters.ContainsKey($Name)) {
        return [bool](Get-Variable -Name $Name -ValueOnly)
    }
    return $DefaultValue
}

if ($PSCmdlet.ParameterSetName -eq 'Resume' -and $Force) {
    throw 'Resume/stage-restart cannot be combined with -Force.'
}
if ($MarkStaleRunFailed -and ($PSCmdlet.ParameterSetName -ne 'FullRun')) {
    throw '-MarkStaleRunFailed requires -SellerID and -MetricDate (full run parameter set).'
}

$dashboardEnabled = Resolve-SwitchDefault -Name 'Dashboard' -DefaultValue $true
$actionsEnabled = Resolve-SwitchDefault -Name 'Actions' -DefaultValue $true
$aiEnabledValue = Resolve-SwitchDefault -Name 'AIEnabled' -DefaultValue $true
$debugArtifactsEnabled = Resolve-SwitchDefault -Name 'DebugArtifacts' -DefaultValue $true
$forceEnabled = Resolve-SwitchDefault -Name 'Force' -DefaultValue $false

$originalLocation = Get-Location
$resultJsonFile = $null
try {
    $repoRoot = Get-RepositoryRoot -StartPath $PSScriptRoot
    Import-LocalDevEnvironment -RepositoryRoot $repoRoot -OverrideEnvironment:$OverrideEnvironment -Quiet
    Resolve-DockerInfrastructure -RepositoryRoot $repoRoot -StartInfrastructure:$StartInfrastructure
    Set-LocalDevDefaultEnvironment -RepositoryRoot $repoRoot

    if ($aiEnabledValue) {
        if ([string]::IsNullOrWhiteSpace($env:AI_PROVIDER_MODE)) {
            [Environment]::SetEnvironmentVariable('AI_PROVIDER_MODE', 'real', [EnvironmentVariableTarget]::Process)
        }
        if ([string]::IsNullOrWhiteSpace($env:OPENAI_API_KEY)) {
            throw 'OPENAI_API_KEY is required for AI-enabled runs. Add it to .env.local once.'
        }
    }
    elseif ([string]::IsNullOrWhiteSpace($env:AI_PROVIDER_MODE)) {
        [Environment]::SetEnvironmentVariable('AI_PROVIDER_MODE', 'fake', [EnvironmentVariableTarget]::Process)
    }

    if ($MarkStaleRunFailed) {
        if (Test-GoDevtoolsPipelineProcessRunning) {
            throw 'An active run-ai-pipeline process is still running. Stop it before marking a stale run failed.'
        }
        if (-not $Confirm) {
            throw 'Refusing to mark stale run failed without -Confirm. Re-run with -MarkStaleRunFailed -Confirm.'
        }
        $markExit = Invoke-GoDevtools -RepositoryRoot $repoRoot -Arguments @(
            'run', './cmd/devtools', 'mark-stale-ai-pipeline-run',
            "--seller-id=$SellerID",
            "--metric-date=$MetricDate",
            '--confirm=true'
        ) -AllowFailure
        if ($markExit -ne 0) {
            exit $markExit
        }
    }

    if ($PSCmdlet.ParameterSetName -eq 'FullRun') {
        $runningArgs = @(
            'run', './cmd/devtools', 'check-ai-pipeline-running',
            "--seller-id=$SellerID",
            "--metric-date=$MetricDate",
            '--json'
        )
        $runningResult = Invoke-GoDevtoolsCore -RepositoryRoot $repoRoot -Arguments $runningArgs
        $runningJson = ($runningResult.StdOut -join "`n")
        $runningExit = $runningResult.ExitCode
        if ($runningExit -eq 2 -and -not [string]::IsNullOrWhiteSpace($runningJson)) {
            $running = $runningJson | ConvertFrom-Json
            Write-Host 'A pipeline run is already marked as running:'
            Write-Host ''
            Write-Host "run_id: $($running.run_id)"
            Write-Host "seller_id: $($running.seller_id)"
            Write-Host "metric_date: $($running.metric_date)"
            Write-Host "started_at: $($running.started_at)"
            Write-Host ''
            Write-Host 'No new run was started.'
            exit 2
        }
    }

    if (-not $SkipReadiness) {
        $readinessArgs = @('run', './cmd/devtools', 'check-ai-run-readiness')
        if ($PSCmdlet.ParameterSetName -eq 'FullRun') {
            $readinessArgs += @("--seller-id=$SellerID", "--metric-date=$MetricDate")
        }
        $readinessResult = Invoke-GoDevtoolsCore -RepositoryRoot $repoRoot -Arguments $readinessArgs
        foreach ($line in $readinessResult.StdOut) { Write-Host $line }
        foreach ($line in $readinessResult.StdErr) { Write-Host $line }
        if ($readinessResult.ExitCode -ne 0) {
            Write-Host ''
            Write-Host 'AI pipeline was not started because readiness failed.'
            Write-Host 'No OpenAI requests were sent.'
            exit $readinessResult.ExitCode
        }
    }
    else {
        Write-Warning 'SKIP READINESS: -SkipReadiness is enabled for technical debugging only.'
    }

    $pipelineArgs = @(
        'run', './cmd/devtools', 'run-ai-pipeline',
        "--dashboard=$(ConvertTo-BooleanString $dashboardEnabled)",
        "--actions=$(ConvertTo-BooleanString $actionsEnabled)",
        "--ai-enabled=$(ConvertTo-BooleanString $aiEnabledValue)",
        "--force=$(ConvertTo-BooleanString $forceEnabled)",
        "--debug-artifacts=$(ConvertTo-BooleanString $debugArtifactsEnabled)"
    )

    if ($PSCmdlet.ParameterSetName -eq 'FullRun') {
        $pipelineArgs += @("--seller-id=$SellerID", "--metric-date=$MetricDate")
    }
    else {
        $pipelineArgs += @("--resume-run-id=$ResumeRunID")
        if (-not [string]::IsNullOrWhiteSpace($RestartFromStage)) {
            $pipelineArgs += "--restart-from-stage=$RestartFromStage"
        }
    }
    $resultJsonFile = Join-Path $env:TEMP ("aivion-pipeline-result-" + [Guid]::NewGuid().ToString('N') + ".json")
    $pipelineArgs += "--result-json=$resultJsonFile"

    $pipelineResult = Invoke-GoDevtoolsCore -RepositoryRoot $repoRoot -Arguments $pipelineArgs
    $pipelineExit = $pipelineResult.ExitCode

    $result = $null
    if ($pipelineExit -eq 0 -and -not [string]::IsNullOrWhiteSpace($resultJsonFile) -and (Test-Path -LiteralPath $resultJsonFile)) {
        $jsonText = (Get-Content -LiteralPath $resultJsonFile -Raw).Trim()
        try {
            $result = $jsonText | ConvertFrom-Json
        }
        catch {
            $result = $null
        }
    }

    if ($pipelineExit -ne 0) {
        Write-Host ''
        Write-Host "Pipeline failed with exit code: $pipelineExit"
        exit $pipelineExit
    }

    if ($null -ne $result) {
        Write-Host ''
        Write-Host 'Pipeline completed.'
        Write-Host ''
        Write-Host "Run ID: $($result.pipeline_run_id)"
        Write-Host "Status: $($result.status)"
        $published = if ($null -ne $result.published) { [string]$result.published } else { 'false' }
        Write-Host "Published: $published"
        Write-Host ''
        Write-Host 'Export Inspector:'
        Write-Host ".\scripts\dev\export-ai-run.ps1 -RunID $($result.pipeline_run_id)"

        if ($ExportInspector) {
            $exportArgs = @(
                '-RunID', [int]$result.pipeline_run_id,
                '-DiagnosticSummary'
            )
            if ($OpenInspector) {
                $exportArgs += '-Open'
            }
            & (Join-Path $PSScriptRoot 'export-ai-run.ps1') @exportArgs
            exit $LASTEXITCODE
        }
    }

    exit 0
}
finally {
    if (-not [string]::IsNullOrWhiteSpace($resultJsonFile) -and (Test-Path -LiteralPath $resultJsonFile)) {
        Remove-Item -LiteralPath $resultJsonFile -Force -ErrorAction SilentlyContinue
    }
    Set-Location $originalLocation
}
