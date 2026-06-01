$ErrorActionPreference = 'Stop'

$RepoRoot = Resolve-Path (Join-Path $PSScriptRoot '..\..')
. (Join-Path $RepoRoot 'scripts\dev\env.ps1')

if (-not $env:HVAC_STUDIO_GO) {
  throw 'go was not found. Run scripts/dev/setup.ps1 first.'
}

$OutDir = Join-Path $RepoRoot 'bin'
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null

Push-Location (Join-Path $RepoRoot 'tools\go')
try {
  Invoke-Checked $env:HVAC_STUDIO_GO @('build', '-o', (Join-Path $OutDir 'bcs-runner.exe'), '.\cmd\bcs-runner')
} finally {
  Pop-Location
}

Write-Host "runner built: $(Join-Path $OutDir 'bcs-runner.exe')"
