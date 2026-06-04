param(
  [string]$Version = '',
  [string]$PortableZip = '',
  [switch]$KeepStage
)

$ErrorActionPreference = 'Stop'

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
. (Join-Path $RepoRoot 'scripts\release\package-common.ps1')

function Resolve-InstallerUpdateChannel {
  param([Parameter(Mandatory = $true)][string]$Version)

  if ($Version -match '-alpha([.-]|$)') {
    return 'alpha-portable'
  }
  if ($Version -match '-beta([.-]|$)') {
    return 'beta-installer'
  }
  if ($Version -match '-rc([.-]|$)') {
    return 'release-candidate'
  }
  if ($Version -match '-dev([.-]|$)') {
    return 'development'
  }
  return 'stable'
}

function Remove-PathWithRetry {
  param([Parameter(Mandatory = $true)][string]$Path)

  for ($Attempt = 1; $Attempt -le 5; $Attempt++) {
    try {
      Remove-Item -LiteralPath $Path -Recurse -Force -ErrorAction Stop
      return
    } catch {
      if (-not (Test-Path -LiteralPath $Path -ErrorAction SilentlyContinue)) {
        return
      }
      if ($Attempt -eq 5) {
        throw
      }
      Start-Sleep -Milliseconds (250 * $Attempt)
    }
  }
}

function Compress-InstallerStage {
  param(
    [Parameter(Mandatory = $true)][string]$Source,
    [Parameter(Mandatory = $true)][string]$Destination
  )

  if (Test-Path -LiteralPath $Destination -ErrorAction SilentlyContinue) {
    Remove-PathWithRetry -Path $Destination
  }

  Add-Type -AssemblyName System.IO.Compression
  Add-Type -AssemblyName System.IO.Compression.FileSystem
  $SourceRoot = (Resolve-Path -LiteralPath $Source).Path
  $EntryRoot = Split-Path -Leaf $SourceRoot
  $Zip = [System.IO.Compression.ZipFile]::Open($Destination, [System.IO.Compression.ZipArchiveMode]::Create)
  try {
    Get-ChildItem -LiteralPath $SourceRoot -Recurse -File | ForEach-Object {
      $Relative = $_.FullName.Substring($SourceRoot.Length).TrimStart('\', '/')
      $EntryName = ($EntryRoot + '/' + ($Relative -replace '\\', '/'))
      [System.IO.Compression.ZipFileExtensions]::CreateEntryFromFile($Zip, $_.FullName, $EntryName, [System.IO.Compression.CompressionLevel]::Optimal) | Out-Null
    }
  } finally {
    $Zip.Dispose()
    [GC]::Collect()
    [GC]::WaitForPendingFinalizers()
  }
}

$ResolvedVersion = Resolve-Version -Version $Version
$RuntimeId = 'windows-amd64'
$PackageName = "hvac-studio-$ResolvedVersion-$RuntimeId-installer"
$DistRoot = Join-Path $RepoRoot 'dist'
$StageParent = Join-Path $DistRoot ('.installer-stage-' + [Guid]::NewGuid().ToString('N'))
$StageRoot = Join-Path $StageParent $PackageName
$ZipPath = Join-Path $DistRoot "$PackageName.zip"

if (-not $PortableZip) {
  $PortableOutput = & (Join-Path $RepoRoot 'scripts\release\package-portable.ps1') -Version $ResolvedVersion
  $PortableZip = ($PortableOutput | Select-Object -Last 1)
}
if (-not (Test-Path -LiteralPath $PortableZip)) {
  throw "portable payload does not exist: $PortableZip"
}

$PayloadName = Split-Path -Leaf $PortableZip
$PayloadRelativePath = "payload/$PayloadName"
$UpdateChannel = Resolve-InstallerUpdateChannel -Version $ResolvedVersion

if (Test-Path -LiteralPath $ZipPath -ErrorAction SilentlyContinue) {
  Remove-PathWithRetry -Path $ZipPath
}
New-Item -ItemType Directory -Force -Path (Join-Path $StageRoot 'installer') | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $StageRoot 'payload') | Out-Null

Copy-Item -LiteralPath $PortableZip -Destination (Join-Path $StageRoot 'payload') -Force

$InstallerManifest = [ordered]@{
  schema = 'hvac-studio.installer.v1'
  package_name = $PackageName
  package_type = 'studio-installer'
  version = $ResolvedVersion
  runtime_id = $RuntimeId
  payload = [ordered]@{
    package_type = 'studio-portable'
    path = $PayloadRelativePath
    sha256 = (Get-FileHash -LiteralPath $PortableZip -Algorithm SHA256).Hash.ToLowerInvariant()
  }
  install = [ordered]@{
    default_scope = 'per-user'
    default_dir = '%LOCALAPPDATA%\Programs\HVAC Studio'
    admin_program_files_supported = $true
  }
  webview2 = [ordered]@{
    required = $true
    policy = 'Check Evergreen WebView2 runtime before launch; warn with remediation when missing.'
    remediation = 'Install Microsoft Edge WebView2 Evergreen Runtime, then launch HVAC Studio again.'
  }
  start_menu = [ordered]@{
    enabled_by_default = $true
    shortcut_name = 'HVAC Studio.lnk'
  }
  path_registration = [ordered]@{
    enabled_by_default = $false
    scope = 'user'
    target = 'bin'
  }
  file_association = [ordered]@{
    extension = '.bcsproj'
    enabled_by_default = $false
    supported = $false
    policy = 'Reserved until Studio accepts a project-file launch argument.'
  }
  update_policy = [ordered]@{
    channel = $UpdateChannel
    alpha = 'Portable zip remains the alpha distribution baseline.'
    beta = 'Installer bundles are beta preview artifacts.'
    stable = 'Stable installer updates require signing and release-note discipline.'
    automatic_updates = $false
  }
}

$InstallerManifest | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $StageRoot 'installer\installer-manifest.json') -Encoding UTF8
$Trust = Copy-ReleaseTrustAssets -RepoRoot $RepoRoot -StageRoot $StageRoot -PackageType 'studio-installer' -Version $ResolvedVersion

@'
param(
  [string]$InstallDir = '',
  [switch]$AddToPath,
  [switch]$AssociateBcsproj,
  [switch]$NoStartMenu,
  [switch]$PlanOnly
)

$ErrorActionPreference = 'Stop'

function Get-DefaultInstallDir {
  if ($env:LOCALAPPDATA) {
    return (Join-Path $env:LOCALAPPDATA 'Programs\HVAC Studio')
  }
  return (Join-Path $env:USERPROFILE 'HVAC Studio')
}

function Test-WebView2Runtime {
  $ClientId = '{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}'
  $RegistryPaths = @(
    "HKLM:\SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\$ClientId",
    "HKLM:\SOFTWARE\Microsoft\EdgeUpdate\Clients\$ClientId",
    "HKCU:\SOFTWARE\Microsoft\EdgeUpdate\Clients\$ClientId"
  )
  foreach ($Path in $RegistryPaths) {
    if (Test-Path -LiteralPath $Path -ErrorAction SilentlyContinue) {
      return $true
    }
  }

  $ProgramRoots = @($env:ProgramFiles, ${env:ProgramFiles(x86)}) | Where-Object { $_ }
  foreach ($Root in $ProgramRoots) {
    $WebViewRoot = Join-Path $Root 'Microsoft\EdgeWebView\Application'
    if (-not (Test-Path -LiteralPath $WebViewRoot -ErrorAction SilentlyContinue)) {
      continue
    }
    $Candidate = Get-ChildItem -LiteralPath $WebViewRoot -Directory -ErrorAction SilentlyContinue |
      ForEach-Object { Join-Path $_.FullName 'msedgewebview2.exe' } |
      Where-Object { Test-Path -LiteralPath $_ -ErrorAction SilentlyContinue } |
      Select-Object -First 1
    if ($Candidate) {
      return $true
    }
  }
  return $false
}

function Get-StartMenuShortcutPath {
  $Programs = [Environment]::GetFolderPath('Programs')
  if (-not $Programs) {
    return ''
  }
  return (Join-Path $Programs 'HVAC Studio.lnk')
}

function New-StartMenuShortcut {
  param([Parameter(Mandatory = $true)][string]$TargetInstallDir)

  $ShortcutPath = Get-StartMenuShortcutPath
  if (-not $ShortcutPath) {
    return ''
  }
  New-Item -ItemType Directory -Force -Path (Split-Path -Parent $ShortcutPath) | Out-Null
  $Shell = New-Object -ComObject WScript.Shell
  $Shortcut = $Shell.CreateShortcut($ShortcutPath)
  $Shortcut.TargetPath = Join-Path $TargetInstallDir 'HVAC Studio.exe'
  $Shortcut.WorkingDirectory = $TargetInstallDir
  $Shortcut.Description = 'HVAC Studio'
  $Shortcut.Save()
  return $ShortcutPath
}

function Add-UserPathEntry {
  param([Parameter(Mandatory = $true)][string]$Entry)

  $Current = [Environment]::GetEnvironmentVariable('PATH', 'User')
  $Parts = @()
  if ($Current) {
    $Parts = @($Current -split [IO.Path]::PathSeparator | Where-Object { $_ })
  }
  if ($Parts -notcontains $Entry) {
    $Parts += $Entry
    [Environment]::SetEnvironmentVariable('PATH', ($Parts -join [IO.Path]::PathSeparator), 'User')
  }
}

$ScriptRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$BundleRoot = Split-Path -Parent $ScriptRoot
$ManifestPath = Join-Path $ScriptRoot 'installer-manifest.json'
$Manifest = Get-Content -Raw -LiteralPath $ManifestPath | ConvertFrom-Json

if (-not $InstallDir) {
  $InstallDir = Get-DefaultInstallDir
}
$PayloadZip = Join-Path $BundleRoot ($Manifest.payload.path -replace '/', '\')
$BinPath = Join-Path $InstallDir 'bin'
$Plan = [ordered]@{
  schema = 'hvac-studio.installer.plan.v1'
  version = $Manifest.version
  update_channel = $Manifest.update_policy.channel
  install_dir = $InstallDir
  payload = $PayloadZip
  webview2 = [ordered]@{
    required = [bool]$Manifest.webview2.required
    present = (Test-WebView2Runtime)
    remediation = $Manifest.webview2.remediation
  }
  start_menu = [ordered]@{
    requested = -not [bool]$NoStartMenu
    shortcut_path = Get-StartMenuShortcutPath
  }
  path_registration = [ordered]@{
    requested = [bool]$AddToPath
    scope = $Manifest.path_registration.scope
    target = $BinPath
  }
  file_association = [ordered]@{
    requested = [bool]$AssociateBcsproj
    supported = [bool]$Manifest.file_association.supported
    extension = $Manifest.file_association.extension
    policy = $Manifest.file_association.policy
  }
}

if ($PlanOnly) {
  $Plan | ConvertTo-Json -Depth 8
  return
}

if (-not (Test-Path -LiteralPath $PayloadZip)) {
  throw "installer payload is missing: $PayloadZip"
}
if ($AssociateBcsproj -and -not [bool]$Manifest.file_association.supported) {
  throw $Manifest.file_association.policy
}
if (-not $Plan.webview2.present) {
  Write-Warning $Manifest.webview2.remediation
}

$TempRoot = Join-Path ([IO.Path]::GetTempPath()) ('hvac-studio-install-' + [Guid]::NewGuid().ToString('N'))
try {
  Expand-Archive -LiteralPath $PayloadZip -DestinationPath $TempRoot -Force
  $PayloadRoot = Get-ChildItem -LiteralPath $TempRoot -Directory | Select-Object -First 1
  if ($null -eq $PayloadRoot) {
    throw "installer payload did not expand to a package directory"
  }
  Remove-Item -LiteralPath $InstallDir -Recurse -Force -ErrorAction SilentlyContinue
  New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
  Get-ChildItem -LiteralPath $PayloadRoot.FullName -Force | Copy-Item -Destination $InstallDir -Recurse -Force
} finally {
  Remove-Item -LiteralPath $TempRoot -Recurse -Force -ErrorAction SilentlyContinue
}

if (-not $NoStartMenu) {
  $null = New-StartMenuShortcut -TargetInstallDir $InstallDir
}
if ($AddToPath) {
  Add-UserPathEntry -Entry $BinPath
}

[ordered]@{
  ok = $true
  install_dir = $InstallDir
  start_menu = -not [bool]$NoStartMenu
  path_registered = [bool]$AddToPath
  file_association = $false
} | ConvertTo-Json -Depth 4
'@ | Set-Content -LiteralPath (Join-Path $StageRoot 'installer\install.ps1') -Encoding UTF8

@'
param(
  [string]$InstallDir = '',
  [switch]$RemovePath,
  [switch]$RemoveAssociation
)

$ErrorActionPreference = 'Stop'

function Get-DefaultInstallDir {
  if ($env:LOCALAPPDATA) {
    return (Join-Path $env:LOCALAPPDATA 'Programs\HVAC Studio')
  }
  return (Join-Path $env:USERPROFILE 'HVAC Studio')
}

function Remove-UserPathEntry {
  param([Parameter(Mandatory = $true)][string]$Entry)

  $Current = [Environment]::GetEnvironmentVariable('PATH', 'User')
  if (-not $Current) {
    return
  }
  $Parts = @($Current -split [IO.Path]::PathSeparator | Where-Object { $_ -and $_ -ne $Entry })
  [Environment]::SetEnvironmentVariable('PATH', ($Parts -join [IO.Path]::PathSeparator), 'User')
}

if (-not $InstallDir) {
  $InstallDir = Get-DefaultInstallDir
}

$ShortcutPath = Join-Path ([Environment]::GetFolderPath('Programs')) 'HVAC Studio.lnk'
Remove-Item -LiteralPath $ShortcutPath -Force -ErrorAction SilentlyContinue

if ($RemovePath) {
  Remove-UserPathEntry -Entry (Join-Path $InstallDir 'bin')
}
if ($RemoveAssociation) {
  Remove-Item -LiteralPath 'HKCU:\Software\Classes\.bcsproj' -Recurse -Force -ErrorAction SilentlyContinue
  Remove-Item -LiteralPath 'HKCU:\Software\Classes\HVACStudio.Project' -Recurse -Force -ErrorAction SilentlyContinue
}

Remove-Item -LiteralPath $InstallDir -Recurse -Force -ErrorAction SilentlyContinue

[ordered]@{
  ok = $true
  install_dir = $InstallDir
} | ConvertTo-Json -Depth 4
'@ | Set-Content -LiteralPath (Join-Path $StageRoot 'installer\uninstall.ps1') -Encoding UTF8

$ReleaseManifest = [ordered]@{
  package_name = $PackageName
  package_type = 'studio-installer'
  version = $ResolvedVersion
  runtime_id = $RuntimeId
  installer_manifest = 'installer/installer-manifest.json'
  install_script = 'installer/install.ps1'
  uninstall_script = 'installer/uninstall.ps1'
  payload = $PayloadRelativePath
  update_channel = $UpdateChannel
  trust = $Trust
}
$ReleaseManifest | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $StageRoot 'release-manifest.json') -Encoding UTF8

@"
# HVAC Studio Windows Installer Bundle

Version: $ResolvedVersion
Runtime: $RuntimeId
Update channel: $UpdateChannel

This bundle installs the portable Studio payload with a per-user PowerShell
installer. The installer can create a Start Menu shortcut and can optionally add
the installed bin folder to the user PATH. The .bcsproj file association is
documented in the manifest but disabled until Studio supports project-file launch.

Preview the install plan without changing the machine:

    powershell -NoProfile -ExecutionPolicy Bypass -File .\installer\install.ps1 -PlanOnly

Install for the current user:

    powershell -NoProfile -ExecutionPolicy Bypass -File .\installer\install.ps1

Uninstall:

    powershell -NoProfile -ExecutionPolicy Bypass -File .\installer\uninstall.ps1
"@ | Set-Content -LiteralPath (Join-Path $StageRoot 'README.md') -Encoding UTF8

Compress-InstallerStage -Source $StageRoot -Destination $ZipPath

if (-not $KeepStage) {
  try {
    Start-Sleep -Milliseconds 500
    Remove-PathWithRetry -Path $StageParent
  } catch {
    Write-Warning "installer stage could not be removed; inspect and delete manually if needed: $StageParent"
  }
}

Write-Host "installer package: $ZipPath"
Write-Output $ZipPath
