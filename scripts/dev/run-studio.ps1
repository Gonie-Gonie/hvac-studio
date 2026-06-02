param(
  [string]$Addr = '127.0.0.1:5174',
  [switch]$Server,
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
    $RepoRoot
  )
  if ($Server -or $NoWindow) {
    $StudioArgs += @(
      '--server',
      '--addr',
      $Addr
    )
  }
  Invoke-Checked $env:HVAC_STUDIO_GO $StudioArgs
} finally {
  Pop-Location
}
