$ErrorActionPreference = 'Stop'

. (Join-Path $PSScriptRoot 'env.ps1')

if (-not $env:HVAC_STUDIO_GO) {
  throw 'go was not found. Run scripts/dev/setup.ps1 first.'
}

$RepoRoot = Resolve-Path (Join-Path $PSScriptRoot '..\..')
Push-Location (Join-Path $RepoRoot 'tools\go')
try {
  Invoke-Checked $env:HVAC_STUDIO_GO (@('run', '.\cmd\bcs-runner') + $args)
} finally {
  Pop-Location
}
