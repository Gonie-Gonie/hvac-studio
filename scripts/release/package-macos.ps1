param(
  [string]$Version = '',
  [switch]$KeepStage
)

$ErrorActionPreference = 'Stop'

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
. (Join-Path $RepoRoot 'scripts\dev\env.ps1')
. (Join-Path $RepoRoot 'scripts\release\package-common.ps1')

$ResolvedVersion = Resolve-Version -Version $Version
$RuntimeId = 'macos-universal-experimental'
$PackageName = "hvac-studio-$ResolvedVersion-$RuntimeId"
$DistRoot = Join-Path $RepoRoot 'dist'
$StageRoot = Join-Path $DistRoot $PackageName
$ZipPath = Join-Path $DistRoot "$PackageName.zip"

Remove-Item -LiteralPath $StageRoot -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item -LiteralPath $ZipPath -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $StageRoot | Out-Null

Copy-Tree -Source (Join-Path $RepoRoot 'python\bcs_worker') -Destination (Join-Path $StageRoot 'python\bcs_worker')
Copy-Tree -Source (Join-Path $RepoRoot 'python\bcs_sdk') -Destination (Join-Path $StageRoot 'python\bcs_sdk')
Copy-Tree -Source (Join-Path $RepoRoot 'schema') -Destination (Join-Path $StageRoot 'schema')
Copy-Tree -Source (Join-Path $RepoRoot 'runtime') -Destination (Join-Path $StageRoot 'runtime')
$Documentation = Copy-DocumentationAssets -RepoRoot $RepoRoot -StageRoot $StageRoot -Version $ResolvedVersion
Copy-Tree -Source (Join-Path $RepoRoot 'examples') -Destination (Join-Path $StageRoot 'examples')
Copy-Tree -Source (Join-Path $RepoRoot 'templates') -Destination (Join-Path $StageRoot 'templates')
Copy-Tree -Source (Join-Path $RepoRoot 'README.md') -Destination (Join-Path $StageRoot 'README.md')
Copy-Tree -Source (Join-Path $RepoRoot 'CHANGELOG.md') -Destination (Join-Path $StageRoot 'CHANGELOG.md')
$Trust = Copy-ReleaseTrustAssets -RepoRoot $RepoRoot -StageRoot $StageRoot -PackageType 'studio-macos-experimental' -Version $ResolvedVersion

$MacRoot = Join-Path $StageRoot 'macos'
New-Item -ItemType Directory -Force -Path $MacRoot | Out-Null

$MacPlan = [ordered]@{
  schema = 'hvac-studio.macos-package-plan.v1'
  status = 'experimental'
  runtime_id = $RuntimeId
  minimum_target = 'macOS 13 Ventura or newer, Apple Silicon and Intel under review'
  host_requirement = 'Build and notarization must run on macOS with Xcode Command Line Tools.'
  supported_contents = @(
    'source-of-truth project files'
    'schemas'
    'Python worker and SDK source'
    'examples'
    'templates'
    'offline documentation'
  )
  required_tools = @(
    'PowerShell 7+',
    'Go matching go.mod',
    'Python 3.11+',
    'Xcode Command Line Tools',
    'Wails CLI for desktop app experiments'
  )
  platform_checks = @(
    './macos/check-prereqs.ps1'
    './macos/check-prereqs.sh'
  )
  target_entrypoints = [ordered]@{
    runner = 'bin/bcs-runner'
    env = 'bin/bcs-env'
    studio_app = 'HVAC Studio.app'
  }
  signing = [ordered]@{
    status = 'not-signed'
    requirement = 'Developer ID Application certificate required before public macOS distribution.'
  }
  notarization = [ordered]@{
    status = 'not-notarized'
    requirement = 'Apple notarization and stapling required before public macOS distribution.'
  }
  caveats = @(
    'This package is an experimental macOS packaging contract and support bundle, not a signed public macOS app.',
    'Windows portable, installer, and runtime packages remain the supported release artifacts.',
    'macOS binaries must be built and smoke-tested on macOS before any user-facing macOS release.',
    'No notarization, Gatekeeper compatibility, auto-update, or file association promise is made by this package.'
  )
}
$MacPlan | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $MacRoot 'package-plan.json') -Encoding UTF8

@'
param(
  [switch]$Json
)

$Checks = @()

function Add-Check {
  param([string]$Id, [string]$Label, [bool]$OK, [string]$Detail)
  $script:Checks += [ordered]@{ id = $Id; label = $Label; ok = $OK; detail = $Detail }
}

Add-Check -Id 'host_macos' -Label 'macOS host' -OK ([bool]$IsMacOS) -Detail $(if ($IsMacOS) { 'running on macOS' } else { 'not running on macOS' })
foreach ($Tool in @('go', 'python3', 'pwsh', 'xcodebuild')) {
  $Command = Get-Command $Tool -ErrorAction SilentlyContinue
  Add-Check -Id "tool_$Tool" -Label $Tool -OK ($null -ne $Command) -Detail $(if ($null -ne $Command) { $Command.Source } else { 'missing' })
}
$Wails = Get-Command wails -ErrorAction SilentlyContinue
Add-Check -Id 'tool_wails' -Label 'wails' -OK ($null -ne $Wails) -Detail $(if ($null -ne $Wails) { $Wails.Source } else { 'missing for desktop app builds' })

$OK = -not (@($Checks | Where-Object { -not $_.ok -and $_.id -ne 'tool_wails' }).Count)
$Result = [ordered]@{
  schema = 'hvac-studio.macos-prereq-check.v1'
  ok = $OK
  checks = $Checks
  caveat = 'Wails is optional for CLI/runtime experiments but required for desktop app packaging.'
}

if ($Json) {
  $Result | ConvertTo-Json -Depth 6
} else {
  $Result.checks | ForEach-Object { Write-Host "$($_.id): $($_.ok) $($_.detail)" }
  if (-not $Result.ok) { throw 'macOS prerequisites are incomplete' }
}
'@ | Set-Content -LiteralPath (Join-Path $MacRoot 'check-prereqs.ps1') -Encoding UTF8

@'
#!/usr/bin/env bash
set -euo pipefail

if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "host_macos=false"
  exit 1
fi

for tool in go python3 pwsh xcodebuild; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "missing tool: $tool"
    exit 1
  fi
done

if ! command -v wails >/dev/null 2>&1; then
  echo "warning: wails is missing; desktop app packaging cannot run"
fi

echo "macOS prerequisite check ok"
'@ | Set-Content -LiteralPath (Join-Path $MacRoot 'check-prereqs.sh') -Encoding UTF8

$Commit = ''
$GitCommand = Get-Command git -ErrorAction SilentlyContinue
if ($null -ne $GitCommand) {
  $Commit = (& git rev-parse HEAD 2>$null)
  if ($LASTEXITCODE -ne 0) {
    $Commit = ''
  }
}

$ReleaseManifest = [ordered]@{
  package_name = $PackageName
  package_type = 'studio-macos-experimental'
  version = $ResolvedVersion
  runtime_id = $RuntimeId
  primary_platform = 'macOS experimental'
  status = 'experimental-support-package'
  commit = $Commit
  built_at_utc = (Get-Date).ToUniversalTime().ToString('o')
  provenance = 'release-provenance.json'
  documentation = $Documentation
  trust = $Trust
  macos = [ordered]@{
    package_plan = 'macos/package-plan.json'
    prereq_check_powershell = 'macos/check-prereqs.ps1'
    prereq_check_shell = 'macos/check-prereqs.sh'
    signing_status = 'not-signed'
    notarization_status = 'not-notarized'
  }
  notes = @(
    'Experimental macOS packaging support bundle.',
    'Not a signed or notarized public macOS application.',
    'Use Windows portable/installer/runtime packages for supported releases.',
    'Run macos/check-prereqs.ps1 or macos/check-prereqs.sh on macOS before native packaging experiments.'
  )
}
$ReleaseManifest | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $StageRoot 'release-manifest.json') -Encoding UTF8

@"
# HVAC Studio Experimental macOS Package

Version: $ResolvedVersion
Runtime: $RuntimeId

This package records the experimental macOS packaging contract. It includes
source artifacts, schemas, examples, templates, offline documentation, release
trust files, and macOS prerequisite checks. It is not a signed or notarized
public macOS app.

Run prerequisite checks on macOS:

```powershell
pwsh ./macos/check-prereqs.ps1 -Json
```

or:

```bash
./macos/check-prereqs.sh
```

Before any public macOS build, create native macOS binaries on macOS, sign them
with a Developer ID Application certificate, notarize with Apple, staple the
ticket, and smoke-test the package on a clean macOS user account.
"@ | Set-Content -LiteralPath (Join-Path $StageRoot 'PACKAGE_README.md') -Encoding UTF8

Remove-PythonCaches -Root $StageRoot

Write-ReleaseProvenance `
  -RepoRoot $RepoRoot `
  -StageRoot $StageRoot `
  -PackageName $PackageName `
  -PackageType 'studio-macos-experimental' `
  -Version $ResolvedVersion `
  -RuntimeId $RuntimeId `
  -Documentation $Documentation

Write-ReleaseChecksums -StageRoot $StageRoot

Compress-Archive -LiteralPath $StageRoot -DestinationPath $ZipPath -Force

if (-not $KeepStage) {
  Remove-Item -LiteralPath $StageRoot -Recurse -Force -ErrorAction SilentlyContinue
}

Write-Host "experimental macOS package: $ZipPath"
Write-Output $ZipPath
