$ErrorActionPreference = 'Stop'

function Copy-Tree {
  param(
    [Parameter(Mandatory = $true)][string]$Source,
    [Parameter(Mandatory = $true)][string]$Destination
  )

  if (-not (Test-Path -LiteralPath $Source)) {
    throw "source path does not exist: $Source"
  }
  New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Destination) | Out-Null
  Copy-Item -LiteralPath $Source -Destination $Destination -Recurse -Force
}

function Resolve-Version {
  param([string]$Version)

  if ($Version) {
    return $Version
  }

  $Git = Get-Command git -ErrorAction SilentlyContinue
  if ($null -eq $Git) {
    return '0.1.0-dev'
  }

  $ExactTag = (& git describe --tags --exact-match 2>$null)
  if ($LASTEXITCODE -eq 0 -and $ExactTag) {
    return $ExactTag.TrimStart('v')
  }

  $ShortSha = (& git rev-parse --short HEAD 2>$null)
  if ($LASTEXITCODE -eq 0 -and $ShortSha) {
    return "0.1.0-dev-$ShortSha"
  }

  return '0.1.0-dev'
}

function Resolve-PythonRuntimeSource {
  param([Parameter(Mandatory = $true)][string]$RepoRoot)

  if ($env:HVAC_STUDIO_PYTHON -and (Test-Path -LiteralPath $env:HVAC_STUDIO_PYTHON)) {
    $BasePrefix = (& $env:HVAC_STUDIO_PYTHON -c 'import sys; print(sys.base_prefix)' 2>$null)
    if ($LASTEXITCODE -eq 0 -and $BasePrefix) {
      $BasePrefix = $BasePrefix.Trim()
      if (Test-Path -LiteralPath (Join-Path $BasePrefix 'python.exe')) {
        return (Resolve-Path -LiteralPath $BasePrefix).Path
      }
    }
  }

  $PythonInstallRoot = Join-Path $RepoRoot '.repo_tools\python'
  if (Test-Path -LiteralPath $PythonInstallRoot) {
    $Candidates = Get-ChildItem -LiteralPath $PythonInstallRoot -Directory |
      Where-Object { Test-Path -LiteralPath (Join-Path $_.FullName 'python.exe') } |
      Sort-Object Name -Descending
    if ($Candidates.Count -gt 0) {
      return $Candidates[0].FullName
    }
  }

  throw 'packaged Python runtime source was not found. Run scripts/dev/setup.ps1 first.'
}

function Copy-PackagedPythonRuntime {
  param(
    [Parameter(Mandatory = $true)][string]$RepoRoot,
    [Parameter(Mandatory = $true)][string]$Destination
  )

  $PythonSource = Resolve-PythonRuntimeSource -RepoRoot $RepoRoot
  Remove-Item -LiteralPath $Destination -Recurse -Force -ErrorAction SilentlyContinue
  Copy-Tree -Source $PythonSource -Destination $Destination

  $PythonExe = Join-Path $Destination 'python.exe'
  if (-not (Test-Path -LiteralPath $PythonExe)) {
    throw "packaged Python runtime is missing python.exe: $PythonExe"
  }
  & $PythonExe --version
  if ($LASTEXITCODE -ne 0) {
    throw "$PythonExe failed with exit code $LASTEXITCODE"
  }
}

function Remove-PythonCaches {
  param([Parameter(Mandatory = $true)][string]$Root)

  if (-not (Test-Path -LiteralPath $Root)) {
    return
  }

  Get-ChildItem -LiteralPath $Root -Recurse -Directory -Filter '__pycache__' -ErrorAction SilentlyContinue |
    Remove-Item -Recurse -Force -ErrorAction SilentlyContinue
  Get-ChildItem -LiteralPath $Root -Recurse -File -ErrorAction SilentlyContinue |
    Where-Object { $_.Extension -in @('.pyc', '.pyo') } |
    Remove-Item -Force -ErrorAction SilentlyContinue
}

function Get-MinimalPackagePath {
  param([Parameter(Mandatory = $true)][string]$PackageRoot)

  $WindowsSystem = Join-Path $env:SystemRoot 'System32'
  $WindowsRoot = $env:SystemRoot
  $PackageBin = Join-Path $PackageRoot 'bin'
  return (@($PackageBin, $WindowsSystem, $WindowsRoot) | Where-Object { $_ -and (Test-Path -LiteralPath $_) }) -join [IO.Path]::PathSeparator
}

function New-PackageTestRoot {
  param([Parameter(Mandatory = $true)][string]$Prefix)

  $Base = 'C:\tmp'
  try {
    New-Item -ItemType Directory -Force -Path $Base | Out-Null
  } catch {
    $Base = [IO.Path]::GetTempPath()
  }

  $Root = Join-Path $Base ($Prefix + '-' + [Guid]::NewGuid().ToString('N'))
  New-Item -ItemType Directory -Force -Path $Root | Out-Null
  return $Root
}
