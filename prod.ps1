<#
  Prod helper script (PowerShell)
  - Syncs env vars from an env file to Vercel project
  - Optionally deploys to the selected target

  Usage:
    pwsh ./prod.ps1                                  # sync .env.production to production, then deploy
    pwsh ./prod.ps1 -NoDeploy                        # only sync env vars
    pwsh ./prod.ps1 -DeployOnly                      # only deploy (no env sync)
    pwsh ./prod.ps1 -Target preview                  # sync to preview and deploy preview
    pwsh ./prod.ps1 -EnvFile .env.local -NoDeploy    # sync from custom file to current target
    pwsh ./prod.ps1 -DryRun                          # show planned actions only

  Notes:
    - Requires Vercel CLI installed and logged in (vercel login)
    - Requires this folder to be linked (vercel link) to the correct project
    - Converts literal \n and \r in values to real newlines by default (disable with -NoUnescape)
    - If VERCEL_TOKEN is set, uses Vercel REST API to sync vars (no newline issues)
#>

param(
  [string]$EnvFile = ".env.production",
  [ValidateSet('production','preview','development')]
  [string]$Target = "production",
  [switch]$NoDeploy,
  [switch]$DeployOnly,
  [switch]$DryRun,
  [switch]$NoUnescape
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Write-Info($msg) { Write-Host "[prod] $msg" -ForegroundColor Cyan }
function Write-Warn($msg) { Write-Host "[prod] $msg" -ForegroundColor Yellow }
function Write-Err($msg)  { Write-Host "[prod] $msg" -ForegroundColor Red }

# Resolve to backend-refactor directory (this script's folder)
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $ScriptDir

function Ensure-Vercel {
  try {
    $ver = (vercel --version) 2>$null
    if (-not $ver) { throw "vercel not found" }
  } catch {
    Write-Err "Vercel CLI not found. Install with: npm i -g vercel"
    exit 1
  }
  try {
    $who = (vercel whoami) 2>$null
    if (-not $who) { throw "not logged in" }
  } catch {
    Write-Err "Not logged in. Run: vercel login"
    exit 1
  }
  if (-not (Test-Path ".vercel/project.json")) {
    Write-Warn "This folder is not linked to a Vercel project."
    Write-Warn "Run: vercel link (interactive) and re-run this script."
    exit 1
  }
}

function Get-VercelProjectMeta {
  $metaPath = Join-Path $ScriptDir ".vercel/project.json"
  if (-not (Test-Path $metaPath)) { throw ".vercel/project.json not found; run 'vercel link'" }
  $json = Get-Content $metaPath -Raw | ConvertFrom-Json
  return @{ projectId = $json.projectId; teamId = $json.orgId }
}

function Parse-EnvFile([string]$path) {
  if (-not (Test-Path $path)) {
    Write-Err "Env file '$path' not found."
    exit 1
  }
  $lines = Get-Content -Raw -Encoding UTF8 $path -ErrorAction Stop | `
           Select-String -Pattern '.*' -AllMatches | ForEach-Object { $_.ToString().Split("`n") } | `
           ForEach-Object { $_.TrimEnd("`r") }

  $map = @{}
  foreach ($line in $lines) {
    if ([string]::IsNullOrWhiteSpace($line)) { continue }
    $trim = $line.Trim()
    if ($trim.StartsWith('#')) { continue }
    if ($trim.StartsWith('export ')) { $trim = $trim.Substring(7).Trim() }

    $kv = $trim.Split('=',2)
    if ($kv.Count -ne 2) { continue }
    $key = $kv[0].Trim()
    $val = $kv[1]

    # Trim and remove surrounding quotes if present
    $val = $val.Trim()
    if ($val.Length -ge 2 -and (
      ($val.StartsWith('"') -and $val.EndsWith('"')) -or
      ($val.StartsWith("'") -and $val.EndsWith("'"))
    )) { $val = $val.Substring(1, $val.Length-2) }

    if (-not $NoUnescape) {
      # Convert literal \n/\r sequences to real newlines/carriage returns
      $val = $val -replace '\\n', "`n"
      $val = $val -replace '\\r', "`r"
    }

    # If the value is single-line (no CR/LF inside), trim trailing spaces/tabs
    if ($val -notmatch "[`r`n]") {
      $val = [Regex]::Replace($val, "[ \t]+$", "")
    }

    $map[$key] = $val
  }
  return $map
}

function Mask([string]$key, [string]$val) {
  $mask = $false
  foreach ($kw in @('SECRET','SERVICE_KEY','TOKEN','PASSWORD','KEY','JWT')) {
    if ($key.ToUpper().Contains($kw)) { $mask = $true; break }
  }
  if ($mask) { return '******' } else { return $val }
}

function Sync-VercelEnv([hashtable]$envMap, [string]$target, [switch]$dryRun) {
  Write-Info "Syncing $(($envMap.Keys).Count) vars to Vercel ($target)..."
  foreach ($key in $envMap.Keys) {
    $val = $envMap[$key]
    if ($dryRun) {
      Write-Info ("plan set {0}={1}" -f $key, (Mask $key $val))
      continue
    }
    try {
      vercel env rm $key $target -y *> $null
    } catch {
      # ignore if not exists
    }
    # Feed exact bytes without trailing newline using a temp file + cmd type
    $tmp = [System.IO.Path]::GetTempFileName()
    try {
      $utf8NoBom = New-Object System.Text.UTF8Encoding($false)
      [System.IO.File]::WriteAllText($tmp, $val, $utf8NoBom)
      & cmd /c "type `"$tmp`"" | vercel env add $key $target
      Write-Info ("set {0}={1}" -f $key, (Mask $key $val))
    } finally {
      Remove-Item -Force $tmp -ErrorAction SilentlyContinue
    }
  }
}

function Sync-VercelEnvApi([hashtable]$envMap, [string]$target, [switch]$dryRun) {
  $meta = Get-VercelProjectMeta
  $projectId = $meta.projectId
  $teamId = $meta.teamId
  $base = "https://api.vercel.com"
  if (-not $env:VERCEL_TOKEN) { throw "VERCEL_TOKEN not set" }
  $headers = @{ Authorization = "Bearer $($env:VERCEL_TOKEN)" }

  # Load current envs for the target
  $listUrl = "$base/v9/projects/$projectId/env?target=$target&decrypt=false" + ($(if ($teamId) { "&teamId=$teamId" } else { "" }))
  $resp = Invoke-RestMethod -Method GET -Uri $listUrl -Headers $headers -ErrorAction Stop
  $items = if ($resp.envs) { $resp.envs } else { $resp }

  foreach ($key in $envMap.Keys) {
    $val = $envMap[$key]
    if ($dryRun) {
      Write-Info ("plan api set {0}={1}" -f $key, (Mask $key $val))
      continue
    }
    # Delete existing entries for this key+target
    $toDelete = @()
    foreach ($it in $items) {
      if ($it.key -eq $key -and $it.target -contains $target) { $toDelete += $it.id }
    }
    foreach ($id in $toDelete) {
      $delUrl = "$base/v9/projects/$projectId/env/$id" + ($(if ($teamId) { "?teamId=$teamId" } else { "" }))
      try { Invoke-RestMethod -Method DELETE -Uri $delUrl -Headers $headers -ErrorAction Stop | Out-Null } catch {}
    }
    # Create new value (plain type; Vercel encrypts at rest)
    $body = @{ key = $key; value = $val; type = 'plain'; target = @($target) } | ConvertTo-Json -Depth 5
    $addUrl = "$base/v9/projects/$projectId/env" + ($(if ($teamId) { "?teamId=$teamId" } else { "" }))
    Invoke-RestMethod -Method POST -Uri $addUrl -Headers ($headers + @{ 'Content-Type'='application/json' }) -Body $body -ErrorAction Stop | Out-Null
    Write-Info ("api set {0}={1}" -f $key, (Mask $key $val))
  }
}

function Deploy-Target([string]$target) {
  switch ($target) {
    'production' {
      Write-Info "Deploying to production: vercel --prod"
      vercel --prod
    }
    'preview' {
      Write-Info "Deploying preview: vercel"
      vercel
    }
    default {
      Write-Warn "Development target doesn't support deploy here. Use dev.ps1."
    }
  }
}

# Decide actions
Ensure-Vercel

$doSync = $true
$doDeploy = $true
if ($DeployOnly) { $doSync = $false; $doDeploy = $true }
if ($NoDeploy) { $doDeploy = $false }

if ($doSync) {
  $map = Parse-EnvFile -path $EnvFile
  if ($env:VERCEL_TOKEN) {
    Write-Info "Using Vercel API mode (VERCEL_TOKEN detected)"
    Sync-VercelEnvApi -envMap $map -target $Target -dryRun:$DryRun
  } else {
    Sync-VercelEnv -envMap $map -target $Target -dryRun:$DryRun
  }
}

if ($doDeploy) {
  if ($DryRun) {
    Write-Info "plan deploy target=$Target"
  } else {
    Deploy-Target $Target
  }
}
