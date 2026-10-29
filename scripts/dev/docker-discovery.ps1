# Discover local infrastructure endpoints from docker-compose when env vars are missing.

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot '_common.ps1')

function Get-DockerComposeCommand {
    $compose = Get-Command docker -ErrorAction SilentlyContinue
    if ($null -eq $compose) {
        return $null
    }
    return @{ Executable = 'docker'; BaseArgs = @('compose') }
}

function Test-DockerEngineAvailable {
    $cmd = Get-DockerComposeCommand
    if ($null -eq $cmd) {
        return $false
    }
    & $cmd.Executable @($cmd.BaseArgs + @('version')) 1>$null 2>$null
    return $LASTEXITCODE -eq 0
}

function Get-DockerComposeConfigJson {
    param([string]$RepositoryRoot)

    $cmd = Get-DockerComposeCommand
    if ($null -eq $cmd) {
        return $null
    }
    Push-Location $RepositoryRoot
    try {
        $raw = & $cmd.Executable @($cmd.BaseArgs + @('config', '--format', 'json')) 2>$null
        if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($raw)) {
            return $null
        }
        return ($raw | ConvertFrom-Json)
    }
    finally {
        Pop-Location
    }
}

function Get-PublishedHostPort {
    param(
        [object]$ServiceConfig,
        [int]$ContainerPort
    )

    if ($null -eq $ServiceConfig -or $null -eq $ServiceConfig.ports) {
        return $null
    }
    foreach ($entry in $ServiceConfig.ports) {
        $text = [string]$entry
        if ($text -match '^(\d+):(\d+)$') {
            if ([int]$Matches[2] -eq $ContainerPort) {
                return [int]$Matches[1]
            }
        }
        if ($text -match ":$ContainerPort$" -and $text -match '^(\d+):') {
            return [int]$Matches[1]
        }
    }
    return $null
}

function Get-ComposeServiceEnvironmentValue {
    param(
        [object]$ServiceConfig,
        [string]$Name
    )

    if ($null -eq $ServiceConfig -or $null -eq $ServiceConfig.environment) {
        return $null
    }
    $envObj = $ServiceConfig.environment
    if ($envObj -is [System.Collections.IDictionary]) {
        if ($envObj.Contains($Name)) {
            return [string]$envObj[$Name]
        }
    }
    else {
        $prop = $envObj.PSObject.Properties[$Name]
        if ($null -ne $prop) {
            return [string]$prop.Value
        }
    }
    return $null
}

function Resolve-DockerInfrastructure {
    param(
        [string]$RepositoryRoot,
        [switch]$StartInfrastructure
    )

    $requiredServices = @('postgres', 'redis', 'minio')
    $composeFile = Join-Path $RepositoryRoot 'docker-compose.yml'
    if (-not (Test-Path -LiteralPath $composeFile)) {
        return
    }
    if (-not (Test-DockerEngineAvailable)) {
        if ([string]::IsNullOrWhiteSpace($env:DATABASE_URL) -or [string]::IsNullOrWhiteSpace($env:REDIS_ADDR)) {
            throw @"
Local infrastructure is not running.

Run:
docker compose up -d postgres redis minio
"@
        }
        return
    }

    if ($StartInfrastructure) {
        Push-Location $RepositoryRoot
        try {
            & docker compose up -d @requiredServices
            if ($LASTEXITCODE -ne 0) {
                throw "docker compose up failed with exit code $LASTEXITCODE"
            }
        }
        finally {
            Pop-Location
        }
    }

    $config = Get-DockerComposeConfigJson -RepositoryRoot $RepositoryRoot
    if ($null -eq $config -or $null -eq $config.services) {
        return
    }

    $services = $config.services
    $postgres = $services.postgres
    if ($null -ne $postgres -and [string]::IsNullOrWhiteSpace($env:DATABASE_URL)) {
        $user = Get-ComposeServiceEnvironmentValue -ServiceConfig $postgres -Name 'POSTGRES_USER'
        if ([string]::IsNullOrWhiteSpace($user)) { $user = 'postgres' }
        $password = Get-ComposeServiceEnvironmentValue -ServiceConfig $postgres -Name 'POSTGRES_PASSWORD'
        if ([string]::IsNullOrWhiteSpace($password)) { $password = 'postgres' }
        $database = Get-ComposeServiceEnvironmentValue -ServiceConfig $postgres -Name 'POSTGRES_DB'
        if ([string]::IsNullOrWhiteSpace($database)) { $database = 'marketplace_ai' }
        $port = Get-PublishedHostPort -ServiceConfig $postgres -ContainerPort 5432
        if ($null -eq $port) { $port = 55432 }
        $encodedPassword = [Uri]::EscapeDataString($password)
        $databaseUrl = "postgres://${user}:${encodedPassword}@127.0.0.1:${port}/${database}?sslmode=disable"
        [Environment]::SetEnvironmentVariable('DATABASE_URL', $databaseUrl, [EnvironmentVariableTarget]::Process)
    }

    $redis = $services.redis
    if ($null -ne $redis -and [string]::IsNullOrWhiteSpace($env:REDIS_ADDR)) {
        $port = Get-PublishedHostPort -ServiceConfig $redis -ContainerPort 6379
        if ($null -eq $port) { $port = 6379 }
        [Environment]::SetEnvironmentVariable('REDIS_ADDR', "127.0.0.1:$port", [EnvironmentVariableTarget]::Process)
    }

    $minio = $services.minio
    if ($null -ne $minio) {
        if ([string]::IsNullOrWhiteSpace($env:S3_ENDPOINT)) {
            $port = Get-PublishedHostPort -ServiceConfig $minio -ContainerPort 9000
            if ($null -eq $port) { $port = 19000 }
            [Environment]::SetEnvironmentVariable('S3_ENDPOINT', "127.0.0.1:$port", [EnvironmentVariableTarget]::Process)
        }
        if ([string]::IsNullOrWhiteSpace($env:S3_ACCESS_KEY)) {
            $accessKey = Get-ComposeServiceEnvironmentValue -ServiceConfig $minio -Name 'MINIO_ROOT_USER'
            if ([string]::IsNullOrWhiteSpace($accessKey)) { $accessKey = 'minio' }
            [Environment]::SetEnvironmentVariable('S3_ACCESS_KEY', $accessKey, [EnvironmentVariableTarget]::Process)
        }
        if ([string]::IsNullOrWhiteSpace($env:S3_SECRET_KEY)) {
            $secretKey = Get-ComposeServiceEnvironmentValue -ServiceConfig $minio -Name 'MINIO_ROOT_PASSWORD'
            if ([string]::IsNullOrWhiteSpace($secretKey)) {
                throw 'S3_SECRET_KEY is required. Set it in .env.local (docker compose does not expose secrets safely in logs).'
            }
            [Environment]::SetEnvironmentVariable('S3_SECRET_KEY', $secretKey, [EnvironmentVariableTarget]::Process)
        }
    }

    $missing = @()
    foreach ($key in @('DATABASE_URL', 'REDIS_ADDR', 'S3_ENDPOINT', 'S3_ACCESS_KEY', 'S3_SECRET_KEY')) {
        $value = [Environment]::GetEnvironmentVariable($key, [EnvironmentVariableTarget]::Process)
        if ([string]::IsNullOrWhiteSpace($value)) {
            $missing += $key
        }
    }
    if ($missing.Count -gt 0) {
        $serviceList = ($requiredServices -join ' ')
        throw @"
Local infrastructure is not running or required settings are missing: $($missing -join ', ')

Run:
docker compose up -d $serviceList

Then copy .env.example to .env.local and fill secrets once.
"@
    }
}
