param(
  [string]$Version = '',
  [switch]$SkipSetup,
  [switch]$SkipFast,
  [switch]$SkipScreenshots
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
$DocsZip = Join-Path $RepoRoot "dist\hvac-studio-docs-$ResolvedVersion.zip"
$SdkZip = Join-Path $RepoRoot "dist\hvac-studio-sdk-$ResolvedVersion.zip"

function Invoke-ReleaseStep {
  param(
    [Parameter(Mandatory = $true)][string]$Name,
    [Parameter(Mandatory = $true)][scriptblock]$Action
  )

  Write-Host ""
  Write-Host "== $Name =="
  & $Action
}

function Invoke-CheckedPowerShell {
  param(
    [Parameter(Mandatory = $true)][string]$Script,
    [string[]]$Arguments = @()
  )

  powershell -NoProfile -ExecutionPolicy Bypass -File $Script @Arguments
  if ($LASTEXITCODE -ne 0) {
    throw "$Script failed with exit code $LASTEXITCODE"
  }
}

Write-Host "HVAC Studio release candidate"
Write-Host "version: $ResolvedVersion"
Write-Host "repo: $RepoRoot"

if (-not $SkipSetup) {
  Invoke-ReleaseStep 'Bootstrap repo-local toolchain' {
    Invoke-CheckedPowerShell -Script (Join-Path $RepoRoot 'scripts\dev\setup.ps1')
  }
}

if (-not $SkipFast) {
  Invoke-ReleaseStep 'Run fast verification' {
    Invoke-CheckedPowerShell -Script (Join-Path $RepoRoot 'scripts\dev\test-fast.ps1')
  }
}

if (-not $SkipScreenshots) {
  Invoke-ReleaseStep 'Capture Studio screenshot matrix' {
    Invoke-CheckedPowerShell -Script (Join-Path $RepoRoot 'scripts\dev\test-screenshot-matrix.ps1')
  }
}

Invoke-ReleaseStep 'Run upgrade rehearsal' {
  Invoke-CheckedPowerShell -Script (Join-Path $RepoRoot 'scripts\release\test-upgrade-rehearsal.ps1') -Arguments @('-Version', $ResolvedVersion)
}

Invoke-ReleaseStep 'Build and smoke-test portable Studio package' {
  Invoke-CheckedPowerShell -Script (Join-Path $RepoRoot 'scripts\release\test-portable-package.ps1') -Arguments @('-Version', $ResolvedVersion)
}

Invoke-ReleaseStep 'Build and smoke-test Windows installer bundle' {
  Invoke-CheckedPowerShell -Script (Join-Path $RepoRoot 'scripts\release\test-installer-package.ps1') -Arguments @('-Version', $ResolvedVersion, '-PortableZip', $PortableZip)
}

Invoke-ReleaseStep 'Build and smoke-test runtime package' {
  Invoke-CheckedPowerShell -Script (Join-Path $RepoRoot 'scripts\release\test-runtime-package.ps1') -Arguments @('-Version', $ResolvedVersion)
}

Invoke-ReleaseStep 'Build and smoke-test experimental macOS package' {
  Invoke-CheckedPowerShell -Script (Join-Path $RepoRoot 'scripts\release\test-macos-package.ps1') -Arguments @('-Version', $ResolvedVersion)
}

Invoke-ReleaseStep 'Build and smoke-test documentation package' {
  Invoke-CheckedPowerShell -Script (Join-Path $RepoRoot 'scripts\release\test-docs-package.ps1') -Arguments @('-Version', $ResolvedVersion)
}

Invoke-ReleaseStep 'Build and smoke-test SDK package' {
  Invoke-CheckedPowerShell -Script (Join-Path $RepoRoot 'scripts\release\test-sdk-package.ps1') -Arguments @('-Version', $ResolvedVersion)
}

foreach ($Artifact in @($PortableZip, $InstallerZip, $RuntimeZip, $MacOSZip, $DocsZip, $SdkZip)) {
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
Write-Host "docs:     $DocsZip"
Write-Host "SDK:      $SdkZip"
Write-Output $PortableZip
Write-Output $InstallerZip
Write-Output $RuntimeZip
Write-Output $MacOSZip
Write-Output $DocsZip
Write-Output $SdkZip
