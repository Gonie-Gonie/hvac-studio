param(
  [string]$Version = '0.1.0-dev',
  [string]$OutputRoot = ''
)

$ErrorActionPreference = 'Stop'

. (Join-Path $PSScriptRoot '..\dev\env.ps1')

if (-not $env:HVAC_STUDIO_GO) {
  throw 'go was not found. Run scripts/dev/setup.ps1 first.'
}

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
if (-not $OutputRoot) {
  $OutputRoot = Join-Path $RepoRoot 'dist'
}

$StudioRoot = Join-Path $OutputRoot "hvac-studio-$Version"
$BinRoot = Join-Path $StudioRoot 'bin'
$StudioExe = Join-Path $BinRoot 'hvac-studio.exe'

New-Item -ItemType Directory -Force -Path $BinRoot | Out-Null

Push-Location (Join-Path $RepoRoot 'tools\go')
try {
  Invoke-Checked $env:HVAC_STUDIO_GO @('build', '-o', $StudioExe, '.\cmd\studio')
} finally {
  Pop-Location
}

Write-Host "Studio build written to $StudioExe"
