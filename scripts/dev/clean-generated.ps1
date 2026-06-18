param(
  [switch]$DryRun
)

$ErrorActionPreference = 'Stop'

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$Targets = @(
  'artifacts',
  'bin',
  'dist\build',
  'dist\docs',
  '.tmp'
)

function Resolve-RepoTarget {
  param([Parameter(Mandatory = $true)][string]$RelativePath)

  $Target = Join-Path $RepoRoot $RelativePath
  $Resolved = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($Target)
  if (-not $Resolved.StartsWith($RepoRoot, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "refusing cleanup outside repo: $Resolved"
  }
  return $Resolved
}

function ConvertTo-RepoRelative {
  param([Parameter(Mandatory = $true)][string]$Path)

  if ($Path.Length -eq $RepoRoot.Length) {
    return '.'
  }
  return $Path.Substring($RepoRoot.Length).TrimStart(
    [System.IO.Path]::DirectorySeparatorChar,
    [System.IO.Path]::AltDirectorySeparatorChar
  )
}

$Removed = New-Object System.Collections.Generic.List[string]
$Skipped = New-Object System.Collections.Generic.List[string]
$RemovedPythonCaches = New-Object System.Collections.Generic.List[string]
$RemovedPythonBuildArtifacts = New-Object System.Collections.Generic.List[string]

foreach ($RelativePath in $Targets) {
  $Resolved = Resolve-RepoTarget -RelativePath $RelativePath
  if (-not (Test-Path -LiteralPath $Resolved)) {
    $Skipped.Add($RelativePath)
    continue
  }

  if ($DryRun) {
    $Removed.Add("$RelativePath (dry run)")
    continue
  }

  Remove-Item -LiteralPath $Resolved -Recurse -Force
  $Removed.Add($RelativePath)
}

$PythonCaches = @(Get-ChildItem -LiteralPath $RepoRoot -Recurse -Directory -Filter '__pycache__' -ErrorAction SilentlyContinue | Sort-Object FullName)
foreach ($PythonCache in $PythonCaches) {
  $Resolved = (Resolve-Path -LiteralPath $PythonCache.FullName).Path
  if (-not $Resolved.StartsWith($RepoRoot, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "refusing cleanup outside repo: $Resolved"
  }

  $RelativePath = ConvertTo-RepoRelative -Path $Resolved
  if ($DryRun) {
    $RemovedPythonCaches.Add("$RelativePath (dry run)")
    continue
  }

  Remove-Item -LiteralPath $Resolved -Recurse -Force
  $RemovedPythonCaches.Add($RelativePath)
}

$PythonRoot = Join-Path $RepoRoot 'python'
if (Test-Path -LiteralPath $PythonRoot) {
  $PythonBuildArtifacts = @(Get-ChildItem -LiteralPath $PythonRoot -Recurse -Directory -ErrorAction SilentlyContinue |
    Where-Object { $_.Name -eq 'build' -or $_.Name.EndsWith('.egg-info', [System.StringComparison]::OrdinalIgnoreCase) } |
    Sort-Object FullName)
  foreach ($PythonBuildArtifact in $PythonBuildArtifacts) {
    $Resolved = (Resolve-Path -LiteralPath $PythonBuildArtifact.FullName).Path
    if (-not $Resolved.StartsWith($RepoRoot, [System.StringComparison]::OrdinalIgnoreCase)) {
      throw "refusing cleanup outside repo: $Resolved"
    }

    $RelativePath = ConvertTo-RepoRelative -Path $Resolved
    if ($DryRun) {
      $RemovedPythonBuildArtifacts.Add("$RelativePath (dry run)")
      continue
    }

    Remove-Item -LiteralPath $Resolved -Recurse -Force
    $RemovedPythonBuildArtifacts.Add($RelativePath)
  }
}

if ($Removed.Count -gt 0) {
  Write-Host "removed generated paths: $($Removed -join ', ')"
} else {
  Write-Host 'no generated paths to remove'
}

if ($RemovedPythonCaches.Count -gt 0) {
  if ($DryRun) {
    Write-Host "would remove Python cache directories: $($RemovedPythonCaches.Count)"
  } else {
    Write-Host "removed Python cache directories: $($RemovedPythonCaches.Count)"
  }
} else {
  Write-Host 'no Python cache directories to remove'
}

if ($RemovedPythonBuildArtifacts.Count -gt 0) {
  if ($DryRun) {
    Write-Host "would remove Python build artifacts: $($RemovedPythonBuildArtifacts.Count)"
  } else {
    Write-Host "removed Python build artifacts: $($RemovedPythonBuildArtifacts.Count)"
  }
} else {
  Write-Host 'no Python build artifacts to remove'
}

if ($Skipped.Count -gt 0) {
  Write-Host "already clean: $($Skipped -join ', ')"
}

$DistRoot = Join-Path $RepoRoot 'dist'
if (Test-Path -LiteralPath $DistRoot) {
  $ZipFiles = @(Get-ChildItem -LiteralPath $DistRoot -File -Filter '*.zip' | Sort-Object Name)
  if ($ZipFiles.Count -gt 0) {
    Write-Host "preserved dist zip artifacts: $($ZipFiles.Name -join ', ')"
  }
}
