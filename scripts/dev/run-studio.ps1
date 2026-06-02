param(
  [string]$Addr = '127.0.0.1:5174',
  [switch]$NoWindow
)

$ErrorActionPreference = 'Stop'

. (Join-Path $PSScriptRoot 'env.ps1')

if (-not $env:HVAC_STUDIO_GO) {
  throw 'go was not found. Run scripts/dev/setup.ps1 first.'
}

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path

Push-Location (Join-Path $RepoRoot 'tools\go')
try {
  $StudioArgs = @(
    'run',
    '.\cmd\studio',
    '--repo',
    $RepoRoot,
    '--addr',
    $Addr
  )
  if ($NoWindow) {
    $StudioArgs += '--no-window'
  }
  Invoke-Checked $env:HVAC_STUDIO_GO $StudioArgs
} finally {
  Pop-Location
}
