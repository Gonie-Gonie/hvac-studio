$ErrorActionPreference = 'Stop'
$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$TestRoot = [IO.Path]::GetFullPath((Join-Path $RepoRoot ('.tmp\cleanup-test-' + [Guid]::NewGuid().ToString('N'))))
$TestPrefix = $RepoRoot.TrimEnd('\', '/') + [IO.Path]::DirectorySeparatorChar
if (-not $TestRoot.StartsWith($TestPrefix, [StringComparison]::OrdinalIgnoreCase)) {
  throw "unsafe cleanup fixture location: $TestRoot"
}
$Current = Split-Path -Parent $TestRoot
while ($Current.Length -gt $RepoRoot.Length) {
  if ((Test-Path -LiteralPath $Current) -and ((Get-Item -LiteralPath $Current -Force).Attributes -band [IO.FileAttributes]::ReparsePoint)) {
    throw "unsafe cleanup fixture parent: $Current"
  }
  $Current = Split-Path -Parent $Current
}
$FixtureRoot = Join-Path $TestRoot 'repo'
$FixtureClean = Join-Path $FixtureRoot 'scripts\dev\clean-generated.ps1'
$SiblingRoot = Join-Path $TestRoot 'repo-sibling'
$JunctionPath = Join-Path $FixtureRoot '.tmp\external'

function Write-FixtureFile {
  param([string]$RelativePath)

  $Path = Join-Path $FixtureRoot $RelativePath
  New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Path) | Out-Null
  [IO.File]::WriteAllText($Path, "fixture: $RelativePath")
}

function Assert-FixtureFiles {
  param([string[]]$Paths, [bool]$Exist)

  foreach ($RelativePath in $Paths) {
    if ((Test-Path -LiteralPath (Join-Path $FixtureRoot $RelativePath)) -ne $Exist) {
      throw "unexpected cleanup result (expected exists=$Exist): $RelativePath"
    }
  }
}

function Get-FixtureSnapshot {
  return (@(Get-ChildItem -LiteralPath $FixtureRoot -Recurse -File -Force |
    Sort-Object FullName | ForEach-Object {
      "$($_.FullName):$((Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash)"
    }) -join "`n")
}

try {
  New-Item -ItemType Directory -Force -Path (Split-Path -Parent $FixtureClean), $SiblingRoot | Out-Null
  Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'clean-generated.ps1') -Destination $FixtureClean
  $Generated = @(
    '.tmp\build\studio.exe', 'artifacts\smoke.log', 'bin\runner.exe', 'out\result.json',
    'build\old.exe', 'logs\studio.log', 'runtime\packages\worker.whl',
    'dist\old-release.zip', 'dist\build\old.exe', 'dist\other\old.zip',
    'python\bcs_worker\__pycache__\worker.pyc', 'python\bcs_worker\orphan.pyc',
    'python\bcs_worker\build\lib\worker.py', 'python\bcs_sdk\dist\sdk.whl',
    'python\bcs_sdk\bcs_sdk.egg-info\PKG-INFO',
    'examples\001_scalar_component\runs\result.json', '.pytest_cache\state',
    '.toolchain\python\.temp\partial-download'
  )
  $Retained = @(
    'dist\latest\HVAC Studio.exe', 'dist\latest\bin\bcs-runner.exe',
    'dist\latest\runtime\python\python.exe', 'dist\latest\projects\mine\graph.json',
    '.toolchain\go\bin\go.exe', '.toolchain\uv-tools\mkdocs\tool.exe',
    '.venv\Scripts\python.exe', 'projects\mine\outputs\result.json',
    'tests\golden\result.json', 'tests\fixtures\build\expected.json',
    'python\bcs_worker\tests\fixtures\build\expected.json',
    'examples\001_scalar_component\graph.json', 'examples\001_scalar_component\outputs\expected.csv',
    'templates\projects\default\graph.json'
  )
  $CacheFiles = @('.cache\go\build\cache', '.cache\uv\wheel', '.cache\downloads\go.zip')
  foreach ($RelativePath in ($Generated + $Retained + $CacheFiles)) { Write-FixtureFile $RelativePath }
  New-Item -ItemType Directory -Force -Path (Join-Path $FixtureRoot 'app\studio') | Out-Null

  $Before = Get-FixtureSnapshot
  & $FixtureClean -Inventory
  & $FixtureClean -DryRun -Caches
  if ((Get-FixtureSnapshot) -ne $Before) { throw 'inventory/dry-run changed fixture files' }
  Assert-FixtureFiles @('app\studio') $true

  & $FixtureClean
  Assert-FixtureFiles ($Generated + @('app')) $false
  Assert-FixtureFiles ($Retained + $CacheFiles) $true
  & $FixtureClean -Caches
  Assert-FixtureFiles @('.cache') $false
  Assert-FixtureFiles $Retained $true
  $After = Get-FixtureSnapshot
  & $FixtureClean -Caches
  if ((Get-FixtureSnapshot) -ne $After) { throw 'repeated cleanup changed retained files' }

  $Rejected = $false
  try {
    & {
      . $FixtureClean -Inventory
      $null = Resolve-RepoTarget -RelativePath '..\repo-sibling\victim.txt'
    }
  } catch {
    if ($_.Exception.Message -notlike '*outside repo*') { throw }
    $Rejected = $true
  }
  if (-not $Rejected) { throw 'cleanup accepted a sibling path with the same repository prefix' }

  # Junctions work without Windows developer mode or elevated symlink privileges.
  [IO.File]::WriteAllText((Join-Path $SiblingRoot 'victim.txt'), 'must survive')
  Write-FixtureFile '.tmp\keep-until-safe.txt'
  New-Item -ItemType Junction -Path $JunctionPath -Target $SiblingRoot | Out-Null
  $Rejected = $false
  try { & $FixtureClean -Caches } catch {
    if ($_.Exception.Message -notlike '*reparse point*') { throw }
    $Rejected = $true
  }
  if (-not $Rejected) { throw 'cleanup accepted an external junction in a generated directory' }
  Assert-FixtureFiles @('.tmp\keep-until-safe.txt') $true
  if ([IO.File]::ReadAllText((Join-Path $SiblingRoot 'victim.txt')) -ne 'must survive') {
    throw 'cleanup changed a file outside the fixture repository'
  }
  Write-Host 'cleanup tests passed: dry-run, actual removal, latest build and source preservation, caches, idempotence, containment, and junction safety'
} finally {
  # Remove only the link itself before deleting our verified, isolated fixture tree.
  if (Test-Path -LiteralPath $JunctionPath) {
    $Link = Get-Item -LiteralPath $JunctionPath -Force
    if ($Link.Attributes -band [IO.FileAttributes]::ReparsePoint) { [IO.Directory]::Delete($JunctionPath) }
  }
  if (Test-Path -LiteralPath $TestRoot) {
    $ResolvedTestRoot = (Resolve-Path -LiteralPath $TestRoot).Path
    if ($ResolvedTestRoot -ne $TestRoot -or -not $ResolvedTestRoot.StartsWith($TestPrefix, [StringComparison]::OrdinalIgnoreCase)) {
      throw "refusing to remove unsafe fixture tree: $ResolvedTestRoot"
    }
    $UnexpectedLinks = @(Get-ChildItem -LiteralPath $ResolvedTestRoot -Recurse -Force -Attributes ReparsePoint)
    if ($UnexpectedLinks.Count -gt 0) { throw 'refusing to remove a fixture tree containing unexpected links' }
    Remove-Item -LiteralPath $ResolvedTestRoot -Recurse -Force
  }
}
