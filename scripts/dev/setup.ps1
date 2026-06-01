param(
  [switch]$Force
)

$ErrorActionPreference = 'Stop'

. (Join-Path $PSScriptRoot 'tool-versions.ps1')

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$ToolsRoot = Join-Path $RepoRoot '.repo_tools'
$GoRoot = Join-Path $ToolsRoot 'go'
$UvRoot = Join-Path $ToolsRoot 'uv'
$VenvRoot = Join-Path $RepoRoot '.venv'
$DownloadRoot = Join-Path $ToolsRoot 'downloads'
$UvPythonInstallDir = Join-Path $ToolsRoot 'python'

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

  $ReusableTempRoot = Join-Path $ToolsRoot '_tmp_go'
  $ReusableExtractedGo = Join-Path $ReusableTempRoot 'go'
  $TempRoot = Join-Path $ToolsRoot ('_tmp_go_' + [Guid]::NewGuid().ToString('N'))
  $Archive = Join-Path $DownloadRoot "go$($ToolVersions.Go).windows-amd64.zip"
  $Url = "https://go.dev/dl/go$($ToolVersions.Go).windows-amd64.zip"

  New-Item -ItemType Directory -Force -Path $DownloadRoot | Out-Null

  if (-not (Test-Path -LiteralPath $Archive) -or $Force) {
    Download-File -Url $Url -OutFile $Archive
  }

  $ExtractedGo = $ReusableExtractedGo
  if ((Test-Path -LiteralPath (Join-Path $ReusableExtractedGo 'src\sync\mutex.go')) -and -not $Force) {
    Write-Host "using existing extracted go archive: $ReusableExtractedGo"
  } else {
    New-Item -ItemType Directory -Force -Path $TempRoot | Out-Null
    $ExtractedGo = Join-Path $TempRoot 'go'
    Write-Host "extract go: $Archive"
    Expand-Zip -Archive $Archive -Destination $TempRoot
  }

  if (-not (Test-Path -LiteralPath $ExtractedGo)) {
    throw "go archive did not contain expected go directory: $Archive"
  }

  if (Test-Path -LiteralPath $GoRoot) {
    $TrashGo = Join-Path $ToolsRoot ('_trash_go_' + (Get-Date -Format 'yyyyMMddHHmmss'))
    Move-Item -LiteralPath $GoRoot -Destination $TrashGo -Force
    Write-Host "moved previous go installation to: $TrashGo"
  }

  Move-Item -LiteralPath $ExtractedGo -Destination $GoRoot -Force
  Remove-Item -LiteralPath (Split-Path -Parent $ExtractedGo) -Force -ErrorAction SilentlyContinue

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
    Remove-Item -LiteralPath $UvRoot -Recurse -Force -ErrorAction SilentlyContinue
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
  $env:UV_CACHE_DIR = Join-Path $ToolsRoot 'uv-cache'
  $env:UV_TOOL_DIR = Join-Path $ToolsRoot 'uv-tools'
  $env:UV_MANAGED_PYTHON = '1'

  New-Item -ItemType Directory -Force -Path $env:UV_PYTHON_INSTALL_DIR, $env:UV_CACHE_DIR, $env:UV_TOOL_DIR | Out-Null

  Write-Host "install python: $($ToolVersions.Python)"
  Invoke-Tool $UvExe python install $ToolVersions.Python --install-dir $env:UV_PYTHON_INSTALL_DIR --default

  if ((Test-Path -LiteralPath (Join-Path $VenvRoot 'Scripts\python.exe')) -and -not $Force) {
    Write-Host "venv already exists: $VenvRoot"
  } else {
    if ($Force) {
      Remove-Item -LiteralPath $VenvRoot -Recurse -Force -ErrorAction SilentlyContinue
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
