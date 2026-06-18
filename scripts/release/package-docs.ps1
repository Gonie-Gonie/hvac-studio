param(
  [string]$Version = '',
  [switch]$KeepStage
)

$ErrorActionPreference = 'Stop'

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
. (Join-Path $RepoRoot 'scripts\dev\env.ps1')
. (Join-Path $RepoRoot 'scripts\release\package-common.ps1')

$ResolvedVersion = Resolve-Version -Version $Version
$RuntimeId = 'docs'
$PackageName = "hvac-studio-docs-$ResolvedVersion"
$DistRoot = Join-Path $RepoRoot 'dist'
$StageRoot = Join-Path $DistRoot $PackageName
$ZipPath = Join-Path $DistRoot "$PackageName.zip"

Remove-Item -LiteralPath $StageRoot -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item -LiteralPath $ZipPath -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $StageRoot | Out-Null

$Documentation = Copy-DocumentationAssets -RepoRoot $RepoRoot -StageRoot $StageRoot -Version $ResolvedVersion
$Trust = Copy-ReleaseTrustAssets -RepoRoot $RepoRoot -StageRoot $StageRoot -PackageType 'docs' -Version $ResolvedVersion

$ReleaseManifest = [ordered]@{
  package_name = $PackageName
  package_type = 'docs'
  version = $ResolvedVersion
  runtime_id = $RuntimeId
  primary_platform = 'all'
  status = 'offline-documentation-package'
  built_at_utc = (Get-Date).ToUniversalTime().ToString('o')
  provenance = 'release-provenance.json'
  documentation = $Documentation
  trust = $Trust
  entrypoints = [ordered]@{
    html = 'docs/site/index.html'
    markdown_manual = 'docs/manual/hvac-studio-manual.md'
    pdf_manual = 'docs/manual/hvac-studio-manual.pdf'
  }
  notes = @(
    'Offline HTML documentation and consolidated manual/PDF package.',
    'Use docs/site/index.html for browser reading.',
    'PDF generation uses pandoc when available and falls back to a plain-text PDF asset otherwise.'
  )
}
$ReleaseManifest | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $StageRoot 'release-manifest.json') -Encoding UTF8

@"
# HVAC Studio Documentation Package

Version: $ResolvedVersion

Open the offline HTML guide:

```text
docs/site/index.html
```

The consolidated manual lives under:

```text
docs/manual/hvac-studio-manual.md
docs/manual/hvac-studio-manual.pdf
docs/manual/manual-build.json
```

Release trust metadata and SHA-256 checksums are included at the package root.
"@ | Set-Content -LiteralPath (Join-Path $StageRoot 'PACKAGE_README.md') -Encoding UTF8

Write-ReleaseProvenance `
  -RepoRoot $RepoRoot `
  -StageRoot $StageRoot `
  -PackageName $PackageName `
  -PackageType 'docs' `
  -Version $ResolvedVersion `
  -RuntimeId $RuntimeId `
  -Documentation $Documentation

Write-ReleaseChecksums -StageRoot $StageRoot

Compress-Archive -LiteralPath $StageRoot -DestinationPath $ZipPath -Force

if (-not $KeepStage) {
  Remove-Item -LiteralPath $StageRoot -Recurse -Force -ErrorAction SilentlyContinue
}

Write-Host "documentation package: $ZipPath"
Write-Output $ZipPath
