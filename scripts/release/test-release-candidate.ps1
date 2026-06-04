param(
  [string]$Version = '',
  [switch]$SkipSetup,
  [switch]$SkipFast
)

$ErrorActionPreference = 'Stop'

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
. (Join-Path $RepoRoot 'scripts\release\package-common.ps1')

$ResolvedVersion = Resolve-Version -Version $Version
$RuntimeId = 'windows-amd64'
$PortableZip = Join-Path $RepoRoot "dist\hvac-studio-$ResolvedVersion-$RuntimeId-portable.zip"
$InstallerZip = Join-Path $RepoRoot "dist\hvac-studio-$ResolvedVersion-$RuntimeId-installer.zip"
$RuntimeZip = Join-Path $RepoRoot "dist\hvac-studio-runtime-$ResolvedVersion-$RuntimeId.zip"
$MacOSZip = Join-Path $RepoRoot "dist\hvac-studio-$ResolvedVersion-macos-universal-experimental.zip"

function Invoke-ReleaseStep {
  param(
    [Parameter(Mandatory = $true)][string]$Name,
    [Parameter(Mandatory = $true)][scriptblock]$Action
  )

  Write-Host ""
  Write-Host "== $Name =="
  & $Action
}

Write-Host "HVAC Studio release candidate"
Write-Host "version: $ResolvedVersion"
Write-Host "repo: $RepoRoot"

if (-not $SkipSetup) {
  Invoke-ReleaseStep 'Bootstrap repo-local toolchain' {
    powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $RepoRoot 'scripts\dev\setup.ps1')
  }
}

if (-not $SkipFast) {
  Invoke-ReleaseStep 'Run fast verification' {
    powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $RepoRoot 'scripts\dev\test-fast.ps1')
  }
}

Invoke-ReleaseStep 'Run upgrade rehearsal' {
  powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $RepoRoot 'scripts\release\test-upgrade-rehearsal.ps1') -Version $ResolvedVersion
}

Invoke-ReleaseStep 'Build and smoke-test portable Studio package' {
  powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $RepoRoot 'scripts\release\test-portable-package.ps1') -Version $ResolvedVersion
}

Invoke-ReleaseStep 'Build and smoke-test Windows installer bundle' {
  powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $RepoRoot 'scripts\release\test-installer-package.ps1') -Version $ResolvedVersion -PortableZip $PortableZip
}

Invoke-ReleaseStep 'Build and smoke-test runtime package' {
  powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $RepoRoot 'scripts\release\test-runtime-package.ps1') -Version $ResolvedVersion
}

Invoke-ReleaseStep 'Build and smoke-test experimental macOS package' {
  powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $RepoRoot 'scripts\release\test-macos-package.ps1') -Version $ResolvedVersion
}

foreach ($Artifact in @($PortableZip, $InstallerZip, $RuntimeZip, $MacOSZip)) {
  if (-not (Test-Path -LiteralPath $Artifact)) {
    throw "release candidate artifact was not written: $Artifact"
  }
}

Write-Host ""
Write-Host 'release candidate ok'
Write-Host "portable: $PortableZip"
Write-Host "installer: $InstallerZip"
Write-Host "runtime:  $RuntimeZip"
Write-Host "macOS:    $MacOSZip"
Write-Output $PortableZip
Write-Output $InstallerZip
Write-Output $RuntimeZip
Write-Output $MacOSZip
