param(
  [switch]$Force
)

$ErrorActionPreference = 'Stop'

. (Join-Path $PSScriptRoot 'tool-versions.ps1')

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$ToolsRoot = Join-Path $RepoRoot '.toolchain'
$CacheRoot = Join-Path $RepoRoot '.cache'
$SetupRoot = Join-Path $RepoRoot '.tmp\setup'
$GoRoot = Join-Path $ToolsRoot 'go'
$UvRoot = Join-Path $ToolsRoot 'uv'
$VenvRoot = Join-Path $RepoRoot '.venv'
$DownloadRoot = Join-Path $CacheRoot 'downloads'
$UvPythonInstallDir = Join-Path $ToolsRoot 'python'

function Assert-WorkspacePath {
  param([Parameter(Mandatory = $true)][string]$Path)

  $Resolved = [IO.Path]::GetFullPath($Path)
  $Prefix = $RepoRoot.TrimEnd('\', '/') + [IO.Path]::DirectorySeparatorChar
  if (-not $Resolved.StartsWith($Prefix, [StringComparison]::OrdinalIgnoreCase)) {
    throw "refusing toolchain operation outside repo: $Resolved"
  }
  $Current = $Resolved
  while ($Current.Length -gt $RepoRoot.Length) {
    if (Test-Path -LiteralPath $Current) {
      $Item = Get-Item -LiteralPath $Current -Force
      if ($Item.Attributes -band [IO.FileAttributes]::ReparsePoint) {
        throw "refusing toolchain operation through reparse point: $Current"
      }
    }
    $Current = Split-Path -Parent $Current
  }
  return $Resolved
}

function Assert-WorkspaceTree {
  param([Parameter(Mandatory = $true)][string]$Path)

  $Resolved = Assert-WorkspacePath -Path $Path
  if (-not (Test-Path -LiteralPath $Resolved -PathType Container)) { return }
  $Pending = New-Object 'System.Collections.Generic.Stack[string]'
  $Pending.Push($Resolved)
  while ($Pending.Count -gt 0) {
    foreach ($Child in @(Get-ChildItem -LiteralPath $Pending.Pop() -Force)) {
      if ($Child.Attributes -band [IO.FileAttributes]::ReparsePoint) {
        throw "refusing toolchain operation on a tree with reparse point: $($Child.FullName)"
      }
      if ($Child.PSIsContainer) { $Pending.Push($Child.FullName) }
    }
  }
}

function Remove-WorkspaceItem {
  param([Parameter(Mandatory = $true)][string]$Path)

  $Resolved = Assert-WorkspacePath -Path $Path
  if (Test-Path -LiteralPath $Resolved) {
    Assert-WorkspaceTree -Path $Resolved
    Remove-Item -LiteralPath $Resolved -Recurse -Force
  }
}

function Move-WorkspaceItem {
  param(
    [Parameter(Mandatory = $true)][string]$Source,
    [Parameter(Mandatory = $true)][string]$Destination
  )

  $ResolvedSource = Assert-WorkspacePath -Path $Source
  $ResolvedDestination = Assert-WorkspacePath -Path $Destination
  Assert-WorkspaceTree -Path $ResolvedSource
  Move-Item -LiteralPath $ResolvedSource -Destination $ResolvedDestination
}

function Assert-WindowsAmd64 {
  if (-not $IsWindows -and $PSVersionTable.PSEdition -eq 'Core') {
    throw 'scripts/dev/setup.ps1 currently bootstraps Windows toolchains only.'
  }
  $Arch = $env:PROCESSOR_ARCHITECTURE
  if ($Arch -notin @('AMD64', 'x86_64')) {
    throw "unsupported Windows architecture for this bootstrap script: $Arch"
  }
}

function Download-File {
  param(
    [Parameter(Mandatory = $true)][string]$Url,
    [Parameter(Mandatory = $true)][string]$OutFile
  )

  Write-Host "download: $Url"
  Invoke-WebRequest -Uri $Url -OutFile $OutFile
}

function Invoke-Tool {
  param(
    [Parameter(Mandatory = $true)][string]$FilePath,
    [Parameter(ValueFromRemainingArguments = $true)][string[]]$Arguments
  )

  & $FilePath @Arguments
  if ($LASTEXITCODE -ne 0) {
    throw "$FilePath failed with exit code $LASTEXITCODE"
  }
}

function Expand-Zip {
  param(
    [Parameter(Mandatory = $true)][string]$Archive,
    [Parameter(Mandatory = $true)][string]$Destination
  )

  $Tar = Get-Command tar -ErrorAction SilentlyContinue
  if ($null -ne $Tar) {
    Invoke-Tool $Tar.Source -xf $Archive -C $Destination
    return
  }

  Expand-Archive -LiteralPath $Archive -DestinationPath $Destination -Force
}

function Install-Go {
  $GoExe = Join-Path $GoRoot 'bin\go.exe'
  $GoMarker = Join-Path $GoRoot 'src\sync\mutex.go'
  if ((Test-Path -LiteralPath $GoExe) -and (Test-Path -LiteralPath $GoMarker) -and -not $Force) {
    Write-Host "go already installed: $GoExe"
    return
  }

  $TempRoot = Join-Path $SetupRoot ('go-' + [Guid]::NewGuid().ToString('N'))
  $Archive = Join-Path $DownloadRoot "go$($ToolVersions.Go).windows-amd64.zip"
  $Url = "https://go.dev/dl/go$($ToolVersions.Go).windows-amd64.zip"

  New-Item -ItemType Directory -Force -Path $DownloadRoot | Out-Null

  if (-not (Test-Path -LiteralPath $Archive) -or $Force) {
    Download-File -Url $Url -OutFile $Archive
  }

  $null = Assert-WorkspacePath -Path $TempRoot
  New-Item -ItemType Directory -Force -Path $TempRoot | Out-Null
  $ExtractedGo = Join-Path $TempRoot 'go'
  Write-Host "extract go: $Archive"
  Expand-Zip -Archive $Archive -Destination $TempRoot
  if (-not (Test-Path -LiteralPath (Join-Path $ExtractedGo 'src\sync\mutex.go'))) {
    throw "go archive did not contain a complete go installation: $Archive"
  }

  Assert-WorkspaceTree -Path $ExtractedGo
  Invoke-Tool (Join-Path $ExtractedGo 'bin\go.exe') version
  Remove-WorkspaceItem -Path $GoRoot
  Move-WorkspaceItem -Source $ExtractedGo -Destination $GoRoot
  Remove-WorkspaceItem -Path $TempRoot

  if (-not (Test-Path -LiteralPath $GoMarker)) {
    throw "go installation appears incomplete; missing $GoMarker"
  }

  Invoke-Tool $GoExe version
}

function Install-Uv {
  $UvExe = Join-Path $UvRoot 'uv.exe'
  if ((Test-Path -LiteralPath $UvExe) -and -not $Force) {
    Write-Host "uv already installed: $UvExe"
    return
  }

  if ($Force) {
    Remove-WorkspaceItem -Path $UvRoot
  }
  New-Item -ItemType Directory -Force -Path $UvRoot | Out-Null

  $env:UV_UNMANAGED_INSTALL = $UvRoot
  $env:UV_NO_MODIFY_PATH = '1'
  $InstallUrl = "https://astral.sh/uv/$($ToolVersions.Uv)/install.ps1"
  Write-Host "install uv: $InstallUrl"
  Invoke-Expression (Invoke-RestMethod -Uri $InstallUrl)

  if (-not (Test-Path -LiteralPath $UvExe)) {
    throw "uv installer completed but uv.exe was not found at $UvExe"
  }
  Invoke-Tool $UvExe --version
}

function Install-Python {
  $UvExe = Join-Path $UvRoot 'uv.exe'
  if (-not (Test-Path -LiteralPath $UvExe)) {
    throw 'uv must be installed before Python can be bootstrapped'
  }

  $env:UV_PYTHON_INSTALL_DIR = $UvPythonInstallDir
  $env:UV_CACHE_DIR = Join-Path $CacheRoot 'uv'
  $env:UV_TOOL_DIR = Join-Path $ToolsRoot 'uv-tools'
  $env:UV_MANAGED_PYTHON = '1'

  New-Item -ItemType Directory -Force -Path $env:UV_PYTHON_INSTALL_DIR, $env:UV_CACHE_DIR, $env:UV_TOOL_DIR | Out-Null

  Write-Host "install python: $($ToolVersions.Python)"
  Invoke-Tool $UvExe python install $ToolVersions.Python --install-dir $env:UV_PYTHON_INSTALL_DIR --no-bin --no-registry

  if ((Test-Path -LiteralPath (Join-Path $VenvRoot 'Scripts\python.exe')) -and -not $Force) {
    Write-Host "venv already exists: $VenvRoot"
  } else {
    if ($Force) {
      Remove-WorkspaceItem -Path $VenvRoot
    }
    Invoke-Tool $UvExe venv $VenvRoot --python $ToolVersions.Python --managed-python
  }

  $VenvPython = Join-Path $VenvRoot 'Scripts\python.exe'
  if (-not (Test-Path -LiteralPath $VenvPython)) {
    throw "venv python was not found at $VenvPython"
  }
  Invoke-Tool $VenvPython --version
}

Assert-WindowsAmd64
New-Item -ItemType Directory -Force -Path $ToolsRoot, $DownloadRoot | Out-Null

Install-Go
Install-Uv
Install-Python

. (Join-Path $PSScriptRoot 'env.ps1')

Write-Host ''
Write-Host 'repo-local development environment is ready'
Write-Host "go:     $env:HVAC_STUDIO_GO"
Write-Host "python: $env:HVAC_STUDIO_PYTHON"
Write-Host "uv:     $env:HVAC_STUDIO_UV"
Write-Host ''
Write-Host 'next: powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\dev\test-fast.ps1'

exit 0
