<#
  Dev helper script (PowerShell)
  - Loads environment variables from .env.local (or specified file)
  - Starts `vercel dev --listen 3000` in backend-refactor

  Usage:
    pwsh ./dev.ps1                # load .env.local then run vercel
    pwsh ./dev.ps1 -EnvFile .env.production  # load custom env file
#>

param(
  [string]$EnvFile = ".env.local",
  [int]$Port = 3000
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Write-Info($msg) { Write-Host "[dev] $msg" -ForegroundColor Cyan }
function Write-Warn($msg) { Write-Host "[dev] $msg" -ForegroundColor Yellow }
function Write-Err($msg)  { Write-Host "[dev] $msg" -ForegroundColor Red }

# Resolve to backend-refactor directory (this script's folder)
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $ScriptDir

if (-not (Test-Path $EnvFile)) {
  Write-Warn "Env file '$EnvFile' not found in $ScriptDir. Skipping load."
} else {
  Write-Info "Loading environment from '$EnvFile'..."
  $lines = Get-Content -Raw -Encoding UTF8 $EnvFile -ErrorAction Stop | `
           Select-String -Pattern '.*' -AllMatches | ForEach-Object { $_.ToString().Split("`n") } | `
           ForEach-Object { $_.TrimEnd("`r") }

  foreach ($line in $lines) {
    if ([string]::IsNullOrWhiteSpace($line)) { continue }
    if ($line.TrimStart().StartsWith('#')) { continue }

    $kv = $line.Split('=',2)
    if ($kv.Count -ne 2) { continue }
    $key = $kv[0].Trim()
    $val = $kv[1].Trim()

    # Remove surrounding quotes if present
    if ($val.Length -ge 2 -and (
      ($val.StartsWith('"') -and $val.EndsWith('"')) -or
      ($val.StartsWith("'") -and $val.EndsWith("'"))
    )) { $val = $val.Substring(1, $val.Length-2) }

    # If single-line, strip trailing spaces/tabs to avoid polluted URLs/DSNs
    if ($val -notmatch "[`r`n]") {
      $val = [Regex]::Replace($val, "[ \t]+$", "")
    }

    # Export to current process env (dynamic env var name)
    Set-Item -Path ("Env:{0}" -f $key) -Value $val

    # Mask sensitive values in logs
    $mask = $false
    foreach ($kw in @('SECRET','SERVICE_KEY','TOKEN','PASSWORD','KEY','JWT')) {
      if ($key.ToUpper().Contains($kw)) { $mask = $true; break }
    }
    if ($mask) { Write-Info "set $key=******" } else { Write-Info "set $key=$val" }
  }
}

# Ensure ENVIRONMENT default
if (-not $env:ENVIRONMENT) { $env:ENVIRONMENT = 'development'; Write-Info "set ENVIRONMENT=$($env:ENVIRONMENT)" }

# Check vercel availability
try {
  $ver = (vercel --version) 2>$null
  if (-not $ver) { throw "vercel not found" }
} catch {
  Write-Err "Vercel CLI not found. Install with: npm i -g vercel"
  exit 1
}

Write-Info "Starting: vercel dev --listen $Port"
Write-Host "---------------------------------------------"
vercel dev --listen $Port
