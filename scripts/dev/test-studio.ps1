$ErrorActionPreference = 'Stop'

. (Join-Path $PSScriptRoot 'env.ps1')

if (-not $env:HVAC_STUDIO_GO) {
  throw 'go was not found. Run scripts/dev/setup.ps1 first.'
}

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$SmokeRoot = Join-Path $RepoRoot 'dist\build\latest\studio'
$StudioExe = Join-Path $SmokeRoot 'hvac-studio.exe'

Remove-Item -LiteralPath $SmokeRoot -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $SmokeRoot | Out-Null

Push-Location (Join-Path $RepoRoot 'tools\go')
try {
  Invoke-Checked $env:HVAC_STUDIO_GO @('test', '.\internal\studio', '.\cmd\studio')
  Invoke-Checked $env:HVAC_STUDIO_GO @('build', '-tags', 'desktop,production', '-ldflags', '-w -s -H=windowsgui', '-o', $StudioExe, '.\cmd\studio')
} finally {
  Pop-Location
}

Write-Host "Studio smoke build written to $StudioExe"
