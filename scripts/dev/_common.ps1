# Shared helpers for local dev PowerShell scripts.

function Get-RepositoryRoot {
    param(
        [string]$StartPath = $PSScriptRoot
    )

    $dir = $StartPath
    if (-not (Test-Path -LiteralPath $dir)) {
        $dir = (Get-Location).Path
    }
    if (Test-Path -LiteralPath $dir -PathType Leaf) {
        $dir = Split-Path -Parent $dir
    }

    while ($true) {
        $gitPath = Join-Path $dir '.git'
        $composePath = Join-Path $dir 'docker-compose.yml'
        if ((Test-Path -LiteralPath $gitPath) -and (Test-Path -LiteralPath $composePath)) {
            return (Resolve-Path -LiteralPath $dir).Path
        }
        $parent = Split-Path -Parent $dir
        if ([string]::IsNullOrWhiteSpace($parent) -or $parent -eq $dir) {
            throw "Repository root not found (expected .git and docker-compose.yml)."
        }
        $dir = $parent
    }
}

function Test-SecretEnvironmentName {
    param([string]$Name)

    $upper = $Name.ToUpperInvariant()
    $patterns = @('KEY', 'SECRET', 'PASSWORD', 'TOKEN', 'CREDENTIAL', 'DSN')
    foreach ($pattern in $patterns) {
        if ($upper.Contains($pattern)) {
            return $true
        }
    }
    if ($upper -eq 'DATABASE_URL') {
        return $true
    }
    return $false
}

function Get-SafeEnvironmentDisplayValue {
    param(
        [string]$Name,
        [string]$Value
    )

    if ([string]::IsNullOrWhiteSpace($Value)) {
        return 'unset'
    }
    if (Test-SecretEnvironmentName -Name $Name) {
        if ($Name.ToUpperInvariant() -eq 'DATABASE_URL') {
            return Get-DatabaseURLDisplayValue -DatabaseURL $Value
        }
        return 'configured'
    }
    return $Value
}

function Get-DatabaseURLDisplayValue {
    param([string]$DatabaseURL)

    try {
        $uri = [Uri]$DatabaseURL
        $database = $uri.AbsolutePath.TrimStart('/')
        if ([string]::IsNullOrWhiteSpace($database)) {
            $database = '(unknown)'
        }
        $port = if ($uri.Port -gt 0) { $uri.Port } else { 'default' }
        return "host=$($uri.Host) port=$port database=$database"
    }
    catch {
        return 'configured'
    }
}

function ConvertTo-BooleanString {
    param([bool]$Value)
    if ($Value) { return 'true' }
    return 'false'
}

function Test-GoDevtoolsPipelineProcessRunning {
    $patterns = @('run-ai-pipeline', 'runAIPipeline')
    $processNames = @('go', 'devtools', 'main')

    foreach ($procName in $processNames) {
        $procs = Get-CimInstance -ClassName Win32_Process -Filter "Name LIKE '$procName%'" -ErrorAction SilentlyContinue
        foreach ($proc in $procs) {
            $cmd = [string]$proc.CommandLine
            if ([string]::IsNullOrWhiteSpace($cmd)) {
                continue
            }
            foreach ($pattern in $patterns) {
                if ($cmd -match [regex]::Escape($pattern)) {
                    return $true
                }
            }
        }
    }
    return $false
}

function Write-GoCommandOutputLine {
    param($Line)

    if ($Line -is [System.Management.Automation.ErrorRecord]) {
        Write-Host $Line.ToString()
        return
    }
    Write-Host ([string]$Line)
}

function ConvertTo-NativeArgumentString {
    param([string[]]$Arguments)

    $escaped = foreach ($arg in $Arguments) {
        if ($null -eq $arg) { '""'; continue }
        $text = [string]$arg
        if ($text -match '[\s"`]') {
            '"' + ($text -replace '(["`])', '`\$1') + '"'
        }
        else {
            $text
        }
    }
    return ($escaped -join ' ')
}

function Invoke-NativeCommandDetailed {
    param(
        [Parameter(Mandatory = $true)]
        [string]$FilePath,
        [Parameter(Mandatory = $true)]
        [string[]]$Arguments,
        [string]$WorkingDirectory,
        [switch]$SuppressStreamingOutput
    )

    $command = Get-Command $FilePath -ErrorAction SilentlyContinue
    if ($null -eq $command) {
        throw "$FilePath not found in PATH."
    }

    if ([string]::IsNullOrWhiteSpace($WorkingDirectory)) {
        $WorkingDirectory = (Get-Location).Path
    }

    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = $command.Source
    $psi.UseShellExecute = $false
    $psi.RedirectStandardOutput = $true
    $psi.RedirectStandardError = $true
    $psi.CreateNoWindow = $true
    $psi.WorkingDirectory = $WorkingDirectory
    $psi.Arguments = ConvertTo-NativeArgumentString -Arguments $Arguments

    $process = New-Object System.Diagnostics.Process
    $process.StartInfo = $psi

    if (-not $process.Start()) {
        throw "Failed to start process: $FilePath"
    }

    $stdoutTask = $process.StandardOutput.ReadToEndAsync()
    $stderrTask = $process.StandardError.ReadToEndAsync()
    $process.WaitForExit()
    $stdoutText = $stdoutTask.GetAwaiter().GetResult()
    $stderrText = $stderrTask.GetAwaiter().GetResult()

    $splitOptions = [System.StringSplitOptions]::RemoveEmptyEntries
    $stdoutArray = @()
    $stderrArray = @()
    if (-not [string]::IsNullOrWhiteSpace($stdoutText)) {
        $stdoutArray = @($stdoutText -split "\r?\n", 0, $splitOptions)
    }
    if (-not [string]::IsNullOrWhiteSpace($stderrText)) {
        $stderrArray = @($stderrText -split "\r?\n", 0, $splitOptions)
    }

    if (-not $SuppressStreamingOutput) {
        foreach ($line in $stdoutArray) { Write-Host $line }
        foreach ($line in $stderrArray) { Write-Host $line }
    }

    $all = New-Object System.Collections.Generic.List[string]
    foreach ($line in $stdoutArray) { [void]$all.Add($line) }
    foreach ($line in $stderrArray) { [void]$all.Add($line) }

    return [PSCustomObject]@{
        ExitCode = [int]$process.ExitCode
        StdOut   = $stdoutArray
        StdErr   = $stderrArray
        Output   = @($all.ToArray())
    }
}

function Invoke-GoDevtoolsCore {
    param(
        [string]$RepositoryRoot,
        [string[]]$Arguments
    )

    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        throw 'Go toolchain not found in PATH. Install Go and ensure go.exe is available.'
    }

    $backendDir = Join-Path $RepositoryRoot 'backend'
    return Invoke-NativeCommandDetailed -FilePath 'go' -Arguments $Arguments -WorkingDirectory $backendDir
}

function Invoke-GoLocalEnvChecker {
    param([string]$RepositoryRoot)

    try {
        return Invoke-GoDevtoolsCore -RepositoryRoot $RepositoryRoot -Arguments @(
            'run', './cmd/devtools', 'check-local-env'
        )
    }
    catch {
        return [PSCustomObject]@{
            ExitCode = 2
            Output   = @($_.Exception.Message)
        }
    }
}

function Invoke-GoDevtools {
    param(
        [string]$RepositoryRoot,
        [string[]]$Arguments,
        [switch]$AllowFailure
    )

    try {
        $result = Invoke-GoDevtoolsCore -RepositoryRoot $RepositoryRoot -Arguments $Arguments
    }
    catch {
        Write-Host $_.Exception.Message
        if (-not $AllowFailure) {
            exit 2
        }
        return 2
    }

    foreach ($line in $result.Output) {
        Write-GoCommandOutputLine -Line $line
    }

    if (-not $AllowFailure -and $result.ExitCode -ne 0) {
        exit $result.ExitCode
    }
    return $result.ExitCode
}

function Get-ResolvedEnvFileName {
    param([string]$RepositoryRoot)

    $candidates = @('.env.local', '.env.development.local', '.env')
    foreach ($name in $candidates) {
        $path = Join-Path $RepositoryRoot $name
        if (Test-Path -LiteralPath $path) {
            return $name
        }
    }
    return $null
}
