param(
  [string]$Version = '',
  [string]$PackagePath = ''
)

$ErrorActionPreference = 'Stop'

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
. (Join-Path $RepoRoot 'scripts\dev\env.ps1')
. (Join-Path $RepoRoot 'scripts\release\package-common.ps1')

if (-not $PackagePath) {
  $PackageOutput = & (Join-Path $RepoRoot 'scripts\release\package-docs.ps1') -Version $Version
  $PackagePath = ($PackageOutput | Select-Object -Last 1)
}

if (-not (Test-Path -LiteralPath $PackagePath)) {
  throw "documentation package does not exist: $PackagePath"
}

$TestRoot = New-PackageTestRoot -Prefix 'hvac-docs-test'

try {
  Expand-Archive -LiteralPath $PackagePath -DestinationPath $TestRoot -Force
  $PackageDir = Get-ChildItem -LiteralPath $TestRoot -Directory | Select-Object -First 1
  if ($null -eq $PackageDir) {
    throw "documentation package did not expand to a directory: $PackagePath"
  }

  Assert-ReleaseProvenance -PackageRoot $PackageDir.FullName -PackageType 'docs' -Version $Version

  foreach ($RequiredPath in @(
    'docs\site\index.html',
    'docs\site\status\index.html',
    'docs\manual\hvac-studio-manual.md',
    'docs\manual\hvac-studio-manual.pdf',
    'docs\manual\manual-build.json',
    'docs\version.json',
    'docs\site\version.json',
    'PACKAGE_README.md'
  )) {
    if (-not (Test-Path -LiteralPath (Join-Path $PackageDir.FullName $RequiredPath))) {
      throw "documentation package is missing $RequiredPath"
    }
  }

  $ManualStatus = Get-Content -Raw -LiteralPath (Join-Path $PackageDir.FullName 'docs\manual\manual-build.json') -Encoding UTF8 | ConvertFrom-Json
  if ($ManualStatus.pdf_status -ne 'built') {
    throw "documentation package PDF was not built: $($ManualStatus.pdf_status) $($ManualStatus.pdf_reason)"
  }

  Write-Host "documentation package smoke test ok: $PackagePath"
} finally {
  Remove-Item -LiteralPath $TestRoot -Recurse -Force -ErrorAction SilentlyContinue
}
