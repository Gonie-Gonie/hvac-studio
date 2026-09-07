param(
  [string]$Version = ''
)

$ErrorActionPreference = 'Stop'
$RepoRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).Path
. (Join-Path $RepoRoot 'scripts\release\package-common.ps1')

$ResolvedVersion = Resolve-Version -Version $Version
$CandidateRoot = Join-Path $RepoRoot '.tmp\latest-package'
$LatestRoot = Join-Path $RepoRoot 'dist\latest'
$UnpackRoot = Join-Path $RepoRoot ('.tmp\unpack-' + [Guid]::NewGuid().ToString('N').Substring(0, 8))
$PreviousRoot = Join-Path $RepoRoot ('.tmp\previous-' + [Guid]::NewGuid().ToString('N').Substring(0, 8))

function Invoke-BuildStep {
  param([string]$Script, [string[]]$Arguments = @())
  & powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $RepoRoot $Script) @Arguments
  if ($LASTEXITCODE -ne 0) {
    throw "$Script failed with exit code $LASTEXITCODE"
  }
}

foreach ($Target in @($CandidateRoot, $LatestRoot, $UnpackRoot, $PreviousRoot)) {
  $Absolute = [IO.Path]::GetFullPath($Target)
  if (-not $Absolute.StartsWith($RepoRoot + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) {
    throw "build path must stay inside the repository: $Absolute"
  }
  $Ancestor = $Absolute
  while ($Ancestor -and $Ancestor -ne $RepoRoot) {
    if ((Test-Path -LiteralPath $Ancestor) -and
        ((Get-Item -LiteralPath $Ancestor -Force).Attributes -band [IO.FileAttributes]::ReparsePoint)) {
      throw "build path must not pass through a link: $Ancestor"
    }
    $Ancestor = Split-Path -Parent $Ancestor
  }
}

# Build and verify the candidate before replacing the previous working build.
Invoke-BuildStep -Script 'scripts\release\package-portable.ps1' -Arguments @(
  '-Version', $ResolvedVersion, '-OutputRoot', $CandidateRoot
)
$CandidateZip = Join-Path $CandidateRoot "hvac-studio-$ResolvedVersion-windows-amd64-portable.zip"
Invoke-BuildStep -Script 'scripts\release\test-portable-package.ps1' -Arguments @('-PackagePath', $CandidateZip)
Expand-Archive -LiteralPath $CandidateZip -DestinationPath $UnpackRoot
$CandidateFolder = Join-Path $UnpackRoot "hvac-studio-$ResolvedVersion-windows-amd64-portable"
if (-not (Test-Path -LiteralPath (Join-Path $CandidateFolder 'HVAC Studio.exe'))) {
  throw 'verified package is missing the portable entrypoint'
}

# Preserve the complete workspace by moving it, including user assets and links.
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $LatestRoot) | Out-Null
if (Test-Path -LiteralPath $LatestRoot) {
  Move-Item -LiteralPath $LatestRoot -Destination $PreviousRoot
}
$PreviousProjects = Join-Path $PreviousRoot 'projects'
$CandidateProjects = Join-Path $CandidateFolder 'projects'
$ProjectsMoved = $false
try {
  if (Test-Path -LiteralPath $PreviousProjects) {
    Remove-Item -LiteralPath (Join-Path $CandidateProjects 'README.md') -Force
    Remove-Item -LiteralPath $CandidateProjects
    Move-Item -LiteralPath $PreviousProjects -Destination $CandidateProjects
    $ProjectsMoved = $true
  }
  Move-Item -LiteralPath $CandidateFolder -Destination $LatestRoot
} catch {
  if ((Test-Path -LiteralPath $PreviousRoot) -and -not (Test-Path -LiteralPath $LatestRoot)) {
    if ($ProjectsMoved) {
      Move-Item -LiteralPath $CandidateProjects -Destination $PreviousProjects
    }
    Move-Item -LiteralPath $PreviousRoot -Destination $LatestRoot
  }
  throw
}
Invoke-BuildStep -Script 'scripts\dev\clean-generated.ps1'
Write-Host "Latest verified build: $(Join-Path $LatestRoot 'HVAC Studio.exe')"
