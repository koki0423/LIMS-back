# dev-run.ps1
# Frontend: python -m http.server 8080
# Backend : go run mian.go
# - Starts each in a separate PowerShell window.
# - Stop each with Ctrl+C in its window.
# Usage example:
# 例：フロントが .\frontend\dist 、バックが .\backend の場合
# .\dev-run.ps1 -FrontendDir ".\frontend" -BackendDir ".\backend"

param(
  [Parameter(Mandatory = $true)]
  [string]$FrontendDir,

  [Parameter(Mandatory = $true)]
  [string]$BackendDir,

  [int]$FrontendPort = 8080,

  [string]$BackendEntry = "mian.go"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Require-Command([string]$cmd) {
  $found = Get-Command $cmd -ErrorAction SilentlyContinue
  if (-not $found) {
    throw "Command not found: $cmd. Please install it and ensure it is on PATH."
  }
}

function Require-Directory([string]$path, [string]$label) {
  $full = Resolve-Path -Path $path -ErrorAction Stop
  if (-not (Test-Path -Path $full -PathType Container)) {
    throw "$label directory does not exist: $path"
  }
  return $full.Path
}

# --- preflight ---
Require-Command "python"
Require-Command "go"

$frontendPath = Require-Directory $FrontendDir "Frontend"
$backendPath  = Require-Directory $BackendDir  "Backend"

$backendMain = Join-Path $backendPath $BackendEntry
if (-not (Test-Path -Path $backendMain -PathType Leaf)) {
  throw "Backend entry file not found: $backendMain"
}

Write-Host "FrontendDir : $frontendPath"
Write-Host "BackendDir  : $backendPath"
Write-Host "FrontendPort: $FrontendPort"
Write-Host "BackendEntry: $BackendEntry"
Write-Host ""

# --- start frontend server ---
$frontendTitle = "Frontend http.server :$FrontendPort"
$frontendCmd = @"
`$Host.UI.RawUI.WindowTitle = '$frontendTitle'
Set-Location -LiteralPath '$frontendPath'
python -m http.server $FrontendPort
"@

Start-Process -FilePath "powershell.exe" -ArgumentList @(
  "-NoExit",
  "-ExecutionPolicy", "Bypass",
  "-Command", $frontendCmd
) | Out-Null

# --- start backend server ---
$backendTitle = "Backend go run $BackendEntry"
$backendCmd = @"
`$Host.UI.RawUI.WindowTitle = '$backendTitle'
Set-Location -LiteralPath '$backendPath'
go run '$BackendEntry'
"@

Start-Process -FilePath "powershell.exe" -ArgumentList @(
  "-NoExit",
  "-ExecutionPolicy", "Bypass",
  "-Command", $backendCmd
) | Out-Null

Write-Host "Started:"
Write-Host "  - $frontendTitle  (http://localhost:$FrontendPort)"
Write-Host "  - $backendTitle"
Write-Host ""
Write-Host "Stop: close each window or press Ctrl+C in each window."
