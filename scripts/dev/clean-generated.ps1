param(
  [switch]$DryRun,
  [switch]$Caches,
  [switch]$Inventory
)

$ErrorActionPreference = 'Stop'
$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$RepoPrefix = $RepoRoot.TrimEnd('\', '/') + [IO.Path]::DirectorySeparatorChar
$Targets = @(
  '.tmp', 'artifacts', 'bin', 'build', 'out', 'logs', 'outputs\generated',
  'runtime\python', 'runtime\packages', '.toolchain\python\.temp',
  '.pytest_cache', '.mypy_cache', '.ruff_cache', '.coverage', 'coverage.out'
)
$CacheTargets = @('.cache')
$SourceRoots = @('docs', 'examples', 'go', 'python', 'schema', 'scripts', 'templates', 'tests')
$PythonPackages = @('python\bcs_sdk', 'python\bcs_worker')
$CleanupTargets = New-Object 'System.Collections.Generic.List[string]'

function Resolve-RepoTarget {
  param([Parameter(Mandatory = $true)][string]$RelativePath)

  $Target = $RelativePath
  if (-not [IO.Path]::IsPathRooted($Target)) { $Target = Join-Path $RepoRoot $Target }
  $Resolved = [IO.Path]::GetFullPath($Target)
  if (-not $Resolved.StartsWith($RepoPrefix, [StringComparison]::OrdinalIgnoreCase)) {
    throw "refusing cleanup outside repo: $Resolved"
  }
  return $Resolved
}

function Assert-NoReparseAncestors {
  param([Parameter(Mandatory = $true)][string]$Path)

  $Current = Resolve-RepoTarget -RelativePath $Path
  while ($Current.Length -gt $RepoRoot.Length) {
    if (Test-Path -LiteralPath $Current) {
      $Item = Get-Item -LiteralPath $Current -Force
      if ($Item.Attributes -band [IO.FileAttributes]::ReparsePoint) {
        throw "refusing cleanup through reparse point: $Current"
      }
    }
    $Current = Split-Path -Parent $Current
  }
}

function Assert-NoReparseTree {
  param([Parameter(Mandatory = $true)][string]$Path)

  Assert-NoReparseAncestors -Path $Path
  if (-not (Test-Path -LiteralPath $Path -PathType Container)) { return }
  $Pending = New-Object 'System.Collections.Generic.Stack[string]'
  $Pending.Push($Path)
  while ($Pending.Count -gt 0) {
    foreach ($Child in @(Get-ChildItem -LiteralPath $Pending.Pop() -Force)) {
      if ($Child.Attributes -band [IO.FileAttributes]::ReparsePoint) {
        throw "refusing cleanup of a tree with reparse point: $($Child.FullName)"
      }
      if ($Child.PSIsContainer) { $Pending.Push($Child.FullName) }
    }
  }
}

function Add-CleanupTarget {
  param([Parameter(Mandatory = $true)][string]$RelativePath)

  $Resolved = Resolve-RepoTarget -RelativePath $RelativePath
  # Check parents even for absent targets: an external junction must never be traversed.
  Assert-NoReparseAncestors -Path $Resolved
  if (-not (Test-Path -LiteralPath $Resolved)) { return }
  foreach ($Existing in $CleanupTargets) {
    if ($Resolved -eq $Existing -or $Resolved.StartsWith(($Existing + [IO.Path]::DirectorySeparatorChar), [StringComparison]::OrdinalIgnoreCase)) {
      return
    }
  }
  $CleanupTargets.Add($Resolved)
}

if ($Inventory) {
  Write-Host 'Generated paths removed by default:'
  $Targets | ForEach-Object { Write-Host "  - $_" }
  Write-Host '  - dist/* except dist/latest/ (the ready-to-run portable build)'
  Write-Host '  - Python bytecode and tool caches in source directories'
  Write-Host '  - Python package build/dist/*.egg-info directories'
  Write-Host '  - examples/*/runs and empty legacy app directories'
  Write-Host 'Pass -Caches to remove .cache/ (Go, uv, and downloaded archives).'
  Write-Host 'Preserved: .toolchain/, .venv/, dist/latest/, user projects, fixtures, and golden outputs.'
  return
}

foreach ($RelativePath in $Targets) { Add-CleanupTarget -RelativePath $RelativePath }
if ($Caches) {
  foreach ($RelativePath in $CacheTargets) { Add-CleanupTarget -RelativePath $RelativePath }
}

# Keep the whole portable directory, including any user projects saved beside the executable.
$DistRoot = Resolve-RepoTarget -RelativePath 'dist'
Assert-NoReparseAncestors -Path $DistRoot
if (Test-Path -LiteralPath $DistRoot -PathType Container) {
  foreach ($Child in @(Get-ChildItem -LiteralPath $DistRoot -Force)) {
    if ($Child.Name -ne 'latest') { Add-CleanupTarget -RelativePath $Child.FullName }
  }
}

# Walk source directories without following junctions or symbolic links.
foreach ($RelativePath in $SourceRoots) {
  $SourceRoot = Resolve-RepoTarget -RelativePath $RelativePath
  Assert-NoReparseAncestors -Path $SourceRoot
  if (-not (Test-Path -LiteralPath $SourceRoot -PathType Container)) { continue }
  $Pending = New-Object 'System.Collections.Generic.Stack[string]'
  $Pending.Push($SourceRoot)
  while ($Pending.Count -gt 0) {
    foreach ($Child in @(Get-ChildItem -LiteralPath $Pending.Pop() -Force)) {
      if ($Child.Attributes -band [IO.FileAttributes]::ReparsePoint) { continue }
      if ($Child.PSIsContainer) {
        if ($Child.Name -in @('__pycache__', '.pytest_cache', '.mypy_cache', '.ruff_cache')) {
          Add-CleanupTarget -RelativePath $Child.FullName
        } else {
          $Pending.Push($Child.FullName)
        }
      } elseif ($Child.Extension -in @('.pyc', '.pyo')) {
        Add-CleanupTarget -RelativePath $Child.FullName
      }
    }
  }
}

# Restrict packaging cleanup to package roots, never arbitrary fixture folders named build.
foreach ($RelativePath in $PythonPackages) {
  $PackageRoot = Resolve-RepoTarget -RelativePath $RelativePath
  Assert-NoReparseAncestors -Path $PackageRoot
  if (-not (Test-Path -LiteralPath $PackageRoot -PathType Container)) { continue }
  foreach ($Child in @(Get-ChildItem -LiteralPath $PackageRoot -Directory -Force)) {
    if ($Child.Name -in @('build', 'dist') -or $Child.Name.EndsWith('.egg-info', [StringComparison]::OrdinalIgnoreCase)) {
      Add-CleanupTarget -RelativePath $Child.FullName
    }
  }
}

$ExamplesRoot = Resolve-RepoTarget -RelativePath 'examples'
if (Test-Path -LiteralPath $ExamplesRoot -PathType Container) {
  foreach ($Example in @(Get-ChildItem -LiteralPath $ExamplesRoot -Directory -Force)) {
    if ($Example.Attributes -band [IO.FileAttributes]::ReparsePoint) { continue }
    Add-CleanupTarget -RelativePath (Join-Path $Example.FullName 'runs')
  }
}

# Validate every tree before the first deletion so unsafe targets cannot cause partial cleanup.
foreach ($Path in $CleanupTargets) { Assert-NoReparseTree -Path $Path }
foreach ($Path in $CleanupTargets) {
  $RelativePath = $Path.Substring($RepoPrefix.Length)
  if ($DryRun) {
    Write-Host "would remove: $RelativePath"
  } else {
    Remove-Item -LiteralPath $Path -Recurse -Force
    Write-Host "removed: $RelativePath"
  }
}

$EmptyDirectories = @(
  'app\studio', 'app', 'examples\006_optimization_case\parameter_sets',
  'docs\adr', 'docs\maintainer', 'docs\user\assets\tutorials', 'docs\user\assets'
)
$EmptyPlanned = New-Object 'System.Collections.Generic.List[string]'
foreach ($RelativePath in $EmptyDirectories) {
  $Path = Resolve-RepoTarget -RelativePath $RelativePath
  Assert-NoReparseAncestors -Path $Path
  if (-not (Test-Path -LiteralPath $Path -PathType Container)) { continue }
  $Remaining = @(Get-ChildItem -LiteralPath $Path -Force | Where-Object { -not $EmptyPlanned.Contains($_.FullName) })
  if ($Remaining.Count -gt 0) { continue }
  $EmptyPlanned.Add($Path)
  if ($DryRun) {
    Write-Host "would remove empty directory: $RelativePath"
  } else {
    Remove-Item -LiteralPath $Path -Force
    Write-Host "removed empty directory: $RelativePath"
  }
}

if ($CleanupTargets.Count -eq 0 -and $EmptyPlanned.Count -eq 0) { Write-Host 'no generated files to remove' }
if (-not $Caches) { Write-Host 'preserved .cache/; pass -Caches to remove disposable caches' }
Write-Host 'preserved .toolchain/, .venv/, dist/latest/, user projects, fixtures, and golden outputs'
