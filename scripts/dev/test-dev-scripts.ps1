# Minimal script-level tests for dev launcher scripts (no Pester dependency).

$ErrorActionPreference = 'Stop'
$failures = 0

function Assert-True {
    param([bool]$Condition, [string]$Message)
    if (-not $Condition) {
        Write-Host "FAIL: $Message"
        $script:failures++
    }
    else {
        Write-Host "PASS: $Message"
    }
}

function Assert-Equals {
    param($Expected, $Actual, [string]$Message)
    if ($Expected -ne $Actual) {
        Write-Host "FAIL: $Message (expected '$Expected', got '$Actual')"
        $script:failures++
    }
    else {
        Write-Host "PASS: $Message"
    }
}

$devDir = $PSScriptRoot
$repoRoot = (Get-Item $devDir).Parent.Parent.FullName
. (Join-Path $devDir '_common.ps1')
. (Join-Path $devDir 'load-env.ps1')

# 1. Repository root from another working directory
Push-Location $env:TEMP
try {
    $root = Get-RepositoryRoot -StartPath $devDir
    Assert-True ($root -eq $repoRoot) 'Repository root resolved from dev scripts directory'
}
finally {
    Pop-Location
}

# 2-4. Dotenv parsing
$comment = Parse-DotEnvLine -Line '# comment'
Assert-True ($null -eq $comment) 'Comments are ignored'

$equals = Parse-DotEnvLine -Line 'NAME=value=with=equals'
Assert-Equals 'NAME' $equals.Name 'Equals sign inside value (name)'
Assert-Equals 'value=with=equals' $equals.Value 'Equals sign inside value (value)'

$quoted = Parse-DotEnvLine -Line 'OPENAI_API_KEY="sk-test"'
Assert-Equals 'sk-test' $quoted.Value 'Quoted values are unwrapped'

# 5-6. Process env precedence and override
$tempEnvDir = Join-Path $env:TEMP ("aivion-env-test-" + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $tempEnvDir | Out-Null
$envFile = Join-Path $tempEnvDir '.env.local'
Set-Content -LiteralPath $envFile -Value @(
    'APP_ENV=from_file'
    'DATABASE_URL=postgres://from:file@127.0.0.1:55432/db?sslmode=disable'
)

[Environment]::SetEnvironmentVariable('APP_ENV', 'process', [EnvironmentVariableTarget]::Process)
Import-DotEnvFile -Path $envFile
Assert-Equals 'process' ([Environment]::GetEnvironmentVariable('APP_ENV', [EnvironmentVariableTarget]::Process)) 'Process env has priority over .env.local'

Import-DotEnvFile -Path $envFile -OverrideEnvironment
Assert-Equals 'from_file' ([Environment]::GetEnvironmentVariable('APP_ENV', [EnvironmentVariableTarget]::Process)) '-OverrideEnvironment replaces process env'

# 7-8. Secret masking
$masked = Get-SafeEnvironmentDisplayValue -Name 'OPENAI_API_KEY' -Value 'sk-secret'
Assert-Equals 'configured' $masked 'Secrets are masked'

$dbDisplay = Get-SafeEnvironmentDisplayValue -Name 'DATABASE_URL' -Value 'postgres://user:pass@127.0.0.1:55432/marketplace_ai?sslmode=disable'
Assert-True ($dbDisplay -notmatch 'pass') 'DATABASE_URL password is not printed'
Assert-True ($dbDisplay -match 'host=127.0.0.1') 'DATABASE_URL host is shown safely'

# 9. Missing .env.local message path
$emptyEnvDir = Join-Path $env:TEMP ("aivion-empty-" + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $emptyEnvDir | Out-Null
$missingRoot = Get-ResolvedEnvFileName -RepositoryRoot $emptyEnvDir
Assert-True ($null -eq $missingRoot) 'Missing .env.local is detectable'
Remove-Item -LiteralPath $emptyEnvDir -Recurse -Force

# 10-11. Docker discovery port helpers
. (Join-Path $devDir 'docker-discovery.ps1')
$sampleService = [PSCustomObject]@{
    ports = @('55432:5432', '6379:6379')
}
$pgPort = Get-PublishedHostPort -ServiceConfig $sampleService -ContainerPort 5432
$redisPort = Get-PublishedHostPort -ServiceConfig $sampleService -ContainerPort 6379
Assert-Equals 55432 $pgPort 'PostgreSQL published port parsing'
Assert-Equals 6379 $redisPort 'Redis published port parsing'

# 12. Stopped docker message is documented in script
$discoveryScript = Get-Content -LiteralPath (Join-Path $devDir 'docker-discovery.ps1') -Raw
Assert-True ($discoveryScript -match 'docker compose up -d postgres redis minio') 'Stopped docker gives actionable compose instruction'

# 13-20. Static script checks
$runScript = Get-Content -LiteralPath (Join-Path $devDir 'run-ai-pipeline.ps1') -Raw
Assert-True ($runScript -notmatch 'Invoke-Expression') 'Invoke-Expression is not used'
Assert-True ($runScript -match 'check-ai-run-readiness') 'Readiness is invoked'
Assert-True ($runScript -match 'check-ai-pipeline-running') 'Running run guard is invoked'
Assert-True ($runScript -match 'MarkStaleRunFailed') 'Stale run requires explicit flag'
Assert-True ($runScript -match '--resume-run-id=') 'Resume arguments are formed'
Assert-True ($runScript -match '--seller-id=') 'Full-run arguments are formed'
Assert-True ($runScript -match 'exit \$pipelineExit') 'Go exit code is propagated'
Assert-True ($runScript -match '--result-json=') 'Machine-readable result file is requested'

$loadScript = Get-Content -LiteralPath (Join-Path $devDir 'load-env.ps1') -Raw
Assert-True ($loadScript -match 'EnvironmentVariableTarget\]::Process') 'Only process environment is modified'

# 21-30. check-env.ps1 and Go checker integration
$checkEnvScript = Get-Content -LiteralPath (Join-Path $devDir 'check-env.ps1') -Raw
Assert-True ($checkEnvScript -match 'Invoke-GoLocalEnvChecker') 'check-env uses dedicated Go checker helper'
Assert-True ($checkEnvScript -notmatch '\$exitCode = Invoke-GoDevtools') 'check-env does not assign leaky Invoke-GoDevtools output to exit code'
Assert-True ($checkEnvScript -match '\$checkExitCode = \$checkerResult\.ExitCode') 'check-env uses checker ExitCode property'
Assert-True ($checkEnvScript -match 'exit \$checkExitCode') 'check-env propagates checker exit code'
Assert-True ($checkEnvScript -match 'OverrideEnvironment:\$OverrideEnvironment') 'check-env passes -OverrideEnvironment to loader'
Assert-True ($checkEnvScript -notmatch 'Ready for real AI run') 'check-env does not parse readiness stdout text'

$commonScript = Get-Content -LiteralPath (Join-Path $devDir '_common.ps1') -Raw
Assert-True ($commonScript -match 'Go toolchain not found in PATH') 'Missing Go gives clear error message'
Assert-True ($commonScript -match 'System\.Diagnostics\.ProcessStartInfo') 'Go runner uses ProcessStartInfo'
Assert-True ($commonScript -match 'RedirectStandardOutput = \$true') 'Go runner captures stdout separately'
Assert-True ($commonScript -match 'RedirectStandardError = \$true') 'Go runner captures stderr separately'
Assert-True ($commonScript.Contains('Join-Path $RepositoryRoot ''backend''')) 'Go commands run from backend working directory'

function Invoke-CmdChecked {
    param([string[]]$Arguments)
    $originalLocation = Get-Location
    $previousErrorAction = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        $output = @(& cmd @Arguments 2>&1)
        $exitCode = $LASTEXITCODE
        return [PSCustomObject]@{ ExitCode = $exitCode; Output = $output }
    }
    finally {
        $ErrorActionPreference = $previousErrorAction
        Set-Location $originalLocation
    }
}

$okResult = Invoke-CmdChecked @('/c', 'echo hello & exit /b 0')
Assert-Equals 0 $okResult.ExitCode 'Native checker pattern returns exit code 0'
Assert-True (($okResult.Output | Out-String) -match 'hello') 'Native checker pattern captures stdout'

$failResult = Invoke-CmdChecked @('/c', 'echo problem 1>&2 & exit /b 1')
Assert-Equals 1 $failResult.ExitCode 'Native checker pattern returns exit code 1'
Assert-True (($failResult.Output | Out-String) -match 'problem') 'Native checker pattern captures stderr'

# Regression: informational stderr must not fail when exit code is 0.
try {
    $nativeInfo = Invoke-NativeCommandDetailed -FilePath 'cmd' -Arguments @('/c', 'echo info-line 1>&2 & echo ok-json & exit /b 0') -WorkingDirectory $repoRoot -SuppressStreamingOutput
    Assert-Equals 0 $nativeInfo.ExitCode 'Informational stderr with exit 0 stays successful'
    Assert-True (($nativeInfo.StdErr | Out-String) -match 'info-line') 'Informational stderr is captured'
    Assert-True (($nativeInfo.StdOut | Out-String) -match 'ok-json') 'stdout remains parseable with stderr present'

    $nativeFail = Invoke-NativeCommandDetailed -FilePath 'cmd' -Arguments @('/c', 'echo err-line 1>&2 & exit /b 1') -WorkingDirectory $repoRoot -SuppressStreamingOutput
    Assert-Equals 1 $nativeFail.ExitCode 'stderr with exit 1 is failure'

    $nativeStdOutOnly = Invoke-NativeCommandDetailed -FilePath 'cmd' -Arguments @('/c', 'echo only-stdout & exit /b 0') -WorkingDirectory $repoRoot -SuppressStreamingOutput
    Assert-Equals 0 $nativeStdOutOnly.ExitCode 'stdout-only command succeeds'

    $nativeMixed = Invoke-NativeCommandDetailed -FilePath 'cmd' -Arguments @('/c', '(echo out1 & echo err1 1>&2 & echo out2 & echo err2 1>&2) & exit /b 0') -WorkingDirectory $repoRoot -SuppressStreamingOutput
    Assert-Equals 0 $nativeMixed.ExitCode 'mixed stdout/stderr succeeds'
    Assert-True (($nativeMixed.StdOut | Out-String) -match 'out1') 'mixed stdout collected'
    Assert-True (($nativeMixed.StdErr | Out-String) -match 'err1') 'mixed stderr collected'

    $nativeLong = Invoke-NativeCommandDetailed -FilePath 'cmd' -Arguments @('/c', 'ping 127.0.0.1 -n 2 >nul & echo done & exit /b 0') -WorkingDirectory $repoRoot -SuppressStreamingOutput
    Assert-Equals 0 $nativeLong.ExitCode 'longer command succeeds'
    Assert-True ($nativeLong.ExitCode -is [int]) 'ExitCode is always int'
}
catch {
    Assert-True $false ("Invoke-NativeCommandDetailed regression block failed: " + $_.Exception.Message)
}

$beforeLocation = (Get-Location).Path
Push-Location $env:TEMP
try {
    $null = Invoke-CmdChecked @('/c', 'cd')
    Assert-Equals $env:TEMP (Get-Location).Path 'Working directory restored after native checker'
}
finally {
    Set-Location $beforeLocation
}

if (Get-Command go -ErrorAction SilentlyContinue) {
    . (Join-Path $devDir 'docker-discovery.ps1')
    Import-LocalDevEnvironment -RepositoryRoot $repoRoot -OverrideEnvironment -Quiet
    Resolve-DockerInfrastructure -RepositoryRoot $repoRoot
    Set-LocalDevDefaultEnvironment -RepositoryRoot $repoRoot

    $directChecker = Invoke-GoLocalEnvChecker -RepositoryRoot $repoRoot
    $directText = ($directChecker.Output | Out-String)
    Assert-True ($directText -match 'Configuration: PASS') 'Go checker report contains configuration result'
    Assert-True ($directText -match 'PostgreSQL connection: PASS') 'Go checker report contains connection results'
    Assert-True ($directText -notmatch 'sk-') 'Go checker output does not leak OpenAI secrets'

    Push-Location $repoRoot
    try {
        $checkOutput = & powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $devDir 'check-env.ps1') -OverrideEnvironment 2>&1
        $checkExit = $LASTEXITCODE
        $checkText = ($checkOutput | ForEach-Object { if ($_ -is [System.Management.Automation.ErrorRecord]) { $_.ToString() } else { [string]$_ } }) -join "`n"
        $expectsYes = ($checkExit -eq 0)
        $hasYes = ($checkText -match 'Ready to run AI pipeline: YES')
        Assert-True ($hasYes -eq $expectsYes) 'check-env YES/NO matches exit code'
        if ($checkExit -eq 0) {
            Assert-True ($checkText -match 'Configuration: PASS') 'check-env prints Go checker report'
        }
    }
    finally {
        Pop-Location
    }
}

$runScriptText = Get-Content -LiteralPath (Join-Path $devDir 'run-ai-pipeline.ps1') -Raw
Assert-True ($runScriptText -notmatch '& go @pipelineArgs 2>&1') 'run-ai-pipeline does not use fragile 2>&1 capture for main run'

Remove-Item -LiteralPath $tempEnvDir -Recurse -Force

Write-Host ''
if ($failures -gt 0) {
    Write-Host "Script tests failed: $failures"
    exit 1
}
Write-Host 'All script tests passed.'
exit 0
