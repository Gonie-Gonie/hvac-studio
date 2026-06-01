$ErrorActionPreference = 'Stop'

. (Join-Path $PSScriptRoot 'env.ps1')

if (-not $env:HVAC_STUDIO_PYTHON) {
  throw 'python was not found. Run scripts/dev/setup.ps1 first.'
}

$RepoRoot = Resolve-Path (Join-Path $PSScriptRoot '..\..')
Invoke-Checked $env:HVAC_STUDIO_PYTHON -m unittest discover -s (Join-Path $RepoRoot 'python\bcs_worker\tests')
Invoke-Checked $env:HVAC_STUDIO_PYTHON -m unittest discover -s (Join-Path $RepoRoot 'python\bcs_sdk\tests')
