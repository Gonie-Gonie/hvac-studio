param(
  [string]$Version = '',
  [switch]$KeepStage
)

$ErrorActionPreference = 'Stop'

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
. (Join-Path $RepoRoot 'scripts\dev\env.ps1')
. (Join-Path $RepoRoot 'scripts\release\package-common.ps1')

function Remove-PythonBuildOutputs {
  param([Parameter(Mandatory = $true)][string]$PackageRoot)

  Remove-Item -LiteralPath (Join-Path $PackageRoot 'build') -Recurse -Force -ErrorAction SilentlyContinue
  Get-ChildItem -LiteralPath $PackageRoot -Directory -Filter '*.egg-info' -ErrorAction SilentlyContinue |
    Remove-Item -Recurse -Force -ErrorAction SilentlyContinue
}

function Invoke-UVBuild {
  param(
    [Parameter(Mandatory = $true)][string]$PackageRoot,
    [Parameter(Mandatory = $true)][string]$OutputRoot
  )

  if (-not $env:HVAC_STUDIO_UV -or -not (Test-Path -LiteralPath $env:HVAC_STUDIO_UV)) {
    throw 'uv was not found. Run scripts/dev/setup.ps1 first.'
  }

  Remove-PythonBuildOutputs -PackageRoot $PackageRoot
  & $env:HVAC_STUDIO_UV build --sdist --wheel --no-create-gitignore --out-dir $OutputRoot $PackageRoot
  if ($LASTEXITCODE -ne 0) {
    throw "uv build failed for $PackageRoot with exit code $LASTEXITCODE"
  }
  Remove-PythonBuildOutputs -PackageRoot $PackageRoot
}

$ResolvedVersion = Resolve-Version -Version $Version
$PackageName = "hvac-studio-sdk-$ResolvedVersion"
$DistRoot = Join-Path $RepoRoot 'dist'
$StageRoot = Join-Path $DistRoot $PackageName
$ZipPath = Join-Path $DistRoot "$PackageName.zip"
$WheelRoot = Join-Path $StageRoot 'python\wheels'
$SdistRoot = Join-Path $StageRoot 'python\sdist'
$BuildRoot = Join-Path $StageRoot 'python\build'

Remove-Item -LiteralPath $StageRoot -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item -LiteralPath $ZipPath -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $WheelRoot, $SdistRoot, $BuildRoot | Out-Null

$PackageRoots = @(
  (Join-Path $RepoRoot 'python\bcs_sdk'),
  (Join-Path $RepoRoot 'python\bcs_worker')
)

foreach ($PackageRoot in $PackageRoots) {
  Invoke-UVBuild -PackageRoot $PackageRoot -OutputRoot $BuildRoot
}

$BuiltFiles = @(Get-ChildItem -LiteralPath $BuildRoot -File | Sort-Object Name)
foreach ($BuiltFile in $BuiltFiles) {
  if ($BuiltFile.Extension -eq '.whl') {
    Move-Item -LiteralPath $BuiltFile.FullName -Destination (Join-Path $WheelRoot $BuiltFile.Name) -Force
  } elseif ($BuiltFile.Name.EndsWith('.tar.gz', [System.StringComparison]::OrdinalIgnoreCase)) {
    Move-Item -LiteralPath $BuiltFile.FullName -Destination (Join-Path $SdistRoot $BuiltFile.Name) -Force
  }
}
Remove-Item -LiteralPath $BuildRoot -Recurse -Force -ErrorAction SilentlyContinue

$Wheels = @(Get-ChildItem -LiteralPath $WheelRoot -File -Filter '*.whl' | Sort-Object Name)
$Sdists = @(Get-ChildItem -LiteralPath $SdistRoot -File -Filter '*.tar.gz' | Sort-Object Name)
foreach ($RequiredPrefix in @('bcs_sdk-', 'bcs_worker-')) {
  if (-not @($Wheels | Where-Object { $_.Name.StartsWith($RequiredPrefix, [System.StringComparison]::OrdinalIgnoreCase) })) {
    throw "SDK package is missing wheel with prefix $RequiredPrefix"
  }
  if (-not @($Sdists | Where-Object { $_.Name.StartsWith($RequiredPrefix, [System.StringComparison]::OrdinalIgnoreCase) })) {
    throw "SDK package is missing source distribution with prefix $RequiredPrefix"
  }
}

Copy-Tree -Source (Join-Path $RepoRoot 'examples\sdk') -Destination (Join-Path $StageRoot 'examples\sdk')
Copy-Tree -Source (Join-Path $RepoRoot 'docs\user\python-sdk.md') -Destination (Join-Path $StageRoot 'docs\python-sdk.md')
Copy-Tree -Source (Join-Path $RepoRoot 'docs\user\external-engine-protocol.md') -Destination (Join-Path $StageRoot 'docs\external-engine-protocol.md')
$Trust = Copy-ReleaseTrustAssets -RepoRoot $RepoRoot -StageRoot $StageRoot -PackageType 'sdk' -Version $ResolvedVersion

$Manifest = [ordered]@{
  schema = 'hvac-studio.sdk-package.v1'
  package_name = $PackageName
  version = $ResolvedVersion
  built_at_utc = (Get-Date).ToUniversalTime().ToString('o')
  python_packages = [ordered]@{
    wheels = @($Wheels | ForEach-Object { "python/wheels/$($_.Name)" })
    source_distributions = @($Sdists | ForEach-Object { "python/sdist/$($_.Name)" })
  }
  docs = [ordered]@{
    python_sdk = 'docs/python-sdk.md'
    external_engine_protocol = 'docs/external-engine-protocol.md'
  }
  examples = [ordered]@{
    sdk = 'examples/sdk'
  }
  trust = $Trust
}
$Manifest | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $StageRoot 'sdk-package-manifest.json') -Encoding UTF8

$PackageReadme = @(
  '# HVAC Studio SDK Package'
  ''
  "Version: $ResolvedVersion"
  ''
  'This package contains Python wheels and source distributions for:'
  ''
  '- `bcs-sdk`: Python wrapper around `bcs-runner serve` and workflow commands.'
  '- `bcs-worker`: Python worker package used by the Go runner for user Python components.'
  ''
  'The SDK package also includes the raw JSONL serve example and SDK user guide excerpts.'
  ''
  'Inspect package contents:'
  ''
  '```powershell'
  'Get-Content .\sdk-package-manifest.json'
  'Get-ChildItem .\python\wheels'
  '```'
)
$PackageReadme | Set-Content -LiteralPath (Join-Path $StageRoot 'PACKAGE_README.md') -Encoding UTF8

Write-ReleaseProvenance `
  -RepoRoot $RepoRoot `
  -StageRoot $StageRoot `
  -PackageName $PackageName `
  -PackageType 'sdk' `
  -Version $ResolvedVersion `
  -RuntimeId 'python' `
  -Documentation ([ordered]@{
    source = 'docs'
    python_sdk = 'docs/python-sdk.md'
    external_engine_protocol = 'docs/external-engine-protocol.md'
  })

Write-ReleaseChecksums -StageRoot $StageRoot

Compress-Archive -LiteralPath $StageRoot -DestinationPath $ZipPath -Force

if (-not $KeepStage) {
  Remove-Item -LiteralPath $StageRoot -Recurse -Force -ErrorAction SilentlyContinue
}

Write-Host "sdk package: $ZipPath"
Write-Output $ZipPath
