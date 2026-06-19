param(
  [switch]$DryRun,
  [switch]$Caches,
  [switch]$Inventory
)

$ErrorActionPreference = 'Stop'

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$Targets = @(
  'artifacts',
  'bin',
  'dist\build',
  'dist\docs',
  '.repo_tools\logs',
  '.repo_tools\release-build',
  '.repo_tools\smoke',
  '.repo_tools\studio-live',
  '.repo_tools\python\.temp',
  '.tmp'
)

$CacheTargets = @(
  '.repo_tools\downloads',
  '.repo_tools\go-cache',
  '.repo_tools\uv-cache',
  '.repo_tools\uv-tools'
)

$EmptyGeneratedDirectories = @(
  'app\studio',
  'app',
  'examples\006_optimization_case\parameter_sets'
)

$PythonCacheRoots = @(
  'docs',
  'examples',
  'python',
  'runtime',
  'schema',
  'scripts',
  'templates',
  'tests',
  'tools'
)

function Write-InventorySection {
  param(
    [Parameter(Mandatory = $true)][string]$Title,
    [Parameter(Mandatory = $true)][string[]]$Items
  )

  Write-Host $Title
  foreach ($Item in $Items) {
    Write-Host "  - $Item"
  }
}

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

if ($Inventory) {
  Write-Host "cleanup inventory for $RepoRoot"
  Write-Host ''
  Write-InventorySection -Title 'Generated paths removed by default:' -Items $Targets
  Write-Host ''
  Write-InventorySection -Title 'Repo-local caches removed only with -Caches:' -Items $CacheTargets
  Write-Host ''
  Write-InventorySection -Title 'Empty generated directories removed only when empty:' -Items $EmptyGeneratedDirectories
  Write-Host ''
  Write-InventorySection -Title 'Python cache/build scan roots:' -Items $PythonCacheRoots
  Write-Host ''
  Write-Host 'Final package zip files under dist\ are preserved.'
  exit 0
}

$Removed = New-Object System.Collections.Generic.List[string]
$Skipped = New-Object System.Collections.Generic.List[string]
$RemovedCaches = New-Object System.Collections.Generic.List[string]
$SkippedCaches = New-Object System.Collections.Generic.List[string]
$RemovedPythonCaches = New-Object System.Collections.Generic.List[string]
$RemovedPythonBuildArtifacts = New-Object System.Collections.Generic.List[string]
$RemovedEmptyGeneratedDirectories = New-Object System.Collections.Generic.List[string]
$SkippedNonEmptyGeneratedDirectories = New-Object System.Collections.Generic.List[string]
$PlannedEmptyGeneratedDirectoryRemovals = New-Object System.Collections.Generic.List[string]

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

if ($Caches) {
  foreach ($RelativePath in $CacheTargets) {
    $Resolved = Resolve-RepoTarget -RelativePath $RelativePath
    if (-not (Test-Path -LiteralPath $Resolved)) {
      $SkippedCaches.Add($RelativePath)
      continue
    }

    if ($DryRun) {
      $RemovedCaches.Add("$RelativePath (dry run)")
      continue
    }

    Remove-Item -LiteralPath $Resolved -Recurse -Force
    $RemovedCaches.Add($RelativePath)
  }
}

foreach ($RelativePath in $EmptyGeneratedDirectories) {
  $Resolved = Resolve-RepoTarget -RelativePath $RelativePath
  if (-not (Test-Path -LiteralPath $Resolved -PathType Container)) {
    $Skipped.Add($RelativePath)
    continue
  }

  $Children = @(Get-ChildItem -LiteralPath $Resolved -Force -ErrorAction SilentlyContinue)
  $BlockingChildren = @($Children | Where-Object {
      $ChildRelativePath = ConvertTo-RepoRelative -Path $_.FullName
      -not $PlannedEmptyGeneratedDirectoryRemovals.Contains($ChildRelativePath)
    })
  if ($BlockingChildren.Count -gt 0) {
    $SkippedNonEmptyGeneratedDirectories.Add($RelativePath)
    continue
  }

  $PlannedEmptyGeneratedDirectoryRemovals.Add($RelativePath)
  if ($DryRun) {
    $RemovedEmptyGeneratedDirectories.Add("$RelativePath (dry run)")
    continue
  }

  Remove-Item -LiteralPath $Resolved -Force
  $RemovedEmptyGeneratedDirectories.Add($RelativePath)
}

$PythonCaches = New-Object System.Collections.Generic.List[object]
foreach ($RelativePath in $PythonCacheRoots) {
  $ResolvedRoot = Resolve-RepoTarget -RelativePath $RelativePath
  if (-not (Test-Path -LiteralPath $ResolvedRoot -PathType Container)) {
    continue
  }
  Get-ChildItem -LiteralPath $ResolvedRoot -Recurse -Directory -Filter '__pycache__' -ErrorAction SilentlyContinue |
    ForEach-Object { $PythonCaches.Add($_) }
}
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
  if ($DryRun) {
    Write-Host "would remove generated paths: $($Removed -join ', ')"
  } else {
    Write-Host "removed generated paths: $($Removed -join ', ')"
  }
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

if ($RemovedCaches.Count -gt 0) {
  if ($DryRun) {
    Write-Host "would remove repo-local caches: $($RemovedCaches -join ', ')"
  } else {
    Write-Host "removed repo-local caches: $($RemovedCaches -join ', ')"
  }
} elseif ($Caches) {
  Write-Host 'no repo-local caches to remove'
} else {
  Write-Host 'preserved repo-local caches; pass -Caches to remove downloads, Go cache, uv cache, and uv tool storage'
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

if ($RemovedEmptyGeneratedDirectories.Count -gt 0) {
  if ($DryRun) {
    Write-Host "would remove empty generated directories: $($RemovedEmptyGeneratedDirectories -join ', ')"
  } else {
    Write-Host "removed empty generated directories: $($RemovedEmptyGeneratedDirectories -join ', ')"
  }
} else {
  Write-Host 'no empty generated directories to remove'
}

if ($SkippedNonEmptyGeneratedDirectories.Count -gt 0) {
  Write-Host "preserved non-empty generated directories: $($SkippedNonEmptyGeneratedDirectories -join ', ')"
}

if ($Skipped.Count -gt 0) {
  Write-Host "already clean: $($Skipped -join ', ')"
}

if ($Caches -and $SkippedCaches.Count -gt 0) {
  Write-Host "already clean caches: $($SkippedCaches -join ', ')"
}

$DistRoot = Join-Path $RepoRoot 'dist'
if (Test-Path -LiteralPath $DistRoot) {
  $ZipFiles = @(Get-ChildItem -LiteralPath $DistRoot -File -Filter '*.zip' | Sort-Object Name)
  if ($ZipFiles.Count -gt 0) {
    Write-Host "preserved dist zip artifacts: $($ZipFiles.Name -join ', ')"
  }
}
