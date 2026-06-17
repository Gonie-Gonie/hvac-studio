param(
  [string]$Version = '',
  [string]$PackagePath = '',
  [string]$PortableZip = ''
)

$ErrorActionPreference = 'Stop'

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
. (Join-Path $RepoRoot 'scripts\release\package-common.ps1')

if (-not $env:HVAC_STUDIO_TEST_ROOT) {
  $env:HVAC_STUDIO_TEST_ROOT = Join-Path $RepoRoot 'artifacts\package-tests'
}

if (-not $PackagePath) {
  $PackageOutput = & (Join-Path $RepoRoot 'scripts\release\package-installer.ps1') -Version $Version -PortableZip $PortableZip
  $PackagePath = ($PackageOutput | Select-Object -Last 1)
}

if (-not (Test-Path -LiteralPath $PackagePath)) {
  throw "installer package does not exist: $PackagePath"
}

$TestRoot = New-PackageTestRoot -Prefix 'hvac-installer-test'

try {
  Expand-Archive -LiteralPath $PackagePath -DestinationPath $TestRoot -Force
  $PackageDir = Get-ChildItem -LiteralPath $TestRoot -Directory | Select-Object -First 1
  if ($null -eq $PackageDir) {
    throw "installer package did not expand to a directory: $PackagePath"
  }

  $ReleaseManifestPath = Join-Path $PackageDir.FullName 'release-manifest.json'
  $TrustPath = Join-Path $PackageDir.FullName 'release-trust.json'
  $InstallerManifestPath = Join-Path $PackageDir.FullName 'installer\installer-manifest.json'
  $InstallScript = Join-Path $PackageDir.FullName 'installer\install.ps1'
  $UninstallScript = Join-Path $PackageDir.FullName 'installer\uninstall.ps1'
  foreach ($RequiredPath in @($ReleaseManifestPath, $TrustPath, $InstallerManifestPath, $InstallScript, $UninstallScript)) {
    if (-not (Test-Path -LiteralPath $RequiredPath)) {
      throw "installer package is missing $RequiredPath"
    }
  }
  foreach ($RequiredTrustPath in @('legal\license-notices.md', 'legal\dependency-notices.md', 'legal\support-matrix.md', 'legal\release-notes-policy.md')) {
    if (-not (Test-Path -LiteralPath (Join-Path $PackageDir.FullName $RequiredTrustPath))) {
      throw "installer package is missing trust asset $RequiredTrustPath"
    }
  }

  $ReleaseManifest = Get-Content -Raw -LiteralPath $ReleaseManifestPath | ConvertFrom-Json
  $Trust = Get-Content -Raw -LiteralPath $TrustPath | ConvertFrom-Json
  $InstallerManifest = Get-Content -Raw -LiteralPath $InstallerManifestPath | ConvertFrom-Json
  if ($ReleaseManifest.package_type -ne 'studio-installer') {
    throw "release manifest package_type mismatch: $($ReleaseManifest.package_type)"
  }
  if ($Trust.schema -ne 'hvac-studio.release-trust.v1' -or $Trust.package_type -ne 'studio-installer') {
    throw "release trust mismatch: $($Trust.schema) $($Trust.package_type)"
  }
  if (-not $ReleaseManifest.trust -or $ReleaseManifest.trust.schema -ne 'hvac-studio.release-trust.v1') {
    throw 'installer release manifest is missing trust metadata'
  }
  if ($InstallerManifest.schema -ne 'hvac-studio.installer.v1') {
    throw "installer manifest schema mismatch: $($InstallerManifest.schema)"
  }
  if (-not $InstallerManifest.webview2.required) {
    throw 'installer manifest should require WebView2 runtime checks'
  }
  if (-not $InstallerManifest.start_menu.enabled_by_default) {
    throw 'installer manifest should enable Start Menu integration by default'
  }
  if ($InstallerManifest.path_registration.enabled_by_default) {
    throw 'installer manifest should keep PATH registration opt-in'
  }
  if ($InstallerManifest.file_association.supported) {
    throw 'installer file association should remain disabled until project-file launch is supported'
  }

  $PayloadPath = Join-Path $PackageDir.FullName (($InstallerManifest.payload.path -as [string]) -replace '/', '\')
  if (-not (Test-Path -LiteralPath $PayloadPath)) {
    throw "installer payload is missing: $PayloadPath"
  }
  $PayloadHash = (Get-FileHash -LiteralPath $PayloadPath -Algorithm SHA256).Hash.ToLowerInvariant()
  if ($PayloadHash -ne $InstallerManifest.payload.sha256) {
    throw 'installer payload checksum mismatch'
  }

  $PlanInstallDir = Join-Path $TestRoot 'install-plan'
  $PlanOutput = & powershell -NoProfile -ExecutionPolicy Bypass -File $InstallScript -PlanOnly -InstallDir $PlanInstallDir -AddToPath
  $Plan = ($PlanOutput -join "`n") | ConvertFrom-Json
  if ($Plan.schema -ne 'hvac-studio.installer.plan.v1') {
    throw "installer plan schema mismatch: $($Plan.schema)"
  }
  if ($Plan.install_dir -ne $PlanInstallDir) {
    throw "installer plan install_dir mismatch: $($Plan.install_dir)"
  }
  if (-not $Plan.start_menu.requested) {
    throw 'installer plan should request Start Menu shortcut by default'
  }
  if (-not $Plan.path_registration.requested) {
    throw 'installer plan should reflect requested PATH registration'
  }
  if ($Plan.file_association.supported) {
    throw 'installer plan should keep file association disabled'
  }

  $AssociationPlanOutput = & powershell -NoProfile -ExecutionPolicy Bypass -File $InstallScript -PlanOnly -InstallDir $PlanInstallDir -AssociateBcsproj
  $AssociationPlan = ($AssociationPlanOutput -join "`n") | ConvertFrom-Json
  if (-not $AssociationPlan.file_association.requested -or $AssociationPlan.file_association.supported) {
    throw 'installer association plan should report requested but unsupported .bcsproj association'
  }

  $PayloadTestRoot = Join-Path $TestRoot 'payload'
  Expand-Archive -LiteralPath $PayloadPath -DestinationPath $PayloadTestRoot -Force
  $PayloadDir = Get-ChildItem -LiteralPath $PayloadTestRoot -Directory | Select-Object -First 1
  if ($null -eq $PayloadDir) {
    throw 'installer payload did not expand to a portable package directory'
  }
  foreach ($RequiredPayloadPath in @('HVAC Studio.exe', 'bin\bcs-env.exe', 'bin\bcs-runner.exe', 'runtime\python\python.exe')) {
    if (-not (Test-Path -LiteralPath (Join-Path $PayloadDir.FullName $RequiredPayloadPath))) {
      throw "installer payload is missing portable file $RequiredPayloadPath"
    }
  }

  Write-Host "installer package smoke test ok: $PackagePath"
} finally {
  Remove-Item -LiteralPath $TestRoot -Recurse -Force -ErrorAction SilentlyContinue
}
