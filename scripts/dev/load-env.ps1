# Load local environment variables into the current PowerShell process.
# Usage (dot-source): . "$PSScriptRoot/load-env.ps1"; Import-LocalDevEnvironment
# Usage (report):     .\scripts\dev\load-env.ps1

[CmdletBinding()]
param(
    [switch]$OverrideEnvironment,
    [switch]$Quiet
)

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot '_common.ps1')

function Parse-DotEnvLine {
    param([string]$Line)

    $trimmed = $Line.Trim()
    if ([string]::IsNullOrWhiteSpace($trimmed) -or $trimmed.StartsWith('#')) {
        return $null
    }
    $eq = $trimmed.IndexOf('=')
    if ($eq -le 0) {
        return $null
    }
    $name = $trimmed.Substring(0, $eq).Trim()
    $value = $trimmed.Substring($eq + 1).Trim()
    if ($value.Length -ge 2) {
        $first = $value[0]
        $last = $value[$value.Length - 1]
        if (($first -eq '"' -and $last -eq '"') -or ($first -eq "'" -and $last -eq "'")) {
            $value = $value.Substring(1, $value.Length - 2)
        }
    }
    return [PSCustomObject]@{ Name = $name; Value = $value }
}

function Import-DotEnvFile {
    param(
        [string]$Path,
        [switch]$OverrideEnvironment
    )

    if (-not (Test-Path -LiteralPath $Path)) {
        return
    }
    foreach ($line in Get-Content -LiteralPath $Path) {
        $entry = Parse-DotEnvLine -Line $line
        if ($null -eq $entry) {
            continue
        }
        $existing = [Environment]::GetEnvironmentVariable($entry.Name, [EnvironmentVariableTarget]::Process)
        if (-not $OverrideEnvironment -and -not [string]::IsNullOrWhiteSpace($existing)) {
            continue
        }
        [Environment]::SetEnvironmentVariable(
            $entry.Name,
            $entry.Value,
            [EnvironmentVariableTarget]::Process
        )
    }
}

function Import-LocalDevEnvironment {
    param(
        [string]$RepositoryRoot,
        [switch]$OverrideEnvironment,
        [switch]$Quiet
    )

    $files = @(
        (Join-Path $RepositoryRoot '.env'),
        (Join-Path $RepositoryRoot '.env.development.local'),
        (Join-Path $RepositoryRoot '.env.local')
    )
    foreach ($file in $files) {
        Import-DotEnvFile -Path $file -OverrideEnvironment:$OverrideEnvironment
    }

    if (-not $Quiet) {
        $loadedFile = Get-ResolvedEnvFileName -RepositoryRoot $RepositoryRoot
        if ($null -eq $loadedFile) {
            Write-Host 'Env file: (none found — create .env.local from .env.example)'
        }
        else {
            Write-Host "Env file: $loadedFile"
        }

        $keys = @(
            'APP_ENV', 'DATABASE_URL', 'REDIS_ADDR', 'S3_ENDPOINT', 'S3_ACCESS_KEY',
            'S3_SECRET_KEY', 'OPENAI_API_KEY', 'AI_PROXY_URL'
        )
        foreach ($key in $keys) {
            $value = [Environment]::GetEnvironmentVariable($key, [EnvironmentVariableTarget]::Process)
            $display = Get-SafeEnvironmentDisplayValue -Name $key -Value $value
            Write-Host "$key=$display"
        }
    }
}

function Set-LocalDevDefaultEnvironment {
    param([string]$RepositoryRoot)

    $defaults = @{
        APP_ENV = 'local'
        MIGRATIONS_PATH = './migrations'
        DATABASE_URL = 'postgres://postgres:postgres@127.0.0.1:55432/marketplace_ai?sslmode=disable'
        REDIS_ADDR = '127.0.0.1:6379'
        REDIS_DB = '0'
        S3_ENDPOINT = '127.0.0.1:19000'
        S3_ACCESS_KEY = 'minio'
        S3_SECRET_KEY = 'minio123'
        S3_BUCKET_RAW = 'raw-payloads'
        S3_BUCKET_EXPORTS = 'exports'
        S3_BUCKET_ARTIFACTS = 'artifacts'
        S3_USE_SSL = 'false'
        OPENAI_MODEL = 'gpt-4.1'
        OPENAI_TIMEOUT_SECONDS = '120'
        OPENAI_GLOBAL_CONTEXT_TIMEOUT_SECONDS = '300'
        OPENAI_GLOBAL_CONTEXT_MAX_OUTPUT_TOKENS = '16384'
        OPENAI_ACTIONS_TIMEOUT_SECONDS = '900'
        OPENAI_ACTIONS_MAX_OUTPUT_TOKENS = '16384'
        OPENAI_ACTION_OBJECT_DETAIL_TIMEOUT_SECONDS = '900'
        OPENAI_ACTION_OBJECT_DETAIL_MAX_OUTPUT_TOKENS = '8192'
        ACTIONS_OBJECT_DETAIL_BATCH_SIZE = '10'
        ACTIONS_OBJECT_DETAIL_MAX_CONCURRENCY = '2'
        OPENAI_MAX_RETRIES = '0'
        DASHBOARD_VALIDATION_MODE = 'pilot_structural'
        ALLOW_DEV_AI_PIPELINE = '1'
        SELLER_AI_PIPELINE_ENABLED = '1'
        ACTIONS_AI_FIRST = '1'
        LEGACY_AI_POST_SYNC_ENABLED = '0'
        JWT_SECRET = 'dev-jwt-secret-at-least-32-chars-long!!'
        ENCRYPTION_KEY = 'REPLACEME_LOCAL_ENC_KEY_32BYTES!'
        AUTH_COOKIE_NAME = 'session_token'
        AUTH_SESSION_TTL_HOURS = '168'
        WORKER_METRICS_PORT = '9091'
    }

    foreach ($entry in $defaults.GetEnumerator()) {
        $existing = [Environment]::GetEnvironmentVariable($entry.Key, [EnvironmentVariableTarget]::Process)
        if ([string]::IsNullOrWhiteSpace($existing)) {
            [Environment]::SetEnvironmentVariable($entry.Key, [string]$entry.Value, [EnvironmentVariableTarget]::Process)
        }
    }
}

if ($MyInvocation.InvocationName -ne '.') {
    $repoRoot = Get-RepositoryRoot -StartPath $PSScriptRoot
    Import-LocalDevEnvironment -RepositoryRoot $repoRoot -OverrideEnvironment:$OverrideEnvironment -Quiet:$Quiet
}


