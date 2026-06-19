$ErrorActionPreference = 'Stop'

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$CleanScript = Join-Path $RepoRoot 'scripts\dev\clean-generated.ps1'
$DistRoot = Join-Path $RepoRoot 'dist'

function Get-DistZipSnapshot {
  if (-not (Test-Path -LiteralPath $DistRoot -PathType Container)) {
    return @()
  }
  return @(Get-ChildItem -LiteralPath $DistRoot -File -Filter '*.zip' |
      Sort-Object Name |
      ForEach-Object { "$($_.Name):$($_.Length)" })
}

$BeforeZips = Get-DistZipSnapshot

& $CleanScript -Inventory
& $CleanScript -DryRun

$AfterZips = Get-DistZipSnapshot
$BeforeText = $BeforeZips -join "`n"
$AfterText = $AfterZips -join "`n"
if ($BeforeText -ne $AfterText) {
  throw "clean-generated dry-run changed retained dist zips:`nbefore:`n$BeforeText`nafter:`n$AfterText"
}

Write-Host 'cleanup script ok'
