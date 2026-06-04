$ErrorActionPreference = 'Stop'

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$ToolsRoot = Join-Path $RepoRoot '.repo_tools'
$VenvRoot = Join-Path $RepoRoot '.venv'
$GoRoot = Join-Path $ToolsRoot 'go'
$UvRoot = Join-Path $ToolsRoot 'uv'
$GoCacheRoot = Join-Path $ToolsRoot 'go-cache'
$UvCacheRoot = Join-Path $ToolsRoot 'uv-cache'
$UvToolRoot = Join-Path $ToolsRoot 'uv-tools'
$UvPythonInstallDir = Join-Path $ToolsRoot 'python'

function Add-PathFirst {
  param([Parameter(Mandatory = $true)][string]$Path)

  if (-not (Test-Path -LiteralPath $Path)) {
    return
  }

  $Parts = @()
  if ($env:PATH) {
    $Parts = $env:PATH -split [IO.Path]::PathSeparator
  }
  $Filtered = $Parts | Where-Object {
    $_ -and -not ([string]::Equals($_, $Path, [System.StringComparison]::OrdinalIgnoreCase))
  }
  $env:PATH = (@($Path) + $Filtered) -join [IO.Path]::PathSeparator
}

function Resolve-Executable {
  param(
    [Parameter(Mandatory = $true)][string]$Preferred,
    [Parameter(Mandatory = $true)][string]$FallbackName
  )

  if (Test-Path -LiteralPath $Preferred) {
    return (Resolve-Path -LiteralPath $Preferred).Path
  }

  $Command = Get-Command $FallbackName -ErrorAction SilentlyContinue
  if ($null -eq $Command) {
    return ''
  }
  return $Command.Source
}

function Invoke-Checked {
  param(
    [Parameter(Mandatory = $true)][string]$FilePath,
    [string[]]$Arguments = @()
  )

  & $FilePath @Arguments
  if ($LASTEXITCODE -ne 0) {
    throw "$FilePath failed with exit code $LASTEXITCODE"
  }
}

New-Item -ItemType Directory -Force -Path $ToolsRoot, $GoCacheRoot, $UvCacheRoot, $UvToolRoot, $UvPythonInstallDir | Out-Null

Add-PathFirst (Join-Path $VenvRoot 'Scripts')
Add-PathFirst (Join-Path $GoRoot 'bin')
Add-PathFirst $UvRoot

$env:HVAC_STUDIO_REPO_ROOT = $RepoRoot
$env:HVAC_STUDIO_TOOLS_ROOT = $ToolsRoot
$env:HVAC_STUDIO_TMP = Join-Path $RepoRoot '.tmp'
$env:GOMODCACHE = Join-Path $GoCacheRoot 'pkg\mod'
$env:GOCACHE = Join-Path $GoCacheRoot 'build'
$env:UV_CACHE_DIR = $UvCacheRoot
$env:UV_TOOL_DIR = $UvToolRoot
$env:UV_PYTHON_INSTALL_DIR = $UvPythonInstallDir
$env:UV_MANAGED_PYTHON = '1'

New-Item -ItemType Directory -Force -Path $env:GOMODCACHE, $env:GOCACHE, $env:HVAC_STUDIO_TMP | Out-Null

$env:HVAC_STUDIO_GO = Resolve-Executable `
  -Preferred (Join-Path $GoRoot 'bin\go.exe') `
  -FallbackName 'go'

$env:HVAC_STUDIO_UV = Resolve-Executable `
  -Preferred (Join-Path $UvRoot 'uv.exe') `
  -FallbackName 'uv'

$env:HVAC_STUDIO_PYTHON = Resolve-Executable `
  -Preferred (Join-Path $VenvRoot 'Scripts\python.exe') `
  -FallbackName 'python'

$PythonPathEntries = @(
  (Join-Path $RepoRoot 'python\bcs_worker')
  (Join-Path $RepoRoot 'python\bcs_sdk')
)
if ($env:PYTHONPATH) {
  $PythonPathEntries += ($env:PYTHONPATH -split [IO.Path]::PathSeparator)
}
$env:PYTHONPATH = ($PythonPathEntries | Where-Object { $_ } | Select-Object -Unique) -join [IO.Path]::PathSeparator
