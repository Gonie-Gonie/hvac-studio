param(
  [string]$Version = '',
  [string]$PackagePath = ''
)

$ErrorActionPreference = 'Stop'

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
. (Join-Path $RepoRoot 'scripts\dev\env.ps1')
. (Join-Path $RepoRoot 'scripts\release\package-common.ps1')

if (-not $PackagePath) {
  $PackageOutput = & (Join-Path $RepoRoot 'scripts\release\build-sdk.ps1') -Version $Version
  $PackagePath = ($PackageOutput | Select-Object -Last 1)
}

if (-not (Test-Path -LiteralPath $PackagePath)) {
  throw "SDK package does not exist: $PackagePath"
}

function Assert-PackageChecksums {
  param([Parameter(Mandatory = $true)][string]$PackageRoot)

  $ChecksumsPath = Join-Path $PackageRoot 'release-checksums.json'
  if (-not (Test-Path -LiteralPath $ChecksumsPath)) {
    throw "SDK package is missing $ChecksumsPath"
  }
  $Checksums = Get-Content -Raw -LiteralPath $ChecksumsPath -Encoding UTF8 | ConvertFrom-Json
  if ($Checksums.schema -ne 'hvac-studio.release-checksums.v1') {
    throw "SDK package checksum schema mismatch: $($Checksums.schema)"
  }
  foreach ($Entry in @($Checksums.files)) {
    $Path = Join-Path $PackageRoot (($Entry.path -as [string]) -replace '/', '\')
    if (-not (Test-Path -LiteralPath $Path)) {
      throw "SDK checksum path is missing: $($Entry.path)"
    }
    $Actual = (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($Actual -ne $Entry.sha256) {
      throw "SDK checksum mismatch for $($Entry.path)"
    }
  }
}

function Expand-WheelForImport {
  param(
    [Parameter(Mandatory = $true)][string]$WheelPath,
    [Parameter(Mandatory = $true)][string]$Destination
  )

  New-Item -ItemType Directory -Force -Path $Destination | Out-Null
  $TemporaryZip = Join-Path ([IO.Path]::GetTempPath()) ("hvac-studio-wheel-$([Guid]::NewGuid().ToString('N')).zip")
  try {
    Copy-Item -LiteralPath $WheelPath -Destination $TemporaryZip -Force
    Expand-Archive -LiteralPath $TemporaryZip -DestinationPath $Destination -Force
  } finally {
    Remove-Item -LiteralPath $TemporaryZip -Force -ErrorAction SilentlyContinue
  }
}

$TestRoot = New-PackageTestRoot -Prefix 'hvac-sdk-test'

try {
  Expand-Archive -LiteralPath $PackagePath -DestinationPath $TestRoot -Force
  $PackageDir = Get-ChildItem -LiteralPath $TestRoot -Directory | Select-Object -First 1
  if ($null -eq $PackageDir) {
    throw "SDK package did not expand to a directory: $PackagePath"
  }

  $PackageRoot = $PackageDir.FullName
  foreach ($RequiredPath in @(
    'sdk-package-manifest.json',
    'release-provenance.json',
    'release-checksums.json',
    'release-trust.json',
    'PACKAGE_README.md',
    'docs\python-sdk.md',
    'docs\external-engine-protocol.md',
    'examples\sdk\README.md',
    'legal\dependency-notices.md'
  )) {
    if (-not (Test-Path -LiteralPath (Join-Path $PackageRoot $RequiredPath))) {
      throw "SDK package is missing $RequiredPath"
    }
  }

  $Manifest = Get-Content -Raw -LiteralPath (Join-Path $PackageRoot 'sdk-package-manifest.json') -Encoding UTF8 | ConvertFrom-Json
  if ($Manifest.schema -ne 'hvac-studio.sdk-package.v1') {
    throw "SDK package manifest schema mismatch: $($Manifest.schema)"
  }
  if ($Version -and $Manifest.version -ne $Version) {
    throw "SDK package version mismatch: $($Manifest.version)"
  }

  $Provenance = Get-Content -Raw -LiteralPath (Join-Path $PackageRoot 'release-provenance.json') -Encoding UTF8 | ConvertFrom-Json
  if ($Provenance.schema -ne 'hvac-studio.release-provenance.v1' -or $Provenance.package_type -ne 'sdk') {
    throw "SDK package provenance mismatch: $($Provenance.schema) $($Provenance.package_type)"
  }
  $Trust = Get-Content -Raw -LiteralPath (Join-Path $PackageRoot 'release-trust.json') -Encoding UTF8 | ConvertFrom-Json
  if ($Trust.schema -ne 'hvac-studio.release-trust.v1' -or $Trust.package_type -ne 'sdk') {
    throw "SDK package trust mismatch: $($Trust.schema) $($Trust.package_type)"
  }
  Assert-PackageChecksums -PackageRoot $PackageRoot

  $WheelRoot = Join-Path $PackageRoot 'python\wheels'
  $SdistRoot = Join-Path $PackageRoot 'python\sdist'
  $SdkWheel = Get-ChildItem -LiteralPath $WheelRoot -File -Filter 'bcs_sdk-*.whl' | Select-Object -First 1
  $WorkerWheel = Get-ChildItem -LiteralPath $WheelRoot -File -Filter 'bcs_worker-*.whl' | Select-Object -First 1
  $SdkSdist = Get-ChildItem -LiteralPath $SdistRoot -File -Filter 'bcs_sdk-*.tar.gz' | Select-Object -First 1
  $WorkerSdist = Get-ChildItem -LiteralPath $SdistRoot -File -Filter 'bcs_worker-*.tar.gz' | Select-Object -First 1
  foreach ($RequiredPackage in @($SdkWheel, $WorkerWheel, $SdkSdist, $WorkerSdist)) {
    if ($null -eq $RequiredPackage) {
      throw 'SDK package is missing a required wheel or source distribution'
    }
  }

  $ImportRoot = Join-Path $TestRoot 'wheel-import'
  $SdkImportRoot = Join-Path $ImportRoot 'bcs_sdk'
  $WorkerImportRoot = Join-Path $ImportRoot 'bcs_worker'
  Expand-WheelForImport -WheelPath $SdkWheel.FullName -Destination $SdkImportRoot
  Expand-WheelForImport -WheelPath $WorkerWheel.FullName -Destination $WorkerImportRoot

  $ImportCode = @'
import bcs_sdk
import bcs_worker
from bcs_sdk import RunnerClient, RunnerPool
assert bcs_sdk
assert bcs_worker.__all__
assert RunnerClient
assert RunnerPool
'@
  $PreviousPythonPath = $env:PYTHONPATH
  try {
    $env:PYTHONPATH = (@($SdkImportRoot, $WorkerImportRoot, $PreviousPythonPath) | Where-Object { $_ }) -join [IO.Path]::PathSeparator
    Invoke-Checked $env:HVAC_STUDIO_PYTHON @('-c', $ImportCode)
  } finally {
    $env:PYTHONPATH = $PreviousPythonPath
  }

  Write-Host "SDK package smoke test ok: $PackagePath"
} finally {
  Remove-Item -LiteralPath $TestRoot -Recurse -Force -ErrorAction SilentlyContinue
}
