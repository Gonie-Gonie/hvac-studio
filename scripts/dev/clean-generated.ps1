param(
  [switch]$DryRun
)

$ErrorActionPreference = 'Stop'

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$Targets = @(
  'artifacts',
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

$Removed = New-Object System.Collections.Generic.List[string]
$Skipped = New-Object System.Collections.Generic.List[string]

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

if ($Removed.Count -gt 0) {
  Write-Host "removed generated paths: $($Removed -join ', ')"
} else {
  Write-Host 'no generated paths to remove'
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
