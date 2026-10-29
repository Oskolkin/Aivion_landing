# Stops common local dev listeners (Next.js on 3000, API on 8081/18081).
# Usage: powershell -ExecutionPolicy Bypass -File scripts/dev/stop-dev-ports.ps1

$ErrorActionPreference = "SilentlyContinue"
$ports = @(3000, 3001, 3002, 8081, 18081)

foreach ($port in $ports) {
  $pids = netstat -ano | ForEach-Object {
    if ($_ -match ":$port\s+\S+\s+\S+\s+LISTENING\s+(\d+)\s*$") { [int]$Matches[1] }
  } | Sort-Object -Unique

  foreach ($pid in $pids) {
    if ($pid -le 4) { continue }
    $proc = Get-Process -Id $pid -ErrorAction SilentlyContinue
    if (-not $proc) { continue }
    Write-Host "Stopping PID $pid ($($proc.ProcessName)) on port $port"
    Stop-Process -Id $pid -Force -ErrorAction SilentlyContinue
  }
}

Write-Host "Done. Verify with: netstat -ano | findstr ':3000 :8081 :18081'"
