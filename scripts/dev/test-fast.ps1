$ErrorActionPreference = 'Stop'

. (Join-Path $PSScriptRoot 'env.ps1')

$RepoRoot = Resolve-Path (Join-Path $PSScriptRoot '..\..')

& (Join-Path $RepoRoot 'scripts\dev\test-go.ps1')
& (Join-Path $RepoRoot 'scripts\dev\test-studio.ps1')
& (Join-Path $RepoRoot 'scripts\dev\test-python.ps1')
& (Join-Path $RepoRoot 'scripts\dev\test-examples.ps1')
& (Join-Path $RepoRoot 'scripts\dev\test-validation.ps1')
& (Join-Path $RepoRoot 'scripts\dev\test-docs.ps1')

$Npm = Get-Command npm -ErrorAction SilentlyContinue
if ($null -ne $Npm -and (Test-Path (Join-Path $RepoRoot 'app\studio\frontend\package.json'))) {
  Push-Location (Join-Path $RepoRoot 'app\studio\frontend')
  try {
    npm run typecheck
    npm run test -- --run
  } finally {
    Pop-Location
  }
} else {
  Write-Host 'frontend checks skipped: npm or app/studio/frontend/package.json not found'
}
