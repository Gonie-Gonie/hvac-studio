param(
  [string]$Version = '',
  [string]$PackagePath = ''
)

$ErrorActionPreference = 'Stop'

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
. (Join-Path $RepoRoot 'scripts\dev\env.ps1')
. (Join-Path $RepoRoot 'scripts\release\package-common.ps1')

if (-not $PackagePath) {
  $PackageOutput = & (Join-Path $RepoRoot 'scripts\release\package-macos.ps1') -Version $Version
  $PackagePath = ($PackageOutput | Select-Object -Last 1)
}

if (-not (Test-Path -LiteralPath $PackagePath)) {
  throw "package does not exist: $PackagePath"
}

$TestRoot = New-PackageTestRoot -Prefix 'hvac-macos-test'

try {
  Expand-Archive -LiteralPath $PackagePath -DestinationPath $TestRoot -Force
  $PackageDir = Get-ChildItem -LiteralPath $TestRoot -Directory | Select-Object -First 1
  if ($null -eq $PackageDir) {
    throw "package did not expand to a directory: $PackagePath"
  }

  Assert-ReleaseProvenance -PackageRoot $PackageDir.FullName -PackageType 'studio-macos-experimental' -Version $Version

  $Manifest = Get-Content -Raw -LiteralPath (Join-Path $PackageDir.FullName 'release-manifest.json') | ConvertFrom-Json
  if ($Manifest.package_type -ne 'studio-macos-experimental') {
    throw "macOS package type mismatch: $($Manifest.package_type)"
  }
  if ($Manifest.macos.signing_status -ne 'not-signed' -or $Manifest.macos.notarization_status -ne 'not-notarized') {
    throw 'macOS package must explicitly record unsigned/not-notarized status'
  }

  $PlanPath = Join-Path $PackageDir.FullName 'macos\package-plan.json'
  $CheckPS1 = Join-Path $PackageDir.FullName 'macos\check-prereqs.ps1'
  $CheckSH = Join-Path $PackageDir.FullName 'macos\check-prereqs.sh'
  foreach ($RequiredPath in @($PlanPath, $CheckPS1, $CheckSH, (Join-Path $PackageDir.FullName 'docs\site\index.html'))) {
    if (-not (Test-Path -LiteralPath $RequiredPath)) {
      throw "experimental macOS package is missing $RequiredPath"
    }
  }

  $Plan = Get-Content -Raw -LiteralPath $PlanPath | ConvertFrom-Json
  if ($Plan.schema -ne 'hvac-studio.macos-package-plan.v1') {
    throw "macOS package plan schema mismatch: $($Plan.schema)"
  }
  if ($Plan.signing.status -ne 'not-signed' -or $Plan.notarization.status -ne 'not-notarized') {
    throw 'macOS package plan must document signing and notarization caveats'
  }
  if (-not (@($Plan.platform_checks) -contains './macos/check-prereqs.ps1')) {
    throw 'macOS package plan is missing PowerShell prereq check'
  }
  if (-not (@($Plan.caveats) -match 'not a signed public macOS app|notarization|Gatekeeper')) {
    throw 'macOS package plan caveats do not explain public distribution limits'
  }

  $PrereqJson = & $CheckPS1 -Json | ConvertFrom-Json
  if ($PrereqJson.schema -ne 'hvac-studio.macos-prereq-check.v1') {
    throw "macOS prereq check schema mismatch: $($PrereqJson.schema)"
  }
  if (-not (@($PrereqJson.checks | ForEach-Object { $_.id }) -contains 'host_macos')) {
    throw 'macOS prereq check does not report host_macos'
  }

  Write-Host "experimental macOS package smoke test ok: $PackagePath"
} finally {
  Remove-Item -LiteralPath $TestRoot -Recurse -Force -ErrorAction SilentlyContinue
}
